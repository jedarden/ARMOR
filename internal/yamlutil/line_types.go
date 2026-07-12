// Package yamlutil provides YAML line parsing and key identification infrastructure.
//
// This file defines the core data structures for representing parsed YAML lines,
// including the LineType enum for categorizing different line types.
package yamlutil

import "fmt"

// LineType represents the classification of a YAML line.
//
// LineType provides structured categorization of different line types that can
// occur in YAML files. Each variant represents a specific semantic category
// recognized by the YAML specification.
type LineType int

const (
	// LineTypeBlank represents an empty line (only whitespace or no characters)
	LineTypeBlank LineType = iota

	// LineTypeComment represents a comment line (starts with #)
	LineTypeComment

	// LineTypeDocumentStart represents a document start marker (---)
	LineTypeDocumentStart

	// LineTypeDocumentEnd represents a document end marker (...)
	LineTypeDocumentEnd

	// LineTypeMappingKey represents a mapping key line (e.g., "key:" or "key: value")
	LineTypeMappingKey

	// LineTypeMappingValue represents a mapping value line (value after colon on same line)
	LineTypeMappingValue

	// LineTypeSequenceItem represents a sequence item line (starts with "- ")
	LineTypeSequenceItem

	// LineTypeFlowMapping represents a flow style mapping (starts with {)
	LineTypeFlowMapping

	// LineTypeFlowSequence represents a flow style sequence (starts with [)
	LineTypeFlowSequence

	// LineTypeAnchor represents an anchor definition (starts with &)
	LineTypeAnchor

	// LineTypeAlias represents an alias reference (starts with *)
	LineTypeAlias

	// LineTypeTag represents a tag directive (starts with !)
	LineTypeTag

	// LineTypeDirective represents a YAML directive (starts with %)
	LineTypeDirective

	// LineTypeLiteralBlockScalar represents a literal block scalar (starts with |)
	LineTypeLiteralBlockScalar

	// LineTypeFoldedBlockScalar represents a folded block scalar (starts with >)
	LineTypeFoldedBlockScalar

	// LineTypeUnknown represents unrecognized content (fallback for unknown patterns)
	LineTypeUnknown
)

// String returns a human-readable description of the line type.
func (lt LineType) String() string {
	switch lt {
	case LineTypeBlank:
		return "blank line"
	case LineTypeComment:
		return "comment line"
	case LineTypeDocumentStart:
		return "document start marker"
	case LineTypeDocumentEnd:
		return "document end marker"
	case LineTypeMappingKey:
		return "mapping key"
	case LineTypeMappingValue:
		return "mapping value"
	case LineTypeSequenceItem:
		return "sequence item"
	case LineTypeFlowMapping:
		return "flow style mapping"
	case LineTypeFlowSequence:
		return "flow style sequence"
	case LineTypeAnchor:
		return "anchor definition"
	case LineTypeAlias:
		return "alias reference"
	case LineTypeTag:
		return "tag directive"
	case LineTypeDirective:
		return "YAML directive"
	case LineTypeLiteralBlockScalar:
		return "literal block scalar"
	case LineTypeFoldedBlockScalar:
		return "folded block scalar"
	case LineTypeUnknown:
		return "unknown content"
	default:
		return "invalid line type"
	}
}

// Description returns a detailed description of the line type.
func (lt LineType) Description() string {
	return lt.String()
}

// IsStructural checks if this line type is a structural element.
//
// Structural elements are blank lines, document markers, and comments
// that don't contribute to the actual data content.
func (lt LineType) IsStructural() bool {
	switch lt {
	case LineTypeBlank, LineTypeDocumentStart, LineTypeDocumentEnd, LineTypeComment:
		return true
	default:
		return false
	}
}

// IsContent checks if this line type is meaningful content.
//
// Meaningful content includes keys, values, and data items.
func (lt LineType) IsContent() bool {
	switch lt {
	case LineTypeMappingKey, LineTypeMappingValue, LineTypeSequenceItem,
		LineTypeFlowMapping, LineTypeFlowSequence,
		LineTypeLiteralBlockScalar, LineTypeFoldedBlockScalar:
		return true
	default:
		return false
	}
}

// IsDirective checks if this line type is a YAML directive or special marker.
func (lt LineType) IsDirective() bool {
	switch lt {
	case LineTypeDirective, LineTypeTag, LineTypeAnchor, LineTypeAlias:
		return true
	default:
		return false
	}
}

