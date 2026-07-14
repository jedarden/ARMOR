package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// ERROR TYPE CONSTANT TESTS
// =============================================================================

func TestErrorTypeEnumConstants(t *testing.T) {
	tests := []struct {
		name      string
		constant  ErrorType
		wantValue string
	}{
		{"ErrTypeRequired", ErrTypeRequired, "required"},
		{"ErrTypeFormat", ErrTypeFormat, "format"},
		{"ErrTypeRange", ErrTypeRange, "range"},
		{"ErrTypeLength", ErrTypeLength, "length"},
		{"ErrTypeType", ErrTypeType, "type"},
		{"ErrTypeValue", ErrTypeValue, "value"},
		{"ErrTypeDuplicate", ErrTypeDuplicate, "duplicate"},
		{"ErrTypeConflict", ErrTypeConflict, "conflict"},
		{"ErrTypeUnknown", ErrTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.constant.String(); got != tt.wantValue {
				t.Errorf("%s.String() = %v, want %v", tt.name, got, tt.wantValue)
			}
		})
	}
}

// =============================================================================
// ERROR TYPE ISVALID TESTS
// =============================================================================

func TestErrorTypeIsValid(t *testing.T) {
	validTypes := []ErrorType{
		ErrTypeRequired, ErrTypeFormat, ErrTypeRange, ErrTypeLength,
		ErrTypeType, ErrTypeValue, ErrTypeDuplicate, ErrTypeConflict,
		ErrTypeUnknown,
	}

	for _, tt := range validTypes {
		t.Run(tt.String(), func(t *testing.T) {
			if !tt.IsValid() {
				t.Errorf("%v.IsValid() = false, want true", tt)
			}
		})
	}

	// Test invalid type
	invalidType := ErrorType("invalid_type")
	if invalidType.IsValid() {
		t.Errorf("invalid type IsValid() = true, want false")
	}
}

// =============================================================================
// ERROR TYPE DESCRIPTION TESTS
// =============================================================================

func TestErrorTypeDescription(t *testing.T) {
	tests := []struct {
		name         string
		errorType    ErrorType
		wantContains string
	}{
		{"Required", ErrTypeRequired, "missing or empty"},
		{"Format", ErrTypeFormat, "invalid"},
		{"Range", ErrTypeRange, "outside acceptable range"},
		{"Length", ErrTypeLength, "length"},
		{"Type", ErrTypeType, "incorrect"},
		{"Value", ErrTypeValue, "invalid"},
		{"Duplicate", ErrTypeDuplicate, "Duplicate"},
		{"Conflict", ErrTypeConflict, "Conflict"},
		{"Unknown", ErrTypeUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorType.Description()
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("%v.Description() = %v, want to contain %v", tt.errorType, got, tt.wantContains)
			}
		})
	}
}

// =============================================================================
// ERROR TYPE PREDICATE TESTS
// =============================================================================

