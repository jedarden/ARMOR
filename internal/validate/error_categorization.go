package validate

import (
	"fmt"
	"strings"
)

// =============================================================================
// ERROR SEVERITY ENUM
// =============================================================================

// ErrorSeverity represents the severity level of a validation error.
// Severity levels help prioritize error handling and user communication.
type ErrorSeverity string

const (
	// SeverityCritical indicates a critical error that prevents the system from functioning.
	// These errors typically indicate complete failure and require immediate attention.
	// Examples: Authentication failures, critical service unavailability.
	SeverityCritical ErrorSeverity = "critical"

	// SeverityHigh indicates a high-severity error that significantly impacts functionality.
	// These errors prevent core features from working but may have workarounds.
	// Examples: Missing required data, invalid authentication tokens.
	SeverityHigh ErrorSeverity = "high"

	// SeverityMedium indicates a medium-severity error that partially impacts functionality.
	// These errors may affect non-critical features or have mitigations.
	// Examples: Optional fields missing, non-critical validation failures.
	SeverityMedium ErrorSeverity = "medium"

	// SeverityLow indicates a low-severity error with minimal impact.
	// These errors typically represent minor issues or deviations.
	// Examples: Formatting issues, minor data inconsistencies.
	SeverityLow ErrorSeverity = "low"

	// SeverityInfo indicates an informational message that doesn't represent a failure.
	// These are typically warnings or notices for attention.
	// Examples: Deprecation notices, informational messages.
	SeverityInfo ErrorSeverity = "info"
)

// String returns the string representation of the ErrorSeverity.
func (es ErrorSeverity) String() string {
	return string(es)
}

// IsValid returns true if this is a valid ErrorSeverity constant.
func (es ErrorSeverity) IsValid() bool {
	switch es {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return true
	default:
		return false
	}
}

// Description returns a human-readable description of this severity level.
func (es ErrorSeverity) Description() string {
	switch es {
	case SeverityCritical:
		return "Critical error that prevents system functionality"
	case SeverityHigh:
		return "High-severity error that significantly impacts functionality"
	case SeverityMedium:
		return "Medium-severity error that partially impacts functionality"
	case SeverityLow:
		return "Low-severity error with minimal impact"
	case SeverityInfo:
		return "Informational message that doesn't represent a failure"
	default:
		return "Unknown severity level"
	}
}

// IsCritical returns true if this is SeverityCritical.
func (es ErrorSeverity) IsCritical() bool {
	return es == SeverityCritical
}

// IsHigh returns true if this is SeverityHigh or higher.
func (es ErrorSeverity) IsHigh() bool {
	return es == SeverityHigh || es == SeverityCritical
}

// IsMediumOrHigher returns true if this is SeverityMedium or higher.
func (es ErrorSeverity) IsMediumOrHigher() bool {
	return es == SeverityMedium || es == SeverityHigh || es == SeverityCritical
}

// IsLowOrHigher returns true if this is SeverityLow or higher (everything except Info).
func (es ErrorSeverity) IsLowOrHigher() bool {
	return es == SeverityLow || es == SeverityMedium || es == SeverityHigh || es == SeverityCritical
}

// Compare compares this severity to another severity.
// Returns:
//   - positive if this severity is higher
//   - negative if this severity is lower
//   - 0 if they are equal
func (es ErrorSeverity) Compare(other ErrorSeverity) int {
	severityOrder := map[ErrorSeverity]int{
		SeverityInfo:     0,
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}
	return severityOrder[es] - severityOrder[other]
}

// ErrorSeverityFromString creates an ErrorSeverity from a string.
// Returns SeverityInfo if the string doesn't match any known severity.
func ErrorSeverityFromString(s string) ErrorSeverity {
	switch ErrorSeverity(strings.ToLower(s)) {
	case SeverityCritical:
		return SeverityCritical
	case SeverityHigh:
		return SeverityHigh
	case SeverityMedium:
		return SeverityMedium
	case SeverityLow:
		return SeverityLow
	case SeverityInfo:
		return SeverityInfo
	default:
		return SeverityInfo
	}
}

