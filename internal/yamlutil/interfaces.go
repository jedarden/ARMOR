// Package yamlutil provides interface definitions for YAML processing components.
//
// These interfaces define the core abstractions used throughout the YAML module,
// enabling testing, dependency injection, and extensibility.
package yamlutil

import (
	"fmt"
	"io"
	"os"
	"time"
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
//
// The YAMLParser interface enables the Strategy pattern, allowing different
// parsing implementations to be used interchangeably. Common strategies include:
//   - StandardParser: Basic parsing with full file loading
//   - CachedParser: Caching results for repeated access
//   - StreamingParser: Memory-efficient parsing for large files
//   - LazyParser: Deferred parsing until data access
//
// Implementations must handle all error cases and return detailed ParseResult
// structures that include success status, parsed data, and comprehensive error information.
type YAMLParser interface {
	// ParseFile reads and parses a YAML file into the provided data structure.
	// The data parameter must be a pointer to the target structure.
	// Returns a ParseResult with success status and any errors encountered.
	//
	// Example:
	//   var data map[string]interface{}
	//   result := parser.ParseFile("config.yaml", &data)
	//   if !result.Success {
	//       log.Fatal(result.Error)
	//   }
	ParseFile(filePath string, data interface{}) ParseResult

	// ParseFileToMap reads and parses a YAML file into a generic map structure.
	// Useful when the YAML structure is unknown or dynamic.
	// Returns a ParseResult with the parsed map[string]interface{} data.
	//
	// Example:
	//   result := parser.ParseFileToMap("config.yaml")
	//   if result.Success {
	//       data := result.Data.(map[string]interface{})
	//       // Use the parsed data
	//   }
	ParseFileToMap(filePath string) ParseResult

	// ParseString parses YAML content from a string into the provided data structure.
	// The data parameter must be a pointer to the target structure.
	// Returns an error if parsing fails, or nil on success.
	//
	// Example:
	//   yamlContent := "name: test\nport: 8080"
	//   var data map[string]interface{}
	//   if err := parser.ParseString(yamlContent, &data); err != nil {
	//       log.Fatal(err)
	//   }
	ParseString(yamlContent string, data interface{}) error

	// MustParseFile reads and parses a YAML file, panicking on any error.
	// Useful for initialization code where YAML files are critical.
	// Panics with a descriptive message if the file cannot be parsed.
	//
	// Example:
	//   var data Config
	//   parser.MustParseFile("critical-config.yaml", &data)
	//   // If we get here, parsing succeeded
	MustParseFile(filePath string, data interface{})

	// Config returns the configuration for this parser.
	//
	// This allows inspection of parser settings and runtime behavior.
	// Returns a pointer to the parser's configuration.
	//
	// Example:
	//   config := parser.Config()
	//   fmt.Printf("Strict mode: %v\n", config.StrictMode)
	Config() *ParserConfig
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
	FilePath  string
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
//
// Returns a YAMLProcessor that combines standard parsing, validation, field access,
// file reading, and file discovery capabilities with default (non-strict) settings.
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
//
// Returns a YAMLProcessor that combines standard parsing, validation, field access,
// file reading, and file discovery capabilities with strict mode enabled for
// production environments where validation should reject unknown fields and enforce constraints.
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

// defaultFileReader implements FileReader using package-level functions.
type defaultFileReader struct{}

// defaultFieldAccessor implements FieldAccessor using package-level functions.
type defaultFieldAccessor struct{}

// defaultFileDiscovery implements FileDiscovery using package-level functions.
type defaultFileDiscovery struct{}

// Read implements FileReader.Read by delegating to the ReadFile function.
func (d *defaultFileReader) Read(path string) ([]byte, error) {
	return ReadFile(path)
}

// Exists implements FileReader.Exists by delegating to the FileExists function.
func (d *defaultFileReader) Exists(path string) bool {
	return FileExists(path)
}

// GetField implements FieldAccessor.GetField by delegating to the GetField function.
func (d *defaultFieldAccessor) GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{} {
	return GetField(data, path, defaultValue)
}

// GetString implements FieldAccessor.GetString by delegating to the GetString function.
func (d *defaultFieldAccessor) GetString(data map[string]interface{}, path string, defaultValue string) string {
	return GetString(data, path, defaultValue)
}

// GetInt implements FieldAccessor.GetInt by delegating to the GetInt function.
func (d *defaultFieldAccessor) GetInt(data map[string]interface{}, path string, defaultValue int) int {
	return GetInt(data, path, defaultValue)
}

// GetBool implements FieldAccessor.GetBool by delegating to the GetBool function.
func (d *defaultFieldAccessor) GetBool(data map[string]interface{}, path string, defaultValue bool) bool {
	return GetBool(data, path, defaultValue)
}

// HasField implements FieldAccessor.HasField by delegating to the HasField function.
func (d *defaultFieldAccessor) HasField(data map[string]interface{}, path string) bool {
	return HasField(data, path)
}

// GetRequiredField implements FieldAccessor.GetRequiredField by delegating to the GetRequiredField function.
func (d *defaultFieldAccessor) GetRequiredField(data map[string]interface{}, path string) (interface{}, error) {
	return GetRequiredField(data, path)
}

// GetRequiredString implements FieldAccessor.GetRequiredString by delegating to the GetRequiredString function.
func (d *defaultFieldAccessor) GetRequiredString(data map[string]interface{}, path string) (string, error) {
	return GetRequiredString(data, path)
}

// GetRequiredInt implements FieldAccessor.GetRequiredInt by delegating to the GetRequiredInt function.
func (d *defaultFieldAccessor) GetRequiredInt(data map[string]interface{}, path string) (int, error) {
	return GetRequiredInt(data, path)
}

// GetRequiredBool implements FieldAccessor.GetRequiredBool by delegating to the GetRequiredBool function.
func (d *defaultFieldAccessor) GetRequiredBool(data map[string]interface{}, path string) (bool, error) {
	return GetRequiredBool(data, path)
}

// ValidateRequiredFields implements FieldAccessor.ValidateRequiredFields by delegating to the ValidateRequiredFields function.
func (d *defaultFieldAccessor) ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string {
	return ValidateRequiredFields(data, requiredFields)
}

// ValidateFieldRequirements implements FieldAccessor.ValidateFieldRequirements by delegating to the ValidateFieldRequirements function.
func (d *defaultFieldAccessor) ValidateFieldRequirements(data map[string]interface{}, requirements []FieldRequirement) []error {
	return ValidateFieldRequirements(data, requirements)
}

// FindYAMLFiles implements FileDiscovery.FindYAMLFiles by delegating to the FindYAMLFiles function.
func (d *defaultFileDiscovery) FindYAMLFiles(dirPath string) ([]string, error) {
	return FindYAMLFiles(dirPath)
}

// FindYAMLFilesRecursive implements FileDiscovery.FindYAMLFilesRecursive by delegating to the FindYAMLFilesRecursive function.
func (d *defaultFileDiscovery) FindYAMLFilesRecursive(dirPath string) ([]string, error) {
	return FindYAMLFilesRecursive(dirPath)
}

// IsYAMLFile implements FileDiscovery.IsYAMLFile by delegating to the IsYAMLFile function.
func (d *defaultFileDiscovery) IsYAMLFile(filePath string) bool {
	return IsYAMLFile(filePath)
}

// ============================================================================
// Parser Strategy Implementations
// ============================================================================

// StandardParser implements YAMLParser using standard parsing with full file loading.
//
// StandardParser is the default parsing strategy that loads entire files into memory
// before parsing. This is suitable for most configuration files and provides good
// performance for small to medium-sized YAML files.
type StandardParser struct {
	parser *Parser
	config *ParserConfig
}

// NewStandardParser creates a new StandardParser with default configuration.
//
// Returns a StandardParser ready to parse YAML files with sensible defaults.
func NewStandardParser() *StandardParser {
	return &StandardParser{
		parser: NewParser(),
		config: DefaultParserConfig(),
	}
}

// NewStandardParserWithConfig creates a new StandardParser with custom configuration.
//
// Allows fine-grained control over parser behavior including strict mode,
// error handling, caching, and performance tuning.
func NewStandardParserWithConfig(config *ParserConfig) *StandardParser {
	return &StandardParser{
		parser: NewParser(),
		config: config,
	}
}

// ParseFile implements YAMLParser.ParseFile by delegating to the underlying Parser.
func (sp *StandardParser) ParseFile(filePath string, data interface{}) ParseResult {
	return sp.parser.ParseFile(filePath, data)
}

// ParseFileToMap implements YAMLParser.ParseFileToMap by delegating to the underlying Parser.
func (sp *StandardParser) ParseFileToMap(filePath string) ParseResult {
	return sp.parser.ParseFileToMap(filePath)
}

// ParseString implements YAMLParser.ParseString by delegating to the underlying Parser.
func (sp *StandardParser) ParseString(yamlContent string, data interface{}) error {
	return sp.parser.ParseString(yamlContent, data)
}

// MustParseFile implements YAMLParser.MustParseFile by delegating to the underlying Parser.
func (sp *StandardParser) MustParseFile(filePath string, data interface{}) {
	sp.parser.MustParseFile(filePath, data)
}

// Config implements YAMLParser.Config by returning the parser configuration.
func (sp *StandardParser) Config() *ParserConfig {
	return sp.config
}

// CachedParser implements YAMLParser with caching support for repeated access.
//
// CachedParser wraps another parser and caches parsed results to avoid re-parsing
// the same file multiple times. This is useful for frequently accessed configuration
// files or when parsing is expensive.
//
// The cache respects the configuration's CacheTTL and MaxCacheSize settings.
// Entries are evicted based on LRU policy when the cache is full.
type CachedParser struct {
	parser     YAMLParser
	cache      map[string]*cachedEntry
	config     *ParserConfig
	cacheStats CacheStats
}

// CacheStats contains statistics about parser cache usage.
//
// CacheStats provides metrics for cache performance monitoring and tuning.
type CacheStats struct {
	Hits      int // Number of cache hits
	Misses    int // Number of cache misses
	Size      int // Current number of cached items
	MaxSize   int // Maximum cache size
	Evictions int // Number of cache evictions
}

type cachedEntry struct {
	data      interface{}
	result    ParseResult
	timestamp int64 // Unix nanoseconds
	accesses  int
}

// NewCachedParser creates a new CachedParser wrapping the given parser.
//
// The returned parser will cache results from the wrapped parser according
// to the default caching configuration.
func NewCachedParser(parser YAMLParser) *CachedParser {
	return &CachedParser{
		parser: parser,
		cache:  make(map[string]*cachedEntry),
		config: DefaultParserConfig(),
	}
}

// NewCachedParserWithConfig creates a new CachedParser with custom configuration.
//
// Allows control over cache behavior including TTL, maximum size, and
// whether to cache invalid files.
func NewCachedParserWithConfig(parser YAMLParser, config *ParserConfig) *CachedParser {
	return &CachedParser{
		parser: parser,
		cache:  make(map[string]*cachedEntry),
		config: config,
	}
}

// ParseFile implements YAMLParser.ParseFile with caching support.
func (cp *CachedParser) ParseFile(filePath string, data interface{}) ParseResult {
	// Check cache if enabled
	if cp.config.EnableCaching {
		if entry, found := cp.getFromCache(filePath); found {
			cp.cacheStats.Hits++
			entry.accesses++
			// Copy cached data to the provided parameter
			if cachedData, ok := entry.data.(map[string]interface{}); ok {
				if targetMap, ok := data.(*map[string]interface{}); ok {
					*targetMap = cachedData
				}
			}
			return entry.result
		}
		cp.cacheStats.Misses++
	}

	// Parse using the wrapped parser
	result := cp.parser.ParseFile(filePath, data)

	// Cache the result if enabled and parsing succeeded
	if cp.config.EnableCaching && result.Success {
		cp.addToCache(filePath, data, result)
	}

	return result
}

// ParseFileToMap implements YAMLParser.ParseFileToMap with caching support.
func (cp *CachedParser) ParseFileToMap(filePath string) ParseResult {
	// Check cache if enabled
	if cp.config.EnableCaching {
		if entry, found := cp.getFromCache(filePath); found {
			cp.cacheStats.Hits++
			entry.accesses++
			return entry.result
		}
		cp.cacheStats.Misses++
	}

	// Parse using the wrapped parser
	result := cp.parser.ParseFileToMap(filePath)

	// Cache the result if enabled and parsing succeeded
	if cp.config.EnableCaching && result.Success {
		cp.addToCache(filePath, result.Data, result)
	}

	return result
}

// ParseString implements YAMLParser.ParseString by delegating to the wrapped parser.
//
// Note: ParseString results are not cached as they are typically one-off operations.
func (cp *CachedParser) ParseString(yamlContent string, data interface{}) error {
	return cp.parser.ParseString(yamlContent, data)
}

// MustParseFile implements YAMLParser.MustParseFile by delegating to the wrapped parser.
func (cp *CachedParser) MustParseFile(filePath string, data interface{}) {
	result := cp.ParseFile(filePath, data)
	if !result.Success {
		panic(fmt.Sprintf("failed to parse YAML file %s: %v", filePath, result.Error))
	}
}

// Config implements YAMLParser.Config by returning the parser configuration.
func (cp *CachedParser) Config() *ParserConfig {
	return cp.config
}

// ClearCache clears all cached entries.
func (cp *CachedParser) ClearCache() {
	cp.cache = make(map[string]*cachedEntry)
}

// CacheSize returns the current number of cached entries.
func (cp *CachedParser) CacheSize() int {
	return len(cp.cache)
}

// CacheStats returns statistics about cache usage.
func (cp *CachedParser) CacheStats() CacheStats {
	cp.cacheStats.Size = len(cp.cache)
	return cp.cacheStats
}

// getFromCache retrieves an entry from the cache if it exists and hasn't expired.
func (cp *CachedParser) getFromCache(filePath string) (*cachedEntry, bool) {
	entry, found := cp.cache[filePath]
	if !found {
		return nil, false
	}

	// Check TTL
	if cp.config.CacheTTL > 0 {
		age := time.Now().UnixNano() - entry.timestamp
		if age > int64(cp.config.CacheTTL) {
			delete(cp.cache, filePath)
			return nil, false
		}
	}

	return entry, true
}

// addToCache adds a result to the cache, evicting entries if necessary.
func (cp *CachedParser) addToCache(filePath string, data interface{}, result ParseResult) {
	// Check if we need to evict entries
	if cp.config.MaxCacheSize > 0 && len(cp.cache) >= cp.config.MaxCacheSize {
		cp.evictLRU()
	}

	cp.cache[filePath] = &cachedEntry{
		data:      data,
		result:    result,
		timestamp: time.Now().UnixNano(),
		accesses:  1,
	}
}

// evictLRU evicts the least recently used cache entry.
func (cp *CachedParser) evictLRU() {
	var lruPath string
	var lruAccesses int = int(^uint(0) >> 1) // Max int

	for path, entry := range cp.cache {
		if entry.accesses < lruAccesses {
			lruAccesses = entry.accesses
			lruPath = path
		}
	}

	if lruPath != "" {
		delete(cp.cache, lruPath)
		cp.cacheStats.Evictions++
	}
}

// StreamingParser implements YAMLParser with streaming support for large files.
//
// StreamingParser reads and processes YAML files incrementally rather than
// loading the entire file into memory. This is useful for very large YAML
// files where memory usage is a concern.
//
// Note: This is a placeholder for future implementation. Currently delegates
// to standard parsing.
type StreamingParser struct {
	config        *ParserConfig
	bufferSize    int
	maxFileSize   int64
}

// NewStreamingParser creates a new StreamingParser with default settings.
//
// Returns a StreamingParser ready to parse large YAML files with default
// buffer size and file size limits.
func NewStreamingParser() *StreamingParser {
	config := PerformanceParserConfig()
	config.EnableStreaming = true
	return &StreamingParser{
		config:     config,
		bufferSize: config.StreamBufferSize,
		maxFileSize: config.MaxFileSize,
	}
}

// NewStreamingParserWithConfig creates a new StreamingParser with custom configuration.
//
// Allows control over buffer size, maximum file size, and other streaming parameters.
func NewStreamingParserWithConfig(config *ParserConfig) *StreamingParser {
	return &StreamingParser{
		config:     config,
		bufferSize: config.StreamBufferSize,
		maxFileSize: config.MaxFileSize,
	}
}

// ParseFile implements YAMLParser.ParseFile with streaming support.
//
// Currently delegates to standard parsing. Future implementation will use
// incremental parsing for memory efficiency.
func (sp *StreamingParser) ParseFile(filePath string, data interface{}) ParseResult {
	// Check file size if max size is set
	if sp.maxFileSize > 0 {
		if info, err := os.Stat(filePath); err == nil {
			if info.Size() > sp.maxFileSize {
				return ParseResult{
					FilePath: filePath,
					Success: false,
					Error: fmt.Errorf("file size %d exceeds maximum %d",
						info.Size(), sp.maxFileSize),
				}
			}
		}
	}

	// Use standard parser for now
	parser := NewParser()
	return parser.ParseFile(filePath, data)
}

// ParseFileToMap implements YAMLParser.ParseFileToMap with streaming support.
func (sp *StreamingParser) ParseFileToMap(filePath string) ParseResult {
	parser := NewParser()
	return parser.ParseFileToMap(filePath)
}

// ParseString implements YAMLParser.ParseString by delegating to standard parser.
func (sp *StreamingParser) ParseString(yamlContent string, data interface{}) error {
	parser := NewParser()
	return parser.ParseString(yamlContent, data)
}

// MustParseFile implements YAMLParser.MustParseFile by delegating to standard parser.
func (sp *StreamingParser) MustParseFile(filePath string, data interface{}) {
	parser := NewParser()
	parser.MustParseFile(filePath, data)
}

// Config implements YAMLParser.Config by returning the parser configuration.
func (sp *StreamingParser) Config() *ParserConfig {
	return sp.config
}

// SetBufferSize sets the buffer size for streaming operations.
func (sp *StreamingParser) SetBufferSize(size int) {
	sp.bufferSize = size
	if sp.config != nil {
		sp.config.StreamBufferSize = size
	}
}

// ============================================================================
// Parser Factory
// ============================================================================

// DefaultParserFactory implements ParserFactory for creating standard parsers.
//
// DefaultParserFactory provides a simple implementation of ParserFactory
// that creates parser instances based on configuration parameters.
type DefaultParserFactory struct{}

// NewParserFactory creates a new DefaultParserFactory.
func NewParserFactory() *DefaultParserFactory {
	return &DefaultParserFactory{}
}

// CreateParser implements ParserFactory.CreateParser.
//
// Selects the appropriate parser implementation based on configuration:
//   - If EnableCaching is true, returns a CachedParser
//   - If EnableStreaming is true, returns a StreamingParser
//   - Otherwise, returns a StandardParser
func (pf *DefaultParserFactory) CreateParser(config *ParserConfig) YAMLParser {
	if config.EnableCaching {
		baseParser := NewStandardParserWithConfig(config)
		return NewCachedParserWithConfig(baseParser, config)
	}
	if config.EnableStreaming {
		return NewStreamingParserWithConfig(config)
	}
	return NewStandardParserWithConfig(config)
}

// CreateDefaultParser implements ParserFactory.CreateDefaultParser.
func (pf *DefaultParserFactory) CreateDefaultParser() YAMLParser {
	return NewStandardParser()
}

// CreateStrictParser implements ParserFactory.CreateStrictParser.
func (pf *DefaultParserFactory) CreateStrictParser() YAMLParser {
	config := StrictParserConfig()
	return pf.CreateParser(config)
}
