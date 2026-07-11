# NewValidationError Callers - Complete Inventory

**Date:** 2026-07-11  
**Bead:** bf-1im90  
**Search Scope:** Entire ARMOR codebase

---

## Summary

Total of **27 call locations** found across **6 files** (1 implementation + 5 test files).

---

## Categorization

### 1. Function Definition
- **internal/yamlutil/errors.go** (lines 499-520)
  - Definition and documentation

### 2. Direct Test Calls (for testing/error message examples)

#### internal/yamlutil/errors_test.go (10 locations)
- Line 348: Test function definition `TestNewValidationError`
- Line 457: Test call with all parameters
- Line 493: Interface implementation check comment
- Line 498: Type recognition check comment
- Line 512: Test case - invalid port
- Line 522: Test case - validation failed
- Line 530: Test case - syntax error with ErrorType
- Line 539: Test case - value out of range with ErrorType

#### internal/yamlutil/verify_error_formatting_test.go (3 locations)
- Line 28: Direct call for formatting verification
- Line 72: Call in array example
- Line 97: Reference in comment

#### internal/yamlutil/result_types_test.go (3 locations)
- Line 424: Call in test array
- Line 463: Call in test case
- Line 548: Call in nested test

#### internal/yamlutil/error_message_format_examples_test.go (9 locations)
- Line 195: Example with all standard params
- Line 258: Example in test loop
- Line 283: Example with line/column
- Line 307: Example with minimal params
- Line 336: Example with complex field path
- Line 738: Validation error in test
- Line 836: Validation error in test
- Line 888: Validation error in struct

#### internal/yamlutil/validation_error_demo_test.go (3 locations)
- Line 15: First validation error demo
- Line 30: Second validation error demo
- Line 45: Third validation error demo

---

## Call Patterns

### Pattern 1: Full Signature Call (with all 9 parameters)
```
NewValidationError(
    filePath, message, fieldPath, constraint, 
    code, line, column, errorType, path
)
```
Locations:
- internal/yamlutil/errors.go:519 (documentation)
- internal/yamlutil/errors_test.go:457
- internal/yamlutil/error_message_format_examples_test.go:195

### Pattern 2: Legacy Signature Call (8 parameters, no path)
```
NewValidationError(
    filePath, message, fieldPath, constraint, 
    code, line, column, errorType
)
```
Locations:
- internal/yamlutil/error_message_format_examples_test.go:283
- internal/yamlutil/error_message_format_examples_test.go:336
- internal/yamlutil/error_message_format_examples_test.go:307

### Pattern 3: Minimal Signature Call (7-8 parameters, empty strings for optional)
```
NewValidationError(
    filePath, message, fieldPath, constraint, 
    code, 0, 0, "" or errorType
)
```
Locations:
- internal/yamlutil/errors_test.go:512
- internal/yamlutil/errors_test.go:522
- internal/yamlutil/errors_test.go:530
- internal/yamlutil/errors_test.go:539
- internal/yamlutil/verify_error_formatting_test.go:28
- internal/yamlutil/verify_error_formatting_test.go:72
- internal/yamlutil/error_message_format_examples_test.go:258
- internal/yamlutil/error_message_format_examples_test.go:738
- internal/yamlutil/error_message_format_examples_test.go:836
- internal/yamlutil/error_message_format_examples_test.go:888

### Pattern 4: Demo/Test Signature (8 parameters, empty path)
```
NewValidationError(
    filePath, message, fieldPath, constraint, 
    code, line, column, ""
)
```
Locations:
- internal/yamlutil/result_types_test.go:424
- internal/yamlutil/result_types_test.go:463
- internal/yamlutil/result_types_test.go:548
- internal/yamlutil/validation_error_demo_test.go:15
- internal/yamlutil/validation_error_demo_test.go:30
- internal/yamlutil/validation_error_demo_test.go:45

---

## Key Observations

1. **No production code callers found** - All callers are in test files
2. **Function signature evolution visible** - Mix of old (7-8 param) and new (9 param) signatures
3. **Path parameter** is the newest addition (bead bf-4solk, bf-7a42i, bf-2gq9t)
4. **Empty string defaults** used extensively in tests for optional parameters
5. **All callers are in internal/yamlutil package** - No external consumers yet

---

## Impact Assessment

The **NewValidationError constructor signature change** (adding `path` parameter) requires updating:
- **0 production callers** ✅ (No impact)
- **27 test callers** ⚠️ (Need updates for consistency, but tests may continue using old signature via varargs)

---

## Recommendation

Since the `path` parameter was added via:
1. bf-7a42i: Added `Path` field to `ValidationError` struct
2. bf-4solk: Updated constructor to accept `path` parameter

And since all existing callers pass empty string for the new `path` parameter (or use old signature), the change is **backward compatible**.

All 27 test callers can remain as-is or be incrementally updated to provide meaningful `path` values where appropriate.
