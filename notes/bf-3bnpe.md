# Bead bf-3bnpe: Update example and formatting test files to pass path parameter

## Task
Update NewValidationError calls in example and formatting test files to pass path parameter.

## Scope
- internal/yamlutil/error_message_format_examples_test.go (8 calls)
- internal/yamlutil/verify_formatting_test.go (2 calls)
- internal/yamlutil/validation_error_demo_test.go (3 calls)

## Findings

This work was already completed in previous commits:
- Commit c6fecfaa: `test(error_message_format_examples_test.go): update NewValidationError calls to pass path parameter`
- Commit b2511939: `test(yamlutil): update NewValidationError callers to pass path parameter`

### Verification Results

All 13 NewValidationError calls across the 3 files already pass the path parameter correctly:

**error_message_format_examples_test.go (8 calls):**
- Line 195: `path="server.port"` (matches fieldPath)
- Line 258: `path=tt.fieldPath` (dynamic, matches fieldPath)
- Line 283: `path="server.port"` (matches fieldPath)
- Line 307: `path="config.yaml"` (file-level error, fieldPath empty)
- Line 336: `path="spec.template.spec.containers[0].image"` (matches fieldPath)
- Line 739: `path="server.port"` (matches fieldPath)
- Line 837: `path="field"` (matches fieldPath)
- Line 889: `path="field"` (matches fieldPath)

**verify_formatting_test.go (2 calls):**
- Line 35: `path="spec.replicas"` (matches fieldPath)
- Line 112: `path=""` (fieldPath empty, correct for general validation error)

**validation_error_demo_test.go (3 calls):**
- Line 15: `path="server.port"` (matches fieldPath)
- Line 31: `path="spec.template.spec.containers[0].image"` (matches fieldPath)
- Line 47: `path="spec.replicas"` (matches fieldPath)

### Test Results
All tests pass:
- TestValidationErrorDemo: PASS ✓
- TestErrorFormattingExamples: PASS ✓
- TestHumanReadableFormatting: PASS ✓

## Acceptance Criteria Met
- ✓ All NewValidationError calls in these 3 files pass path parameter
- ✓ Path values reflect the actual validation error location (typically use fieldPath value)
- ✓ Tests still pass after changes

## Conclusion
No code changes required. The work was completed in previous commits and all acceptance criteria are met.
