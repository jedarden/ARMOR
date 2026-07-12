# YAMLUtil Coverage Gap Analysis

**Generated:** 2026-07-11
**Coverage File:** yamlutil_coverage.out
**Objective:** Identify coverage gaps in yamlutil functions and error handling paths

## Executive Summary

The yamlutil package has **106 functions with 0% coverage** and **38 functions below 80% coverage**. This analysis identifies specific gaps in error handling, parser variants, schema validation, and template processing features.

## Functions Below 80% Coverage

### Critical (0% Coverage)

#### Error Constructors - Untested
These constructor functions have **0% coverage** and represent critical error creation paths:

1. `NewValidationError` (errors.go:541) - Creates validation errors
2. `NewFileError` (errors.go:581) - Creates file I/O errors
3. `NewSchemaValidationError` (errors.go:591) - Creates schema validation errors
4. `NewTypeMismatchError` (errors.go:605) - Creates type mismatch errors
5. `NewSyntaxError` (errors.go:642) - Creates YAML syntax errors
6. `NewStructureError` (errors.go:655) - Creates YAML structure errors

#### Error Type Methods - Untested
Most error type implementation methods have **0% coverage**:

**ValidationError:**
- `Code()` - errors.go:762
- `YAMLErrorType()` - errors.go:770
- `Context()` - errors.go:775
- `Error()` - errors.go:784 (25% coverage)
- `Unwrap()` - errors.go:809
- `IsFileError()` - errors.go:814

**TypeMismatchError:**
- `Code()` - errors.go:848
- `Error()` - errors.go:857
- `Unwrap()` - errors.go:865 (50% coverage)

**ConstraintError:**
- `Code()` - errors.go:936
- `YAMLErrorType()` - errors.go:944
- `Context()` - errors.go:949
- `Error()` - errors.go:955

**FieldNotFoundError:**
- `Code()` - errors.go:1017
- `YAMLErrorType()` - errors.go:1025
- `Context()` - errors.go:1030
- `Error()` - errors.go:1036 (11.1% coverage)

**DuplicateKeyError:**
- `Code()` - errors.go:1161
- `YAMLErrorType()` - errors.go:1169
- `Context()` - errors.go:1174
- `Error()` - errors.go:1186

**SyntaxError:**
- `Code()` - errors.go:976
- `YAMLErrorType()` - errors.go:984
- `Context()` - errors.go:989
- `Error()` - errors.go:994

#### Error Predicate Functions - Untested
All error type checking functions have **0% coverage**:

1. `IsParseError` - errors.go:385
2. `IsValidationError` - errors.go:575
3. `IsYAMLError` - errors.go:1211
4. `GetYAMLErrorType` - errors.go:1222
5. `IsFileNotFoundError` - errors.go:1239
6. `IsPermissionError` - errors.go:1261
7. `IsIOError` - parse_error_design.go:317

#### Parser Variants - Completely Untested
All parser variant implementations have **0% coverage**:

**CachedParser:**
- `NewCachedParser` - interfaces.go:550
- `ParseFile` - interfaces.go:571
- `ParseFileToMap` - interfaces.go:600
- `ParseString` - interfaces.go:625
- `MustParseFile` - interfaces.go:630
- `Config` - interfaces.go:638
- `ClearCache` - interfaces.go:643
- `CacheSize` - interfaces.go:648
- `CacheStats` - interfaces.go:653
- `getFromCache` - interfaces.go:659
- `addToCache` - interfaces.go:678
- `evictLRU` - interfaces.go:693

**StreamingParser:**
- `NewStreamingParser` - interfaces.go:728
- `NewStreamingParserWithConfig` - interfaces.go:741
- `ParseFile` - interfaces.go:753
- `ParseFileToMap` - interfaces.go:774
- `ParseString` - interfaces.go:780
- `MustParseFile` - interfaces.go:786
- `Config` - interfaces.go:792
- `SetBufferSize` - interfaces.go:797

**StandardParser (Interface Implementation):**
- `ParseFile` - interfaces.go:489
- `ParseFileToMap` - interfaces.go:494
- `ParseString` - interfaces.go:499
- `MustParseFile` - interfaces.go:504
- `Config` - interfaces.go:509

**DataProcessor:**
- `NewDefaultProcessor` - interfaces.go:337
- `NewStrictProcessor` - interfaces.go:352
- `Read` - interfaces.go:374
- `Exists` - interfaces.go:379
- `GetField` - interfaces.go:384
- `IsYAMLFile` - interfaces.go:449

