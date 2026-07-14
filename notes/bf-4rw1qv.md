# Bead bf-4rw1qv: Validation Error Formatting Data Structure Design

## Summary

Designed and documented the comprehensive data structure for consistent validation error formatting in the ARMOR validation library.

## Completed Work

### 1. Design Document (`internal/validate/error_format_design.md`)

Created a comprehensive design document that includes:

- **Core Design Principles**: Consistency, actionability, context, flexibility, composability
- **ValidationError Structure**: Complete field documentation with required/optional classifications
- **Supporting Types**: StatusCodeValidationResult, ErrorMessageValidationResult, ErrorCodeMatch
- **Builder Pattern**: ValidationFormatter with fluent API documentation
- **Convenience Functions**: FormatStatusCodeError, FormatErrorMessageError, etc.
- **Design Decisions**: Rationale for key architectural choices
- **Extensibility Guidelines**: How to add new validation types

### 2. Practical Examples (`internal/validate/error_format_examples.go`)

Created 15 comprehensive examples demonstrating:

- Basic status code validation
- Convenience function usage
- Multiple expected status codes
- Error message pattern validation
- Status code range validation
- Content-Type validation
- Custom validation with format options
- Complex validation scenarios
- Programmatic field access
- Error interface implementation
- Test usage patterns
- Reusable helper functions

## Key Design Decisions

1. **Required Fields**: ValidationType, Expected, Actual (must always be present)
2. **Optional Context Fields**: Context, ResponseSnippet, FieldName (for debugging)
3. **Validation-Specific Details**: PatternDetails, RangeInfo, ValidationDetails (for specialized scenarios)
4. **Auto-Generated Suggestions**: Suggestions populated automatically when not provided
5. **Builder Pattern**: Fluent API for ergonomic error construction
6. **Interface{} for Values**: Flexibility to handle different validation types

## File Structure

- `internal/validate/error_format_design.md` - Complete design documentation
- `internal/validate/error_format_examples.go` - 15 usage examples

## Validation Types Supported

1. `status_code` - HTTP status code validation
2. `error_message` - Error message content validation
3. `content_type` - Content-Type header validation
4. `status_code_range` - Status code range validation

## Acceptance Criteria Met

✅ Define the error format structure (what fields are required vs optional)
✅ Specify how error context, expected values, actual values, and suggestions are represented
✅ Create a clear type/struct definition for the error format
✅ Document the design decisions for the structure

## Next Steps

The design is complete and ready for implementation. The existing code in `validate.go` and `format_helper.go` already implements most of this design. This documentation formalizes the structure and provides clear guidance for future extensions.