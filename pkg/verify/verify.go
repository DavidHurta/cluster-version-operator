package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/openpgp"
	"k8s.io/klog"

	"github.com/openshift/cluster-version-operator/pkg/verify/store"
)

// Interface performs verification of the provided content. The default implementation
// in this package uses the container signature format defined at https://github.com/containers/image
// to authenticate that a given release image digest has been signed by a trusted party.
type Interface interface {
	// Verify should return nil if the provided release digest has suffient signatures to be considered
	// valid. It should return an error in all other cases.
	Verify(ctx context.Context, releaseDigest string) error
}

type rejectVerifier struct{}

func (rejectVerifier) Verify(ctx context.Context, releaseDigest string) error {
	return fmt.Errorf("verification is not possible")
}

// Reject fails always fails verification.
var Reject Interface = rejectVerifier{}

// HTTPClient returns a client suitable for retrieving signatures. It is not
// required to be unique per call, but may be called concurrently.
type HTTPClient func() (*http.Client, error)

// ClientBuilder provides a method for generating an HTTP Client configured
// with cluster proxy settings, if they exist.
type ClientBuilder interface {
	// HTTPClient returns a client suitable for retrieving signatures. It is not
	// required to be unique per call, but may be called concurrently.
	HTTPClient() (*http.Client, error)
}

// DefaultClient uses the default http.Client for accessing signatures.
var DefaultClient = simpleClientBuilder{}

// simpleClientBuilder implements the ClientBuilder interface and may be used for testing.
type simpleClientBuilder struct{}

// HTTPClient from simpleClientBuilder creates an http.Client with no configuration.
func (s simpleClientBuilder) HTTPClient() (*http.Client, error) {
	return &http.Client{}, nil
}

// maxSignatureSearch prevents unbounded recursion on malicious signature stores (if
// an attacker was able to take ownership of the store to perform DoS on clusters).
const maxSignatureSearch = 10

// validReleaseDigest is a verification rule to filter clearly invalid digests.
var validReleaseDigest = regexp.MustCompile(`^[a-zA-Z0-9:]+$`)

// ReleaseVerifier implements a signature intersection operation on a provided release
// digest - all verifiers must have at least one valid signature attesting the release
// digest. If any failure occurs the caller should assume the content is unverified.
type ReleaseVerifier struct {
	verifiers map[string]openpgp.EntityList

	// Store is the store from which release signatures are retrieved.
	Store store.Store

	lock           sync.Mutex
	signatureCache map[string][][]byte
}

// NewReleaseVerifier creates a release verifier for the provided inputs.
func NewReleaseVerifier(verifiers map[string]openpgp.EntityList, store store.Store) *ReleaseVerifier {
	return &ReleaseVerifier{
		verifiers: verifiers,
		Store:     store,

		signatureCache: make(map[string][][]byte),
	}
}

// Verifiers returns a copy of the verifiers in this payload.
func (v *ReleaseVerifier) Verifiers() map[string]openpgp.EntityList {
	out := make(map[string]openpgp.EntityList, len(v.verifiers))
	for k, v := range v.verifiers {
		out[k] = v
	}
	return out
}

// String summarizes the verifier for human consumption
func (v *ReleaseVerifier) String() string {
	var keys []string
	for name := range v.verifiers {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.Grow(256)
	fmt.Fprintf(&builder, "All release image digests must have GPG signatures from")
	if len(keys) == 0 {
		fmt.Fprint(&builder, " <ERROR: no verifiers>")
	}
	for _, name := range keys {
		verifier := v.verifiers[name]
		fmt.Fprintf(&builder, " %s (", name)
		for i, entity := range verifier {
			if i != 0 {
				fmt.Fprint(&builder, ", ")
			}
			if entity.PrimaryKey != nil {
				fmt.Fprintf(&builder, strings.ToUpper(fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)))
				fmt.Fprint(&builder, ": ")
			}
			count := 0
			for identityName := range entity.Identities {
				if count != 0 {
					fmt.Fprint(&builder, ", ")
				}
				fmt.Fprintf(&builder, "%s", identityName)
				count++
			}
		}
		fmt.Fprint(&builder, ")")
	}
	fmt.Fprintf(&builder, " - will check for signatures in containers/image format at")
	if v.Store == nil {
		fmt.Fprint(&builder, " <ERROR: no store>")
	} else {
		fmt.Fprintf(&builder, " %s", v.Store)
	}
	return builder.String()
}

