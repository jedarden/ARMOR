// Package yamlutil provides schema-based YAML validation.
//
// This module implements JSON Schema-style validation for YAML documents,
// enabling structural validation with type checking, required field validation,
// and constraint enforcement.
//
// VALIDATE() CALLERS AUDIT (bf-17y15):
// This file contains the SchemaDefinition.Validate() method implementation (line 780)
// and the following call sites:
//
// 1. Line 190: sv.schema.Validate(data) - inside SchemaValidator.Validate() method
//    - Context: Validates data against the schema after compilation
//    - Error handling (lines 192-205): Type assertion to YAMLError for structured error codes
//    - Pattern: Checks if err.(YAMLError), extracts ErrorCode if available
//
// 2. Line 263: sv.Validate(data) - inside SchemaValidator.ValidateFile() method
//    - Context: Delegates to SchemaValidator.Validate() after parsing YAML file
//    - Error handling: Inherits structured error handling from SchemaValidator.Validate()
//    - Pattern: Returns SchemaValidationResult containing all errors/warnings
//
// NOTE: Both calls use the Schema interface (sv.schema is type Schema, not *SchemaDefinition).
// The actual SchemaDefinition.Validate() implementation is at line 780 and is dispatched
// via the interface when sv.schema holds a *SchemaDefinition.
package yamlutil

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Schema defines the interface for validating values against a schema.
//
// Schema provides a generic validation interface that can be implemented by
// different schema types (JSON Schema, YAML Schema, custom validation schemas, etc.)
// to validate arbitrary values against defined rules and constraints.
//
// The Validate method accepts any value type and returns an error if validation
// fails. Implementations may return schema-specific error types that provide
// detailed validation failure information.
//
// Example usage:
//
//	type MySchema struct { ... }
//	func (s *MySchema) Validate(value interface{}) error { ... }
//
//	schema := NewMySchema()
//	err := schema.Validate(data)
//	if err != nil {
//	    return fmt.Errorf("validation failed: %w", err)
//	}
type Schema interface {
	// Validate validates the given value against the schema rules.
	//
	// The value parameter can be of any type:
	//   - Primitive types (string, int, float64, bool, etc.)
	//   - Struct types for typed validation
	//   - map[string]interface{} for dynamic YAML/JSON data
	//   - []interface{} for array validation
	//   - Any other custom type
	//
	// Returns nil if the value conforms to the schema rules.
	// Returns an error if the value violates any schema constraints.
	// The error type may provide additional details about the validation failure.
	Validate(value interface{}) error
}

// SchemaDefinition represents a YAML schema definition for validation.
//
// SchemaDefinition defines the expected structure of YAML documents, including field
// definitions, type constraints, required fields, and validation rules.
// This implements the Schema interface.
type SchemaDefinition struct {
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
	NestedSchemas map[string]Schema

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
	NestedSchema Schema

	// ArrayItemSchema defines the schema for array items.
	ArrayItemSchema *FieldDefinition

	// customValidators stores additional validation functions.
	customValidators []FieldValidator
}

// SchemaValidator provides schema-based validation for YAML documents.
type SchemaValidator struct {
	schema  Schema
	config  *ValidatorConfig
	compiled bool
}

// NewSchemaValidator creates a new schema validator with the given schema.
//
// The validator uses default configuration unless configured otherwise.
// The schema is compiled on first validation for performance.
func NewSchemaValidator(schema Schema) *SchemaValidator {
	return &SchemaValidator{
		schema: schema,
		config: DefaultValidatorConfig(),
	}
}

// NewSchemaValidatorWithConfig creates a new schema validator with custom configuration.
//
// The configuration controls validation strictness and behavior.
func NewSchemaValidatorWithConfig(schema Schema, config *ValidatorConfig) *SchemaValidator {
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
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult {
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
		if err := sv.compileSchema(); err != nil {
			result.Valid = false

			// Handle YAMLError with structured information
			if yamlErr, ok := err.(YAMLError); ok {
				result.Errors = append(result.Errors, SchemaValidationError{
					Message:   fmt.Sprintf("Schema compilation failed: %s", yamlErr.Error()),
					ErrorCode: yamlErr.Code(),
				})
			} else {
				// Handle generic errors
				result.Errors = append(result.Errors, SchemaValidationError{
					Message: fmt.Sprintf("Schema compilation failed: %v", err),
				})
			}
			return result
		}
		sv.compiled = true
	}

	// Validate data against schema
	if err := sv.schema.Validate(data); err != nil {
		result.Valid = false

		// Handle YAMLError with structured information
		if yamlErr, ok := err.(YAMLError); ok {
			result.Errors = append(result.Errors, SchemaValidationError{
				Message:   yamlErr.Error(),
				ErrorCode: yamlErr.Code(),
			})
		} else {
			// Handle generic errors
			result.Errors = append(result.Errors, SchemaValidationError{
				Message: fmt.Sprintf("Validation failed: %v", err),
			})
		}
		return result
	}

	// For SchemaDefinition, do detailed field validation
	if schemaDef, ok := sv.schema.(*SchemaDefinition); ok {
		// Convert data to map if needed
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			result.Valid = false
			result.Errors = append(result.Errors, SchemaValidationError{
				Message: fmt.Sprintf("Expected map[string]interface{}, got %T", data),
			})
			return result
		}
		sv.validateFields(dataMap, schemaDef.RootFields, "", &result)
	}

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

