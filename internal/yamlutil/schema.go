// Package yamlutil provides schema-based YAML validation.
//
// This module implements JSON Schema-style validation for YAML documents,
// enabling structural validation with type checking, required field validation,
// and constraint enforcement.
package yamlutil

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema represents a YAML schema definition for validation.
//
// Schema defines the expected structure of YAML documents, including field
// definitions, type constraints, required fields, and validation rules.
type Schema struct {
	// Type specifies the schema format type.
	Type SchemaType

	// Name is an optional identifier for the schema.
	Name string

	// Description provides a human-readable description of the schema.
	Description string

	// Version specifies the schema version for compatibility tracking.
	Version string

	// RootFields defines the top-level fields in the schema.
	RootFields map[string]*FieldDefinition

	// NestedSchemas contains subschemas for nested structures.
	NestedSchemas map[string]*Schema

	// definitions stores reusable schema definitions.
	definitions map[string]*FieldDefinition
}

// FieldDefinition defines a single field in the schema.
type FieldDefinition struct {
	// Type specifies the expected data type.
	Type string

	// Description provides a human-readable description.
	Description string

	// Required specifies whether the field must be present.
	Required bool

	// Optional specifies whether the field can be omitted.
	Optional bool

	// Min specifies minimum value or length constraint.
	Min *int

	// Max specifies maximum value or length constraint.
	Max *int

	// Pattern specifies a regex pattern for string validation.
	Pattern string

	// AllowedValues specifies an enumeration of allowed values.
	AllowedValues []interface{}

	// DefaultValue specifies the default value if missing.
	DefaultValue interface{}

	// NestedSchema defines the schema for nested objects/arrays.
	NestedSchema *Schema

	// ArrayItemSchema defines the schema for array items.
	ArrayItemSchema *FieldDefinition

	// customValidators stores additional validation functions.
	customValidators []FieldValidator
}

// SchemaValidator provides schema-based validation for YAML documents.
type SchemaValidator struct {
	schema  *Schema
	config  *ValidatorConfig
	compiled bool
}

// NewSchemaValidator creates a new schema validator with the given schema.
//
// The validator uses default configuration unless configured otherwise.
// The schema is compiled on first validation for performance.
func NewSchemaValidator(schema *Schema) *SchemaValidator {
	return &SchemaValidator{
		schema: schema,
		config: DefaultValidatorConfig(),
	}
}

// NewSchemaValidatorWithConfig creates a new schema validator with custom configuration.
//
// The configuration controls validation strictness and behavior.
func NewSchemaValidatorWithConfig(schema *Schema, config *ValidatorConfig) *SchemaValidator {
	return &SchemaValidator{
		schema: schema,
		config: config,
	}
}

// Validate validates YAML data against the schema.
//
// Returns a comprehensive SchemaValidationResult with all errors and warnings found.
// The validation checks:
// - Required field presence
// - Field type correctness
// - Constraint satisfaction (min, max, pattern, allowed values)
// - Nested structure validation
func (sv *SchemaValidator) Validate(data map[string]interface{}) SchemaValidationResult {
	result := SchemaValidationResult{
		Valid:                true,
		Errors:               []SchemaValidationError{},
		Warnings:             []SchemaValidationError{},
		MissingRequiredFields: []string{},
		TypeMismatches:       []FieldTypeError{},
		ConstraintViolations: []ConstraintViolation{},
	}

	// Compile schema if not already compiled
	if !sv.compiled {
		if err := sv.schema.Validate(); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, SchemaValidationError{
				Message: fmt.Sprintf("Invalid schema: %v", err),
			})
			return result
		}
		sv.compiled = true
	}

	// Validate root fields
	sv.validateFields(data, sv.schema.RootFields, "", &result)

	// Set overall valid flag
	result.Valid = !result.HasErrors()

	return result
}

