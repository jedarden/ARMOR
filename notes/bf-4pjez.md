# Bead bf-4pjez: SchemaValidationError Constructor Replacement - Already Completed

## Task Description
Replace direct `SchemaValidationError` struct initialization with `NewSchemaValidationError()` constructor calls in `internal/yamlutil/errors_test.go`.

## Finding
**Work already completed.** The target file `internal/yamlutil/errors_test.go` already uses `NewSchemaValidationError()` constructor calls for all SchemaValidationError instances.

### Evidence
1. **Line 43**: `err: NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`
2. **Line 91**: `err: NewSchemaValidationError("test.yaml", "", "", "", "", "", 0, "")`

### Verification
- No instances of `&SchemaValidationError{` found in errors_test.go
- Backup file (errors_test.go.bak) also uses NewSchemaValidationError()
- Tests compile successfully: `go test -c -o /dev/null ./internal/yamlutil/`

### Conclusion
The bead was created based on outdated information. The SchemaValidationError constructor replacements were already completed in a prior change (likely as part of the broader error constructor refactoring effort that also covered ParseError, ValidationError, and FileError).

## Recommendation
Close this bead as completed - no action required.
