# Error Severity Levels - Design and Implementation Guide

## Overview

This document describes the severity level system for validation error formatting in ARMOR. Severity levels provide a standardized way to indicate the impact and urgency of validation errors, enabling better error prioritization, filtering, and user communication.

## Severity Level Definitions

ARMOR defines five severity levels for validation errors, ordered from least to most severe:

### 1. Info (`SeverityInfo`)

**Definition:** Informational messages that don't represent a failure condition.

**Use Cases:**
- Deprecation notices
- Informational messages about validation behavior
- Debug or trace information
- Non-critical warnings

**Examples:**
- "Using deprecated endpoint /v1/api"
- "Response format may change in future versions"
- "Cache hit for validation check"

**Default Behavior:**
- Logged but does not fail validations
- Can be filtered out in production environments
- No visual emphasis in error display

### 2. Low (`SeverityLow`)

**Definition:** Low-severity errors with minimal impact on functionality.

**Use Cases:**
- Minor formatting issues
- Minor data inconsistencies
- Optional field deviations
- Non-critical validation failures

**Examples:**
- Response encoding differs from expected (UTF-8 vs UTF-16)
- Whitespace differences in text content
- Optional header fields missing
- Minor timestamp discrepancies

**Default Behavior:**
- Logged for visibility
- May be suppressed in high-volume scenarios
- Minimal visual emphasis in error display

### 3. Medium (`SeverityMedium`)

**Definition:** Medium-severity errors that partially impact functionality or have workarounds.

**Use Cases:**
- Optional fields missing or invalid
- Non-critical validation failures
- CORS or custom header issues
- Content type variations
- Rate limiting warnings

**Examples:**
- Optional field `description` exceeds maximum length
- CORS header configuration incomplete
- Content-Type includes charset parameter not expected
- Rate limit approaching threshold

**Default Behavior:**
- Logged and highlighted in error reports
- Triggers alerts in monitoring systems
- Moderate visual emphasis in error display

### 4. High (`SeverityHigh`)

**Definition:** High-severity errors that significantly impact functionality but may have workarounds.

**Use Cases:**
- Core validation failures
- Required data missing
- Invalid authentication tokens (not critical auth failures)
- Response structure mismatches
- Timeout issues

**Examples:**
- Required field `email` is missing
- Authentication token expired
- Response structure doesn't match expected schema
- API endpoint timeout (retriable)
- JSON schema validation failed

**Default Behavior:**
- Immediately visible in error reports
- Triggers alerts and notifications
- Strong visual emphasis in error display
- May trigger automatic retry logic

### 5. Critical (`SeverityCritical`)

**Definition:** Critical errors that prevent the system from functioning and require immediate attention.

**Use Cases:**
- Authentication and authorization failures
- Critical service unavailability
- Data integrity issues
- Security violations
- Non-retriable core failures

**Examples:**
- Authentication credentials invalid
- Authorization failed (forbidden access)
- Critical service completely unavailable
- Security token validation failed
- Data corruption detected

**Default Behavior:**
- Maximum visibility in all systems
- Triggers immediate alerts and paging
- Maximum visual emphasis in error display
- Blocks critical operations
- Requires manual intervention

## Severity Level Hierarchy

```
SeverityInfo (Level 0)
    ↓
SeverityLow (Level 1)
    ↓
SeverityMedium (Level 2)
    ↓
SeverityHigh (Level 3)
    ↓
SeverityCritical (Level 4)
```

**Numeric Levels:**
- Info: Level 1 (in some contexts), Level 0 (in comparisons)
- Low: Level 2 (in some contexts), Level 1 (in comparisons)
- Medium: Level 3 (in some contexts), Level 2 (in comparisons)
- High: Level 4 (in some contexts), Level 3 (in comparisons)
- Critical: Level 5 (in some contexts), Level 4 (in comparisons)

Note: There are two different level systems in the codebase:
1. Comparison levels (0-4) for `Compare()` method
2. Display levels (1-5) for `FormatSeverityWithLevel()` method

## Formatting Implications

### Visual Indicators

Each severity level has specific visual indicators for different display contexts:

#### Emoji Indicators

| Severity | Indicator | Usage |
|----------|-----------|-------|
| Critical | 🚨 | Alerts, sirens, urgent issues |
| High | ⚠️ | Warnings, attention needed |
| Medium | ⚡ | Medium priority, action required |
| Low | ℹ️ | Informational, minor issues |
| Info | 💡 | Helpful information, notices |
| Unknown | ❓ | Unrecognized severity |

#### Compact Indicators

