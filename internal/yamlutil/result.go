// Package yamlutil provides Rust-style Result type for error handling.
//
// The Result[T, E] type represents either success (Ok) containing a value of type T,
// or failure (Err) containing an error of type E.
//
// This pattern enables explicit error handling without exceptions, making error
// paths visible in the type system and forcing proper error handling.
package yamlutil

import "fmt"

// Result represents the outcome of an operation that can either succeed with a value
// or fail with an error.
//
// Result is modeled after Rust's Result<T, E> type and provides a type-safe way to
// handle operations that can fail without using exceptions.
//
// Type parameters:
//   - T: The success value type
//   - E: The error type (typically ParseError, ValidationError, or any YAMLError)
//
// Example usage:
//
//	// A function that returns Result[Config, ParseError]
//	func ParseConfig(path string) Result[Config, ParseError] {
//	    data, err := os.ReadFile(path)
//	    if err != nil {
//	        return Err[Config](ParseError{...})
//	    }
//	    config := parseConfig(data)
//	    return Ok[Config](config)
//	}
//
//	// Using the result
//	result := ParseConfig("config.yaml")
//	if result.IsOk() {
//	    config := result.Unwrap()
//	    // Use config
//	} else {
//	    err := result.UnwrapErr()
//	    // Handle error
//	}
type Result[T any, E any] struct {
	// ok is true when the result is successful
	ok bool

	// value contains the success value when ok is true
	value T

	// err contains the error when ok is false
	err E
}

// Ok creates a successful Result containing the provided value.
//
// Example:
//	result := Ok(42)
//	result := Ok("success")
//	result := Ok(myStruct{})
func Ok[T any, E any](value T) Result[T, E] {
	return Result[T, E]{
		ok:    true,
		value: value,
	}
}

// Err creates a failed Result containing the provided error.
//
// Example:
//	result := Err[Config](ParseError{FilePath: "config.yaml", Line: 5, Message: "syntax error"})
//	result := Err[Data](ValidationError{Message: "required field missing"})
func Err[T any, E any](err E) Result[T, E] {
	return Result[T, E]{
		ok:  false,
		err: err,
	}
}

// IsOk returns true if the Result is successful (contains a value).
func (r Result[T, E]) IsOk() bool {
	return r.ok
}

// IsErr returns true if the Result is failed (contains an error).
func (r Result[T, E]) IsErr() bool {
	return !r.ok
}

// Unwrap returns the contained success value.
//
// Panics if the Result is an error. Use IsOk() to check before calling.
//
// This is typically used when you've already verified the Result is Ok:
//
//	if result.IsOk() {
//	    value := result.Unwrap()
//	    // Use value
//	}
func (r Result[T, E]) Unwrap() T {
	if r.ok {
		return r.value
	}
	// Panic with a helpful message
	panic("unwrap called on Err result: " + stringifyError(r.err))
}

// UnwrapErr returns the contained error.
//
// Panics if the Result is Ok. Use IsErr() to check before calling.
//
// This is typically used when you've already verified the Result is an error:
//
//	if result.IsErr() {
//	    err := result.UnwrapErr()
//	    // Handle error
//	}
func (r Result[T, E]) UnwrapErr() E {
	if !r.ok {
		return r.err
	}
	panic("unwrap_err called on Ok result")
}

// UnwrapOrDefault returns the contained success value, or the zero value for T if Err.
//
// This is useful when you want to provide a default value instead of panicking.
func (r Result[T, E]) UnwrapOrDefault() T {
	if r.ok {
		return r.value
	}
	var zero T
	return zero
}

// UnwrapOr returns the contained success value, or the provided default value if Err.
//
// Example:
//	result := ParseOptionalField(data)
//	value := result.UnwrapOr("default")
func (r Result[T, E]) UnwrapOr(defaultValue T) T {
	if r.ok {
		return r.value
	}
	return defaultValue
}

// UnwrapOrElse returns the contained success value, or computes a value from the provided function.
//
// Example:
//	result := ParseConfig(data)
//	config := result.UnwrapOrElse(func() Config {
//	    log.Println("Using default config")
//	    return DefaultConfig()
//	})
func (r Result[T, E]) UnwrapOrElse(f func() T) T {
	if r.ok {
		return r.value
	}
	return f()
}

// Map applies a function to the contained success value, transforming it.
//
// If the Result is Ok, the function is applied to the value and a new Ok is returned.
// If the Result is Err, the error is propagated unchanged.
//
// Example:
//	result := ParseInt("42")
//	doubled := result.Map(func(n int) int { return n * 2 })
func (r Result[T, E]) Map(f func(T) T) Result[T, E] {
	if r.ok {
		return Ok[T, E](f(r.value))
	}
	return Err[T, E](r.err)
}