// AllLineTypes returns all line type variants.
func AllLineTypes() []LineType {
	return []LineType{
		LineTypeBlank,
		LineTypeComment,
		LineTypeDocumentStart,
		LineTypeDocumentEnd,
		LineTypeMappingKey,
		LineTypeMappingValue,
		LineTypeSequenceItem,
		LineTypeFlowMapping,
		LineTypeFlowSequence,
		LineTypeAnchor,
		LineTypeAlias,
		LineTypeTag,
		LineTypeDirective,
		LineTypeLiteralBlockScalar,
		LineTypeFoldedBlockScalar,
		LineTypeUnknown,
	}
}

// LineContent represents structured content for different line types.
//
// LineContent provides typed access to the semantic content of a parsed line,
// allowing type-safe extraction of line-specific data.
type LineContent struct {
	// ContentType indicates which field contains valid data
	ContentType LineType

	// Comment content (text after #)
	Comment string

	// MappingKey content
	MappingKey struct {
		Key   string
		Value string // Empty string if no value on same line
	}

	// SequenceItem content
	SequenceItem string

	// FlowContent contains raw flow style content (e.g., "{key: value}" or "[item1, item2]")
	FlowContent string

	// Anchor content
	AnchorName string

	// Alias content
	AliasName string

	// Tag content
	Tag string

	// Directive content
	Directive struct {
		Type       string // e.g., "YAML", "TAG"
		Parameters string
	}

	// BlockScalar content
	BlockScalar struct {
		Indicator string // |, >, |-, >-, etc.
		Header    string // Optional header comment or content
	}

	// Unknown content
	Unknown string
}

// NewCommentContent creates a new LineContent for a comment.
func NewCommentContent(text string) LineContent {
	return LineContent{
		ContentType: LineTypeComment,
		Comment:     text,
	}
}

// NewMappingKeyContent creates a new LineContent for a mapping key.
func NewMappingKeyContent(key string, value string) LineContent {
	content := LineContent{
		ContentType: LineTypeMappingKey,
	}
	content.MappingKey.Key = key
	content.MappingKey.Value = value
	return content
}

// NewSequenceItemContent creates a new LineContent for a sequence item.
func NewSequenceItemContent(value string) LineContent {
	return LineContent{
		ContentType:  LineTypeSequenceItem,
		SequenceItem: value,
	}
}

// NewFlowContent creates a new LineContent for flow style content.
func NewFlowContent(content string) LineContent {
	return LineContent{
		ContentType:  LineTypeFlowMapping,
		FlowContent:  content,
	}
}

// NewAnchorContent creates a new LineContent for an anchor.
func NewAnchorContent(name string) LineContent {
	return LineContent{
		ContentType:  LineTypeAnchor,
		AnchorName:  name,
	}
}

// NewAliasContent creates a new LineContent for an alias.
func NewAliasContent(name string) LineContent {
	return LineContent{
		ContentType: LineTypeAlias,
		AliasName:   name,
	}
}

// NewTagContent creates a new LineContent for a tag.
func NewTagContent(tag string) LineContent {
	return LineContent{
		ContentType: LineTypeTag,
		Tag:        tag,
	}
}

// NewDirectiveContent creates a new LineContent for a directive.
func NewDirectiveContent(directiveType, parameters string) LineContent {
	content := LineContent{
		ContentType: LineTypeDirective,
	}
	content.Directive.Type = directiveType
	content.Directive.Parameters = parameters
	return content
}

// NewBlockScalarContent creates a new LineContent for a block scalar.
func NewBlockScalarContent(indicator, header string) LineContent {
	content := LineContent{
		ContentType: LineTypeLiteralBlockScalar,
	}
	content.BlockScalar.Indicator = indicator
	content.BlockScalar.Header = header
	return content
}

// NewUnknownContent creates a new LineContent for unknown content.
func NewUnknownContent(content string) LineContent {
	return LineContent{
		ContentType: LineTypeUnknown,
		Unknown:    content,
	}
}

// IsEmpty checks if this content is empty.
func (lc LineContent) IsEmpty() bool {
	return lc.ContentType == LineTypeBlank
}