| Severity | Single Char | Usage |
|----------|-------------|-------|
| Critical | C | Space-constrained displays |
| High | H | Log file indicators |
| Medium | M | Status column displays |
| Low | L | Compact error lists |
| Info | I | Minimal UI elements |
| Unknown | ? | Fallback indicator |

#### Unicode Symbols

| Severity | Symbol | Usage |
|----------|--------|-------|
| Critical | ⛔ | Blocked operations, errors |
| High | ⚠ | Warnings, cautions |
| Medium | ⚡ | Medium priority issues |
| Low | ◦ | Minor issues, informational |
| Info | ℹ | Information, notices |
| Unknown | ? | Unknown severity |

### Color Coding (ANSI)

When color coding is enabled (typically for console output), severity levels use these ANSI color codes:

| Severity | ANSI Color | Hex Code | Usage |
|----------|------------|----------|-------|
| Critical | Bold Red | `\033[1;31m` | Maximum urgency |
| High | Red | `\033[31m` | High urgency |
| Medium | Yellow | `\033[33m` | Medium urgency |
| Low | Blue | `\033[34m` | Low urgency |
| Info | Cyan | `\033[36m` | Informational |
| Unknown | Gray | `\033[90m` | Default/unknown |

**Example colored output:**
```bash
# Critical (bold red)
🚨 CRITICAL: Authentication failed

# High (red)
⚠️ HIGH: Required field missing

# Medium (yellow)
⚡ MEDIUM: Optional field validation failed

# Low (blue)
ℹ️ LOW: Minor formatting issue

# Info (cyan)
💡 INFO: Using deprecated endpoint
```

### Text Formatting

#### Uppercase Labels

Most severity labels are displayed in uppercase for visibility:

```
CRITICAL
HIGH
MEDIUM
LOW
INFO
```

#### Bracketed Format

Severity indicators are often wrapped in brackets:

```
[🚨] CRITICAL
[⚠️] HIGH
[⚡] MEDIUM
[ℹ️] LOW
[💡] INFO
```

#### With Level Indicators

Severity can be displayed with numeric levels:

```
CRITICAL (Level 5)
HIGH (Level 4)
MEDIUM (Level 3)
LOW (Level 2)
INFO (Level 1)
```

### Log Format

For structured logging, severity levels use lowercase format:

```
critical
high
medium
low
info
```

This format is machine-readable and suitable for log aggregation systems.

## Default Severity Mappings

### HTTP Status Code Errors

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `status_code` | High | Core validation failure |
| `status_code_range` | High | Range validation failure |
| `status_code_class` | Medium | Class-level validation less specific |

### Content Validation Errors

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `content_type` | Medium | Content type mismatch has workarounds |
| `response_structure` | High | Structure mismatch breaks parsing |
| `response_body` | High | Body content validation failure |
| `response_encoding` | Low | Encoding issues are usually minor |

### Error Message Errors

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `error_message` | High | Error message validation failure |
| `error_message_pattern` | Medium | Pattern matching is less critical |
| `error_code` | Medium | Error code validation is medium priority |
| `error_detail` | Low | Detail validation is low priority |

### Header Validation Errors

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `cors_headers` | Medium | CORS issues have workarounds |
| `auth_headers` | Critical | Authentication is critical |
| `custom_headers` | Low | Custom header issues are minor |

### Schema and Data Validation Errors

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `json_schema` | High | Schema validation breaks parsing |
| `data_validation` | High | Data validation failures are critical |
| `field_validation` | Medium | Field-level validation is medium priority |
| `type_validation` | High | Type validation failures break processing |

### Performance Errors

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `timeout` | High | Timeouts indicate significant issues |
| `rate_limit` | Medium | Rate limits are expected and retriable |
| `retry_exceeded` | High | Retry exhaustion is serious |

### Enum-Based Error Types

| Error Type | Default Severity | Rationale |
|------------|------------------|-----------|
| `ErrTypeRequired` | High | Missing required fields is critical |
| `ErrTypeType` | High | Type mismatches break processing |
| `ErrTypeLength` | Medium | Length issues are medium priority |
| `ErrTypeFormat` | Medium | Format issues have workarounds |
| `ErrTypeRange` | Medium | Range issues are medium priority |
| `ErrTypeValue` | Low | Value issues are low priority |
| `ErrTypeDuplicate` | High | Duplicate violations are critical |
| `ErrTypeConflict` | Medium | Conflicts are medium priority |

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

### Default Severity Assignment Pattern

```go
// In error_categorization.go
var defaultSeverityForErrorType = map[string]ErrorSeverity{
    // Start with Medium as baseline
    "your_error_type": SeverityMedium,
    
    // Escalate to High if:
    // - Core functionality affected
    // - Data integrity at risk
    // - No simple workaround
    
    // Escalate to Critical if:
    // - Security-related
    // - Complete system failure
    // - Data corruption possible
    
    // Downgrade to Low if:
    // - Cosmetic issue
    // - Optional feature affected
    // - Minor formatting problem
}
```

