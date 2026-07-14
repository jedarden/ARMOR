# Error Severity Levels - Design Documentation

## Overview

This document describes the severity levels in the ARMOR validation error system, their meanings, and formatting implications. Severity levels help prioritize error handling, guide user communication, and enable consistent error display across different contexts.

## Severity Levels

The ARMOR error system defines five severity levels, ordered from lowest to highest impact:

### 1. SeverityInfo (Level 1)

**String Identifier:** `"info"`

**Description:** Informational messages that don't represent a validation failure. These are typically notices, warnings, or deprecation notices that require attention but don't indicate a problem.

**Common Use Cases:**
- Deprecation notices for deprecated features or API endpoints
- Informational messages about optional best practices
- Notices about configuration suggestions
- Debugging or diagnostic information

**Default Severity For Error Types:**
- None (typically used explicitly rather than assigned)

**Examples:**
```go
// Informational notice about deprecated API version
ValidationError{
    ErrorType: "deprecated_api",
    Severity:  SeverityInfo,
    Message:   "API v1 is deprecated, please migrate to v2",
}
```

**Visual Indicators:**
- Emoji: 💡 (light bulb)
- Text tag: INFO
- ANSI Color: Cyan (`\033[36m`)
- Compact mode: I
- Symbol: ℹ

---

### 2. SeverityLow (Level 2)

**String Identifier:** `"low"`

**Description:** Low-severity errors with minimal impact. These errors typically represent minor issues, deviations, or formatting problems that don't prevent functionality but should be addressed for quality purposes.

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

**Examples:**
```go
// Minor encoding issue
ValidationError{
    ErrorType: "response_encoding",
    Severity:  SeverityLow,
    Message:   "Response uses UTF-8 with BOM, consider plain UTF-8",
}

// Field value issue (domain-specific)
ValidationError{
    ErrorType: string(ErrTypeValue),
    Severity:  SeverityLow,
    Message:   "Country code 'XX' is not recognized",
    FieldName: "country",
}
```

**Visual Indicators:**
- Emoji: ℹ️ (information)
- Text tag: LOW
- ANSI Color: Blue (`\033[34m`)
- Compact mode: L
- Symbol: ◦

**Behavioral Characteristics:**
- Logged but may not trigger alerts
- Can be aggregated or batched for reporting
- Often tolerated in production environments
- May indicate quality debt rather than functional problems

---

### 3. SeverityMedium (Level 3)

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

**Examples:**
```go
// Content type mismatch (non-critical)
ValidationError{
    ErrorType: "content_type",
    Severity:  SeverityMedium,
    Message:   "Expected application/json, got text/plain",
    Expected:  "application/json",
    Actual:    "text/plain",
}

// Rate limit encountered
ValidationError{
    ErrorType: "rate_limit",
    Severity:  SeverityMedium,
    Message:   "API rate limit exceeded, retry after 60s",
}

// Field format issue (non-required field)
ValidationError{
    ErrorType: string(ErrTypeFormat),
    Severity:  SeverityMedium,
    Message:   "Phone number format is invalid",
    FieldName: "phone",
}
```

**Visual Indicators:**
- Emoji: ⚡ (lightning bolt)
- Text tag: MED
- ANSI Color: Yellow (`\033[33m`)
- Compact mode: M
- Symbol: ⚡

**Behavioral Characteristics:**
- Triggers alerts in production systems
- Logged and monitored
- May require manual intervention or workaround
- Typically included in error reports and dashboards

---

### 4. SeverityHigh (Level 4)

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

**Examples:**
```go
// Critical status code mismatch
ValidationError{
    ErrorType: "status_code",
    Severity:  SeverityHigh,
    Message:   "Expected status 200, got 500 Internal Server Error",
    Expected:  200,
    Actual:    500,
    Context:   "POST /api/users",
}

// Required field missing
ValidationError{
    ErrorType: string(ErrTypeRequired),
    Severity:  SeverityHigh,
    Message:   "Required field is missing",
    FieldName: "email",
}

// Timeout error
ValidationError{
    ErrorType: "timeout",
    Severity:  SeverityHigh,
    Message:   "Request timed out after 30s",
    Context:   "GET /api/data",
}

// Response structure failure
ValidationError{
    ErrorType: "response_structure",
    Severity:  SeverityHigh,
    Message:   "Response body is not valid JSON",
    ResponseSnippet: `{"data": invalid}`,
}
```

**Visual Indicators:**
- Emoji: ⚠️ (warning sign)
- Text tag: HIGH
- ANSI Color: Red (`\033[31m`)
- Compact mode: H
- Symbol: ⚠

**Behavioral Characteristics:**
- Triggers immediate alerts and paging
- Logged to all monitoring systems
- May block deployment or release processes
- Requires root cause analysis and resolution
- Often included in SLA calculations

---

### 5. SeverityCritical (Level 5)

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

