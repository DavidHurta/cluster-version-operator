package featurechangestopper

import (
	"context"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	fakeconfigv1client "github.com/openshift/client-go/config/clientset/versioned/fake"
	configv1informer "github.com/openshift/client-go/config/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTechPreviewChangeStopper(t *testing.T) {
	tests := []struct {
		name                       string
		startingRequiredFeatureSet string
		featureGate                string
		expectedShutdownCalled     bool
	}{
		{
			name:                       "default-no-change",
			startingRequiredFeatureSet: "",
			featureGate:                "",
			expectedShutdownCalled:     false,
		},
		{
			name:                       "default-with-change-to-tech-preview",
			startingRequiredFeatureSet: "",
			featureGate:                "TechPreviewNoUpgrade",
			expectedShutdownCalled:     true,
		},
		{
			name:                       "default-with-change-to-other",
			startingRequiredFeatureSet: "",
			featureGate:                "AnythingElse",
			expectedShutdownCalled:     true,
		},
		{
			name:                       "techpreview-to-techpreview",
			startingRequiredFeatureSet: "TechPreviewNoUpgrade",
			featureGate:                "TechPreviewNoUpgrade",
			expectedShutdownCalled:     false,
		},
		{
			name:                       "techpreview-to-not-tech-preview", // this isn't allowed today
			startingRequiredFeatureSet: "TechPreviewNoUpgrade",
			featureGate:                "",
			expectedShutdownCalled:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			actualShutdownCalled := false
			shutdownFn := func() {
				actualShutdownCalled = true
			}

			client := fakeconfigv1client.NewSimpleClientset(
				&configv1.FeatureGate{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec: configv1.FeatureGateSpec{
						FeatureGateSelection: configv1.FeatureGateSelection{
							FeatureSet: configv1.FeatureSet(tt.featureGate),
						},
					},
				},
			)

			informerFactory := configv1informer.NewSharedInformerFactory(client, 0)
			featureGates := informerFactory.Config().V1().FeatureGates()
			c, err := New(tt.startingRequiredFeatureSet, featureGates)
			if err != nil {
				t.Fatal(err)
			}
			informerFactory.Start(ctx.Done())

			if err := c.Run(ctx, shutdownFn); err != nil {
				t.Fatal(err)
			}

			if actualShutdownCalled != tt.expectedShutdownCalled {
				t.Errorf("shutdown called %t, but expected %t", actualShutdownCalled, tt.expectedShutdownCalled)
			}
		})
	}
}
