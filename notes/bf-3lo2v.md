# Bead bf-3lo2v: FileError.Error() Already Fixed

## Finding

The issue described in bead bf-3lo2v has **already been resolved**.

## Evidence

1. **Commit 99ed1772** (2026-07-11 15:13:17):
   ```
   fix(yamlutil): Include underlying error message in FileError.Error()
   ```

2. **Current Implementation** (`internal/yamlutil/errors.go:533-535`):
   ```go
   if fe.Err != nil {
       sb.WriteString(fmt.Sprintf(": %s", fe.Err.Error()))
   }
   ```

3. **Test Results**: All 4 FileError tests passing:
   - ✅ error_message_format
   - ✅ error_message_without_underlying_error
   - ✅ unwrap_returns_underlying_error
   - ✅ unwrap_with_nil_underlying_error

## What Was Fixed

The fix changed `FileError.Error()` to:
- Use `strings.Builder` for efficient string concatenation
- Include the underlying error message (`fe.Err.Error()`) when present
- Maintain backward compatibility (when `Err` is nil, behavior unchanged)

## Example Output

```go
// Before: "file error during read on /test/file.yaml"
// After:  "file error during read on /test/file.yaml: file does not exist"
```

This provides much better debugging context by including the OS-level error message.

## Pattern

This follows the same pattern as bead `bf-ck666` (GetRequiredInt string parsing), where the functionality was already implemented but a tracking bead was created before verification.
