package yamlutil

// Test fixtures for yaml.TypeError message format parsing and testing.
//
// This file contains representative error message samples that demonstrate
// the various formats produced by gopkg.in/yaml.v3's TypeError.
// Use these fixtures for testing error parsing, formatting, and analysis.

import "gopkg.in/yaml.v3"

// ================================================================
// TypeError Message Format Fixtures
// ================================================================

// Basic line-based error format fixtures
var (
	// TypeErrorBasicFormat1 represents the simplest error format
	// Format: "line <number>: cannot unmarshal <actual> into <expected>"
	TypeErrorBasicFormat1 = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorBasicFormat2 shows sequence to array error
	TypeErrorBasicFormat2 = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!seq into []string",
		},
	}

	// TypeErrorBasicFormat3 shows map to struct error
	TypeErrorBasicFormat3 = &yaml.TypeError{
		Errors: []string{
			"line 20: cannot unmarshal !!map into struct",
		},
	}

	// TypeErrorBasicFormat4 shows bool to string error
	TypeErrorBasicFormat4 = &yaml.TypeError{
		Errors: []string{
			"line 25: cannot unmarshal !!bool into string",
		},
	}

	// TypeErrorBasicFormat5 shows float to int error
	TypeErrorBasicFormat5 = &yaml.TypeError{
		Errors: []string{
			"line 30: cannot unmarshal !!float into int",
		},
	}
)

// YAML-prefixed error format fixtures
var (
	// TypeErrorYAMLPrefixed1 shows standard YAML prefix format
	TypeErrorYAMLPrefixed1 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 10: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorYAMLPrefixed2 shows sequence error with prefix
	TypeErrorYAMLPrefixed2 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 15: cannot unmarshal !!seq into []string",
		},
	}

	// TypeErrorYAMLPrefixed3 shows map error with prefix
	TypeErrorYAMLPrefixed3 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 20: cannot unmarshal !!map into struct",
		},
	}
)

// Line and column error format fixtures
var (
	// TypeErrorLineColumn1 shows precise location with column
	TypeErrorLineColumn1 = &yaml.TypeError{
		Errors: []string{
			"line 10, column 5: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorLineColumn2 shows different column location
	TypeErrorLineColumn2 = &yaml.TypeError{
		Errors: []string{
			"line 15, column 12: cannot unmarshal !!bool into string",
		},
	}

	// TypeErrorLineColumn3 shows sequence with column info
	TypeErrorLineColumn3 = &yaml.TypeError{
		Errors: []string{
			"line 20, column 8: cannot unmarshal !!seq into []int",
		},
	}
)

// Error context prefix format fixtures
var (
	// TypeErrorErrorAt1 shows "error at line" format
	TypeErrorErrorAt1 = &yaml.TypeError{
		Errors: []string{
			"error at line 25: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorErrorAt2 shows error at format with sequence
	TypeErrorErrorAt2 = &yaml.TypeError{
		Errors: []string{
			"error at line 30: cannot unmarshal !!seq into []string",
		},
	}
)

// Nested context error format fixtures
var (
	// TypeErrorNestedContext1 shows YAML to JSON conversion error
	TypeErrorNestedContext1 = &yaml.TypeError{
		Errors: []string{
			"error converting YAML to JSON: yaml: line 30: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorNestedContext2 shows complex nested context
	TypeErrorNestedContext2 = &yaml.TypeError{
		Errors: []string{
			"error converting YAML to JSON: yaml: line 45: cannot unmarshal !!map into string",
		},
	}
)

// Value-specific error format fixtures
var (
	// TypeErrorWithValue1 shows backtick-quoted value
	TypeErrorWithValue1 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 10: cannot unmarshal !!str `hello` into int",
		},
	}

	// TypeErrorWithValue2 shows different backtick value
	TypeErrorWithValue2 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 15: cannot unmarshal !!str `world` into bool",
		},
	}

	// TypeErrorWithValue3 shows numeric value
	TypeErrorWithValue3 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 20: cannot unmarshal !!str `123` into bool",
		},
	}

	// TypeErrorWithValue4 shows value with quotes
	TypeErrorWithValue4 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 25: cannot unmarshal !!str \"quoted\" into int",
		},
	}
)

