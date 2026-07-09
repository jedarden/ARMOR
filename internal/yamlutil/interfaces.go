// Package yamlutil provides interface definitions for YAML processing components.
//
// These interfaces define the core abstractions used throughout the YAML module,
// enabling testing, dependency injection, and extensibility.
package yamlutil

import (
	"io"
)

// FileReader defines the interface for reading file contents.
//
// This abstraction allows for different file reading strategies,
// including direct file system access, memory buffers, or remote sources.
type FileReader interface {
	// Read reads the entire contents of a file and returns the bytes.
	// Returns an error if the file cannot be read.
	Read(path string) ([]byte, error)

	// Exists checks if a file exists at the given path.
	Exists(path string) bool
}

// YAMLParser defines the interface for parsing YAML content.
//
// Implementations can provide different parsing strategies,
// error handling approaches, or performance characteristics.
type YAMLParser interface {
	// ParseFile reads and parses a YAML file into the provided data structure.
	// The data parameter must be a pointer to the target structure.
	// Returns a ParseResult with success status and any errors encountered.
	ParseFile(filePath string, data interface{}) ParseResult

	// ParseFileToMap reads and parses a YAML file into a generic map structure.
	// Useful when the YAML structure is unknown or dynamic.
	// Returns a ParseResult with the parsed map[string]interface{} data.
	ParseFileToMap(filePath string) ParseResult

	// ParseString parses YAML content from a string into the provided data structure.
	// The data parameter must be a pointer to the target structure.
	ParseString(yamlContent string, data interface{}) error

	// MustParseFile reads and parses a YAML file, panicking on any error.
	// Useful for initialization code where YAML files are critical.
	MustParseFile(filePath string, data interface{})
}

// YAMLValidator defines the interface for YAML validation operations.
//
// Validators check YAML syntax and structure before parsing,
// providing detailed error reporting with line and column information.
type YAMLValidator interface {
	// ValidateFile validates a YAML file at the given path.
	// Returns a ValidationResult with detailed error and warning information.
	ValidateFile(filePath string) ValidationResult

	// ValidateString validates YAML content from a string.
	// Returns a ValidationResult with detailed error and warning information.
	ValidateString(yamlContent string) ValidationResult

	// ValidateStringWithPath validates YAML content from a string with a file path for error reporting.
	// Returns a ValidationResult with detailed error and warning information.
	ValidateStringWithPath(yamlContent, filePath string) ValidationResult

	// ValidateMultipleFiles validates multiple YAML files.
	// Returns a slice of ValidationResult, one per file.
	ValidateMultipleFiles(filePaths []string) []ValidationResult
}

// FieldAccessor defines the interface for accessing fields in YAML data structures.
//
// FieldAccessors provide type-safe access to nested fields using dot notation,
// with support for default values and required field validation.
type FieldAccessor interface {
	// GetField retrieves a field value from nested YAML data using a dot-separated path.
	// Returns the value if found, or the provided defaultValue if the field is missing.
	GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{}

	// GetString retrieves a string field value from nested YAML data.
	// Returns the string value if found and is a string, or defaultValue otherwise.
	GetString(data map[string]interface{}, path string, defaultValue string) string

	// GetInt retrieves an integer field value from nested YAML data.
	// Returns the integer value if found and is numeric, or defaultValue otherwise.
	GetInt(data map[string]interface{}, path string, defaultValue int) int

	// GetBool retrieves a boolean field value from nested YAML data.
	// Returns the boolean value if found and is boolean, or defaultValue otherwise.
	GetBool(data map[string]interface{}, path string, defaultValue bool) bool

	// HasField checks if a field exists at the given path in the YAML data.
	// Returns true if the field exists and is not nil, false otherwise.
	HasField(data map[string]interface{}, path string) bool

	// GetRequiredField retrieves a field value, returning an error if the field is missing.
	// Returns the field value if found, or FieldNotFoundError if missing.
	GetRequiredField(data map[string]interface{}, path string) (interface{}, error)

	// GetRequiredString retrieves a string field value, returning an error if missing or not a string.
	// Returns the string value or FieldNotFoundError if missing, TypeMismatchError if not a string.
	GetRequiredString(data map[string]interface{}, path string) (string, error)

	// GetRequiredInt retrieves an integer field value, returning an error if missing or not numeric.
	// Returns the integer value or FieldNotFoundError if missing, TypeMismatchError if not numeric.
	GetRequiredInt(data map[string]interface{}, path string) (int, error)

	// GetRequiredBool retrieves a boolean field value, returning an error if missing or not boolean.
	// Returns the boolean value or FieldNotFoundError if missing, TypeMismatchError if not boolean.
	GetRequiredBool(data map[string]interface{}, path string) (bool, error)

	// ValidateRequiredFields checks that all required field paths exist in the YAML data.
	// Returns a list of missing field paths. Empty list means all required fields are present.
	ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string

	// ValidateFieldRequirements validates field requirements with type checking.
	// Returns a list of validation errors. Empty list means all requirements passed.
	ValidateFieldRequirements(data map[string]interface{}, requirements []FieldRequirement) []error
}

