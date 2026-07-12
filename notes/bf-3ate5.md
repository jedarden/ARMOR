# Bead bf-3ate5: TypeMismatchError Replacement in debug_helpers_test.go

## Status: ALREADY COMPLETED

This bead's objective was to replace the single TypeMismatchError direct struct initialization in `internal/yamlutil/debug_helpers_test.go` with the `NewTypeMismatchError()` constructor.

## Discovery

The work was already completed in commit `426fd0d0` from bead `bf-3sh3e`:

```
fix(bf-3sh3e): replace TypeMismatchError struct initialization with NewTypeMismatchError constructor
```

## Verification

1. **File compiles:** `go build ./internal/yamlutil/...` ✓
2. **Tests pass:** `go test ./internal/yamlutil/...` ✓
3. **No direct struct initializations remain:** grep confirmed 0 instances of `&TypeMismatchError{` or `TypeMismatchError{`

## Current State (Line 846)

```go
func TestTypeMismatchError(t *testing.T) {
	err := NewTypeMismatchError("", "server.port", "int", "string", "", 0, "")
	// The actual format is "type mismatch in <filepath>, field <fieldpath>: expected <expected>, got <actual>"
	expected := "type mismatch in , field server.port: expected int, got string"
	if err.Error() != expected {
		t.Errorf("expected error message %q, got %q", expected, err.Error())
	}
}
```

The task was to ensure all 7 parameters are passed correctly to `NewTypeMismatchError()`, and they are:
1. `filePath` (empty string)
2. `fieldPath` ("server.port")
3. `expectedType` ("int")
4. `actualType` ("string")
5. `value` (empty string)
6. `line` (0)
7. `yamlValue` (empty string)

## Acceptance Criteria Met

- ✅ The single TypeMismatchError construction replaced with NewTypeMismatchError()
- ✅ File compiles
- ✅ Tests pass
- ✅ No test logic changed
