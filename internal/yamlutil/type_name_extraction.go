// Package yamlutil provides type name extraction from yaml.TypeError messages.
//
// This module implements logic to extract and normalize type names from
// yaml.TypeError message strings, handling type names in different positions
// within error messages.
package yamlutil

import (
	"fmt"
	"regexp"
	"strings"
)

// EnhancedTypeErrorDetail represents detailed information extracted from a
// yaml.TypeError message string.
type EnhancedTypeErrorDetail struct {
	LineNumber  int    // Line number where the error occurred
	ColumnNumber int   // Column number where the error occurred
	FieldPath   string // Field path (e.g., "server.port", "items[0].name")
	Expected    string // Expected type (normalized)
	Actual      string // Actual type (normalized)
	Value       string // Actual value that caused the error
	RawError    string // Original error message
	Context     string // Generated error context message
}

// parseTypeErrorString extracts detailed information from a yaml.TypeError message string.
//
// This function parses error messages in various formats:
// - "line 10: cannot unmarshal !!str into int"
// - "field server.port type mismatch: expected int, got string"
// - "yaml: line 15: cannot unmarshal !!seq into []string"
//
// Returns an EnhancedTypeErrorDetail with all extracted information.
// extractTypeName extracts the first type name from a yaml.TypeError message string.
//
// This function uses basic regex patterns to extract type names from error messages,
// handling simple cases where type names appear in common positions.
//
// Patterns matched:
// - "cannot unmarshal !!<tag> into <type>" → extracts "<type>"
// - "expected <type>, got <type>" → extracts first "<type>"
// - "want <type>, got <type>" → extracts first "<type>"
// - "<type> cannot be converted to <type>" → extracts first "<type>"
// - Simple type names at start: "<type>: error" → extracts "<type>"
//
// Parameters:
//   errorStr - The error message string to parse
//
// Returns:
//   The first matched type name, or empty string if no match found
func extractTypeName(errorStr string) string {
	errorStr = strings.TrimSpace(errorStr)

	// Pattern 1: "cannot unmarshal !!<tag> into <type>" (handles complex types)
	// Matches: basic types (int), dotted types (time.Time), spaced types (chan int),
	//          arrays ([]string), maps (map[string]int), fixed arrays ([10]string)
	// Note: Requires valid YAML tag (at least one char after !!)
	re1 := regexp.MustCompile(`cannot\s+unmarshal\s+!!(\w+)\s+into\s+((?:chan|chan<-|<-chan)\s+[\w\-*]+|interface\{\}|[\[\]\*\w{}]+(?:\.[\w\-*]+)*)`)
	if matches := re1.FindStringSubmatch(errorStr); matches != nil {
		// Trim any trailing punctuation
		typeName := strings.TrimRight(matches[2], ".,")
		return typeName
	}

	// Pattern 2: "expected <type>, got <type>"
	re2 := regexp.MustCompile(`\bexpected\s+([^,\s]+(?:\s+[\w\-*]+)*(?:\.[\w\-*]+)*),\s*got\s+\S+`)
	if matches := re2.FindStringSubmatch(errorStr); matches != nil {
		return strings.TrimSpace(matches[1])
	}

	// Pattern 3: "want <type>, got <type>"
	re3 := regexp.MustCompile(`\bwant\s+([^,\s]+(?:\s+[\w\-*]+)*(?:\.[\w\-*]+)*),\s*got\s+\S+`)
	if matches := re3.FindStringSubmatch(errorStr); matches != nil {
		return strings.TrimSpace(matches[1])
	}

	// Pattern 4: "<type> cannot be converted to <type>"
	re4 := regexp.MustCompile(`^(\S+)\s+cannot\s+be\s+converted\s+to\s+\S+`)
	if matches := re4.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 5: Simple type name at start of error: "<type>: error message"
	// Make sure it doesn't match prefixes like "yaml:", "field:", "error:", "warning:", "fatal:", "panic:", or "line:"
	// These are common error message prefixes, not type names
	re5 := regexp.MustCompile(`^([a-z][A-Za-z0-9]*(?:\.[A-Za-z0-9]+)*):`)
	if matches := re5.FindStringSubmatch(errorStr); matches != nil {
		typeName := matches[1]
		// Filter out common error message prefixes that aren't type names
		excludedPrefixes := []string{"yaml", "field", "error", "warning", "fatal", "panic", "line"}
		for _, prefix := range excludedPrefixes {
			if typeName == prefix {
				return ""
			}
		}
		return typeName
	}

	// Pattern 6: Quoted type name "cannot unmarshal "<type>" into "<type>""
	re6 := regexp.MustCompile(`cannot\s+unmarshal\s+"([^"]+)"\s+into\s+"[^"]+"`)
	if matches := re6.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 7: Type name after "into": "...into <type>" (handles complex types)
	// Must be preceded by unmarshal or similar context to avoid matching common English phrases
	re7 := regexp.MustCompile(`(?:unmarshal|marshal|convert)\s+(?:[\w\s]+)?\s+into\s+((?:chan|chan<-|<-chan)\s+[\w\-*]+|interface\{\}|[\[\]\*\w{}]+(?:\.[\w\-*]+)*)`)
	if matches := re7.FindStringSubmatch(errorStr); matches != nil {
		typeName := strings.TrimRight(matches[1], ".,")
		return typeName
	}

	// Pattern 8: Type name at end after "expected": "...invalid type, expected <type>"
	// Handles: basic types, complex types (map, slice, pointer, array, channel), package-qualified types
	// Matches patterns like:
	// - "expected int", "expected string", "expected bool"
	// - "expected []string", "expected *int", "expected map[string]int"
	// - "expected [10]int", "expected chan string", "expected time.Time"
	// - "expected interface{}", "expected struct{}"
	// Note: More restrictive to avoid matching common English words
	re8 := regexp.MustCompile(`\bexpected\s+((?:(?:chan|chan<-|<-chan)\s+)?map\[[^\]]+\][\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?\[[\d\]]*[\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?\*[\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?interface\{\}|(?:(?:chan|chan<-|<-chan)\s+)?struct\{\}|(?:(?:chan|chan<-|<-chan)\s+)?[\w\-*]+\.[\w\-*]+|(?:chan|chan<-|<-chan)\s+[\w\-*]+|(?:(?:\*|\[)?(?:string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|float32|float64|bool|time\.Time|rune|byte)))`)
	if matches := re8.FindStringSubmatch(errorStr); matches != nil {
		typeName := strings.TrimRight(matches[1], ".,")
		return typeName
	}

	// Pattern 9: Type name at end after "want": "...error, want <type>"
	// Handles: basic types, complex types (map, slice, pointer, array, channel), package-qualified types
	// Matches patterns like:
	// - "want int", "want string", "want bool"
	// - "want []string", "want *int", "want map[string]int"
	// - "want [10]int", "want chan string", "want time.Time"
	// - "want interface{}", "want struct{}"
	// Note: More restrictive to avoid matching common English words
	re9 := regexp.MustCompile(`\bwant\s+((?:(?:chan|chan<-|<-chan)\s+)?map\[[^\]]+\][\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?\[[\d\]]*[\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?\*[\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?interface\{\}|(?:(?:chan|chan<-|<-chan)\s+)?struct\{\}|(?:(?:chan|chan<-|<-chan)\s+)?[\w\-*]+\.[\w\-*]+|(?:chan|chan<-|<-chan)\s+[\w\-*]+|(?:(?:\*|\[)?(?:string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|float32|float64|bool|time\.Time|rune|byte)))`)
	if matches := re9.FindStringSubmatch(errorStr); matches != nil {
		typeName := strings.TrimRight(matches[1], ".,")
		return typeName
	}

	// Pattern 10: Type name at end after "got" (extract actual type): "...error, got <type>"
	// Handles: basic types, complex types (map, slice, pointer, array, channel), package-qualified types
	// Note: More restrictive to avoid matching common English words
	re10 := regexp.MustCompile(`\bgot\s+((?:(?:chan|chan<-|<-chan)\s+)?map\[[^\]]+\][\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?\[[\d\]]*[\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?\*[\w\-*]+|(?:(?:chan|chan<-|<-chan)\s+)?interface\{\}|(?:(?:chan|chan<-|<-chan)\s+)?struct\{\}|(?:(?:chan|chan<-|<-chan)\s+)?[\w\-*]+\.[\w\-*]+|(?:chan|chan<-|<-chan)\s+[\w\-*]+|(?:(?:\*|\[)?(?:string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|float32|float64|bool|time\.Time|rune|byte)))`)
	if matches := re10.FindStringSubmatch(errorStr); matches != nil {
		typeName := strings.TrimRight(matches[1], ".,")
		return typeName
	}

	// Pattern 11: Type name at end in "type <type>": "...error type <type>"
	// This pattern is very restrictive to avoid matching regular words
	// Must start with special char ([, ], *, {, }) or be a known basic type
	re11 := regexp.MustCompile(`\btype\s+((?:chan|chan<-|<-chan)\s+[\w\-*]+|map\[[^\]]+\][\w\-*]+|interface\{\}|struct\{\}|[\[\]\*{}]+[\w\-*]*(?:\.[\w\-*]+)*|(?:string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|float32|float64|bool|rune|byte|interface|struct)(?:\s*[,\.\s]*|$))`)
	if matches := re11.FindStringSubmatch(errorStr); matches != nil {
		typeName := strings.TrimRight(matches[1], ".,")
		return typeName
	}

	// Pattern 12: Type name after "into" as fallback: "...into <type>"
	// This is a permissive fallback pattern that matches "into <type>" at the end
	// Used when more specific patterns don't match but "into" suggests type information
	re12 := regexp.MustCompile(`\binto\s+([\w\-*]+(?:\.[\w\-*]+)*)\b`)
	if matches := re12.FindStringSubmatch(errorStr); matches != nil {
		typeName := strings.TrimRight(matches[1], ".,")
		return typeName
	}

	return ""
}