// Verify ensures that at least one valid signature exists for an image with digest
// matching release digest in any of the provided locations for all verifiers, or returns
// an error.
func (v *ReleaseVerifier) Verify(ctx context.Context, releaseDigest string) error {
	if len(v.verifiers) == 0 || v.Store == nil {
		return fmt.Errorf("the release verifier is incorrectly configured, unable to verify digests")
	}
	if len(releaseDigest) == 0 {
		return fmt.Errorf("release images that are not accessed via digest cannot be verified")
	}
	if !validReleaseDigest.MatchString(releaseDigest) {
		return fmt.Errorf("the provided release image digest contains prohibited characters")
	}

	if v.hasVerified(releaseDigest) {
		return nil
	}

	remaining := make(map[string]openpgp.EntityList, len(v.verifiers))
	for k, v := range v.verifiers {
		remaining[k] = v
	}

	var signedWith [][]byte
	err := v.Store.Signatures(ctx, "", releaseDigest, func(ctx context.Context, signature []byte, errIn error) (done bool, err error) {
		if errIn != nil {
			klog.V(4).Infof("error retrieving signature for %s: %v", releaseDigest, errIn)
			return false, nil
		}
		for k, keyring := range remaining {
			content, _, err := verifySignatureWithKeyring(bytes.NewReader(signature), keyring)
			if err != nil {
				klog.V(4).Infof("keyring %q could not verify signature for %s: %v", k, releaseDigest, err)
				continue
			}
			if err := verifyAtomicContainerSignature(content, releaseDigest); err != nil {
				klog.V(4).Infof("signature for %s is not valid: %v", releaseDigest, err)
				continue
			}
			delete(remaining, k)
			signedWith = append(signedWith, signature)
		}
		return len(remaining) == 0, nil
	})
	if err != nil {
		klog.V(4).Infof("Failed to retrieve signatures for %s (should never happen)", releaseDigest)
		return err
	}

	if len(remaining) > 0 {
		if klog.V(4) {
			for k := range remaining {
				klog.Infof("Unable to verify %s against keyring %s", releaseDigest, k)
			}
		}
		return fmt.Errorf("unable to locate a valid signature for one or more sources")
	}

	v.cacheVerification(releaseDigest, signedWith)

	return nil
}

// Signatures returns a copy of any cached signatures that have been validated
// so far. It may return no signatures.
func (v *ReleaseVerifier) Signatures() map[string][][]byte {
	copied := make(map[string][][]byte)
	v.lock.Lock()
	defer v.lock.Unlock()
	for k, v := range v.signatureCache {
		copied[k] = v
	}
	return copied
}

// hasVerified returns true if the digest has already been verified.
func (v *ReleaseVerifier) hasVerified(releaseDigest string) bool {
	v.lock.Lock()
	defer v.lock.Unlock()
	_, ok := v.signatureCache[releaseDigest]
	return ok
}

const maxSignatureCacheSize = 64

// cacheVerification caches the result of signature check for a digest for later retrieval.
func (v *ReleaseVerifier) cacheVerification(releaseDigest string, signedWith [][]byte) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if len(signedWith) == 0 || len(releaseDigest) == 0 || v.signatureCache == nil {
		return
	}
	// remove the new entry
	delete(v.signatureCache, releaseDigest)
	// ensure the cache doesn't grow beyond our cap
	for k := range v.signatureCache {
		if len(v.signatureCache) < maxSignatureCacheSize {
			break
		}
		delete(v.signatureCache, k)
	}
	v.signatureCache[releaseDigest] = signedWith
}

type fileStore struct {
	directory string
}

