// Package yamlutil provides configuration interfaces for YAML processing components.
//
// Configuration interfaces enable flexible customization of parser and validator
// behavior without requiring changes to core implementation code.
package yamlutil

import (
	"time"
)

// ParserConfig defines configuration options for YAML parsers.
//
// ParserConfig provides comprehensive control over parsing behavior including
// strict mode, error handling, caching, and performance tuning.
type ParserConfig struct {
	// Strict parsing mode
	StrictMode bool // Reject unknown fields and enforce strict YAML rules

	// Error handling
	VerboseErrors    bool          // Include detailed context in error messages
	IncludeLineInfo  bool          // Always include line/column information in errors
	ErrorContextLines int          // Number of context lines to include in errors (0 = none)

	// Caching
	EnableCaching     bool         // Cache parsed YAML documents to avoid re-parsing
	CacheTTL         time.Duration // How long to keep cached documents
	MaxCacheSize     int          // Maximum number of documents to cache (0 = unlimited)

	// Performance
	EnableStreaming   bool         // Enable streaming for large files (experimental)
	StreamBufferSize  int          // Buffer size for streaming (bytes)
	MaxFileSize       int64        // Maximum file size to accept (bytes, 0 = unlimited)

	// Validation integration
	ValidateAfterParse bool         // Automatically validate after parsing
	ValidatorConfig   *ValidatorConfig // Validator config to use for auto-validation

	// Type handling
	ExplicitTypeTags  bool         // Require explicit YAML type tags (!!str, !!int, etc.)
	CoerceTypes       bool         // Automatically coerce between compatible types
	DefaultZeroValues bool         // Use zero values for missing fields instead of errors

	// Document handling
	MultiDocument     bool         // Support multi-document YAML files
	DocumentSeparator string       // Separator between documents (default: "---")

	// Custom options
	CustomResolvers   []TypeResolver // Custom type resolution functions
	PostProcessors    []PostProcessor // Functions to run after parsing
}

// TypeResolver defines a function for custom type resolution during parsing.
//
// TypeResolvers allow custom logic for determining how to interpret YAML values,
// enabling domain-specific parsing behaviors.
type TypeResolver func(fieldPath string, value interface{}, expectedType string) (interface{}, error)

// PostProcessor defines a function for post-processing parsed YAML data.
//
// PostProcessors can transform, validate, or enrich parsed data before it's
// returned to the caller.
type PostProcessor func(filePath string, data interface{}) (interface{}, error)

// DefaultParserConfig returns a ParserConfig with sensible defaults.
//
// The default configuration provides lenient parsing suitable for development
// environments with basic error reporting and no caching.
func DefaultParserConfig() *ParserConfig {
	return &ParserConfig{
		StrictMode:         false,
		VerboseErrors:      true,
		IncludeLineInfo:    true,
		ErrorContextLines:  2,
		EnableCaching:      false,
		CacheTTL:           5 * time.Minute,
		MaxCacheSize:       100,
		EnableStreaming:    false,
		StreamBufferSize:   4096,
		MaxFileSize:        10 * 1024 * 1024, // 10MB
		ValidateAfterParse: false,
		ExplicitTypeTags:   false,
		CoerceTypes:        true,
		DefaultZeroValues:  true,
		MultiDocument:      false,
		DocumentSeparator:  "---",
		CustomResolvers:    nil,
		PostProcessors:     nil,
	}
}

// StrictParserConfig returns a ParserConfig for strict parsing.
//
// Strict configuration enforces all validation rules, rejects unknown fields,
// and provides detailed error messages suitable for production environments.
func StrictParserConfig() *ParserConfig {
	return &ParserConfig{
		StrictMode:         true,
		VerboseErrors:      true,
		IncludeLineInfo:    true,
		ErrorContextLines:  3,
		EnableCaching:      true,
		CacheTTL:           10 * time.Minute,
		MaxCacheSize:       500,
		EnableStreaming:    false,
		StreamBufferSize:   8192,
		MaxFileSize:        50 * 1024 * 1024, // 50MB
		ValidateAfterParse: true,
		ValidatorConfig:    StrictValidatorConfig(),
		ExplicitTypeTags:   false,
		CoerceTypes:        false,
		DefaultZeroValues:  false,
		MultiDocument:      true,
		DocumentSeparator:  "---",
		CustomResolvers:    nil,
		PostProcessors:     nil,
	}
}

