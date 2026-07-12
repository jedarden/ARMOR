// Package yamlutil provides YAML syntax validation interfaces and implementations.
//
// The syntax validator provides detailed detection and reporting of YAML syntax errors,
// including indentation errors, delimiter errors, and structure errors.
package yamlutil

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// SyntaxValidator defines the interface for YAML syntax error detection.
//
// SyntaxValidator implementations detect and report syntax-level errors in YAML
// content, providing detailed location information and error categorization.
// This interface extends beyond basic parsing to provide comprehensive syntax
// error detection before full parsing attempts.
type SyntaxValidator interface {
	// ValidateSyntax validates YAML syntax only, without semantic validation.
	// Returns a SyntaxValidationResult with detailed syntax error information.
	ValidateSyntax(yamlContent string) SyntaxValidationResult

	// ValidateSyntaxInFile validates YAML syntax in a file.
	// Returns a SyntaxValidationResult with detailed syntax error information.
	ValidateSyntaxInFile(filePath string) SyntaxValidationResult

	// DetectIndentationErrors detects indentation inconsistencies in YAML.
	// Returns a list of indentation errors with line and column information.
	DetectIndentationErrors(yamlContent string) []IndentationError

	// DetectDelimiterErrors detects delimiter issues (braces, brackets, colons).
	// Returns a list of delimiter errors with location information.
	DetectDelimiterErrors(yamlContent string) []DelimiterError

	// DetectStructureErrors detects structural issues before full parsing.
	// Returns a list of structure errors with context.
	DetectStructureErrors(yamlContent string) []StructureError

	// GetErrorContext provides contextual information around an error location.
	// Returns lines before and after the error for better error reporting.
	GetErrorContext(content string, line int, contextLines int) SyntaxErrorContext
}

// SyntaxValidationResult represents the result of syntax validation.
//
// SyntaxValidationResult provides comprehensive information about syntax
// validation operations, including errors found, warnings, and contextual
// information to help diagnose and fix syntax issues.
type SyntaxValidationResult struct {
	FilePath        string                   // Path to the file (if applicable)
	Valid           bool                     // Whether syntax is valid
	SyntaxErrors    []SyntaxError            // List of syntax errors detected
	IndentationErrors []IndentationError    // List of indentation errors
	DelimiterErrors []DelimiterError        // List of delimiter errors
	StructureErrors []StructureError        // List of structure errors
	Warnings        []SyntaxWarning         // List of warnings (non-blocking issues)
	ParseError      error                   // Underlying parse error if any
	ContextLines    int                     // Number of context lines to provide
	TotalLines      int                     // Total lines in the validated content
	ErrorLine       int                     // Primary error line (if any)
}

// HasErrors returns true if any syntax errors were detected.
func (svr SyntaxValidationResult) HasErrors() bool {
	return len(svr.SyntaxErrors) > 0 ||
		len(svr.IndentationErrors) > 0 ||
		len(svr.DelimiterErrors) > 0 ||
		len(svr.StructureErrors) > 0
}

// HasWarnings returns true if any warnings were generated.
func (svr SyntaxValidationResult) HasWarnings() bool {
	return len(svr.Warnings) > 0
}

// ErrorCount returns the total number of errors detected.
func (svr SyntaxValidationResult) ErrorCount() int {
	return len(svr.SyntaxErrors) +
		len(svr.IndentationErrors) +
		len(svr.DelimiterErrors) +
		len(svr.StructureErrors)
}

// WarningCount returns the total number of warnings.
func (svr SyntaxValidationResult) WarningCount() int {
	return len(svr.Warnings)
}