#### Schema Validation - Completely Untested
All schema validation functions have **0% coverage**:

1. `NewSchemaValidator` - schema.go:95
2. `NewSchemaValidatorWithConfig` - schema.go:105
3. `Validate` (instance method) - schema.go:120
4. `ValidateFile` - schema.go:155
5. `validateFields` - schema.go:190
6. `validateField` - schema.go:226
7. `validateType` - schema.go:266
8. `validateConstraints` - schema.go:304
9. `checkMinConstraint` - schema.go:367
10. `checkMaxConstraint` - schema.go:405
11. `isAllowedValue` - schema.go:443
12. `valuesEqual` - schema.go:453
13. `getTypeName` - schema.go:486
14. `joinPath` - schema.go:506
15. `LoadSchema` - schema.go:516
16. `buildSchemaFromData` - schema.go:565
17. `buildFieldDefinition` - schema.go:615
18. `Validate` (SchemaDef method) - schema.go:659
19. `validateFieldDefinition` - schema.go:678
20. `Error` methods (3 instances) - schema.go:707, 742, 757

#### Template Processing - Completely Untested
All template processing functions have **0% coverage**:

1. `NewTemplateProcessor` - template.go:23
2. `ProcessTemplate` - template.go:35
3. `ProcessTemplateFile` - template.go:45
4. `Error` (TemplateError) - template.go:63

#### Result Type String Methods - Untested
Multiple `String()` methods on result types have **0% coverage**:

1. `InputSource.String()` - result_types.go:285
2. `ParseStatus.String()` - result_types.go:302
3. `ValidationStatus.String()` - result_types.go:312
4. `SchemaStatus.String()` - result_types.go:420
5. `FileStatus.String()` - result_types.go:769
6. `ParseErrorCollection.String()` - parse_result.go:354
7. `ParseResult.String()` - parse_result.go:169

#### Multi-File Result Methods - Untested
Methods for processing multi-file results have **0% coverage**:

1. `MultiFileParseResult.HasErrors` - result_types.go:615
2. `MultiFileParseResult.HasWarnings` - result_types.go:620
3. `MultiFileParseResult.ErrorSummary` - result_types.go:625
4. `MultiFileParseResult.HasParseErrors` - result_types.go:696
5. `MultiFileParseResult.HasValidationErrors` - result_types.go:701
6. `MultiFileParseResult.Summary` - result_types.go:706
7. `MultiFileParseResult.IsSuccess` - result_types.go:759
8. `MultiFileParseResult.IsMissing` - result_types.go:764
9. `MultiFileParseResult.HasErrors` - result_types.go:815
10. `MultiFileParseResult.HasWarnings` - result_types.go:820
11. `MultiFileParseResult.SuccessRate` - result_types.go:825
12. `MultiFileParseResult.Summary` - result_types.go:833
13. `MultiFileParseResult.GetFailedFiles` - result_types.go:858
14. `MultiFileParseResult.GetResultsByStatus` - result_types.go:869

#### Other Untested Functions
1. `Transpose` - result.go:356
2. `MatchReturn` - result.go:260
3. `OrElseTry` - parse_result.go:161
4. `IsParseValidationError` - parse_result.go:211
5. `IsParseEmptyError` - parse_result.go:221
6. `ErrorLine` - parse_result.go:262
7. `ErrorColumn` - parse_result.go:270
8. `NewEmptyParseError` - parse_error_design.go:438
9. `ToLegacyParseError` - parse_error_design.go:528
10. `isWhitespaceRune` - parser.go:260
11. `Config` (Parser) - parser.go:105
12. `Unwrap` (ParseError) - parser.go:199
13. `ValidateString` - validator.go:108
14. `Error` (ValidationResult) - validator.go:24
15. `String` (ValidationResult) - validator.go:35
16. `HasErrors` (ValidationResult) - validator.go:71
17. `HasWarnings` (ValidationResult) - validator.go:76

### Low Coverage (1-79%)

#### Debug Helpers (70-79%)
1. `GetRequiredInt` - 70.8% coverage (debug_helpers.go:193)
2. `GetInt` - 76.5% coverage (debug_helpers.go:69)