// =============================================================================
// DEFAULT SEVERITY MAPPINGS
// =============================================================================

// defaultSeverityForErrorType defines the default severity for each error type.
var defaultSeverityForErrorType = map[string]ErrorSeverity{
	// HTTP status codes
	ErrorTypeStatusCode:          SeverityHigh,
	ErrorTypeStatusCodeRange:     SeverityHigh,
	ErrorTypeStatusCodeClass:     SeverityMedium,

	// Content validation
	ErrorTypeContentType:         SeverityMedium,
	ErrorTypeResponseStructure:   SeverityHigh,
	ErrorTypeResponseBody:        SeverityHigh,
	ErrorTypeResponseEncoding:    SeverityLow,

	// Error messages
	ErrorTypeErrorMessage:        SeverityHigh,
	ErrorTypeErrorMessagePattern: SeverityMedium,
	ErrorTypeErrorCode:           SeverityMedium,
	ErrorTypeErrorDetail:         SeverityLow,

	// Headers
	ErrorTypeCORSHeaders:         SeverityMedium,
	ErrorTypeAuthHeaders:         SeverityCritical,
	ErrorTypeCustomHeaders:       SeverityLow,

	// Schema and data validation
	ErrorTypeJSONSchema:          SeverityHigh,
	ErrorTypeDataValidation:      SeverityHigh,
	ErrorTypeFieldValidation:     SeverityMedium,
	ErrorTypeTypeValidation:      SeverityHigh,

	// Performance
	ErrorTypeTimeout:             SeverityHigh,
	ErrorTypeRateLimit:           SeverityMedium,
	ErrorTypeRetryExceeded:       SeverityHigh,

	// Custom
	ErrorTypeCustom:              SeverityMedium,
	ErrorTypeUnknown:             SeverityLow,
}

// GetDefaultSeverityForErrorType returns the default severity for a given error type.
// Returns SeverityLow if the error type is not recognized.
func GetDefaultSeverityForErrorType(errorType string) ErrorSeverity {
	if severity, ok := defaultSeverityForErrorType[errorType]; ok {
		return severity
	}
	return SeverityLow
}

// =============================================================================
// FIELD-BASED ERROR GROUPING
// =============================================================================

// ErrorFieldGroup represents a collection of validation errors grouped by field name.
// This type is useful for organizing errors that occur across multiple fields
// and for presenting field-specific error summaries to users.
type ErrorFieldGroup map[string][]ValidationError

// NewErrorFieldGroup creates a new ErrorFieldGroup from a slice of ValidationErrors.
// Errors are grouped by their FieldName field. Errors without a FieldName are
// grouped under an empty string key.
func NewErrorFieldGroup(errors []ValidationError) ErrorFieldGroup {
	group := make(ErrorFieldGroup)
	for _, err := range errors {
		fieldName := err.FieldName
		group[fieldName] = append(group[fieldName], err)
	}
	return group
}

// Add adds an error to the appropriate field group.
func (efg ErrorFieldGroup) Add(err ValidationError) {
	fieldName := err.FieldName
	efg[fieldName] = append(efg[fieldName], err)
}

// GetErrors returns all errors for a specific field.
// Returns nil if the field has no errors.
func (efg ErrorFieldGroup) GetErrors(fieldName string) []ValidationError {
	return efg[fieldName]
}

// GetFieldNames returns a list of all field names that have errors.
// The list is sorted alphabetically for consistent ordering.
func (efg ErrorFieldGroup) GetFieldNames() []string {
	names := make([]string, 0, len(efg))
	for name := range efg {
		names = append(names, name)
	}
	return names
}

// HasErrors returns true if the specified field has any errors.
func (efg ErrorFieldGroup) HasErrors(fieldName string) bool {
	return len(efg[fieldName]) > 0
}

// Count returns the total number of errors across all fields.
func (efg ErrorFieldGroup) Count() int {
	total := 0
	for _, errors := range efg {
		total += len(errors)
	}
	return total
}