// ErrorSummary returns a formatted summary of all errors.
func (svr SyntaxValidationResult) ErrorSummary() string {
	var sb strings.Builder

	if svr.Valid {
		sb.WriteString("Syntax validation passed")
		if svr.HasWarnings() {
			sb.WriteString(fmt.Sprintf(" with %d warning(s)", svr.WarningCount()))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Syntax validation failed with %d error(s)", svr.ErrorCount()))

		if svr.HasWarnings() {
			sb.WriteString(fmt.Sprintf(" and %d warning(s)", svr.WarningCount()))
		}
	}

	return sb.String()
}

// Error returns a formatted error message if validation failed.
func (svr SyntaxValidationResult) Error() string {
	if svr.Valid {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Syntax validation failed for %s:\n", svr.FilePath))

	// Add syntax errors
	for _, err := range svr.SyntaxErrors {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}

	// Add indentation errors
	for _, err := range svr.IndentationErrors {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}

	// Add delimiter errors
	for _, err := range svr.DelimiterErrors {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}

	// Add structure errors
	for _, err := range svr.StructureErrors {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}

	return sb.String()
}

// IndentationError represents an indentation error in YAML.
//
// IndentationError captures issues with YAML indentation, including:
// - Mixed tabs and spaces
// - Inconsistent indentation levels
// - Invalid indentation for nesting level
// - Incorrect indentation for mapping/sequence items
type IndentationError struct {
	FilePath     string   // Path to the file
	Line         int      // Line number where error occurred (1-indexed)
	Column       int      // Column number where error occurred (1-indexed)
	Message      string   // Description of the indentation error
	Expected     int      // Expected indentation level (in spaces)
	Actual       int      // Actual indentation level (in spaces)
	IndentType  string   // Type of indentation used ("space", "tab", "mixed")
	ContextStr   string   // Contextual information (renamed to avoid method conflict)
	SuggestedFix string  // Suggested fix for the indentation issue
}

// Error implements the error interface.
func (ie IndentationError) Error() string {
	var sb strings.Builder

	if ie.Line > 0 {
		sb.WriteString(fmt.Sprintf("indentation error at line %d", ie.Line))
		if ie.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column %d", ie.Column))
		}
	} else {
		sb.WriteString("indentation error")
	}

	sb.WriteString(fmt.Sprintf(": %s", ie.Message))

	if ie.Expected > 0 || ie.Actual > 0 {
		sb.WriteString(fmt.Sprintf(" (expected: %d spaces, actual: %d spaces)", ie.Expected, ie.Actual))
	}

	if ie.IndentType != "" {
		sb.WriteString(fmt.Sprintf(" [type: %s]", ie.IndentType))
	}

	return sb.String()
}

// Code returns the error code for programmatic handling.
func (ie IndentationError) Code() ErrorCode {
	return ErrCodeInvalidSyntax
}

// YAMLErrorType returns the error type category.
func (ie IndentationError) YAMLErrorType() ErrorType {
	return ErrorTypeSyntax
}

// Context returns additional context about the error.
func (ie IndentationError) Context() string {
	return ie.ContextStr
}

// DelimiterError represents a delimiter error in YAML.
//
// DelimiterError captures issues with YAML delimiters, including:
// - Unmatched braces, brackets, or quotes
// - Missing colons in mappings
// - Invalid use of flow vs block style delimiters
// - Improper delimiter escaping
type DelimiterError struct {
	FilePath       string // Path to the file
	Line           int    // Line number where error occurred (1-indexed)
	Column         int    // Column number where error occurred (1-indexed)
	Message        string // Description of the delimiter error
	DelimiterType  string // Type of delimiter ("{", "}", "[", "]", ":", "'", "\"", "|", ">")
	Expected       string // Expected delimiter (if mismatch)
	Found          string // Actual delimiter found
	ContextStr     string // Contextual information (renamed to avoid method conflict)
	SuggestedFix   string // Suggested fix for the delimiter issue
	ErrorCategory  string // Classification of error ("missing_colon", "unmatched_bracket", "unmatched_brace", "unclosed_string", "invalid_spacing")
}

// Error implements the error interface.
func (de DelimiterError) Error() string {
	var sb strings.Builder

	if de.Line > 0 {
		sb.WriteString(fmt.Sprintf("delimiter error at line %d", de.Line))
		if de.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column %d", de.Column))
		}
	} else {
		sb.WriteString("delimiter error")
	}

	sb.WriteString(fmt.Sprintf(": %s", de.Message))

	if de.DelimiterType != "" {
		sb.WriteString(fmt.Sprintf(" [delimiter: %s]", de.DelimiterType))
	}

	if de.ErrorCategory != "" {
		sb.WriteString(fmt.Sprintf(" [category: %s]", de.ErrorCategory))
	}

	if de.Expected != "" || de.Found != "" {
		sb.WriteString(fmt.Sprintf(" (expected: %q, found: %q)", de.Expected, de.Found))
	}

	return sb.String()
}

// Code returns the error code for programmatic handling.
func (de DelimiterError) Code() ErrorCode {
	return ErrCodeInvalidSyntax
}

// YAMLErrorType returns the error type category.
func (de DelimiterError) YAMLErrorType() ErrorType {
	return ErrorTypeSyntax
}

// Context returns additional context about the error.
func (de DelimiterError) Context() string {
	return de.ContextStr
}