// MapErr applies a function to the contained error, transforming it.
//
// If the Result is Err, the function is applied to the error and a new Err is returned.
// If the Result is Ok, the value is propagated unchanged.
//
// Example:
//	result := ParseConfig(data)
//	annotated := result.MapErr(func(e ParseError) ParseError {
//	    e.ContextStr = "while parsing main config"
//	    return e
//	})
func (r Result[T, E]) MapErr(f func(E) E) Result[T, E] {
	if r.ok {
		return Ok[T, E](r.value)
	}
	return Err[T, E](f(r.err))
}

// AndThen chains operations that can fail, returning the result of the function if Ok.
//
// If the Result is Ok, the function is applied to the value and its result is returned.
// If the Result is Err, the error is propagated unchanged.
//
// This is useful for chaining multiple operations that can fail:
//
//	result := ParseFile(path).AndThen(func(data []byte) Result[Config, ParseError] {
//	    return ParseConfig(data)
//	})
func (r Result[T, E]) AndThen(f func(T) Result[T, E]) Result[T, E] {
	if r.ok {
		return f(r.value)
	}
	return Err[T, E](r.err)
}

// OrElse returns the provided Result if this Result is Err, otherwise returns itself.
//
// This is useful for providing fallback results:
//
//	result := TryPrimaryConfig().OrElse(func() Result[Config, ParseError] {
//	    return TryFallbackConfig()
//	})
func (r Result[T, E]) OrElse(f func(E) Result[T, E]) Result[T, E] {
	if r.ok {
		return r
	}
	return f(r.err)
}

// Match executes the appropriate function based on the Result state.
//
// If Ok, onOk is called with the value.
// If Err, onErr is called with the error.
//
// Example:
//	result := ParseConfig(path)
//	result.Match(
//	    func(config Config) {
//	        log.Println("Config parsed successfully")
//	    },
//	    func(err ParseError) {
//	        log.Printf("Parse failed: %v", err)
//	    },
//	)
func (r Result[T, E]) Match(onOk func(T), onErr func(E)) {
	if r.ok {
		onOk(r.value)
	} else {
		onErr(r.err)
	}
}

// Match returns a value based on the Result state.
//
// If Ok, onOk is called with the value and its result is returned.
// If Err, onErr is called with the error and its result is returned.
//
// Example:
//	result := ParseConfig(path)
//	message := result.MatchReturn(
//	    func(config Config) string { return "Success" },
//	    func(err ParseError) string { return "Failed: " + err.Error() },
//	)
func (r Result[T, E]) MatchReturn(onOk func(T) any, onErr func(E) any) any {
	if r.ok {
		return onOk(r.value)
	}
	return onErr(r.err)
}

// ToOption converts the Result to an Option[T] (if you had an Option type).
//
// Ok(value) becomes Some(value)
// Err becomes None
//
// This is useful when you want to ignore errors and just work with optional values.
func (r Result[T, E]) ToOption() Option[T] {
	if r.ok {
		return Some[T](r.value)
	}
	return None[T]()
}

// Error returns the error as an error interface, or nil if Ok.
//
// This allows Result types to satisfy the error interface indirectly.
func (r Result[T, E]) Error() error {
	if r.ok {
		return nil
	}
	// Try to convert E to error interface
	if err, ok := any(r.err).(error); ok {
		return err
	}
	return nil
}

// String returns a string representation of the Result.
func (r Result[T, E]) String() string {
	if r.ok {
		return fmt.Sprintf("Ok(%v)", r.value)
	}
	return fmt.Sprintf("Err(%v)", r.err)
}

// ============================================================================
// Helper functions for common patterns
// ============================================================================

// CollectResults collects multiple Results into a single Result.
//
// If all Results are Ok, returns Ok with a slice of all values.
// If any Result is Err, returns Err with the first error encountered.
//
// Example:
//	results := []Result[int, ParseError]{Ok(1), Ok(2), Ok(3)}
//	combined := CollectResults(results)
//	// combined is Ok([]int{1, 2, 3})
func CollectResults[T any, E any](results []Result[T, E]) Result[[]T, E] {
	values := make([]T, 0, len(results))
	for _, result := range results {
		if result.IsOk() {
			values = append(values, result.Unwrap())
		} else {
			return Err[[]T, E](result.UnwrapErr())
		}
	}
	return Ok[[]T, E](values)
}

// PartitionResults separates Results into Ok and Err groups.
//
// Returns two slices: all successful values and all errors.
//
// Example:
//	results := []Result[int, ParseError]{Ok(1), Err(...), Ok(2)}
//	oks, errs := PartitionResults(results)
//	// oks = []int{1, 2}
//	// errs = []ParseError{...}
func PartitionResults[T any, E any](results []Result[T, E]) ([]T, []E) {
	oks := make([]T, 0)
	errs := make([]E, 0)
	for _, result := range results {
		if result.IsOk() {
			oks = append(oks, result.Unwrap())
		} else {
			errs = append(errs, result.UnwrapErr())
		}
	}
	return oks, errs
}

