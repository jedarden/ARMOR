package yamlutil

import (
	"testing"
)

// TestAcceptanceCriteria verifies the bead bf-1sjr7 acceptance criteria.
func TestAcceptanceCriteria(t *testing.T) {
	validator := NewSyntaxValidator()

	tests := []struct {
		name             string
		content          string
		shouldFindErrors bool
		expectedCount    int
		checkLineNumbers bool
		checkKeyNames    bool
	}{
		{
			name:             "AC1: Single key missing colon",
			content:          `key value`,
			shouldFindErrors: true,
			expectedCount:    1,
			checkLineNumbers: true,
			checkKeyNames:    true,
		},
		{
			name:             "AC2: Multiple keys missing colons",
			content:          `first item
second thing
third: value`,
			shouldFindErrors: true,
			expectedCount:    2,
			checkLineNumbers: true,
			checkKeyNames:    true,
		},
		{
			name:             "AC3: Nested mapping with missing colon",
			content:          `parent:
  child value
  other: item`,
			shouldFindErrors: true,
			expectedCount:    1,
			checkLineNumbers: true,
			checkKeyNames:    true,
		},
		{
			name:             "AC4: Valid YAML produces no false positives",
			content:           `key: value
another: item
parent:
  child: value`,
			shouldFindErrors: false,
			expectedCount:    0,
		},
		{
			name:             "AC5: Common YAML mapping patterns work",
			content:           `name: John
age: 30
address:
  street: Main St
  city: Boston`,
			shouldFindErrors: false,
			expectedCount:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.DetectMissingColonsUsingLineParser(tt.content)

			if tt.shouldFindErrors {
				if len(errors) != tt.expectedCount {
					t.Errorf("Expected %d errors, got %d", tt.expectedCount, len(errors))
					for i, err := range errors {
						t.Logf("  Error %d: Line %d - %s", i+1, err.Line, err.Message)
					}
				}

				if tt.checkLineNumbers {
					for _, err := range errors {
						if err.Line == 0 {
							t.Errorf("Expected line number to be set for error: %s", err.Message)
						}
					}
				}

				if tt.checkKeyNames {
					for _, err := range errors {
						if !containsSubstring(err.Message, "Missing") {
							t.Errorf("Expected error message to mention 'Missing', got: %s", err.Message)
						}
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no errors for valid YAML, got %d", len(errors))
					for i, err := range errors {
						t.Logf("  Unexpected error %d: Line %d - %s", i+1, err.Line, err.Message)
					}
				}
			}
		})
	}
}
