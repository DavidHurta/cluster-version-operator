package resourcemerge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestEnsureValidatingWebhookConfiguration(t *testing.T) {
	defaulting := struct {
		failurePolicy     admissionregv1.FailurePolicyType
		matchPolicy       admissionregv1.MatchPolicyType
		namespaceSelector *metav1.LabelSelector
		objectSelector    *metav1.LabelSelector
		timeoutSeconds    *int32
	}{
		failurePolicy:     admissionregv1.Fail,
		matchPolicy:       admissionregv1.Equivalent,
		namespaceSelector: &metav1.LabelSelector{},
		objectSelector:    &metav1.LabelSelector{},
		timeoutSeconds:    ptr.To(int32(10)),
	}
	nonDefaulting := struct {
		failurePolicy     admissionregv1.FailurePolicyType
		matchPolicy       admissionregv1.MatchPolicyType
		namespaceSelector *metav1.LabelSelector
		objectSelector    *metav1.LabelSelector
		timeoutSeconds    *int32
	}{
		failurePolicy: admissionregv1.Ignore,
		matchPolicy:   admissionregv1.Exact,
		namespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "foo",
			},
		},
		objectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "foo",
			},
		},
		timeoutSeconds: ptr.To(int32(42)),
	}

	tests := []struct {
		name     string
		existing admissionregv1.ValidatingWebhookConfiguration
		required admissionregv1.ValidatingWebhookConfiguration

		expectedModified bool
		expected         admissionregv1.ValidatingWebhookConfiguration
	}{
		{
			name: "same failure policy",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &nonDefaulting.failurePolicy,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &nonDefaulting.failurePolicy,
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &nonDefaulting.failurePolicy,
					},
				},
			},
		},
		{
			name: "different failure policy",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &defaulting.failurePolicy,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &nonDefaulting.failurePolicy,
					},
				},
			},

			expectedModified: true,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &nonDefaulting.failurePolicy,
					},
				},
			},
		},
		{
			name: "implicit failure policy",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &defaulting.failurePolicy,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						FailurePolicy: &defaulting.failurePolicy,
					},
				},
			},
		},
		{
			name: "same match policy",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &nonDefaulting.matchPolicy,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &nonDefaulting.matchPolicy,
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &nonDefaulting.matchPolicy,
					},
				},
			},
		},
		{
			name: "different match policy",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &defaulting.matchPolicy,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &nonDefaulting.matchPolicy,
					},
				},
			},

			expectedModified: true,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &nonDefaulting.matchPolicy,
					},
				},
			},
		},
		{
			name: "implicit match policy",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &defaulting.matchPolicy,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						MatchPolicy: &defaulting.matchPolicy,
					},
				},
			},
		},
		{
			name: "same namespace selector",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.namespaceSelector,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.namespaceSelector,
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.namespaceSelector,
					},
				},
			},
		},
		{
			name: "different namespace selector",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: defaulting.namespaceSelector,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.namespaceSelector,
					},
				},
			},

			expectedModified: true,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.namespaceSelector,
					},
				},
			},
		},
		{
			name: "implicit namespace selector",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: defaulting.namespaceSelector,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: defaulting.namespaceSelector,
					},
				},
			},
		},
		{
			name: "same object selector",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.objectSelector,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.objectSelector,
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.objectSelector,
					},
				},
			},
		},
		{
			name: "different object selector",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: defaulting.objectSelector,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.objectSelector,
					},
				},
			},

			expectedModified: true,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: nonDefaulting.objectSelector,
					},
				},
			},
		},
		{
			name: "implicit object selector",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: defaulting.objectSelector,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						NamespaceSelector: defaulting.objectSelector,
					},
				},
			},
		},
		{
			name: "same timeout seconds",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: nonDefaulting.timeoutSeconds,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: nonDefaulting.timeoutSeconds,
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: nonDefaulting.timeoutSeconds,
					},
				},
			},
		},
		{
			name: "different timeout seconds",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: defaulting.timeoutSeconds,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: nonDefaulting.timeoutSeconds,
					},
				},
			},

			expectedModified: true,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: nonDefaulting.timeoutSeconds,
					},
				},
			},
		},
		{
			name: "implicit timeout seconds",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: defaulting.timeoutSeconds,
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						TimeoutSeconds: defaulting.timeoutSeconds,
					},
				},
			},
		},
		{
			name: "respect injected caBundle when the annotation `...inject-cabundle=true` is set",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: nil,
						},
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
				},
			},
		},
		{
			name: "respect injected caBundle when the annotation `...inject-cabundle=true` is set by the user",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: nil,
						},
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
				},
			},
		},
		{
			name: "remove injected caBundle when the annotation `...inject-cabundle=true` is not set",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the user"),
						},
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: nil,
						},
					},
				},
			},

			expectedModified: true,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: nil,
						},
					},
				},
			},
		},
		{
			name: "respect injected caBundles in all webhooks when the annotation `...inject-cabundle=true` is set",
			existing: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
				},
			},
			required: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: nil,
						},
					},
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: nil,
						},
					},
				},
			},

			expectedModified: false,
			expected: admissionregv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						injectCABundleAnnotation: "true",
					},
				},
				Webhooks: []admissionregv1.ValidatingWebhook{
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
					{
						ClientConfig: admissionregv1.WebhookClientConfig{
							CABundle: []byte("CA bundle injected by the ca operator"),
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defaultValidatingWebhookConfiguration(&test.existing, test.existing)
			defaultValidatingWebhookConfiguration(&test.expected, test.expected)
			modified := ptr.To(false)
			EnsureValidatingWebhookConfiguration(modified, &test.existing, test.required)
			if *modified != test.expectedModified {
				t.Errorf("mismatch modified got: %v want: %v", *modified, test.expectedModified)
			}

			if !equality.Semantic.DeepEqual(test.existing, test.expected) {
				t.Errorf("unexpected: %s", cmp.Diff(test.expected, test.existing))
			}
		})
	}
}

// Ensures the structure contains any defaults not explicitly set by the test
func defaultValidatingWebhookConfiguration(in *admissionregv1.ValidatingWebhookConfiguration, from admissionregv1.ValidatingWebhookConfiguration) {
	modified := ptr.To(false)
	EnsureValidatingWebhookConfiguration(modified, in, from)
}