#### Error Constructors (25-75%)
1. `NewStructureError` - 25% coverage (errors.go:655)
2. `NewConstraintError` - 50% coverage (errors.go:628)
3. `NewFieldNotFoundError` - 75% coverage (errors.go:618)
4. `NewDuplicateKeyError` - 66.7% coverage (errors.go:680)

#### Error Type Methods (partial coverage)
1. `ValidationError.Error()` - 25% coverage (errors.go:784)
2. `ValidationError.Unwrap()` - 50% coverage (errors.go:865)
3. `ConstraintError.Unwrap()` - 50% coverage (errors.go:809)
4. `ParseError.Context()` - 0% coverage (errors.go:302)
5. `ParseError.Unwrap()` - 50% coverage (errors.go:342)
6. `ParseError.Error()` - 83.3% coverage (errors.go:441)
7. `SyntaxError.YAMLErrorType()` - 0% coverage (errors.go:420)
8. `SyntaxError.Unwrap()` - 0% coverage (errors.go:486)
9. `ValidationError.String()` - 33.3% coverage (errors.go:491)

#### Parser and Result Methods (60-79%)
1. `extractErrorLine` - 75% coverage (parser.go:266)
2. `AsParseError` - 60.9% coverage (result.go:376)
3. `stringifyError` - 60% coverage (result.go:528)
4. `FilePath` - 66.7% coverage (result_types.go:190)
5. `WarningSummary` - 53.8% coverage (result_types.go:516)
6. `checkNode` - 78.6% coverage (validator.go:274)
7. `parseYAMLError` - 66.7% coverage (validator.go:177)
8. `ToLegacySyntaxError` - 66.7% coverage (parse_error_design.go:479)

## Uncovered Error Cases

### File I/O Error Paths
Based on `file.go`, these error cases are untested:
- **Absolute path resolution failure**: `filepath.Abs()` error
- **Permission errors**: Files exist but cannot be read due to permissions
- **Directory paths**: Path exists but is a directory (should return false from `FileExists`)

### Parse Error Paths
Based on `parser.go` and error types:
- **Empty file parsing**: Files with 0 bytes
- **Whitespace-only files**: Files with only whitespace characters
- **Invalid YAML syntax**: Malformed YAML structure
- **Type conversion failures**: YAML to Go type mismatches
- **Line/column extraction**: Error location parsing

### Validation Error Paths
Based on `validator.go`:
- **Required field validation**: Missing required fields
- **Constraint validation**: Min/max, pattern, allowed values
- **Type validation**: Schema type mismatches
- **Structural issues**: Duplicate keys, invalid structure

### Schema Validation Error Paths
Based on `schema.go` (completely untested):
- **Schema loading failures**: Invalid schema files
- **Schema building failures**: Invalid field definitions
- **Field validation failures**: Type, constraint, required field errors
- **Nested path validation**: Deep field path validation

## Specific Test Cases Needed

### 1. Error Constructor Tests

#### Test Case: NewValidationError
**Purpose:** Test validation error creation
```go
func TestNewValidationError(t *testing.T) {
    err := NewValidationError(
        "/path/to/file.yaml",
        "Field 'name' is required",
        "user.name",
        "required",
        ErrValidation,
        10,
        5,
        ErrorTypeValidation,
        "user.name",
    )
    assert.NotNil(t, err)
    assert.Equal(t, "/path/to/file.yaml", err.FilePath)
    assert.Equal(t, "Field 'name' is required", err.Message)
    // Add assertions for all fields
}
```

#### Test Case: NewFileError
**Purpose:** Test file I/O error creation
```go
func TestNewFileError(t *testing.T) {
    err := NewFileError(
        "/path/to/file.yaml",
        "read",
        "Failed to read file",
        ErrFileIO,
    )
    assert.NotNil(t, err)
    // Test all error type methods
    assert.Equal(t, ErrFileIO, err.Code())
    assert.Equal(t, ErrorTypeFile, err.YAMLErrorType())
}
```

#### Test Case: NewSyntaxError
**Purpose:** Test YAML syntax error creation
```go
func TestNewSyntaxError(t *testing.T) {
    err := NewSyntaxError(
        "/path/to/file.yaml",
        "invalid YAML syntax",
        5,
        10,
        "mapping",
        "scalar",
        ErrSyntax,
    )
    assert.NotNil(t, err)
    // Verify all fields are set
}
```

### 2. Error Type Method Tests

