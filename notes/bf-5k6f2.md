# BF-5k6f2: Test Struct Field Mapping Analysis

## Task: Determine correct field mappings for test structs

## Investigation Results

### Finding 1: No Undefined Field References Found

The task description referenced "undefined field references (filePath0, filePath1, etc.)" that were supposedly identified in the previous bead (bf-35c47). However, the previous bead's investigation concluded:

- **Search for numbered field patterns found NO instances** of `filePath0`, `filePath1`, etc. in the codebase
- The actual issues found were **missing function parameters**, not incorrect field names
- All fields referenced in test code **exist and are correctly named**

### Finding 2: Test Struct Naming Convention

Test structs in `internal/yamlutil` use a simple, consistent naming pattern without numbered variants:

```go
tests := []struct {
    name         string    // Test case name
    yamlContent  string    // YAML content to test
    target       interface{} // Target struct for unmarshaling
    shouldError  bool      // Whether error is expected
    description  string    // Test description
}{
    // test cases...
}
```

**Key Fields:**
- `name` - test case identifier
- `yamlContent` or `content` - YAML source content
- `target` - struct for unmarshaling
- `shouldError` - expected error state
- `filePath` - file path (single instance per test case)
- `expected` - expected result value
- `description` - test description
- `errorMsg` - expected error message

### Finding 3: When Numbered Variants Are NOT Needed

The numbering pattern (`field0`, `field1`, `field2`) is **not used** because:

1. Each test case tests **one scenario** - no need for multiple instances of the same field type
2. Test cases are defined in **array slices**, where each array element represents a different scenario
3. When multiple files need testing, they use **separate test cases** in the array, not multiple fields in one struct

**Example pattern (correct):**
```go
tests := []struct {
    name     string
    filePath string  // Single filePath per case
    content  string
}{
    {name: "case 1", filePath: "file1.yaml", content: "..."},
    {name: "case 2", filePath: "file2.yaml", content: "..."},
    // Each case is a separate array element
}
```

**NOT this (unnecessary):**
```go
// This pattern is NOT used in the codebase
testCase := struct {
    name      string
    filePath0 string  // ❌ Not used
    filePath1 string  // ❌ Not used
    filePath2 string  // ❌ Not used
}{}
```

### Finding 4: Field Mapping Table

| Test Field Name | Type | Purpose | Used In |
|-----------------|------|---------|---------|
| `name` | string | Test case identifier | All test files |
| `yamlContent` | string | YAML source content | Conversion tests |
| `content` | string | YAML source content | Validation tests |
| `target` | interface{} | Struct for unmarshaling | Type conversion tests |
| `shouldError` | bool | Expected error state | Error scenario tests |
| `filePath` | string | File path | File operation tests |
| `expected` | bool/int/string | Expected result | Assertion tests |
| `description` | string | Test description | All test files |
| `errorMsg` | string | Expected error message | Error message tests |
| `err` | error | Error value | Error helper tests |
| `line`/`column` | int | Position in file | Parse error tests |

## Conclusion

### No Field Mapping Corrections Needed

- **All existing field names are correct**
- **No numbered variants (filePath0, filePath1, etc.) exist or are needed**
- **Test struct naming follows Go conventions without ambiguity**

### The Real Issue (from bf-35c47)

The previous investigation found that the actual problem was:
- **Missing function parameters** in `NewValidationError` calls
- Missing `expectedType` and `actualType` parameters (11 parameters required, only 9 provided)
- This was **fixed in commit fdf4871f**

### Naming Convention Summary

**When to use numbered variants:** **NEVER** - not needed in this codebase
**When to use simple names:** **ALWAYS** - single field names are sufficient

Test cases use array slices for multiple scenarios, not multiple fields of the same type in a single struct.

## Related Documentation

- Bead bf-35c47: Undefined field reference investigation results
- Commit fdf4871f: "fix(bf-445du): Update NewValidationError calls with type parameters"
- Previous bead concluded no undefined field references exist
