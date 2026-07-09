# bf-27bno: Fix test compilation error in parser_test.go

## Investigation
The `fmt` import was already present on line 5 of `internal/yamlutil/parser_test.go`.

## Root Cause
This was already fixed in commit `cbf45e7` (bead bf-4lqn4).

## Verification
- `go test ./internal/yamlutil/... -c` compiles successfully with no errors
- Import block on lines 4-9 includes `"fmt"` on line 5
- Test using `fmt.Errorf` at line 622 runs successfully

## Status
No changes needed - issue already resolved.
