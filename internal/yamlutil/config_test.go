// Package yamlutil tests for configuration functions
package yamlutil

import (
	"testing"
	"time"
)

func TestPerformanceParserConfig(t *testing.T) {
	config := PerformanceParserConfig()

	if config == nil {
		t.Fatal("PerformanceParserConfig returned nil")
	}

	tests := []struct {
		name     string
		check    func(*ParserConfig) bool
		expected bool
	}{
		{"StrictMode should be false", func(c *ParserConfig) bool { return c.StrictMode == false }, true},
		{"VerboseErrors should be false", func(c *ParserConfig) bool { return c.VerboseErrors == false }, true},
		{"IncludeLineInfo should be false", func(c *ParserConfig) bool { return c.IncludeLineInfo == false }, true},
		{"ErrorContextLines should be 0", func(c *ParserConfig) bool { return c.ErrorContextLines == 0 }, true},
		{"EnableCaching should be true", func(c *ParserConfig) bool { return c.EnableCaching == true }, true},
		{"CacheTTL should be 30 minutes", func(c *ParserConfig) bool { return c.CacheTTL == 30*time.Minute }, true},
		{"MaxCacheSize should be 1000", func(c *ParserConfig) bool { return c.MaxCacheSize == 1000 }, true},
		{"EnableStreaming should be true", func(c *ParserConfig) bool { return c.EnableStreaming == true }, true},
		{"StreamBufferSize should be 16384", func(c *ParserConfig) bool { return c.StreamBufferSize == 16384 }, true},
		{"MaxFileSize should be 100MB", func(c *ParserConfig) bool { return c.MaxFileSize == 100*1024*1024 }, true},
		{"ValidateAfterParse should be false", func(c *ParserConfig) bool { return c.ValidateAfterParse == false }, true},
		{"ExplicitTypeTags should be false", func(c *ParserConfig) bool { return c.ExplicitTypeTags == false }, true},
		{"CoerceTypes should be true", func(c *ParserConfig) bool { return c.CoerceTypes == true }, true},
		{"DefaultZeroValues should be true", func(c *ParserConfig) bool { return c.DefaultZeroValues == true }, true},
		{"MultiDocument should be true", func(c *ParserConfig) bool { return c.MultiDocument == true }, true},
		{"DocumentSeparator should be ---", func(c *ParserConfig) bool { return c.DocumentSeparator == "---" }, true},
		{"CustomResolvers should be nil", func(c *ParserConfig) bool { return c.CustomResolvers == nil }, true},
		{"PostProcessors should be nil", func(c *ParserConfig) bool { return c.PostProcessors == nil }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.check(config); result != tt.expected {
				t.Errorf("%s failed", tt.name)
			}
		})
	}
}

func TestDefaultValidatorConfig(t *testing.T) {
	config := DefaultValidatorConfig()

	if config == nil {
		t.Fatal("DefaultValidatorConfig returned nil")
	}

	tests := []struct {
		name     string
		check    func(*ValidatorConfig) bool
		expected bool
	}{
		{"StrictMode should be false", func(c *ValidatorConfig) bool { return c.StrictMode == false }, true},
		{"RequireAllFields should be false", func(c *ValidatorConfig) bool { return c.RequireAllFields == false }, true},
		{"RejectUnknownKeys should be false", func(c *ValidatorConfig) bool { return c.RejectUnknownKeys == false }, true},
		{"VerboseErrors should be true", func(c *ValidatorConfig) bool { return c.VerboseErrors == true }, true},
		{"MaxErrors should be 50", func(c *ValidatorConfig) bool { return c.MaxErrors == 50 }, true},
		{"StopAtFirstError should be false", func(c *ValidatorConfig) bool { return c.StopAtFirstError == false }, true},
		{"WarningThreshold should be 10", func(c *ValidatorConfig) bool { return c.WarningThreshold == 10 }, true},
		{"EnableSchemaValidation should be false", func(c *ValidatorConfig) bool { return c.EnableSchemaValidation == false }, true},
		{"SchemaPaths should be nil", func(c *ValidatorConfig) bool { return c.SchemaPaths == nil }, true},
		{"SchemaValidationMode should be Lenient", func(c *ValidatorConfig) bool { return c.SchemaValidationMode == SchemaModeLenient }, true},
		{"EnableConstraints should be true", func(c *ValidatorConfig) bool { return c.EnableConstraints == true }, true},
		{"ConstraintMode should be Warn", func(c *ValidatorConfig) bool { return c.ConstraintMode == ConstraintModeWarn }, true},
		{"CheckDuplicateKeys should be true", func(c *ValidatorConfig) bool { return c.CheckDuplicateKeys == true }, true},
		{"CheckCircularRefs should be false", func(c *ValidatorConfig) bool { return c.CheckCircularRefs == false }, true},
		{"CheckDeprecatedSyntax should be false", func(c *ValidatorConfig) bool { return c.CheckDeprecatedSyntax == false }, true},
		{"ValidateTypes should be true", func(c *ValidatorConfig) bool { return c.ValidateTypes == true }, true},
		{"ValidateRanges should be false", func(c *ValidatorConfig) bool { return c.ValidateRanges == false }, true},
		{"ValidatePatterns should be false", func(c *ValidatorConfig) bool { return c.ValidatePatterns == false }, true},
		{"ValidateLengths should be false", func(c *ValidatorConfig) bool { return c.ValidateLengths == false }, true},
		{"CustomValidators should be nil", func(c *ValidatorConfig) bool { return c.CustomValidators == nil }, true},
		{"SchemaValidators should be nil", func(c *ValidatorConfig) bool { return c.SchemaValidators == nil }, true},
		{"EnableValidationCache should be false", func(c *ValidatorConfig) bool { return c.EnableValidationCache == false }, true},
		{"CacheInvalidFiles should be false", func(c *ValidatorConfig) bool { return c.CacheInvalidFiles == false }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.check(config); result != tt.expected {
				t.Errorf("%s failed", tt.name)
			}
		})
	}
}

