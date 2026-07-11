// Package yamlutil provides schema validation interfaces for YAML processing.
//
// This module defines formal interfaces for schema-based validation, enabling
// extensible validation with custom constraints, composition rules, and type checking.
package yamlutil

import (
	"fmt"
	"reflect"
	"regexp"
)

// ============================================================================
// Schema Validation Interface
// ============================================================================

// SchemaDefinition defines the interface for validation rules.
//
// SchemaDefinition implementations define validation rules for YAML documents, including
// field definitions, type constraints, and validation logic. The Validate method
// ensures the schema definition itself is valid before use.
type SchemaDefinition interface {
	// Validate checks if the schema definition itself is valid.
	// Returns an error if the schema has invalid configuration.
	Validate() error

	// Name returns the schema name identifier.
	Name() string

	// Description returns a human-readable description of the schema.
	Description() string

	// Version returns the schema version for compatibility tracking.
	Version() string
}

// ============================================================================
// Schema Validator Interface
// ============================================================================

// SchemaValidationHandler defines the interface for schema-based validation.
//
// SchemaValidationHandler implementations validate YAML data against schema definitions,
// providing comprehensive error reporting for validation failures.
type SchemaValidationHandler interface {
	// ValidateSchema validates the schema definition itself.
	// Returns an error if the schema is invalid or cannot be used for validation.
	ValidateSchema(schema SchemaDefinition) error

	// ValidateValue validates a single value against a field definition.
	// Returns a ValidationError if the value doesn't satisfy constraints.
	ValidateValue(fieldPath string, value interface{}, fieldDef *FieldDefinition) error

	// Validate validates YAML data against the schema.
	// Returns a SchemaValidationResult with detailed error information.
	Validate(data map[string]interface{}) SchemaValidationResult

	// ValidateFile validates a YAML file against the schema.
	// Returns a SchemaValidationResult with detailed error information.
	ValidateFile(filePath string) SchemaValidationResult
}

// ============================================================================
// Constraint Interfaces
// ============================================================================

// Constraint defines the base interface for all validation constraints.
//
// Constraint implementations validate that values satisfy specific rules.
// Each constraint type specializes in validating particular aspects of values.
type Constraint interface {
	// Validate checks if a value satisfies this constraint.
	// Returns nil if the constraint is satisfied, or a ConstraintError if violated.
	Validate(value interface{}) *ConstraintError

	// Description returns a human-readable description of the constraint.
	Description() string

	// ConstraintType returns the type identifier for this constraint.
	ConstraintType() string
}

// StringConstraint defines validation rules for string values.
//
// StringConstraint provides validation for string-specific properties including
// length, pattern matching, and format validation.
type StringConstraint interface {
	Constraint

	// MinLength returns the minimum allowed string length (0 = no minimum).
	MinLength() int

	// MaxLength returns the maximum allowed string length (0 = no maximum).
	MaxLength() int

	// Pattern returns the regex pattern for string validation (empty = no pattern).
	Pattern() *regexp.Regexp

	// AllowedValues returns the list of allowed string values (nil = any value).
	AllowedValues() []string

	// Format returns the string format validator (e.g., "email", "uuid").
	Format() string
}

// NumberConstraint defines validation rules for numeric values.
//
// NumberConstraint provides validation for numeric-specific properties including
// range bounds, multiples, and precision requirements.
type NumberConstraint interface {
	Constraint

	// Min returns the minimum allowed value (nil = no minimum).
	Min() *float64

	// Max returns the maximum allowed value (nil = no maximum).
	Max() *float64

	// ExclusiveMin indicates whether the minimum value is exclusive.
	ExclusiveMin() bool

	// ExclusiveMax indicates whether the maximum value is exclusive.
	ExclusiveMax() bool

	// MultipleOf returns the value that valid numbers must be a multiple of (nil = no requirement).
	MultipleOf() *float64
}

// ArrayConstraint defines validation rules for array values.
//
// ArrayConstraint provides validation for array-specific properties including
// item count, item schema validation, and uniqueness constraints.
type ArrayConstraint interface {
	Constraint

	// MinItems returns the minimum number of items required (0 = no minimum).
	MinItems() int

	// MaxItems returns the maximum number of items allowed (0 = no maximum).
	MaxItems() int

	// UniqueItems indicates whether all items must be unique.
	UniqueItems() bool

	// ItemSchema returns the schema for validating array items (nil = no item validation).
	ItemSchema() *FieldDefinition

	// ContainsSchema returns a schema that at least one item must match (nil = no requirement).
	ContainsSchema() *FieldDefinition
}

