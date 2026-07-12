// Package yamlutil provides YAML line parsing and key identification infrastructure.
//
// The line parser processes YAML content line-by-line to identify potential mapping
// keys and track their line numbers, providing foundational parsing infrastructure
// for syntax validation and error detection.
package yamlutil

import (
	"strings"
	"unicode"
)

// ParsedLine represents a single line of YAML with parsed metadata.
//
// ParsedLine captures the essential information about each YAML line needed
// for key identification and syntax validation, including the original content,
// indentation level, and whether it appears to be a mapping key.
type ParsedLine struct {
	LineNumber      int    // Line number (1-indexed)
	OriginalContent string // Original line content
	TrimmedContent  string // Line content with leading/trailing whitespace removed
	Indentation     int    // Number of leading whitespace characters
	IndentType      string // "space", "tab", or "mixed"
	IsKeyCandidate  bool   // Whether this line appears to contain a mapping key
	KeyName         string // Extracted key name (if IsKeyCandidate is true)
	HasColon        bool   // Whether line contains a colon (mapping delimiter)
	IsEmpty         bool   // Whether line is empty or whitespace-only
	IsComment       bool   // Whether line is a comment
	IsSequenceItem  bool   // Whether line starts a sequence item (- )
	IsDocumentStart bool   // Whether line is a document start marker (---)
	IsDocumentEnd   bool   // Whether line is a document end marker (...)
	InFlowStyle     bool   // Whether line appears to use flow style ({} or [])
}

// LineParserResult represents the result of parsing YAML content line by line.
//
// LineParserResult contains all parsed lines along with summary statistics
// and metadata about the YAML structure.
type LineParserResult struct {
	Lines           []ParsedLine // All parsed lines
	TotalLines      int          // Total number of lines
	EmptyLines      int          // Number of empty/whitespace-only lines
	CommentLines    int          // Number of comment lines
	KeyCandidates   int          // Number of potential key lines
	SequenceItems   int          // Number of sequence item lines
	IndentSpaces    int          // Expected spaces per indent level (detected)
	IndentTabs      bool         // Whether tabs were detected
	HasMixedIndent  bool         // Whether mixed tabs/spaces were detected
	MaxIndentLevel  int          // Maximum indentation level detected
	indentNormalized bool        // Whether Indentation field is normalized to level numbers
}

// LineParser parses YAML content line by line.
//
// LineParser implements the foundational parsing infrastructure for YAML syntax
// validation, processing content line-by-line to identify keys, track indentation,
// and detect structural patterns.
type LineParser struct {
	indentSpaces int // Expected spaces per indentation level (0 = auto-detect)
}

// NewLineParser creates a new line parser.
//
// Parameters:
//   - indentSpaces: Expected number of spaces per indentation level (0 for auto-detect)
//
// Returns a LineParser ready to process YAML content.
func NewLineParser(indentSpaces int) *LineParser {
	return &LineParser{
		indentSpaces: indentSpaces,
	}
}

// Parse parses YAML content line by line.
//
// Parse processes the YAML content and returns detailed information about each line,
// including indentation levels, potential keys, and structural patterns.
//
// Parameters:
//   - content: The YAML content to parse
//
// Returns a LineParserResult with all parsed lines and metadata.
func (lp *LineParser) Parse(content string) LineParserResult {
	lines := strings.Split(content, "\n")

	result := LineParserResult{
		Lines:      make([]ParsedLine, len(lines)),
		TotalLines: len(lines),
	}

	// Auto-detect indentation if not specified
	if lp.indentSpaces == 0 {
		lp.indentSpaces = lp.detectIndentation(content)
	}
	result.IndentSpaces = lp.indentSpaces

	maxIndent := 0
	hasSpaceIndent := false
	hasTabIndent := false

	// Parse each line
	for lineNum, originalLine := range lines {
		parsedLine := lp.parseLine(lineNum+1, originalLine)
		result.Lines[lineNum] = parsedLine

		// Update statistics
		if parsedLine.IsEmpty {
			result.EmptyLines++
		}
		if parsedLine.IsComment {
			result.CommentLines++
		}
		if parsedLine.IsKeyCandidate {
			result.KeyCandidates++
		}
		if parsedLine.IsSequenceItem {
			result.SequenceItems++
		}

		// Track indentation types across document
		if parsedLine.IndentType == "space" && parsedLine.Indentation > 0 {
			hasSpaceIndent = true
		}
		if parsedLine.IndentType == "tab" && parsedLine.Indentation > 0 {
			hasTabIndent = true
		}
		if parsedLine.IndentType == "mixed" {
			result.HasMixedIndent = true
		}

		// Track maximum indent level
		if parsedLine.Indentation > maxIndent && !parsedLine.IsEmpty && !parsedLine.IsComment {
			maxIndent = parsedLine.Indentation
		}
	}

	// Set overall flags
	result.IndentTabs = hasTabIndent
	if hasSpaceIndent && hasTabIndent {
		result.HasMixedIndent = true
	}

	// Calculate maximum indent level
	if result.IndentSpaces > 0 {
		result.MaxIndentLevel = maxIndent / result.IndentSpaces
	}

	return result
}