// SyntaxWarning represents a non-critical syntax issue.
//
// SyntaxWarning captures issues that don't prevent parsing but may
// indicate problems or potential bugs, such as:
// - Deprecated syntax usage
// - Stylistic inconsistencies
// - Potential ambiguity
type SyntaxWarning struct {
	FilePath string // Path to the file
	Line     int    // Line number (1-indexed)
	Column   int    // Column number (1-indexed)
	Message  string // Warning message
	Level    string // Warning level ("info", "warning", "deprecation")
}

// Error implements the error interface.
func (sw SyntaxWarning) Error() string {
	if sw.Line > 0 {
		return fmt.Sprintf("warning at line %d: %s", sw.Line, sw.Message)
	}
	return fmt.Sprintf("warning: %s", sw.Message)
}

// SyntaxErrorContext provides contextual lines around an error location.
//
// SyntaxErrorContext helps users understand errors by showing the problematic
// line in context with surrounding lines.
type SyntaxErrorContext struct {
	ErrorLine       int      // The primary error line
	StartLine       int      // First line of context
	EndLine         int      // Last line of context
	Lines           []string // The actual line content
	Pointer         string   // Pointer to error location (e.g., "    ^")
	IndentSpaces    int      // Number of spaces to indent the pointer
	HasError        bool     // Whether this context contains an error
}

// String returns a formatted error context display.
func (ec SyntaxErrorContext) String() string {
	var sb strings.Builder

	for i, line := range ec.Lines {
		lineNum := ec.StartLine + i
		sb.WriteString(fmt.Sprintf("%4d | %s\n", lineNum, line))

		// Add pointer for error line
		if lineNum == ec.ErrorLine && ec.Pointer != "" {
			sb.WriteString(fmt.Sprintf("     | %s%s\n", strings.Repeat(" ", ec.IndentSpaces), ec.Pointer))
		}
	}

	return sb.String()
}

// DefaultSyntaxValidator provides a comprehensive syntax validator implementation.
//
// DefaultSyntaxValidator implements the SyntaxValidator interface using the
// yaml.v3 parser and additional syntax checking logic.
type DefaultSyntaxValidator struct {
	strict         bool   // Whether to use strict validation
	indentSpaces   int    // Expected number of spaces per indentation level
	allowTabs      bool   // Whether to allow tabs in indentation
	contextLines   int    // Number of context lines to provide
	ignorePatterns []string // Patterns to ignore during validation
}

// NewSyntaxValidator creates a new syntax validator with default settings.
//
// Returns a SyntaxValidator with sensible defaults for YAML syntax validation:
// - 2 spaces per indentation level
// - Tabs not allowed
// - 2 context lines
func NewSyntaxValidator() *DefaultSyntaxValidator {
	return &DefaultSyntaxValidator{
		strict:       false,
		indentSpaces: 2,
		allowTabs:    false,
		contextLines: 2,
	}
}

// NewStrictSyntaxValidator creates a strict syntax validator.
//
// Returns a SyntaxValidator with strict validation rules enabled for
// production environments where maximum syntax coverage is required.
func NewStrictSyntaxValidator() *DefaultSyntaxValidator {
	return &DefaultSyntaxValidator{
		strict:       true,
		indentSpaces: 2,
		allowTabs:    false,
		contextLines: 3,
	}
}

// ValidateSyntax validates YAML syntax only.
func (sv *DefaultSyntaxValidator) ValidateSyntax(yamlContent string) SyntaxValidationResult {
	result := SyntaxValidationResult{
		Valid:        true,
		ContextLines: sv.contextLines,
	}

	// Count total lines
	result.TotalLines = strings.Count(yamlContent, "\n") + 1

	// Check for empty content
	if strings.TrimSpace(yamlContent) == "" {
		result.Valid = false
		result.SyntaxErrors = append(result.SyntaxErrors, SyntaxError{
			Message:   "YAML content is empty",
			ErrorCode: ErrCodeFileEmpty,
		})
		return result
	}

	// Parse YAML to detect syntax errors
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &node)
	if err != nil {
		result.Valid = false
		result.ParseError = err
		result.ErrorLine = sv.extractErrorLine(err.Error())
		// Check for specific error types using type assertions
		if err == io.EOF {
			result.SyntaxErrors = append(result.SyntaxErrors, SyntaxError{
				Message:   "Unexpected end of YAML content - content may be incomplete",
				ErrorCode: ErrCodeFileIOError,
			})
		} else {
			result.SyntaxErrors = append(result.SyntaxErrors, sv.convertParseError(err))
		}
	}

	// Detect indentation errors
	indentErrors := sv.DetectIndentationErrors(yamlContent)
	if len(indentErrors) > 0 {
		result.Valid = false
		result.IndentationErrors = append(result.IndentationErrors, indentErrors...)
	}

	// Detect delimiter errors
	delimiterErrors := sv.DetectDelimiterErrors(yamlContent)
	if len(delimiterErrors) > 0 {
		result.Valid = false
		result.DelimiterErrors = append(result.DelimiterErrors, delimiterErrors...)
	}

	// Detect structure errors
	structureErrors := sv.DetectStructureErrors(yamlContent)
	if len(structureErrors) > 0 {
		result.StructureErrors = append(result.StructureErrors, structureErrors...)
		if sv.strict {
			result.Valid = false
		} else {
			// Add as warnings in non-strict mode
			for _, se := range structureErrors {
				result.Warnings = append(result.Warnings, SyntaxWarning{
					Message: se.Message,
					Line:    se.Line,
					Level:   "warning",
				})
			}
		}
	}

	return result
}