#### Test Case: ValidationError Methods
**Purpose:** Test all ValidationError interface methods
```go
func TestValidationErrorMethods(t *testing.T) {
    err := NewValidationError(...)
    
    // Test Code()
    assert.Equal(t, ErrValidation, err.Code())
    
    // Test YAMLErrorType()
    assert.Equal(t, ErrorTypeValidation, err.YAMLErrorType())
    
    // Test Context()
    assert.Contains(t, err.Context(), "user.name")
    
    // Test Error()
    assert.Contains(t, err.Error(), "Field 'name' is required")
    
    // Test Unwrap()
    baseErr := errors.New("base error")
    err2 := NewValidationError(...)
    err2.Base = baseErr
    assert.Equal(t, baseErr, errors.Unwrap(err2))
}
```

### 3. Error Predicate Tests

#### Test Case: IsParseError
**Purpose:** Test parse error detection
```go
func TestIsParseError(t *testing.T) {
    parseErr := NewParseError(...)
    validationErr := NewValidationError(...)
    
    assert.True(t, IsParseError(parseErr))
    assert.False(t, IsParseError(validationErr))
    assert.False(t, IsParseError(nil))
}
```

#### Test Case: IsFileNotFoundError
**Purpose:** Test file not found error detection
```go
func TestIsFileNotFoundError(t *testing.T) {
    fileErr := &FileError{Err: os.ErrNotExist}
    otherErr := &FileError{Err: errors.New("other")}
    
    assert.True(t, IsFileNotFoundError(fileErr))
    assert.False(t, IsFileNotFoundError(otherErr))
}
```

#### Test Case: IsPermissionError
**Purpose:** Test permission error detection
```go
func TestIsPermissionError(t *testing.T) {
    permErr := &FileError{Err: os.ErrPermission}
    otherErr := &FileError{Err: errors.New("other")}
    
    assert.True(t, IsPermissionError(permErr))
    assert.False(t, IsPermissionError(otherErr))
}
```

### 4. File I/O Error Tests

#### Test Case: ReadFile - Path Resolution Failure
**Purpose:** Test handling of invalid file paths
```go
func TestReadFile_PathResolutionFailure(t *testing.T) {
    // Use a path with invalid characters that cause filepath.Abs to fail
    invalidPath := string([]byte{0x00}) + "/test.yaml"
    
    content, err := ReadFile(invalidPath)
    assert.Nil(t, content)
    assert.NotNil(t, err)
    
    fileErr, ok := err.(*FileError)
    assert.True(t, ok)
    assert.Equal(t, "resolve", fileErr.Operation)
}
```

#### Test Case: ReadFile - File Not Found
**Purpose:** Test handling of missing files
```go
func TestReadFile_FileNotFound(t *testing.T) {
    content, err := ReadFile("/nonexistent/file.yaml")
    assert.Nil(t, content)
    assert.NotNil(t, err)
    
    assert.True(t, IsFileNotFoundError(err))
}
```

#### Test Case: ReadFile - Permission Denied
**Purpose:** Test handling of permission errors
```go
func TestReadFile_PermissionDenied(t *testing.T) {
    if os.Getuid() == 0 {
        t.Skip("Running as root, permissions don't apply")
    }
    
    // Create a file with no read permissions
    tmpDir := t.TempDir()
    noReadFile := filepath.Join(tmpDir, "noread.yaml")
    writeFile(t, noReadFile, "test: data")
    chmod(t, noReadFile, 0000)
    
    content, err := ReadFile(noReadFile)
    assert.Nil(t, content)
    assert.NotNil(t, err)
    
    assert.True(t, IsPermissionError(err))
}
```

#### Test Case: FileExists - Directory Path
**Purpose:** Test that directories return false
```go
func TestFileExists_DirectoryPath(t *testing.T) {
    tmpDir := t.TempDir()
    assert.False(t, FileExists(tmpDir))
}
```

### 5. Parse Error Tests

#### Test Case: ParseFile - Empty File
**Purpose:** Test handling of empty YAML files
```go
func TestParseFile_EmptyFile(t *testing.T) {
    tmpDir := t.TempDir()
    emptyFile := filepath.Join(tmpDir, "empty.yaml")
    writeFile(t, emptyFile, "")
    
    parser := NewParser()
    result := parser.ParseFile(emptyFile)
    
    assert.True(t, result.IsError())
    assert.True(t, result.IsParseEmptyError())
}
```

