package cvo

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/loglevel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/cluster-version-operator/pkg/internal"
)

const ClusterVersionOperatorConfigurationName = "cluster"

func (optr *Operator) syncConfiguration(ctx context.Context, config *operatorv1alpha1.ClusterVersionOperator) error {
	if !optr.enabledFeatureGates.CVOConfiguration() {
		// TODO: Change to `internal.Debug` after testing
		// TODO: Reconsider placement of FeatureGate checks
		klog.V(internal.Normal).Infof("The ClusterVersionOperatorConfiguration feature gate is disabled; skipping sync configuration")
		return nil
	}

	if config.Status.ObservedGeneration != config.Generation {
		config.Status.ObservedGeneration = config.Generation
		_, err := optr.operatorClient.OperatorV1alpha1().ClusterVersionOperators().UpdateStatus(ctx, config, metav1.UpdateOptions{})
		if err != nil {
			err = fmt.Errorf("Failed to update the ClusterVersionOperator resource, err=%w", err)
			klog.Error(err)
			return err
		}
	}

	current, notFound := loglevel.GetLogLevel()
	if notFound {
		klog.Warningf("The current log level could not be found; an attempt to set the log level to desired level will be made")
	}

	desired := config.Spec.OperatorLogLevel
	if !notFound && current == desired {
		klog.Infof("No need to update the current CVO log level '%s'; it is already set to the desired value", current)
	} else {
		err := loglevel.SetLogLevel(desired)
		if err != nil {
			err = fmt.Errorf("Failed to set the log level to %s, err=%w", desired, err)
			klog.Error(err)
			return err
		}
		klog.Infof("Successfully updated the log level from '%s' to '%s'", current, desired)

		// TODO: Remove when done
		klog.V(internal.Normal).Infof("This line is only printed at the 'Normal' log level and above")
		klog.V(internal.Debug).Infof("This line is only printed at the 'Debug' log level and above")
		klog.V(internal.Trace).Infof("This line is only printed at the 'Trace' log level and above")
		klog.V(internal.TraceAll).Infof("This line is only printed at the 'TraceAll' log level and above")
	}
	return nil
}