// ObjectConstraint defines validation rules for object/map values.
//
// ObjectConstraint provides validation for object-specific properties including
// required fields, property names, and nested schema validation.
type ObjectConstraint interface {
	Constraint

	// RequiredFields returns the list of required field names (nil = no required fields).
	RequiredFields() []string

	// AllowedProperties returns the list of allowed property names (nil = any property).
	AllowedProperties() []string

	// PatternProperties returns regex patterns for property names (nil = no pattern requirements).
	PatternProperties() []*regexp.Regexp

	// MinProperties returns the minimum number of properties required (0 = no minimum).
	MinProperties() int

	// MaxProperties returns the maximum number of properties allowed (0 = no maximum).
	MaxProperties() int

	// PropertySchema returns the schema for validating property values (nil = no validation).
	PropertySchema() map[string]*FieldDefinition
}

// BooleanConstraint defines validation rules for boolean values.
//
// BooleanConstraint provides validation for boolean-specific properties.
type BooleanConstraint interface {
	Constraint

	// AllowedValues returns which boolean values are allowed (nil = both true and false allowed).
	AllowedValues() []bool
}

// ============================================================================
// Type Constraint Interface
// ============================================================================

// TypeConstraint defines the interface for runtime type checking.
//
// TypeConstraint implementations validate that values match expected types,
// supporting both basic YAML types and custom type definitions.
type TypeConstraint interface {
	Constraint

	// ExpectedType returns the expected type name.
	ExpectedType() string

	// CheckType returns true if a value matches the expected type.
	CheckType(value interface{}) bool

	// Coerce attempts to coerce a value to the expected type.
	// Returns the coerced value and an error if coercion fails.
	Coerce(value interface{}) (interface{}, error)

	// IsNullable returns true if nil/null values are allowed.
	IsNullable() bool
}

// ============================================================================
// Schema Composition Support
// ============================================================================

// CompositionType defines the type of schema composition.
type CompositionType string

const (
	// CompositionAllOf requires data to satisfy all component schemas.
	CompositionAllOf CompositionType = "allOf"

	// CompositionAnyOf requires data to satisfy at least one component schema.
	CompositionAnyOf CompositionType = "anyOf"

	// CompositionOneOf requires data to satisfy exactly one component schema.
	CompositionOneOf CompositionType = "oneOf"

	// CompositionNot requires data to NOT satisfy the component schema.
	CompositionNot CompositionType = "not"
)

// ComposableSchema defines the interface for schemas that support composition.
//
// ComposableSchema implementations can combine multiple schemas using
// composition rules (allOf, anyOf, oneOf, not) to create complex validation logic.
type ComposableSchema interface {
	SchemaDefinition

	// AllOf returns schemas that must all be satisfied.
	AllOf() []SchemaDefinition

	// AnyOf returns schemas where at least one must be satisfied.
	AnyOf() []SchemaDefinition

	// OneOf returns schemas where exactly one must be satisfied.
	OneOf() []SchemaDefinition

	// Not returns a schema that must not be satisfied.
	Not() SchemaDefinition

	// AddAllOf adds schemas to the allOf composition.
	AddAllOf(schemas ...SchemaDefinition) error

	// AddAnyOf adds schemas to the anyOf composition.
	AddAnyOf(schemas ...SchemaDefinition) error

	// AddOneOf adds schemas to the oneOf composition.
	AddOneOf(schemas ...SchemaDefinition) error

	// SetNot sets the schema for the not composition.
	SetNot(schema SchemaDefinition) error
}

