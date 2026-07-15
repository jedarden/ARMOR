package validate

import (
	"fmt"
	"strings"
)

// =============================================================================
// OPTIONAL ERROR FIELD DATA STRUCTURES
// =============================================================================

// ValidationErrorContext provides positional and relational context for validation errors.
// This struct helps identify where in a document or response the validation error occurred
// and which other fields might be related to the issue.
//
// Example usage:
//
//	ctx := ValidationErrorContext{
//	    Location: "field 'user.email' in line 42",
//	    RelatedFields: []string{"email_confirmation", "user.email_format"},
//	}
//
//	if ctx.HasLocation() {
//	    fmt.Printf("Error location: %s", ctx.Location)
//	}
type ValidationErrorContext struct {
	// Location specifies where in the input the error occurred.
	// Examples: "line 5", "field 'user.email'", "position 123", "header 'Authorization'"
	// This field is optional but recommended for complex validation scenarios.
	Location string `json:"location,omitempty"`

	// RelatedFields lists fields related to this error for additional context.
	// Examples: ["email", "email_confirmation"], ["access_token", "refresh_token"]
	// This field is optional and provides additional debugging context.
	RelatedFields []string `json:"related_fields,omitempty"`
}

// HasLocation checks if a location has been specified in the context.
// Returns true if Location is non-empty, false otherwise.
func (vec ValidationErrorContext) HasLocation() bool {
	return vec.Location != ""
}

// HasRelatedFields checks if related fields have been specified in the context.
// Returns true if RelatedFields is non-empty, false otherwise.
func (vec ValidationErrorContext) HasRelatedFields() bool {
	return len(vec.RelatedFields) > 0
}

// IsEmpty checks if the ValidationErrorContext has no information set.
// Returns true if both Location is empty and RelatedFields is empty/nil.
func (vec ValidationErrorContext) IsEmpty() bool {
	return !vec.HasLocation() && !vec.HasRelatedFields()
}

// String returns a string representation of the validation error context.
// This is useful for debugging and logging validation context information.
func (vec ValidationErrorContext) String() string {
	var parts []string

	if vec.HasLocation() {
		parts = append(parts, fmt.Sprintf("location: %s", vec.Location))
	}

	if vec.HasRelatedFields() {
		parts = append(parts, fmt.Sprintf("related_fields: [%s]", strings.Join(vec.RelatedFields, ", ")))
	}

	if len(parts) == 0 {
		return "ValidationErrorContext(empty)"
	}

	return fmt.Sprintf("ValidationErrorContext(%s)", strings.Join(parts, ", "))
}

// Validate checks if the ValidationErrorContext is valid.
// An ValidationErrorContext is always valid, as all fields are optional.
// This method exists for consistency with other validation types.
func (vec ValidationErrorContext) Validate() error {
	// All fields are optional, so a ValidationErrorContext is always valid
	return nil
}

// ExpectedActual represents the comparison between expected and actual values in validation.
// This struct provides a structured way to represent what was expected versus what was
// actually received during validation.
//
// Example usage:
//
//	ea := ExpectedActual{
//	    Expected: 200,
//	    Actual: 404,
//	}
//	if ea.Mismatched() {
//	    fmt.Printf("Expected %d but got %d", ea.Expected, ea.Actual)
//	}
type ExpectedActual struct {
	// Expected contains the value that was expected during validation.
	// Can be of any type: int, string, []int, etc.
	Expected interface{} `json:"expected,omitempty"`

	// Actual contains the value that was actually received during validation.
	// Can be of any type: int, string, []int, etc.
	Actual interface{} `json:"actual,omitempty"`
}

// HasExpected checks if an expected value has been set.
// Returns true if Expected is non-nil, false otherwise.
func (ea ExpectedActual) HasExpected() bool {
	return ea.Expected != nil
}

// HasActual checks if an actual value has been set.
// Returns true if Actual is non-nil, false otherwise.
func (ea ExpectedActual) HasActual() bool {
	return ea.Actual != nil
}