// YAMLErrorHandler defines the interface for handling YAML processing errors.
//
// ErrorHandlers can implement different error handling strategies,
// such as logging, recovery, or custom error formatting.
type YAMLErrorHandler interface {
	// HandleFileError handles file I/O errors during YAML processing.
	HandleFileError(err error, filePath string)

	// HandleParseError handles YAML parsing errors.
	HandleParseError(err error, filePath string)

	// HandleValidationError handles YAML validation errors.
	HandleValidationError(errs []ValidationError, filePath string)
}

// YAMLReader defines the interface for reading YAML from various sources.
//
// This interface abstracts the source of YAML content, enabling
// reading from files, streams, memory, or network sources.
type YAMLReader interface {
	// ReadYAML reads YAML content from the source.
	// Returns the content as a byte slice and any error encountered.
	ReadYAML() ([]byte, error)
}

// FileDiscovery defines the interface for discovering YAML files in filesystems.
//
// FileDiscovery implementations can find YAML files using different
// strategies and filtering criteria.
type FileDiscovery interface {
	// FindYAMLFiles finds all YAML files in a directory (non-recursive).
	FindYAMLFiles(dirPath string) ([]string, error)

	// FindYAMLFilesRecursive finds all YAML files in a directory recursively.
	FindYAMLFilesRecursive(dirPath string) ([]string, error)

	// IsYAMLFile checks if a file has a YAML extension (.yaml or .yml).
	IsYAMLFile(filePath string) bool
}

// StreamYAMLParser defines the interface for streaming YAML parsing.
//
// This interface is reserved for future implementation of streaming
// support for large YAML files.
type StreamYAMLParser interface {
	// ParseStream parses YAML content from a reader stream.
	ParseStream(reader io.Reader, data interface{}) error

	// ParseStreamToMap parses YAML content from a reader into a generic map.
	ParseStreamToMap(reader io.Reader) (map[string]interface{}, error)
}

// YAMLConverter defines the interface for converting YAML data structures.
//
// Converters can transform YAML data between different formats
// or apply transformations to the data structure.
type YAMLConverter interface {
	// ToJSON converts YAML data to JSON format.
	ToJSON(yamlData map[string]interface{}) ([]byte, error)

	// ToXML converts YAML data to XML format.
	ToXML(yamlData map[string]interface{}) ([]byte, error)

	// ToEnv converts YAML data to environment variable format.
	ToEnv(yamlData map[string]interface{}, prefix string) ([]string, error)

	// Merge merges multiple YAML data structures into one.
	Merge(datas ...map[string]interface{}) (map[string]interface{}, error)
}

// YAMLPathNavigator defines the interface for advanced path navigation.
//
// PathNavigators can support more complex path expressions beyond
// simple dot notation, including array indexing and wildcards.
type YAMLPathNavigator interface {
	// GetPath retrieves a value using a complex path expression.
	// Supports array indexing, wildcards, and other advanced patterns.
	GetPath(data map[string]interface{}, pathExpr string) (interface{}, error)

	// SetPath sets a value at a complex path expression.
	SetPath(data map[string]interface{}, pathExpr string, value interface{}) error

	// DeletePath removes a value at a complex path expression.
	DeletePath(data map[string]interface{}, pathExpr string) error
}

// YAMLCache defines the interface for caching parsed YAML content.
//
// Caches can improve performance by avoiding repeated parsing
// of frequently accessed configuration files.
type YAMLCache interface {
	// Get retrieves cached parsed YAML data if available.
	Get(filePath string) (map[string]interface{}, bool)

	// Set stores parsed YAML data in the cache.
	Set(filePath string, data map[string]interface{})

	// Invalidate removes a file from the cache.
	Invalidate(filePath string)

	// Clear removes all entries from the cache.
	Clear()

	// Size returns the number of entries in the cache.
	Size() int
}

// YAMLWatcher defines the interface for watching YAML files for changes.
//
// Watchers can monitor files for modifications and trigger callbacks,
// enabling hot-reload configuration capabilities.
type YAMLWatcher interface {
	// Watch starts monitoring a file for changes.
	// Returns a channel that receives events when the file changes.
	Watch(filePath string) (<-chan FileChangeEvent, error)

	// Unwatch stops monitoring a file.
	Unwatch(filePath string) error

	// Close stops all watching and releases resources.
	Close() error
}

// FileChangeEvent represents a change to a monitored file.
type FileChangeEvent struct {
	FilePath string
	EventType FileEventType
}

// FileEventType represents the type of file change event.
type FileEventType int

