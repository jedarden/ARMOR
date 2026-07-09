// Package yamlutil tests for Status enum
package yamlutil

import (
	"testing"
)

func TestStatusValues(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "StatusSuccess value",
			status:   StatusSuccess,
			expected: "success",
		},
		{
			name:     "StatusError value",
			status:   StatusError,
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Status value = %q, expected %q", tt.status, tt.expected)
			}
		})
	}
}

func TestStatusEquality(t *testing.T) {
	// Test that status constants can be compared
	if StatusSuccess != StatusSuccess {
		t.Error("StatusSuccess should equal itself")
	}
	if StatusError != StatusError {
		t.Error("StatusError should equal itself")
	}
	if StatusSuccess == StatusError {
		t.Error("StatusSuccess should not equal StatusError")
	}

	// Test that status variables can be compared
	var s1, s2 Status = StatusSuccess, StatusSuccess
	if s1 != s2 {
		t.Error("Identical Status variables should be equal")
	}

	s2 = StatusError
	if s1 == s2 {
		t.Error("Different Status variables should not be equal")
	}
}

func TestStatusInFunction(t *testing.T) {
	// Test that Status can be used as function parameter and return type
	result := getStatus(StatusSuccess)
	if result != StatusSuccess {
		t.Errorf("getStatus(StatusSuccess) = %v, expected StatusSuccess", result)
	}

	result = getStatus(StatusError)
	if result != StatusError {
		t.Errorf("getStatus(StatusError) = %v, expected StatusError", result)
	}
}

func getStatus(s Status) Status {
	return s
}

func TestStatusInStruct(t *testing.T) {
	// Test that Status can be used in struct fields
	type ParseResult struct {
		Status Status
		Value  interface{}
	}

	result := ParseResult{
		Status: StatusSuccess,
		Value:  "test",
	}

	if result.Status != StatusSuccess {
		t.Errorf("Struct Status field = %v, expected StatusSuccess", result.Status)
	}

	result.Status = StatusError
	if result.Status != StatusError {
		t.Errorf("Updated struct Status field = %v, expected StatusError", result.Status)
	}
}

func TestStatusStringConversion(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "StatusSuccess string",
			status:   StatusSuccess,
			expected: "success",
		},
		{
			name:     "StatusError string",
			status:   StatusError,
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.status)
			if got != tt.expected {
				t.Errorf("string(Status) = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestStatusType(t *testing.T) {
	// Verify Status can be assigned from string constant
	var s Status = StatusSuccess
	if s != StatusSuccess {
		t.Error("Status assignment failed")
	}

	// Verify Status can be used in switch statements
	var result string
	switch s {
	case StatusSuccess:
		result = "got_success"
	case StatusError:
		result = "got_error"
	default:
		result = "got_unknown"
	}

	if result != "got_success" {
		t.Errorf("Status switch case failed: got %q", result)
	}

	// Test with error status
	s = StatusError
	switch s {
	case StatusSuccess:
		result = "got_success"
	case StatusError:
		result = "got_error"
	default:
		result = "got_unknown"
	}

	if result != "got_error" {
		t.Errorf("Status switch case failed: got %q", result)
	}
}