// Mismatched checks if the expected and actual values differ.
// Returns true if both Expected and Actual are set and they are not equal.
// Note: This uses simple equality comparison and may not work correctly for all types.
func (ea ExpectedActual) Mismatched() bool {
	if !ea.HasExpected() || !ea.HasActual() {
		return false
	}

	// Handle specific types appropriately
	switch exp := ea.Expected.(type) {
	case int:
		if act, ok := ea.Actual.(int); ok {
			return exp != act
		}
	case string:
		if act, ok := ea.Actual.(string); ok {
			return exp != act
		}
	case []int:
		if act, ok := ea.Actual.(int); ok {
			// Check if actual is in the expected list
			for _, e := range exp {
				if e == act {
					return false
				}
			}
			return true
		}
	}

	// Default: use simple comparison
	return fmt.Sprintf("%v", ea.Expected) != fmt.Sprintf("%v", ea.Actual)
}

// IsEmpty checks if both Expected and Actual values are unset.
// Returns true if both Expected and Actual are nil, false otherwise.
func (ea ExpectedActual) IsEmpty() bool {
	return !ea.HasExpected() && !ea.HasActual()
}

// String returns a string representation of the expected vs actual comparison.
// This formats the comparison in a human-readable way for debugging and logging.
func (ea ExpectedActual) String() string {
	var parts []string

	if ea.HasExpected() {
		switch exp := ea.Expected.(type) {
		case int:
			parts = append(parts, fmt.Sprintf("expected: %d (%s)", exp, getStatusCodeDescription(exp)))
		case []int:
			expectedStr := "expected: one of ["
			for i, code := range exp {
				if i > 0 {
					expectedStr += ", "
				}
				expectedStr += fmt.Sprintf("%d (%s)", code, getStatusCodeDescription(code))
			}
			expectedStr += "]"
			parts = append(parts, expectedStr)
		case string:
			parts = append(parts, fmt.Sprintf("expected: %s", exp))
		default:
			parts = append(parts, fmt.Sprintf("expected: %v", exp))
		}
	}

	if ea.HasActual() {
		switch act := ea.Actual.(type) {
		case int:
			parts = append(parts, fmt.Sprintf("actual: %d (%s)", act, getStatusCodeDescription(act)))
		case string:
			if len(act) > 50 {
				act = act[:50] + "..."
			}
			parts = append(parts, fmt.Sprintf("actual: %s", act))
		default:
			parts = append(parts, fmt.Sprintf("actual: %v", act))
		}
	}

	if len(parts) == 0 {
		return "ExpectedActual(empty)"
	}

	return fmt.Sprintf("ExpectedActual(%s)", strings.Join(parts, ", "))
}

// Validate checks if the ExpectedActual struct is valid.
// An ExpectedActual is considered valid if at least one of Expected or Actual is set.
// Returns an error if both fields are nil, nil otherwise.
func (ea ExpectedActual) Validate() error {
	if ea.IsEmpty() {
		return fmt.Errorf("ExpectedActual must have at least Expected or Actual set")
	}
	return nil
}

// Suggestion represents a single actionable recommendation for resolving a validation error.
// This struct provides a structured way to present hints and guidance to users about how
// to fix validation issues.
//
// Example usage:
//
//	suggestion := Suggestion{
//	    Message: "Check the API documentation",
//	    Priority: "high",
//	    Category: "documentation",
//	}
//	if suggestion.IsActionable() {
//	    fmt.Println(suggestion.Message)
//	}
type Suggestion struct {
	// Message is the actionable recommendation text.
	// This should be clear, specific, and actionable guidance for the user.
	// This field is required for a meaningful suggestion.
	Message string `json:"message,omitempty"`

	// Priority indicates the importance or urgency of this suggestion.
	// Values: "high", "medium", "low". If empty, defaults to "medium".
	// This field is optional.
	Priority string `json:"priority,omitempty"`

	// Category groups related suggestions together.
	// Examples: "configuration", "authentication", "network", "data_format"
	// This field is optional and useful for organizing multiple suggestions.
	Category string `json:"category,omitempty"`

	// Actionable indicates whether this suggestion can be directly acted upon.
	// If true, the suggestion describes a concrete action the user can take.
	// If false, the suggestion may be informational only.
	// This field is optional and defaults to true.
	Actionable bool `json:"actionable,omitempty"`
}

