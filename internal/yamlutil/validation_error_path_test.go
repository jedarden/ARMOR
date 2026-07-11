// Package yamlutil tests for ValidationError path formatting
package yamlutil

import (
	"testing"
)

// TestValidationErrorPathFormatting_SimplePaths tests simple field paths (single level)
func TestValidationErrorPathFormatting_SimplePaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		wantInMsg string // substring that should be in the error message
	}{
		{
			name:      "single field name",
			fieldPath: "replicas",
			wantInMsg: "at field replicas",
		},
		{
			name:      "single field with underscores",
			fieldPath: "max_connections",
			wantInMsg: "at field max_connections",
		},
		{
			name:      "single field with numbers",
			fieldPath: "port8080",
			wantInMsg: "at field port8080",
		},
		{
			name:      "single camelCase field",
			fieldPath: "connectionTimeout",
			wantInMsg: "at field connectionTimeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, "constraint", "", 0, 0, "", "")
			errorMsg := err.Error()

			if !contains(errorMsg, tt.wantInMsg) {
				t.Errorf("Error() should contain %q, got: %s", tt.wantInMsg, errorMsg)
			}

			// Verify the field path is set correctly
			if err.FieldPath != tt.fieldPath {
				t.Errorf("FieldPath = %q, want %q", err.FieldPath, tt.fieldPath)
			}
		})
	}
}

// TestValidationErrorPathFormatting_NestedPaths tests nested field paths (multiple levels)
func TestValidationErrorPathFormatting_NestedPaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		wantInMsg string
	}{
		{
			name:      "two-level nested path",
			fieldPath: "spec.replicas",
			wantInMsg: "at field spec.replicas",
		},
		{
			name:      "three-level nested path",
			fieldPath: "server.port",
			wantInMsg: "at field server.port",
		},
		{
			name:      "four-level nested path",
			fieldPath: "database.connectionPool.maxConnections",
			wantInMsg: "at field database.connectionPool.maxConnections",
		},
		{
			name:      "kubernetes-style spec path",
			fieldPath: "spec.template.spec.containers",
			wantInMsg: "at field spec.template.spec.containers",
		},
		{
			name:      "deep nested configuration path",
			fieldPath: "servers.api.responses.endpoints",
			wantInMsg: "at field servers.api.responses.endpoints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, "constraint", "", 0, 0, "", "")
			errorMsg := err.Error()

			if !contains(errorMsg, tt.wantInMsg) {
				t.Errorf("Error() should contain %q, got: %s", tt.wantInMsg, errorMsg)
			}
		})
	}
}

// TestValidationErrorPathFormatting_ArrayIndexedPaths tests array-indexed field paths
func TestValidationErrorPathFormatting_ArrayIndexedPaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		wantInMsg string
	}{
		{
			name:      "single array index",
			fieldPath: "containers[0]",
			wantInMsg: "at field containers[0]",
		},
		{
			name:      "array index with nested field",
			fieldPath: "containers[0].image",
			wantInMsg: "at field containers[0].image",
		},
		{
			name:      "nested array indexes",
			fieldPath: "servers[0].endpoints[1].port",
			wantInMsg: "at field servers[0].endpoints[1].port",
		},
		{
			name:      "kubernetes deployment path",
			fieldPath: "spec.template.spec.containers[0].image",
			wantInMsg: "at field spec.template.spec.containers[0].image",
		},
		{
			name:      "multiple nested arrays",
			fieldPath: "items[0].metadata.labels[0]",
			wantInMsg: "at field items[0].metadata.labels[0]",
		},
		{
			name:      "large array index",
			fieldPath: "containers[99].name",
			wantInMsg: "at field containers[99].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, "constraint", "", 0, 0, "", "")
			errorMsg := err.Error()

			if !contains(errorMsg, tt.wantInMsg) {
				t.Errorf("Error() should contain %q, got: %s", tt.wantInMsg, errorMsg)
			}
		})
	}
}

