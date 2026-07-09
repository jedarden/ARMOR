// Package yamlutil provides type definitions and interfaces for YAML parsing utilities.
package yamlutil

// YAMLParser defines the interface for YAML parsing operations.
// Implementations should support both typed and generic parsing.
type YAMLParser interface {
	// ParseFile reads and parses a YAML file into the provided data structure.
	// The data parameter must be a pointer to the target structure.
	ParseFile(filePath string, data interface{}) ParseResult

	// ParseFileToMap reads and parses a YAML file into a generic map structure.
	// Useful when the YAML structure is unknown or dynamic.
	ParseFileToMap(filePath string) ParseResult

	// ParseString parses YAML content from a string into the provided data structure.
	// The data parameter must be a pointer to the target structure.
	ParseString(yamlContent string, data interface{}) error

	// MustParseFile reads and parses a YAML file, panicking on any error.
	// Useful for initialization code where YAML files are critical.
	MustParseFile(filePath string, data interface{})
}

// YAMLValidator defines the interface for YAML validation operations.
// Implementations should provide detailed error reporting and categorization.
type YAMLValidator interface {
	// ValidateFile validates a YAML file at the given path.
	// Returns ValidationResult with detailed error and warning information.
	ValidateFile(filePath string) ValidationResult

	// ValidateString validates YAML content from a string.
	// Returns ValidationResult with detailed error and warning information.
	ValidateString(yamlContent string) ValidationResult

	// ValidateMultipleFiles validates multiple YAML files.
	// Returns a slice of ValidationResults, one per file.
	ValidateMultipleFiles(filePaths []string) []ValidationResult
}

// FieldAccessor defines the interface for type-safe field access operations.
// Implementations should support dot notation and type conversion.
type FieldAccessor interface {
	// GetField retrieves a field value using dot notation.
	// Returns defaultValue if field is missing or nil.
	GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{}

	// GetString retrieves a string field value with automatic type conversion.
	// Returns defaultValue if field is missing, nil, or not convertible.
	GetString(data map[string]interface{}, path string, defaultValue string) string

	// GetInt retrieves an integer field value with automatic type conversion.
	// Returns defaultValue if field is missing, nil, or not convertible.
	GetInt(data map[string]interface{}, path string, defaultValue int) int

	// GetBool retrieves a boolean field value with automatic type conversion.
	// Returns defaultValue if field is missing, nil, or not convertible.
	GetBool(data map[string]interface{}, path string, defaultValue bool) bool

	// HasField checks if a field exists and is not nil.
	// Returns true only if field exists and has a non-nil value.
	HasField(data map[string]interface{}, path string) bool

	// GetRequiredField retrieves a required field value.
	// Returns error if field is missing or nil.
	GetRequiredField(data map[string]interface{}, path string) (interface{}, error)

	// GetRequiredString retrieves a required string field value.
	// Returns error if field is missing, nil, or not a string.
	GetRequiredString(data map[string]interface{}, path string) (string, error)

	// GetRequiredInt retrieves a required integer field value.
	// Returns error if field is missing, nil, or not numeric.
	GetRequiredInt(data map[string]interface{}, path string) (int, error)

	// GetRequiredBool retrieves a required boolean field value.
	// Returns error if field is missing, nil, or not boolean.
	GetRequiredBool(data map[string]interface{}, path string) (bool, error)
}

// FileOperations defines the interface for file I/O operations with contextual error handling.
type FileOperations interface {
	// ReadFile reads the entire contents of a file.
	// Returns wrapped errors with operation context and file path.
	ReadFile(filePath string) ([]byte, error)

	// FileExists checks if a file exists at the given path.
	// Returns false for missing files, permission errors, or directories.
	FileExists(filePath string) bool
}

// FieldValidator defines the interface for field validation operations.
type FieldValidator interface {
	// ValidateRequiredFields checks that all required field paths exist.
	// Returns a list of missing field paths.
	ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string

	// ValidateFieldRequirements validates field requirements with type checking.
	// Returns a list of validation errors.
	ValidateFieldRequirements(data map[string]interface{}, requirements []FieldRequirement) []error
}

// YAMLFileFinder defines the interface for YAML file discovery operations.
type YAMLFileFinder interface {
	// FindYAMLFiles finds all YAML files in a directory (non-recursive).
	FindYAMLFiles(dirPath string) ([]string, error)

	// FindYAMLFilesRecursive finds all YAML files in a directory recursively.
	FindYAMLFilesRecursive(dirPath string) ([]string, error)

	// IsYAMLFile checks if a file has a YAML extension.
	IsYAMLFile(filePath string) bool
}

// ParseErrorDetail provides detailed information about parsing errors.
type ParseErrorDetail struct {
	Line      int    // Line number where error occurred (1-indexed)
	Column    int    // Column number where error occurred (1-indexed)
	Context   string // Surrounding line content for context
	Message   string // Human-readable error message
	FilePath  string // Path to the file with the error
	ErrorType ErrorType // Category of error
}

// SchemaValidator defines the interface for schema-based YAML validation (future enhancement).
type SchemaValidator interface {
	// ValidateAgainstSchema validates YAML data against a schema definition.
	ValidateAgainstSchema(data map[string]interface{}, schema interface{}) []ValidationError

	// LoadSchema loads a schema definition from a file.
	LoadSchema(schemaPath string) (interface{}, error)
}

// YAMLProcessor provides advanced YAML processing capabilities (future enhancement).
type YAMLProcessor interface {
	// Merge merges multiple YAML documents with conflict resolution.
	Merge(documents []map[string]interface{}) (map[string]interface{}, error)

	// Diff compares two YAML documents and returns differences.
	Diff(original, modified map[string]interface{}) []YAMLDiff

	// Transform applies a transformation function to YAML data.
	Transform(data map[string]interface{}, transformer func(interface{}) interface{}) (map[string]interface{}, error)
}

// YAMLDiff represents a difference between two YAML documents.
type YAMLDiff struct {
	Path     string      // Dot-separated path to the difference
	Original interface{} // Original value
	Modified interface{} // Modified value
	Type     DiffType    // Type of difference (added, removed, changed)
}

// DiffType represents the type of difference between YAML documents.
type DiffType string

const (
	DiffTypeAdded   DiffType = "added"   // Field was added
	DiffTypeRemoved DiffType = "removed" // Field was removed
	DiffTypeChanged DiffType = "changed" // Field value was changed
)

// TemplateProcessor defines the interface for YAML template processing (future enhancement).
type TemplateProcessor interface {
	// ProcessTemplate expands variables in a YAML template.
	ProcessTemplate(template string, variables map[string]string) (string, error)

	// ProcessTemplateFile expands variables in a YAML template file.
	ProcessTemplateFile(templatePath string, variables map[string]string) (string, error)
}

// StreamProcessor defines the interface for streaming YAML processing (future enhancement).
type StreamProcessor interface {
	// ProcessStream processes YAML content in a streaming fashion for memory efficiency.
	ProcessStream(filePath string, handler func(map[string]interface{}) error) error

	// ProcessStreamWithProgress processes YAML with progress reporting.
	ProcessStreamWithProgress(filePath string, handler func(map[string]interface{}) error, progress func(int, int)) error
}

// Ensure that concrete types implement the required interfaces.
// These checks happen at compile time.

var (
	// Parser implements YAMLParser
	_ YAMLParser = (*Parser)(nil)

	// Validator implements YAMLValidator
	_ YAMLValidator = (*Validator)(nil)

	// Package-level functions implement FieldAccessor
	// (verified through function signatures matching interface)
)