// Multi-error format fixtures
var (
	// TypeErrorMultiple1 shows multiple type errors in one TypeError
	TypeErrorMultiple1 = &yaml.TypeError{
		Errors: []string{
			"line 5: cannot unmarshal !!seq into []string",
			"line 10: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorMultiple2 shows three errors
	TypeErrorMultiple2 = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!bool into string",
			"line 20: cannot unmarshal !!map into int",
			"line 25: cannot unmarshal !!str into []int",
		},
	}

	// TypeErrorMultiple3 shows complex multi-error scenario
	TypeErrorMultiple3 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 5: cannot unmarshal !!seq into []string",
			"yaml: line 10: cannot unmarshal !!str into int",
			"yaml: line 15: cannot unmarshal !!map into struct",
		},
	}

	// TypeErrorMultiple4 shows mixed format multi-error
	TypeErrorMultiple4 = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!str into int",
			"yaml: line 15: cannot unmarshal !!seq into []string",
			"line 20, column 5: cannot unmarshal !!bool into float64",
		},
	}
)

// ================================================================
// Complex Type Error Fixtures
// ================================================================

// Complex Go type error fixtures
var (
	// TypeErrorComplexArray1 shows array of arrays error
	TypeErrorComplexArray1 = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!seq into [][]string",
		},
	}

	// TypeErrorComplexArray2 shows 2D array error
	TypeErrorComplexArray2 = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!map into []int",
		},
	}

	// TypeErrorComplexMap1 shows complex map type error
	TypeErrorComplexMap1 = &yaml.TypeError{
		Errors: []string{
			"line 20: cannot unmarshal !!str into map[string]int",
		},
	}

	// TypeErrorComplexMap2 shows map with interface value
	TypeErrorComplexMap2 = &yaml.TypeError{
		Errors: []string{
			"line 25: cannot unmarshal !!seq into map[string]interface{}",
		},
	}

	// TypeErrorComplexPointer1 shows pointer type error
	TypeErrorComplexPointer1 = &yaml.TypeError{
		Errors: []string{
			"line 30: cannot unmarshal !!str into *string",
		},
	}

	// TypeErrorComplexPointer2 shows double pointer error
	TypeErrorComplexPointer2 = &yaml.TypeError{
		Errors: []string{
			"line 35: cannot unmarshal !!str into **int",
		},
	}

	// TypeErrorComplexInterface1 shows interface type error
	TypeErrorComplexInterface1 = &yaml.TypeError{
		Errors: []string{
			"line 40: cannot unmarshal !!str into interface{}",
		},
	}

	// TypeErrorComplexInterface2 shows named interface error
	TypeErrorComplexInterface2 = &yaml.TypeError{
		Errors: []string{
			"line 45: cannot unmarshal !!map into io.Reader",
		},
	}
)

// ================================================================
// Edge Case Fixtures
// ================================================================

// Edge case and special scenario fixtures
var (
	// TypeErrorEdgeCase1 shows null value error
	TypeErrorEdgeCase1 = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!null into string",
		},
	}

	// TypeErrorEdgeCase2 shows timestamp error
	TypeErrorEdgeCase2 = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!timestamp into int",
		},
	}

	// TypeErrorEdgeCase3 shows empty string error
	TypeErrorEdgeCase3 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 20: cannot unmarshal !!str `` into int",
		},
	}

	// TypeErrorEdgeCase4 shows special characters in value
	TypeErrorEdgeCase4 = &yaml.TypeError{
		Errors: []string{
			"yaml: line 25: cannot unmarshal !!str `hello-world` into int",
		},
	}

	// TypeErrorEdgeCase5 shows numeric string error
	TypeErrorEdgeCase5 = &yaml.TypeError{
		Errors: []string{
			"line 30: cannot unmarshal !!str `3.14` into int",
		},
	}

	// TypeErrorEdgeCase6 shows boolean string error
	TypeErrorEdgeCase6 = &yaml.TypeError{
		Errors: []string{
			"line 35: cannot unmarshal !!str `true` into bool",
		},
	}

	// TypeErrorEdgeCase7 shows oversized integer error
	TypeErrorEdgeCase7 = &yaml.TypeError{
		Errors: []string{
			"line 40: cannot unmarshal !!str `999999999999999999999` into int64",
		},
	}

	// TypeErrorEdgeCase8 shows negative number string error
	TypeErrorEdgeCase8 = &yaml.TypeError{
		Errors: []string{
			"line 45: cannot unmarshal !!str `-123` into uint",
		},
	}
)

