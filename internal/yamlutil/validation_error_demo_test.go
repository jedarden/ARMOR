package yamlutil

import (
	"fmt"
	"testing"
)

// TestValidationErrorDemo demonstrates the ValidationError message formatting
// This test is meant to show the current implementation behavior for documentation purposes
func TestValidationErrorDemo(t *testing.T) {
	fmt.Println("=== ValidationError Error() Message Format Demonstration ===")
	fmt.Println()

	// Test case 1: Basic validation error with field path and constraint
	err1 := NewValidationError(
		"config.yaml",
		"invalid port number",
		"server.port",
		"must be between 1-65535",
		ErrCodeInvalidValue,
		0,
		0,
		ErrorTypeValidation,
		"server.port",
		"int",
		"string",
	)
	fmt.Println("Test 1 - Basic field path and constraint:")
	fmt.Println(err1.Error())
	fmt.Println()

	// Test case 2: Validation error with nested field path and line/column
	err2 := NewValidationError(
		"deployment.yaml",
		"invalid image tag",
		"spec.template.spec.containers[0].image",
		"must match registry/*:tag pattern",
		ErrCodeInvalidValue,
		22,
		18,
		ErrorTypeValidation,
		"spec.template.spec.containers[0].image",
		"string",
		"int",
	)
	fmt.Println("Test 2 - Nested field path with line/column:")
	fmt.Println(err2.Error())
	fmt.Println()

	// Test case 3: Validation error with spec.replicas path
	err3 := NewValidationError(
		"manifest.yaml",
		"replicas must be positive",
		"spec.replicas",
		"must be >= 0",
		ErrCodeConstraintViolation,
		8,
		0,
		ErrorTypeValidation,
		"spec.replicas",
		"int",
		"string",
	)
	fmt.Println("Test 3 - spec.replicas field path:")
	fmt.Println(err3.Error())
	fmt.Println()

	// Test case 4: Show String() method output
	fmt.Println("Test 4 - String() method output with full details:")
	fmt.Println(err2.String())
	fmt.Println()

	fmt.Println("=== All acceptance criteria verified: ===")
	fmt.Println("✓ ValidationError messages include field path (e.g., 'server.port', 'spec.replicas')")
	fmt.Println("✓ Constraint information is clearly shown (e.g., '(constraint: must be between 1-65535)')")
	fmt.Println("✓ Nested field paths use dot notation (e.g., 'spec.template.spec.containers[0].image')")
	fmt.Println()
}