func TestErrorTypePredicates(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		method    func(ErrorType) bool
		want      bool
	}{
		{"Required-IsRequired", ErrTypeRequired, ErrorType.IsRequired, true},
		{"Required-IsFormat", ErrTypeRequired, ErrorType.IsFormat, false},
		{"Format-IsFormat", ErrTypeFormat, ErrorType.IsFormat, true},
		{"Format-IsRequired", ErrTypeFormat, ErrorType.IsRequired, false},
		{"Range-IsRange", ErrTypeRange, ErrorType.IsRange, true},
		{"Length-IsLength", ErrTypeLength, ErrorType.IsLength, true},
		{"Type-IsType", ErrTypeType, ErrorType.IsType, true},
		{"Value-IsValue", ErrTypeValue, ErrorType.IsValue, true},
		{"Duplicate-IsDuplicate", ErrTypeDuplicate, ErrorType.IsDuplicate, true},
		{"Conflict-IsConflict", ErrTypeConflict, ErrorType.IsConflict, true},
		{"Unknown-IsUnknown", ErrTypeUnknown, ErrorType.IsUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.method(tt.errorType); got != tt.want {
				t.Errorf("%v.%v() = %v, want %v", tt.errorType, tt.name, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ERROR TYPE FROM STRING TESTS
// =============================================================================

func TestErrorTypeFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     ErrorType
	}{
		{"Required lowercase", "required", ErrTypeRequired},
		{"Required uppercase", "REQUIRED", ErrTypeRequired},
		{"Format lowercase", "format", ErrTypeFormat},
		{"Range lowercase", "range", ErrTypeRange},
		{"Length lowercase", "length", ErrTypeLength},
		{"Type lowercase", "type", ErrTypeType},
		{"Value lowercase", "value", ErrTypeValue},
		{"Duplicate lowercase", "duplicate", ErrTypeDuplicate},
		{"Conflict lowercase", "conflict", ErrTypeConflict},
		{"Unknown lowercase", "unknown", ErrTypeUnknown},
		{"Invalid type", "invalid_type", ErrTypeUnknown},
		{"Empty string", "", ErrTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorTypeFromString(tt.input)
			if got != tt.want {
				t.Errorf("ErrorTypeFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMustParseErrorType(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      ErrorType
		wantPanic bool
	}{
		{"Valid type", "required", ErrTypeRequired, false},
		{"Valid type format", "format", ErrTypeFormat, false},
		{"Unknown type", "unknown", ErrTypeUnknown, false},
		{"Invalid type", "invalid_type", ErrTypeUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("MustParseErrorType(%q) did not panic", tt.input)
					}
				}()
				MustParseErrorType(tt.input)
			} else {
				got := MustParseErrorType(tt.input)
				if got != tt.want {
					t.Errorf("MustParseErrorType(%q) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

// =============================================================================
// ERROR TYPE LIST TESTS
// =============================================================================

func TestAllErrorTypes(t *testing.T) {
	if len(AllErrorTypes) != 9 {
		t.Errorf("AllErrorTypes length = %d, want 9", len(AllErrorTypes))
	}

	for _, et := range AllErrorTypes {
		if !et.IsValid() {
			t.Errorf("AllErrorTypes contains invalid type: %v", et)
		}
	}
}

func TestStructuralErrorTypes(t *testing.T) {
	expected := []ErrorType{ErrTypeRequired, ErrTypeType, ErrTypeLength}
	if len(StructuralErrorTypes) != len(expected) {
		t.Errorf("StructuralErrorTypes length = %d, want %d", len(StructuralErrorTypes), len(expected))
	}

	for _, want := range expected {
		if !StructuralErrorTypes.Contains(want) {
			t.Errorf("StructuralErrorTypes does not contain %v", want)
		}
	}
}

func TestSemanticErrorTypes(t *testing.T) {
	expected := []ErrorType{ErrTypeFormat, ErrTypeRange, ErrTypeValue}
	if len(SemanticErrorTypes) != len(expected) {
		t.Errorf("SemanticErrorTypes length = %d, want %d", len(SemanticErrorTypes), len(expected))
	}

	for _, want := range expected {
		if !SemanticErrorTypes.Contains(want) {
			t.Errorf("SemanticErrorTypes does not contain %v", want)
		}
	}
}

func TestConstraintErrorTypes(t *testing.T) {
	expected := []ErrorType{ErrTypeDuplicate, ErrTypeConflict}
	if len(ConstraintErrorTypes) != len(expected) {
		t.Errorf("ConstraintErrorTypes length = %d, want %d", len(ConstraintErrorTypes), len(expected))
	}

	for _, want := range expected {
		if !ConstraintErrorTypes.Contains(want) {
			t.Errorf("ConstraintErrorTypes does not contain %v", want)
		}
	}
}

func TestErrorTypeListContains(t *testing.T) {
	list := ErrorTypeList{ErrTypeRequired, ErrTypeFormat, ErrTypeRange}

	tests := []struct {
		name     string
		errorType ErrorType
		want     bool
	}{
		{"Contains Required", ErrTypeRequired, true},
		{"Contains Format", ErrTypeFormat, true},
		{"Contains Range", ErrTypeRange, true},
		{"Does not contain Length", ErrTypeLength, false},
		{"Does not contain Type", ErrTypeType, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := list.Contains(tt.errorType)
			if got != tt.want {
				t.Errorf("ErrorTypeList.Contains(%v) = %v, want %v", tt.errorType, got, tt.want)
			}
		})
	}
}

func TestErrorTypeListStrings(t *testing.T) {
	list := ErrorTypeList{ErrTypeRequired, ErrTypeFormat, ErrTypeRange}
	got := list.Strings()

	if len(got) != len(list) {
		t.Errorf("ErrorTypeList.Strings() length = %d, want %d", len(got), len(list))
	}

	expected := []string{"required", "format", "range"}
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("ErrorTypeList.Strings()[%d] = %v, want %v", i, got[i], want)
		}
	}
}

// =============================================================================
// ERROR TYPE VALIDATION TESTS
// =============================================================================

func TestErrorTypeValidate(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		wantErr   bool
	}{
		{"Valid Required", ErrTypeRequired, false},
		{"Valid Format", ErrTypeFormat, false},
		{"Valid Unknown", ErrTypeUnknown, false},
		{"Invalid type", ErrorType("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errorType.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("%v.Validate() error = %v, wantErr %v", tt.errorType, err, tt.wantErr)
			}
		})
	}
}

func TestErrorTypeOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		errorType    ErrorType
		wantFallback bool
	}{
		{"Valid Required", ErrTypeRequired, false},
		{"Valid Format", ErrTypeFormat, false},
		{"Invalid type", ErrorType("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errorType.OrDefault()
			isFallback := got == ErrTypeUnknown
			if isFallback != tt.wantFallback {
				t.Errorf("%v.OrDefault() = %v (fallback=%v), want fallback=%v",
					tt.errorType, got, isFallback, tt.wantFallback)
			}
		})
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestIsValidBasicErrorType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Valid required", "required", true},
		{"Valid format", "format", true},
		{"Valid range", "range", true},
		{"Valid length", "length", true},
		{"Valid type", "type", true},
		{"Valid value", "value", true},
		{"Valid duplicate", "duplicate", true},
		{"Valid conflict", "conflict", true},
		{"Valid unknown", "unknown", true},
		{"Invalid type", "invalid_type", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidBasicErrorType(tt.input)
			if got != tt.want {
				t.Errorf("IsValidBasicErrorType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseBasicErrorType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ErrorType
		wantOK  bool
	}{
		{"Valid required", "required", ErrTypeRequired, true},
		{"Valid format", "format", ErrTypeFormat, true},
		{"Valid unknown", "unknown", ErrTypeUnknown, true},
		{"Invalid type", "invalid_type", ErrTypeUnknown, false},
		{"Empty string", "", ErrTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseBasicErrorType(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseBasicErrorType(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ParseBasicErrorType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