// calculateIndentation calculates the indentation level of a YAML line.
//
// This function counts leading spaces and tabs to determine indentation depth.
// Tabs are counted as single characters, not expanded to spaces.
//
// Parameters:
//   - line: The line content to analyze
//
// Returns the number of leading whitespace characters (spaces + tabs).
// Returns 0 for lines with no leading whitespace.
func calculateIndentation(line string) int {
	trimmed := strings.TrimLeft(line, " \t")
	return len(line) - len(trimmed)
}

// parseLine parses a single line of YAML.
//
// Parameters:
//   - lineNum: Line number (1-indexed)
//   - originalLine: Original line content
//
// Returns a ParsedLine with metadata about the line.
func (lp *LineParser) parseLine(lineNum int, originalLine string) ParsedLine {
	line := originalLine
	trimmed := strings.TrimLeft(line, " \t")

	parsedLine := ParsedLine{
		LineNumber:      lineNum,
		OriginalContent: line,
		TrimmedContent:  strings.TrimSpace(trimmed),
		Indentation:     calculateIndentation(line),
	}

	// Detect indentation type
	parsedLine.IndentType = lp.detectIndentType(line)

	// Check for empty lines
	if parsedLine.TrimmedContent == "" {
		parsedLine.IsEmpty = true
		return parsedLine
	}

	// Check for comments
	if strings.HasPrefix(trimmed, "#") {
		parsedLine.IsComment = true
		return parsedLine
	}

	// Check for document markers
	if strings.HasPrefix(trimmed, "---") {
		parsedLine.IsDocumentStart = true
		// Document markers can also be key candidates in some contexts
		if len(trimmed) > 3 && trimmed[3] != ' ' && trimmed[3] != '\t' {
			parsedLine.IsKeyCandidate = true
			parsedLine.KeyName = lp.extractKeyName(trimmed)
		}
		return parsedLine
	}

	if strings.HasPrefix(trimmed, "...") {
		parsedLine.IsDocumentEnd = true
		return parsedLine
	}

	// Check for sequence items
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t") {
		parsedLine.IsSequenceItem = true

		// After the dash, the rest might be a key-value pair
		afterDash := strings.TrimSpace(trimmed[1:])
		if afterDash != "" {
			parsedLine.TrimmedContent = afterDash
			parsedLine.HasColon = strings.Contains(afterDash, ":")

			// Check if this looks like a nested mapping
			if parsedLine.HasColon {
				parsedLine.IsKeyCandidate = true
				parsedLine.KeyName = lp.extractKeyName(afterDash)
			}
		}
		return parsedLine
	}

	// Check for flow style
	if strings.Contains(trimmed, "{") || strings.Contains(trimmed, "}") ||
	   strings.Contains(trimmed, "[") || strings.Contains(trimmed, "]") {
		parsedLine.InFlowStyle = true
	}

	// Check for colon (mapping key indicator)
	parsedLine.HasColon = strings.Contains(trimmed, ":")

	// Identify key candidates
	if parsedLine.HasColon && !parsedLine.InFlowStyle {
		parsedLine.IsKeyCandidate = lp.isKeyCandidate(trimmed)
		if parsedLine.IsKeyCandidate {
			parsedLine.KeyName = lp.extractKeyName(trimmed)
		}
	}

	return parsedLine
}

