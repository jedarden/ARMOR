# Syntax Errors in int64 Negative Conversion Test File

## Analysis Date
2026-07-12

## File Analyzed
`/home/coding/ARMOR/internal/yamlutil/int64_to_uint64_negative_conversion_test.go`

## Critical Syntax Errors

### 1. Malformed Import Statement (Line 5)
**Location:** Line 5
**Current Code:**
```go
import (
	"strings"
	"testing"
)
```

**Error:** Missing opening quote on `"strings"` import

**Expected:**
```go
import (
	"strings"
	"testing"
)
```

**Impact:** This is a compilation error that will prevent the file from being parsed or built.

---

### 2. Undefined Helper Function `containsAny`
**Location:** Lines 279 and 844

**Usage on Line 279:**
```go
if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "out of range", "invalid"}) {
    t.Logf("✓ Error message indicates invalid conversion for negative value")
}
```

**Usage on Line 844:**
```go
if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "invalid", "out of range"}) {
    t.Logf("✓ Error message indicates invalid conversion")
}
```

**Error:** The function `containsAny` is called but never defined in the file.

**Expected Function Definition:**
```go
func containsAny(s string, substrs []string) bool {
    for _, substr := range substrs {
        if strings.Contains(s, substr) {
            return true
        }
    }
    return false
}
```

**Impact:** Compilation error - undefined function reference.

---

## Structural Analysis vs. int32 Version

### Comparison with int32 Test File
The int64 version follows the same pattern as the int32 version (`int32_to_uint32_negative_conversion_test.go`). Both files share the **same two issues**:

1. Both use the malformed import statement (though the int32 version doesn't show it in the read output, suggesting it may have been fixed separately)
2. Both are missing the `containsAny` helper function

### Differences from int32 Pattern

The int64 version correctly adapts the int32 pattern for 64-bit values:
- Uses `-9223372036854775808` (int64 minimum) instead of `-2147483648` (int32 minimum)
- Uses `uint64` target types instead of `uint32`
- Test names updated from `int32` to `int64`
- Value ranges appropriate for 64-bit integers

The structural pattern is consistent, suggesting this was a copy-paste adaptation that preserved both the working pattern and the bugs.

---

## Summary

### Severity: CRITICAL
Both errors are **compilation errors** that prevent the test file from being built or run.

### Issues Count: 2
1. Malformed import statement (Line 5)
2. Missing `containsAny` helper function (used on lines 279, 844)

### Recommendation
1. Fix the import statement by adding the opening quote to `"strings"`
2. Add the `containsAny` helper function definition (likely should be in a shared utility file used by both int32 and int64 test files)

### Pattern Inconsistency
The fact that both int32 and int64 versions are missing the same `containsAny` function suggests:
- Either the function was defined in a shared location that's not being imported
- Or the function was removed/never added during development
- A common utility file should be created and imported by both test files
