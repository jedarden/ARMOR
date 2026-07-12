# Bead bf-1cnxs: FileError to NewFileError Replacement

## Task
Replace all 2 FileError struct initializations with NewFileError() constructor calls in error_message_quality_test.go.

## Finding
The task has already been completed. All FileError usages in error_message_quality_test.go already use the NewFileError() constructor.

## Evidence
Git history shows this was completed in two commits:
- `02e4eb46`: "test: replace FileError constructions with NewFileError() constructor"
- `09966765`: "refactor: replace all FileError struct initializations with NewFileError() constructor"

Current usages in the file:
1. Line 72: `return NewFileError("/etc/config/app.yaml", "read", "file not found", nil)`
2. Line 402: `return NewFileError("config.yaml", "", "", fmt.Errorf("not found"))`
3. Line 977: `return NewFileError("f.yaml", "", "", fmt.Errorf("e"))`

## Status
✓ All FileError instances already use NewFileError() constructor
✓ No changes needed - task was pre-completed