const (
	FileCreated FileEventType = iota
	FileModified
	FileDeleted
)

// YAMLProcessor defines a comprehensive interface combining multiple YAML operations.
//
// This interface provides a complete YAML processing facility that can parse,
// validate, access fields, and handle errors in a unified way.
type YAMLProcessor interface {
	FileReader
	YAMLParser
	YAMLValidator
	FieldAccessor
	FileDiscovery
}

// DefaultProcessor provides a default implementation of YAMLProcessor.
//
// This struct combines all the standard YAML processing components
// into a single, comprehensive processor.
type DefaultProcessor struct {
	parser    YAMLParser
	validator YAMLValidator
	accessor  FieldAccessor
	reader    FileReader
	discovery FileDiscovery
}

// NewDefaultProcessor creates a new default YAML processor with standard components.
func NewDefaultProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		parser:    NewParser(),
		validator: NewValidator(),
		accessor:  &defaultFieldAccessor{},
		reader:    &defaultFileReader{},
		discovery: &defaultFileDiscovery{},
	}
}

// NewStrictProcessor creates a new YAML processor with strict validation enabled.
func NewStrictProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		parser:    NewStrictParser(),
		validator: NewStrictValidator(),
		accessor:  &defaultFieldAccessor{},
		reader:    &defaultFileReader{},
		discovery: &defaultFileDiscovery{},
	}
}

// Default implementations of interfaces

type defaultFileReader struct{}
type defaultFieldAccessor struct{}
type defaultFileDiscovery struct{}

func (d *defaultFileReader) Read(path string) ([]byte, error) {
	return ReadFile(path)
}

func (d *defaultFileReader) Exists(path string) bool {
	return FileExists(path)
}

func (d *defaultFieldAccessor) GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{} {
	return GetField(data, path, defaultValue)
}

func (d *defaultFieldAccessor) GetString(data map[string]interface{}, path string, defaultValue string) string {
	return GetString(data, path, defaultValue)
}

func (d *defaultFieldAccessor) GetInt(data map[string]interface{}, path string, defaultValue int) int {
	return GetInt(data, path, defaultValue)
}

func (d *defaultFieldAccessor) GetBool(data map[string]interface{}, path string, defaultValue bool) bool {
	return GetBool(data, path, defaultValue)
}

func (d *defaultFieldAccessor) HasField(data map[string]interface{}, path string) bool {
	return HasField(data, path)
}

func (d *defaultFieldAccessor) GetRequiredField(data map[string]interface{}, path string) (interface{}, error) {
	return GetRequiredField(data, path)
}

func (d *defaultFieldAccessor) GetRequiredString(data map[string]interface{}, path string) (string, error) {
	return GetRequiredString(data, path)
}

func (d *defaultFieldAccessor) GetRequiredInt(data map[string]interface{}, path string) (int, error) {
	return GetRequiredInt(data, path)
}

func (d *defaultFieldAccessor) GetRequiredBool(data map[string]interface{}, path string) (bool, error) {
	return GetRequiredBool(data, path)
}

func (d *defaultFieldAccessor) ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string {
	return ValidateRequiredFields(data, requiredFields)
}

func (d *defaultFieldAccessor) ValidateFieldRequirements(data map[string]interface{}, requirements []FieldRequirement) []error {
	return ValidateFieldRequirements(data, requirements)
}

func (d *defaultFileDiscovery) FindYAMLFiles(dirPath string) ([]string, error) {
	return FindYAMLFiles(dirPath)
}

func (d *defaultFileDiscovery) FindYAMLFilesRecursive(dirPath string) ([]string, error) {
	return FindYAMLFilesRecursive(dirPath)
}

func (d *defaultFileDiscovery) IsYAMLFile(filePath string) bool {
	return IsYAMLFile(filePath)
}

// Interface implementations for existing types

func (p *Parser) ParseFile(filePath string, data interface{}) ParseResult {
	return p.ParseFile(filePath, data)
}

func (p *Parser) ParseFileToMap(filePath string) ParseResult {
	return p.ParseFileToMap(filePath)
}

func (p *Parser) ParseString(yamlContent string, data interface{}) error {
	return p.ParseString(yamlContent, data)
}

func (p *Parser) MustParseFile(filePath string, data interface{}) {
	p.MustParseFile(filePath, data)
}

func (v *Validator) ValidateFile(filePath string) ValidationResult {
	return v.ValidateFile(filePath)
}

func (v *Validator) ValidateString(yamlContent string) ValidationResult {
	return v.ValidateString(yamlContent)
}

func (v *Validator) ValidateStringWithPath(yamlContent, filePath string) ValidationResult {
	return v.ValidateStringWithPath(yamlContent, filePath)
}

func (v *Validator) ValidateMultipleFiles(filePaths []string) []ValidationResult {
	return v.ValidateMultipleFiles(filePaths)
}