// ================================================================
// Field Path Error Fixtures
// ================================================================

// Error messages with field path context
var (
	// Note: These are representative patterns. Actual field path errors
	// may come from different sources or have additional context.

	// TypeErrorFieldPath1 shows nested field error pattern
	TypeErrorFieldPath1 = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!str into int",
		},
	}
	// Context: This might occur at field "server.port" where port should be int

	// TypeErrorFieldPath2 shows array indexing error pattern
	TypeErrorFieldPath2 = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!str into int",
		},
	}
	// Context: This might occur at field "items[0].id" where id should be int

	// TypeErrorFieldPath3 shows deep nesting error pattern
	TypeErrorFieldPath3 = &yaml.TypeError{
		Errors: []string{
			"line 20: cannot unmarshal !!map into []string",
		},
	}
	// Context: This might occur at field "data.tags[0]" where tags should be array
)

// ================================================================
// Type Tag Coverage Fixtures
// ================================================================

// Fixtures covering all YAML type tags
var (
	// TypeErrorTagStr shows !!str tag error
	TypeErrorTagStr = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorTagInt shows !!int tag error
	TypeErrorTagInt = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!int into string",
		},
	}

	// TypeErrorTagFloat shows !!float tag error
	TypeErrorTagFloat = &yaml.TypeError{
		Errors: []string{
			"line 20: cannot unmarshal !!float into int",
		},
	}

	// TypeErrorTagBool shows !!bool tag error
	TypeErrorTagBool = &yaml.TypeError{
		Errors: []string{
			"line 25: cannot unmarshal !!bool into string",
		},
	}

	// TypeErrorTagSeq shows !!seq tag error
	TypeErrorTagSeq = &yaml.TypeError{
		Errors: []string{
			"line 30: cannot unmarshal !!seq into string",
		},
	}

	// TypeErrorTagMap shows !!map tag error
	TypeErrorTagMap = &yaml.TypeError{
		Errors: []string{
			"line 35: cannot unmarshal !!map into []string",
		},
	}

	// TypeErrorTagNull shows !!null tag error
	TypeErrorTagNull = &yaml.TypeError{
		Errors: []string{
			"line 40: cannot unmarshal !!null into string",
		},
	}

	// TypeErrorTagTimestamp shows !!timestamp tag error
	TypeErrorTagTimestamp = &yaml.TypeError{
		Errors: []string{
			"line 45: cannot unmarshal !!timestamp into string",
		},
	}
)

// ================================================================
// Go Type Coverage Fixtures
// ================================================================