// CountByField returns the number of errors for a specific field.
func (efg ErrorFieldGroup) CountByField(fieldName string) int {
	return len(efg[fieldName])
}

// FilterBySeverity returns a new ErrorFieldGroup containing only errors
// of the specified severity or higher.
func (efg ErrorFieldGroup) FilterBySeverity(minSeverity ErrorSeverity) ErrorFieldGroup {
	result := make(ErrorFieldGroup)
	for fieldName, errors := range efg {
		for _, err := range errors {
			errSeverity := GetDefaultSeverityForErrorType(err.ErrorType)
			if errSeverity.Compare(minSeverity) >= 0 {
				result[fieldName] = append(result[fieldName], err)
			}
		}
	}
	return result
}

// FilterByCategory returns a new ErrorFieldGroup containing only errors
// of the specified category.
func (efg ErrorFieldGroup) FilterByCategory(category ErrorCategory) ErrorFieldGroup {
	result := make(ErrorFieldGroup)
	for fieldName, errors := range efg {
		for _, err := range errors {
			if GetCategoryForErrorType(err.ErrorType) == category {
				result[fieldName] = append(result[fieldName], err)
			}
		}
	}
	return result
}

// ToMap converts the ErrorFieldGroup to a map[string][]ValidationError.
// This is useful for serialization and external communication.
func (efg ErrorFieldGroup) ToMap() map[string][]ValidationError {
	return map[string][]ValidationError(efg)
}