// ValidateFile validates a YAML file against the schema.
//
// Reads and parses the file, then validates against the schema.
// Returns SchemaValidationResult with detailed error information.
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
	result := SchemaValidationResult{
		Valid:                true,
		Errors:               []SchemaValidationError{},
		Warnings:             []SchemaValidationError{},
		MissingRequiredFields: []string{},
		TypeMismatches:       []FieldTypeError{},
		ConstraintViolations: []ConstraintViolation{},
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, SchemaValidationError{
			Message: fmt.Sprintf("Failed to read file: %v", err),
		})
		return result
	}

	// Parse YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, SchemaValidationError{
			Message: fmt.Sprintf("Failed to parse YAML: %v", err),
		})
		return result
	}

	// Validate against schema
	return sv.Validate(data)
}

// validateFields validates data fields against their definitions.
func (sv *SchemaValidator) validateFields(
	data map[string]interface{},
	fields map[string]*FieldDefinition,
	pathPrefix string,
	result *SchemaValidationResult,
) {
	// Check required fields
	for fieldName, fieldDef := range fields {
		if fieldDef.Required {
			fullPath := sv.joinPath(pathPrefix, fieldName)
			if _, exists := data[fieldName]; !exists {
				result.Valid = false
				result.MissingRequiredFields = append(result.MissingRequiredFields, fullPath)
			}
		}
	}

	// Validate existing fields
	for fieldName, value := range data {
		fieldDef, exists := fields[fieldName]
		if !exists {
			if sv.config.Strict {
				result.Warnings = append(result.Warnings, SchemaValidationError{
					FieldPath:      sv.joinPath(pathPrefix, fieldName),
					Message:        "Unknown field in strict mode",
					ConstraintType: "unknown_field",
				})
			}
			continue
		}

		fullPath := sv.joinPath(pathPrefix, fieldName)
		sv.validateField(value, fieldDef, fullPath, result)
	}
}

// validateField validates a single field value against its definition.
func (sv *SchemaValidator) validateField(
	value interface{},
	fieldDef *FieldDefinition,
	fieldPath string,
	result *SchemaValidationResult,
) {
	// Type validation
	if !sv.validateType(value, fieldDef.Type) {
		result.Valid = false
		result.TypeMismatches = append(result.TypeMismatches, FieldTypeError{
			FieldPath:    fieldPath,
			ExpectedType: fieldDef.Type,
			ActualType:   sv.getTypeName(value),
		})
		return
	}

	// Constraint validation
	sv.validateConstraints(value, fieldDef, fieldPath, result)

	// Nested structure validation
	if fieldDef.NestedSchema != nil {
		if nestedMap, ok := value.(map[string]interface{}); ok {
			nestedPrefix := fieldPath
			sv.validateFields(nestedMap, fieldDef.NestedSchema.RootFields, nestedPrefix, result)
		}
	}

	// Array validation
	if fieldDef.ArrayItemSchema != nil {
		if array, ok := value.([]interface{}); ok {
			for i, item := range array {
				itemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
				sv.validateField(item, fieldDef.ArrayItemSchema, itemPath, result)
			}
		}
	}
}