func parseTypeErrorString(errorStr string) EnhancedTypeErrorDetail {
	detail := EnhancedTypeErrorDetail{
		RawError:    errorStr,
		LineNumber:  extractLineNumber(errorStr),
		ColumnNumber: extractColumnNumber(errorStr),
		FieldPath:   extractFieldPath(errorStr),
		Value:       extractValue(errorStr),
	}

	// Extract type mismatch information
	fieldPath, expectedType, actualType := extractTypeMismatchInfo(errorStr)
	if fieldPath != "" && detail.FieldPath == "" {
		detail.FieldPath = fieldPath
	}
	detail.Expected = expectedType
	detail.Actual = actualType

	// Build context message
	detail.Context = buildErrorContext(detail, errorStr)

	return detail
}

// extractTypeMismatchInfo extracts field path, expected type, and actual type from error message.
//
// Handles formats like:
// - "field server.port type mismatch: expected int, got string"
// - "cannot unmarshal !!str into int"
// - "expected bool, got string"
// - "want float64, got int"
//
// Returns (fieldPath, expectedType, actualType).
func extractTypeMismatchInfo(errorStr string) (fieldPath, expectedType, actualType string) {
	errorStr = strings.TrimSpace(errorStr)

	// Pattern 1: "field <path> type mismatch: expected <type>, got <type>"
	re1 := regexp.MustCompile(`field\s+([^\s]+(?:\[[^\]]+\])?(?:\.[^\s]+(?:\[[^\]]+\])?)*)\s+type\s+mismatch:\s*expected\s+([^\s,]+),\s*got\s+([^\s.]+)`)
	if matches := re1.FindStringSubmatch(errorStr); matches != nil {
		return matches[1], normalizeYAMLType(matches[2]), normalizeYAMLType(matches[3])
	}

	// Pattern 2: "field <path> cannot unmarshal !!<type> into <type>"
	re2 := regexp.MustCompile(`field\s+([^\s]+(?:\[[^\]]+\])?(?:\.[^\s]+(?:\[[^\]]+\])?)*)\s+cannot\s+unmarshal\s+!!(\w+)\s+into\s+(\S+)`)
	if matches := re2.FindStringSubmatch(errorStr); matches != nil {
		return matches[1], normalizeYAMLType(matches[3]), normalizeYAMLType(matches[2])
	}

	// Pattern 3: "cannot unmarshal !!<type> into <type>"
	re3 := regexp.MustCompile(`cannot\s+unmarshal\s+!!(\w+)\s+into\s+(\S+)`)
	if matches := re3.FindStringSubmatch(errorStr); matches != nil {
		return "", normalizeYAMLType(matches[2]), normalizeYAMLType(matches[1])
	}

	// Pattern 4: "expected <type>, got <type>"
	re4 := regexp.MustCompile(`expected\s+(\S+),\s*got\s+(\S+)`)
	if matches := re4.FindStringSubmatch(errorStr); matches != nil {
		return "", normalizeYAMLType(matches[1]), normalizeYAMLType(matches[2])
	}

	// Pattern 5: "want <type>, got <type>"
	re5 := regexp.MustCompile(`want\s+(\S+),\s*got\s+(\S+)`)
	if matches := re5.FindStringSubmatch(errorStr); matches != nil {
		return "", normalizeYAMLType(matches[1]), normalizeYAMLType(matches[2])
	}

	// Pattern 6: "<type> cannot be converted to <type>"
	re6 := regexp.MustCompile(`(\S+)\s+cannot\s+be\s+converted\s+to\s+(\S+)`)
	if matches := re6.FindStringSubmatch(errorStr); matches != nil {
		return "", normalizeYAMLType(matches[2]), normalizeYAMLType(matches[1])
	}

	// Pattern 7: "cannot unmarshal "<type>" into "<type>"" (quoted types)
	re7 := regexp.MustCompile(`cannot\s+unmarshal\s+"([^"]+)"\s+into\s+"([^"]+)"`)
	if matches := re7.FindStringSubmatch(errorStr); matches != nil {
		return "", normalizeYAMLType(matches[2]), normalizeYAMLType(matches[1])
	}

	return "", "", ""
}