**Examples:**
```go
// Authentication failure
ValidationError{
    ErrorType: "auth_headers",
    Severity:  SeverityCritical,
    Message:   "Authentication failed: invalid or missing credentials",
    FieldName: "Authorization",
    Context:   "API access check",
}

// Critical service unavailable
ValidationError{
    ErrorType: "service_unavailable",
    Severity:  SeverityCritical,
    Message:   "Critical database service is not responding",
    Context:   "Data persistence layer",
}
```

**Visual Indicators:**
- Emoji: 🚨 (siren/alert)
- Text tag: CRIT
- ANSI Color: Bold Red (`\033[1;31m`)
- Compact mode: C
- Symbol: ⛔

**Behavioral Characteristics:**
- Triggers emergency alerts and escalations
- May trigger automatic rollback or failover procedures
- Logged to all monitoring systems with highest priority
- May cause service degradation or outage
- Requires immediate investigation and resolution
- Often reported to management and stakeholders

---

## Severity Formatting Functions

The ARMOR error system provides multiple formatting functions that respect severity levels:

### Basic Severity Formatting

```go
// Format a severity level as a human-readable label
FormatSeverity(SeverityCritical)  // Returns: "CRITICAL"
FormatSeverity(SeverityHigh)      // Returns: "High"

// Format with visual indicator
FormatSeverityWithIndicator(SeverityCritical)  // Returns: "[🚨] CRITICAL"
FormatSeverityWithIndicator(SeverityHigh)      // Returns: "[⚠️] High"
```

### Styled Severity Formatting

```go
// Default styling (emoji + brackets + uppercase)
config := DefaultSeverityStyleConfig()
FormatSeverityStyled(SeverityCritical, config)  // Returns: "[🚨] CRITICAL"

// Console styling (with ANSI color codes)
config := ConsoleSeverityStyleConfig()
FormatSeverityStyled(SeverityHigh, config)     // Returns: "[⚠️] HIGH" (red)

// Compact styling (minimal formatting)
config := CompactSeverityStyleConfig()
FormatSeverityStyled(SeverityMedium, config)   // Returns: "medium"
```

### Severity with Level

```go
// Add numeric level for sorting/prioritization
FormatSeverityWithLevel(SeverityCritical)  // Returns: "CRITICAL (Level 5)"
FormatSeverityWithLevel(SeverityInfo)      // Returns: "Info (Level 1)"
```

### Compact Representations

```go
// Single-character indicator for space-constrained displays
FormatSeverityCompact(SeverityCritical)  // Returns: "C"
FormatSeverityCompact(SeverityHigh)      // Returns: "H"
FormatSeverityCompact(SeverityMedium)     // Returns: "M"
FormatSeverityCompact(SeverityLow)       // Returns: "L"
FormatSeverityCompact(SeverityInfo)      // Returns: "I"
```

### Symbol Representations

```go
// Unicode symbols for monospace displays
FormatSeveritySymbol(SeverityCritical)  // Returns: "⛔"
FormatSeveritySymbol(SeverityHigh)      // Returns: "⚠"
FormatSeveritySymbol(SeverityMedium)     // Returns: "⚡"
FormatSeveritySymbol(SeverityLow)       // Returns: "◦"
FormatSeveritySymbol(SeverityInfo)      // Returns: "ℹ"
```

### Log-Friendly Formatting

```go
// Machine-readable format for log aggregation
FormatSeverityForLog(SeverityCritical)  // Returns: "critical"
FormatSeverityForLog(SeverityHigh)      // Returns: "high"
```

---

## Severity Comparison and Filtering

The error system provides methods for comparing and filtering errors by severity:

### Comparison Methods

```go
// Check if severity is at a certain level
severity.IsCritical()              // true if SeverityCritical
severity.IsHigh()                  // true if SeverityHigh or SeverityCritical
severity.IsMediumOrHigher()        // true if SeverityMedium or higher
severity.IsLowOrHigher()           // true if SeverityLow or higher (everything except Info)

// Compare two severities (returns positive/negative/zero)
SeverityCritical.Compare(SeverityHigh)     // Returns: 1 (higher)
SeverityLow.Compare(SeverityMedium)        // Returns: -1 (lower)
SeverityMedium.Compare(SeverityMedium)    // Returns: 0 (equal)
```

### Filtering Operations

```go
// Filter errors by minimum severity
errors := []ValidationError{...}

// Create ErrorSeverityGroup for filtering
group := NewErrorSeverityGroup(errors)

// Get only critical errors
criticalErrors := group.GetCriticalErrors()

// Get only high-severity errors
highErrors := group.GetHighErrors()

// Check for critical errors
if group.HasCriticalErrors() {
    // Handle critical errors
}

// Check for high or critical errors
if group.HasHighOrCriticalErrors() {
    // Handle high-severity errors
}

// Filter to specific severity or higher
filtered := group.FilterBySeverity(SeverityMedium)
// Returns ErrorSeverityGroup with only Medium, High, and Critical errors
```

---

## Default Severity Mappings

### String-Based Error Types

The following table shows default severity assignments for string-based error types:

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

### Enum-Based Error Types (Field Validation)

The following table shows default severity assignments for enum-based field validation error types:

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

---

## Severity in Error Message Formatting

