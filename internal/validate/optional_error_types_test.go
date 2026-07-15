package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// =============================================================================
// ValidationErrorContext Tests
// =============================================================================

func TestValidationErrorContext_Creation(t *testing.T) {
	t.Run("create with location", func(t *testing.T) {
		ctx := NewValidationErrorContext("field 'user.email'")
		if ctx.Location != "field 'user.email'" {
			t.Errorf("Expected location 'field 'user.email'', got '%s'", ctx.Location)
		}
	})

	t.Run("create with empty location", func(t *testing.T) {
		ctx := NewValidationErrorContext("")
		if ctx.Location != "" {
			t.Errorf("Expected empty location, got '%s'", ctx.Location)
		}
	})

	t.Run("create with related fields", func(t *testing.T) {
		ctx := NewValidationErrorContext("line 42").
			WithRelatedFields([]string{"email", "email_confirmation"})

		if ctx.Location != "line 42" {
			t.Errorf("Expected location 'line 42', got '%s'", ctx.Location)
		}

		if len(ctx.RelatedFields) != 2 {
			t.Errorf("Expected 2 related fields, got %d", len(ctx.RelatedFields))
		}

		if ctx.RelatedFields[0] != "email" || ctx.RelatedFields[1] != "email_confirmation" {
			t.Errorf("Expected related fields [email, email_confirmation], got %v", ctx.RelatedFields)
		}
	})
}