// SchemaComposer defines the interface for composing schemas.
//
// SchemaComposer implementations provide logic for evaluating composition
// rules and determining if data satisfies the composed schema requirements.
type SchemaComposer interface {
	// ValidateComposition validates data against a composed schema.
	ValidateComposition(data map[string]interface{}, schema ComposableSchema) SchemaValidationResult

	// ValidateAllOf validates that data satisfies all schemas in the list.
	ValidateAllOf(data map[string]interface{}, schemas []SchemaDefinition) []SchemaValidationError

	// ValidateAnyOf validates that data satisfies at least one schema in the list.
	ValidateAnyOf(data map[string]interface{}, schemas []SchemaDefinition) []SchemaValidationError

	// ValidateOneOf validates that data satisfies exactly one schema in the list.
	ValidateOneOf(data map[string]interface{}, schemas []SchemaDefinition) []SchemaValidationError

	// ValidateNot validates that data does not satisfy the given schema.
	ValidateNot(data map[string]interface{}, schema SchemaDefinition) *SchemaValidationError
}

// ============================================================================
// Concrete Constraint Implementations
// ============================================================================

// StringConstraintImpl implements StringConstraint.
type StringConstraintImpl struct {
	minLength      int
	maxLength      int
	pattern        *regexp.Regexp
	allowedValues  []string
	format         string
	description    string
}

// NewStringConstraint creates a new StringConstraintImpl.
func NewStringConstraint(minLength, maxLength int, pattern string, allowedValues []string, format string) *StringConstraintImpl {
	var patternRegex *regexp.Regexp
	if pattern != "" {
		patternRegex = regexp.MustCompile(pattern)
	}

	return &StringConstraintImpl{
		minLength:     minLength,
		maxLength:     maxLength,
		pattern:       patternRegex,
		allowedValues: allowedValues,
		format:        format,
		description: fmt.Sprintf("string constraint: minLen=%d, maxLen=%d, pattern=%s, format=%s",
			minLength, maxLength, pattern, format),
	}
}

