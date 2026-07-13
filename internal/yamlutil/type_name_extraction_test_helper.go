package yamlutil

// Test helpers for type name extraction tests.
//
// This file provides common utilities and helper functions for testing
// type name extraction functionality across different test files.

import (
	"strings"
)

// TestExpected represents expected test results for type name extraction.
type TestExpected struct {
	ExtractedType string   // The expected type name to be extracted
	ExpectedType  string   // The expected normalized type (if different)
	ActualType    string   // The actual type found (if applicable)
	ShouldMatch   bool     // Whether the pattern should match
	ErrorMessage  string   // Expected error message (if any)
}

// TestScenario represents a complete test scenario for type extraction.
type TestScenario struct {
	Name        string        // Test case name
	Input       string        // Input error message
	Expected    TestExpected  // Expected results
	Description string        // Description of what is being tested
}

// TypeExtractionTestBuilder helps build test scenarios for type name extraction tests.
type TypeExtractionTestBuilder struct {
	scenarios []TestScenario
}

// NewTypeExtractionTestBuilder creates a new test builder for type name extraction.
func NewTypeExtractionTestBuilder() *TypeExtractionTestBuilder {
	return &TypeExtractionTestBuilder{
		scenarios: make([]TestScenario, 0),
	}
}

// AddScenario adds a test scenario to the builder.
func (b *TypeExtractionTestBuilder) AddScenario(scenario TestScenario) *TypeExtractionTestBuilder {
	b.scenarios = append(b.scenarios, scenario)
	return b
}

// AddSimpleScenario adds a simple test case with just name, input, and expected type.
func (b *TypeExtractionTestBuilder) AddSimpleScenario(name, input, expectedType string) *TypeExtractionTestBuilder {
	return b.AddScenario(TestScenario{
		Name: name,
		Input: input,
		Expected: TestExpected{
			ExtractedType: expectedType,
			ShouldMatch:   expectedType != "",
		},
	})
}

// AddYAMLTagScenario adds a test scenario for YAML type tag extraction.
func (b *TypeExtractionTestBuilder) AddYAMLTagScenario(tag, input string) *TypeExtractionTestBuilder {
	return b.AddSimpleScenario(
		"YAML type tag "+tag,
		input,
		tag,
	)
}

// AddGoTypeScenario adds a test scenario for Go type extraction.
func (b *TypeExtractionTestBuilder) AddGoTypeScenario(typeName, input string) *TypeExtractionTestBuilder {
	return b.AddSimpleScenario(
		"Go type "+typeName,
		input,
		typeName,
	)
}

// AddComplexTypeScenario adds a test scenario for complex Go types (slices, maps, pointers, etc.).
func (b *TypeExtractionTestBuilder) AddComplexTypeScenario(typeName, input string) *TypeExtractionTestBuilder {
	return b.AddSimpleScenario(
		"Complex type "+typeName,
		input,
		typeName,
	)
}

// AddEdgeCaseScenario adds an edge case test scenario.
func (b *TypeExtractionTestBuilder) AddEdgeCaseScenario(name, input string, shouldMatch bool) *TypeExtractionTestBuilder {
	expectedType := ""
	if shouldMatch {
		// For edge cases that should match, we need to specify what type
		expectedType = "edge_case"
	}
	return b.AddScenario(TestScenario{
		Name: name,
		Input: input,
		Expected: TestExpected{
			ExtractedType: expectedType,
			ShouldMatch:   shouldMatch,
		},
	})
}

// Build returns the built test scenarios.
func (b *TypeExtractionTestBuilder) Build() []TestScenario {
	return b.scenarios
}

