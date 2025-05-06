package configuration

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	operatorv1 "github.com/openshift/api/operator/v1"
)

func TestClusterVersionOperatorConfigurationFile_load(t *testing.T) {
	tests := []struct {
		name                    string
		filename                string
		expectedDesiredLogLevel operatorv1.LogLevel
		expectedErr             bool
	}{
		{
			name:                    "config-valid-log-normal",
			filename:                "config-valid-log-normal.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
		},
		{
			name:                    "config-valid-log-debug",
			filename:                "config-valid-log-debug.yaml",
			expectedDesiredLogLevel: operatorv1.Debug,
		},
		{
			name:                    "config-valid-log-trace",
			filename:                "config-valid-log-trace.yaml",
			expectedDesiredLogLevel: operatorv1.Trace,
		},
		{
			name:                    "config-valid-log-traceall",
			filename:                "config-valid-log-traceall.yaml",
			expectedDesiredLogLevel: operatorv1.TraceAll,
		},
		{
			name:                    "config-valid-log-missing",
			filename:                "config-valid-log-missing.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
		},
		{
			name:                    "config-valid-log-empty",
			filename:                "config-valid-log-empty.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
		},
		{
			name:                    "config-invalid-empty-file",
			filename:                "config-invalid-empty-file.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
			expectedErr:             true,
		},
		{
			name:                    "config-invalid-log-field-name-typo",
			filename:                "config-invalid-log-field-name-typo.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
			expectedErr:             true,
		},
		{
			name:                    "config-invalid-log-level",
			filename:                "config-invalid-log-level.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
			expectedErr:             true,
		},
		{
			name:                    "config-invalid-unsupported-version",
			filename:                "config-invalid-unsupported-version.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
			expectedErr:             true,
		},
		{
			name:                    "config-invalid-file-does-not-exist",
			filename:                "THIS_FILE_DOES_NOT_EXIST.yaml",
			expectedDesiredLogLevel: operatorv1.Normal,
			expectedErr:             true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join("testdata", tt.filename)
			config := NewClusterVersionOperatorConfigurationFile(filename)
			if err := config.load(); (err != nil) != tt.expectedErr {
				t.Errorf("load() error = %v, expectedErr %v", err, tt.expectedErr)
			}
			if cmp.Diff(config.desiredLogLevel, tt.expectedDesiredLogLevel) != "" {
				t.Errorf("load() desiredLogLevel = %v, want %v", config.desiredLogLevel, tt.expectedDesiredLogLevel)
			}
		})
	}
}