// PerformanceParserConfig returns a ParserConfig optimized for performance.
//
// Performance configuration enables caching, streaming for large files, and
// minimizes validation overhead suitable for high-throughput scenarios.
func PerformanceParserConfig() *ParserConfig {
	return &ParserConfig{
		StrictMode:         false,
		VerboseErrors:      false,
		IncludeLineInfo:    false,
		ErrorContextLines:  0,
		EnableCaching:      true,
		CacheTTL:           30 * time.Minute,
		MaxCacheSize:       1000,
		EnableStreaming:    true,
		StreamBufferSize:   16384,
		MaxFileSize:        100 * 1024 * 1024, // 100MB
		ValidateAfterParse: false,
		ExplicitTypeTags:   false,
		CoerceTypes:        true,
		DefaultZeroValues:  true,
		MultiDocument:      true,
		DocumentSeparator:  "---",
		CustomResolvers:    nil,
		PostProcessors:     nil,
	}
}

// ValidatorConfig defines configuration options for YAML validators.
//
// ValidatorConfig provides comprehensive control over validation behavior
// including strict mode, schema validation, and constraint checking.
type ValidatorConfig struct {
	// Strict validation mode
	StrictMode        bool // Enforce strict YAML validation rules
	RequireAllFields  bool // Require all fields in schema to be present
	RejectUnknownKeys bool // Reject keys not defined in schema

	// Error reporting
	VerboseErrors      bool   // Include detailed context in validation errors
	MaxErrors         int    // Maximum number of errors to collect (0 = unlimited)
	StopAtFirstError  bool   // Stop validation at first error
	WarningThreshold  int    // Number of warnings before treating as error (0 = ignore warnings)

	// Schema validation
	EnableSchemaValidation bool         // Enable schema-based validation
	SchemaPaths            []string      // Paths to schema definition files
	SchemaValidationMode   SchemaMode   // How strictly to apply schema rules

	// Constraint validation
	EnableConstraints      bool         // Enable constraint checking (ranges, patterns, etc.)
	ConstraintMode        ConstraintMode // How strictly to enforce constraints

	// Structural validation
	CheckDuplicateKeys    bool // Check for duplicate keys in mappings
	CheckCircularRefs     bool // Check for circular references
	CheckDeprecatedSyntax bool // Warn about deprecated YAML features

	// Type validation
	ValidateTypes         bool // Validate field types against schema
	ValidateRanges        bool // Validate numeric ranges
	ValidatePatterns      bool // Validate string patterns (regex)
	ValidateLengths       bool // Validate string/array lengths

	// Custom validation
	CustomValidators     []FieldValidator // Custom field validation functions
	SchemaValidators     []SchemaValidatorFunc // Custom schema validation functions

	// Performance
	EnableValidationCache bool         // Cache validation results
	CacheInvalidFiles    bool         // Even cache files that fail validation
}

// SchemaMode defines how strictly schema rules are applied.
type SchemaMode int

const (
	// SchemaModeDisabled disables schema validation entirely
	SchemaModeDisabled SchemaMode = iota
	// SchemaModeLenient applies schema rules but allows unknown fields
	SchemaModeLenient
	// SchemaModeStrict applies schema rules and rejects unknown fields
	SchemaModeStrict
	// SchemaModeRequired requires all schema fields to be present
	SchemaModeRequired
)

// ConstraintMode defines how strictly constraints are enforced.
type ConstraintMode int

