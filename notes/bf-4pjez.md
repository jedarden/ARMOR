# Bead bf-4pjez: SchemaValidationError Constructor Replacement - Already Completed

## Task Description
Replace direct `SchemaValidationError` struct initialization with `NewSchemaValidationError()` constructor calls in `internal/yamlutil/errors_test.go`.

## Finding
**Work already completed.** The target file `internal/yamlutil/errors_test.go` already uses `NewSchemaValidationError()` constructor calls for all SchemaValidationError instances.

### Evidence
1. **Line 43**: `err: NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`
2. **Line 91**: `err: NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`

### Verification (2026-07-12)
- ✅ No instances of `&SchemaValidationError{` found in errors_test.go (confirmed via grep)
- ✅ Backup file (errors_test.go.bak) also uses NewSchemaValidationError() (identical to current file)
- ✅ Tests compile successfully: `go test -c ./internal/yamlutil/...`
- ✅ Git history confirms completion in prior commits:
  - `b0160528` - docs(bf-4pjez): document that SchemaValidationError constructor replacements already completed
  - `2bd6d6a2` - docs(bf-4pjez): document that SchemaValidationError constructor replacements already completed

### Conclusion
The bead was created based on outdated information. The SchemaValidationError constructor replacements were already completed in a prior change (likely as part of the broader error constructor refactoring effort that also covered ParseError, ValidationError, and FileError).

**Status: VERIFIED COMPLETED** - No code changes needed.

## Recommendation
Close this bead as completed - no action required.