// normalizeYAMLType normalizes a YAML/Go type string to a human-readable format.
//
// Handles:
// - YAML type tags: !!str, !!int, !!seq, !!map, !!bool, !!float, !!null
// - Go basic types: string, int, int8, int16, int32, int64, uint, float32, float64, bool
// - Go complex types: []T, *T, map[K]V, chan T, interface{}
// - Package-qualified types: time.Time, http.Response (strips package name)
//
// Examples:
//   - "!!str" → "string"
//   - "[]string" → "array of string"
//   - "*int" → "pointer to integer"
//   - "map[string]int" → "map[integer]int"
//   - "time.Time" → "Time"
func normalizeYAMLType(typeStr string) string {
	typeStr = strings.TrimSpace(typeStr)

	// Handle empty string
	if typeStr == "" {
		return ""
	}

	// YAML type tags (with !! prefix)
	yamlTypes := map[string]string{
		"!!str":   "string",
		"!!int":   "integer",
		"!!float": "float",
		"!!bool":  "boolean",
		"!!seq":   "array",
		"!!map":   "object",
		"!!null":  "null",
	}
	if normalized, ok := yamlTypes[typeStr]; ok {
		return normalized
	}

	// YAML type tags (without !! prefix - from regex extraction)
	yamlTypesNoPrefix := map[string]string{
		"str":   "string",
		"int":   "integer",
		"float": "float",
		"bool":  "boolean",
		"seq":   "array",
		"map":   "object",
		"null":  "null",
	}
	if normalized, ok := yamlTypesNoPrefix[typeStr]; ok {
		return normalized
	}

	// Pointer types
	if strings.HasPrefix(typeStr, "*") {
		if strings.HasPrefix(typeStr, "**") {
			// Double pointer - treat as single pointer
			return "pointer to " + normalizeYAMLType(typeStr[2:])
		}
		return "pointer to " + normalizeYAMLType(typeStr[1:])
	}

	// Array/slice types
	if strings.HasPrefix(typeStr, "[]") {
		elemType := normalizeYAMLType(typeStr[2:])
		return "array of " + elemType
	}

	// Fixed-size arrays
	reArray := regexp.MustCompile(`^\[(\d+)\](.+)$`)
	if matches := reArray.FindStringSubmatch(typeStr); matches != nil {
		elemType := normalizeYAMLType(matches[2])
		return "array of " + elemType
	}

	// Map types - normalize both key and value types
	reMap := regexp.MustCompile(`^map\[([^\]]+)\](.+)$`)
	if matches := reMap.FindStringSubmatch(typeStr); matches != nil {
		// Normalize both key and value types
		return "map[" + normalizeYAMLType(matches[1]) + "]" + normalizeYAMLType(matches[2])
	}

	// Channel types - check directional channels first
	if strings.HasPrefix(typeStr, "<-chan") {
		elemType := normalizeYAMLType(typeStr[7:])
		return "receive-only channel of " + elemType
	}
	if strings.HasPrefix(typeStr, "chan<-") {
		elemType := normalizeYAMLType(typeStr[6:])
		return "send-only channel of " + elemType
	}
	if strings.HasPrefix(typeStr, "chan") {
		elemType := normalizeYAMLType(typeStr[4:])
		return "channel of " + elemType
	}

	// Interface types
	if typeStr == "interface{}" {
		return "interface"
	}

	// Package-qualified types (e.g., time.Time, http.Response, encoding/json.Marshaler)
	rePkg := regexp.MustCompile(`^[a-z0-9_/]+\.([A-Z][a-zA-Z0-9]*)$`)
	if matches := rePkg.FindStringSubmatch(typeStr); matches != nil {
		return matches[1]
	}

	// Go basic types - normalize integers
	intTypes := map[string]string{
		"int":        "integer",
		"int8":       "integer",
		"int16":      "integer",
		"int32":      "integer",
		"int64":      "integer",
		"uint":       "integer",
		"uint8":      "unsigned integer",
		"uint16":     "unsigned integer",
		"uint32":     "unsigned integer",
		"uint64":     "unsigned integer",
		"uintptr":    "unsigned integer",
		"float32":    "float",
		"float64":    "float",
		"bool":       "boolean",
		"string":     "string",
		"rune":       "integer",
		"byte":       "unsigned integer",
		"complex64":  "complex number",
		"complex128": "complex number",
		"struct":     "object",
	}
	if normalized, ok := intTypes[typeStr]; ok {
		return normalized
	}

	// Return as-is for unknown types (e.g., custom struct names)
	return typeStr
}