// TestValidationErrorPathFormatting_DeepNestedPaths tests very deep field paths
func TestValidationErrorPathFormatting_DeepNestedPaths(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		wantInMsg string
	}{
		{
			name:      "six-level nested path",
			fieldPath: "spec.template.spec.containers[0].resources.limits.cpu",
			wantInMsg: "at field spec.template.spec.containers[0].resources.limits.cpu",
		},
		{
			name:      "seven-level nested path",
			fieldPath: "spec.template.spec.containers[0].volumeMounts[0].mountPath",
			wantInMsg: "at field spec.template.spec.containers[0].volumeMounts[0].mountPath",
		},
		{
			name:      "very deep configuration path",
			fieldPath: "a.b.c.d.e.f.g.h",
			wantInMsg: "at field a.b.c.d.e.f.g.h",
		},
		{
			name:      "complex kubernetes path",
			fieldPath: "spec.template.spec.containers[0].livenessProbe.httpGet.port",
			wantInMsg: "at field spec.template.spec.containers[0].livenessProbe.httpGet.port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, "constraint", "", 0, 0, "", "")
			errorMsg := err.Error()

			if !contains(errorMsg, tt.wantInMsg) {
				t.Errorf("Error() should contain %q, got: %s", tt.wantInMsg, errorMsg)
			}
		})
	}
}

// TestValidationErrorPathFormatting_EmptyAndMissingPaths tests empty and missing paths
func TestValidationErrorPathFormatting_EmptyAndMissingPaths(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		wantNotInMsg []string // substrings that should NOT be in the error message
		wantInMsg    []string // substrings that SHOULD be in the error message
	}{
		{
			name:         "empty field path",
			fieldPath:    "",
			wantNotInMsg: []string{"at field", "field:"},
			wantInMsg:    []string{"validation error in config.yaml"},
		},
		{
			name:         "whitespace field path is treated as non-empty",
			fieldPath:    "   ",
			wantNotInMsg: []string{}, // whitespace IS included by current implementation
			wantInMsg:    []string{"at field    ", "validation error in"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, "constraint", "", 0, 0, "", "")
			errorMsg := err.Error()

			// Verify that "at field" prefix is not present when path is empty
			for _, notWant := range tt.wantNotInMsg {
				if contains(errorMsg, notWant) {
					t.Errorf("Error() should NOT contain %q when field path is empty, got: %s", notWant, errorMsg)
				}
			}

			// Verify expected substrings are present
			for _, want := range tt.wantInMsg {
				if !contains(errorMsg, want) {
					t.Errorf("Error() should contain %q, got: %s", want, errorMsg)
				}
			}
		})
	}
}

// TestValidationErrorPathFormatting_WithLineAndColumn tests path formatting with line and column
func TestValidationErrorPathFormatting_WithLineAndColumn(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		line      int
		column    int
		wantInMsg []string
	}{
		{
			name:      "simple path with line and column",
			fieldPath: "spec.replicas",
			line:      10,
			column:    5,
			wantInMsg: []string{
				"at line 10, column 5",
				"at field spec.replicas",
			},
		},
		{
			name:      "nested path with line only",
			fieldPath: "server.port",
			line:      15,
			column:    0,
			wantInMsg: []string{
				"at line 15",
				"at field server.port",
			},
		},
		{
			name:      "array path with line and column",
			fieldPath: "containers[0].image",
			line:      22,
			column:    18,
			wantInMsg: []string{
				"at line 22, column 18",
				"at field containers[0].image",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, "constraint", "", tt.line, tt.column, "", "")
			errorMsg := err.Error()

			for _, want := range tt.wantInMsg {
				if !contains(errorMsg, want) {
					t.Errorf("Error() should contain %q, got: %s", want, errorMsg)
				}
			}
		})
	}
}

// TestValidationErrorPathFormatting_ExactFormat verifies exact format of error messages
func TestValidationErrorPathFormatting_ExactFormat(t *testing.T) {
	tests := []struct {
		name         string
		fieldPath    string
		line         int
		column       int
		constraint   string
		message      string
		wantExactMsg string
	}{
		{
			name:         "simple path - exact format",
			fieldPath:    "replicas",
			line:         0,
			column:       0,
			constraint:   "must be positive",
			message:      "invalid value",
			wantExactMsg: "validation error in config.yaml at field replicas: invalid value (constraint: must be positive)",
		},
		{
			name:         "nested path - exact format",
			fieldPath:    "spec.replicas",
			line:         0,
			column:       0,
			constraint:   "must be >= 0",
			message:      "negative value",
			wantExactMsg: "validation error in config.yaml at field spec.replicas: negative value (constraint: must be >= 0)",
		},
		{
			name:         "deep nested path - exact format",
			fieldPath:    "spec.template.spec.containers[0].image",
			line:         0,
			column:       0,
			constraint:   "must match registry/*:tag",
			message:      "invalid tag",
			wantExactMsg: "validation error in config.yaml at field spec.template.spec.containers[0].image: invalid tag (constraint: must match registry/*:tag)",
		},
		{
			name:         "with line and column - exact format",
			fieldPath:    "server.port",
			line:         15,
			column:       12,
			constraint:   "must be between 1-65535",
			message:      "port out of range",
			wantExactMsg: "validation error in config.yaml at line 15, column 12 at field server.port: port out of range (constraint: must be between 1-65535)",
		},
		{
			name:         "empty path - exact format",
			fieldPath:    "",
			line:         0,
			column:       0,
			constraint:   "",
			message:      "validation failed",
			wantExactMsg: "validation error in config.yaml: validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", tt.message, tt.fieldPath, tt.constraint, "", tt.line, tt.column, "", "")
			errorMsg := err.Error()

			if errorMsg != tt.wantExactMsg {
				t.Errorf("Error() = %q, want %q", errorMsg, tt.wantExactMsg)
			}
		})
	}
}

