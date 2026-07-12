// Package yamlutil provides YAML line parsing and key identification infrastructure.
//
// This file contains dedicated functions for calculating indentation levels
// and classifying YAML line types, providing foundational parsing infrastructure
// for syntax validation and error detection.
package yamlutil

import (
	"fmt"
	"strings"
	"unicode"
)

// IndentationInfo represents detailed information about line indentation.
type IndentationInfo struct {
	Level        int    // Indentation level (0 = no indent, 1 = first level, etc.)
	SpaceCount   int    // Number of leading spaces
	TabCount     int    // Number of leading tabs
	TotalWidth   int    // Total whitespace character count
	IndentType   string // "space", "tab", "mixed", or "none"
	IsMixed      bool   // True if both spaces and tabs are present in leading whitespace
}

// CalculateIndentation calculates the indentation level from a line.
//
// This function counts leading spaces and tabs in a line and returns detailed
// indentation information. The indentation level is calculated based on the
// provided spacesPerLevel parameter (typically 2 or 4 for space-based indentation,
// or 1 for tab-based indentation).
//
// Parameters:
//   - line: The line content to analyze
//   - spacesPerLevel: Number of spaces per indentation level (0 to detect from line)
//
// Returns an IndentationInfo struct with detailed indentation metadata.
//
// Behavior:
//   - Spaces are counted as 1 character each
//   - Tabs are counted as 1 character each (for width calculations, assume tab width = spacesPerLevel)
//   - If both spaces and tabs appear in the leading whitespace, IsMixed is set to true
//   - The Level field is calculated as SpaceCount / spacesPerLevel when using space indentation
//   - For tab indentation, Level equals TabCount
//
// Examples:
//   - CalculateIndentation("  key: value", 2) → {Level: 1, SpaceCount: 2, IndentType: "space"}
//   - CalculateIndentation("    key: value", 2) → {Level: 2, SpaceCount: 4, IndentType: "space"}
//   - CalculateIndentation("\tkey: value", 1) → {Level: 1, TabCount: 1, IndentType: "tab"}
func CalculateIndentation(line string, spacesPerLevel int) IndentationInfo {
	info := IndentationInfo{
		Level:      0,
		SpaceCount: 0,
		TabCount:   0,
		TotalWidth: 0,
		IndentType: "none",
		IsMixed:    false,
	}

	// Count leading whitespace
	for _, ch := range line {
		if ch == ' ' {
			info.SpaceCount++
			info.TotalWidth++
		} else if ch == '\t' {
			info.TabCount++
			info.TotalWidth++
		} else {
			break // Stop at first non-whitespace character
		}
	}

	// Determine indentation type
	if info.SpaceCount > 0 && info.TabCount > 0 {
		info.IndentType = "mixed"
		info.IsMixed = true
	} else if info.TabCount > 0 {
		info.IndentType = "tab"
	} else if info.SpaceCount > 0 {
		info.IndentType = "space"
	}

	// Calculate indentation level
	if spacesPerLevel > 0 {
		if info.IndentType == "space" {
			info.Level = info.SpaceCount / spacesPerLevel
		} else if info.IndentType == "tab" {
			// For tab-based indentation, each tab is one level
			info.Level = info.TabCount
		}
	}

	return info
}

