package validate

import (
	"fmt"
)

// ExampleOptionalFields_Demo demonstrates all optional fields working together
func ExampleOptionalFields_Demo() {
	// Full error with all optional fields populated
	fullError := FormatValidationErrorWithDetails(
		"error_message",
		"invalid.*token",
		"access_denied",
		"OAuth token validation",
		`{"error": "access_denied"}`,
		"error",
		"line 42 in user config",
		[]string{"token_type", "access_token"},
		"regex pattern did not match",
		"",
		[]string{"No matching error field"},
		"Custom suggestion 1",
		"Custom suggestion 2",
	)

	fmt.Println("=== Full Error with All Optional Fields ===")
	fmt.Println(fullError.Error())
	fmt.Println()

	// Minimal error - backward compatible
	minimalError := FormatValidationError("test", "expected", "actual", "", "")
	fmt.Println("=== Minimal Error (Backward Compatible) ===")
	fmt.Println(minimalError.Error())

	// Output:
	// === Full Error with All Optional Fields ===
	// error_message validation failed
	//   Expected: invalid.*token
	//   Actual:   access_denied
	//   Context:  OAuth token validation
	//   Response: {"error": "access_denied"}
	//   Field:    error
	//   Location: line 42 in user config
	//   Related fields: [token_type access_token]
	//   Pattern:  regex pattern did not match
	//   Details:
	//     - No matching error field
	//   Suggestions:
	//     - Custom suggestion 1
	//     - Custom suggestion 2
	//
	// === Minimal Error (Backward Compatible) ===
	// test validation failed
	//   Expected: expected
	//   Actual:   actual
	//   Suggestions:
	//     - Review the request parameters and try again
}
