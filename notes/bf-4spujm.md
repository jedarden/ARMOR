# Bead bf-4spujm: FormatError Documentation - Code Removed

## Status: OBSOLETE - Codebase Changed

### Summary

This bead was created to add FormatError struct documentation and cleanup, but the entire `internal/validate` package (including the FormatError struct) was removed from the codebase before this work could be completed.

### Timeline

1. **2026-07-16 13:07** - Commit `722b848e` added FormatError struct documentation:
   - Added `NewFormatError()` function for creating ValidationErrors with explicit category control
   - Added `NewFormatErrorWithAutoCategory()` function for auto-detecting category from error type
   - Added `WithExpected()`, `WithActual()`, and `WithLocation()` FormatOption functions
   - Updated `FormatConfig` struct to include Expected, Actual, and Location fields
   - Added comprehensive godoc comments explaining category field usage
   - Included detailed examples for all new functions

2. **2026-07-15 22:59** - Commit `c49bca3d` removed the entire `internal/validate` package:
   - CI (armor-build: golangci-lint + go test -race) had been red for days
   - Root cause: auto-generated error-formatting/validation work from 07-14/07-15
   - `internal/validate`: dozens of its own tests failing, plus a test hanging past the 600s package timeout
   - The package was not referenced by any planned ARMOR functionality; sole consumer was itself generated
   - Also removed 20 generated test/helper files in `internal/server` (45 failing tests, 2 data races)

3. **Current state** - HEAD `e63ed8df`:
   - The `internal/validate` directory no longer exists
   - All FormatError-related code has been removed
   - CI is now green

### Files That Existed (Now Removed)

- `internal/validate/error_types.go` - FormatError struct definition
- `internal/validate/format_helper.go` - NewFormatError and helper functions
- `internal/validate/format_helper_doc_examples_test.go` - Examples and tests
- Plus 80+ other test files and documentation files

### Commit Message from Removal

From commit `c49bca3d`:
> "fix: make lint and race-tested suite green — remove broken generated validation cluster
>
> CI (armor-build: golangci-lint + go test -race) has been red for days.
> Root causes, all from the auto-generated error-formatting/validation
> work of 07-14/07-15:
>
> - internal/validate: dozens of its own tests failing, plus a test
>   hanging past the 600s package timeout. Not referenced by any planned
>   ARMOR functionality; sole consumer was itself generated. Removed.
> - internal/server: 20 generated test/helper files (45 failing tests,
>   2 data races under -race, misnamed non-_test.go test files). Removed."

### Acceptance Criteria Status

- [X] Add godoc comments for FormatError struct explaining category field - **CODE REMOVED**
- [X] Add examples for NewFormatError and NewFormatErrorWithAutoCategory - **CODE REMOVED**
- [X] Verify backward compatibility is maintained - **N/A, CODE REMOVED**
- [X] Clean up any TODOs or FIXMEs related to category field - **N/A, CODE REMOVED**
- [X] Ensure all code follows project conventions - **N/A, CODE REMOVED**

### Recommendation

**CLOSE THIS BEAD AS OBSOLETE**

The work described in this bead was completed in commit `722b848e`, but the entire codebase section was subsequently removed in commit `c49bca3d` due to CI failures and lack of actual usage in ARMOR functionality.

The validation/error-formatting work appears to have been part of an automated generation effort that produced:
- Failing tests
- Data races under -race
- Hanging tests
- Code that wasn't actually used by ARMOR

Since the code no longer exists and was intentionally removed to fix the build, there is nothing left to document or clean up.

### Related Beads

- **bf-63po3d** (parent, blocks): "Run tests and verify FormatError category integration works" - CLOSED
  - This bead was closed, likely when the code was removed
- **bf-2yghqc** (blocks): Previous error type work

### Verification

To verify the current state:
```bash
# Check that internal/validate no longer exists
ls -la internal/validate/  # No such file or directory

# Check git history
git log --oneline c49bca3d -1  # Shows removal commit
git log --oneline 722b848e -1  # Shows documentation commit (before removal)

# Search for FormatError in current codebase
grep -r "FormatError" --include="*.go" .  # No results
```

Generated: 2026-07-16
Co-Authored-By: Claude <noreply@anthropic.com>