// extractLineNumber extracts the line number from an error message.
//
// Handles formats:
// - "line 10: error"
// - "yaml: line 15: error"
// - "error at line 25: ..."
// - "10: error"
// - "error converting YAML to JSON: yaml: line 30: error"
// - "line 10, column 5: error"
//
// Returns 0 if no line number is found.
func extractLineNumber(errorStr string) int {
	// Pattern 1: "line <number>:"
	re1 := regexp.MustCompile(`\bline\s+(\d+):`)
	if matches := re1.FindStringSubmatch(errorStr); matches != nil {
		return parseIntSafe(matches[1])
	}

	// Pattern 2: "line <number>, column <number>"
	re2 := regexp.MustCompile(`\bline\s+(\d+),\s+column\s+\d+`)
	if matches := re2.FindStringSubmatch(errorStr); matches != nil {
		return parseIntSafe(matches[1])
	}

	// Pattern 3: "at line <number>"
	re3 := regexp.MustCompile(`\bat\s+line\s+(\d+)`)
	if matches := re3.FindStringSubmatch(errorStr); matches != nil {
		return parseIntSafe(matches[1])
	}

	// Pattern 4: "<number>:" at start (after optional prefix)
	re4 := regexp.MustCompile(`^(\w+:\s+)?(\d+):`)
	if matches := re4.FindStringSubmatch(errorStr); matches != nil && len(matches) > 2 {
		return parseIntSafe(matches[2])
	}

	return 0
}