// ClassifyLineType classifies a YAML line into its type category.
//
// This function analyzes a line (after leading whitespace has been removed)
// and determines what type of YAML line it is. The classification is based
// on the YAML specification for line types.
//
// Parameters:
//   - line: The line content (should have leading whitespace already trimmed)
//
// Returns the LineType that best describes this line.
//
// Classification rules:
//   - Empty or whitespace-only strings → LineTypeBlank
//   - Lines starting with '#' → LineTypeComment
//   - Lines starting with '---' → LineTypeDocumentStart
//   - Lines starting with '...' → LineTypeDocumentEnd
//   - All other lines → LineTypeRegular
//
// Examples:
//   - ClassifyLineType("") → LineTypeBlank
//   - ClassifyLineType("  ") → LineTypeBlank
//   - ClassifyLineType("# comment") → LineTypeComment
//   - ClassifyLineType("---") → LineTypeDocumentStart
//   - ClassifyLineType("...") → LineTypeDocumentEnd
//   - ClassifyLineType("key: value") → LineTypeRegular
//   - ClassifyLineType("- item") → LineTypeRegular
func ClassifyLineType(line string) LineType {
	trimmed := strings.TrimSpace(line)

	// Check for blank lines
	if trimmed == "" {
		return LineTypeBlank
	}

	// Check for comment lines
	if strings.HasPrefix(trimmed, "#") {
		return LineTypeComment
	}

	// Check for document start marker
	if strings.HasPrefix(trimmed, "---") {
		return LineTypeDocumentStart
	}

	// Check for document end marker
	if strings.HasPrefix(trimmed, "...") {
		return LineTypeDocumentEnd
	}

		// For any other content, classify based on pattern
		// Check for sequence items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t") {
			return LineTypeSequenceItem
		}

		// Check for mapping keys (lines with colons)
		if strings.Contains(trimmed, ":") {
			// Could be a mapping key or value, default to mapping key
			return LineTypeMappingKey
		}

		// Default to unknown for anything else
		return LineTypeUnknown
}

// IsBlankLine determines if a line is blank (empty or whitespace-only).
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line is empty or contains only whitespace.
//
// Examples:
//   - IsBlankLine("") → true
//   - IsBlankLine("   ") → true
//   - IsBlankLine("\t\t") → true
//   - IsBlankLine("  \t  ") → true
//   - IsBlankLine("key: value") → false
func IsBlankLine(line string) bool {
	return len(strings.TrimSpace(line)) == 0
}

// IsCommentLine determines if a line is a comment line.
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line's first non-whitespace character is '#'.
//
// Examples:
//   - IsCommentLine("# comment") → true
//   - IsCommentLine("  # comment") → true
//   - IsCommentLine("\t# comment") → true
//   - IsCommentLine("key: value # not a comment") → false
//   - IsCommentLine("") → false
func IsCommentLine(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return len(trimmed) > 0 && trimmed[0] == '#'
}

// IsSequenceItem determines if a line starts a sequence item.
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line's first non-whitespace character is '-' followed by
// a space or tab (indicating a sequence item in YAML).
//
// Examples:
//   - IsSequenceItem("- item") → true
//   - IsSequenceItem("  - item") → true
//   - IsSequenceItem("\t- item") → true
//   - IsSequenceItem("-item") → false (no space after dash)
//   - IsSequenceItem("key: value") → false
func IsSequenceItem(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t")
}

// ExtractLeadingWhitespace extracts the leading whitespace from a line.
//
// Parameters:
//   - line: The line content to process
//
// Returns the leading whitespace portion of the line (may be empty).
//
// Examples:
//   - ExtractLeadingWhitespace("  key: value") → "  "
//   - ExtractLeadingWhitespace("\t\tkey: value") → "\t\t"
//   - ExtractLeadingWhitespace("key: value") → ""
//   - ExtractLeadingWhitespace("  \t  key: value") → "  \t  "
func ExtractLeadingWhitespace(line string) string {
	var whitespace strings.Builder
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			whitespace.WriteRune(ch)
		} else {
			break
		}
	}
	return whitespace.String()
}

// HasValidIndentation checks if a line has consistent indentation.
//
// This function verifies that the leading whitespace in a line is consistent
// (all spaces or all tabs, not mixed). Mixed indentation is generally considered
// invalid in YAML and can cause parsing errors.
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line has no leading whitespace or uses consistent
// indentation (all spaces or all tabs). Returns false if the line has mixed
// spaces and tabs in the leading whitespace.
//
// Examples:
//   - HasValidIndentation("key: value") → true (no indent)
//   - HasValidIndentation("  key: value") → true (spaces only)
//   - HasValidIndentation("\t\tkey: value") → true (tabs only)
//   - HasValidIndentation("  \tkey: value") → false (mixed)
//   - HasValidIndentation("\t  key: value") → false (mixed)
func HasValidIndentation(line string) bool {
	info := CalculateIndentation(line, 0)
	return !info.IsMixed
}