// ValidateSyntaxInFile validates YAML syntax in a file.
func (sv *DefaultSyntaxValidator) ValidateSyntaxInFile(filePath string) SyntaxValidationResult {
	result := SyntaxValidationResult{
		FilePath:     filePath,
		Valid:        true,
		ContextLines: sv.contextLines,
	}

	// Read file content
	content, err := ReadFile(filePath)
	if err != nil {
		result.Valid = false
		// Check for specific error types using type assertions
		if err == io.EOF {
			result.SyntaxErrors = append(result.SyntaxErrors, SyntaxError{
				Message:   "Unexpected end of file - file may be incomplete or truncated",
				ErrorCode: ErrCodeFileIOError,
			})
			return result
		}
		result.SyntaxErrors = append(result.SyntaxErrors, SyntaxError{
			Message:   fmt.Sprintf("Failed to read file: %v", err),
			ErrorCode: ErrCodeFileIOError,
		})
		return result
	}

	// Validate content
	contentResult := sv.ValidateSyntax(string(content))
	contentResult.FilePath = filePath
	return contentResult
}

// DetectIndentationErrors detects indentation inconsistencies in YAML.
func (sv *DefaultSyntaxValidator) DetectIndentationErrors(yamlContent string) []IndentationError {
	errors := make([]IndentationError, 0)
	lines := strings.Split(yamlContent, "\n")

	for lineNum, line := range lines {
		// Skip empty lines and comment lines
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Count leading whitespace
		leadingSpaces := 0
		leadingTabs := 0
		for _, ch := range line {
			if ch == ' ' {
				leadingSpaces++
			} else if ch == '\t' {
				leadingTabs++
			} else {
				break
			}
		}

		// Check for mixed tabs and spaces
		if leadingSpaces > 0 && leadingTabs > 0 {
			errors = append(errors, IndentationError{
				Line:         lineNum + 1,
				Column:       1,
				Message:      "Mixed tabs and spaces in indentation",
				IndentType:   "mixed",
				ContextStr:   fmt.Sprintf("Line has %d spaces and %d tabs", leadingSpaces, leadingTabs),
				SuggestedFix: "Use only spaces or only tabs for indentation",
			})
			continue
		}

		// Check if tabs are used when not allowed
		if leadingTabs > 0 && !sv.allowTabs {
			errors = append(errors, IndentationError{
				Line:         lineNum + 1,
				Column:       1,
				Message:      "Tabs not allowed in indentation",
				IndentType:   "tab",
				Actual:       leadingTabs * 4, // Assume tab = 4 spaces for display
				SuggestedFix: "Replace tabs with spaces",
			})
			continue
		}

		// Check if indentation is a multiple of indentSpaces
		if leadingSpaces > 0 && sv.indentSpaces > 0 {
			if leadingSpaces%sv.indentSpaces != 0 {
				expectedLevel := ((leadingSpaces / sv.indentSpaces) + 1) * sv.indentSpaces
				errors = append(errors, IndentationError{
					Line:         lineNum + 1,
					Column:       1,
					Message:      "Indentation not a multiple of expected level",
					Expected:     expectedLevel,
					Actual:       leadingSpaces,
					IndentType:   "space",
					SuggestedFix: fmt.Sprintf("Use %d spaces per indentation level", sv.indentSpaces),
				})
			}
		}
	}

	return errors
}