// extractColumnNumber extracts the column number from an error message.
//
// Handles formats:
// - "line 10, column 5: error"
// - "line 10:5: error"
//
// Returns 0 if no column number is found.
func extractColumnNumber(errorStr string) int {
	// Pattern 1: "line <number>, column <number>"
	re1 := regexp.MustCompile(`\bline\s+\d+,\s+column\s+(\d+)`)
	if matches := re1.FindStringSubmatch(errorStr); matches != nil {
		return parseIntSafe(matches[1])
	}

	// Pattern 2: "line <number>:<number>" (note: this might conflict with line:column format)
	re2 := regexp.MustCompile(`\bline\s+(\d+):(\d+):`)
	if matches := re2.FindStringSubmatch(errorStr); matches != nil {
		return parseIntSafe(matches[2])
	}

	return 0
}

// extractFieldPath extracts the field path from an error message.
//
// Handles formats:
// - "field server.port type mismatch"
// - "field items[0].name cannot unmarshal"
// - "at field server.name"
// - "in field config.key field"
//
// Returns empty string if no field path is found.
func extractFieldPath(errorStr string) string {
	// Pattern 1: "field <path> type mismatch"
	re1 := regexp.MustCompile(`\bfield\s+([^\s]+(?:\[[^\]]+\])?(?:\.[^\s]+(?:\[[^\]]+\])?)*)\s+type\s+mismatch`)
	if matches := re1.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 2: "field <path> cannot"
	re2 := regexp.MustCompile(`\bfield\s+([^\s]+(?:\[[^\]]+\])?(?:\.[^\s]+(?:\[[^\]]+\])?)*)\s+cannot`)
	if matches := re2.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 3: "at field <path>"
	re3 := regexp.MustCompile(`\bat\s+field\s+([^\s]+(?:\[[^\]]+\])?(?:\.[^\s]+(?:\[[^\]]+\])?)*)`)
	if matches := re3.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 4: "in field <path> field"
	re4 := regexp.MustCompile(`\bin\s+field\s+([^\s]+(?:\[[^\]]+\])?(?:\.[^\s]+(?:\[[^\]]+\])?)*)\s+field`)
	if matches := re4.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	return ""
}