// Signatures reads signatures as "signature-1", "signature-2", etc. out of a digest-based subdirectory.
func (s *fileStore) Signatures(ctx context.Context, name string, digest string, fn store.Callback) error {
	parts := strings.SplitN(digest, ":", 3)
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return fmt.Errorf("the provided release image digest must be of the form ALGO:HASH")
	}
	algo, hash := parts[0], parts[1]
	digestPathSegment := fmt.Sprintf("%s=%s", algo, hash)

	base := filepath.Join(s.directory, digestPathSegment, "signature-")
	for i := 1; i < maxSignatureSearch; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		path := base + strconv.Itoa(i)
		data, err := ioutil.ReadFile(path)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			klog.V(4).Infof("unable to load signature: %v", err)
			done, err := fn(ctx, nil, err)
			if done || err != nil {
				return err
			}
			continue
		}
		done, err := fn(ctx, data, nil)
		if done || err != nil {
			return err
		}
	}
	return nil
}

func (s *fileStore) String() string {
	return fmt.Sprintf("file://%s", s.directory)
}

var errNotFound = fmt.Errorf("no more signatures to check")

type httpStore struct {
	uri        *url.URL
	httpClient HTTPClient
}

// Signatures reads signatures as "signature-1", "signature-2", etc. as children of a digest URI
// over HTTP or HTTPS until either the provided fn returns an error, false, or the server returns 404. No
// more than maxSignaturesToCheck will be read. If the provided context is cancelled search will be terminated.
func (s *httpStore) Signatures(ctx context.Context, name string, digest string, fn store.Callback) error {
	parts := strings.SplitN(digest, ":", 3)
	if len(parts) != 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return fmt.Errorf("the provided release image digest must be of the form ALGO:HASH")
	}
	algo, hash := parts[0], parts[1]
	digestPathSegment := fmt.Sprintf("%s=%s", algo, hash)

	switch s.uri.Scheme {
	case "http", "https":
		client, err := s.httpClient()
		if err != nil {
			_, err = fn(ctx, nil, err)
			return err
		}

		copied := *s.uri
		copied.Path = path.Join(copied.Path, digestPathSegment)
		if err := checkHTTPSignatures(ctx, client, copied, maxSignatureSearch, fn); err != nil {
			return err
		}
	default:
		return fmt.Errorf("the store %s scheme is unrecognized", s.uri)
	}

	return nil
}

// checkHTTPSignatures reads signatures as "signature-1", "signature-2", etc. as children of the provided URL
// over HTTP or HTTPS.  No more than maxSignatureSearch will be read. If the provided context is cancelled
// search will be terminated.
func checkHTTPSignatures(ctx context.Context, client *http.Client, u url.URL, maxSignaturesToCheck int, fn store.Callback) error {
	base := path.Join(u.Path, "signature-")
	sigURL := u
	for i := 1; i < maxSignatureSearch; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		sigURL.Path = base + strconv.Itoa(i)

		req, err := http.NewRequest("GET", sigURL.String(), nil)
		if err != nil {
			_, err = fn(ctx, nil, fmt.Errorf("could not build request to check signature: %v", err))
			return err // even if the callback ate the error, no sense in checking later indexes which will fail the same way
		}
		req = req.WithContext(ctx)
		// load the body, being careful not to allow unbounded reads
		resp, err := client.Do(req)
		if err != nil {
			klog.V(4).Infof("unable to load signature: %v", err)
			done, err := fn(ctx, nil, err)
			if done || err != nil {
				return err
			}
			continue
		}
		data, err := func() ([]byte, error) {
			body := resp.Body
			r := io.LimitReader(body, 50*1024)

			defer func() {
				// read the remaining body to avoid breaking the connection
				io.Copy(ioutil.Discard, r)
				body.Close()
			}()

			if resp.StatusCode == http.StatusNotFound {
				return nil, errNotFound
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				if i == 1 {
					klog.V(4).Infof("Could not find signature at store location %v", sigURL)
				}
				return nil, fmt.Errorf("unable to retrieve signature from %v: %d", sigURL, resp.StatusCode)
			}

			return ioutil.ReadAll(resp.Body)
		}()
		if err == errNotFound {
			break
		}
		if err != nil {
			klog.V(4).Info(err)
			done, err := fn(ctx, nil, err)
			if done || err != nil {
				return err
			}
			continue
		}
		if len(data) == 0 {
			continue
		}

		done, err := fn(ctx, data, nil)
		if done || err != nil {
			return err
		}
	}
	return nil
}

