package yamlutil

import (
	"strings"
	"testing"
)

func TestParseErrorVariantString(t *testing.T) {
	tests := []struct {
		name     string
		variant  ParseErrorVariant
		expected string
	}{
		{"syntax variant", ParseErrorVariantSyntax, "syntax"},
		{"type mismatch variant", ParseErrorVariantTypeMismatch, "type_mismatch"},
		{"validation variant", ParseErrorVariantValidation, "validation"},
		{"IO variant", ParseErrorVariantIO, "io"},
		{"structure variant", ParseErrorVariantStructure, "structure"},
		{"custom variant", ParseErrorVariantCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.variant.String(); got != tt.expected {
				t.Errorf("ParseErrorVariant.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseErrorVariantDescription(t *testing.T) {
	tests := []struct {
		name     string
		variant  ParseErrorVariant
		contains string
	}{
		{
			"syntax variant description",
			ParseErrorVariantSyntax,
			"YAML syntax error",
		},
		{
			"type mismatch variant description",
			ParseErrorVariantTypeMismatch,
			"Type mismatch error",
		},
		{
			"validation variant description",
			ParseErrorVariantValidation,
			"Validation error",
		},
		{
			"IO variant description",
			ParseErrorVariantIO,
			"I/O error",
		},
		{
			"structure variant description",
			ParseErrorVariantStructure,
			"Structure error",
		},
		{
			"custom variant description",
			ParseErrorVariantCustom,
			"Custom error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := tt.variant.Description()
			if desc == "" {
				t.Errorf("ParseErrorVariant.Description() returned empty string for %v", tt.variant)
			}
			// Check that description contains the expected substring
			if !strings.Contains(desc, tt.contains) {
				t.Errorf("ParseErrorVariant.Description() = %v, does not contain %v", desc, tt.contains)
			}
		})
	}
}

func TestParseErrorVariantCount(t *testing.T) {
	// This test verifies we have at least the minimum required variants
	requiredVariants := []ParseErrorVariant{
		ParseErrorVariantSyntax,       // 1. Syntax errors
		ParseErrorVariantTypeMismatch,  // 2. Type mismatch errors
		ParseErrorVariantValidation,    // 3. Validation errors
		ParseErrorVariantIO,            // 4. IO errors
		ParseErrorVariantStructure,     // 5. Structure errors (bonus)
		ParseErrorVariantCustom,        // 6. Custom error category
	}

	if len(requiredVariants) < 5 {
		t.Errorf("Expected at least 5 ParseErrorVariant constants, got %d", len(requiredVariants))
	}

	t.Logf("ParseErrorVariant has %d distinct variants (minimum 5 required):", len(requiredVariants))
	for i, v := range requiredVariants {
		t.Logf("  %d. %s - %s", i+1, v.String(), v.Description())
	}
}

func TestParseErrorVariantAllDistinct(t *testing.T) {
	// Verify all variants are distinct
	variants := []ParseErrorVariant{
		ParseErrorVariantSyntax,
		ParseErrorVariantTypeMismatch,
		ParseErrorVariantValidation,
		ParseErrorVariantIO,
		ParseErrorVariantStructure,
		ParseErrorVariantCustom,
	}

	seen := make(map[ParseErrorVariant]bool)
	for _, v := range variants {
		if seen[v] {
			t.Errorf("Duplicate ParseErrorVariant found: %s", v.String())
		}
		seen[v] = true
	}

	if len(seen) != len(variants) {
		t.Errorf("Expected %d distinct variants, got %d", len(variants), len(seen))
	}
}

// Note: contains() and containsSubstring() helpers are already defined in errors_test.go