// Fixtures covering common Go target types
var (
	// TypeErrorGoTypeString shows string target type
	TypeErrorGoTypeString = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!int into string",
		},
	}

	// TypeErrorGoTypeInt shows int target type
	TypeErrorGoTypeInt = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorGoTypeInt8 shows int8 target type
	TypeErrorGoTypeInt8 = &yaml.TypeError{
		Errors: []string{
			"line 20: cannot unmarshal !!str into int8",
		},
	}

	// TypeErrorGoTypeInt16 shows int16 target type
	TypeErrorGoTypeInt16 = &yaml.TypeError{
		Errors: []string{
			"line 25: cannot unmarshal !!str into int16",
		},
	}

	// TypeErrorGoTypeInt32 shows int32 target type
	TypeErrorGoTypeInt32 = &yaml.TypeError{
		Errors: []string{
			"line 30: cannot unmarshal !!str into int32",
		},
	}

	// TypeErrorGoTypeInt64 shows int64 target type
	TypeErrorGoTypeInt64 = &yaml.TypeError{
		Errors: []string{
			"line 35: cannot unmarshal !!str into int64",
		},
	}

	// TypeErrorGoTypeUint shows uint target type
	TypeErrorGoTypeUint = &yaml.TypeError{
		Errors: []string{
			"line 40: cannot unmarshal !!str into uint",
		},
	}

	// TypeErrorGoTypeFloat32 shows float32 target type
	TypeErrorGoTypeFloat32 = &yaml.TypeError{
		Errors: []string{
			"line 45: cannot unmarshal !!str into float32",
		},
	}

	// TypeErrorGoTypeFloat64 shows float64 target type
	TypeErrorGoTypeFloat64 = &yaml.TypeError{
		Errors: []string{
			"line 50: cannot unmarshal !!bool into float64",
		},
	}

	// TypeErrorGoTypeBool shows bool target type
	TypeErrorGoTypeBool = &yaml.TypeError{
		Errors: []string{
			"line 55: cannot unmarshal !!str into bool",
		},
	}

	// TypeErrorGoTypeSlice shows slice target type
	TypeErrorGoTypeSlice = &yaml.TypeError{
		Errors: []string{
			"line 60: cannot unmarshal !!str into []string",
		},
	}

	// TypeErrorGoTypeArray shows array target type
	TypeErrorGoTypeArray = &yaml.TypeError{
		Errors: []string{
			"line 65: cannot unmarshal !!seq into [5]int",
		},
	}

	// TypeErrorGoTypeMap shows map target type
	TypeErrorGoTypeMap = &yaml.TypeError{
		Errors: []string{
			"line 70: cannot unmarshal !!seq into map[string]string",
		},
	}
)

// ================================================================
// Real-World Scenario Fixtures
// ================================================================