// validateType checks if a value matches the expected type.
func (sv *SchemaValidator) validateType(value interface{}, expectedType string) bool {
	if expectedType == "" || expectedType == "any" {
		return true
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "int", "integer":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "float", "number":
		switch value.(type) {
		case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "bool", "boolean":
		_, ok := value.(bool)
		return ok
	case "array", "list":
		_, ok := value.([]interface{})
		return ok
	case "object", "map":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true // Unknown type, accept
	}
}

// validateConstraints checks if a value satisfies its constraints.
func (sv *SchemaValidator) validateConstraints(
	value interface{},
	fieldDef *FieldDefinition,
	fieldPath string,
	result *SchemaValidationResult,
) {
	// Min constraint
	if fieldDef.Min != nil {
		if !sv.checkMinConstraint(value, *fieldDef.Min) {
			result.Valid = false
			result.ConstraintViolations = append(result.ConstraintViolations, ConstraintViolation{
				FieldPath:       fieldPath,
				ConstraintType:  "min",
				Message:         fmt.Sprintf("Value violates minimum constraint: %d", *fieldDef.Min),
				ActualValue:     value,
				ConstraintValue: *fieldDef.Min,
			})
		}
	}

	// Max constraint
	if fieldDef.Max != nil {
		if !sv.checkMaxConstraint(value, *fieldDef.Max) {
			result.Valid = false
			result.ConstraintViolations = append(result.ConstraintViolations, ConstraintViolation{
				FieldPath:       fieldPath,
				ConstraintType:  "max",
				Message:         fmt.Sprintf("Value violates maximum constraint: %d", *fieldDef.Max),
				ActualValue:     value,
				ConstraintValue: *fieldDef.Max,
			})
		}
	}

	// Pattern constraint
	if fieldDef.Pattern != "" {
		if strVal, ok := value.(string); ok {
			matched, err := regexp.MatchString(fieldDef.Pattern, strVal)
			if err != nil || !matched {
				result.Valid = false
				result.ConstraintViolations = append(result.ConstraintViolations, ConstraintViolation{
					FieldPath:       fieldPath,
					ConstraintType:  "pattern",
					Message:         fmt.Sprintf("Value does not match pattern: %s", fieldDef.Pattern),
					ActualValue:     value,
					ConstraintValue: fieldDef.Pattern,
				})
			}
		}
	}

	// Allowed values constraint
	if len(fieldDef.AllowedValues) > 0 {
		if !sv.isAllowedValue(value, fieldDef.AllowedValues) {
			result.Valid = false
			result.ConstraintViolations = append(result.ConstraintViolations, ConstraintViolation{
				FieldPath:       fieldPath,
				ConstraintType:  "allowed_values",
				Message:         fmt.Sprintf("Value not in allowed list: %v", fieldDef.AllowedValues),
				ActualValue:     value,
				ConstraintValue: fieldDef.AllowedValues,
			})
		}
	}
}

// checkMinConstraint checks if a value satisfies the minimum constraint.
func (sv *SchemaValidator) checkMinConstraint(value interface{}, min int) bool {
	switch v := value.(type) {
	case int:
		return v >= min
	case int8:
		return int(v) >= min
	case int16:
		return int(v) >= min
	case int32:
		return int(v) >= min
	case int64:
		return int(v) >= min
	case uint:
		return int(v) >= min
	case uint8:
		return int(v) >= min
	case uint16:
		return int(v) >= min
	case uint32:
		return int(v) >= min
	case uint64:
		return int(v) >= min
	case float32:
		return float64(v) >= float64(min)
	case float64:
		return v >= float64(min)
	case string:
		return len(v) >= min
	case []interface{}:
		return len(v) >= min
	case map[string]interface{}:
		return len(v) >= min
	default:
		return true
	}
}

// checkMaxConstraint checks if a value satisfies the maximum constraint.
func (sv *SchemaValidator) checkMaxConstraint(value interface{}, max int) bool {
	switch v := value.(type) {
	case int:
		return v <= max
	case int8:
		return int(v) <= max
	case int16:
		return int(v) <= max
	case int32:
		return int(v) <= max
	case int64:
		return int(v) <= max
	case uint:
		return int(v) <= max
	case uint8:
		return int(v) <= max
	case uint16:
		return int(v) <= max
	case uint32:
		return int(v) <= max
	case uint64:
		return int(v) <= max
	case float32:
		return float64(v) <= float64(max)
	case float64:
		return v <= float64(max)
	case string:
		return len(v) <= max
	case []interface{}:
		return len(v) <= max
	case map[string]interface{}:
		return len(v) <= max
	default:
		return true
	}
}

// isAllowedValue checks if a value is in the allowed values list.
func (sv *SchemaValidator) isAllowedValue(value interface{}, allowed []interface{}) bool {
	for _, allowedVal := range allowed {
		if sv.valuesEqual(value, allowedVal) {
			return true
		}
	}
	return false
}

// valuesEqual compares two values for equality.
func (sv *SchemaValidator) valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Type-specific comparison
	switch va := a.(type) {
	case string:
		if vb, ok := b.(string); ok {
			return va == vb
		}
	case int:
		if vb, ok := b.(int); ok {
			return va == vb
		}
	case float64:
		if vb, ok := b.(float64); ok {
			return va == vb
		}
	case bool:
		if vb, ok := b.(bool); ok {
			return va == vb
		}
	}

	// Fallback to interface equality
	return a == b
}