// NormalizeIndentation converts tabs to spaces in the leading whitespace.
//
// This function takes a line and converts any leading tabs to spaces based on
// the specified tab width. This is useful for normalizing indentation before
// processing or when working with mixed indentation styles.
//
// Parameters:
//   - line: The line content to normalize
//   - tabWidth: Number of spaces to replace each tab with (typically 2 or 4)
//
// Returns a new string with leading tabs converted to spaces.
//
// Examples:
//   - NormalizeIndentation("\tkey: value", 2) → "  key: value"
//   - NormalizeIndentation("\t\tkey: value", 2) → "    key: value"
//   - NormalizeIndentation("  key: value", 2) → "  key: value" (no change)
//   - NormalizeIndentation("key: value", 2) → "key: value" (no change)
func NormalizeIndentation(line string, tabWidth int) string {
	if tabWidth <= 0 {
		tabWidth = 2 // Default to 2 spaces per tab
	}

	info := CalculateIndentation(line, 0)
	if info.TabCount == 0 {
		return line // No tabs to convert
	}

	// Build the normalized leading whitespace
	var normalized strings.Builder
	for i := 0; i < info.TabCount; i++ {
		for j := 0; j < tabWidth; j++ {
			normalized.WriteByte(' ')
		}
	}
	for i := 0; i < info.SpaceCount; i++ {
		normalized.WriteByte(' ')
	}

	// Append the rest of the line
	restStart := info.TabCount + info.SpaceCount
	if restStart < len(line) {
		normalized.WriteString(line[restStart:])
	}

	return normalized.String()
}

// DetectIndentStyle analyzes a line to detect its indentation style.
//
// This function determines whether a line uses space-based or tab-based indentation,
// or if it has no indentation at all.
//
// Parameters:
//   - line: The line content to analyze
//
// Returns a string describing the indentation style: "space", "tab", "mixed", or "none".
//
// Examples:
//   - DetectIndentStyle("  key: value") → "space"
//   - DetectIndentStyle("\tkey: value") → "tab"
//   - DetectIndentStyle("  \tkey: value") → "mixed"
//   - DetectIndentStyle("key: value") → "none"
func DetectIndentStyle(line string) string {
	info := CalculateIndentation(line, 0)
	return info.IndentType
}

// CountIndentationLevel counts the indentation level of a line.
//
// This is a convenience function that calculates the indentation level based on
// the provided spaces per level. It's equivalent to calling CalculateIndentation
// and accessing the Level field.
//
// Parameters:
//   - line: The line content to analyze
//   - spacesPerLevel: Number of spaces per indentation level
//
// Returns the indentation level (0 for no indent, 1 for first level, etc.).
//
// Examples:
//   - CountIndentationLevel("  key: value", 2) → 1
//   - CountIndentationLevel("    key: value", 2) → 2
//   - CountIndentationLevel("\tkey: value", 1) → 1
//   - CountIndentationLevel("key: value", 2) → 0
func CountIndentationLevel(line string, spacesPerLevel int) int {
	info := CalculateIndentation(line, spacesPerLevel)
	return info.Level
}

// TrimLeadingWhitespace removes leading whitespace from a line.
//
// This is a convenience function that's equivalent to strings.TrimLeft(line, " \t")
// but is provided here for API completeness and consistency with other indentation
// functions in this package.
//
// Parameters:
//   - line: The line content to trim
//
// Returns the line with leading spaces and tabs removed.
//
// Examples:
//   - TrimLeadingWhitespace("  key: value") → "key: value"
//   - TrimLeadingWhitespace("\t\tkey: value") → "key: value"
//   - TrimLeadingWhitespace("  \t  key: value") → "key: value"
func TrimLeadingWhitespace(line string) string {
	return strings.TrimLeft(line, " \t")
}

// IsPrintableWithoutContent checks if a line can be printed as-is without affecting content.
//
// This function determines if a line is "structurally significant" - meaning it's not
// blank, not a comment, and not purely whitespace. Structurally significant lines are
// those that contain actual YAML content (keys, values, sequence items, etc.).
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line contains structurally significant content.
//
// Examples:
//   - IsPrintableWithoutContent("key: value") → true
//   - IsPrintableWithoutContent("- item") → true
//   - IsPrintableWithoutContent("---") → true
//   - IsPrintableWithoutContent("# comment") → false
//   - IsPrintableWithoutContent("") → false
//   - IsPrintableWithoutContent("   ") → false
func IsPrintableWithoutContent(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) == 0 {
		return false // Blank line
	}
	if trimmed[0] == '#' {
		return false // Comment line
	}
	return true
}