const (
	// ConstraintModeDisabled disables constraint checking
	ConstraintModeDisabled ConstraintMode = iota
	// ConstraintModeWarn issues warnings for constraint violations
	ConstraintModeWarn
	// ConstraintModeError treats constraint violations as errors
	ConstraintModeError
	// ConstraintModeFatal treats constraint violations as fatal errors
	ConstraintModeFatal
)

// FieldValidator defines a function for custom field validation.
//
// FieldValidators can implement domain-specific validation logic beyond
// what standard schema validation provides.
type FieldValidator func(fieldPath string, value interface{}, data map[string]interface{}) *ConstraintError

// SchemaValidatorFunc defines a function for custom schema-level validation.
//
// SchemaValidatorFuncs can implement cross-field validation, complex constraints,
// or other validation that requires examining the entire document.
type SchemaValidatorFunc func(data map[string]interface{}, schema *Schema) []ValidationError

// DefaultValidatorConfig returns a ValidatorConfig with sensible defaults.
//
// The default configuration provides basic validation with detailed error
// reporting suitable for development environments.
func DefaultValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		StrictMode:             false,
		RequireAllFields:      false,
		RejectUnknownKeys:      false,
		VerboseErrors:          true,
		MaxErrors:             50,
		StopAtFirstError:      false,
		WarningThreshold:      10,
		EnableSchemaValidation: false,
		SchemaPaths:           nil,
		SchemaValidationMode:  SchemaModeLenient,
		EnableConstraints:     true,
		ConstraintMode:        ConstraintModeWarn,
		CheckDuplicateKeys:    true,
		CheckCircularRefs:     false,
		CheckDeprecatedSyntax: false,
		ValidateTypes:         true,
		ValidateRanges:        false,
		ValidatePatterns:      false,
		ValidateLengths:       false,
		CustomValidators:      nil,
		SchemaValidators:      nil,
		EnableValidationCache: false,
		CacheInvalidFiles:     false,
	}
}

// StrictValidatorConfig returns a ValidatorConfig for strict validation.
//
// Strict configuration enforces all validation rules, requires schema
// compliance, and treats all warnings as errors suitable for production.
func StrictValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		StrictMode:             true,
		RequireAllFields:      true,
		RejectUnknownKeys:      true,
		VerboseErrors:          true,
		MaxErrors:             100,
		StopAtFirstError:      false,
		WarningThreshold:      0,
		EnableSchemaValidation: true,
		SchemaPaths:           nil,
		SchemaValidationMode:  SchemaModeStrict,
		EnableConstraints:     true,
		ConstraintMode:        ConstraintModeError,
		CheckDuplicateKeys:    true,
		CheckCircularRefs:     true,
		CheckDeprecatedSyntax: true,
		ValidateTypes:         true,
		ValidateRanges:        true,
		ValidatePatterns:      true,
		ValidateLengths:       true,
		CustomValidators:      nil,
		SchemaValidators:      nil,
		EnableValidationCache: true,
		CacheInvalidFiles:     false,
	}
}

// LenientValidatorConfig returns a ValidatorConfig for lenient validation.
//
// Lenient configuration provides basic validation without strict enforcement,
// suitable for development and testing environments.
func LenientValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		StrictMode:             false,
		RequireAllFields:      false,
		RejectUnknownKeys:      false,
		VerboseErrors:          true,
		MaxErrors:             25,
		StopAtFirstError:      false,
		WarningThreshold:      20,
		EnableSchemaValidation: false,
		SchemaPaths:           nil,
		SchemaValidationMode:  SchemaModeLenient,
		EnableConstraints:     true,
		ConstraintMode:        ConstraintModeWarn,
		CheckDuplicateKeys:    true,
		CheckCircularRefs:     false,
		CheckDeprecatedSyntax: false,
		ValidateTypes:         false,
		ValidateRanges:        false,
		ValidatePatterns:      false,
		ValidateLengths:       false,
		CustomValidators:      nil,
		SchemaValidators:      nil,
		EnableValidationCache: false,
		CacheInvalidFiles:     false,
	}
}