// extractValue extracts the actual value from an error message.
//
// Handles formats:
// - "cannot unmarshal !!str `hello` into int"
// - "cannot unmarshal 'world' into int"
// - "cannot unmarshal "test" into int"
// - "value: '123' is invalid"
// - "actual value: true"
//
// Returns empty string if no value is found.
func extractValue(errorStr string) string {
	// Pattern 1: Backtick-enclosed value (e.g., "cannot unmarshal !!str `hello` into int")
	// We need to construct this pattern without using backticks in raw strings
	backtickPattern := regexp.MustCompile("!!\\w+\\s+`([^`]+)`\\s+into")
	if matches := backtickPattern.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Fallback pattern for backticks without type tag
	backtickFallback := regexp.MustCompile("`([^`]+)`\\s+into")
	if matches := backtickFallback.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 2: Single-quoted value
	re2 := regexp.MustCompile(`(?:unmarshal|marshal)\s+'([^']+)'\s+into`)
	if matches := re2.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 3: Double-quoted value
	re3 := regexp.MustCompile(`(?:unmarshal|marshal)\s+"([^"]+)"\s+into`)
	if matches := re3.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 4: "value: '<value>'"
	re4 := regexp.MustCompile(`value:\s*['"]([^'"]+)['"]`)
	if matches := re4.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	// Pattern 5: "actual value: <value>"
	re5 := regexp.MustCompile(`actual\s+value:\s*(\S+)`)
	if matches := re5.FindStringSubmatch(errorStr); matches != nil {
		return matches[1]
	}

	return ""
}

// inferTypeFromValue infers the type from a string value.
//
// Handles:
// - Booleans: "true", "false"
// - Numbers: integers and floats
// - Strings: quoted or unquoted
//
// Returns "unknown" if the type cannot be inferred.
func inferTypeFromValue(value string) string {
	value = strings.TrimSpace(value)

	// Empty value
	if value == "" {
		return "unknown"
	}

	// Boolean
	if value == "true" || value == "false" {
		return "boolean"
	}

	// Number (integer or float)
	re := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	if re.MatchString(value) {
		return "number"
	}

	// Quoted string (without quotes)
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return "string"
	}
	if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
		return "string"
	}

	// Default to string for anything else
	return "string"
}

// buildErrorContext builds a human-readable error context message.
//
// Generates messages like:
// - "Unable to convert field 'server.port': expected integer, got string (value: 'abc')"
// - "Type mismatch: expected boolean, got string"
// - "Field 'items[0]' type mismatch: expected string, got integer"
func buildErrorContext(detail EnhancedTypeErrorDetail, rawError string) string {
	var parts []string

	if detail.FieldPath != "" {
		parts = append(parts, fmt.Sprintf("field '%s'", detail.FieldPath))
	}

	if detail.Expected != "" && detail.Actual != "" {
		parts = append(parts, fmt.Sprintf("expected %s, got %s", detail.Expected, detail.Actual))
	}

	if detail.Value != "" {
		parts = append(parts, fmt.Sprintf("value: '%s'", detail.Value))
	}

	if len(parts) == 0 {
		return "Unable to convert value: " + rawError
	}

	if detail.FieldPath != "" {
		return "Unable to convert " + strings.Join(parts, ": ")
	}
	return "Type mismatch: " + strings.Join(parts, ", ")
}