### Custom Severity Assignment

For domain-specific scenarios, override default severity:

```go
// Create error with custom severity consideration
err := ValidationError{
    ErrorType: "custom_validation",
    Message: "Custom business rule violation",
    // Severity would be determined by GetDefaultSeverityForErrorType
    // or custom logic in your code
}

// In error handling code, you can apply custom severity logic:
if err.ErrorType == "custom_business_critical" {
    // Treat as critical regardless of default
    severity = SeverityCritical
}
```

## Formatting Functions

### Basic Severity Formatting

```go
// Get severity for error type
severity := validate.GetDefaultSeverityForErrorType("status_code")

// Format severity with indicator
label := validate.FormatSeverityWithIndicator(severity)
// Output: "[⚠️] High"

// Format severity with level
label := validate.FormatSeverityWithLevel(severity)
// Output: "HIGH (Level 4)"

// Format severity for logging
label := validate.FormatSeverityForLog(severity)
// Output: "high"
```

### Styled Severity Formatting

```go
// Default style configuration
config := validate.DefaultSeverityStyleConfig()
label := validate.FormatSeverityStyled(severity, config)
// Output: "[🚨] CRITICAL"

// Console style (with colors)
config := validate.ConsoleSeverityStyleConfig()
label := validate.FormatSeverityStyled(severity, config)
// Output: "[🚨] CRITICAL" (with ANSI color codes)

// Compact style
config := validate.CompactSeverityStyleConfig()
label := validate.FormatSeverityStyled(severity, config)
// Output: "C" (for Critical)
```

### Error Formatting with Severity

```go
err := ValidationError{
    ErrorType: "status_code",
    Message: "Expected 200 but got 404",
    Expected: 200,
    Actual: 404,
}

// Format error with severity
formatted := validate.FormatValidationErrorFull(err, true)
// Output: "[⚠️] HIGH [status_code] Expected 200 but got 404"

// Format error by category
formatted := validate.FormatErrorByCategory(err, true)
// Output: "[⚠️] HIGH [HTTP] status_code - Expected 200 but got 404"
```

## Usage Patterns

### Error Filtering by Severity

```go
// Group errors by severity
severityGroup := validate.NewErrorSeverityGroup(errors)

// Get only critical errors
criticalErrors := severityGroup.GetCriticalErrors()

// Get high and critical errors
hasHighOrCritical := severityGroup.HasHighOrCriticalErrors()

// Filter by minimum severity
filteredGroup := group.FilterBySeverity(validate.SeverityHigh)
```

### Severity-Based Error Handling

```go
// Check severity and handle accordingly
severity := validate.GetDefaultSeverityForErrorType(err.ErrorType)

switch {
case severity.IsCritical():
    // Immediately alert and block operation
    alertCriticalError(err)
    return err
case severity.IsHigh():
    // Log prominently and notify
    logHigh(err)
    notifyTeam(err)
case severity.IsMediumOrHigher():
    // Log and track
    logMedium(err)
    trackMetric("validation_failure", err.ErrorType)
default:
    // Log for visibility
    logLow(err)
}
```

### Severity in Error Responses

```go
// Include severity in API error responses
type ErrorResponse struct {
    Severity string `json:"severity"`
    ErrorType string `json:"error_type"`
    Message string `json:"message"`
}

err := ValidationError{
    ErrorType: "status_code",
    Message: "Expected 200 but got 404",
}

response := ErrorResponse{
    Severity: validate.FormatSeverityForLog(
        validate.GetDefaultSeverityForErrorType(err.ErrorType)
    ),
    ErrorType: err.ErrorType,
    Message: err.Message,
}
// Output: {"severity":"high","error_type":"status_code","message":"..."}
```

## Best Practices

### DO

- **Use default severity mappings** for consistency
- **Consider impact and urgency** when assigning severity
- **Use visual indicators** for user-facing error messages
- **Log severity appropriately** for monitoring systems
- **Filter by severity** for error reporting and alerting
- **Document custom severity assignments** in code comments

### DON'T

- **Don't overuse Critical severity** - reserve for truly blocking issues
- **Don't ignore High severity** - these need attention
- **Don't use severity for error categorization** - use ErrorType instead
- **Don't mix severity levels** inconsistently for similar error types
- **Don't forget to update mappings** when adding new error types
- **Don't use color codes in non-terminal environments** - test display capabilities

## Testing and Validation

### Unit Testing Severity Assignments