// detectIndentation detects the indentation style from YAML content.
//
// Parameters:
//   - content: The YAML content to analyze
//
// Returns the detected number of spaces per indentation level.
func (lp *LineParser) detectIndentation(content string) int {
	lines := strings.Split(content, "\n")

	// Track indentation levels
	indentLevels := make(map[int]bool)

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		leadingSpaces := 0
		for _, ch := range line {
			if ch == ' ' {
				leadingSpaces++
			} else if ch == '\t' {
				// Assume tabs = don't count for space-based detection
				break
			} else {
				break
			}
		}

		if leadingSpaces > 0 {
			indentLevels[leadingSpaces] = true
		}
	}

	// If no indented lines found, default to 2
	if len(indentLevels) == 0 {
		return 2
	}

	// Find the greatest common divisor (GCD) of all indentation levels
	levels := make([]int, 0, len(indentLevels))
	for level := range indentLevels {
		levels = append(levels, level)
	}

	// Start with the smallest level as candidate
	gcd := levels[0]
	for _, level := range levels[1:] {
		gcd = computeGCD(gcd, level)
	}

	// Sanity check: GCD should be 1, 2, or 4 typically
	if gcd == 0 || gcd > 8 {
		return 2
	}

	return gcd
}

// computeGCD computes the greatest common divisor of two numbers.
func computeGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// detectIndentType detects the type of indentation used in a line.
//
// Parameters:
//   - line: The line content to analyze
//
// Returns "space", "tab", "mixed", or "" (no indentation).
func (lp *LineParser) detectIndentType(line string) string {
	hasSpaces := false
	hasTabs := false
	spaceCount := 0
	tabCount := 0

	for _, ch := range line {
		if ch == ' ' {
			hasSpaces = true
			spaceCount++
		} else if ch == '\t' {
			hasTabs = true
			tabCount++
		} else {
			break // Stop at first non-whitespace character
		}
	}

	// If we have both tabs AND spaces in the leading whitespace, it's mixed
	// But we need to check the actual order - tabs and spaces shouldn't be mixed in the indent
	if hasSpaces && hasTabs {
		// Check if they're truly mixed (both appear in indent portion)
		if spaceCount > 0 && tabCount > 0 {
			return "mixed"
		}
	}

	if hasTabs {
		return "tab"
	}
	if hasSpaces {
		return "space"
	}

	return "" // No indentation
}

// isKeyCandidate determines if a trimmed line appears to be a mapping key.
//
// Parameters:
//   - trimmed: Trimmed line content
//
// Returns true if the line appears to be a mapping key.
func (lp *LineParser) isKeyCandidate(trimmed string) bool {
	if trimmed == "" {
		return false
	}

	// Must contain a colon
	if !strings.Contains(trimmed, ":") {
		return false
	}

	// Check if the colon is at a valid position (after the key)
	colonPos := strings.Index(trimmed, ":")
	if colonPos <= 0 {
		return false
	}

	// Extract potential key
	potentialKey := trimmed[:colonPos]

	// Key must not be empty
	if strings.TrimSpace(potentialKey) == "" {
		return false
	}

	// Check if key looks valid (alphanumeric, underscore, hyphen, dot, or quoted)
	return lp.isValidKey(potentialKey)
}

// extractKeyName extracts the key name from a line.
//
// Parameters:
//   - trimmed: Trimmed line content
//
// Returns the extracted key name.
func (lp *LineParser) extractKeyName(trimmed string) string {
	if trimmed == "" {
		return ""
	}

	colonPos := strings.Index(trimmed, ":")
	if colonPos <= 0 {
		return ""
	}

	potentialKey := trimmed[:colonPos]

	// Remove quotes if present
	if len(potentialKey) >= 2 {
		firstChar := potentialKey[0]
		lastChar := potentialKey[len(potentialKey)-1]

		if (firstChar == '\'' && lastChar == '\'') ||
		   (firstChar == '"' && lastChar == '"') {
			return potentialKey[1 : len(potentialKey)-1]
		}
	}

	return strings.TrimSpace(potentialKey)
}