// String returns a description of where this store finds
// signatures.
func (s *httpStore) String() string {
	return s.uri.String()
}

// verifySignatureWithKeyring performs a containers/image verification of the provided signature
// message, checking for the integrity and authenticity of the provided message in r. It will return
// the identity of the signer if successful along with the message contents.
func verifySignatureWithKeyring(r io.Reader, keyring openpgp.EntityList) ([]byte, string, error) {
	md, err := openpgp.ReadMessage(r, keyring, nil, nil)
	if err != nil {
		return nil, "", fmt.Errorf("could not read the message: %v", err)
	}
	if !md.IsSigned {
		return nil, "", fmt.Errorf("not signed")
	}
	content, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, "", err
	}
	if md.SignatureError != nil {
		return nil, "", fmt.Errorf("signature error: %v", md.SignatureError)
	}
	if md.SignedBy == nil {
		return nil, "", fmt.Errorf("invalid signature")
	}
	if md.Signature != nil {
		if md.Signature.SigLifetimeSecs != nil {
			expiry := md.Signature.CreationTime.Add(time.Duration(*md.Signature.SigLifetimeSecs) * time.Second)
			if time.Now().After(expiry) {
				return nil, "", fmt.Errorf("signature expired on %s", expiry)
			}
		}
	} else if md.SignatureV3 == nil {
		return nil, "", fmt.Errorf("unexpected openpgp.MessageDetails: neither Signature nor SignatureV3 is set")
	}

	// follow conventions in containers/image
	return content, strings.ToUpper(fmt.Sprintf("%x", md.SignedBy.PublicKey.Fingerprint)), nil
}

// An atomic container signature has the following schema:
//
// {
// 	"critical": {
// 			"type": "atomic container signature",
// 			"image": {
// 					"docker-manifest-digest": "sha256:817a12c32a39bbe394944ba49de563e085f1d3c5266eb8e9723256bc4448680e"
// 			},
// 			"identity": {
// 					"docker-reference": "docker.io/library/busybox:latest"
// 			}
// 	},
// 	"optional": {
// 			"creator": "some software package v1.0.1-35",
// 			"timestamp": 1483228800,
// 	}
// }
type signature struct {
	Critical criticalSignature `json:"critical"`
	Optional optionalSignature `json:"optional"`
}

type criticalSignature struct {
	Type     string           `json:"type"`
	Image    criticalImage    `json:"image"`
	Identity criticalIdentity `json:"identity"`
}

type criticalImage struct {
	DockerManifestDigest string `json:"docker-manifest-digest"`
}

type criticalIdentity struct {
	DockerReference string `json:"docker-reference"`
}

type optionalSignature struct {
	Creator   string `json:"creator"`
	Timestamp int64  `json:"timestamp"`
}

// verifyAtomicContainerSignature verifiers that the provided data authenticates the
// specified release digest. If error is returned the provided data does NOT authenticate
// the release digest and the signature must be ignored.
func verifyAtomicContainerSignature(data []byte, releaseDigest string) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var sig signature
	if err := d.Decode(&sig); err != nil {
		return fmt.Errorf("the signature is not valid JSON: %v", err)
	}
	if sig.Critical.Type != "atomic container signature" {
		return fmt.Errorf("signature is not the correct type")
	}
	if len(sig.Critical.Identity.DockerReference) == 0 {
		return fmt.Errorf("signature must have an identity")
	}
	if sig.Critical.Image.DockerManifestDigest != releaseDigest {
		return fmt.Errorf("signature digest does not match")
	}
	return nil
}
