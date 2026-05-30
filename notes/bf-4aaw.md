# 'which test' Behavior on Bash

## Test Environment
- Shell: bash
- Platform: Linux (Debian-based)
- Date: 2026-05-30

## Core Findings

### 1. `which test` Output
```
$ which test
/usr/bin/test
```
- **Exit code:** 0 (success)
- **Returns:** The external `/usr/bin/test` binary path, NOT the shell builtin

### 2. `type test` Output
```
$ type test
test is a shell builtin
```
- **Shows:** test is primarily a shell builtin

### 3. Is `which` a builtin or external command?
```
$ type which
which is /usr/bin/which

$ type -a which
which is /usr/bin/which
which is /bin/which
```
- **Conclusion:** `which` is an **external command** (`/usr/bin/which`), not a shell builtin
- **Note:** Two versions exist in PATH: `/usr/bin/which` and `/bin/which`

### 4. `which -a test` (Find all matches)
```
$ which -a test
/usr/bin/test
/bin/test
```
- **Shows both external binaries:** `/usr/bin/test` and `/bin/test`
- **Does NOT show the shell builtin**

### 5. External test binary details
```
$ ls -la /usr/bin/test
-rwxr-xr-x 1 root root 47528 Jun  4  2025 /usr/bin/test

$ file /usr/bin/test
/usr/bin/test: ELF 64-bit LSB pie executable, x86-64, version 1 (SYSV)
```
- The external `/usr/bin/test` is a real compiled binary (ELF executable)
- Per Debian policy, core utilities like `test` must also exist as standalone executables

### 6. Exit code behavior comparison

**When command exists:**
```
$ which test
$ echo $?
0
```

**When command does NOT exist:**
```
$ which nonexistencommandxxxxx
$ echo $?
1
```

### 7. `command -V test` (Alternative to `type`)
```
$ command -V test
test is a shell builtin
```
- `command -V` behaves like `type` and correctly identifies the builtin

## Summary

| Command | Output | What it finds |
|---------|--------|---------------|
| `which test` | `/usr/bin/test` | External binary only |
| `type test` | `test is a shell builtin` | Shell builtin (correct) |
| `command -V test` | `test is a shell builtin` | Shell builtin (correct) |
| `which -a test` | `/usr/bin/test` + `/bin/test` | All external binaries |

## Key Takeaway

**`which` does NOT know about shell builtins.** It only searches PATH for external executables. When you run `which test`, it finds `/usr/bin/test` but completely ignores the shell builtin `test`. Use `type` or `command -V` instead to see whether a command is a builtin, alias, function, or external command.

This explains why scripts that check for command existence using `which` may incorrectly report that builtins like `test`, `echo`, or `printf` are "not found" if only the external binary is missing (even though the builtin would work fine).
