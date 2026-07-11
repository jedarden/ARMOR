# Bead bf-4pjzs: YAMLError Base Interface Already Defined

## Task
Define YAMLError base interface with Code() and Error() methods, plus documentation.

## Status
**ALREADY COMPLETE** - The YAMLError interface was already defined in `/home/coding/ARMOR/internal/yamlutil/errors.go` (lines 27-42).

## Verification
The existing YAMLError interface meets all acceptance criteria:

### 1. Interface Definition ✅
Located in `internal/yamlutil/errors.go` at lines 27-42.

### 2. Code() Method ✅
```go
Code() ErrorCode
```
Returns `ErrorCode` (which is `type ErrorCode string`), providing machine-readable error codes for programmatic handling.

### 3. Error() Method ✅
The interface embeds the standard `error` interface, which provides the `Error() string` method. All concrete error types (ParseError, ValidationError, FileError, etc.) implement this method.

### 4. Documentation ✅
Comprehensive documentation explains:
- Purpose: "base interface for all YAML processing errors"
- Usage: "common type for all errors that occur during YAML file I/O, parsing, validation, and schema operations"
- Method documentation for Code(), YAMLErrorType(), and Context()

## Additional Features
The interface also includes:
- `YAMLErrorType() ErrorType` - returns error category for type switching
- `Context() string` - returns additional context about the error

## Conclusion
No changes were needed. The bead's requirements were already satisfied by the existing codebase.
