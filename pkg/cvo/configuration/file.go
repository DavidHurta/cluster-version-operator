package configuration

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

type ClusterVersionOperatorConfigurationFile struct {
	path            string
	desiredLogLevel operatorv1.LogLevel
}

func NewClusterVersionOperatorConfigurationFile(path string) *ClusterVersionOperatorConfigurationFile {
	return &ClusterVersionOperatorConfigurationFile{
		path:            path,
		desiredLogLevel: operatorv1.Normal,
	}
}

func (config *ClusterVersionOperatorConfigurationFile) setDefaultDesired() {
	config.desiredLogLevel = operatorv1.Normal
}

func (config *ClusterVersionOperatorConfigurationFile) load() error {
	bytes, err := os.ReadFile(config.path)
	if err != nil {
		return err
	}

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	if err := operatorv1alpha1.Install(scheme); err != nil {
		return err
	}
	decoder := codecs.UniversalDecoder(operatorv1alpha1.GroupVersion)

	obj, err := runtime.Decode(decoder, bytes)
	if err != nil {
		return err
	}
	switch obj := obj.(type) {
	case *operatorv1alpha1.ClusterVersionOperator:
		loadedConfig := ClusterVersionOperatorConfigurationFile{desiredLogLevel: obj.Spec.OperatorLogLevel}
		if err := loadedConfig.validate(); err != nil {
			return fmt.Errorf("failed to validate CVO configuration: %w", err)
		}
		config.desiredLogLevel = loadedConfig.desiredLogLevel
		if config.desiredLogLevel == "" {
			config.desiredLogLevel = operatorv1.Normal
		}
		return nil
	default:
		return fmt.Errorf("unsupported object type %T", obj)
	}
}

func (config *ClusterVersionOperatorConfigurationFile) validate() error {
	switch config.desiredLogLevel {
	case "":
	case operatorv1.Normal:
	case operatorv1.Debug:
	case operatorv1.Trace:
	case operatorv1.TraceAll:
	default:
		return fmt.Errorf("invalid log level: `%s`", config.desiredLogLevel)
	}
	return nil
}

func (config *ClusterVersionOperatorConfigurationFile) Sync() error {
	if err := config.load(); err != nil {
		klog.Errorf("unable to load configuration, setting default: %v", err)
		config.setDefaultDesired()
	}
	if err := config.apply(); err != nil {
		return fmt.Errorf("unable to apply configuration: %w", err)
	}
	return nil
}

func (config *ClusterVersionOperatorConfigurationFile) apply() error {
	return applyLogLevel(config.desiredLogLevel)
}