// IsActionable checks if the suggestion is marked as actionable.
// Returns the Actionable field value, or true if Actionable is not set.
func (s Suggestion) IsActionable() bool {
	return s.Actionable || s.Actionable == false
}

// HasPriority checks if a priority has been set for the suggestion.
// Returns true if Priority is non-empty, false otherwise.
func (s Suggestion) HasPriority() bool {
	return s.Priority != ""
}

// HasCategory checks if a category has been set for the suggestion.
// Returns true if Category is non-empty, false otherwise.
func (s Suggestion) HasCategory() bool {
	return s.Category != ""
}

// IsEmpty checks if the suggestion has no message set.
// Returns true if Message is empty, false otherwise.
func (s Suggestion) IsEmpty() bool {
	return s.Message == ""
}

// String returns a string representation of the suggestion.
// This formats the suggestion in a human-readable way for debugging and logging.
func (s Suggestion) String() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("message: %s", s.Message))

	if s.HasPriority() {
		parts = append(parts, fmt.Sprintf("priority: %s", s.Priority))
	}

	if s.HasCategory() {
		parts = append(parts, fmt.Sprintf("category: %s", s.Category))
	}

	if !s.Actionable {
		parts = append(parts, "informational")
	}

	return fmt.Sprintf("Suggestion(%s)", strings.Join(parts, ", "))
}

// Validate checks if the Suggestion is valid.
// A Suggestion is valid if it has a non-empty Message field.
// Returns an error if Message is empty, nil otherwise.
func (s Suggestion) Validate() error {
	if s.Message == "" {
		return fmt.Errorf("Suggestion must have a Message field set")
	}
	return nil
}

// GetPriority returns the priority level of the suggestion.
// If Priority is not set, returns "medium" as the default.
func (s Suggestion) GetPriority() string {
	if s.Priority == "" {
		return "medium"
	}
	return s.Priority
}

// =============================================================================
// CONSTRUCTOR FUNCTIONS
// =============================================================================

// NewValidationErrorContext creates a new ValidationErrorContext with the specified location.
// RelatedFields can be added using the WithRelatedFields method or by setting directly.
//
// Example usage:
//
//	ctx := NewValidationErrorContext("field 'user.email' in line 42")
func NewValidationErrorContext(location string) ValidationErrorContext {
	return ValidationErrorContext{
		Location: location,
	}
}

// NewExpectedActual creates a new ExpectedActual struct with the specified expected and actual values.
//
// Example usage:
//
//	ea := NewExpectedActual(200, 404)
func NewExpectedActual(expected, actual interface{}) ExpectedActual {
	return ExpectedActual{
		Expected: expected,
		Actual:   actual,
	}
}

// NewSuggestion creates a new Suggestion with the specified message.
// Priority defaults to "medium" and Actionable defaults to true.
//
// Example usage:
//
//	suggestion := NewSuggestion("Check the API documentation")
func NewSuggestion(message string) Suggestion {
	return Suggestion{
		Message:    message,
		Priority:   "medium",
		Actionable: true,
	}
}

// =============================================================================
// BUILDER METHODS FOR ValidationErrorContext
// =============================================================================

// WithRelatedFields adds related fields to the ValidationErrorContext.
// Returns a new ValidationErrorContext with the specified related fields.
//
// Example usage:
//
//	ctx := NewValidationErrorContext("field 'user.email'").
//	    WithRelatedFields([]string{"email_confirmation", "user.email_format"})
func (vec ValidationErrorContext) WithRelatedFields(fields []string) ValidationErrorContext {
	vec.RelatedFields = fields
	return vec
}