// AsString returns the content as a string, if applicable.
func (lc LineContent) AsString() string {
	switch lc.ContentType {
	case LineTypeComment:
		return lc.Comment
	case LineTypeMappingKey:
		return lc.MappingKey.Key
	case LineTypeSequenceItem:
		return lc.SequenceItem
	case LineTypeFlowMapping, LineTypeFlowSequence:
		return lc.FlowContent
	case LineTypeAnchor:
		return lc.AnchorName
	case LineTypeAlias:
		return lc.AliasName
	case LineTypeTag:
		return lc.Tag
	case LineTypeDirective:
		return lc.Directive.Parameters
	case LineTypeLiteralBlockScalar, LineTypeFoldedBlockScalar:
		return lc.BlockScalar.Header
	default:
		return ""
	}
}

// String returns a string representation of the line content.
func (lc LineContent) String() string {
	switch lc.ContentType {
	case LineTypeBlank:
		return "<empty>"
	case LineTypeComment:
		return fmt.Sprintf("# %s", lc.Comment)
	case LineTypeMappingKey:
		if lc.MappingKey.Value != "" {
			return fmt.Sprintf("%s: %s", lc.MappingKey.Key, lc.MappingKey.Value)
		}
		return fmt.Sprintf("%s:", lc.MappingKey.Key)
	case LineTypeSequenceItem:
		return fmt.Sprintf("- %s", lc.SequenceItem)
	case LineTypeFlowMapping, LineTypeFlowSequence:
		return lc.FlowContent
	case LineTypeAnchor:
		return fmt.Sprintf("&%s", lc.AnchorName)
	case LineTypeAlias:
		return fmt.Sprintf("*%s", lc.AliasName)
	case LineTypeTag:
		return fmt.Sprintf("!%s", lc.Tag)
	case LineTypeDirective:
		return fmt.Sprintf("%%%s %s", lc.Directive.Type, lc.Directive.Parameters)
	case LineTypeLiteralBlockScalar, LineTypeFoldedBlockScalar:
		if lc.BlockScalar.Header != "" {
			return fmt.Sprintf("%s %s", lc.BlockScalar.Indicator, lc.BlockScalar.Header)
		}
		return lc.BlockScalar.Indicator
	case LineTypeUnknown:
		return fmt.Sprintf("<unknown: %s>", lc.Unknown)
	default:
		return "<invalid>"
	}
}

// YamlLine represents a single parsed YAML line.
//
// YamlLine contains all the information extracted from a single line of YAML
// content, including the raw text, line number, indentation level, and classified
// line type.
type YamlLine struct {
	// LineNumber is the 1-indexed line number in the source file
	LineNumber int

	// RawContent is the original, unmodified line content
	RawContent string

	// IndentationLevel is the number of leading whitespace characters
	IndentationLevel int

	// LineType is the classified type of this line
	LineType LineType

	// Content is the structured content representation (optional)
	Content LineContent
}

// NewYamlLine creates a new YAML line with the specified properties.
//
// Parameters:
//   - lineNumber: 1-indexed line number in the source file
//   - rawContent: The original line content
//   - indentationLevel: Number of leading whitespace characters
//   - lineType: The classified type of this line
//
// Returns a new YamlLine instance.
func NewYamlLine(lineNumber int, rawContent string, indentationLevel int, lineType LineType) YamlLine {
	return YamlLine{
		LineNumber:        lineNumber,
		RawContent:       rawContent,
		IndentationLevel: indentationLevel,
		LineType:         lineType,
		Content:          LineContent{ContentType: LineTypeBlank},
	}
}

// NewYamlLineWithContent creates a new YAML line with structured content.
//
// Parameters:
//   - lineNumber: 1-indexed line number in the source file
//   - rawContent: The original line content
//   - indentationLevel: Number of leading whitespace characters
//   - lineType: The classified type of this line
//   - content: Structured content representation
//
// Returns a new YamlLine instance with content.
func NewYamlLineWithContent(lineNumber int, rawContent string, indentationLevel int, lineType LineType, content LineContent) YamlLine {
	return YamlLine{
		LineNumber:        lineNumber,
		RawContent:       rawContent,
		IndentationLevel: indentationLevel,
		LineType:         lineType,
		Content:          content,
	}
}

// IsBlank checks if this line is blank.
func (yl YamlLine) IsBlank() bool {
	return yl.LineType == LineTypeBlank
}

// IsComment checks if this line is a comment.
func (yl YamlLine) IsComment() bool {
	return yl.LineType == LineTypeComment
}