// GetTrailingComment extracts a trailing comment from a line, if present.
//
// This function looks for a '#' character that's not inside a quoted string and
// returns the comment portion of the line. If no trailing comment is found, it
// returns an empty string.
//
// Note: This is a simple implementation that doesn't handle all YAML edge cases
// (like quotes within quotes, escaped quotes, etc.). For full YAML parsing,
// use a proper YAML parser.
//
// Parameters:
//   - line: The line content to check
//
// Returns the trailing comment text (without the '#' character), or empty string
// if no trailing comment is present.
//
// Examples:
//   - GetTrailingComment("key: value # comment") → " comment"
//   - GetTrailingComment("key: value") → ""
//   - GetTrailingComment("# full line comment") → " full line comment"
//   - GetTrailingComment("key: 'value # not comment'") → "" (inside quotes)
func GetTrailingComment(line string) string {
	// First, check if this is a comment-only line
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) > 0 && trimmed[0] == '#' {
		return trimmed[1:] // Return everything after #
	}

	// Look for # outside of quotes
	inSingleQuote := false
	inDoubleQuote := false
	escapeNext := false

	for i, ch := range line {
		if escapeNext {
			escapeNext = false
			continue
		}

		switch ch {
		case '\\':
			escapeNext = true
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case '#':
			if !inSingleQuote && !inDoubleQuote {
				// Found a comment start outside of quotes
				return line[i+1:]
			}
		}
	}

	return ""
}

// MeasureIndentWidth calculates the display width of indentation.
//
// This function calculates how wide the indentation appears when displayed,
// accounting for the fact that tabs are typically rendered as multiple spaces.
//
// Parameters:
//   - line: The line content to measure
//   - tabWidth: Number of space equivalents per tab (typically 2, 4, or 8)
//
// Returns the display width of the leading whitespace.
//
// Examples:
//   - MeasureIndentWidth("  key: value", 4) → 2
//   - MeasureIndentWidth("\tkey: value", 4) → 4
//   - MeasureIndentWidth("\t\tkey: value", 2) → 4
//   - MeasureIndentWidth("  \tkey: value", 4) → 6 (2 spaces + 1 tab)
func MeasureIndentWidth(line string, tabWidth int) int {
	info := CalculateIndentation(line, 0)
	return info.SpaceCount + (info.TabCount * tabWidth)
}

// IsValidIndentLevel checks if an indentation level is valid for the given spaces per level.
//
// This function verifies that the indentation on a line is a multiple of the expected
// indentation unit, which helps detect inconsistent indentation in YAML files.
//
// Parameters:
//   - line: The line content to check
//   - spacesPerLevel: Expected number of spaces per indentation level
//
// Returns true if the line has no indentation or if its space count is a multiple of
// spacesPerLevel. Returns false for tab-indented lines (if spacesPerLevel > 0) or
// if the indentation is not a multiple of the expected unit.
//
// Examples:
//   - IsValidIndentLevel("key: value", 2) → true (no indent)
//   - IsValidIndentLevel("  key: value", 2) → true (1 level)
//   - IsValidIndentLevel("    key: value", 2) → true (2 levels)
//   - IsValidIndentLevel("   key: value", 2) → false (3 spaces, not multiple of 2)
//   - IsValidIndentLevel("\tkey: value", 2) → false (tab indent with space expectation)
func IsValidIndentLevel(line string, spacesPerLevel int) bool {
	if spacesPerLevel <= 0 {
		return true // Can't validate without a reference
	}

	info := CalculateIndentation(line, 0)

	// No indentation is always valid
	if info.TotalWidth == 0 {
		return true
	}

	// Tabs with space-based expectation is invalid
	if info.TabCount > 0 && info.SpaceCount == 0 {
		return false
	}

	// Mixed indentation is invalid
	if info.IsMixed {
		return false
	}

	// Check if space count is a multiple of spacesPerLevel
	return info.SpaceCount % spacesPerLevel == 0
}