// DetectDelimiterErrors detects delimiter issues in YAML.
func (sv *DefaultSyntaxValidator) DetectDelimiterErrors(yamlContent string) []DelimiterError {
	errors := make([]DelimiterError, 0)
	lines := strings.Split(yamlContent, "\n")

	// Track delimiter balance
	parenStack := make([]rune, 0)
	bracketStack := make([]rune, 0)
	braceStack := make([]rune, 0)

	// Track multi-line string blocks
	inMultiLineBlock := false
	multiLineBlockIndent := 0


	for lineNum, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		leadingSpaces := len(line) - len(trimmed)

		// Check if we're inside a multi-line block
		if inMultiLineBlock {
			// Multi-line blocks continue until we find a line with less or equal indentation
			// that's not empty or a comment
			if leadingSpaces > multiLineBlockIndent ||
			   (leadingSpaces == multiLineBlockIndent && (trimmed == "" || strings.HasPrefix(trimmed, "#"))) {
				continue // Skip lines inside the multi-line block
			} else {
				// Exit multi-line block when indentation decreases
				inMultiLineBlock = false
				multiLineBlockIndent = 0
			}
		}

		// Skip empty lines and comments (but don't skip them inside multi-line blocks - handled above)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Handle multi-line block scalars (|, >, |-, |- , >-, >+, |+)
		// These appear after a colon, like "key: |" or "key: >"
		if strings.Contains(trimmed, ": |") || strings.Contains(trimmed, ": >") ||
		   strings.Contains(trimmed, ":|-") || strings.Contains(trimmed, ":>-") ||
		   strings.Contains(trimmed, ":|+") || strings.Contains(trimmed, ":>+") {
			inMultiLineBlock = true
			multiLineBlockIndent = leadingSpaces // Content must be MORE indented than this line
		}

		// Check for missing colons in mapping lines
		// A mapping line should have a colon unless it's a sequence item or flow style
		if !strings.HasPrefix(trimmed, "- ") &&
		   !strings.HasPrefix(trimmed, "---") &&
		   !strings.HasPrefix(trimmed, "...") &&
		   !strings.Contains(trimmed, "{") &&
		   !strings.HasPrefix(trimmed, "&") &&
		   !strings.HasPrefix(trimmed, "*") &&
		   !strings.HasPrefix(trimmed, "!!") {

			// Check if this looks like a mapping line (starts with alphanumeric or quote)
			if len(trimmed) > 0 {
				firstChar := rune(trimmed[0])
				isMappingCandidate := unicode.IsLetter(firstChar) || unicode.IsDigit(firstChar) ||
				                      firstChar == '\'' || firstChar == '"' || firstChar == '_'

				// Additional check: if the line has less indentation than previous line,
				// it's more likely to be a key at a higher level
				if isMappingCandidate && !strings.Contains(trimmed, ":") {
					// Extract the first word to see if it looks like a key
					words := strings.Fields(trimmed)
					if len(words) > 0 {
						firstWord := words[0]
						// Check if first word looks like a YAML key (alphanumeric with possible underscores)
						looksLikeKey := true
						for _, ch := range firstWord {
							if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-' && ch != '.' {
								looksLikeKey = false
								break
							}
						}

						if looksLikeKey {
							errors = append(errors, DelimiterError{
								Line:          lineNum + 1,
								Column:        leadingSpaces + 1,
								Message:       fmt.Sprintf("Missing colon in mapping key %q", firstWord),
								DelimiterType: ":",
								Found:         "no colon",
								Expected:      ":",
								SuggestedFix:  fmt.Sprintf("Add colon after key %q", firstWord),
								ErrorCategory: "missing_colon",
							})
						}
					}
				}
			}
		}

		inString := false
		stringChar := rune(0)

		for colNum, ch := range line {
			// Handle string context
			if ch == '\'' || ch == '"' {
				if !inString {
					inString = true
					stringChar = ch
				} else if ch == stringChar {
					// Check for escape
					if colNum == 0 || line[colNum-1] != '\\' {
						inString = false
						stringChar = 0
					}
				}
			}

			// Skip delimiter checking inside strings
			if inString {
				continue
			}

			switch ch {
			case '(':
				parenStack = append(parenStack, ch)
			case ')':
				if len(parenStack) == 0 || parenStack[len(parenStack)-1] != '(' {
					errors = append(errors, DelimiterError{
						Line:          lineNum + 1,
						Column:        colNum + 1,
						Message:       "Unmatched closing parenthesis",
						DelimiterType: ")",
						Found:         ")",
						Expected:      "(",
						ErrorCategory: "unmatched_paren",
					})
				} else {
					parenStack = parenStack[:len(parenStack)-1]
				}
			case '[':
				bracketStack = append(bracketStack, ch)
			case ']':
				if len(bracketStack) == 0 || bracketStack[len(bracketStack)-1] != '[' {
					errors = append(errors, DelimiterError{
						Line:          lineNum + 1,
						Column:        colNum + 1,
						Message:       "Unmatched closing bracket",
						DelimiterType: "]",
						Found:         "]",
						Expected:      "[",
						ErrorCategory: "unmatched_bracket",
					})
				} else {
					bracketStack = bracketStack[:len(bracketStack)-1]
				}
			case '{':
				braceStack = append(braceStack, ch)
			case '}':
				if len(braceStack) == 0 || braceStack[len(braceStack)-1] != '{' {
					errors = append(errors, DelimiterError{
						Line:          lineNum + 1,
						Column:        colNum + 1,
						Message:       "Unmatched closing brace",
						DelimiterType: "}",
						Found:         "}",
						Expected:      "{",
						ErrorCategory: "unmatched_brace",
					})
				} else {
					braceStack = braceStack[:len(braceStack)-1]
				}
			case ':':
				// Check for invalid colon spacing (not followed by space or end of line)
				if colNum+1 < len(line) && line[colNum+1] != ' ' && line[colNum+1] != '\t' && line[colNum+1] != '\n' && line[colNum+1] != '\r' {
					// This might be valid in flow collections, but could be an error in block mappings
					// Only warn in strict mode for non-flow contexts
					if sv.strict && !unicode.IsSpace(rune(line[colNum+1])) && !strings.Contains(line, "{") && !strings.Contains(line, "[") {
						errors = append(errors, DelimiterError{
							Line:          lineNum + 1,
							Column:        colNum + 1,
							Message:       "Colon not followed by space in mapping",
							DelimiterType: ":",
							Found:         string(line[colNum:min(colNum+2, len(line))]),
							SuggestedFix:  "Add space after colon",
							ErrorCategory: "invalid_spacing",
						})
					}
				}
			}
		}

		// Check for unclosed strings at end of line
		if inString {
			errors = append(errors, DelimiterError{
				Line:          lineNum + 1,
				Column:        len(line) + 1,
				Message:       "Unclosed string at end of line",
				DelimiterType: string(stringChar),
				Found:         "end of line",
				Expected:      string(stringChar),
				SuggestedFix:  "Close the string with matching quote",
				ErrorCategory: "unclosed_string",
			})
		}
	}

	// Check for unmatched delimiters at end of file
	for _, paren := range parenStack {
		errors = append(errors, DelimiterError{
			Line:          len(lines),
			Message:       "Unclosed parenthesis at end of file",
			DelimiterType: string(paren),
			Expected:      ")",
			ErrorCategory: "unmatched_paren",
		})
	}

	for _, bracket := range bracketStack {
		errors = append(errors, DelimiterError{
			Line:          len(lines),
			Message:       "Unclosed bracket at end of file",
			DelimiterType: string(bracket),
			Expected:      "]",
			ErrorCategory: "unmatched_bracket",
		})
	}

	for _, brace := range braceStack {
		errors = append(errors, DelimiterError{
			Line:          len(lines),
			Message:       "Unclosed brace at end of file",
			DelimiterType: string(brace),
			Expected:      "}",
			ErrorCategory: "unmatched_brace",
		})
	}

	return errors
}