// TestValidationErrorPathFormatting_StringMethod tests String() method output with paths
func TestValidationErrorPathFormatting_StringMethod(t *testing.T) {
	tests := []struct {
		name       string
		fieldPath  string
		line       int
		column     int
		constraint string
		wantFields []string
	}{
		{
			name:      "simple path in String() output",
			fieldPath: "server.port",
			line:      10,
			column:    5,
			constraint: "must be positive",
			wantFields: []string{
				"Error:",
				"Type:",
				"Location: Line 10, Column 5",
				"Field: server.port",
				"Constraint: must be positive",
			},
		},
		{
			name:      "nested path in String() output",
			fieldPath: "spec.template.spec.containers[0].image",
			line:      22,
			column:    18,
			constraint: "must match pattern",
			wantFields: []string{
				"Error:",
				"Type:",
				"Location: Line 22, Column 18",
				"Field: spec.template.spec.containers[0].image",
				"Constraint: must match pattern",
			},
		},
		{
			name:      "empty path in String() output",
			fieldPath: "",
			line:      0,
			column:    0,
			constraint: "",
			wantFields: []string{
				"Error:",
				"Type:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("config.yaml", "test message", tt.fieldPath, tt.constraint, "", tt.line, tt.column, "", "")
			result := err.String()

			for _, field := range tt.wantFields {
				if !contains(result, field) {
					t.Errorf("String() should contain %q, got: %s", field, result)
				}
			}

			// If fieldPath is empty, Field line should not be present
			if tt.fieldPath == "" && contains(result, "Field:") {
				t.Errorf("String() should NOT contain 'Field:' when fieldPath is empty, got: %s", result)
			}
		})
	}
}

// TestValidationErrorPathFormatting_RealWorldExamples tests real-world Kubernetes and config scenarios
func TestValidationErrorPathFormatting_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		scenario string
	}{
		{
			name:      "Kubernetes deployment replicas",
			fieldPath: "spec.replicas",
			scenario: "Deployment replicas validation",
		},
		{
			name:      "Kubernetes container image",
			fieldPath: "spec.template.spec.containers[0].image",
			scenario: "Container image validation",
		},
		{
			name:      "Kubernetes service port",
			fieldPath: "spec.ports[0].targetPort",
			scenario: "Service port configuration",
		},
		{
			name:      "Kubernetes configMap data",
			fieldPath: "data.application.properties",
			scenario: "ConfigMap data validation",
		},
		{
			name:      "Kubernetes volume mount",
			fieldPath: "spec.template.spec.containers[0].volumeMounts[0].mountPath",
			scenario: "Volume mount path validation",
		},
		{
			name:      "Kubernetes probe configuration",
			fieldPath: "spec.template.spec.containers[0].readinessProbe.httpGet.path",
			scenario: "Health check probe path",
		},
		{
			name:      "Application database config",
			fieldPath: "database.connectionPool.maxConnections",
			scenario: "Database connection pool",
		},
		{
			name:      "Application server config",
			fieldPath: "server.api.endpoints[0].timeout",
			scenario: "API endpoint configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError("k8s-deployment.yaml", "invalid value", tt.fieldPath, "constraint", "", 10, 5, "", "")
			errorMsg := err.Error()

			// Verify the field path appears in the error message
			if !contains(errorMsg, tt.fieldPath) {
				t.Errorf("Error() should contain field path %q for scenario %q, got: %s", tt.fieldPath, tt.scenario, errorMsg)
			}

			// Verify "at field" prefix is present
			if !contains(errorMsg, "at field "+tt.fieldPath) && tt.fieldPath != "" {
				t.Errorf("Error() should contain 'at field %s' for scenario %q, got: %s", tt.fieldPath, tt.scenario, errorMsg)
			}
		})
	}
}