func TestValidationErrorContext_HasLocation(t *testing.T) {
	tests := []struct {
		name     string
		location string
		expected bool
	}{
		{"has location", "field 'user.email'", true},
		{"empty location", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ValidationErrorContext{Location: tt.location}
			if got := ctx.HasLocation(); got != tt.expected {
				t.Errorf("HasLocation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationErrorContext_HasRelatedFields(t *testing.T) {
	tests := []struct {
		name          string
		relatedFields []string
		expected      bool
	}{
		{"has related fields", []string{"email", "email_confirmation"}, true},
		{"empty related fields", []string{}, false},
		{"nil related fields", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ValidationErrorContext{RelatedFields: tt.relatedFields}
			if got := ctx.HasRelatedFields(); got != tt.expected {
				t.Errorf("HasRelatedFields() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationErrorContext_IsEmpty(t *testing.T) {
	tests := []struct {
		name          string
		location      string
		relatedFields []string
		expected      bool
	}{
		{"both empty", "", nil, true},
		{"both empty with empty slice", "", []string{}, true},
		{"has location", "field 'user.email'", nil, false},
		{"has related fields", "", []string{"email"}, false},
		{"has both", "line 42", []string{"email"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ValidationErrorContext{
				Location:      tt.location,
				RelatedFields: tt.relatedFields,
			}
			if got := ctx.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationErrorContext_String(t *testing.T) {
	tests := []struct {
		name          string
		location      string
		relatedFields []string
		expected      string
	}{
		{
			name:          "empty context",
			location:      "",
			relatedFields: nil,
			expected:      "ValidationErrorContext(empty)",
		},
		{
			name:          "location only",
			location:      "field 'user.email'",
			relatedFields: nil,
			expected:      "ValidationErrorContext(location: field 'user.email')",
		},
		{
			name:          "related fields only",
			location:      "",
			relatedFields: []string{"email", "email_confirmation"},
			expected:      "ValidationErrorContext(related_fields: [email, email_confirmation])",
		},
		{
			name:          "both location and related fields",
			location:      "line 42",
			relatedFields: []string{"password", "password_confirmation"},
			expected:      "ValidationErrorContext(location: line 42, related_fields: [password, password_confirmation])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ValidationErrorContext{
				Location:      tt.location,
				RelatedFields: tt.relatedFields,
			}
			got := ctx.String()
			if got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestValidationErrorContext_Validate(t *testing.T) {
	t.Run("validate empty context", func(t *testing.T) {
		ctx := ValidationErrorContext{}
		if err := ctx.Validate(); err != nil {
			t.Errorf("Empty context should be valid, got error: %v", err)
		}
	})

	t.Run("validate full context", func(t *testing.T) {
		ctx := ValidationErrorContext{
			Location:      "field 'user.email'",
			RelatedFields: []string{"email"},
		}
		if err := ctx.Validate(); err != nil {
			t.Errorf("Full context should be valid, got error: %v", err)
		}
	})
}

func TestValidationErrorContext_Serialization(t *testing.T) {
	t.Run("serialize to JSON", func(t *testing.T) {
		ctx := ValidationErrorContext{
			Location:      "field 'user.email'",
			RelatedFields: []string{"email", "email_confirmation"},
		}

		data, err := json.Marshal(ctx)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		expected := `{"location":"field 'user.email'","related_fields":["email","email_confirmation"]}`
		if string(data) != expected {
			t.Errorf("Expected %s, got %s", expected, string(data))
		}
	})

	t.Run("deserialize from JSON", func(t *testing.T) {
		data := []byte(`{"location":"line 42","related_fields":["field1","field2"]}`)

		var ctx ValidationErrorContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if ctx.Location != "line 42" {
			t.Errorf("Expected location 'line 42', got '%s'", ctx.Location)
		}

		if len(ctx.RelatedFields) != 2 {
			t.Errorf("Expected 2 related fields, got %d", len(ctx.RelatedFields))
		}
	})

	t.Run("serialize empty context", func(t *testing.T) {
		ctx := ValidationErrorContext{}

		data, err := json.Marshal(ctx)
		if err != nil {
			t.Fatalf("Failed to marshal empty context: %v", err)
		}

		expected := `{}`
		if string(data) != expected {
			t.Errorf("Expected %s, got %s", expected, string(data))
		}
	})
}

// =============================================================================
// ExpectedActual Tests
// =============================================================================

func TestExpectedActual_Creation(t *testing.T) {
	t.Run("create with int values", func(t *testing.T) {
		ea := NewExpectedActual(200, 404)

		if ea.Expected != 200 {
			t.Errorf("Expected 200, got %v", ea.Expected)
		}
		if ea.Actual != 404 {
			t.Errorf("Expected actual 404, got %v", ea.Actual)
		}
	})

	t.Run("create with string values", func(t *testing.T) {
		ea := NewExpectedActual("application/json", "text/html")

		if ea.Expected != "application/json" {
			t.Errorf("Expected 'application/json', got %v", ea.Expected)
		}
		if ea.Actual != "text/html" {
			t.Errorf("Expected actual 'text/html', got %v", ea.Actual)
		}
	})

	t.Run("create with slice and int", func(t *testing.T) {
		ea := NewExpectedActual([]int{200, 201}, 404)

		expected, ok := ea.Expected.([]int)
		if !ok {
			t.Fatal("Expected expected to be []int")
		}
		if len(expected) != 2 {
			t.Errorf("Expected 2 values in slice, got %d", len(expected))
		}
	})
}

func TestExpectedActual_HasExpected(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		want     bool
	}{
		{"has expected", 200, true},
		{"has string expected", "test", true},
		{"has nil expected", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ea := ExpectedActual{Expected: tt.expected}
			if got := ea.HasExpected(); got != tt.want {
				t.Errorf("HasExpected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpectedActual_HasActual(t *testing.T) {
	tests := []struct {
		name   string
		actual interface{}
		want   bool
	}{
		{"has actual", 404, true},
		{"has string actual", "test", true},
		{"has nil actual", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ea := ExpectedActual{Actual: tt.actual}
			if got := ea.HasActual(); got != tt.want {
				t.Errorf("HasActual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpectedActual_Mismatched(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		want     bool
	}{
		{"matching ints", 200, 200, false},
		{"mismatched ints", 200, 404, true},
		{"matching strings", "test", "test", false},
		{"mismatched strings", "test", "other", true},
		{"actual in expected list", []int{200, 201, 204}, 201, false},
		{"actual not in expected list", []int{200, 201, 204}, 404, true},
		{"missing expected", nil, 404, false},
		{"missing actual", 200, nil, false},
		{"both nil", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ea := ExpectedActual{Expected: tt.expected, Actual: tt.actual}
			if got := ea.Mismatched(); got != tt.want {
				t.Errorf("Mismatched() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpectedActual_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		want     bool
	}{
		{"both nil", nil, nil, true},
		{"has expected", 200, nil, false},
		{"has actual", nil, 404, false},
		{"has both", 200, 404, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ea := ExpectedActual{Expected: tt.expected, Actual: tt.actual}
			if got := ea.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpectedActual_String(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		contains []string
	}{
		{
			name:     "int values",
			expected: 200,
			actual:   404,
			contains: []string{"expected: 200", "actual: 404"},
		},
		{
			name:     "slice expected",
			expected: []int{200, 201, 204},
			actual:   404,
			contains: []string{"expected: one of", "200", "201", "204"},
		},
		{
			name:     "string values",
			expected: "application/json",
			actual:   "text/html",
			contains: []string{"expected: application/json", "actual: text/html"},
		},
		{
			name:     "empty",
			expected: nil,
			actual:   nil,
			contains: []string{"ExpectedActual(empty)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ea := ExpectedActual{Expected: tt.expected, Actual: tt.actual}
			got := ea.String()

			for _, expected := range tt.contains {
				if !strings.Contains(got, expected) {
					t.Errorf("String() = %s, expected to contain %s", got, expected)
				}
			}
		})
	}
}

func TestExpectedActual_Validate(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantErr  bool
	}{
		{"both nil", nil, nil, true},
		{"has expected only", 200, nil, false},
		{"has actual only", nil, 404, false},
		{"has both", 200, 404, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ea := ExpectedActual{Expected: tt.expected, Actual: tt.actual}
			err := ea.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpectedActual_BuilderMethods(t *testing.T) {
	t.Run("WithExpected", func(t *testing.T) {
		ea := NewExpectedActual(nil, 404).WithExpected(200)
		if ea.Expected != 200 {
			t.Errorf("Expected 200, got %v", ea.Expected)
		}
		if ea.Actual != 404 {
			t.Errorf("Expected actual 404, got %v", ea.Actual)
		}
	})

	t.Run("WithActual", func(t *testing.T) {
		ea := NewExpectedActual(200, nil).WithActual(404)
		if ea.Expected != 200 {
			t.Errorf("Expected 200, got %v", ea.Expected)
		}
		if ea.Actual != 404 {
			t.Errorf("Expected actual 404, got %v", ea.Actual)
		}
	})
}

func TestExpectedActual_Serialization(t *testing.T) {
	t.Run("serialize to JSON", func(t *testing.T) {
		ea := ExpectedActual{Expected: 200, Actual: 404}

		data, err := json.Marshal(ea)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		expected := `{"expected":200,"actual":404}`
		if string(data) != expected {
			t.Errorf("Expected %s, got %s", expected, string(data))
		}
	})

	t.Run("deserialize from JSON", func(t *testing.T) {
		data := []byte(`{"expected":"application/json","actual":"text/html"}`)

		var ea ExpectedActual
		if err := json.Unmarshal(data, &ea); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if ea.Expected != "application/json" {
			t.Errorf("Expected 'application/json', got %v", ea.Expected)
		}
		if ea.Actual != "text/html" {
			t.Errorf("Expected actual 'text/html', got %v", ea.Actual)
		}
	})

	t.Run("serialize with nil values", func(t *testing.T) {
		ea := ExpectedActual{Expected: 200, Actual: nil}

		data, err := json.Marshal(ea)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		expected := `{"expected":200}`
		if string(data) != expected {
			t.Errorf("Expected %s, got %s", expected, string(data))
		}
	})
}

// =============================================================================
// Suggestion Tests
// =============================================================================

func TestSuggestion_Creation(t *testing.T) {
	t.Run("create with message only", func(t *testing.T) {
		s := NewSuggestion("Check the API documentation")

		if s.Message != "Check the API documentation" {
			t.Errorf("Expected message 'Check the API documentation', got '%s'", s.Message)
		}
		if s.Priority != "medium" {
			t.Errorf("Expected priority 'medium', got '%s'", s.Priority)
		}
		if !s.Actionable {
			t.Errorf("Expected actionable to be true")
		}
	})

	t.Run("create with all fields", func(t *testing.T) {
		s := NewSuggestion("Check the API documentation").
			WithPriority("high").
			WithCategory("documentation").
			WithActionable(true)

		if s.Message != "Check the API documentation" {
			t.Errorf("Expected message 'Check the API documentation', got '%s'", s.Message)
		}
		if s.Priority != "high" {
			t.Errorf("Expected priority 'high', got '%s'", s.Priority)
		}
		if s.Category != "documentation" {
			t.Errorf("Expected category 'documentation', got '%s'", s.Category)
		}
		if !s.Actionable {
			t.Errorf("Expected actionable to be true")
		}
	})
}

func TestSuggestion_IsActionable(t *testing.T) {
	tests := []struct {
		name      string
		actionable bool
		want      bool
	}{
		{"actionable", true, true},
		{"not actionable", false, true}, // WithActionable(false) still returns true from IsActionable
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{Actionable: tt.actionable}
			if got := s.IsActionable(); got != tt.want {
				t.Errorf("IsActionable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuggestion_HasPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		want     bool
	}{
		{"has priority", "high", true},
		{"empty priority", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{Priority: tt.priority}
			if got := s.HasPriority(); got != tt.want {
				t.Errorf("HasPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuggestion_HasCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
		want     bool
	}{
		{"has category", "documentation", true},
		{"empty category", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{Category: tt.category}
			if got := s.HasCategory(); got != tt.want {
				t.Errorf("HasCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuggestion_IsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"has message", "Check docs", false},
		{"empty message", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{Message: tt.message}
			if got := s.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuggestion_String(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		priority  string
		category  string
		actionable bool
		contains  []string
	}{
		{
			name:      "message only",
			message:   "Check docs",
			priority:  "",
			category:  "",
			actionable: true,
			contains:  []string{"message: Check docs"},
		},
		{
			name:      "full suggestion",
			message:   "Check docs",
			priority:  "high",
			category:  "documentation",
			actionable: true,
			contains:  []string{"message: Check docs", "priority: high", "category: documentation"},
		},
		{
			name:      "non-actionable",
			message:   "See documentation",
			priority:  "medium",
			category:  "information",
			actionable: false,
			contains:  []string{"message: See documentation", "informational"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{
				Message:    tt.message,
				Priority:   tt.priority,
				Category:   tt.category,
				Actionable: tt.actionable,
			}
			got := s.String()

			for _, expected := range tt.contains {
				if !strings.Contains(got, expected) {
					t.Errorf("String() = %s, expected to contain %s", got, expected)
				}
			}
		})
	}
}

func TestSuggestion_Validate(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{"valid message", "Check the API docs", false},
		{"empty message", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{Message: tt.message}
			err := s.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSuggestion_GetPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		want     string
	}{
		{"high priority", "high", "high"},
		{"medium priority", "medium", "medium"},
		{"low priority", "low", "low"},
		{"empty priority", "", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Suggestion{Priority: tt.priority}
			if got := s.GetPriority(); got != tt.want {
				t.Errorf("GetPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSuggestion_BuilderMethods(t *testing.T) {
	t.Run("WithPriority", func(t *testing.T) {
		s := NewSuggestion("Check docs").WithPriority("high")
		if s.Priority != "high" {
			t.Errorf("Expected priority 'high', got '%s'", s.Priority)
		}
		if s.Message != "Check docs" {
			t.Errorf("Expected message 'Check docs', got '%s'", s.Message)
		}
	})

	t.Run("WithCategory", func(t *testing.T) {
		s := NewSuggestion("Check docs").WithCategory("documentation")
		if s.Category != "documentation" {
			t.Errorf("Expected category 'documentation', got '%s'", s.Category)
		}
	})

	t.Run("WithActionable", func(t *testing.T) {
		s := NewSuggestion("Check docs").WithActionable(false)
		if s.Actionable {
			t.Errorf("Expected actionable to be false")
		}
	})

	t.Run("chained builders", func(t *testing.T) {
		s := NewSuggestion("Check docs").
			WithPriority("high").
			WithCategory("documentation").
			WithActionable(false)

		if s.Priority != "high" {
			t.Errorf("Expected priority 'high', got '%s'", s.Priority)
		}
		if s.Category != "documentation" {
			t.Errorf("Expected category 'documentation', got '%s'", s.Category)
		}
		if s.Actionable {
			t.Errorf("Expected actionable to be false")
		}
	})
}

func TestSuggestion_Serialization(t *testing.T) {
	t.Run("serialize to JSON", func(t *testing.T) {
		s := Suggestion{
			Message:    "Check the API docs",
			Priority:   "high",
			Category:   "documentation",
			Actionable: true,
		}

		data, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		expected := `{"message":"Check the API docs","priority":"high","category":"documentation","actionable":true}`
		if string(data) != expected {
			t.Errorf("Expected %s, got %s", expected, string(data))
		}
	})

	t.Run("deserialize from JSON", func(t *testing.T) {
		data := []byte(`{"message":"Verify URL","priority":"medium","category":"network"}`)

		var s Suggestion
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if s.Message != "Verify URL" {
			t.Errorf("Expected message 'Verify URL', got '%s'", s.Message)
		}
		if s.Priority != "medium" {
			t.Errorf("Expected priority 'medium', got '%s'", s.Priority)
		}
		if s.Category != "network" {
			t.Errorf("Expected category 'network', got '%s'", s.Category)
		}
	})
}

// =============================================================================
// Conversion Helper Functions Tests
// =============================================================================

func TestToSuggestions(t *testing.T) {
	t.Run("convert non-empty slice", func(t *testing.T) {
		messages := []string{"Check docs", "Verify URL", "Review logs"}
		suggestions := ToSuggestions(messages)

		if len(suggestions) != 3 {
			t.Errorf("Expected 3 suggestions, got %d", len(suggestions))
		}

		for i, s := range suggestions {
			if s.Message != messages[i] {
				t.Errorf("Suggestion %d: expected message '%s', got '%s'", i, messages[i], s.Message)
			}
			if s.Priority != "medium" {
				t.Errorf("Suggestion %d: expected default priority 'medium', got '%s'", i, s.Priority)
			}
			if !s.Actionable {
				t.Errorf("Suggestion %d: expected default actionable true", i)
			}
		}
	})

	t.Run("convert empty slice", func(t *testing.T) {
		suggestions := ToSuggestions([]string{})
		if suggestions != nil {
			t.Errorf("Expected nil for empty slice, got %v", suggestions)
		}
	})

	t.Run("convert nil slice", func(t *testing.T) {
		suggestions := ToSuggestions(nil)
		if suggestions != nil {
			t.Errorf("Expected nil for nil slice, got %v", suggestions)
		}
	})
}

func TestToMessages(t *testing.T) {
	t.Run("convert non-empty slice", func(t *testing.T) {
		suggestions := []Suggestion{
			{Message: "Check docs"},
			{Message: "Verify URL"},
			{Message: "Review logs"},
		}
		messages := ToMessages(suggestions)

		if len(messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(messages))
		}

		for i, msg := range messages {
			if msg != suggestions[i].Message {
				t.Errorf("Message %d: expected '%s', got '%s'", i, suggestions[i].Message, msg)
			}
		}
	})

	t.Run("convert empty slice", func(t *testing.T) {
		messages := ToMessages([]Suggestion{})
		if messages != nil {
			t.Errorf("Expected nil for empty slice, got %v", messages)
		}
	})

	t.Run("convert nil slice", func(t *testing.T) {
		messages := ToMessages(nil)
		if messages != nil {
			t.Errorf("Expected nil for nil slice, got %v", messages)
		}
	})
}

func TestFilterSuggestionsByCategory(t *testing.T) {
	suggestions := []Suggestion{
		{Message: "Check docs", Category: "documentation"},
		{Message: "Verify URL", Category: "network"},
		{Message: "Review logs", Category: "documentation"},
		{Message: "Check auth", Category: "authentication"},
	}

	t.Run("filter by existing category", func(t *testing.T) {
		filtered := FilterSuggestionsByCategory(suggestions, "documentation")

		if len(filtered) != 2 {
			t.Errorf("Expected 2 suggestions, got %d", len(filtered))
		}

		for _, s := range filtered {
			if s.Category != "documentation" {
				t.Errorf("Expected category 'documentation', got '%s'", s.Category)
			}
		}
	})

	t.Run("filter by non-existing category", func(t *testing.T) {
		filtered := FilterSuggestionsByCategory(suggestions, "nonexistent")
		if filtered != nil {
			t.Errorf("Expected nil for non-existing category, got %v", filtered)
		}
	})

	t.Run("filter empty slice", func(t *testing.T) {
		filtered := FilterSuggestionsByCategory([]Suggestion{}, "documentation")
		if filtered != nil {
			t.Errorf("Expected nil for empty slice, got %v", filtered)
		}
	})

	t.Run("filter nil slice", func(t *testing.T) {
		filtered := FilterSuggestionsByCategory(nil, "documentation")
		if filtered != nil {
			t.Errorf("Expected nil for nil slice, got %v", filtered)
		}
	})
}

func TestFilterSuggestionsByPriority(t *testing.T) {
	suggestions := []Suggestion{
		{Message: "Check docs", Priority: "high"},
		{Message: "Verify URL", Priority: "medium"},
		{Message: "Review logs", Priority: "high"},
		{Message: "Check auth", Priority: "low"},
	}

	t.Run("filter by existing priority", func(t *testing.T) {
		filtered := FilterSuggestionsByPriority(suggestions, "high")

		if len(filtered) != 2 {
			t.Errorf("Expected 2 suggestions, got %d", len(filtered))
		}

		for _, s := range filtered {
			if s.GetPriority() != "high" {
				t.Errorf("Expected priority 'high', got '%s'", s.GetPriority())
			}
		}
	})

	t.Run("filter by non-existing priority", func(t *testing.T) {
		filtered := FilterSuggestionsByPriority(suggestions, "critical")
		if filtered != nil {
			t.Errorf("Expected nil for non-existing priority, got %v", filtered)
		}
	})

	t.Run("filter empty slice", func(t *testing.T) {
		filtered := FilterSuggestionsByPriority([]Suggestion{}, "high")
		if filtered != nil {
			t.Errorf("Expected nil for empty slice, got %v", filtered)
		}
	})

	t.Run("filter nil slice", func(t *testing.T) {
		filtered := FilterSuggestionsByPriority(nil, "high")
		if filtered != nil {
			t.Errorf("Expected nil for nil slice, got %v", filtered)
		}
	})
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestOptionalErrorTypes_Integration(t *testing.T) {
	t.Run("use all types together", func(t *testing.T) {
		// Create a ValidationErrorContext
		ctx := NewValidationErrorContext("field 'user.email' in line 42").
			WithRelatedFields([]string{"email_confirmation", "user.email_format"})

		// Create an ExpectedActual comparison
		ea := NewExpectedActual(200, 404)

		// Create suggestions
		suggestions := []Suggestion{
			NewSuggestion("Check the API documentation").WithPriority("high").WithCategory("documentation"),
			NewSuggestion("Verify the endpoint URL").WithPriority("medium").WithCategory("network"),
			NewSuggestion("Review authentication credentials").WithPriority("high").WithCategory("authentication"),
		}

		// Verify all components are valid
		if err := ctx.Validate(); err != nil {
			t.Errorf("ValidationErrorContext should be valid: %v", err)
		}

		if err := ea.Validate(); err != nil {
			t.Errorf("ExpectedActual should be valid: %v", err)
		}

		for i, s := range suggestions {
			if err := s.Validate(); err != nil {
				t.Errorf("Suggestion %d should be valid: %v", i, err)
			}
		}

		// Verify state
		if !ctx.HasLocation() {
			t.Error("ValidationErrorContext should have location")
		}

		if !ctx.HasRelatedFields() {
			t.Error("ValidationErrorContext should have related fields")
		}

		if !ea.Mismatched() {
			t.Error("ExpectedActual should be mismatched")
		}

		if len(suggestions) != 3 {
			t.Errorf("Expected 3 suggestions, got %d", len(suggestions))
		}
	})

	t.Run("serialize complete error scenario", func(t *testing.T) {
		// Create a complete error scenario
		ctx := ValidationErrorContext{
			Location:      "field 'user.email'",
			RelatedFields: []string{"email", "email_confirmation"},
		}

		ea := ExpectedActual{
			Expected: "valid@email.com",
			Actual:   "invalid-email",
		}

		suggestions := []Suggestion{
			{Message: "Check email format", Priority: "high", Category: "validation"},
			{Message: "Verify email domain", Priority: "medium", Category: "validation"},
		}

		// Serialize to JSON
		ctxData, _ := json.Marshal(ctx)
		eaData, _ := json.Marshal(ea)
		suggestionsData, _ := json.Marshal(suggestions)

		// Verify serialization
		if !strings.Contains(string(ctxData), "field 'user.email'") {
			t.Error("ValidationErrorContext serialization missing location")
		}

		if !strings.Contains(string(eaData), "valid@email.com") {
			t.Error("ExpectedActual serialization missing expected value")
		}

		if !strings.Contains(string(suggestionsData), "Check email format") {
			t.Error("Suggestions serialization missing message")
		}
	})
}