func TestStrictValidatorConfig(t *testing.T) {
	config := StrictValidatorConfig()

	if config == nil {
		t.Fatal("StrictValidatorConfig returned nil")
	}

	tests := []struct {
		name     string
		check    func(*ValidatorConfig) bool
		expected bool
	}{
		{"StrictMode should be true", func(c *ValidatorConfig) bool { return c.StrictMode == true }, true},
		{"RequireAllFields should be true", func(c *ValidatorConfig) bool { return c.RequireAllFields == true }, true},
		{"RejectUnknownKeys should be true", func(c *ValidatorConfig) bool { return c.RejectUnknownKeys == true }, true},
		{"VerboseErrors should be true", func(c *ValidatorConfig) bool { return c.VerboseErrors == true }, true},
		{"MaxErrors should be 100", func(c *ValidatorConfig) bool { return c.MaxErrors == 100 }, true},
		{"StopAtFirstError should be false", func(c *ValidatorConfig) bool { return c.StopAtFirstError == false }, true},
		{"WarningThreshold should be 0", func(c *ValidatorConfig) bool { return c.WarningThreshold == 0 }, true},
		{"EnableSchemaValidation should be true", func(c *ValidatorConfig) bool { return c.EnableSchemaValidation == true }, true},
		{"SchemaValidationMode should be Strict", func(c *ValidatorConfig) bool { return c.SchemaValidationMode == SchemaModeStrict }, true},
		{"ConstraintMode should be Error", func(c *ValidatorConfig) bool { return c.ConstraintMode == ConstraintModeError }, true},
		{"CheckCircularRefs should be true", func(c *ValidatorConfig) bool { return c.CheckCircularRefs == true }, true},
		{"CheckDeprecatedSyntax should be true", func(c *ValidatorConfig) bool { return c.CheckDeprecatedSyntax == true }, true},
		{"ValidateRanges should be true", func(c *ValidatorConfig) bool { return c.ValidateRanges == true }, true},
		{"ValidatePatterns should be true", func(c *ValidatorConfig) bool { return c.ValidatePatterns == true }, true},
		{"ValidateLengths should be true", func(c *ValidatorConfig) bool { return c.ValidateLengths == true }, true},
		{"EnableValidationCache should be true", func(c *ValidatorConfig) bool { return c.EnableValidationCache == true }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.check(config); result != tt.expected {
				t.Errorf("%s failed", tt.name)
			}
		})
	}
}

