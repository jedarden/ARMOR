# NewValidationError Callers Inventory

**Task:** Identify all NewValidationError callers in the ARMOR codebase

## Summary

**Total Files Containing NewValidationError:** 10 files  
**Production Code Calls:** 0 (only definition, no actual usage)  
**Test Code Calls:** All usage is in test files

## Function Signature

```go
func NewValidationError(filePath string, message string, fieldPath string, constraint string, code ErrorCode, line int, column int, errorType ErrorType, path string) *ValidationError
```

**Parameters:**
- `filePath`: Path to the file being validated
- `message`: Human-readable error message
- `fieldPath`: Dot-notation path to the invalid field (optional)
- `constraint`: Constraint that was violated (optional)
- `code`: Error code for programmatic handling (use empty string for default)
- `line`: Line number where error occurred (1-indexed, use 0 if unknown)
- `column`: Column number where error occurred (1-indexed, use 0 if unknown)
- `errorType`: Category of error (use empty string for default ErrorTypeValidation)
- `path`: Dot-notation field path (optional, for backward compatibility defaults to empty string)

## Files Containing NewValidationError

### Production Code

1. **internal/yamlutil/errors.go** (lines 520-572)
   - **Type:** Function definition only
   - **Production Usage:** None (definition includes documentation example only)
   - **Status:** ✅ Passes path parameter (final parameter in signature)

### Test Files (All calls are in test code)

2. **internal/yamlutil/errors_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Test function for NewValidationError implementation
   - **Example call:** `NewValidationError("config.yaml", "invalid port", "server.port", "must be 1-65535", ErrCodeInvalidValue, 0, 0, "", "server.port")`

3. **internal/yamlutil/result_types_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Tests for type checking and result handling

4. **internal/yamlutil/verify_formatting_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Tests for error message formatting

5. **internal/yamlutil/validation_error_path_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Tests specifically for path parameter handling

6. **internal/yamlutil/path_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Tests for path handling edge cases

7. **internal/yamlutil/error_message_format_examples_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Examples of error message formatting

8. **internal/yamlutil/validation_error_demo_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Demonstration of ValidationError usage

9. **internal/yamlutil/verify_error_formatting_test.go**
   - **Status:** ✅ All calls pass path parameter
   - **Usage:** Verification of error formatting

10. **internal/yamlutil/error_message_quality_test.go**
    - **Status:** ✅ All calls pass path parameter
    - **Usage:** Tests for error message quality

11. **internal/yamlutil/error_message_quality_comprehensive_test.go**
    - **Status:** ✅ All calls pass path parameter
    - **Usage:** Comprehensive quality tests

## Key Findings

1. **No Production Usage:** `NewValidationError` is not actually used in any production code within the ARMOR codebase. All calls are exclusively in test files.

2. **Test Coverage Complete:** All test files properly pass the `path` parameter (9th parameter) to `NewValidationError`.

3. **Function Location:** The function is defined in `internal/yamlutil/errors.go` at line 541.

4. **Backward Compatibility:** The function includes backward compatibility logic (lines 554-559) that uses `fieldPath` as fallback when `path` is empty, ensuring the `Path` field is populated when available.

5. **Recent Work:** Based on git log showing recent verification commits (d8d59082, 71383f0a), the task of ensuring all NewValidationError calls pass the path parameter has already been completed.

## Conclusion

**Task Complete:** All `NewValidationError` callers have been identified and cataloged. The verification confirms that all test code properly passes the `path` parameter, and there is no production code usage of this function in the current ARMOR codebase.

**Files Changed:** 0 (this was a discovery/cataloging task only)

**Next Steps:** None required - all calls already pass the path parameter correctly.
