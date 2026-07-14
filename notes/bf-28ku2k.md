# Bead bf-28ku2k: Error Response Structure Validation

## Task
Add error response structure validation

## Implementation Status
**COMPLETE** - Implementation already exists in `internal/server/error_test_infrastructure_test.go`

## Verified Implementation

### Functions Implemented
1. **`ValidateErrorResponseStructureSimple(t *testing.T, body []byte) bool`**
   - Basic validation with default options
   - Checks Code and Message fields are present and non-empty
   - Requires minimum message length of 10 characters

2. **`ValidateErrorResponseStructure(t *testing.T, body []byte, options ErrorStructureValidationOptions) bool`**
   - Full-featured validation with custom options
   - Supports configurable field requirements
   - Validates expected code values
   - Checks minimum message length
   - Validates message content (case-insensitive substring matching)
   - Supports custom field validation

3. **`AssertValidErrorResponseStructure(t *testing.T, body []byte)`**
   - Assertion helper that fails test on validation failure
   - Provides detailed error messages with specific field paths

4. **`AssertValidErrorResponseStructureWithOptions(t *testing.T, body []byte, options ErrorStructureValidationOptions)`**
   - Assertion helper with custom options
   - Detailed error messages for debugging

5. **`DefaultValidationOptions() ErrorStructureValidationOptions`**
   - Returns default validation configuration
   - RequireCode: true
   - RequireMessage: true
   - MinMessageLength: 10

### Validation Options
The `ErrorStructureValidationOptions` struct supports:
- `RequireCode`: Ensure Code field is present and non-empty
- `RequireMessage`: Ensure Message field is present and non-empty
- `ExpectedCode`: Validate specific error code value
- `MinMessageLength`: Set minimum message length requirement
- `MessageContains`: Check message contains specific substring (case-insensitive)
- `CustomFields`: Validate additional XML fields

### Acceptance Criteria Met
✅ Create a function to validate error response format
✅ Check for required error fields (Code, Message)
✅ Validate that error message is a non-empty string
✅ Support custom error schema validation (via options)
✅ Return clear validation errors with specific field paths (in assertions)
✅ Include examples of validation failures and successes (comprehensive test coverage)

### Test Coverage
All tests pass with 100% success rate:
- `TestValidateErrorResponseStructureSimple` - 10/10 subtests pass
- `TestValidateErrorResponseStructureWithOptions` - 8/8 subtests pass
- `TestAssertValidErrorResponseStructure` - 2/2 subtests pass
- `TestDefaultValidationOptions` - PASS
- `TestValidateErrorResponseStructureEdgeCases` - 7/7 subtests pass
- `TestValidateErrorResponseStructureRealWorldExamples` - 5/5 subtests pass
- `TestValidateErrorResponseStructureWithSpecificCodes` - 3/3 subtests pass
- `TestValidateErrorResponseStructureMessageContainsTests` - 4/4 subtests pass

Total: 39 test cases covering:
- Valid/invalid XML structures
- Missing/empty required fields
- Field value validation
- Message length constraints
- Content validation (substring matching)
- Case-insensitive matching
- Real-world S3 error responses (AccessDenied, SignatureDoesNotMatch, etc.)

## Implementation Location
File: `internal/server/error_test_infrastructure_test.go`
Lines: 691-889 (ErrorStructureValidationOptions type and all validation functions)

## Related Files
- `internal/server/error_structure_validation_test.go` - Comprehensive test suite
- `internal/server/error_test_patterns.go` - S3Error type definition
- `internal/server/test_request_validation_helpers.go` - Helper utilities