// Transpose converts a Result of Option to an Option of Result.
//
// Ok(Some(x)) -> Some(Ok(x))
// Ok(None) -> None
// Err(e) -> Some(Err(e))
//
// This is useful when working with nested optional/error results.
func Transpose[T any, E any](r Result[Option[T], E]) Option[Result[T, E]] {
	if r.IsErr() {
		return Some(Result[T, E]{ok: false, err: r.UnwrapErr()})
	}
	opt := r.Unwrap()
	if opt.IsSome() {
		return Some(Ok[T, E](opt.Unwrap()))
	}
	return None[Result[T, E]]()
}

// ============================================================================
// Integration with existing error types
// ============================================================================

// AsParseError converts any error to a ParseError.
//
// If the error is already a ParseError, it's returned as-is.
// If it's a YAMLError, it's converted to a ParseError.
// Otherwise, a generic ParseError is created.
func AsParseError(err error) *ParseError {
	if err == nil {
		return nil
	}

	// Already a ParseError
	if pe, ok := err.(*ParseError); ok {
		return pe
	}

	// Try to extract info from YAMLError types
	if ye, ok := err.(YAMLError); ok {
		// Extract line/column info if available
		line, column := 0, 0
		filePath := ""
		message := err.Error()

		// Try to get location info from specific error types
		switch e := err.(type) {
		case *SyntaxError:
			line, column = e.Line, e.Column
			filePath = e.FilePath
			message = e.Message
		case *StructureError:
			line = e.Line
			filePath = e.FilePath
			message = e.Message
		case *TypeMismatchError:
			line = e.Line
			filePath = e.FilePath
			message = fmt.Sprintf("type mismatch at %s: expected %s, got %s",
				e.FieldPath, e.ExpectedType, e.ActualType)
		case *ValidationError:
			line, column = e.Line, e.Column
			filePath = e.FilePath
			message = e.Message
		}

		return &ParseError{
			FilePath:  filePath,
			Line:      line,
			Column:    column,
			Message:   message,
			Err:       err,
			ErrorType: ye.YAMLErrorType(),
			ErrorCode: ye.Code(),
		}
	}

	// Generic error - create basic ParseError
	return &ParseError{
		Message: err.Error(),
		Err:     err,
	}
}

// FromError converts a standard Go error to a Result[T, ParseError].
//
// If err is nil, returns Ok with the provided value.
// If err is non-nil, converts it to a ParseError and returns Err.
func FromError[T any](value T, err error) Result[T, *ParseError] {
	if err == nil {
		return Ok[T, *ParseError](value)
	}
	return Err[T, *ParseError](AsParseError(err))
}

// WithLineNumber adds line number information to a ParseError.
//
// If the result is Ok, returns it unchanged.
// If the result is Err, adds the line number to the error.
func WithLineNumber[T any](r Result[T, *ParseError], line int) Result[T, *ParseError] {
	if r.IsOk() {
		return r
	}
	err := r.UnwrapErr()
	if err.Line == 0 {
		err.Line = line
	}
	return Err[T, *ParseError](err)
}

// WithContext adds context information to a ParseError.
//
// If the result is Ok, returns it unchanged.
// If the result is Err, adds the context string to the error.
func WithContext[T any](r Result[T, *ParseError], context string) Result[T, *ParseError] {
	if r.IsOk() {
		return r
	}
	err := r.UnwrapErr()
	if err.ContextStr == "" {
		err.ContextStr = context
	} else {
		err.ContextStr = context + ": " + err.ContextStr
	}
	return Err[T, *ParseError](err)
}

// ============================================================================
// Option type (for ToOption conversion)
// ============================================================================

// Option represents an optional value that can be Some(T) or None.
//
// This is a simple option type used for interoperability with Result.
type Option[T any] struct {
	some bool
	value T
}

// Some creates an Option containing a value.
func Some[T any](value T) Option[T] {
	return Option[T]{some: true, value: value}
}

// None creates an empty Option.
func None[T any]() Option[T] {
	return Option[T]{some: false}
}

// IsSome returns true if the Option contains a value.
func (o Option[T]) IsSome() bool {
	return o.some
}

// IsNone returns true if the Option is empty.
func (o Option[T]) IsNone() bool {
	return !o.some
}

// Unwrap returns the contained value, panics if None.
func (o Option[T]) Unwrap() T {
	if o.some {
		return o.value
	}
	panic("unwrap called on None")
}

// UnwrapOr returns the contained value or the provided default.
func (o Option[T]) UnwrapOr(defaultValue T) T {
	if o.some {
		return o.value
	}
	return defaultValue
}

// ============================================================================
// Helper functions
// ============================================================================

// stringifyError converts an error to a string representation.
func stringifyError(err any) string {
	if err == nil {
		return "<nil>"
	}
	if e, ok := err.(error); ok {
		return e.Error()
	}
	return fmt.Sprintf("%v", err)
}