Severity levels affect error message formatting in several ways:

### Basic Error Formatting with Severity

```go
// FormatErrorWithSeverity includes severity indicators
err := FormatErrorWithSeverity(
    ErrTypeRequired,
    SeverityHigh,
    "This field is required",
    "email",
)
// Returns: "[⚠️] HIGH [required] email: This field is required"
```

### Category-Aware Formatting with Severity

```go
// FormatErrorWithCategoryAndSeverity combines category and severity
err := FormatErrorWithCategoryAndSeverity(
    ErrTypeRequired,
    "Field is required",
    "email",
    WithSeverityOverride(SeverityCritical),
    WithCategoryHint(CategoryValidation),
)
// Returns: "[🚨 CRIT] [Validation] [required] email: Field is required"
```

### Full Error Formatting with Severity

```go
// FormatValidationErrorFull optionally includes severity
validationError := ValidationError{
    ErrorType: string(ErrTypeRequired),
    Message:   "Field is required",
    FieldName: "email",
}

formatted := FormatValidationErrorFull(validationError, true)
// Returns: "[⚠️] [required] email: Field is required"
```

---

## Severity Configuration and Overrides

The error system allows customizing severity levels through several mechanisms:

### Severity Override in Format Options

```go
// Override default severity for a specific error
err := FormatErrorWithCategoryAndSeverity(
    ErrTypeFormat,
    "Invalid email format",
    "email",
    WithSeverityOverride(SeverityCritical),  // Override default Medium
)
```

### Custom Severity Style Configurations

```go
// Create custom severity styling
customConfig := SeverityStyleConfig{
    UseEmoji:       true,
    UseColorCodes:  true,
    UseBrackets:    true,
    UppercaseLabel: true,
    CompactMode:    false,
}

formatted := FormatSeverityStyled(SeverityHigh, customConfig)
```

### Programmatic Severity Assignment

```go
// Assign severity when creating ValidationError
err := ValidationError{
    ErrorType: "custom_error",
    Message:   "Custom validation failed",
    Severity:  SeverityCritical,  // Custom severity assignment
}
```

---

## Best Practices for Severity Assignment

### DO

- **Use default severity mappings** for common error types
- **Override severity** only when domain-specific knowledge justifies it
- **Consider impact** when choosing severity - how badly does this affect functionality?
- **Think about recoverability** - is there a workaround?
- **Consider frequency** - common low-impact errors may still need Medium severity
- **Be consistent** - similar errors should have similar severity across the codebase

### DON'T

- **Don't overuse Critical** - reserve for truly catastrophic failures
- **Don't under-severe security issues** - authentication and authorization failures are typically Critical or High
- **Don't ignore Low severity errors** - they may indicate quality issues
- **Don't assign severity based on ease of fix** - severity should reflect impact, not effort
- **Don't use severity to prioritize work items** - that's what backlog management is for

---

## Severity in Monitoring and Alerting

Severity levels integrate with monitoring and alerting systems:

### Alert Thresholds

```go
// Example alert configuration
alertConfig := map[ErrorSeverity]AlertAction{
    SeverityCritical: AlertAction{"page_oncall", "email_management", "block_deploy"},
    SeverityHigh:     AlertAction{"email_team", "create_ticket"},
    SeverityMedium:   AlertAction{"log_to_dashboard", "weekly_report"},
    SeverityLow:      AlertAction{"log_only"},
    SeverityInfo:     AlertAction{"log_to_debug"},
}
```

### SLA Calculations

```go
// Example SLA impact calculation
func SLAImpact(severity ErrorSeverity) float64 {
    switch severity {
    case SeverityCritical:
        return 1.0  // Full SLA credit
    case SeverityHigh:
        return 0.5  // Partial SLA credit
    case SeverityMedium:
        return 0.1  // Minimal SLA credit
    default:
        return 0.0  // No SLA impact
    }
}
```

### Error Budget Impact

```go
// Example error budget calculation
func ErrorBudgetCost(severity ErrorSeverity, duration time.Duration) float64 {
    weight := map[ErrorSeverity]float64{
        SeverityCritical: 10.0,
        SeverityHigh:      5.0,
        SeverityMedium:    1.0,
        SeverityLow:       0.1,
        SeverityInfo:      0.0,
    }
    return weight[severity] * duration.Minutes()
}
```

---

## Summary

The ARMOR error system's severity levels provide:

1. **Clear Prioritization:** Five levels from Info to Critical enable consistent prioritization
2. **Rich Formatting:** Visual indicators, colors, and text representations for different contexts
3. **Type Safety:** Enum-based severity prevents typos and ensures consistency
4. **Flexible Configuration:** Override mechanisms for domain-specific requirements
5. **Monitoring Integration:** Severity-aware monitoring and alerting capabilities
6. **Behavioral Guidance:** Default mappings that reflect best practices

The severity system is designed to help developers and operators quickly understand the impact of validation errors and prioritize their response appropriately. By using severity levels consistently, teams can improve debugging efficiency, reduce mean time to resolution (MTTR), and provide better user experiences through appropriate error handling.