// ContainsOnlyASCIIWhitespace checks if a string contains only ASCII whitespace characters.
//
// This function verifies that a line contains only space (0x20) and tab (0x09) characters,
// which are the only whitespace characters recognized by the YAML specification for
// indentation purposes.
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line is empty or contains only spaces and tabs.
//
// Examples:
//   - ContainsOnlyASCIIWhitespace("") → true
//   - ContainsOnlyASCIIWhitespace("   ") → true
//   - ContainsOnlyASCIIWhitespace("\t\t") → true
//   - ContainsOnlyASCIIWhitespace("  \t  ") → true
//   - ContainsOnlyASCIIWhitespace(" \n ") → false (contains newline)
//   - ContainsOnlyASCIIWhitespace("  a") → false (contains non-whitespace)
func ContainsOnlyASCIIWhitespace(line string) bool {
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			return false
		}
	}
	return true
}

// EstimateIndentFromContent attempts to estimate the indentation style from content.
//
// This function analyzes multiple lines to determine the most likely indentation
// pattern. It's useful for auto-detecting the indentation style when parsing YAML.
//
// Parameters:
//   - lines: Slice of lines to analyze
//
// Returns the most common indentation level (number of spaces) or 0 if tabs are
// predominantly used.
//
// Note: This is a heuristic function and may not be 100% accurate for all YAML files.
// For definitive results, use a full YAML parser.
func EstimateIndentFromContent(lines []string) int {
	if len(lines) == 0 {
		return 2 // Default to 2 spaces
	}

	// Track indentation frequencies
	indentFreq := make(map[int]int)
	hasTabs := false

	for _, line := range lines {
		info := CalculateIndentation(line, 0)

		// Skip blank and comment lines for indent detection
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) == 0 || (len(trimmed) > 0 && trimmed[0] == '#') {
			continue
		}

		if info.TabCount > 0 {
			hasTabs = true
		}

		if info.SpaceCount > 0 {
			indentFreq[info.SpaceCount]++
		}
	}

	// If tabs are present and no spaces, return 0 to indicate tab-based
	if hasTabs && len(indentFreq) == 0 {
		return 0
	}

	// Find the most common non-zero indentation
	maxCount := 0
	mostCommonIndent := 2 // Default

	for indent, count := range indentFreq {
		if count > maxCount {
			maxCount = count
			mostCommonIndent = indent
		}
	}

	// If the most common indent is large, it might be a multiple
	// Try to find the GCD of all indents to determine the base unit
	indents := make([]int, 0, len(indentFreq))
	for indent := range indentFreq {
		indents = append(indents, indent)
	}

	if len(indents) == 0 {
		return 2 // No indentation found, default to 2
	}

	// Calculate GCD of all indents
	gcd := indents[0]
	for _, indent := range indents[1:] {
		gcd = computeGCD(gcd, indent)
	}

	// Sanity check: GCD should be reasonable
	if gcd <= 0 || gcd > 8 {
		return mostCommonIndent
	}

	return gcd
}

// GetIndentSummary returns a summary string describing the indentation of a line.
//
// This function produces a human-readable description of the indentation style
// and level, useful for debugging and error messages.
//
// Parameters:
//   - line: The line content to describe
//   - spacesPerLevel: Expected number of spaces per indentation level
//
// Returns a descriptive string.
//
// Examples:
//   - GetIndentSummary("  key: value", 2) → "space indent, level 1 (2 spaces)"
//   - GetIndentSummary("\tkey: value", 2) → "tab indent, level 1 (1 tab)"
//   - GetIndentSummary("key: value", 2) → "no indent"
//   - GetIndentSummary("  \tkey: value", 2) → "mixed indent (invalid)"
func GetIndentSummary(line string, spacesPerLevel int) string {
	info := CalculateIndentation(line, spacesPerLevel)

	switch info.IndentType {
	case "none":
		return "no indent"
	case "space":
		return fmt.Sprintf("space indent, level %d (%d spaces)", info.Level, info.SpaceCount)
	case "tab":
		return fmt.Sprintf("tab indent, level %d (%d tab%s)", info.Level, info.TabCount, plural(info.TabCount))
	case "mixed":
		return "mixed indent (invalid)"
	default:
		return "unknown indent type"
	}
}

