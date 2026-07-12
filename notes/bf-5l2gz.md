# Bead bf-5l2gz: ParseError Constructor Verification

## Task
Update ParseError constructions to use NewParseError() in parse_error test files.

## Files Checked
- `internal/yamlutil/parse_error_design_test.go`
- `internal/yamlutil/parse_error_examples_test.go`

## Verification Results

### parse_error_design_test.go
✅ **Status: Already compliant**

All ParseError constructions already use constructor functions:
- `NewSyntaxParseError()` - used for syntax error tests
- `NewStructureParseError()` - used for structure error tests
- `NewTypeMismatchParseError()` - used for type mismatch tests
- `NewIOParseError()` - used for I/O error tests
- `NewValidationParseError()` - used for validation error tests
- `NewSchemaParseError()` - used for schema error tests
- `NewEmptyParseError()` - used for empty file error tests

Total constructor calls: **25+**
Direct struct constructions: **0**

### parse_error_examples_test.go
✅ **Status: Already compliant**

All example code uses constructor functions:
- All example functions demonstrate proper use of `New*ParseError()` constructors
- No direct `&EnhancedParseError{}` constructions found

Total constructor calls: **30+**
Direct struct constructions: **0**

## Additional Fix
While verifying the task, found and fixed a build error in `error_message_quality_test.go`:
- Line 948: Fixed `NewSyntaxError()` call - added missing `column` parameter (0)
- Changed from: `NewSyntaxError("f.yaml", "m", 0, "", "", "")`
- Changed to: `NewSyntaxError("f.yaml", "m", 0, 0, "", "", "")`

## Tests Passing
All EnhancedParseError tests pass successfully:
```
TestEnhancedParseErrorYAMLErrorInterface - PASS
TestEnhancedParseErrorKindCheckers - PASS
TestEnhancedParseErrorToLegacyConversions - PASS
TestEnhancedParseErrorString - PASS
```

## Conclusion
The parse_error test files already follow the best practice of using constructor functions instead of direct struct construction. No changes needed to the target files. The only modification was a bug fix to error_message_quality_test.go.
