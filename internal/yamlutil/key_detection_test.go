// Package yamlutil tests the basic key detection functions.
package yamlutil

import (
	"testing"
)

// TestIsMappingKey verifies the IsMappingKey function.
func TestIsMappingKey(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "simple key-value pair",
			line:     "name: John",
			expected: true,
		},
		{
			name:     "key with spaces",
			line:     "first name: John",
			expected: true,
		},
		{
			name:     "line without colon",
			line:     "just some text",
			expected: false,
		},
		{
			name:     "empty string",
			line:     "",
			expected: false,
		},
		{
			name:     "colon at start",
			line:     ": value",
			expected: true,
		},
		{
			name:     "colon only",
			line:     ":",
			expected: true,
		},
		{
			name:     "multiple colons",
			line:     "key: value: with: colons",
			expected: true,
		},
		{
			name:     "colon in middle of word",
			line:     "key:value",
			expected: true,
		},
		{
			name:     "indented key with colon",
			line:     "  name: John",
			expected: true,
		},
		{
			name:     "tab indented key with colon",
			line:     "\tname: John",
			expected: true,
		},
		{
			name:     "quoted key with colon",
			line:     `"name": John`,
			expected: true,
		},
		{
			name:     "single quoted key with colon",
			line:     `'name': John`,
			expected: true,
		},
		{
			name:     "comment line",
			line:     "# This is a comment",
			expected: false,
		},
		{
			name:     "comment with colon",
			line:     "# TODO: fix this",
			expected: true,
		},
		{
			name:     "sequence item",
			line:     "- item",
			expected: false,
		},
		{
			name:     "sequence item with colon",
			line:     "- name: value",
			expected: true,
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMappingKey(tt.line)
			if result != tt.expected {
				t.Errorf("IsMappingKey(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestExtractKey verifies the ExtractKey function.
func TestExtractKey(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "simple key-value pair",
			line:     "name: John",
			expected: "name",
		},
		{
			name:     "key with spaces",
			line:     "first name: John",
			expected: "first name",
		},
		{
			name:     "line without colon",
			line:     "just some text",
			expected: "",
		},
		{
			name:     "empty string",
			line:     "",
			expected: "",
		},
		{
			name:     "colon at start",
			line:     ": value",
			expected: "",
		},
		{
			name:     "colon only",
			line:     ":",
			expected: "",
		},
		{
			name:     "key with trailing spaces",
			line:     "  key  : value",
			expected: "  key  ",
		},
		{
			name:     "key with leading spaces",
			line:     "  key: value",
			expected: "  key",
		},
		{
			name:     "indented key with colon",
			line:     "    name: John",
			expected: "    name",
		},
		{
			name:     "tab indented key with colon",
			line:     "\tname: John",
			expected: "\tname",
		},
		{
			name:     "quoted key",
			line:     `"quoted key": value`,
			expected: `"quoted key"`,
		},
		{
			name:     "single quoted key",
			line:     `'quoted key': value`,
			expected: `'quoted key'`,
		},
		{
			name:     "multiple colons returns first key",
			line:     "key1: key2: value",
			expected: "key1",
		},
		{
			name:     "key with hyphens",
			line:     "my-key-name: value",
			expected: "my-key-name",
		},
		{
			name:     "key with underscores",
			line:     "my_key_name: value",
			expected: "my_key_name",
		},
		{
			name:     "key with dots",
			line:     "my.key.name: value",
			expected: "my.key.name",
		},
		{
			name:     "key with numbers",
			line:     "key123: value",
			expected: "key123",
		},
		{
			name:     "numeric key",
			line:     "123: value",
			expected: "123",
		},
		{
			name:     "sequence item with colon",
			line:     "- name: value",
			expected: "- name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractKey(tt.line)
			if result != tt.expected {
				t.Errorf("ExtractKey(%q) = %q, want %q", tt.line, result, tt.expected)
			}
		})
	}
}

// TestKeyDetectionIntegration tests IsMappingKey and ExtractKey together.
func TestKeyDetectionIntegration(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		isKey          bool
		extractedKey   string
	}{
		{
			name:         "valid mapping key",
			line:         "name: John",
			isKey:        true,
			extractedKey: "name",
		},
		{
			name:         "key with spaces",
			line:         "first name: John",
			isKey:        true,
			extractedKey: "first name",
		},
		{
			name:         "not a key - no colon",
			line:         "just some text",
			isKey:        false,
			extractedKey: "",
		},
		{
			name:         "empty string",
			line:         "",
			isKey:        false,
			extractedKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsMappingKey
			isKey := IsMappingKey(tt.line)
			if isKey != tt.isKey {
				t.Errorf("IsMappingKey(%q) = %v, want %v", tt.line, isKey, tt.isKey)
			}

			// Test ExtractKey
			extractedKey := ExtractKey(tt.line)
			if extractedKey != tt.extractedKey {
				t.Errorf("ExtractKey(%q) = %q, want %q", tt.line, extractedKey, tt.extractedKey)
			}

			// Verify consistency: if IsMappingKey returns true, ExtractKey should return non-empty
			if isKey && extractedKey == "" {
				t.Errorf("Inconsistency: IsMappingKey(%q) = true but ExtractKey returned empty string", tt.line)
			}
		})
	}
}

// BenchmarkIsMappingKey benchmarks the IsMappingKey function.
func BenchmarkIsMappingKey(b *testing.B) {
	line := "name: John"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsMappingKey(line)
	}
}

// BenchmarkExtractKey benchmarks the ExtractKey function.
func BenchmarkExtractKey(b *testing.B) {
	line := "name: John"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractKey(line)
	}
}