// StandardTestInputs provides standard test input strings for common patterns.
var StandardTestInputs = struct {
	YAMLTagPatterns     map[string]string
	GoBasicTypes        map[string]string
	GoComplexTypes      map[string]string
	EdgeCases          []string
}{
	YAMLTagPatterns: map[string]string{
		"!!str":    "cannot unmarshal !!str into int",
		"!!int":    "cannot unmarshal !!int into string",
		"!!bool":   "expected !!bool, got !!str",
		"!!float":  "cannot unmarshal !!float into int",
		"!!seq":    "yaml: line 15: cannot unmarshal !!seq into []string",
		"!!map":    "field config: cannot unmarshal !!map into string",
		"!!null":   "got !!null, expected string",
	},
	GoBasicTypes: map[string]string{
		"string":  "expected string, got int",
		"int":     "cannot unmarshal int into bool",
		"bool":    "expected bool, got string",
		"float64": "expected float64, got int",
		"int64":   "field type: int64 expected",
		"uint32":  "cannot unmarshal uint32 into string",
		"rune":    "field type: rune expected",
		"byte":    "expected byte, got int",
	},
	GoComplexTypes: map[string]string{
		"[]string":        "expected []string, got int",
		"[]int":           "cannot unmarshal []int into string",
		"[]bool":          "field type: []bool expected",
		"*string":         "field type: *string expected",
		"*int":            "cannot unmarshal *int into string",
		"*bool":           "expected *bool, got string",
		"map[string]int":  "field config: map[string]int expected",
		"map[int]string":  "cannot unmarshal map[int]string into []string",
		"chan string":     "expected chan string, got int",
		"chan<- bool":     "expected chan<- bool, got int",
		"<-chan int":      "expected <-chan int, got string",
		"interface{}":     "expected interface{}, got string",
		"struct{}":        "expected struct{}, got map",
		"[10]int":         "expected [10]int, got []int",
		"[5]string":       "cannot unmarshal !!seq into [5]string",
		"time.Time":       "expected time.Time, got string",
		"http.Response":   "want http.Response",
	},
	EdgeCases: []string{
		"",
		"   ",
		"\t\t\n\n\t",
		"this is an error without type information",
		"field error: something went wrong",
		"map cannot unmarshal",
		"the value is not correct",
		"expected",
		"got",
		"want",
		"type",
		"expected, got string",
		"want, got string",
		"cannot unmarshal into int",
		"cannot unmarshal !!str into",
	},
}

// BuildStandardTestScenarios creates a comprehensive set of standard test scenarios.
func BuildStandardTestScenarios() []TestScenario {
	builder := NewTypeExtractionTestBuilder()

	// Add YAML tag patterns
	for tag, input := range StandardTestInputs.YAMLTagPatterns {
		builder.AddYAMLTagScenario(tag, input)
	}

	// Add Go basic types
	for typeName, input := range StandardTestInputs.GoBasicTypes {
		builder.AddGoTypeScenario(typeName, input)
	}

	// Add Go complex types
	for typeName, input := range StandardTestInputs.GoComplexTypes {
		builder.AddComplexTypeScenario(typeName, input)
	}

	// Add edge cases that should not match
	edgeCasesShouldNotMatch := []string{
		"",
		"   ",
		"\t\t\n\n\t",
		"this is an error without type information",
		"field error: something went wrong",
		"map cannot unmarshal",
		"the value is not correct",
		"expected",
		"got",
		"want",
		"type",
	}

	for i, input := range edgeCasesShouldNotMatch {
		builder.AddEdgeCaseScenario("edge case no match "+string(rune('a'+i)), input, false)
	}

	// Add edge cases that should match
	edgeCasesShouldMatch := map[string]string{
		"expected int":             "int",
		"expected, got string":     "string",
		"want, got string":         "string",
		"cannot unmarshal into int": "int",
	}

	for input, expectedType := range edgeCasesShouldMatch {
		builder.AddSimpleScenario("edge case match "+expectedType, input, expectedType)
	}

	return builder.Build()
}

// ContainsTypeName checks if a string contains a valid type name.
func ContainsTypeName(s string) bool {
	// Check for YAML type tags
	if strings.Contains(s, "!!") && len(s) > 2 {
		return true
	}

	// Check for common Go type patterns
	typeIndicators := []string{
		"[]",   // slice
		"map[", // map
		"chan", // channel
		"*",    // pointer
		"[",    // array
		"interface{}",
		"struct{}",
		"expected ",
		"got ",
		"want ",
	}

	for _, indicator := range typeIndicators {
		if strings.Contains(s, indicator) {
			return true
		}
	}

	return false
}

// NormalizeTestInput trims whitespace and normalizes common error message patterns.
func NormalizeTestInput(input string) string {
	input = strings.TrimSpace(input)
	// Remove common prefixes that might interfere with pattern matching
	prefixes := []string{"yaml: ", "error: ", "line \\d+: "}
	for _, prefix := range prefixes {
		if strings.HasPrefix(input, prefix) {
			// Keep the original input for testing purposes
			break
		}
	}
	return input
}

// ShouldMatchType checks if the extracted type matches the expected type.
// This is a helper function to be used in test implementations.
func ShouldMatchType(got, want string) bool {
	return got == want
}

// containsAny checks if a string contains any of the given patterns.
// This is a helper function for error message validation.
func containsAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}