// DetectStructureErrors detects structural issues in YAML.
func (sv *DefaultSyntaxValidator) DetectStructureErrors(yamlContent string) []StructureError {
	errors := make([]StructureError, 0)

	// Parse YAML to detect structure issues
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &node)
	if err != nil {
		// Parse errors are already captured in SyntaxErrors
		return errors
	}

	// Check for duplicate keys
	errors = append(errors, sv.checkDuplicateKeys(&node, yamlContent)...)

	// Check for invalid mapping structures
	errors = append(errors, sv.checkInvalidMappings(&node, yamlContent)...)

	// Check for invalid sequence structures
	errors = append(errors, sv.checkInvalidSequences(&node, yamlContent)...)

	// Check for mixed content types
	errors = append(errors, sv.checkMixedContent(&node, yamlContent)...)

	return errors
}

// checkDuplicateKeys checks for duplicate mapping keys.
func (sv *DefaultSyntaxValidator) checkDuplicateKeys(node *yaml.Node, content string) []StructureError {
	var errors []StructureError

	if node == nil {
		return errors
	}

	// Check this node if it's a mapping
	if node.Kind == yaml.MappingNode {
		seenKeys := make(map[string]int)
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 < len(node.Content) {
				keyNode := node.Content[i]
				if keyNode.Kind == yaml.ScalarNode && keyNode.Value != "" {
					key := keyNode.Value
					if firstLine, seen := seenKeys[key]; seen {
						errors = append(errors, StructureError{
							FilePath:     "",
							Line:         keyNode.Line,
							Message:      fmt.Sprintf("Duplicate key %q detected", key),
							DuplicateKey: key,
							Location:     fmt.Sprintf("first occurrence at line %d", firstLine),
							ErrorCode:    ErrCodeDuplicateKey,
						})
					} else {
						seenKeys[key] = keyNode.Line
					}
				}
			}
		}
	}

	// Recursively check child nodes
	for _, child := range node.Content {
		errors = append(errors, sv.checkDuplicateKeys(child, content)...)
	}

	return errors
}