// plural returns "s" if count is not 1, for English pluralization.
func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// ScanLineTokens scans a line and returns information about its structure.
//
// This function performs a lexical analysis of a YAML line, identifying its
// constituent parts (indentation, key, colon, value, comment, etc.).
//
// Parameters:
//   - line: The line content to scan
//
// Returns a map of token information.
//
// Note: This is a basic scanner and doesn't handle all YAML edge cases.
// For full YAML parsing, use a proper YAML parser library.
func ScanLineTokens(line string) map[string]interface{} {
	tokens := make(map[string]interface{})

	// Extract indentation
	info := CalculateIndentation(line, 0)
	tokens["indent"] = map[string]int{
		"spaces":     info.SpaceCount,
		"tabs":       info.TabCount,
		"total":      info.TotalWidth,
		"is_mixed":   boolToInt(info.IsMixed),
		"indent_type": 0, // Will be set below
	}

	// Set indent type as integer for easier serialization
	switch info.IndentType {
	case "space":
		tokens["indent"].(map[string]int)["indent_type"] = 1
	case "tab":
		tokens["indent"].(map[string]int)["indent_type"] = 2
	case "mixed":
		tokens["indent"].(map[string]int)["indent_type"] = 3
	default:
		tokens["indent"].(map[string]int)["indent_type"] = 0
	}

	// Trim leading whitespace
	trimmed := strings.TrimLeft(line, " \t")

	// Check for blank line
	if len(trimmed) == 0 {
		tokens["is_blank"] = true
		return tokens
	}
	tokens["is_blank"] = false

	// Check for comment
	if trimmed[0] == '#' {
		tokens["is_comment"] = true
		tokens["comment"] = trimmed[1:]
		return tokens
	}
	tokens["is_comment"] = false

	// Check for document markers
	if strings.HasPrefix(trimmed, "---") {
		tokens["is_document_start"] = true
		return tokens
	}
	tokens["is_document_start"] = false

	if strings.HasPrefix(trimmed, "...") {
		tokens["is_document_end"] = true
		return tokens
	}
	tokens["is_document_end"] = false

	// Check for sequence item
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t") {
		tokens["is_sequence_item"] = true
		rest := trimmed[1:]
		trimmed = strings.TrimSpace(rest)
	} else {
		tokens["is_sequence_item"] = false
	}

	// Look for colon (key-value separator)
	colonPos := strings.Index(trimmed, ":")
	if colonPos > 0 {
		// Potential key-value pair
		potentialKey := trimmed[:colonPos]
		if isValidYAMLKey(potentialKey) {
			tokens["is_key_value"] = true
			tokens["key"] = potentialKey
			if colonPos+1 < len(trimmed) {
				valuePart := strings.TrimSpace(trimmed[colonPos+1:])
				tokens["value"] = valuePart
			} else {
				tokens["value"] = ""
			}
		} else {
			tokens["is_key_value"] = false
			tokens["content"] = trimmed
		}
	} else {
		tokens["is_key_value"] = false
		tokens["content"] = trimmed
	}

	// Check for trailing comment
	trailingComment := GetTrailingComment(line)
	if trailingComment != "" {
		tokens["has_trailing_comment"] = true
		tokens["trailing_comment"] = trailingComment
	} else {
		tokens["has_trailing_comment"] = false
	}

	return tokens
}

// boolToInt converts a boolean to an integer (1 for true, 0 for false).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isValidYAMLKey checks if a string appears to be a valid YAML key.
// This is a helper function for ScanLineTokens.
func isValidYAMLKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}

	// Check for quoted keys
	if len(trimmed) >= 2 {
		firstChar := trimmed[0]
		lastChar := trimmed[len(trimmed)-1]
		if (firstChar == '\'' && lastChar == '\'') ||
			(firstChar == '"' && lastChar == '"') {
			return true
		}
	}

	// Check for plain scalar keys
	for i, ch := range trimmed {
		if i == 0 {
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) &&
				ch != '_' && ch != '-' && ch != '.' {
				return false
			}
		}
	}

	return true
}
