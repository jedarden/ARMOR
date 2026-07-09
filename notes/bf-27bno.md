# Bead bf-27bno: Fix test compilation error in parser_test.go

## Investigation

The task description mentioned:
- Line 621 uses fmt.Errorf but the fmt package is not imported in parser_test.go

## Actual Findings

1. **parser_test.go already has `fmt` imported** on line 5 - this was not the issue

2. **The actual compilation error** was a misplaced `// +build ignore` comment in `internal/yamlutil/future.go`:
   - Line 1 had: `//go:build ignore`
   - Line 8 had: `// +build ignore` (misplaced after other comments)
   - This caused the error: `internal/yamlutil/future.go:8:1: misplaced +build comment`

## Resolution

The compilation error was already fixed in commit `8d0802b`:
```
fix(yamlutil): Remove misplaced legacy +build comment from future.go
```

The `// +build ignore` line was removed, leaving only the modern `//go:build ignore` directive.

## Verification

```bash
go build ./internal/yamlutil/...
# Success - no output, no errors
```

The package compiles successfully. The original task description appears to have been based on an outdated version of the code or a misdiagnosis of the actual issue.

## Notes

- `parser_test.go` line 621 exists and uses `fmt.Errorf`, but the `fmt` import is present on line 5
- The build constraint syntax requires `// +build` comments to appear immediately after `//go:build` comments, not separated by other content
- Since Go 1.17, the preferred approach is to use only `//go:build` comments