// SchemaConfig defines configuration for schema-based validation.
//
// SchemaConfig provides settings for loading and applying schema definitions
// to validate YAML documents.
type SchemaConfig struct {
	// Schema sources
	SchemaPaths      []string      // Paths to schema definition files
	SchemaURIs       []string      // URIs to remote schema definitions
	SchemaStrings    []string      // Inline schema definitions

	// Schema loading
	EnableReloading   bool         // Reload schemas when files change
	ReloadInterval    time.Duration // How often to check for schema changes
	CacheSchemas      bool         // Cache loaded schemas in memory

	// Schema validation
	ValidationMode    SchemaMode   // How strictly to apply schema rules
	RequireAllFields  bool         // Require all schema fields to be present
	RejectUnknownKeys bool         // Reject keys not defined in schema

	// Type checking
	EnableTypeCheck   bool         // Validate field types against schema
	StrictTypes       bool         // Require exact type matches (no coercion)

	// Custom schema extensions
	CustomTypes       map[string]TypeDefinition // Custom type definitions
	CustomConstraints map[string]ConstraintDefinition // Custom constraint definitions
}

// TypeDefinition defines a custom type for schema validation.
//
// TypeDefinitions allow domain-specific types beyond the standard YAML types.
type TypeDefinition struct {
	Name        string              // Type name
	BaseType    string              // Base type to extend from
	Validator   func(interface{}) bool // Validation function
	Description string              // Human-readable description
}

// ConstraintDefinition defines a custom constraint for schema validation.
//
// ConstraintDefinitions allow domain-specific constraints beyond standard
// range, length, and pattern constraints.
type ConstraintDefinition struct {
	Name        string              // Constraint name
	Validator   func(interface{}) bool // Validation function
	Message     string              // Error message template
	Description string              // Human-readable description
}

// DefaultSchemaConfig returns a SchemaConfig with sensible defaults.
//
// The default configuration provides basic schema validation with
// lenient mode suitable for development.
func DefaultSchemaConfig() *SchemaConfig {
	return &SchemaConfig{
		SchemaPaths:       nil,
		SchemaURIs:        nil,
		SchemaStrings:     nil,
		EnableReloading:   false,
		ReloadInterval:    60 * time.Second,
		CacheSchemas:      true,
		ValidationMode:    SchemaModeLenient,
		RequireAllFields:  false,
		RejectUnknownKeys: false,
		EnableTypeCheck:   true,
		StrictTypes:       false,
		CustomTypes:       nil,
		CustomConstraints: nil,
	}
}

// FieldRequirement defines requirements for a single field.
//
// FieldRequirements specify what constraints and validations apply to
// individual fields in a YAML document.
type FieldRequirement struct {
	Path            string        // Dot-notation path to the field
	Required        bool          // Whether the field must be present
	Type            string        // Expected type (string, int, bool, etc.)
	AllowedValues   []interface{} // List of allowed values (if enumerable)
	MinValue        *float64      // Minimum value for numeric types
	MaxValue        *float64      // Maximum value for numeric types
	MinLength       *int          // Minimum length for strings/arrays
	MaxLength       *int          // Maximum length for strings/arrays
	Pattern         string        // Regex pattern for string validation
	CustomValidator FieldValidator // Custom validation function
	Description     string        // Human-readable description
}

// ValidationOptions defines options for a single validation operation.
//
// ValidationOptions allow runtime control of validation behavior without
// modifying the base ValidatorConfig.
type ValidationOptions struct {
	FilePath         string        // Path to file being validated (for error reporting)
	StopAtFirstError bool          // Override StopAtFirstError from config
	MaxErrors        int           // Override MaxErrors from config
	SkipWarnings     bool          // Skip warning collection
	CustomValidators []FieldValidator // Additional validators for this run
	ValidateOnly     []string      // Only validate these field paths (if specified)
	SkipFields       []string      // Skip validation for these field paths
}