// checkInvalidMappings checks for invalid mapping structures.
func (sv *DefaultSyntaxValidator) checkInvalidMappings(node *yaml.Node, content string) []StructureError {
	var errors []StructureError

	if node == nil {
		return errors
	}

	// Check this node if it's a mapping
	if node.Kind == yaml.MappingNode {
		// Mappings must have an even number of children (key-value pairs)
		if len(node.Content)%2 != 0 {
			errors = append(errors, StructureError{
				FilePath:  "",
				Line:      node.Line,
				Message:   "Mapping has odd number of nodes (missing value for key)",
				Location:  fmt.Sprintf("mapping at line %d", node.Line),
				ErrorCode: ErrCodeInvalidStructure,
			})
		}

		// Check for non-scalar keys (keys must be scalar)
		for i := 0; i < len(node.Content); i += 2 {
			if i < len(node.Content) {
				keyNode := node.Content[i]
				if keyNode.Kind != yaml.ScalarNode {
					errors = append(errors, StructureError{
						FilePath:  "",
						Line:      keyNode.Line,
						Message:   fmt.Sprintf("Mapping key must be scalar, found %s", kindToString(keyNode.Kind)),
						Location:  fmt.Sprintf("key at line %d", keyNode.Line),
						ErrorCode: ErrCodeInvalidStructure,
					})
				}
			}
		}
	}

	// Recursively check child nodes
	for _, child := range node.Content {
		errors = append(errors, sv.checkInvalidMappings(child, content)...)
	}

	return errors
}

// checkInvalidSequences checks for invalid sequence structures.
func (sv *DefaultSyntaxValidator) checkInvalidSequences(node *yaml.Node, content string) []StructureError {
	var errors []StructureError

	if node == nil {
		return errors
	}

	// Check this node if it's a sequence
	if node.Kind == yaml.SequenceNode {
		// Check for inconsistent indentation in multi-line sequences
		lines := strings.Split(content, "\n")
		for i, child := range node.Content {
			if child.Line-1 < len(lines) {
				line := lines[child.Line-1]
				// Check if sequence items have consistent indentation
				trimmed := strings.TrimLeft(line, " \t")
				if strings.HasPrefix(trimmed, "- ") {
					// Valid sequence item
				} else if !strings.HasPrefix(trimmed, "-") && trimmed != "" {
					// Potential structure issue - sequence item without dash
					if child.Kind == yaml.ScalarNode {
						errors = append(errors, StructureError{
							FilePath:  "",
							Line:      child.Line,
							Message:   "Sequence item should start with '-'",
							Location:  fmt.Sprintf("item %d at line %d", i+1, child.Line),
							ErrorCode: ErrCodeInvalidStructure,
						})
					}
				}
			}
		}
	}

	// Recursively check child nodes
	for _, child := range node.Content {
		errors = append(errors, sv.checkInvalidSequences(child, content)...)
	}

	return errors
}

// checkMixedContent checks for mixed content types in collections.
func (sv *DefaultSyntaxValidator) checkMixedContent(node *yaml.Node, content string) []StructureError {
	var errors []StructureError

	if node == nil {
		return errors
	}

	// Check sequences for mixed types
	if node.Kind == yaml.SequenceNode && len(node.Content) > 1 {
		firstKind := node.Content[0].Kind
		for i, child := range node.Content {
			if child.Kind != firstKind && child.Kind != yaml.ScalarNode {
				// Allow mixing scalars with other types, but flag mixed complex types
				errors = append(errors, StructureError{
					FilePath:  "",
					Line:      child.Line,
					Message:   fmt.Sprintf("Sequence contains mixed content types (%s and %s)",
						kindToString(firstKind), kindToString(child.Kind)),
					Location:  fmt.Sprintf("item %d at line %d", i+1, child.Line),
					ErrorCode: ErrCodeInvalidStructure,
				})
			}
		}
	}

	// Recursively check child nodes
	for _, child := range node.Content {
		errors = append(errors, sv.checkMixedContent(child, content)...)
	}

	return errors
}