// Summary returns a human-readable summary of the field errors.
// The summary includes field names and error counts.
func (efg ErrorFieldGroup) Summary() string {
	var parts []string
	for fieldName, errors := range efg {
		if fieldName == "" {
			parts = append(parts, fmt.Sprintf("general: %d errors", len(errors)))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %d errors", fieldName, len(errors)))
		}
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// ERROR CATEGORY GROUP
// =============================================================================

// ErrorCategoryGroup represents a collection of validation errors grouped by category.
// This type is useful for analyzing errors by their type and understanding
// which categories of validation are failing.
type ErrorCategoryGroup map[ErrorCategory][]ValidationError

// NewErrorCategoryGroup creates a new ErrorCategoryGroup from a slice of ValidationErrors.
// Errors are grouped by their ErrorCategory.
func NewErrorCategoryGroup(errors []ValidationError) ErrorCategoryGroup {
	group := make(ErrorCategoryGroup)
	for _, err := range errors {
		category := GetCategoryForErrorType(err.ErrorType)
		group[category] = append(group[category], err)
	}
	return group
}

// Add adds an error to the appropriate category group.
func (ecg ErrorCategoryGroup) Add(err ValidationError) {
	category := GetCategoryForErrorType(err.ErrorType)
	ecg[category] = append(ecg[category], err)
}

// GetErrors returns all errors for a specific category.
// Returns nil if the category has no errors.
func (ecg ErrorCategoryGroup) GetErrors(category ErrorCategory) []ValidationError {
	return ecg[category]
}

// GetCategories returns a list of all categories that have errors.
func (ecg ErrorCategoryGroup) GetCategories() []ErrorCategory {
	categories := make([]ErrorCategory, 0, len(ecg))
	for category := range ecg {
		categories = append(categories, category)
	}
	return categories
}

// HasErrors returns true if the specified category has any errors.
func (ecg ErrorCategoryGroup) HasErrors(category ErrorCategory) bool {
	return len(ecg[category]) > 0
}

// Count returns the total number of errors across all categories.
func (ecg ErrorCategoryGroup) Count() int {
	total := 0
	for _, errors := range ecg {
		total += len(errors)
	}
	return total
}

// CountByCategory returns the number of errors for a specific category.
func (ecg ErrorCategoryGroup) CountByCategory(category ErrorCategory) int {
	return len(ecg[category])
}

// FilterBySeverity returns a new ErrorCategoryGroup containing only errors
// of the specified severity or higher.
func (ecg ErrorCategoryGroup) FilterBySeverity(minSeverity ErrorSeverity) ErrorCategoryGroup {
	result := make(ErrorCategoryGroup)
	for category, errors := range ecg {
		for _, err := range errors {
			errSeverity := GetDefaultSeverityForErrorType(err.ErrorType)
			if errSeverity.Compare(minSeverity) >= 0 {
				result[category] = append(result[category], err)
			}
		}
	}
	return result
}

// ToMap converts the ErrorCategoryGroup to a map[ErrorCategory][]ValidationError.
func (ecg ErrorCategoryGroup) ToMap() map[ErrorCategory][]ValidationError {
	return map[ErrorCategory][]ValidationError(ecg)
}

// Summary returns a human-readable summary of the category errors.
func (ecg ErrorCategoryGroup) Summary() string {
	var parts []string
	for category, errors := range ecg {
		parts = append(parts, fmt.Sprintf("%s: %d errors", category, len(errors)))
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// ERROR SEVERITY GROUP
// =============================================================================

// ErrorSeverityGroup represents a collection of validation errors grouped by severity.
// This type is useful for prioritizing error handling and understanding
// the severity distribution of validation failures.
type ErrorSeverityGroup map[ErrorSeverity][]ValidationError

// NewErrorSeverityGroup creates a new ErrorSeverityGroup from a slice of ValidationErrors.
// Errors are grouped by their default severity level.
func NewErrorSeverityGroup(errors []ValidationError) ErrorSeverityGroup {
	group := make(ErrorSeverityGroup)
	for _, err := range errors {
		severity := GetDefaultSeverityForErrorType(err.ErrorType)
		group[severity] = append(group[severity], err)
	}
	return group
}

// Add adds an error to the appropriate severity group.
func (esg ErrorSeverityGroup) Add(err ValidationError) {
	severity := GetDefaultSeverityForErrorType(err.ErrorType)
	esg[severity] = append(esg[severity], err)
}

// GetErrors returns all errors for a specific severity level.
// Returns nil if the severity has no errors.
func (esg ErrorSeverityGroup) GetErrors(severity ErrorSeverity) []ValidationError {
	return esg[severity]
}

// GetSeverities returns a list of all severity levels that have errors.
func (esg ErrorSeverityGroup) GetSeverities() []ErrorSeverity {
	severities := make([]ErrorSeverity, 0, len(esg))
	for severity := range esg {
		severities = append(severities, severity)
	}
	return severities
}

// HasErrors returns true if the specified severity has any errors.
func (esg ErrorSeverityGroup) HasErrors(severity ErrorSeverity) bool {
	return len(esg[severity]) > 0
}

// Count returns the total number of errors across all severities.
func (esg ErrorSeverityGroup) Count() int {
	total := 0
	for _, errors := range esg {
		total += len(errors)
	}
	return total
}

// CountBySeverity returns the number of errors for a specific severity level.
func (esg ErrorSeverityGroup) CountBySeverity(severity ErrorSeverity) int {
	return len(esg[severity])
}

// GetCriticalErrors returns all critical errors.
func (esg ErrorSeverityGroup) GetCriticalErrors() []ValidationError {
	return esg.GetErrors(SeverityCritical)
}

// GetHighErrors returns all high-severity errors.
func (esg ErrorSeverityGroup) GetHighErrors() []ValidationError {
	return esg.GetErrors(SeverityHigh)
}

// HasCriticalErrors returns true if there are any critical errors.
func (esg ErrorSeverityGroup) HasCriticalErrors() bool {
	return esg.HasErrors(SeverityCritical)
}

// HasHighOrCriticalErrors returns true if there are any high or critical errors.
func (esg ErrorSeverityGroup) HasHighOrCriticalErrors() bool {
	return esg.HasErrors(SeverityCritical) || esg.HasErrors(SeverityHigh)
}

// ToMap converts the ErrorSeverityGroup to a map[ErrorSeverity][]ValidationError.
func (esg ErrorSeverityGroup) ToMap() map[ErrorSeverity][]ValidationError {
	return map[ErrorSeverity][]ValidationError(esg)
}

// Summary returns a human-readable summary of the severity errors.
func (esg ErrorSeverityGroup) Summary() string {
	var parts []string
	for severity, errors := range esg {
		parts = append(parts, fmt.Sprintf("%s: %d errors", severity, len(errors)))
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// MULTI-DIMENSIONAL ERROR INDEX
// =============================================================================

// ErrorIndex provides multi-dimensional indexing for validation errors.
// This type allows efficient filtering and grouping by multiple dimensions:
// - Field name
// - Category
// - Severity
// - Error type
type ErrorIndex struct {
	byField    ErrorFieldGroup
	byCategory ErrorCategoryGroup
	bySeverity ErrorSeverityGroup
	byType     map[string][]ValidationError
	allErrors  []ValidationError
}

// NewErrorIndex creates a new ErrorIndex from a slice of ValidationErrors.
func NewErrorIndex(errors []ValidationError) *ErrorIndex {
	index := &ErrorIndex{
		byField:    make(ErrorFieldGroup),
		byCategory: make(ErrorCategoryGroup),
		bySeverity: make(ErrorSeverityGroup),
		byType:     make(map[string][]ValidationError),
		allErrors:  errors,
	}

	for _, err := range errors {
		// Index by field
		fieldName := err.FieldName
		index.byField[fieldName] = append(index.byField[fieldName], err)

		// Index by category
		category := GetCategoryForErrorType(err.ErrorType)
		index.byCategory[category] = append(index.byCategory[category], err)

		// Index by severity
		severity := GetDefaultSeverityForErrorType(err.ErrorType)
		index.bySeverity[severity] = append(index.bySeverity[severity], err)

		// Index by error type
		index.byType[err.ErrorType] = append(index.byType[err.ErrorType], err)
	}

	return index
}

// ByField returns the field-based grouping.
func (ei *ErrorIndex) ByField() ErrorFieldGroup {
	return ei.byField
}

// ByCategory returns the category-based grouping.
func (ei *ErrorIndex) ByCategory() ErrorCategoryGroup {
	return ei.byCategory
}

// BySeverity returns the severity-based grouping.
func (ei *ErrorIndex) BySeverity() ErrorSeverityGroup {
	return ei.bySeverity
}

// ByType returns errors grouped by error type.
func (ei *ErrorIndex) ByType() map[string][]ValidationError {
	return ei.byType
}

// AllErrors returns all errors in the index.
func (ei *ErrorIndex) AllErrors() []ValidationError {
	return ei.allErrors
}

// FilterByField returns all errors for a specific field.
func (ei *ErrorIndex) FilterByField(fieldName string) []ValidationError {
	return ei.byField.GetErrors(fieldName)
}

// FilterByCategory returns all errors for a specific category.
func (ei *ErrorIndex) FilterByCategory(category ErrorCategory) []ValidationError {
	return ei.byCategory.GetErrors(category)
}

// FilterBySeverity returns all errors for a specific severity level.
func (ei *ErrorIndex) FilterBySeverity(severity ErrorSeverity) []ValidationError {
	return ei.bySeverity.GetErrors(severity)
}

// FilterByTypeError returns all errors for a specific error type.
func (ei *ErrorIndex) FilterByTypeError(errorType string) []ValidationError {
	return ei.byType[errorType]
}

// HasCriticalErrors returns true if there are any critical errors.
func (ei *ErrorIndex) HasCriticalErrors() bool {
	return ei.bySeverity.HasCriticalErrors()
}

// HasHighOrCriticalErrors returns true if there are any high or critical errors.
func (ei *ErrorIndex) HasHighOrCriticalErrors() bool {
	return ei.bySeverity.HasHighOrCriticalErrors()
}

// Count returns the total number of errors in the index.
func (ei *ErrorIndex) Count() int {
	return len(ei.allErrors)
}

// Summary returns a comprehensive summary of all errors.
func (ei *ErrorIndex) Summary() string {
	return fmt.Sprintf("Total: %d errors | By field: %s | By category: %s | By severity: %s",
		ei.Count(),
		ei.byField.Summary(),
		ei.byCategory.Summary(),
		ei.bySeverity.Summary(),
	)
}
