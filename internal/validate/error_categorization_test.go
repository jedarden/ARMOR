package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// ERROR SEVERITY TESTS
// =============================================================================

func TestErrorSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		want     string
	}{
		{"critical", SeverityCritical, "critical"},
		{"high", SeverityHigh, "high"},
		{"medium", SeverityMedium, "medium"},
		{"low", SeverityLow, "low"},
		{"info", SeverityInfo, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("ErrorSeverity.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorSeverity_IsValid(t *testing.T) {
	validSeverities := []ErrorSeverity{
		SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo,
	}

	for _, severity := range validSeverities {
		t.Run(severity.String(), func(t *testing.T) {
			if !severity.IsValid() {
				t.Errorf("ErrorSeverity.IsValid() = false, want true for %v", severity)
			}
		})
	}

	invalidSeverity := ErrorSeverity("invalid")
	if invalidSeverity.IsValid() {
		t.Error("ErrorSeverity.IsValid() = true for invalid severity, want false")
	}
}

func TestErrorSeverity_Description(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		want     string
	}{
		{SeverityCritical, "Critical error that prevents system functionality"},
		{SeverityHigh, "High-severity error that significantly impacts functionality"},
		{SeverityMedium, "Medium-severity error that partially impacts functionality"},
		{SeverityLow, "Low-severity error with minimal impact"},
		{SeverityInfo, "Informational message that doesn't represent a failure"},
	}

	for _, tt := range tests {
		t.Run(tt.severity.String(), func(t *testing.T) {
			if got := tt.severity.Description(); got != tt.want {
				t.Errorf("ErrorSeverity.Description() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorSeverity_Compare(t *testing.T) {
	tests := []struct {
		name     string
		base     ErrorSeverity
		compare  ErrorSeverity
		expected int
	}{
		{"critical vs high", SeverityCritical, SeverityHigh, 1},
		{"high vs medium", SeverityHigh, SeverityMedium, 1},
		{"medium vs low", SeverityMedium, SeverityLow, 1},
		{"low vs info", SeverityLow, SeverityInfo, 1},
		{"high vs high", SeverityHigh, SeverityHigh, 0},
		{"low vs high", SeverityLow, SeverityHigh, -1},
		{"info vs critical", SeverityInfo, SeverityCritical, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.base.Compare(tt.compare); got != tt.expected {
				t.Errorf("ErrorSeverity.Compare() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorSeverity_Predicates(t *testing.T) {
	tests := []struct {
		name             string
		severity         ErrorSeverity
		isCritical       bool
		isHigh           bool
		isMediumOrHigher bool
		isLowOrHigher    bool
	}{
		{"critical", SeverityCritical, true, true, true, true},
		{"high", SeverityHigh, false, true, true, true},
		{"medium", SeverityMedium, false, false, true, true},
		{"low", SeverityLow, false, false, false, true},
		{"info", SeverityInfo, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.IsCritical(); got != tt.isCritical {
				t.Errorf("IsCritical() = %v, want %v", got, tt.isCritical)
			}
			if got := tt.severity.IsHigh(); got != tt.isHigh {
				t.Errorf("IsHigh() = %v, want %v", got, tt.isHigh)
			}
			if got := tt.severity.IsMediumOrHigher(); got != tt.isMediumOrHigher {
				t.Errorf("IsMediumOrHigher() = %v, want %v", got, tt.isMediumOrHigher)
			}
			if got := tt.severity.IsLowOrHigher(); got != tt.isLowOrHigher {
				t.Errorf("IsLowOrHigher() = %v, want %v", got, tt.isLowOrHigher)
			}
		})
	}
}

func TestErrorSeverityFromString(t *testing.T) {
	tests := []struct {
		input string
		want  ErrorSeverity
	}{
		{"critical", SeverityCritical},
		{"CRITICAL", SeverityCritical},
		{"high", SeverityHigh},
		{"HIGH", SeverityHigh},
		{"medium", SeverityMedium},
		{"MEDIUM", SeverityMedium},
		{"low", SeverityLow},
		{"LOW", SeverityLow},
		{"info", SeverityInfo},
		{"INFO", SeverityInfo},
		{"invalid", SeverityInfo},
		{"", SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ErrorSeverityFromString(tt.input); got != tt.want {
				t.Errorf("ErrorSeverityFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetDefaultSeverityForErrorType(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		want      ErrorSeverity
	}{
		{"status code", ErrorTypeStatusCode, SeverityHigh},
		{"status code range", ErrorTypeStatusCodeRange, SeverityHigh},
		{"status code class", ErrorTypeStatusCodeClass, SeverityMedium},
		{"content type", ErrorTypeContentType, SeverityMedium},
		{"response structure", ErrorTypeResponseStructure, SeverityHigh},
		{"auth headers", ErrorTypeAuthHeaders, SeverityCritical},
		{"timeout", ErrorTypeTimeout, SeverityHigh},
		{"rate limit", ErrorTypeRateLimit, SeverityMedium},
		{"custom headers", ErrorTypeCustomHeaders, SeverityLow},
		{"response encoding", ErrorTypeResponseEncoding, SeverityLow},
		{"unknown", "unknown_type", SeverityLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDefaultSeverityForErrorType(tt.errorType); got != tt.want {
				t.Errorf("GetDefaultSeverityForErrorType(%q) = %v, want %v", tt.errorType, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ERROR FIELD GROUP TESTS
// =============================================================================

func TestNewErrorFieldGroup(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "Error 1"},
		{ErrorType: ErrorTypeContentType, FieldName: "field1", Message: "Error 2"},
		{ErrorType: ErrorTypeResponseStructure, FieldName: "field2", Message: "Error 3"},
		{ErrorType: ErrorTypeTimeout, Message: "Error 4"},
	}

	group := NewErrorFieldGroup(errors)

	if got := group.Count(); got != 4 {
		t.Errorf("Count() = %v, want 4", got)
	}

	if got := len(group.GetErrors("field1")); got != 2 {
		t.Errorf("GetErrors(field1) = %v, want 2", got)
	}

	if got := len(group.GetErrors("field2")); got != 1 {
		t.Errorf("GetErrors(field2) = %v, want 1", got)
	}

	if got := len(group.GetErrors("")); got != 1 {
		t.Errorf("GetErrors() [general] = %v, want 1", got)
	}
}

func TestErrorFieldGroup_Add(t *testing.T) {
	group := NewErrorFieldGroup([]ValidationError{})

	err := ValidationError{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "Error 1"}
	group.Add(err)

	if got := group.Count(); got != 1 {
		t.Errorf("Count() = %v, want 1", got)
	}

	if !group.HasErrors("field1") {
		t.Error("HasErrors(field1) = false, want true")
	}
}

func TestErrorFieldGroup_FilterBySeverity(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeAuthHeaders, FieldName: "field1", Message: "Critical error"},
		{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "High error"},
		{ErrorType: ErrorTypeContentType, FieldName: "field2", Message: "Medium error"},
		{ErrorType: ErrorTypeCustomHeaders, FieldName: "field2", Message: "Low error"},
	}

	group := NewErrorFieldGroup(errors)

	filtered := group.FilterBySeverity(SeverityHigh)

	if got := filtered.Count(); got != 2 {
		t.Errorf("FilterBySeverity(SeverityHigh).Count() = %v, want 2", got)
	}

	if got := filtered.CountByField("field1"); got != 2 {
		t.Errorf("FilterBySeverity(SeverityHigh).CountByField(field1) = %v, want 2", got)
	}

	if got := filtered.CountByField("field2"); got != 0 {
		t.Errorf("FilterBySeverity(SeverityHigh).CountByField(field2) = %v, want 0", got)
	}
}

func TestErrorFieldGroup_FilterByCategory(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "HTTP error"},
		{ErrorType: ErrorTypeAuthHeaders, FieldName: "field1", Message: "HTTP error"},
		{ErrorType: ErrorTypeResponseStructure, FieldName: "field2", Message: "Content error"},
	}

	group := NewErrorFieldGroup(errors)

	filtered := group.FilterByCategory(CategoryHTTP)

	if got := filtered.Count(); got != 2 {
		t.Errorf("FilterByCategory(CategoryHTTP).Count() = %v, want 2", got)
	}

	if got := filtered.CountByField("field1"); got != 2 {
		t.Errorf("FilterByCategory(CategoryHTTP).CountByField(field1) = %v, want 2", got)
	}

	if got := filtered.CountByField("field2"); got != 0 {
		t.Errorf("FilterByCategory(CategoryHTTP).CountByField(field2) = %v, want 0", got)
	}
}

func TestErrorFieldGroup_Summary(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "Error 1"},
		{ErrorType: ErrorTypeContentType, FieldName: "field1", Message: "Error 2"},
		{ErrorType: ErrorTypeResponseStructure, FieldName: "field2", Message: "Error 3"},
		{ErrorType: ErrorTypeTimeout, Message: "Error 4"},
	}

	group := NewErrorFieldGroup(errors)

	summary := group.Summary()

	if !strings.Contains(summary, "field1") {
		t.Error("Summary should contain 'field1'")
	}
	if !strings.Contains(summary, "field2") {
		t.Error("Summary should contain 'field2'")
	}
	if !strings.Contains(summary, "2 errors") {
		t.Error("Summary should contain '2 errors'")
	}
}

// =============================================================================
// ERROR CATEGORY GROUP TESTS
// =============================================================================

func TestNewErrorCategoryGroup(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeStatusCode, Message: "HTTP error 1"},
		{ErrorType: ErrorTypeAuthHeaders, Message: "HTTP error 2"},
		{ErrorType: ErrorTypeResponseStructure, Message: "Content error 1"},
		{ErrorType: ErrorTypeResponseBody, Message: "Content error 2"},
	}

	group := NewErrorCategoryGroup(errors)

	if got := group.Count(); got != 4 {
		t.Errorf("Count() = %v, want 4", got)
	}

	if got := len(group.GetErrors(CategoryHTTP)); got != 2 {
		t.Errorf("GetErrors(CategoryHTTP) = %v, want 2", got)
	}

	if got := len(group.GetErrors(CategoryContent)); got != 2 {
		t.Errorf("GetErrors(CategoryContent) = %v, want 2", got)
	}
}

func TestErrorSeverityGroup_HasCriticalErrors(t *testing.T) {
	tests := []struct {
		name              string
		errors            []ValidationError
		hasCritical       bool
		hasHighOrCritical bool
	}{
		{
			name: "with critical",
			errors: []ValidationError{
				{ErrorType: ErrorTypeAuthHeaders, Message: "Critical"},
			},
			hasCritical:       true,
			hasHighOrCritical: true,
		},
		{
			name: "with high only",
			errors: []ValidationError{
				{ErrorType: ErrorTypeStatusCode, Message: "High"},
			},
			hasCritical:       false,
			hasHighOrCritical: true,
		},
		{
			name: "with medium only",
			errors: []ValidationError{
				{ErrorType: ErrorTypeContentType, Message: "Medium"},
			},
			hasCritical:       false,
			hasHighOrCritical: false,
		},
		{
			name:              "empty",
			errors:            []ValidationError{},
			hasCritical:       false,
			hasHighOrCritical: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := NewErrorSeverityGroup(tt.errors)

			if got := group.HasCriticalErrors(); got != tt.hasCritical {
				t.Errorf("HasCriticalErrors() = %v, want %v", got, tt.hasCritical)
			}

			if got := group.HasHighOrCriticalErrors(); got != tt.hasHighOrCritical {
				t.Errorf("HasHighOrCriticalErrors() = %v, want %v", got, tt.hasHighOrCritical)
			}
		})
	}
}

// =============================================================================
// ERROR INDEX TESTS
// =============================================================================

func TestNewErrorIndex(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "HTTP error"},
		{ErrorType: ErrorTypeResponseStructure, FieldName: "field2", Message: "Content error"},
		{ErrorType: ErrorTypeAuthHeaders, FieldName: "field3", Message: "Auth error"},
	}

	index := NewErrorIndex(errors)

	if got := index.Count(); got != 3 {
		t.Errorf("Count() = %v, want 3", got)
	}

	fieldErrors := index.FilterByField("field1")
	if len(fieldErrors) != 1 {
		t.Errorf("FilterByField(field1) = %v, want 1", len(fieldErrors))
	}

	httpErrors := index.FilterByCategory(CategoryHTTP)
	if len(httpErrors) != 2 {
		t.Errorf("FilterByCategory(CategoryHTTP) = %v, want 2", len(httpErrors))
	}

	criticalErrors := index.FilterBySeverity(SeverityCritical)
	if len(criticalErrors) != 1 {
		t.Errorf("FilterBySeverity(SeverityCritical) = %v, want 1", len(criticalErrors))
	}

	statusCodeErrors := index.FilterByTypeError(ErrorTypeStatusCode)
	if len(statusCodeErrors) != 1 {
		t.Errorf("FilterByTypeError(ErrorTypeStatusCode) = %v, want 1", len(statusCodeErrors))
	}
}

func TestErrorIndex_Summary(t *testing.T) {
	errors := []ValidationError{
		{ErrorType: ErrorTypeStatusCode, FieldName: "field1", Message: "HTTP error 1"},
		{ErrorType: ErrorTypeAuthHeaders, FieldName: "field1", Message: "HTTP error 2"},
		{ErrorType: ErrorTypeResponseStructure, FieldName: "field2", Message: "Content error"},
	}

	index := NewErrorIndex(errors)

	summary := index.Summary()

	if !strings.Contains(summary, "Total: 3 errors") {
		t.Error("Summary should contain 'Total: 3 errors'")
	}

	if !strings.Contains(summary, "By field:") {
		t.Error("Summary should contain 'By field:'")
	}
	if !strings.Contains(summary, "By category:") {
		t.Error("Summary should contain 'By category:'")
	}
	if !strings.Contains(summary, "By severity:") {
		t.Error("Summary should contain 'By severity:'")
	}
}