func (sc *StringConstraintImpl) Validate(value interface{}) *ConstraintError {
	str, ok := value.(string)
	if !ok {
		return &ConstraintError{
			Constraint:     fmt.Sprintf("value is not a string: %T", value),
			ConstraintType: "string",
			Value:          fmt.Sprintf("%v", value),
		}
	}

	if sc.minLength > 0 && len(str) < sc.minLength {
		return &ConstraintError{
			Constraint:     fmt.Sprintf("string length %d is less than minimum %d", len(str), sc.minLength),
			ConstraintType: "min_length",
			Value:          str,
		}
	}

	if sc.maxLength > 0 && len(str) > sc.maxLength {
		return &ConstraintError{
			Constraint:     fmt.Sprintf("string length %d exceeds maximum %d", len(str), sc.maxLength),
			ConstraintType: "max_length",
			Value:          str,
		}
	}

	if sc.pattern != nil && !sc.pattern.MatchString(str) {
		return &ConstraintError{
			Constraint:     fmt.Sprintf("string does not match pattern: %s", sc.pattern.String()),
			ConstraintType: "pattern",
			Value:          str,
		}
	}

	if len(sc.allowedValues) > 0 {
		allowed := false
		for _, allowedVal := range sc.allowedValues {
			if str == allowedVal {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ConstraintError{
				Constraint:     fmt.Sprintf("string value %q is not in allowed values: %v", str, sc.allowedValues),
				ConstraintType: "allowed_values",
				Value:          str,
			}
		}
	}

	return nil
}

func (sc *StringConstraintImpl) Description() string {
	return sc.description
}

func (sc *StringConstraintImpl) ConstraintType() string {
	return "string"
}

func (sc *StringConstraintImpl) MinLength() int {
	return sc.minLength
}

func (sc *StringConstraintImpl) MaxLength() int {
	return sc.maxLength
}

func (sc *StringConstraintImpl) Pattern() *regexp.Regexp {
	return sc.pattern
}

func (sc *StringConstraintImpl) AllowedValues() []string {
	return sc.allowedValues
}

func (sc *StringConstraintImpl) Format() string {
	return sc.format
}

// NumberConstraintImpl implements NumberConstraint.
type NumberConstraintImpl struct {
	min          *float64
	max          *float64
	exclusiveMin bool
	exclusiveMax bool
	multipleOf   *float64
	description  string
}

// NewNumberConstraint creates a new NumberConstraintImpl.
func NewNumberConstraint(min, max, multipleOf *float64, exclusiveMin, exclusiveMax bool) *NumberConstraintImpl {
	desc := "number constraint"
	if min != nil {
		desc += fmt.Sprintf(", min=%f", *min)
	}
	if max != nil {
		desc += fmt.Sprintf(", max=%f", *max)
	}
	if multipleOf != nil {
		desc += fmt.Sprintf(", multipleOf=%f", *multipleOf)
	}

	return &NumberConstraintImpl{
		min:          min,
		max:          max,
		exclusiveMin: exclusiveMin,
		exclusiveMax: exclusiveMax,
		multipleOf:   multipleOf,
		description:  desc,
	}
}

func (nc *NumberConstraintImpl) Validate(value interface{}) *ConstraintError {
	num, err := toFloat64(value)
	if err != nil {
		return &ConstraintError{
			Constraint: fmt.Sprintf("value is not a number: %v", err),
		}
	}

	if nc.min != nil {
		if nc.exclusiveMin {
			if num <= *nc.min {
				return &ConstraintError{
					Constraint: fmt.Sprintf("number %f is not greater than minimum %f", num, *nc.min),
				}
			}
		} else {
			if num < *nc.min {
				return &ConstraintError{
					Constraint: fmt.Sprintf("number %f is less than minimum %f", num, *nc.min),
				}
			}
		}
	}

	if nc.max != nil {
		if nc.exclusiveMax {
			if num >= *nc.max {
				return &ConstraintError{
					Constraint: fmt.Sprintf("number %f is not less than maximum %f", num, *nc.max),
				}
			}
		} else {
			if num > *nc.max {
				return &ConstraintError{
					Constraint: fmt.Sprintf("number %f exceeds maximum %f", num, *nc.max),
				}
			}
		}
	}

	if nc.multipleOf != nil && *nc.multipleOf != 0 {
		if remainder := num / *nc.multipleOf; remainder != float64(int(remainder)) {
			return &ConstraintError{
				Constraint: fmt.Sprintf("number %f is not a multiple of %f", num, *nc.multipleOf),
			}
		}
	}

	return nil
}

func (nc *NumberConstraintImpl) Description() string {
	return nc.description
}

func (nc *NumberConstraintImpl) ConstraintType() string {
	return "number"
}

func (nc *NumberConstraintImpl) Min() *float64 {
	return nc.min
}

func (nc *NumberConstraintImpl) Max() *float64 {
	return nc.max
}

func (nc *NumberConstraintImpl) ExclusiveMin() bool {
	return nc.exclusiveMin
}

func (nc *NumberConstraintImpl) ExclusiveMax() bool {
	return nc.exclusiveMax
}

func (nc *NumberConstraintImpl) MultipleOf() *float64 {
	return nc.multipleOf
}

// ArrayConstraintImpl implements ArrayConstraint.
type ArrayConstraintImpl struct {
	minItems       int
	maxItems       int
	uniqueItems    bool
	itemSchema     *FieldDefinition
	containsSchema  *FieldDefinition
	description    string
}

// NewArrayConstraint creates a new ArrayConstraintImpl.
func NewArrayConstraint(minItems, maxItems int, uniqueItems bool, itemSchema, containsSchema *FieldDefinition) *ArrayConstraintImpl {
	return &ArrayConstraintImpl{
		minItems:      minItems,
		maxItems:      maxItems,
		uniqueItems:   uniqueItems,
		itemSchema:    itemSchema,
		containsSchema: containsSchema,
		description: fmt.Sprintf("array constraint: minItems=%d, maxItems=%d, unique=%v",
			minItems, maxItems, uniqueItems),
	}
}

func (ac *ArrayConstraintImpl) Validate(value interface{}) *ConstraintError {
	arr, ok := value.([]interface{})
	if !ok {
		return &ConstraintError{
			Constraint: fmt.Sprintf("value is not an array: %T", value),
		}
	}

	if ac.minItems > 0 && len(arr) < ac.minItems {
		return &ConstraintError{
			Constraint: fmt.Sprintf("array length %d is less than minimum %d", len(arr), ac.minItems),
		}
	}

	if ac.maxItems > 0 && len(arr) > ac.maxItems {
		return &ConstraintError{
			Constraint: fmt.Sprintf("array length %d exceeds maximum %d", len(arr), ac.maxItems),
		}
	}

	if ac.uniqueItems {
		seen := make(map[interface{}]bool)
		for _, item := range arr {
			if seen[item] {
				return &ConstraintError{
					Constraint: fmt.Sprintf("array contains duplicate items: %v", item),
				}
			}
			seen[item] = true
		}
	}

	return nil
}

func (ac *ArrayConstraintImpl) Description() string {
	return ac.description
}

func (ac *ArrayConstraintImpl) ConstraintType() string {
	return "array"
}

func (ac *ArrayConstraintImpl) MinItems() int {
	return ac.minItems
}

func (ac *ArrayConstraintImpl) MaxItems() int {
	return ac.maxItems
}

func (ac *ArrayConstraintImpl) UniqueItems() bool {
	return ac.uniqueItems
}

func (ac *ArrayConstraintImpl) ItemSchema() *FieldDefinition {
	return ac.itemSchema
}

func (ac *ArrayConstraintImpl) ContainsSchema() *FieldDefinition {
	return ac.containsSchema
}

// ObjectConstraintImpl implements ObjectConstraint.
type ObjectConstraintImpl struct {
	requiredFields    []string
	allowedProperties []string
	patternProperties  []*regexp.Regexp
	minProperties     int
	maxProperties     int
	propertySchema    map[string]*FieldDefinition
	description       string
}

// NewObjectConstraint creates a new ObjectConstraintImpl.
func NewObjectConstraint(requiredFields, allowedProperties []string, minProperties, maxProperties int, propertySchema map[string]*FieldDefinition) *ObjectConstraintImpl {
	return &ObjectConstraintImpl{
		requiredFields:    requiredFields,
		allowedProperties: allowedProperties,
		minProperties:     minProperties,
		maxProperties:     maxProperties,
		propertySchema:    propertySchema,
		description: fmt.Sprintf("object constraint: minProps=%d, maxProps=%d, required=%v",
			minProperties, maxProperties, requiredFields),
	}
}

func (oc *ObjectConstraintImpl) Validate(value interface{}) *ConstraintError {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return &ConstraintError{
			Constraint: fmt.Sprintf("value is not an object: %T", value),
		}
	}

	propertyCount := len(obj)

	if oc.minProperties > 0 && propertyCount < oc.minProperties {
		return &ConstraintError{
			Constraint: fmt.Sprintf("object property count %d is less than minimum %d", propertyCount, oc.minProperties),
		}
	}

	if oc.maxProperties > 0 && propertyCount > oc.maxProperties {
		return &ConstraintError{
			Constraint: fmt.Sprintf("object property count %d exceeds maximum %d", propertyCount, oc.maxProperties),
		}
	}

	for _, requiredField := range oc.requiredFields {
		if _, exists := obj[requiredField]; !exists {
			return &ConstraintError{
				Constraint: fmt.Sprintf("object missing required property: %s", requiredField),
			}
		}
	}

	if len(oc.allowedProperties) > 0 {
		for prop := range obj {
			allowed := false
			for _, allowedProp := range oc.allowedProperties {
				if prop == allowedProp {
					allowed = true
					break
				}
			}
			if !allowed {
				return &ConstraintError{
					Constraint: fmt.Sprintf("object has disallowed property: %s", prop),
				}
			}
		}
	}

	return nil
}