// getTypeName returns the type name of a value.
func (sv *SchemaValidator) getTypeName(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}

// joinPath joins path components with dot notation.
func (sv *SchemaValidator) joinPath(prefix, component string) string {
	if prefix == "" {
		return component
	}
	return prefix + "." + component
}

// LoadSchema loads a schema definition from a file.
//
// Supports JSON and YAML schema files. Returns a compiled Schema object.
func LoadSchema(schemaPath string) (*Schema, error) {
	// Read file content
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, &SchemaError{
			Message:  fmt.Sprintf("Failed to read schema file: %v", err),
			FilePath: schemaPath,
		}
	}

	// Detect format and parse
	ext := strings.ToLower(schemaPath[strings.LastIndex(schemaPath, "."):])
	var data map[string]interface{}

	switch ext {
	case ".json":
		if err := json.Unmarshal(content, &data); err != nil {
			return nil, &SchemaError{
				Message:  fmt.Sprintf("Failed to parse JSON schema: %v", err),
				FilePath: schemaPath,
			}
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &data); err != nil {
			return nil, &SchemaError{
				Message:  fmt.Sprintf("Failed to parse YAML schema: %v", err),
				FilePath: schemaPath,
			}
		}
	default:
		return nil, &SchemaError{
			Message:  fmt.Sprintf("Unsupported schema format: %s", ext),
			FilePath: schemaPath,
		}
	}

	// Build schema from parsed data
	schema, err := buildSchemaFromData(data)
	if err != nil {
		return nil, &SchemaError{
			Message:  fmt.Sprintf("Failed to build schema: %v", err),
			FilePath: schemaPath,
		}
	}

	return schema, nil
}

// buildSchemaFromData constructs a Schema from parsed data.
func buildSchemaFromData(data map[string]interface{}) (*Schema, error) {
	schema := &Schema{
		Type:          SchemaTypeJSON,
		RootFields:    make(map[string]*FieldDefinition),
		NestedSchemas: make(map[string]*Schema),
		definitions:   make(map[string]*FieldDefinition),
	}

	// Extract schema metadata
	if name, ok := data["name"].(string); ok {
		schema.Name = name
	}
	if desc, ok := data["description"].(string); ok {
		schema.Description = desc
	}
	if version, ok := data["version"].(string); ok {
		schema.Version = version
	}

	// Extract field definitions
	if properties, ok := data["properties"].(map[string]interface{}); ok {
		for fieldName, fieldData := range properties {
			fieldMap, ok := fieldData.(map[string]interface{})
			if !ok {
				continue
			}

			fieldDef, err := buildFieldDefinition(fieldMap)
			if err != nil {
				return nil, fmt.Errorf("invalid field definition for %s: %w", fieldName, err)
			}

			// Check if required
			if required, ok := data["required"].([]interface{}); ok {
				for _, reqField := range required {
					if reqStr, ok := reqField.(string); ok && reqStr == fieldName {
						fieldDef.Required = true
						break
					}
				}
			}

			schema.RootFields[fieldName] = fieldDef
		}
	}

	return schema, nil
}

// buildFieldDefinition constructs a FieldDefinition from field data.
func buildFieldDefinition(data map[string]interface{}) (*FieldDefinition, error) {
	fieldDef := &FieldDefinition{}

	// Extract type
	if fieldTyp, ok := data["type"].(string); ok {
		fieldDef.Type = fieldTyp
	}

	// Extract description
	if desc, ok := data["description"].(string); ok {
		fieldDef.Description = desc
	}

	// Extract min/max constraints
	if minVal, ok := data["minimum"]; ok {
		if minInt, ok := minVal.(int); ok {
			fieldDef.Min = &minInt
		}
	}
	if maxVal, ok := data["maximum"]; ok {
		if maxInt, ok := maxVal.(int); ok {
			fieldDef.Max = &maxInt
		}
	}

	// Extract pattern
	if pattern, ok := data["pattern"].(string); ok {
		fieldDef.Pattern = pattern
	}

	// Extract allowed values (enum)
	if enum, ok := data["enum"].([]interface{}); ok {
		fieldDef.AllowedValues = enum
	}

	// Extract default value
	if defaultVal, ok := data["default"]; ok {
		fieldDef.DefaultValue = defaultVal
	}

	return fieldDef, nil
}