// compileSchema compiles and validates the schema definition.
func (sv *SchemaValidator) compileSchema() error {
	if schemaDef, ok := sv.schema.(*SchemaDefinition); ok {
		if err := schemaDef.Compile(); err != nil {
			// Handle YAMLError with structured information
			if yamlErr, ok := err.(YAMLError); ok {
				return fmt.Errorf("schema compilation failed: %w", yamlErr)
			}
			// Handle generic errors
			return fmt.Errorf("schema compilation failed: %w", err)
		}
	}
	return nil
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
			if sv.config.StrictMode {
				result.Warnings = append(result.Warnings, SchemaValidationError{
					FieldPath:      sv.joinPath(pathPrefix, fieldName),
					Message:        "Unknown field in strict mode",
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
			// Type assert to SchemaDefinition to access RootFields
			if nestedSchemaDef, ok := fieldDef.NestedSchema.(*SchemaDefinition); ok {
				sv.validateFields(nestedMap, nestedSchemaDef.RootFields, nestedPrefix, result)
			}
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
// Supports JSON and YAML schema files. Returns a compiled SchemaDefinition object.
func LoadSchema(schemaPath string) (*SchemaDefinition, error) {
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
	schemaDef, err := buildSchemaFromData(data)
	if err != nil {
		return nil, &SchemaError{
			Message:  fmt.Sprintf("Failed to build schema: %v", err),
			FilePath: schemaPath,
		}
	}

	// Compile the schema
	if err := schemaDef.Compile(); err != nil {
		// Handle YAMLError with structured information
		if yamlErr, ok := err.(YAMLError); ok {
			return nil, &SchemaError{
				Message:  fmt.Sprintf("Failed to compile schema: %w", yamlErr),
				FilePath: schemaPath,
			}
		}
		// Handle generic errors
		return nil, &SchemaError{
			Message:  fmt.Sprintf("Failed to compile schema: %v", err),
			FilePath: schemaPath,
		}
	}

	return schemaDef, nil
}

// buildSchemaFromData constructs a SchemaDefinition from parsed data.
func buildSchemaFromData(data map[string]interface{}) (*SchemaDefinition, error) {
	schema := &SchemaDefinition{
		Type:          SchemaTypeJSON,
		RootFields:    make(map[string]*FieldDefinition),
		NestedSchemas: make(map[string]Schema),
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

// Compile checks if the schema definition itself is valid and compiles it for use.
func (s *SchemaDefinition) Compile() error {
	if s == nil {
		return NewSchemaLoadError("", "schema is nil", nil, ErrCodeSchemaInvalid)
	}

	// Validate root fields
	for fieldName, fieldDef := range s.RootFields {
		if fieldDef == nil {
			return NewValidationError("", fmt.Sprintf("field %s has nil definition", fieldName), fieldName, "", ErrCodeSchemaInvalid, 0, 0, ErrorTypeSchemaValidate, "")
		}
		if err := s.validateFieldDefinition(fieldDef, fieldName); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates data against the schema definition.
//
// This method satisfies the Schema interface. It validates the given value
// against the schema rules and returns an error if validation fails.
//
// The value parameter can be of any type but should typically be
// map[string]interface{} for YAML/JSON data validation.
func (s *SchemaDefinition) Validate(value interface{}) error {
	if value == nil {
		return NewValidationError("", "value cannot be nil", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")
	}

	// Convert to map if needed
	data, ok := value.(map[string]interface{})
	if !ok {
		return NewTypeMismatchError("", "", "map[string]interface{}", fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)
	}

	// Validate root fields
	for fieldName, fieldDef := range s.RootFields {
		if fieldDef.Required {
			if _, exists := data[fieldName]; !exists {
				return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)
			}
		}

		// Validate existing field
		if fieldValue, exists := data[fieldName]; exists {
			if err := s.validateField(fieldValue, fieldDef, fieldName); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateField validates a single field value against its definition.
func (s *SchemaDefinition) validateField(value interface{}, fieldDef *FieldDefinition, fieldPath string) error {
	// Type validation
	if !s.validateType(value, fieldDef.Type) {
		return NewTypeMismatchError("", fieldPath, fieldDef.Type, s.getTypeName(value), fmt.Sprintf("%v", value), 0, ErrCodeTypeMismatch)
	}

	// Constraint validation
	if err := s.validateConstraints(value, fieldDef, fieldPath); err != nil {
		return err
	}

	return nil
}

// validateType checks if a value matches the expected type.
func (s *SchemaDefinition) validateType(value interface{}, expectedType string) bool {
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
func (s *SchemaDefinition) validateConstraints(value interface{}, fieldDef *FieldDefinition, fieldPath string) error {
	// Min constraint
	if fieldDef.Min != nil {
		if !s.checkMinConstraint(value, *fieldDef.Min) {
			return NewConstraintError("", fieldPath, "min", fmt.Sprintf("must be >= %d", *fieldDef.Min), fmt.Sprintf("value violates minimum constraint %d", *fieldDef.Min), fmt.Sprintf("%v", value), 0, ErrCodeConstraintViolation)
		}
	}

	// Max constraint
	if fieldDef.Max != nil {
		if !s.checkMaxConstraint(value, *fieldDef.Max) {
			return NewConstraintError("", fieldPath, "max", fmt.Sprintf("must be <= %d", *fieldDef.Max), fmt.Sprintf("value violates maximum constraint %d", *fieldDef.Max), fmt.Sprintf("%v", value), 0, ErrCodeConstraintViolation)
		}
	}

	// Pattern constraint
	if fieldDef.Pattern != "" {
		if strVal, ok := value.(string); ok {
			matched, err := regexp.MatchString(fieldDef.Pattern, strVal)
			if err != nil || !matched {
				return NewConstraintError("", fieldPath, "pattern", fieldDef.Pattern, fmt.Sprintf("value does not match pattern '%s'", fieldDef.Pattern), strVal, 0, ErrCodeConstraintViolation)
			}
		}
	}

	// Allowed values constraint
	if len(fieldDef.AllowedValues) > 0 {
		if !s.isAllowedValue(value, fieldDef.AllowedValues) {
			return NewConstraintError("", fieldPath, "enum", fmt.Sprintf("must be one of: %v", fieldDef.AllowedValues), fmt.Sprintf("value not in allowed list"), fmt.Sprintf("%v", value), 0, ErrCodeInvalidValue)
		}
	}

	return nil
}

// checkMinConstraint checks if a value satisfies the minimum constraint.
func (s *SchemaDefinition) checkMinConstraint(value interface{}, min int) bool {
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
func (s *SchemaDefinition) checkMaxConstraint(value interface{}, max int) bool {
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
func (s *SchemaDefinition) isAllowedValue(value interface{}, allowed []interface{}) bool {
	for _, allowedVal := range allowed {
		if s.valuesEqual(value, allowedVal) {
			return true
		}
	}
	return false
}

// valuesEqual compares two values for equality.
func (s *SchemaDefinition) valuesEqual(a, b interface{}) bool {
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
func (s *SchemaDefinition) getTypeName(value interface{}) string {
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

// validateFieldDefinition validates a single field definition.
func (s *SchemaDefinition) validateFieldDefinition(fieldDef *FieldDefinition, fieldName string) error {
	// Check type is valid
	if fieldDef.Type != "" {
		validTypes := map[string]bool{
			"string": true, "integer": true, "number": true,
			"boolean": true, "array": true, "object": true,
		}
		if !validTypes[fieldDef.Type] {
			return NewValidationError("", fmt.Sprintf("field %s has invalid type: %s", fieldName, fieldDef.Type), fieldName, "valid type", ErrCodeInvalidValue, 0, 0, ErrorTypeSchemaValidate, "")
		}
	}

	// Validate min/max constraints
	if fieldDef.Min != nil && fieldDef.Max != nil {
		if *fieldDef.Min > *fieldDef.Max {
			return NewValidationError("", fmt.Sprintf("field %s has min > max", fieldName), fieldName, "min <= max", ErrCodeConstraintViolation, 0, 0, ErrorTypeSchemaValidate, "")
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

// Status represents the success or error state of a parse operation.
type Status string

const (
	// StatusSuccess indicates a successful parse operation
	StatusSuccess Status = "success"
	// StatusError indicates a failed parse operation
	StatusError Status = "error"
)