// parseIntSafe parses an integer string and returns 0 on failure.
func parseIntSafe(s string) int {
	var result int
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			result = result*10 + int(ch-'0')
		} else {
			return 0
		}
	}
	return result
}

// ExtractTypeNameBasic extracts the first type name from a yaml.TypeError message string.
//
// This is a basic implementation that handles simple type name formats at the beginning
// of error messages. It serves as the foundation for type name extraction.
//
// Supported patterns:
// - YAML type tags: !!str, !!int, !!seq, !!map, !!bool, !!float, !!null
// - Go basic types: string, int, int8, int16, int32, int64, uint, float32, float64, bool
// - Go slice types: []string, []int, etc.
// - Go pointer types: *string, *int, etc.
// - Go map types: map[string]int, etc.
//
// The function searches the error message and returns the first matched type name,
// or an empty string if no match is found.
//
// Examples:
// - "line 10: cannot unmarshal !!str into int" → returns "str"
// - "cannot unmarshal !!int into string" → returns "int"
// - "expected []string, got int" → returns "[]string"
// - "field type: *bool expected" → returns "*bool"
// - "map cannot unmarshal" → returns ""
//
// Returns empty string when no match is found.
func ExtractTypeNameBasic(errorMsg string) string {
	errorMsg = strings.TrimSpace(errorMsg)
	if errorMsg == "" {
		return ""
	}

	// Pattern 1: YAML type tags (e.g., !!str, !!int, !!seq)
	// These are the most common and should be checked first
	yamlTypeTagRegex := regexp.MustCompile(`!!(\w+)`)
	if matches := yamlTypeTagRegex.FindStringSubmatch(errorMsg); matches != nil {
		return matches[1]
	}

	// Pattern 2: Go slice types (e.g., []string, []int, []map[string]int)
	sliceTypeRegex := regexp.MustCompile(`\[\](\w+(?:\.\w+)*)`)
	if matches := sliceTypeRegex.FindStringSubmatch(errorMsg); matches != nil {
		return "[]" + matches[1]
	}

	// Pattern 3: Go pointer types (e.g., *string, *int, *bool)
	pointerTypeRegex := regexp.MustCompile(`\*(\w+(?:\.\w+)*)`)
	if matches := pointerTypeRegex.FindStringSubmatch(errorMsg); matches != nil {
		return "*" + matches[1]
	}

	// Pattern 4: Go map types (e.g., map[string]int, map[int]string)
	mapTypeRegex := regexp.MustCompile(`map\[([^\]]+)\](\w+(?:\.\w+)*)`)
	if matches := mapTypeRegex.FindStringSubmatch(errorMsg); matches != nil {
		return "map[" + matches[1] + "]" + matches[2]
	}

	// Pattern 5: Go basic types (e.g., string, int, bool, float64)
	// This pattern matches common Go type names at word boundaries
	// IMPORTANT: Must match complete words only, not substrings (e.g., "int" in "internal" should NOT match)
	// Uses trailing \b to ensure complete word matches
	// IMPORTANT: Longer types must come before shorter ones to avoid partial matches
	// (e.g., "interface" before "int", "string" before nothing, etc.)
	basicTypeRegex := regexp.MustCompile(`(interface|string|int8|int16|int32|int64|uint8|uint16|uint32|uint64|float32|float64|int|uint|bool|rune|byte)\b`)
	if matches := basicTypeRegex.FindStringSubmatch(errorMsg); matches != nil {
		return matches[1]
	}

	// Pattern 5a: Special case for interface{} which needs exact match
	interfaceRegex := regexp.MustCompile(`\b(interface\{\})`)
	if matches := interfaceRegex.FindStringSubmatch(errorMsg); matches != nil {
		return matches[1]
	}

	// Return empty string when no match is found
	return ""
}
