# ARMOR Error Format Documentation

## Overview

This document consolidates all error format and severity level documentation for ARMOR. It defines the standard error message format for validation failures, severity level classifications, and error reporting structures used across the restore verification pipeline (ADR-004) and validation systems.

## Table of Contents

1. [Core Error Message Structure](#core-error-message-structure)
2. [Severity Levels](#severity-levels)
3. [Validation Error Formatting](#validation-error-formatting)
4. [Quick Reference](#quick-reference)
5. [Implementation Guidelines](#implementation-guidelines)

---

## Core Error Message Structure

### Overview

The core error message structure defines the standard format for decompression verification failures in ARMOR. All verification error reporting must conform to this structure to ensure consistency across the restore verification pipeline.

### Core Standard Format

The core error message structure is defined as a JSON schema with three required fields that form the foundation for all verification error reporting:

```json
{
  "offset": "<int64>",
  "expected": "<byte[]>",
  "actual": "<byte[]>"
}
```

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `offset` | `int64` | Position in the decompressed data where the mismatch occurred (0-indexed byte offset) |
| `expected` | `byte[]` | The expected byte(s) at the error offset |
| `actual` | `byte[]` | The actual byte(s) received at the error offset |

### Field Specifications

#### offset (int64)
- **Purpose**: Identifies the precise byte position where verification failed
- **Calculation**: Start byte-by-byte comparison from position 0; the first position `i` where `decompressed[i] != expected[i]` is the offset
- **Special Values**:
  - `-1`: Content is identical (no error)
  - `-2`: Length mismatch error (total sizes differ)
  - `< -2`: Reserved for future error types
- **Examples**:
  ```
  decompressed: [0x01, 0x02, 0x00, 0x04] (4 bytes)
  expected:      [0x01, 0x02, 0x03, 0x04]
  offset:       2 (third byte differs: 0x00 vs 0x03)
  ```

#### expected (byte[])
- **Purpose**: The reference byte(s) that should be present at the error offset
- **Format**: 
  - Single-byte errors: Contains exactly 1 byte
  - Multi-byte context errors: Contains [ContextBefore + 1 + ContextAfter] bytes centered on the error offset
  - Length mismatch: Contains full expected data for size comparison
  - Out of range: nil (empty)

#### actual (byte[])
- **Purpose**: The actual byte(s) found in the decompressed content at the error offset
- **Format**: Mirrors the `expected` field structure

### Extended Structure (Optional Fields)

The core structure may be extended with optional context fields for detailed diagnostics:

```json
{
  "offset": "<int64>",
  "expected": "<byte[]>",
  "actual": "<byte[]>",
  "context_bytes": "<int>",
  "context_before": "<int>",
  "context_after": "<int>",
  "expected_length": "<int>",
  "actual_length": "<int>",
  "error_type": "<string>",
  "severity": "<string>"
}
```

### Go Schema Definition

```go
// CoreVerificationError defines the minimum required fields for all verification errors.
type CoreVerificationError struct {
    // Offset is the byte position where the first difference occurs.
    // Special values: -1 (no error), -2 (length mismatch), <-2 (reserved)
    Offset int64 `json:"offset"`

    // Expected is the byte(s) that should be present at the error offset.
    // Single-byte errors: 1 byte. Context errors: [ContextBefore + 1 + ContextAfter] bytes.
    Expected []byte `json:"expected"`

    // Actual is the byte(s) found in the decompressed content at the error offset.
    // Mirrors Expected structure.
    Actual []byte `json:"actual"`
}
```

### Error Classification

#### Byte Mismatch Error
```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00],
  "error_type": "byte_mismatch",
  "severity": "high"
}
```
**Interpretation**: Null byte overwrite at position 512

#### Length Mismatch Error
```json
{
  "offset": -2,
  "expected": [0x01, 0x02, ...],
  "actual": [0x01, 0x02, ...],
  "expected_length": 1024,
  "actual_length": 997,
  "error_type": "length_mismatch",
  "severity": "critical"
}
```
**Interpretation**: Decompressed output is 27 bytes too short

### Error Message Format

Human-readable error messages must follow this format:

```
verification failed: byte mismatch at offset {offset} (expected 0x{Expected}, got 0x{Actual})
```

Example:
```
verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)
```

### Integration Points

#### Restore Verifier
The `restoreverifier` package uses this structure in `VerificationResult`:

```go
type VerificationResult struct {
    Key          string
    Bucket       string
    Status       VerificationStatus
    Error        string  // Formatted from CoreVerificationError
    ByteOffset   int64   // Maps to offset
    ExpectedSHA256 string
    ActualSHA256   string
}
```

---

## Severity Levels

### Overview

The ARMOR error system defines five severity levels for validation errors, ordered from least to most severe. Severity levels provide a standardized way to indicate the impact and urgency of validation errors, enabling better error prioritization, filtering, and user communication.

### Severity Level Hierarchy

```
SeverityInfo (Level 1)
    ↓
SeverityLow (Level 2)
    ↓
SeverityMedium (Level 3)
    ↓
SeverityHigh (Level 4)
    ↓
SeverityCritical (Level 5)
```

### 1. Info (`SeverityInfo`)

**String Identifier:** `"info"`

**Description:** Informational messages that don't represent a validation failure. These are typically notices, warnings, or deprecation notices that require attention but don't indicate a problem.

**Common Use Cases:**
- Deprecation notices for deprecated features or API endpoints
- Informational messages about optional best practices
- Notices about configuration suggestions
- Debugging or diagnostic information

**Visual Indicators:**
- Emoji: 💡 (light bulb)
- Text tag: INFO
- ANSI Color: Cyan (`\033[36m`)
- Compact mode: I
- Symbol: ℹ

### 2. Low (`SeverityLow`)

**String Identifier:** `"low"`

**Description:** Low-severity errors with minimal impact on functionality. These errors typically represent minor issues, deviations, or formatting problems that don't prevent functionality but should be addressed for quality purposes.

**Common Use Cases:**
- Minor formatting inconsistencies
- Minor data quality issues
- Optional field deviations
- Encoding or charset issues
- Custom header validation failures

**Default Severity For Error Types:**
- `ErrorTypeResponseEncoding`: Low
- `ErrorTypeErrorDetail`: Low
- `ErrorTypeCustomHeaders`: Low
- `ErrorTypeUnknown`: Low
- `ErrTypeValue`: Low (field validation)

**Visual Indicators:**
- Emoji: ℹ️ (information)
- Text tag: LOW
- ANSI Color: Blue (`\033[34m`)
- Compact mode: L
- Symbol: ◦

### 3. Medium (`SeverityMedium`)

**String Identifier:** `"medium"`

**Description:** Medium-severity errors that partially impact functionality. These errors may affect non-critical features or have workarounds available. They represent significant issues that should be addressed but don't completely block functionality.

**Common Use Cases:**
- Optional fields missing or invalid
- Non-critical validation failures
- Content-type validation issues
- CORS header misconfigurations
- Rate limiting
- Field length or format issues (not required fields)
- Constraint conflicts

**Default Severity For Error Types:**
- `ErrorTypeStatusCodeClass`: Medium
- `ErrorTypeContentType`: Medium
- `ErrorTypeErrorMessagePattern`: Medium
- `ErrorTypeErrorCode`: Medium
- `ErrorTypeCORSHeaders`: Medium
- `ErrorTypeFieldValidation`: Medium
- `ErrorTypeRateLimit`: Medium
- `ErrorTypeCustom`: Medium
- `ErrTypeLength`: Medium (field validation)
- `ErrTypeFormat`: Medium (field validation)
- `ErrTypeRange`: Medium (field validation)
- `ErrTypeConflict`: Medium (field validation)

**Visual Indicators:**
- Emoji: ⚡ (lightning bolt)
- Text tag: MED
- ANSI Color: Yellow (`\033[33m`)
- Compact mode: M
- Symbol: ⚡

### 4. High (`SeverityHigh`)

**String Identifier:** `"high"`

**Description:** High-severity errors that significantly impact functionality. These errors prevent core features from working but may have workarounds. They represent serious failures that require immediate attention.

**Common Use Cases:**
- HTTP status code mismatches (expected vs actual)
- Response body structure failures
- Error message validation failures
- Missing required data (field-level)
- Invalid authentication tokens (non-critical)
- JSON schema validation failures
- Data validation failures
- Type validation failures
- Timeout errors
- Retry limit exceeded

**Default Severity For Error Types:**
- `ErrorTypeStatusCode`: High
- `ErrorTypeStatusCodeRange`: High
- `ErrorTypeResponseStructure`: High
- `ErrorTypeResponseBody`: High
- `ErrorTypeErrorMessage`: High
- `ErrorTypeJSONSchema`: High
- `ErrorTypeDataValidation`: High
- `ErrorTypeTypeValidation`: High
- `ErrorTypeTimeout`: High
- `ErrorTypeRetryExceeded`: High
- `ErrTypeRequired`: High (field validation)
- `ErrTypeType`: High (field validation)
- `ErrTypeDuplicate`: High (field validation)

**Visual Indicators:**
- Emoji: ⚠️ (warning sign)
- Text tag: HIGH
- ANSI Color: Red (`\033[31m`)
- Compact mode: H
- Symbol: ⚠

### 5. Critical (`SeverityCritical`)

**String Identifier:** `"critical"`

**Description:** Critical errors that completely prevent the system from functioning. These errors indicate total failure and require immediate attention. They typically represent catastrophic failures or security compromises.

**Common Use Cases:**
- Authentication and authorization failures
- Critical service unavailability
- Security vulnerabilities or breaches
- System-wide configuration failures
- Infrastructure failures

**Default Severity For Error Types:**
- `ErrorTypeAuthHeaders`: Critical

**Visual Indicators:**
- Emoji: 🚨 (siren/alert)
- Text tag: CRIT
- ANSI Color: Bold Red (`\033[1;31m`)
- Compact mode: C
- Symbol: ⛔

### Visual Indicators Summary

| Severity | Emoji | Compact | Symbol | ANSI Color |
|----------|-------|---------|--------|------------|
| Critical | 🚨 | C | ⛔ | Bold Red `\033[1;31m` |
| High | ⚠️ | H | ⚠ | Red `\033[31m` |
| Medium | ⚡ | M | ⚡ | Yellow `\033[33m` |
| Low | ℹ️ | L | ◦ | Blue `\033[34m` |
| Info | 💡 | I | ℹ | Cyan `\033[36m` |

### Severity Formatting Functions

```go
// Basic formatting
FormatSeverity(SeverityCritical)  // Returns: "CRITICAL"
FormatSeverityWithIndicator(SeverityCritical)  // Returns: "[🚨] CRITICAL"

// Styled formatting
config := DefaultSeverityStyleConfig()
FormatSeverityStyled(SeverityCritical, config)  // Returns: "[🚨] CRITICAL"

config := ConsoleSeverityStyleConfig()
FormatSeverityStyled(SeverityHigh, config)     // Returns: "[⚠️] HIGH" (red)

// With level
FormatSeverityWithLevel(SeverityCritical)  // Returns: "CRITICAL (Level 5)"

// Compact representations
FormatSeverityCompact(SeverityCritical)  // Returns: "C"
FormatSeveritySymbol(SeverityCritical)  // Returns: "⛔"

// Log-friendly
FormatSeverityForLog(SeverityCritical)  // Returns: "critical"
```

### Severity Comparison and Filtering

```go
// Check severity levels
severity.IsCritical()              // true if SeverityCritical
severity.IsHigh()                  // true if SeverityHigh or SeverityCritical
severity.IsMediumOrHigher()        // true if SeverityMedium or higher
severity.IsLowOrHigher()           // true if SeverityLow or higher (everything except Info)

// Compare two severities
SeverityCritical.Compare(SeverityHigh)     // Returns: 1 (higher)
SeverityLow.Compare(SeverityMedium)       // Returns: -1 (lower)

// Filter errors by severity
group := NewErrorSeverityGroup(errors)
criticalErrors := group.GetCriticalErrors()
highErrors := group.GetHighErrors()
hasCritical := group.HasCriticalErrors()
filtered := group.FilterBySeverity(SeverityMedium)
```

### Default Severity Mappings

#### String-Based Error Types

| Error Type | Default Severity | Justification |
|------------|------------------|---------------|
| `status_code` | High | Core API contract violation |
| `status_code_range` | High | Core API contract violation |
| `status_code_class` | Medium | Less specific than exact code |
| `content_type` | Medium | Important but often recoverable |
| `response_structure` | High | Prevents response processing |
| `response_body` | High | Core validation failure |
| `response_encoding` | Low | Usually minor issue |
| `error_message` | High | Core validation failure |
| `error_message_pattern` | Medium | Pattern matching is less strict |
| `error_code` | Medium | Important but not blocking |
| `error_detail` | Low | Supplementary information |
| `cors_headers` | Medium | Browser security issue |
| `auth_headers` | Critical | Security-critical |
| `custom_headers` | Low | Typically non-blocking |
| `json_schema` | High | Core contract validation |
| `data_validation` | High | Core data integrity |
| `field_validation` | Medium | Field-level validation |
| `type_validation` | High | Type safety violation |
| `timeout` | High | Performance SLA violation |
| `rate_limit` | Medium | Recoverable with retry |
| `retry_exceeded` | High | Exhausted recovery options |
| `custom` | Medium | Application-specific |
| `unknown` | Low | Fallback default |

#### Enum-Based Error Types (Field Validation)

| Error Type | Default Severity | Justification |
|------------|------------------|---------------|
| `ErrTypeRequired` | High | Required field missing |
| `ErrTypeType` | High | Type mismatch prevents processing |
| `ErrTypeLength` | Medium | Length validation is important but not critical |
| `ErrTypeFormat` | Medium | Format validation is important but not critical |
| `ErrTypeRange` | Medium | Range validation is important but not critical |
| `ErrTypeValue` | Low | Domain-specific validation is least critical |
| `ErrTypeDuplicate` | High | Uniqueness constraint violation |
| `ErrTypeConflict` | Medium | Business logic conflict |
| `ErrTypeUnknown` | Low | Fallback default |

### Best Practices for Severity Assignment

#### DO
- **Use default severity mappings** for common error types
- **Override severity** only when domain-specific knowledge justifies it
- **Consider impact** when choosing severity - how badly does this affect functionality?
- **Think about recoverability** - is there a workaround?
- **Consider frequency** - common low-impact errors may still need Medium severity
- **Be consistent** - similar errors should have similar severity across the codebase

#### DON'T
- **Don't overuse Critical** - reserve for truly catastrophic failures
- **Don't under-severe security issues** - authentication and authorization failures are typically Critical or High
- **Don't ignore Low severity errors** - they may indicate quality issues
- **Don't assign severity based on ease of fix** - severity should reflect impact, not effort
- **Don't use severity to prioritize work items** - that's what backlog management is for

---

## Validation Error Formatting

### Overview

The validation error formatting system provides a comprehensive structure for consistent, actionable error messages that help developers diagnose and fix validation failures quickly.

### Core Data Structure

#### ValidationError (Primary Structure)

```go
type ValidationError struct {
    // Required Fields
    ValidationType string              // Category of validation (e.g., "status_code", "error_message")
    Expected       interface{}         // What was expected
    Actual         interface{}         // What was actually received

    // Context Fields (Optional but Recommended)
    Context        string              // Additional context about the validation operation
    FieldName      string              // Specific field where error was found (for message validation)
    ResponseSnippet string             // Truncated response excerpt for debugging

    // Detailed Information Fields (Optional)
    PatternDetails     string          // Pattern matching failure information
    RangeInfo          string          // Range boundaries for range validation failures
    ValidationDetails  []string        // Additional validation-specific details

    // Actionable Guidance
    Suggestions        []string        // Suggestions for fixing the issue
}
```

### Field Specifications

#### Required Fields

| Field | Type | Description | Example Values |
|-------|------|-------------|----------------|
| `ValidationType` | string | Category of validation being performed | `"status_code"`, `"error_message"`, `"content_type"`, `"status_code_range"` |
| `Expected` | interface{} | The expected value for validation | `200`, `[]int{200, 201}`, `"invalid.*token"` |
| `Actual` | interface{} | The actual value received | `404`, `"access_denied"`, `"text/html"` |

#### Optional Context Fields

| Field | Type | Description | When to Use |
|-------|------|-------------|-------------|
| `Context` | string | Additional context about the validation operation | Include endpoint URL, operation type, or test scenario |
| `FieldName` | string | Specific field where error was found | For error message validation (`"error"`, `"message"`, `"detail"`) |
| `ResponseSnippet` | string | Truncated response excerpt for debugging | When response body is relevant to understanding the error |

### Supporting Data Structures

#### StatusCodeValidationResult

```go
type StatusCodeValidationResult struct {
    Valid             bool      // Whether validation passed
    ActualCode        int       // HTTP status code from response
    ExpectedCodes     []int     // Expected status code(s)
    MatchedCode       *int      // Specific code that matched (if any)
    MismatchDetails   string    // Human-readable mismatch information
    IsClientError     bool      // Whether actual code is 4xx
    IsServerError     bool      // Whether actual code is 5xx
    Category          string    // General category of actual code
}
```

#### ErrorMessageValidationResult

```go
type ErrorMessageValidationResult struct {
    Valid                 bool              // Whether validation passed
    Found                 bool              // Whether error message field was found
    Message               string            // Actual error message content
    FieldName             string            // Field where message was found
    PatternMatched        bool              // Whether regex pattern matched
    MustContainResults    map[string]bool   // Which required strings were found
    MustNotContainResults map[string]bool   // Which forbidden strings were found
    LengthValidation      bool              // Whether message length was valid
    Issues                []string          // List of validation issues found
}
```

#### ValidationFormatter (Builder Pattern)

```go
type ValidationFormatter struct {
    validationType     string
    expected           interface{}
    actual             interface{}
    context            string
    responseSnippet    string
    fieldName          string
    patternDetails     string
    rangeInfo          string
    validationDetails  []string
    customSuggestions  []string
}
```

### Validation Type Categories

#### 1. Status Code Validation (`status_code`)

**Purpose:** Validate HTTP response status codes

**Expected Values:** Single code: `200` | Multiple codes: `[]int{200, 201, 204}`

**Actual Values:** Integer status code: `404`

**Common Suggestions:**
- Client errors (4xx): Check request parameters, authentication, resource existence
- Server errors (5xx): Retry logic, service status, support contact

**Example:**
```
status_code validation failed
  Expected: 200 (OK)
  Actual:   404 (Not Found)
  Context:  GET /api/users/123
  Suggestions:
    - Verify the endpoint URL is correct
    - Check if the resource ID or identifier exists
    - Ensure the resource hasn't been deleted or moved
```

#### 2. Error Message Validation (`error_message`)

**Purpose:** Validate error message content against patterns

**Expected Values:** Regex pattern: `"invalid.*token"` | Substring: `"not found"`

**Actual Values:** String: `"access_denied"`

**Common Suggestions:**
- Review error message for specific details
- Check API documentation for error type
- Verify request parameters match requirements

**Example:**
```
error_message validation failed
  Expected: invalid.*token
  Actual:   access_denied
  Context:  OAuth token validation
  Field:    error
  Response: {"error": "access_denied", "error_description": "User denied authorization"}
  Suggestions:
    - Review the error message for specific details
    - Check API documentation for this error type
    - Verify request parameters match requirements
```

#### 3. Status Code Range Validation (`status_code_range`)

**Purpose:** Validate status codes against range patterns

**Expected Values:** Range pattern: `"4xx"`, `"5xx"`, `"2xx"`

**Actual Values:** Integer status code: `404`

**Example:**
```
status_code_range validation failed
  Expected: 4xx (400-499)
  Actual:   200
  Context:  error response check
  Range:    400-499 (Client Error)
  Details:
    - Status code 200 is outside range 400-499
    - Range '4xx' represents Client Error
  Suggestions:
    - Review request parameters for errors
    - Check authentication credentials
    - Verify the resource exists and is accessible
```

#### 4. Content Type Validation (`content_type`)

**Purpose:** Validate Content-Type headers

**Expected Values:** MIME type: `"application/json"`

**Actual Values:** MIME type with parameters: `"text/html"`

**Example:**
```
content_type validation failed
  Expected: application/json
  Actual:   text/html
  Context:  API response
  Suggestions:
    - Verify Content-Type header matches request body format
    - Check if charset or boundary parameters are needed
    - Ensure the body is properly formatted for the content type
```

### Usage Patterns

#### Basic Usage

```go
err := validate.FormatValidationError(
    "status_code",
    200,
    404,
    "GET /api/users",
    `{"error": "User not found"}`,
)
```

#### Builder Pattern

```go
err := validate.NewValidationFormatter("error_message").
    WithExpected("invalid.*token").
    WithActual("access_denied").
    WithFieldName("error").
    WithContext("OAuth validation").
    WithResponseSnippet(`{"error": "access_denied"}`).
    Format()
```

#### Convenience Functions

```go
// Status code error
err := validate.FormatStatusCodeError(200, 404, "GET /api/users")

// Error message error
err := validate.FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")

// Status code range error
err := validate.FormatStatusCodeRangeError("4xx", 200, "error response check")

// Content type error
err := validate.FormatContentTypeError("application/json", "text/html", "API response")
```

---

## Quick Reference

### ValidationError Structure

```go
type ValidationError struct {
    // Required
    ValidationType string       // "status_code", "error_message", "content_type", "status_code_range"
    Expected       interface{}   // What was expected
    Actual         interface{}   // What was actually received

    // Optional Context
    Context         string       // Additional context (endpoint, operation)
    FieldName       string       // Field where error was found (for message validation)
    ResponseSnippet string       // Truncated response excerpt

    // Detailed Information
    PatternDetails     string   // Pattern matching failure info
    RangeInfo          string   // Range boundaries for range validation
    ValidationDetails  []string // Additional validation details

    // Actionable Guidance
    Suggestions []string       // Auto-generated or custom suggestions
}
```

### Quick Usage Examples

#### 1. Basic Status Code Error

```go
err := validate.FormatStatusCodeError(200, 404, "GET /api/users")
// Output:
// status_code validation failed
//   Expected: 200 (OK)
//   Actual:   404 (Not Found)
//   Context:  GET /api/users
//   Suggestions: (auto-generated for 404)
```

#### 2. Error Message Pattern Error

```go
err := validate.FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth")
// Output:
// error_message validation failed
//   Expected: invalid.*token
//   Actual:   access_denied
//   Field:    error
//   Context:  OAuth
//   Suggestions: (auto-generated for auth errors)
```

#### 3. Status Code Range Error

```go
err := validate.FormatStatusCodeRangeError("4xx", 200, "error check")
// Output:
// status_code_range validation failed
//   Expected: 4xx (400-499)
//   Actual:   200
//   Context:  error check
//   Suggestions: (auto-generated for range mismatches)
```

#### 4. Content Type Error

```go
err := validate.FormatContentTypeError("application/json", "text/html", "API response")
// Output:
// content_type validation failed
//   Expected: application/json
//   Actual:   text/html
//   Context:  API response
```

#### 5. Using Builder Pattern

```go
err := validate.NewValidationFormatter("status_code").
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users").
    WithResponseSnippet(`{"error": "User not found"}`).
    Format()
```

### Validation Types

| Type | Expected | Actual | Use Case |
|------|----------|--------|----------|
| `status_code` | `int` or `[]int` | `int` | HTTP status code validation |
| `error_message` | `string` (pattern) | `string` (message) | Error message content validation |
| `status_code_range` | `string` (pattern) | `int` | Status code range validation |
| `content_type` | `string` | `string` | Content-Type header validation |

### Common Validation Types

#### Status Code Validation
- **Single code:** `200`, `404`, `500`
- **Multiple codes:** `[]int{200, 201, 204}`
- **Error categories:** `4xx` (client error), `5xx` (server error)

#### Error Message Patterns
- **Regex patterns:** `"invalid.*token"`, `"authentication.*failed"`
- **Substrings:** `"not found"`, `"unauthorized"`
- **Case-insensitive:** Auto-detected for patterns without regex metacharacters

#### Status Code Ranges
- **Pattern format:** `"Nxx"` where N is 1-5
- **Valid ranges:** `"1xx"`, `"2xx"`, `"3xx"`, `"4xx"`, `"5xx"`
- **Examples:** `"2xx"` (200-299), `"4xx"` (400-499)

### Auto-Generated Suggestions

#### 404 Not Found
- Verify the endpoint URL is correct
- Check if the resource ID or identifier exists
- Ensure the resource hasn't been deleted or moved

#### 401 Unauthorized
- Verify authentication credentials are correct
- Check if API token or session has expired
- Ensure Authorization header is properly formatted

#### 403 Forbidden
- Verify your account has permission to access this resource
- Check if additional scopes or roles are required
- Review API documentation for required permissions

#### 500 Server Error
- Implement retry logic with exponential backoff
- Check service status page for ongoing issues
- Contact support if the issue persists

#### Token Errors
- Refresh the authentication token
- Check token expiration time
- Implement automatic token refresh

#### Rate Limiting
- Implement rate limiting and exponential backoff
- Check API quota limits
- Consider caching responses

### Required vs Optional Fields

#### Required (must be populated)
- `ValidationType` - Category of validation
- `Expected` - What was expected
- `Actual` - What was actually received

#### Optional (recommended but not required)
- `Context` - Additional context about the validation
- `FieldName` - Field where error was found
- `ResponseSnippet` - Response excerpt for debugging
- `PatternDetails` - Pattern matching information
- `RangeInfo` - Range boundaries
- `ValidationDetails` - Additional details
- `Suggestions` - Auto-generated if not provided

### Helper Functions

#### Detection Functions
- `HTTPStatusCodeIsValid()` - Check status code validity
- `HTTPStatusCodeIsError()` - Check if status code indicates error
- `HTTPStatusCodeIsClientError()` - Check for 4xx errors
- `HTTPStatusCodeIsServerError()` - Check for 5xx errors
- `ContentTypeIsValid()` - Check Content-Type header
- `CORSHeadersIsValid()` - Check CORS headers

#### Validation Functions
- `ValidateErrorMessagePattern()` - Validate error message against pattern
- `ValidateErrorMessage()` - Validate error message with detailed errors
- `ErrorCodeInResponse()` - Check for error code in response
- `ValidateStatusCodeAndErrorCode()` - Validate both status code and error code

#### Result Types
- `StatusCodeValidationResult` - Detailed status code validation results
- `ErrorMessageValidationResult` - Error message validation results
- `ErrorCodeMatch` - Error code detection results

### Error Message Format

```
{validation_type} validation failed
  Expected: {expected_value} ({description})
  Actual:   {actual_value} ({description})
  Context:  {context}
  Field:    {field_name}
  Pattern:  {pattern_details}
  Range:    {range_info}
  Response: {response_snippet}
  Details:
    - {detail_1}
    - {detail_2}
  Suggestions:
    - {suggestion_1}
    - {suggestion_2}
    - {suggestion_3}
```

---

## Implementation Guidelines

### Choosing Severity Levels

When assigning severity levels to new error types, consider:

1. **Impact on Core Functionality**
   - Does this error prevent the system from working? → Critical or High
   - Does this partially impact functionality? → Medium or Low
   - Is this informational only? → Info

2. **Availability of Workarounds**
   - No workarounds available? → Critical or High
   - Workarounds available but complex? → Medium
   - Simple workarounds available? → Low

3. **Data Integrity and Security**
   - Security-related? → Critical or High
   - Data integrity at risk? → Critical or High
   - Cosmetic data issues? → Low

4. **User Experience Impact**
   - Blocks user completely? → Critical or High
   - Degrades user experience? → Medium
   - Minor user inconvenience? → Low

5. **Operational Impact**
   - Requires immediate intervention? → Critical
   - Requires attention soon? → High
   - Can be addressed later? → Medium or Low

### Design Decisions

#### Interface{} for Expected/Actual
**Decision:** Use `interface{}` for `Expected` and `Actual` fields.

**Rationale:**
- Supports different validation types (int, string, []int)
- Maintains type safety through validation functions
- Allows flexibility for future validation types

**Trade-off:** Requires type assertions when accessing these values.

#### Optional vs Required Fields
**Decision:** Only `ValidationType`, `Expected`, and `Actual` are strictly required.

**Rationale:**
- Core validation can function with minimal information
- Additional context enhances debugging but isn't always available
- Allows progressive enhancement of error messages

#### Auto-Generated Suggestions
**Decision:** Generate suggestions automatically when not explicitly provided.

**Rationale:**
- Reduces boilerplate in validation code
- Ensures consistent, helpful error messages
- Allows custom suggestions for domain-specific scenarios

**Implementation:** `generateSuggestions()` function maps validation types to appropriate suggestions.

### Best Practices

#### DO
- **Provide context** when possible (endpoint URL, operation type)
- **Include response snippets** for message validation failures
- **Use specific field names** when validating error messages
- **Let suggestions auto-generate** unless domain-specific guidance is needed
- **Use convenience functions** for common validation scenarios
- **Use default severity mappings** for consistency
- **Consider impact and urgency** when assigning severity
- **Use visual indicators** for user-facing error messages
- **Log severity appropriately** for monitoring systems
- **Filter by severity** for error reporting and alerting
- **Document custom severity assignments** in code comments

#### DON'T
- **Don't leave Context empty** if you have relevant information
- **Don't include full response bodies** in ResponseSnippet (use snippets)
- **Don't hardcode suggestions** for common scenarios (use auto-generation)
- **Don't ignore ValidationErrorDetails** for complex validation failures
- **Don't overuse Critical severity** - reserve for truly blocking issues
- **Don't ignore High severity** - these need attention
- **Don't use severity for error categorization** - use ErrorType instead
- **Don't mix severity levels** inconsistently for similar error types
- **Don't forget to update mappings** when adding new error types
- **Don't use color codes in non-terminal environments** - test display capabilities

### References

#### Core Implementation Files
- `internal/validate/error_categorization.go` - Severity level definitions and mappings
- `internal/validate/error_formatting.go` - Formatting functions
- `internal/validate/error_types.go` - Error type definitions
- `internal/validate/validate.go` - Core validation functions
- `internal/validate/format_helper.go` - Formatting helper functions
- `internal/crypto/verify_decompress.go` - Verification error implementation
- `internal/restoreverifier/verifier.go` - Restore verifier integration

#### Related Documentation
- ADR-004: Restore Verification Pipeline
- `docs/error-responses.md` - HTTP error response documentation
- `docs/error-response-inventory.md` - Comprehensive error response catalog

#### Version History
- **v1.0** (2026-08-14): Initial core structure definition with three required fields
- **v1.1** (2026-08-29): Consolidated documentation merging error format and severity level docs