#### Test Case: ParseFile - Whitespace Only
**Purpose:** Test handling of whitespace-only files
```go
func TestParseFile_WhitespaceOnly(t *testing.T) {
    tmpDir := t.TempDir()
    wsFile := filepath.Join(tmpDir, "whitespace.yaml")
    writeFile(t, wsFile, "   \n\t\n   ")
    
    parser := NewParser()
    result := parser.ParseFile(wsFile)
    
    assert.True(t, result.IsError())
    assert.True(t, result.IsParseEmptyError())
}
```

#### Test Case: ParseFile - Invalid Syntax
**Purpose:** Test handling of malformed YAML
```go
func TestParseFile_InvalidSyntax(t *testing.T) {
    tmpDir := t.TempDir()
    invalidFile := filepath.Join(tmpDir, "invalid.yaml")
    writeFile(t, invalidFile, "key: value\n  bad_indent: more")
    
    parser := NewParser()
    result := parser.ParseFile(invalidFile)
    
    assert.True(t, result.IsError())
    assert.True(t, result.IsParseSyntaxError())
    
    // Test ErrorLine and ErrorColumn
    line := result.ErrorLine()
    col := result.ErrorColumn()
    assert.Greater(t, line, 0)
    assert.Greater(t, col, 0)
}
```

### 6. Parser Variant Tests

#### Test Case: CachedParser - Basic Caching
**Purpose:** Test that cached parser caches results
```go
func TestCachedParser_BasicCaching(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.yaml")
    writeFile(t, testFile, "key: value")
    
    parser := NewCachedParser(10)
    
    // First call should cache
    result1 := parser.ParseFile(testFile)
    assert.True(t, result1.IsOk())
    
    // Second call should use cache
    result2 := parser.ParseFile(testFile)
    assert.True(t, result2.IsOk())
    
    // Cache size should be 1
    assert.Equal(t, 1, parser.CacheSize())
}
```

#### Test Case: CachedParser - Cache Eviction
**Purpose:** Test LRU eviction when cache is full
```go
func TestCachedParser_CacheEviction(t *testing.T) {
    tmpDir := t.TempDir()
    
    // Create 5 test files
    var files []string
    for i := 0; i < 5; i++ {
        file := filepath.Join(tmpDir, fmt.Sprintf("file%d.yaml", i))
        writeFile(t, file, fmt.Sprintf("key%d: value%d", i, i))
        files = append(files, file)
    }
    
    parser := NewCachedParser(3)
    
    // Parse all 5 files
    for _, file := range files {
        parser.ParseFile(file)
    }
    
    // Cache should only contain last 3
    assert.Equal(t, 3, parser.CacheSize())
    
    // First file should have been evicted
    stats := parser.CacheStats()
    assert.Equal(t, 3, stats.Hits+stats.Misses) // Current cache size
}
```

#### Test Case: CachedParser - ClearCache
**Purpose:** Test cache clearing
```go
func TestCachedParser_ClearCache(t *testing.T) {
    parser := NewCachedParser(10)
    
    // Add some cached items
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.yaml")
    writeFile(t, testFile, "key: value")
    parser.ParseFile(testFile)
    
    assert.Equal(t, 1, parser.CacheSize())
    
    // Clear cache
    parser.ClearCache()
    assert.Equal(t, 0, parser.CacheSize())
}
```

### 7. Schema Validation Tests

#### Test Case: NewSchemaValidator
**Purpose:** Test schema validator creation
```go
func TestNewSchemaValidator(t *testing.T) {
    schema := &SchemaDef{
        Fields: map[string]*FieldDef{
            "name": {
                Type: "string",
                Required: true,
            },
        },
    }
    
    validator, err := NewSchemaValidator(schema)
    assert.Nil(t, err)
    assert.NotNil(t, validator)
}
```

#### Test Case: SchemaValidator - Type Validation
**Purpose:** Test type constraint validation
```go
func TestSchemaValidator_TypeValidation(t *testing.T) {
    schema := &SchemaDef{
        Fields: map[string]*FieldDef{
            "age": {
                Type: "integer",
                Required: true,
            },
        },
    }
    
    validator, _ := NewSchemaValidator(schema)
    
    // Valid input
    data := map[string]interface{}{"age": 25}
    result := validator.Validate(data)
    assert.True(t, result.IsValid())
    
    // Invalid type
    data2 := map[string]interface{}{"age": "twenty-five"}
    result2 := validator.Validate(data2)
    assert.False(t, result2.IsValid())
    assert.True(t, result2.HasErrors())
}
```