```go
func TestDefaultSeverityAssignments(t *testing.T) {
    tests := []struct {
        errorType string
        expected validate.ErrorSeverity
    }{
        {"status_code", validate.SeverityHigh},
        {"auth_headers", validate.SeverityCritical},
        {"custom_headers", validate.SeverityLow},
    }
    
    for _, tt := range tests {
        actual := validate.GetDefaultSeverityForErrorType(tt.errorType)
        if actual != tt.expected {
            t.Errorf("Error type %s: expected %v, got %v",
                tt.errorType, tt.expected, actual)
        }
    }
}
```

### Integration Testing with Severity

```go
func TestSeverityFormattingIntegration(t *testing.T) {
    err := ValidationError{
        ErrorType: "status_code",
        Message: "Expected 200 but got 404",
        Expected: 200,
        Actual: 404,
    }
    
    // Test formatting with severity
    formatted := validate.FormatValidationErrorFull(err, true)
    
    // Verify severity indicator is present
    if !strings.Contains(formatted, "HIGH") {
        t.Errorf("Expected HIGH severity indicator in formatted output")
    }
}
```

## Extensibility

### Adding New Severity Levels

While the current five-level system is designed to be comprehensive, future requirements may necessitate additional levels. To add a new severity level:

1. **Define the new constant** in `error_categorization.go`:

```go
const (
    // Existing severities...
    SeverityCritical ErrorSeverity = "critical"
    
    // New severity
    SeverityEmergency ErrorSeverity = "emergency"
)
```

2. **Update comparison logic** in the `Compare()` method to include the new level

3. **Add formatting support** in `error_formatting.go`:

```go
func FormatSeverity(severity ErrorSeverity) string {
    switch severity {
    case SeverityEmergency:
        return "EMERGENCY"
    // Existing cases...
    }
}
```

4. **Add visual indicators**:

```go
func severityIndicator(severity ErrorSeverity) string {
    switch severity {
    case SeverityEmergency:
        return "🆘"
    // Existing cases...
    }
}
```

5. **Update color coding** if applicable:

```go
func applySeverityColor(s string, severity ErrorSeverity) string {
    const (
        colorEmergency = "\033[1;35m" // Bold magenta for emergency
        // Existing colors...
    )
    
    var colorCode string
    switch severity {
    case SeverityEmergency:
        colorCode = colorEmergency
    // Existing cases...
    }
}
```

### Custom Severity Implementations

For domain-specific severity requirements, consider creating a custom severity type that implements similar interfaces:

```go
// Custom severity for specific domain
type CustomSeverity string

const (
    CustomSeverityTrivial CustomSeverity = "trivial"
    CustomSeverityStandard CustomSeverity = "standard"
    CustomSeverityUrgent CustomSeverity = "urgent"
)

// Implement similar methods for consistency
func (cs CustomSeverity) String() string {
    return string(cs)
}

func (cs CustomSeverity) IsValid() bool {
    switch cs {
    case CustomSeverityTrivial, CustomSeverityStandard, CustomSeverityUrgent:
        return true
    default:
        return false
    }
}
```

## Monitoring and Alerting

### Severity-Based Alerting

Configure monitoring systems to trigger alerts based on severity:

```
Critical: Immediate paging (within 1 minute)
High: Alert within 5 minutes
Medium: Alert within 15 minutes
Low: Daily summary
Info: No alerting (log only)
```

### Severity Metrics

Track severity distribution in metrics:

```
validation_errors_total{severity="critical"} 0
validation_errors_total{severity="high"} 3
validation_errors_total{severity="medium"} 15
validation_errors_total{severity="low"} 42
validation_errors_total{severity="info"} 128
```

### Log Aggregation

Use severity for log filtering and aggregation:

- **Critical logs:** Sent to all alerting channels
- **High logs:** Sent to primary monitoring
- **Medium logs:** Sent to team notifications
- **Low logs:** Logged for historical analysis
- **Info logs:** Logged in debug mode only

## References

- **Core Implementation:** `internal/validate/error_categorization.go`
- **Formatting Functions:** `internal/validate/error_formatting.go`
- **Error Types:** `internal/validate/error_types.go`
- **Usage Examples:** `internal/validate/*_test.go`

## Summary

The ARMOR severity level system provides:

1. **Five standardized severity levels** from Info to Critical
2. **Visual indicators** including emoji, symbols, and color coding
3. **Default severity mappings** for all error types
4. **Flexible formatting** for different display contexts
5. **Consistent behavior** across the error system
6. **Extensibility** for custom severity requirements

By following these guidelines and using the provided formatting functions, implementers can ensure consistent, meaningful error severity communication throughout their applications.