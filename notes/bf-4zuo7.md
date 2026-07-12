# Bead bf-4zuo7: TypeMismatchError and SchemaValidationError Constructor Replacements

## Task
Replace direct struct initializations for TypeMismatchError and SchemaValidationError in internal/yamlutil/errors_test.go with their respective constructor calls.

## Status: ALREADY COMPLETED

This work was completed in previous beads:
- **TypeMismatchError**: Replaced in bead `bf-3sh3e` (commit 426fd0d0)
- **SchemaValidationError**: Replaced in bead `bf-46pf0` (commit 93f704b0)

## Verification Results

### Current State (errors_test.go)
All 5 instances already use constructor calls:

**TypeMismatchError (3 instances):**
- Line 573: `NewTypeMismatchError("config.yaml", "server.port", "integer", "string", "abc", 15, ErrCodeTypeMismatch)`
- Line 579: `NewTypeMismatchError("data.yaml", "database.timeout", "integer", "boolean", "true", 0, ErrCodeTypeMismatch)`
- Line 585: `NewTypeMismatchError("app.yaml", "servers.api.responses[0].statusCode", "integer", "string", "\"200\"", 42, ErrCodeTypeMismatch)`

**SchemaValidationError (2 instances):**
- Line 43: `NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`
- Line 91: `NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`

### Constructor Signatures Verified
```go
// NewTypeMismatchError - 7 parameters
func NewTypeMismatchError(filePath string, fieldPath string, expectedType string, actualType string, value string, line int, errorCode ErrorCode) *TypeMismatchError

// NewSchemaValidationError - 8 parameters (not 9 as initially stated)
func NewSchemaValidationError(filePath string, schemaPath string, fieldPath string, message string, expected string, found string, line int, errorCode ErrorCode) *SchemaValidationError
```

### Acceptance Criteria Met
- ✅ All 3 TypeMismatchError constructions use `NewTypeMismatchError()`
- ✅ All 2 SchemaValidationError constructions use `NewSchemaValidationError()`
- ✅ File compiles (`go build ./internal/yamlutil/...`)
- ✅ Tests pass (all TypeMismatch and Schema validation tests pass)
- ✅ No direct struct initializations found (`grep -E "&TypeMismatchError\{|&SchemaValidationError\{"` returns empty)
- ✅ No test logic changed

## Test Execution Results
All error tests passed successfully:
- TestIsYAMLError: PASS
- TestGetYAMLErrorType: PASS
- TestTypeMismatchErrorFormatting: PASS
- TestConstraintErrorFieldPathFormatting: PASS
- TestFieldNotFoundErrorFormatting: PASS
- TestValidationErrorWithTypeInformation: PASS
- TestValidationErrorStringWithTypeInformation: PASS

## Conclusion
The task objective was already achieved. No code changes were needed. The constructor replacements were done in previous beads and verified in this session.
