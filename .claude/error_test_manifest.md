# Error Test Pattern Manifest

## Overview
This document inventories all test files in `internal/yamlutil` that use direct struct initialization for error types.

**Generated:** 2026-07-12
**Purpose:** Systematic migration from direct struct initialization to constructor functions
**Total test files scanned:** 45
**Files with direct struct initialization:** 10

## Summary Table

| File | ParseError | ValidationError | FileError | SchemaValidationError | TypeMismatchError | Total |
|------|-----------|-----------------|-----------|----------------------|-------------------|-------|
| debug_helpers_test.go | 0 | 0 | 0 | 0 | 1 | 1 |
| error_message_format_examples_test.go | 0 | 0 | 0 | 0 | 8 | 8 |
| error_message_quality_test.go | 0 | 0 | 2 | 0 | 6 | 8 |
| errors_test.go | 0 | 5 | 2 | 2 | 3 | 12 |
| file_test.go | 0 | 0 | 18 | 0 | 0 | 18 |
| missing_file_scenarios_test.go | 0 | 0 | 18 | 0 | 0 | 18 |
| parse_result_test.go | 2 | 0 | 0 | 0 | 0 | 2 |
| result_test.go | 3 | 0 | 0 | 0 | 0 | 3 |
| result_types_test.go | 0 | 9 | 0 | 0 | 0 | 9 |
| validator_test.go | 0 | 4 | 0 | 0 | 0 | 4 |

## Detailed Findings

### debug_helpers_test.go

| Error Type | Count |
|-----------|-------|
| TypeMismatchError | 1 |

**Total changes needed:** 1

---

### error_message_format_examples_test.go

| Error Type | Count |
|-----------|-------|
| TypeMismatchError | 8 |

**Total changes needed:** 8

---

### error_message_quality_test.go

| Error Type | Count |
|-----------|-------|
| FileError | 2 |
| TypeMismatchError | 6 |

**Total changes needed:** 8

---

### errors_test.go

| Error Type | Count |
|-----------|-------|
| ValidationError | 5 |
| FileError | 2 |
| SchemaValidationError | 2 |
| TypeMismatchError | 3 |

**Total changes needed:** 12

---

### file_test.go

| Error Type | Count |
|-----------|-------|
| FileError | 18 |

**Total changes needed:** 18

---

### missing_file_scenarios_test.go

| Error Type | Count |
|-----------|-------|
| FileError | 18 |

**Total changes needed:** 18

---

### parse_result_test.go

| Error Type | Count |
|-----------|-------|
| ParseError | 2 |

**Total changes needed:** 2

---

### result_test.go

| Error Type | Count |
|-----------|-------|
| ParseError | 3 |

**Total changes needed:** 3

---

### result_types_test.go

| Error Type | Count |
|-----------|-------|
| ValidationError | 9 |

**Total changes needed:** 9

---

### validator_test.go

| Error Type | Count |
|-----------|-------|
| ValidationError | 4 |

**Total changes needed:** 4

---

## Overall Totals

| Error Type | Total Count | Files Affected |
|-----------|--------------|----------------|
| ParseError | 5 | 2 |
| ValidationError | 18 | 3 |
| FileError | 40 | 4 |
| SchemaValidationError | 2 | 1 |
| TypeMismatchError | 18 | 4 |
| **Grand Total** | **83** | **10** |

## Migration Priority

### High Priority (20+ occurrences)
- **FileError** (40 occurrences across 4 files)
  - file_test.go: 18
  - missing_file_scenarios_test.go: 18
  - errors_test.go: 2
  - error_message_quality_test.go: 2

### Medium Priority (10+ occurrences)
- **ValidationError** (18 occurrences across 3 files)
  - result_types_test.go: 9
  - errors_test.go: 5
  - validator_test.go: 4
- **TypeMismatchError** (18 occurrences across 4 files)
  - error_message_format_examples_test.go: 8
  - error_message_quality_test.go: 6
  - errors_test.go: 3
  - debug_helpers_test.go: 1

### Low Priority (<10 occurrences)
- **ParseError** (5 occurrences across 2 files)
  - result_test.go: 3
  - parse_result_test.go: 2
- **SchemaValidationError** (2 occurrences across 1 file)
  - errors_test.go: 2

## Recommended Migration Order

1. **Phase 1: FileError** (40 occurrences) - Highest volume, focused in 2 major files
2. **Phase 2: ValidationError** (18 occurrences) - Second highest volume
3. **Phase 3: TypeMismatchError** (18 occurrences) - Same volume as ValidationError
4. **Phase 4: ParseError** (5 occurrences) - Low volume, quick wins
5. **Phase 5: SchemaValidationError** (2 occurrences) - Lowest volume

## Notes

- All files are located in `internal/yamlutil/` directory
- Direct struct initialization patterns identified: `ErrorName{...}`
- Migration should use constructor functions once available
- Test files should continue to compile and pass tests after migration