func (oc *ObjectConstraintImpl) Description() string {
	return oc.description
}

func (oc *ObjectConstraintImpl) ConstraintType() string {
	return "object"
}

func (oc *ObjectConstraintImpl) RequiredFields() []string {
	return oc.requiredFields
}

func (oc *ObjectConstraintImpl) AllowedProperties() []string {
	return oc.allowedProperties
}

func (oc *ObjectConstraintImpl) PatternProperties() []*regexp.Regexp {
	return oc.patternProperties
}

func (oc *ObjectConstraintImpl) MinProperties() int {
	return oc.minProperties
}

func (oc *ObjectConstraintImpl) MaxProperties() int {
	return oc.maxProperties
}

func (oc *ObjectConstraintImpl) PropertySchema() map[string]*FieldDefinition {
	return oc.propertySchema
}

// BooleanConstraintImpl implements BooleanConstraint.
type BooleanConstraintImpl struct {
	allowedValues []bool
	description   string
}

// NewBooleanConstraint creates a new BooleanConstraintImpl.
func NewBooleanConstraint(allowedValues []bool) *BooleanConstraintImpl {
	if len(allowedValues) == 0 {
		allowedValues = []bool{true, false}
	}
	return &BooleanConstraintImpl{
		allowedValues: allowedValues,
		description:   fmt.Sprintf("boolean constraint: allowedValues=%v", allowedValues),
	}
}