// Fixtures representing common real-world YAML configuration errors
var (
	// TypeErrorServerPort represents a common config error:
	// server:
	//   port: "8080"  # Should be int, not string
	TypeErrorServerPort = &yaml.TypeError{
		Errors: []string{
			"yaml: line 10: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorDatabaseTimeout represents:
	// database:
	//   timeout: "30"  # Should be int
	TypeErrorDatabaseTimeout = &yaml.TypeError{
		Errors: []string{
			"line 15: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorDebugFlag represents:
	// debug: "true"  # Should be bool, not string
	TypeErrorDebugFlag = &yaml.TypeError{
		Errors: []string{
			"line 20: cannot unmarshal !!str into bool",
		},
	}

	// TypeErrorArrayField represents:
	// hosts: "localhost"  # Should be array
	TypeErrorArrayField = &yaml.TypeError{
		Errors: []string{
			"line 25: cannot unmarshal !!str into []string",
		},
	}

	// TypeErrorNestedConfig represents:
	// server:
	//   replicas: "high"  # Should be int
	TypeErrorNestedConfig = &yaml.TypeError{
		Errors: []string{
			"line 30: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorPortRange represents:
	// ports:
	//   - "8080"  # Should be int
	TypeErrorPortRange = &yaml.TypeError{
		Errors: []string{
			"line 35: cannot unmarshal !!str into int",
		},
	}

	// TypeErrorMultipleConfigErrors represents multiple config issues
	TypeErrorMultipleConfigErrors = &yaml.TypeError{
		Errors: []string{
			"line 10: cannot unmarshal !!str into int",
			"line 15: cannot unmarshal !!str into bool",
			"line 20: cannot unmarshal !!str into []string",
		},
	}
)

// ================================================================
// Fixture Collection Maps
// ================================================================

// AllTypeErrorFixtures collects all TypeError fixtures for comprehensive testing
var AllTypeErrorFixtures = []*yaml.TypeError{
	// Basic format fixtures
	TypeErrorBasicFormat1,
	TypeErrorBasicFormat2,
	TypeErrorBasicFormat3,
	TypeErrorBasicFormat4,
	TypeErrorBasicFormat5,

	// YAML-prefixed fixtures
	TypeErrorYAMLPrefixed1,
	TypeErrorYAMLPrefixed2,
	TypeErrorYAMLPrefixed3,

	// Line and column fixtures
	TypeErrorLineColumn1,
	TypeErrorLineColumn2,
	TypeErrorLineColumn3,

	// Error context fixtures
	TypeErrorErrorAt1,
	TypeErrorErrorAt2,

	// Nested context fixtures
	TypeErrorNestedContext1,
	TypeErrorNestedContext2,

	// Value-specific fixtures
	TypeErrorWithValue1,
	TypeErrorWithValue2,
	TypeErrorWithValue3,
	TypeErrorWithValue4,

	// Multi-error fixtures
	TypeErrorMultiple1,
	TypeErrorMultiple2,
	TypeErrorMultiple3,
	TypeErrorMultiple4,

	// Complex type fixtures
	TypeErrorComplexArray1,
	TypeErrorComplexArray2,
	TypeErrorComplexMap1,
	TypeErrorComplexMap2,
	TypeErrorComplexPointer1,
	TypeErrorComplexPointer2,
	TypeErrorComplexInterface1,
	TypeErrorComplexInterface2,

	// Edge case fixtures
	TypeErrorEdgeCase1,
	TypeErrorEdgeCase2,
	TypeErrorEdgeCase3,
	TypeErrorEdgeCase4,
	TypeErrorEdgeCase5,
	TypeErrorEdgeCase6,
	TypeErrorEdgeCase7,
	TypeErrorEdgeCase8,

	// Type tag fixtures
	TypeErrorTagStr,
	TypeErrorTagInt,
	TypeErrorTagFloat,
	TypeErrorTagBool,
	TypeErrorTagSeq,
	TypeErrorTagMap,
	TypeErrorTagNull,
	TypeErrorTagTimestamp,

	// Go type fixtures
	TypeErrorGoTypeString,
	TypeErrorGoTypeInt,
	TypeErrorGoTypeInt8,
	TypeErrorGoTypeInt16,
	TypeErrorGoTypeInt32,
	TypeErrorGoTypeInt64,
	TypeErrorGoTypeUint,
	TypeErrorGoTypeFloat32,
	TypeErrorGoTypeFloat64,
	TypeErrorGoTypeBool,
	TypeErrorGoTypeSlice,
	TypeErrorGoTypeArray,
	TypeErrorGoTypeMap,

	// Real-world scenario fixtures
	TypeErrorServerPort,
	TypeErrorDatabaseTimeout,
	TypeErrorDebugFlag,
	TypeErrorArrayField,
	TypeErrorNestedConfig,
	TypeErrorPortRange,
	TypeErrorMultipleConfigErrors,
}

// TypeErrorFixturesByFormat organizes fixtures by error message format pattern
var TypeErrorFixturesByFormat = map[string][]*yaml.TypeError{
	"basic_line": {
		TypeErrorBasicFormat1,
		TypeErrorBasicFormat2,
		TypeErrorBasicFormat3,
		TypeErrorBasicFormat4,
		TypeErrorBasicFormat5,
	},
	"yaml_prefixed": {
		TypeErrorYAMLPrefixed1,
		TypeErrorYAMLPrefixed2,
		TypeErrorYAMLPrefixed3,
	},
	"line_column": {
		TypeErrorLineColumn1,
		TypeErrorLineColumn2,
		TypeErrorLineColumn3,
	},
	"error_context": {
		TypeErrorErrorAt1,
		TypeErrorErrorAt2,
	},
	"nested_context": {
		TypeErrorNestedContext1,
		TypeErrorNestedContext2,
	},
	"with_value": {
		TypeErrorWithValue1,
		TypeErrorWithValue2,
		TypeErrorWithValue3,
		TypeErrorWithValue4,
	},
	"multi_error": {
		TypeErrorMultiple1,
		TypeErrorMultiple2,
		TypeErrorMultiple3,
		TypeErrorMultiple4,
	},
}

// TypeErrorFixturesByYAMLTag organizes fixtures by YAML type tag
var TypeErrorFixturesByYAMLTag = map[string][]*yaml.TypeError{
	"!!str": {
		TypeErrorBasicFormat1,
		TypeErrorTagStr,
		TypeErrorWithValue1,
	},
	"!!int": {
		TypeErrorTagInt,
		TypeErrorBasicFormat5,
	},
	"!!float": {
		TypeErrorTagFloat,
		TypeErrorGoTypeFloat64,
	},
	"!!bool": {
		TypeErrorTagBool,
		TypeErrorDebugFlag,
	},
	"!!seq": {
		TypeErrorBasicFormat2,
		TypeErrorTagSeq,
		TypeErrorArrayField,
	},
	"!!map": {
		TypeErrorBasicFormat3,
		TypeErrorTagMap,
		TypeErrorComplexMap1,
	},
	"!!null": {
		TypeErrorEdgeCase1,
		TypeErrorTagNull,
	},
	"!!timestamp": {
		TypeErrorTagTimestamp,
		TypeErrorEdgeCase2,
	},
}

// TypeErrorFixturesByGoType organizes fixtures by target Go type
var TypeErrorFixturesByGoType = map[string][]*yaml.TypeError{
	"int": {
		TypeErrorBasicFormat1,
		TypeErrorGoTypeInt,
		TypeErrorServerPort,
	},
	"string": {
		TypeErrorTagInt,
		TypeErrorGoTypeString,
	},
	"bool": {
		TypeErrorTagBool,
		TypeErrorGoTypeBool,
		TypeErrorDebugFlag,
	},
	"[]string": {
		TypeErrorBasicFormat2,
		TypeErrorGoTypeSlice,
		TypeErrorArrayField,
	},
	"struct": {
		TypeErrorBasicFormat3,
	},
	"float64": {
		TypeErrorTagFloat,
		TypeErrorGoTypeFloat64,
	},
	"interface{}": {
		TypeErrorComplexInterface1,
	},
	"map[string]int": {
		TypeErrorComplexMap1,
	},
}

// ================================================================
// Helper Functions for Fixtures
// ================================================================

// GetFixturesByFormat returns all fixtures matching a specific format pattern
func GetFixturesByFormat(format string) []*yaml.TypeError {
	return TypeErrorFixturesByFormat[format]
}

// GetFixturesByYAMLTag returns all fixtures with a specific YAML type tag
func GetFixturesByYAMLTag(tag string) []*yaml.TypeError {
	return TypeErrorFixturesByYAMLTag[tag]
}

// GetFixturesByGoType returns all fixtures targeting a specific Go type
func GetFixturesByGoType(goType string) []*yaml.TypeError {
	return TypeErrorFixturesByGoType[goType]
}

// GetAllFixtures returns the complete collection of TypeError fixtures
func GetAllFixtures() []*yaml.TypeError {
	return AllTypeErrorFixtures
}

// CountFixturesByFormat returns the number of fixtures for each format
func CountFixturesByFormat() map[string]int {
	counts := make(map[string]int)
	for format, fixtures := range TypeErrorFixturesByFormat {
		counts[format] = len(fixtures)
	}
	return counts
}

// CountFixturesByYAMLTag returns the number of fixtures for each YAML tag
func CountFixturesByYAMLTag() map[string]int {
	counts := make(map[string]int)
	for tag, fixtures := range TypeErrorFixturesByYAMLTag {
		counts[tag] = len(fixtures)
	}
	return counts
}