#### Test Case: SchemaValidator - Required Field Validation
**Purpose:** Test required field constraints
```go
func TestSchemaValidator_RequiredField(t *testing.T) {
    schema := &SchemaDef{
        Fields: map[string]*FieldDef{
            "name": {
                Type: "string",
                Required: true,
            },
        },
    }
    
    validator, _ := NewSchemaValidator(schema)
    
    // Missing required field
    data := map[string]interface{}{"age": 25}
    result := validator.Validate(data)
    assert.False(t, result.IsValid())
    assert.True(t, result.HasErrors())
}
```

#### Test Case: SchemaValidator - Min/Max Constraints
**Purpose:** Test numeric constraints
```go
func TestSchemaValidator_MinMaxConstraints(t *testing.T) {
    schema := &SchemaDef{
        Fields: map[string]*FieldDef{
            "age": {
                Type: "integer",
                Required: true,
                Min: intPtr(0),
                Max: intPtr(120),
            },
        },
    }
    
    validator, _ := NewSchemaValidator(schema)
    
    // Below minimum
    data1 := map[string]interface{}{"age": -1}
    result1 := validator.Validate(data1)
    assert.False(t, result1.IsValid())
    
    // Above maximum
    data2 := map[string]interface{}{"age": 150}
    result2 := validator.Validate(data2)
    assert.False(t, result2.IsValid())
    
    // Valid range
    data3 := map[string]interface{}{"age": 25}
    result3 := validator.Validate(data3)
    assert.True(t, result3.IsValid())
}
```

### 8. Template Processing Tests

#### Test Case: NewTemplateProcessor
**Purpose:** Test template processor creation
```go
func TestNewTemplateProcessor(t *testing.T) {
    processor := NewTemplateProcessor()
    assert.NotNil(t, processor)
}
```

#### Test Case: ProcessTemplate
**Purpose:** Test template processing
```go
func TestProcessTemplate(t *testing.T) {
    processor := NewTemplateProcessor()
    
    template := "name: {{ .name }}"
    data := map[string]interface{}{"name": "test"}
    
    result, err := processor.ProcessTemplate(template, data)
    assert.Nil(t, err)
    assert.Equal(t, "name: test", result)
}
```

#### Test Case: ProcessTemplateFile
**Purpose:** Test template file processing
```go
func TestProcessTemplateFile(t *testing.T) {
    tmpDir := t.TempDir()
    templateFile := filepath.Join(tmpDir, "template.yaml")
    writeFile(t, templateFile, "name: {{ .name }}")
    
    processor := NewTemplateProcessor()
    data := map[string]interface{}{"name": "test"}
    
    result, err := processor.ProcessTemplateFile(templateFile, data)
    assert.Nil(t, err)
    assert.Equal(t, "name: test", result)
}
```

### 9. Multi-File Result Tests

#### Test Case: MultiFileParseResult - HasErrors
**Purpose:** Test error detection in multi-file results
```go
func TestMultiFileParseResult_HasErrors(t *testing.T) {
    results := []*ParseResult{
        OkParse(map[string]interface{}{"key": "value1"}),
        ErrParse(errors.New("parse error")),
        OkParse(map[string]interface{}{"key": "value2"}),
    }
    
    multiResult := &MultiFileParseResult{
        Results: results,
    }
    
    assert.True(t, multiResult.HasErrors())
    assert.Equal(t, 1, multiResult.ErrorCount())
}
```

#### Test Case: MultiFileParseResult - SuccessRate
**Purpose:** Test success rate calculation
```go
func TestMultiFileParseResult_SuccessRate(t *testing.T) {
    results := []*ParseResult{
        OkParse(map[string]interface{}{"key": "value1"}),
        ErrParse(errors.New("parse error")),
        OkParse(map[string]interface{}{"key": "value2"}),
        ErrParse(errors.New("another error")),
    }
    
    multiResult := &MultiFileParseResult{
        Results: results,
    }
    
    rate := multiResult.SuccessRate()
    assert.Equal(t, 0.5, rate) // 2 successes out of 4
}
```