// IsMappingKey checks if this line is a mapping key.
func (yl YamlLine) IsMappingKey() bool {
	return yl.LineType == LineTypeMappingKey
}

// IsSequenceItem checks if this line is a sequence item.
func (yl YamlLine) IsSequenceItem() bool {
	return yl.LineType == LineTypeSequenceItem
}

// IsContent checks if this line is meaningful content (not structural).
func (yl YamlLine) IsContent() bool {
	return yl.LineType.IsContent()
}

// IsStructural checks if this line is structural (blank, comment, document marker).
func (yl YamlLine) IsStructural() bool {
	return yl.LineType.IsStructural()
}

// Trimmed returns the trimmed content (without leading/trailing whitespace).
func (yl YamlLine) Trimmed() string {
	// Simple trim - would use strings.TrimSpace in actual usage
	start, end := 0, len(yl.RawContent)
	for start < end && (yl.RawContent[start] == ' ' || yl.RawContent[start] == '\t' || yl.RawContent[start] == '\n' || yl.RawContent[start] == '\r') {
		start++
	}
	for end > start && (yl.RawContent[end-1] == ' ' || yl.RawContent[end-1] == '\t' || yl.RawContent[end-1] == '\n' || yl.RawContent[end-1] == '\r') {
		end--
	}
	return yl.RawContent[start:end]
}

// Indentation returns the indentation substring (leading whitespace characters).
func (yl YamlLine) Indentation() string {
	if yl.IndentationLevel <= 0 {
		return ""
	}
	if yl.IndentationLevel > len(yl.RawContent) {
		return yl.RawContent
	}
	return yl.RawContent[:yl.IndentationLevel]
}

// String returns a string representation of the YAML line.
func (yl YamlLine) String() string {
	return fmt.Sprintf("Line %d: %s (indent: %d, type: %s)",
		yl.LineNumber, yl.RawContent, yl.IndentationLevel, yl.LineType)
}

// LineParseResult represents the result of parsing a YAML file into lines.
//
// LineParseResult contains the complete results of a line-by-line YAML parsing operation,
// including all parsed lines and summary statistics.
type LineParseResult struct {
	// Lines contains all parsed lines in order
	Lines []YamlLine

	// TotalLines is the total number of lines processed
	TotalLines int

	// BlankLines is the number of blank lines found
	BlankLines int

	// CommentLines is the number of comment lines found
	CommentLines int

	// ContentLines is the number of content lines found
	ContentLines int

	// MaxIndentation is the maximum indentation level found
	MaxIndentation int
}

// NewLineParseResult creates a new line parse result from a vector of lines.
//
// Parameters:
//   - lines: The parsed YAML lines
//
// Returns a new LineParseResult with computed statistics.
func NewLineParseResult(lines []YamlLine) LineParseResult {
	result := LineParseResult{
		Lines:      lines,
		TotalLines: len(lines),
	}

	// Compute statistics
	for _, line := range lines {
		if line.IsBlank() {
			result.BlankLines++
		}
		if line.IsComment() {
			result.CommentLines++
		}
		if line.IsContent() {
			result.ContentLines++
		}
		if line.IndentationLevel > result.MaxIndentation {
			result.MaxIndentation = line.IndentationLevel
		}
	}

	return result
}

// IsEmpty checks if the parse result is empty.
func (lpr LineParseResult) IsEmpty() bool {
	return len(lpr.Lines) == 0
}

// LinesByType returns lines matching the specified type.
//
// Parameters:
//   - lineType: The line type to filter by
//
// Returns a slice of lines matching the specified type.
func (lpr LineParseResult) LinesByType(lineType LineType) []YamlLine {
	var matching []YamlLine
	for _, line := range lpr.Lines {
		if line.LineType == lineType {
			matching = append(matching, line)
		}
	}
	return matching
}

// ContentLinesIter returns an iterator over content lines (non-structural).
func (lpr LineParseResult) ContentLinesIter() []YamlLine {
	var content []YamlLine
	for _, line := range lpr.Lines {
		if line.IsContent() {
			content = append(content, line)
		}
	}
	return content
}

// StructuralLinesIter returns an iterator over structural lines.
func (lpr LineParseResult) StructuralLinesIter() []YamlLine {
	var structural []YamlLine
	for _, line := range lpr.Lines {
		if line.IsStructural() {
			structural = append(structural, line)
		}
	}
	return structural
}