// =============================================================================
// BUILDER METHODS FOR ExpectedActual
// =============================================================================

// WithExpected sets the expected value in the ExpectedActual struct.
// Returns a new ExpectedActual with the specified expected value.
//
// Example usage:
//
//	ea := NewExpectedActual(nil, 404).WithExpected(200)
func (ea ExpectedActual) WithExpected(expected interface{}) ExpectedActual {
	ea.Expected = expected
	return ea
}

// WithActual sets the actual value in the ExpectedActual struct.
// Returns a new ExpectedActual with the specified actual value.
//
// Example usage:
//
//	ea := NewExpectedActual(200, nil).WithActual(404)
func (ea ExpectedActual) WithActual(actual interface{}) ExpectedActual {
	ea.Actual = actual
	return ea
}

// =============================================================================
// BUILDER METHODS FOR Suggestion
// =============================================================================

// WithPriority sets the priority level for the suggestion.
// Returns a new Suggestion with the specified priority.
//
// Example usage:
//
//	suggestion := NewSuggestion("Check the API documentation").
//	    WithPriority("high")
func (s Suggestion) WithPriority(priority string) Suggestion {
	s.Priority = priority
	return s
}

// WithCategory sets the category for the suggestion.
// Returns a new Suggestion with the specified category.
//
// Example usage:
//
//	suggestion := NewSuggestion("Check the API documentation").
//	    WithCategory("documentation")
func (s Suggestion) WithCategory(category string) Suggestion {
	s.Category = category
	return s
}

// WithActionable sets whether the suggestion is actionable.
// Returns a new Suggestion with the specified actionable flag.
//
// Example usage:
//
//	suggestion := NewSuggestion("See documentation for details").
//	    WithActionable(false)
func (s Suggestion) WithActionable(actionable bool) Suggestion {
	s.Actionable = actionable
	return s
}

// =============================================================================
// CONVERSION HELPERS
// =============================================================================

// ToSuggestions converts a slice of strings to a slice of Suggestion structs.
// Each string becomes a Suggestion with default priority and actionable settings.
//
// Example usage:
//
//	suggestions := ToSuggestions([]string{"Check API docs", "Verify URL"})
func ToSuggestions(messages []string) []Suggestion {
	if len(messages) == 0 {
		return nil
	}

	suggestions := make([]Suggestion, len(messages))
	for i, msg := range messages {
		suggestions[i] = NewSuggestion(msg)
	}
	return suggestions
}

// ToMessages converts a slice of Suggestion structs to a slice of message strings.
// This is useful for compatibility with existing code that expects string slices.
//
// Example usage:
//
//	messages := ToMessages([]Suggestion{suggestion1, suggestion2})
func ToMessages(suggestions []Suggestion) []string {
	if len(suggestions) == 0 {
		return nil
	}

	messages := make([]string, len(suggestions))
	for i, s := range suggestions {
		messages[i] = s.Message
	}
	return messages
}

// FilterSuggestionsByCategory filters suggestions by category.
// Returns a new slice containing only suggestions with the specified category.
//
// Example usage:
//
//	authSuggestions := FilterSuggestionsByCategory(suggestions, "authentication")
func FilterSuggestionsByCategory(suggestions []Suggestion, category string) []Suggestion {
	if len(suggestions) == 0 {
		return nil
	}

	var filtered []Suggestion
	for _, s := range suggestions {
		if s.Category == category {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

// FilterSuggestionsByPriority filters suggestions by priority level.
// Returns a new slice containing only suggestions with the specified priority.
//
// Example usage:
//
//	highPrioritySuggestions := FilterSuggestionsByPriority(suggestions, "high")
func FilterSuggestionsByPriority(suggestions []Suggestion, priority string) []Suggestion {
	if len(suggestions) == 0 {
		return nil
	}

	var filtered []Suggestion
	for _, s := range suggestions {
		if s.GetPriority() == priority {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}