#### Test Case: MultiFileParseResult - GetFailedFiles
**Purpose:** Test getting list of failed files
```go
func TestMultiFileParseResult_GetFailedFiles(t *testing.T) {
    results := []*ParseResult{
        OkParseWithPath(map[string]interface{}{"key": "value"}, "/path/to/file1.yaml"),
        ErrParseWithPath(errors.New("parse error"), "/path/to/file2.yaml"),
        OkParseWithPath(map[string]interface{}{"key": "value"}, "/path/to/file3.yaml"),
    }
    
    multiResult := &MultiFileParseResult{
        Results: results,
    }
    
    failed := multiResult.GetFailedFiles()
    assert.Len(t, failed, 1)
    assert.Equal(t, "/path/to/file2.yaml", failed[0])
}
```

### 10. Result Type String Method Tests

#### Test Case: InputSource String
**Purpose:** Test InputSource String() method
```go
func TestInputSourceString(t *testing.T) {
    fileSrc := FileSource("/path/to/file.yaml")
    assert.Equal(t, "file:/path/to/file.yaml", fileSrc.String())
    
    strSrc := StringSource("inline content")
    assert.Equal(t, "string:inline content", strSrc.String())
}
```

#### Test Case: ParseStatus String
**Purpose:** Test ParseStatus String() method
```go
func TestParseStatusString(t *testing.T) {
    assert.Equal(t, "success", ParseStatusSuccess.String())
    assert.Equal(t, "failure", ParseStatusFailure.String())
    assert.Equal(t, "empty", ParseStatusEmpty.String())
}
```

## Recommendations

### Priority 1: Critical Error Handling (Immediate)
1. **Test all error constructors** - Create comprehensive tests for NewValidationError, NewFileError, NewSyntaxError, etc.
2. **Test error type methods** - Ensure Code(), YAMLErrorType(), Context(), Error(), and Unwrap() work correctly for all error types
3. **Test error predicates** - IsParseError, IsValidationError, IsFileNotFoundError, IsPermissionError, IsYAMLError

### Priority 2: Parser Variants (High)
1. **Test CachedParser** - Basic caching, eviction, cache stats
2. **Test StreamingParser** - Basic parsing functionality
3. **Test StandardParser** - Interface implementation

### Priority 3: Schema Validation (High)
1. **Test schema validator creation** - NewSchemaValidator, NewSchemaValidatorWithConfig
2. **Test schema validation** - Type validation, required fields, constraints
3. **Test schema loading** - LoadSchema, BuildSchemaFromData

### Priority 4: Template Processing (Medium)
1. **Test template processor** - NewTemplateProcessor, ProcessTemplate, ProcessTemplateFile

### Priority 5: Multi-File Results (Medium)
1. **Test multi-file result methods** - HasErrors, ErrorCount, SuccessRate, GetFailedFiles
2. **Test result type String methods** - All String() methods across result types

### Priority 6: Edge Cases (Low)
1. **Test whitespace-only files** - isWhitespace, isWhitespaceRune
2. **Test Config() methods** - All Config() accessors
3. **Test helper functions** - extractErrorLine, checkNode, parseYAMLError

## Test File Organization

Suggested new test files:
- `errors_constructor_test.go` - Test all error constructors
- `errors_methods_test.go` - Test error type methods
- `errors_predicate_test.go` - Test error predicate functions
- `file_errors_test.go` - Test file I/O error cases
- `parser_variants_test.go` - Test CachedParser, StreamingParser
- `schema_validation_test.go` - Test schema validation
- `template_processing_test.go` - Test template processing
- `multi_file_results_test.go` - Test multi-file result methods
- `result_types_string_test.go` - Test String() methods on result types
- `parse_error_cases_test.go` - Test specific parse error scenarios

## Coverage Targets

**Current State:**
- 106 functions with 0% coverage
- 38 functions with <80% coverage

**Target State:**
- All error constructors: 100%
- All error type methods: 100%
- All error predicates: 100%
- All parser variants: 90%+ (core functionality)
- Schema validation: 80%+ (basic validation paths)
- Template processing: 80%+ (core features)
- Multi-file results: 80%+ (common use cases)

## Conclusion

This analysis reveals significant coverage gaps in the yamlutil package, particularly in:
1. **Error handling infrastructure** - Most error constructors and methods are untested
2. **Parser variants** - CachedParser and StreamingParser are completely untested
3. **Schema validation** - The entire schema validation subsystem is untested
4. **Template processing** - Template processing is completely untested
5. **Multi-file results** - Most multi-file result methods are untested

Implementing the test cases identified in this document will significantly improve the reliability and maintainability of the yamlutil package.