// Validate checks if the schema definition itself is valid.
func (s *Schema) Validate() error {
	if s == nil {
		return fmt.Errorf("schema is nil")
	}

	// Validate root fields
	for fieldName, fieldDef := range s.RootFields {
		if fieldDef == nil {
			return fmt.Errorf("field %s has nil definition", fieldName)
		}
		if err := s.validateFieldDefinition(fieldDef, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// validateFieldDefinition validates a single field definition.
func (s *Schema) validateFieldDefinition(fieldDef *FieldDefinition, fieldName string) error {
	// Check type is valid
	if fieldDef.Type != "" {
		validTypes := map[string]bool{
			"string": true, "integer": true, "number": true,
			"boolean": true, "array": true, "object": true,
		}
		if !validTypes[fieldDef.Type] {
			return fmt.Errorf("field %s has invalid type: %s", fieldName, fieldDef.Type)
		}
	}

	// Validate min/max constraints
	if fieldDef.Min != nil && fieldDef.Max != nil {
		if *fieldDef.Min > *fieldDef.Max {
			return fmt.Errorf("field %s has min > max", fieldName)
		}
	}

	return nil
}

// SchemaError represents an error in schema definition or loading.
type SchemaError struct {
	Message  string
	FilePath string
}

// Error implements the error interface.
func (se *SchemaError) Error() string {
	if se.FilePath != "" {
		return "Schema error in " + se.FilePath + ": " + se.Message
	}
	return "Schema error: " + se.Message
}

// FieldValidator is defined in config.go to consolidate all configuration types.
// AddCustomValidator is defined in config.go to consolidate all configuration types.

// SchemaType defines the schema format type.
type SchemaType string

const (
	// SchemaTypeJSON represents JSON Schema format
	SchemaTypeJSON SchemaType = "json"
	// SchemaTypeYAML represents YAML Schema format
	SchemaTypeYAML SchemaType = "yaml"
	// SchemaTypeCustom represents custom schema format
	SchemaTypeCustom SchemaType = "custom"
)

// SchemaValidationResult is defined in result_types.go to consolidate all result types.
// SchemaValidationResult is defined in result_types.go to consolidate all result types.
// SchemaValidationError is defined in errors.go to consolidate all error types.

// FieldTypeError represents a type mismatch between expected and actual types.
type FieldTypeError struct {
	FieldPath    string // Path to the field with type error
	ExpectedType string // Expected type
	ActualType   string // Actual type found
	Value        interface{} // Actual value
}

// Error implements the error interface.
func (fte FieldTypeError) Error() string {
	return fmt.Sprintf("type error at %s: expected %s, got %s", fte.FieldPath, fte.ExpectedType, fte.ActualType)
}

// ConstraintViolation represents a constraint validation failure.
type ConstraintViolation struct {
	FieldPath       string        // Path to the field with constraint violation
	ConstraintType  string        // Type of constraint that failed
	Message         string        // Human-readable error message
	ActualValue     interface{}   // Actual value that violated the constraint
	ConstraintValue interface{}   // Value or description of the constraint
	Severity        ConstraintSeverity // Severity of the violation
}

// Error implements the error interface.
func (cv ConstraintViolation) Error() string {
	return fmt.Sprintf("constraint violation at %s: %s", cv.FieldPath, cv.Message)
}

// ConstraintSeverity represents the severity level of a constraint violation.
type ConstraintSeverity int

const (
	// SeverityInfo indicates an informational violation
	SeverityInfo ConstraintSeverity = iota
	// SeverityWarning indicates a warning-level violation
	SeverityWarning
	// SeverityError indicates an error-level violation
	SeverityError
	// SeverityFatal indicates a fatal violation that prevents processing
	SeverityFatal
)