func TestLenientValidatorConfig(t *testing.T) {
	config := LenientValidatorConfig()

	if config == nil {
		t.Fatal("LenientValidatorConfig returned nil")
	}

	tests := []struct {
		name     string
		check    func(*ValidatorConfig) bool
		expected bool
	}{
		{"StrictMode should be false", func(c *ValidatorConfig) bool { return c.StrictMode == false }, true},
		{"RequireAllFields should be false", func(c *ValidatorConfig) bool { return c.RequireAllFields == false }, true},
		{"RejectUnknownKeys should be false", func(c *ValidatorConfig) bool { return c.RejectUnknownKeys == false }, true},
		{"VerboseErrors should be true", func(c *ValidatorConfig) bool { return c.VerboseErrors == true }, true},
		{"MaxErrors should be 25", func(c *ValidatorConfig) bool { return c.MaxErrors == 25 }, true},
		{"StopAtFirstError should be false", func(c *ValidatorConfig) bool { return c.StopAtFirstError == false }, true},
		{"WarningThreshold should be 20", func(c *ValidatorConfig) bool { return c.WarningThreshold == 20 }, true},
		{"EnableSchemaValidation should be false", func(c *ValidatorConfig) bool { return c.EnableSchemaValidation == false }, true},
		{"SchemaValidationMode should be Lenient", func(c *ValidatorConfig) bool { return c.SchemaValidationMode == SchemaModeLenient }, true},
		{"ConstraintMode should be Warn", func(c *ValidatorConfig) bool { return c.ConstraintMode == ConstraintModeWarn }, true},
		{"CheckDuplicateKeys should be true", func(c *ValidatorConfig) bool { return c.CheckDuplicateKeys == true }, true},
		{"CheckCircularRefs should be false", func(c *ValidatorConfig) bool { return c.CheckCircularRefs == false }, true},
		{"ValidateTypes should be false", func(c *ValidatorConfig) bool { return c.ValidateTypes == false }, true},
		{"ValidateRanges should be false", func(c *ValidatorConfig) bool { return c.ValidateRanges == false }, true},
		{"ValidatePatterns should be false", func(c *ValidatorConfig) bool { return c.ValidatePatterns == false }, true},
		{"ValidateLengths should be false", func(c *ValidatorConfig) bool { return c.ValidateLengths == false }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.check(config); result != tt.expected {
				t.Errorf("%s failed", tt.name)
			}
		})
	}
}

func TestDefaultSchemaConfig(t *testing.T) {
	config := DefaultSchemaConfig()

	if config == nil {
		t.Fatal("DefaultSchemaConfig returned nil")
	}

	tests := []struct {
		name     string
		check    func(*SchemaConfig) bool
		expected bool
	}{
		{"SchemaPaths should be nil", func(c *SchemaConfig) bool { return c.SchemaPaths == nil }, true},
		{"SchemaURIs should be nil", func(c *SchemaConfig) bool { return c.SchemaURIs == nil }, true},
		{"SchemaStrings should be nil", func(c *SchemaConfig) bool { return c.SchemaStrings == nil }, true},
		{"EnableReloading should be false", func(c *SchemaConfig) bool { return c.EnableReloading == false }, true},
		{"ReloadInterval should be 60s", func(c *SchemaConfig) bool { return c.ReloadInterval == 60*time.Second }, true},
		{"CacheSchemas should be true", func(c *SchemaConfig) bool { return c.CacheSchemas == true }, true},
		{"ValidationMode should be Lenient", func(c *SchemaConfig) bool { return c.ValidationMode == SchemaModeLenient }, true},
		{"RequireAllFields should be false", func(c *SchemaConfig) bool { return c.RequireAllFields == false }, true},
		{"RejectUnknownKeys should be false", func(c *SchemaConfig) bool { return c.RejectUnknownKeys == false }, true},
		{"EnableTypeCheck should be true", func(c *SchemaConfig) bool { return c.EnableTypeCheck == true }, true},
		{"StrictTypes should be false", func(c *SchemaConfig) bool { return c.StrictTypes == false }, true},
		{"CustomTypes should be nil", func(c *SchemaConfig) bool { return c.CustomTypes == nil }, true},
		{"CustomConstraints should be nil", func(c *SchemaConfig) bool { return c.CustomConstraints == nil }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.check(config); result != tt.expected {
				t.Errorf("%s failed", tt.name)
			}
		})
	}
}

func TestSchemaModeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value SchemaMode
	}{
		{"SchemaModeDisabled", SchemaModeDisabled},
		{"SchemaModeLenient", SchemaModeLenient},
		{"SchemaModeStrict", SchemaModeStrict},
		{"SchemaModeRequired", SchemaModeRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure the constants are defined and usable
			_ = tt.value
		})
	}
}

func TestConstraintModeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value ConstraintMode
	}{
		{"ConstraintModeDisabled", ConstraintModeDisabled},
		{"ConstraintModeWarn", ConstraintModeWarn},
		{"ConstraintModeError", ConstraintModeError},
		{"ConstraintModeFatal", ConstraintModeFatal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure the constants are defined and usable
			_ = tt.value
		})
	}
}