// kindToString converts a yaml.Node Kind to a readable string.
func kindToString(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// GetErrorContext provides contextual information around an error location.
func (sv *DefaultSyntaxValidator) GetErrorContext(content string, line int, contextLines int) SyntaxErrorContext {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	startLine := max(1, line-contextLines)
	endLine := min(totalLines, line+contextLines)

	context := SyntaxErrorContext{
		ErrorLine: line,
		StartLine: startLine,
		EndLine:   endLine,
		Lines:     lines[startLine-1 : endLine],
		HasError:  true,
	}

	// Create pointer for error line if available
	if line >= startLine && line <= endLine && line <= totalLines {
		errorLineIdx := line - startLine
		if errorLineIdx >= 0 && errorLineIdx < len(context.Lines) {
			// Find first non-whitespace character for pointer
			errorLineContent := context.Lines[errorLineIdx]
			indentSpaces := 0
			for _, ch := range errorLineContent {
				if ch == ' ' || ch == '\t' {
					indentSpaces++
				} else {
					break
				}
			}
			context.IndentSpaces = indentSpaces
			context.Pointer = "^"
		}
	}

	return context
}

// convertParseError converts a yaml.v3 parse error to SyntaxError.
func (sv *DefaultSyntaxValidator) convertParseError(err error) SyntaxError {
	se := SyntaxError{
		Err: err,
	}

	// Check for specific YAML error types using type assertions
	if typeErr, ok := err.(*yaml.TypeError); ok {
		// This is a YAML type error - provide detailed information
		se.Message = fmt.Sprintf("YAML type mismatch: %v", typeErr.Errors)
		se.ErrorCode = ErrCodeTypeMismatch
		return se
	}

	errMsg := err.Error()
	se.Message = errMsg

	// Extract line and column from error message
	se.Line = sv.extractErrorLine(errMsg)
	se.Column = sv.extractErrorColumn(errMsg)

	// Set error code based on message content
	if strings.Contains(errMsg, "duplicate") {
		se.ErrorCode = ErrCodeDuplicateKey
	} else {
		se.ErrorCode = ErrCodeInvalidSyntax
	}

	return se
}

// extractErrorLine extracts line number from error message.
func (sv *DefaultSyntaxValidator) extractErrorLine(errMsg string) int {
	// Try common patterns
	patterns := []string{
		"line %d",
		"line %d:",
		"at line %d",
	}

	for _, pattern := range patterns {
		var line int
		if _, err := fmt.Sscanf(errMsg, pattern, &line); err == nil && line > 0 {
			return line
		}
	}

	// Try regex-like pattern matching
	if strings.Contains(errMsg, "line ") {
		parts := strings.Split(errMsg, "line ")
		if len(parts) > 1 {
			lineStr := strings.Fields(parts[1])[0]
			var line int
			if _, err := fmt.Sscanf(lineStr, "%d", &line); err == nil && line > 0 {
				return line
			}
		}
	}

	return 0
}

// extractErrorColumn extracts column number from error message.
func (sv *DefaultSyntaxValidator) extractErrorColumn(errMsg string) int {
	if strings.Contains(errMsg, "column ") {
		parts := strings.Split(errMsg, "column ")
		if len(parts) > 1 {
			colStr := strings.Fields(parts[1])[0]
			var col int
			if _, err := fmt.Sscanf(colStr, "%d", &col); err == nil && col > 0 {
				return col
			}
		}
	}

	return 0
}

// extractQuotedKey extracts a quoted key from a line.
// Returns the quoted string (including quotes) if found, empty string otherwise.
func extractQuotedKey(line string) string {
	if len(line) == 0 {
		return ""
	}

	// Check for single-quoted key
	if line[0] == '\'' {
		for i := 1; i < len(line); i++ {
			if line[i] == '\'' && (i == 0 || line[i-1] != '\\') {
				return line[:i+1]
			}
		}
	}

	// Check for double-quoted key
	if line[0] == '"' {
		for i := 1; i < len(line); i++ {
			if line[i] == '"' && (i == 0 || line[i-1] != '\\') {
				return line[:i+1]
			}
		}
	}

	return ""
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