// isValidKey checks if a string appears to be a valid YAML key.
//
// Parameters:
//   - key: The potential key string
//
// Returns true if the key appears to be valid.
func (lp *LineParser) isValidKey(key string) bool {
	trimmedKey := strings.TrimSpace(key)

	if trimmedKey == "" {
		return false
	}

	// Check for quoted keys
	if len(trimmedKey) >= 2 {
		firstChar := trimmedKey[0]
		lastChar := trimmedKey[len(trimmedKey)-1]

		if (firstChar == '\'' && lastChar == '\'') ||
		   (firstChar == '"' && lastChar == '"') {
			return true
		}
	}

	// Check for plain scalar keys
	// Valid YAML keys can contain letters, digits, underscores, hyphens, and dots
	for i, ch := range trimmedKey {
		if i == 0 {
			// First character can be letter, digit, or underscore
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
				return false
			}
		} else {
			// Subsequent characters can also include hyphens and dots
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) &&
			   ch != '_' && ch != '-' && ch != '.' {
				return false
			}
		}
	}

	return true
}

// KeyIdentifier identifies mapping keys from parsed lines.
//
// KeyIdentifier processes parsed lines and identifies which lines represent
// mapping keys, providing a structured view of the YAML key hierarchy.
type KeyIdentifier struct {
	parser *LineParser // Underlying line parser
}

// NewKeyIdentifier creates a new key identifier.
//
// Parameters:
//   - indentSpaces: Expected number of spaces per indentation level (0 for auto-detect)
//
// Returns a KeyIdentifier ready to identify keys in YAML content.
func NewKeyIdentifier(indentSpaces int) *KeyIdentifier {
	return &KeyIdentifier{
		parser: NewLineParser(indentSpaces),
	}
}

// IdentifyKeys identifies all mapping keys in YAML content.
//
// IdentifyKeys parses the YAML content and extracts all mapping keys with their
// line numbers and indentation levels, providing a structured view of the key hierarchy.
//
// Parameters:
//   - content: The YAML content to analyze
//
// Returns a LineParserResult with key identification metadata.
func (ki *KeyIdentifier) IdentifyKeys(content string) LineParserResult {
	result := ki.parser.Parse(content)

	// Post-process to identify key hierarchy
	for i := range result.Lines {
		if result.Lines[i].IsKeyCandidate {
			// Calculate the indentation level of this key
			if result.IndentSpaces > 0 {
				result.Lines[i].Indentation = result.Lines[i].Indentation / result.IndentSpaces
			}
		}
	}

	result.indentNormalized = true
	return result
}

// GetKeyLines returns only the lines that contain mapping keys.
//
// Parameters:
//   - result: A LineParserResult from Parse or IdentifyKeys
//
// Returns a slice of ParsedLine entries that are key candidates.
func (ki *KeyIdentifier) GetKeyLines(result LineParserResult) []ParsedLine {
	keyLines := make([]ParsedLine, 0, result.KeyCandidates)

	for _, line := range result.Lines {
		if line.IsKeyCandidate {
			keyLines = append(keyLines, line)
		}
	}

	return keyLines
}

// GetKeyHierarchy returns the keys organized by indentation level.
//
// Parameters:
//   - result: A LineParserResult from Parse or IdentifyKeys
//
// Returns a map where keys are indentation levels and values are slices of keys at that level.
//
// Note: If result comes from IdentifyKeys(), the Indentation field is already converted
// to level number (0, 1, 2, etc.), so we use it directly. If it comes from Parse(),
// the Indentation field contains the raw space count, so we divide by IndentSpaces.
func (ki *KeyIdentifier) GetKeyHierarchy(result LineParserResult) map[int][]ParsedLine {
	hierarchy := make(map[int][]ParsedLine)

	for _, line := range result.Lines {
		if line.IsKeyCandidate {
			indentLevel := line.Indentation

			// Only convert if indent is NOT already normalized
			if !result.indentNormalized && result.IndentSpaces > 0 {
				// This is raw space count, convert to level
				indentLevel = indentLevel / result.IndentSpaces
			}

			// Debug: ensure we're using the correct level
			_ = indentLevel // Use indentLevel to build hierarchy
			hierarchy[indentLevel] = append(hierarchy[indentLevel], line)
		}
	}

	return hierarchy
}