func (bc *BooleanConstraintImpl) Validate(value interface{}) *ConstraintError {
	b, ok := value.(bool)
	if !ok {
		return &ConstraintError{
			Constraint: fmt.Sprintf("value is not a boolean: %T", value),
		}
	}

	for _, allowedVal := range bc.allowedValues {
		if b == allowedVal {
			return nil
		}
	}

	return &ConstraintError{
		Constraint: fmt.Sprintf("boolean value %v is not in allowed values: %v", b, bc.allowedValues),
	}
}

func (bc *BooleanConstraintImpl) Description() string {
	return bc.description
}

func (bc *BooleanConstraintImpl) ConstraintType() string {
	return "boolean"
}

func (bc *BooleanConstraintImpl) AllowedValues() []bool {
	return bc.allowedValues
}

// TypeConstraintImpl implements TypeConstraint.
type TypeConstraintImpl struct {
	expectedType string
	isNullable    bool
	customChecker func(interface{}) bool
	coercer       func(interface{}) (interface{}, error)
	description   string
}

// NewTypeConstraint creates a new TypeConstraintImpl.
func NewTypeConstraint(expectedType string, nullable bool) *TypeConstraintImpl {
	return &TypeConstraintImpl{
		expectedType: expectedType,
		isNullable:   nullable,
		description:  fmt.Sprintf("type constraint: expected=%s, nullable=%v", expectedType, nullable),
	}
}

func (tc *TypeConstraintImpl) Validate(value interface{}) *ConstraintError {
	if value == nil {
		if !tc.isNullable {
			return &ConstraintError{
				Constraint: fmt.Sprintf("null value is not allowed for non-nullable type %s", tc.expectedType),
			}
		}
		return nil
	}

	if !tc.CheckType(value) {
		return &ConstraintError{
			Constraint: fmt.Sprintf("value type %T does not match expected type %s", value, tc.expectedType),
		}
	}

	return nil
}

func (tc *TypeConstraintImpl) Description() string {
	return tc.description
}

func (tc *TypeConstraintImpl) ConstraintType() string {
	return "type"
}

func (tc *TypeConstraintImpl) ExpectedType() string {
	return tc.expectedType
}

func (tc *TypeConstraintImpl) CheckType(value interface{}) bool {
	if tc.customChecker != nil {
		return tc.customChecker(value)
	}

	switch tc.expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer", "int":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "number", "float":
		switch value.(type) {
		case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "boolean", "bool":
		_, ok := value.(bool)
		return ok
	case "array", "list":
		_, ok := value.([]interface{})
		return ok
	case "object", "map":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true
	}
}

func (tc *TypeConstraintImpl) Coerce(value interface{}) (interface{}, error) {
	if tc.coercer != nil {
		return tc.coercer(value)
	}

	// Default coercion logic
	if value == nil {
		return nil, nil
	}

	switch tc.expectedType {
	case "string":
		return fmt.Sprintf("%v", value), nil
	case "number":
		return toFloat64(value)
	case "integer":
		if f, err := toFloat64(value); err == nil {
			return int(f), nil
		}
		return nil, fmt.Errorf("cannot coerce to integer")
	default:
		return value, nil
	}
}

func (tc *TypeConstraintImpl) IsNullable() bool {
	return tc.isNullable
}

// SetCustomChecker sets a custom type checker function.
func (tc *TypeConstraintImpl) SetCustomChecker(checker func(interface{}) bool) {
	tc.customChecker = checker
}

// SetCoercer sets a custom coercion function.
func (tc *TypeConstraintImpl) SetCoercer(coercer func(interface{}) (interface{}, error)) {
	tc.coercer = coercer
}

// ============================================================================
// Helper Functions
// ============================================================================

// toFloat64 converts a value to float64 if possible.
func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// GetTypeName returns the type name of a value using reflection.
func GetTypeName(value interface{}) string {
	if value == nil {
		return "null"
	}

	t := reflect.TypeOf(value)
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Array, reflect.Slice:
		return "array"
	case reflect.Map:
		return "object"
	default:
		return t.Name()
	}
}
