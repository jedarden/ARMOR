//! YAML Line Parser Module
//!
//! This module provides the core data structures for representing parsed YAML lines.
//! It defines types that hold line-by-line parsing results, including line type,
//! indentation level, and raw content.
//!
//! ## Architecture
//!
//! The line parser module provides the foundational types for line-based YAML analysis:
//!
//! - [`LineType`] - Enumeration of all possible YAML line types
//! - [`YamlLine`] - Struct representing a single parsed YAML line
//! - [`LineContent`] - Structured content representation for different line types
//!
//! ## Usage
//!
//! ```ignore
//! use armor::parsers::yaml::line_parser::{YamlLine, LineType};
//!
//! let line = YamlLine::new(1, "key: value", 0, LineType::MappingKey);
//! assert_eq!(line.line_number(), 1);
//! assert_eq!(line.raw_content(), "key: value");
//! ```
//!
//! ## Line Type Categories
//!
//! - **Structural**: Blank lines, document markers
//! - **Comments**: Full and inline comments
//! - **Content**: Mapping keys, values, sequence items
//! - **Special**: Anchors, aliases, tags, directives

use std::fmt;

/// Classification of YAML line types
///
/// This enum provides structured categorization of different line types
/// that can occur in YAML files. Each variant represents a specific semantic
/// category recognized by the YAML specification.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LineType {
    /// Empty line (only whitespace or no characters)
    Blank,

    /// Comment line (starts with #)
    Comment,

    /// Document start marker (---)
    DocumentStart,

    /// Document end marker (...)
    DocumentEnd,

    /// Mapping key line (e.g., "key:" or "key: value")
    MappingKey,

    /// Mapping value line (value after colon on same line)
    MappingValue,

    /// Sequence item line (starts with "- ")
    SequenceItem,

    /// Flow style mapping (starts with {)
    FlowMapping,

    /// Flow style sequence (starts with [)
    FlowSequence,

    /// Anchor definition (starts with &)
    Anchor,

    /// Alias reference (starts with *)
    Alias,

    /// Tag directive (starts with !)
    Tag,

    /// YAML directive (starts with %)
    Directive,

    /// Literal block scalar (starts with |)
    LiteralBlockScalar,

    /// Folded block scalar (starts with >)
    FoldedBlockScalar,

    /// Unrecognized content (fallback for unknown patterns)
    Unknown,
}

impl LineType {
    /// Get a human-readable description of this line type
    pub fn description(&self) -> &'static str {
        match self {
            Self::Blank => "blank line",
            Self::Comment => "comment line",
            Self::DocumentStart => "document start marker",
            Self::DocumentEnd => "document end marker",
            Self::MappingKey => "mapping key",
            Self::MappingValue => "mapping value",
            Self::SequenceItem => "sequence item",
            Self::FlowMapping => "flow style mapping",
            Self::FlowSequence => "flow style sequence",
            Self::Anchor => "anchor definition",
            Self::Alias => "alias reference",
            Self::Tag => "tag directive",
            Self::Directive => "YAML directive",
            Self::LiteralBlockScalar => "literal block scalar",
            Self::FoldedBlockScalar => "folded block scalar",
            Self::Unknown => "unknown content",
        }
    }

    /// Check if this line type is a structural element
    ///
    /// Structural elements are blank lines, document markers, and comments
    /// that don't contribute to the actual data content.
    pub fn is_structural(&self) -> bool {
        matches!(
            self,
            Self::Blank | Self::DocumentStart | Self::DocumentEnd | Self::Comment
        )
    }

    /// Check if this line type is meaningful content
    ///
    /// Meaningful content includes keys, values, and data items.
    pub fn is_content(&self) -> bool {
        matches!(
            self,
            Self::MappingKey |
            Self::MappingValue |
            Self::SequenceItem |
            Self::FlowMapping |
            Self::FlowSequence |
            Self::LiteralBlockScalar |
            Self::FoldedBlockScalar
        )
    }

    /// Check if this line type is a YAML directive or special marker
    pub fn is_directive(&self) -> bool {
        matches!(self, Self::Directive | Self::Tag | Self::Anchor | Self::Alias)
    }

    /// Get all line type variants
    pub fn all() -> &'static [Self] {
        &[
            Self::Blank,
            Self::Comment,
            Self::DocumentStart,
            Self::DocumentEnd,
            Self::MappingKey,
            Self::MappingValue,
            Self::SequenceItem,
            Self::FlowMapping,
            Self::FlowSequence,
            Self::Anchor,
            Self::Alias,
            Self::Tag,
            Self::Directive,
            Self::LiteralBlockScalar,
            Self::FoldedBlockScalar,
            Self::Unknown,
        ]
    }
}

impl fmt::Display for LineType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.description())
    }
}

/// Structured content representation for different line types
///
/// This enum provides typed access to the semantic content of a parsed line,
/// allowing type-safe extraction of line-specific data.
#[derive(Debug, Clone)]
pub enum LineContent {
    /// No content (blank lines, document markers)
    Empty,

    /// Comment content (text after #)
    Comment(String),

    /// Mapping key content (key name and optional value)
    MappingKey {
        /// The key name
        key: String,
        /// The value, if present on the same line
        value: Option<String>,
    },

    /// Sequence item content
    SequenceItem {
        /// The item value (after the "- " prefix)
        value: String,
    },

    /// Flow style content
    FlowContent {
        /// The raw flow content (e.g., "{key: value}" or "[item1, item2]")
        content: String,
    },

    /// Anchor definition
    Anchor {
        /// The anchor name
        name: String,
    },

    /// Alias reference
    Alias {
        /// The alias name
        name: String,
    },

    /// Tag directive
    Tag {
        /// The tag identifier
        tag: String,
    },

    /// Directive content
    Directive {
        /// The directive type (e.g., "YAML", "TAG")
        directive_type: String,
        /// The directive parameters
        parameters: String,
    },

    /// Block scalar content
    BlockScalar {
        /// The block type indicator (|, >, |-, >-, etc.)
        indicator: String,
        /// The optional header comment or content
        header: Option<String>,
    },

    /// Unknown content
    Unknown(String),
}

impl LineContent {
    /// Check if this content is empty
    pub fn is_empty(&self) -> bool {
        matches!(self, Self::Empty)
    }

    /// Get the content as a string slice, if applicable
    pub fn as_str(&self) -> Option<&str> {
        match self {
            Self::Comment(s) => Some(s),
            Self::MappingKey { key, .. } => Some(key),
            Self::SequenceItem { value } => Some(value),
            Self::FlowContent { content } => Some(content),
            Self::Anchor { name } => Some(name),
            Self::Alias { name } => Some(name),
            Self::Tag { tag } => Some(tag),
            Self::Directive { parameters, .. } => Some(parameters),
            Self::BlockScalar { header, .. } => header.as_deref(),
            Self::Empty | Self::Unknown(_) => None,
        }
    }
}

impl fmt::Display for LineContent {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Empty => write!(f, "<empty>"),
            Self::Comment(s) => write!(f, "# {}", s),
            Self::MappingKey { key, value } => {
                if let Some(v) = value {
                    write!(f, "{}: {}", key, v)
                } else {
                    write!(f, "{}:", key)
                }
            }
            Self::SequenceItem { value } => write!(f, "- {}", value),
            Self::FlowContent { content } => write!(f, "{}", content),
            Self::Anchor { name } => write!(f, "&{}", name),
            Self::Alias { name } => write!(f, "*{}", name),
            Self::Tag { tag } => write!(f, "!{}", tag),
            Self::Directive { directive_type, parameters } => {
                write!(f, "%{} {}", directive_type, parameters)
            }
            Self::BlockScalar { indicator, header } => {
                write!(f, "{}", indicator)?;
                if let Some(h) = header {
                    write!(f, " {}", h)
                } else {
                    Ok(())
                }
            }
            Self::Unknown(s) => write!(f, "<unknown: {}>", s),
        }
    }
}

/// Represents a single parsed YAML line
///
/// `YamlLine` contains all the information extracted from a single line of YAML
/// content, including the raw text, line number, indentation level, and classified
/// line type.
///
/// # Fields
///
/// - `line_number` - 1-indexed line number in the source file
/// - `raw_content` - The original, unmodified line content
/// - `indentation_level` - Number of leading whitespace characters (spaces/tabs)
/// - `line_type` - The classified type of this line
/// - `content` - Structured content representation (optional)
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::{YamlLine, LineType};
///
/// // Create a mapping key line
/// let line = YamlLine::new(1, "name: John", 0, LineType::MappingKey);
/// assert_eq!(line.line_number(), 1);
/// assert_eq!(line.raw_content(), "name: John");
/// assert_eq!(line.indentation_level(), 0);
/// assert_eq!(line.line_type(), LineType::MappingKey);
/// ```
#[derive(Debug, Clone)]
pub struct YamlLine {
    /// 1-indexed line number in the source file
    line_number: usize,

    /// The original, unmodified line content
    raw_content: String,

    /// Number of leading whitespace characters
    indentation_level: usize,

    /// The classified type of this line
    line_type: LineType,

    /// Structured content representation
    content: Option<LineContent>,
}

impl YamlLine {
    /// Create a new YAML line with the specified properties
    ///
    /// # Arguments
    ///
    /// * `line_number` - 1-indexed line number in the source file
    /// * `raw_content` - The original line content
    /// * `indentation_level` - Number of leading whitespace characters
    /// * `line_type` - The classified type of this line
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::line_parser::{YamlLine, LineType};
    ///
    /// let line = YamlLine::new(5, "  key: value", 2, LineType::MappingKey);
    /// assert_eq!(line.line_number(), 5);
    /// ```
    pub fn new(line_number: usize, raw_content: impl Into<String>, indentation_level: usize, line_type: LineType) -> Self {
        Self {
            line_number,
            raw_content: raw_content.into(),
            indentation_level,
            line_type,
            content: None,
        }
    }

    /// Create a new YAML line with structured content
    ///
    /// # Arguments
    ///
    /// * `line_number` - 1-indexed line number in the source file
    /// * `raw_content` - The original line content
    /// * `indentation_level` - Number of leading whitespace characters
    /// * `line_type` - The classified type of this line
    /// * `content` - Structured content representation
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::line_parser::{YamlLine, LineType, LineContent};
    ///
    /// let content = LineContent::MappingKey {
    ///     key: "name".to_string(),
    ///     value: Some("John".to_string()),
    /// };
    /// let line = YamlLine::with_content(1, "name: John", 0, LineType::MappingKey, content);
    /// ```
    pub fn with_content(
        line_number: usize,
        raw_content: impl Into<String>,
        indentation_level: usize,
        line_type: LineType,
        content: LineContent,
    ) -> Self {
        Self {
            line_number,
            raw_content: raw_content.into(),
            indentation_level,
            line_type,
            content: Some(content),
        }
    }

    /// Get the line number (1-indexed)
    ///
    /// # Returns
    /// The line number in the source file (1-indexed)
    pub fn line_number(&self) -> usize {
        self.line_number
    }

    /// Get the raw content of the line
    ///
    /// # Returns
    /// The original, unmodified line content
    pub fn raw_content(&self) -> &str {
        &self.raw_content
    }

    /// Get the indentation level
    ///
    /// # Returns
    /// The number of leading whitespace characters
    pub fn indentation_level(&self) -> usize {
        self.indentation_level
    }

    /// Get the line type
    ///
    /// # Returns
    /// The classified type of this line
    pub fn line_type(&self) -> LineType {
        self.line_type
    }

    /// Get the structured content, if available
    ///
    /// # Returns
    /// An optional reference to the structured content
    pub fn content(&self) -> Option<&LineContent> {
        self.content.as_ref()
    }

    /// Check if this line is blank
    ///
    /// # Returns
    /// `true` if the line type is `Blank`
    pub fn is_blank(&self) -> bool {
        self.line_type == LineType::Blank
    }

    /// Check if this line is a comment
    ///
    /// # Returns
    /// `true` if the line type is `Comment`
    pub fn is_comment(&self) -> bool {
        self.line_type == LineType::Comment
    }

    /// Check if this line is a mapping key
    ///
    /// # Returns
    /// `true` if the line type is `MappingKey`
    pub fn is_mapping_key(&self) -> bool {
        self.line_type == LineType::MappingKey
    }

    /// Check if this line is a sequence item
    ///
    /// # Returns
    /// `true` if the line type is `SequenceItem`
    pub fn is_sequence_item(&self) -> bool {
        self.line_type == LineType::SequenceItem
    }

    /// Check if this line is meaningful content (not structural)
    ///
    /// # Returns
    /// `true` if the line type represents data content
    pub fn is_content(&self) -> bool {
        self.line_type.is_content()
    }

    /// Check if this line is structural (blank, comment, document marker)
    ///
    /// # Returns
    /// `true` if the line type is structural
    pub fn is_structural(&self) -> bool {
        self.line_type.is_structural()
    }

    /// Get the trimmed content (without leading/trailing whitespace)
    ///
    /// # Returns
    /// The line content with leading and trailing whitespace removed
    pub fn trimmed(&self) -> &str {
        self.raw_content.trim()
    }

    /// Get the indentation substring
    ///
    /// # Returns
    /// The leading whitespace characters
    pub fn indentation(&self) -> &str {
        &self.raw_content[..self.indentation_level]
    }
}

impl fmt::Display for YamlLine {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Line {}: {} (indent: {}, type: {})",
            self.line_number,
            self.raw_content,
            self.indentation_level,
            self.line_type
        )
    }
}

/// Indentation details for a YAML line
///
/// This struct provides detailed information about the indentation of a line,
/// including separate counts for spaces and tabs, and the total indentation level.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct IndentationInfo {
    /// Number of leading spaces
    pub leading_spaces: usize,
    /// Number of leading tabs
    pub leading_tabs: usize,
    /// Total indentation level (spaces + tabs)
    pub total_level: usize,
}

impl IndentationInfo {
    /// Create new indentation info from a line
    ///
    /// # Arguments
    ///
    /// * `line` - The line to analyze
    ///
    /// # Returns
    /// A new `IndentationInfo` instance with the indentation details
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::line_parser::IndentationInfo;
    ///
    /// let info = IndentationInfo::from_line("  key: value");
    /// assert_eq!(info.leading_spaces, 2);
    /// assert_eq!(info.leading_tabs, 0);
    /// assert_eq!(info.total_level, 2);
    /// ```
    pub fn from_line(line: &str) -> Self {
        let mut leading_spaces = 0;
        let mut leading_tabs = 0;

        for ch in line.chars() {
            match ch {
                ' ' => leading_spaces += 1,
                '\t' => leading_tabs += 1,
                _ => break, // Stop at first non-whitespace character
            }
        }

        Self {
            leading_spaces,
            leading_tabs,
            total_level: leading_spaces + leading_tabs,
        }
    }

    /// Check if the line has mixed indentation (both spaces and tabs)
    ///
    /// # Returns
    /// `true` if the line contains both spaces and tabs in leading whitespace
    pub fn is_mixed(&self) -> bool {
        self.leading_spaces > 0 && self.leading_tabs > 0
    }

    /// Check if the line uses only tab indentation
    ///
    /// # Returns
    /// `true` if the line uses only tabs for indentation
    pub fn is_tabs_only(&self) -> bool {
        self.leading_tabs > 0 && self.leading_spaces == 0
    }

    /// Check if the line uses only space indentation
    ///
    /// # Returns
    /// `true` if the line uses only spaces for indentation
    pub fn is_spaces_only(&self) -> bool {
        self.leading_spaces > 0 && self.leading_tabs == 0
    }

    /// Check if the line has no indentation
    ///
    /// # Returns
    /// `true` if the line has no leading whitespace
    pub fn is_empty(&self) -> bool {
        self.total_level == 0
    }
}

impl fmt::Display for IndentationInfo {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.is_mixed() {
            write!(
                f,
                "{} spaces + {} tabs (MIXED)",
                self.leading_spaces, self.leading_tabs
            )
        } else if self.is_tabs_only() {
            write!(f, "{} tabs", self.leading_tabs)
        } else if self.is_spaces_only() {
            write!(f, "{} spaces", self.leading_spaces)
        } else {
            write!(f, "no indentation")
        }
    }
}

/// Classify a YAML line into its type
///
/// This function analyzes a line and determines its type based on YAML
/// specification rules and common patterns.
///
/// # Arguments
///
/// * `line` - The line to classify
///
/// # Returns
/// The classified `LineType` for the line
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::{classify_line_type, LineType};
///
/// assert_eq!(classify_line_type(""), LineType::Blank);
/// assert_eq!(classify_line_type("  "), LineType::Blank);
/// assert_eq!(classify_line_type("# comment"), LineType::Comment);
/// assert_eq!(classify_line_type("---"), LineType::DocumentStart);
/// assert_eq!(classify_line_type("key: value"), LineType::MappingKey);
/// ```
pub fn classify_line_type(line: &str) -> LineType {
    let trimmed = line.trim();

    // Empty lines (including whitespace-only lines)
    if trimmed.is_empty() {
        return LineType::Blank;
    }

    // Comment lines
    if trimmed.starts_with('#') {
        return LineType::Comment;
    }

    // Document markers
    if trimmed == "---" {
        return LineType::DocumentStart;
    }
    if trimmed == "..." {
        return LineType::DocumentEnd;
    }

    // YAML directives (start with %)
    if trimmed.starts_with('%') {
        return LineType::Directive;
    }

    // Tags (start with !)
    if trimmed.starts_with('!') {
        return LineType::Tag;
    }

    // Anchors (start with &)
    if trimmed.starts_with('&') {
        return LineType::Anchor;
    }

    // Aliases (start with *)
    if trimmed.starts_with('*') {
        return LineType::Alias;
    }

    // Literal block scalar (|)
    if trimmed.starts_with('|') {
        return LineType::LiteralBlockScalar;
    }

    // Folded block scalar (>)
    if trimmed.starts_with('>') {
        return LineType::FoldedBlockScalar;
    }

    // Sequence items (start with -)
    // Note: YAML spec requires "- " but we're lenient for lone "-" as well
    if trimmed.starts_with("-") {
        return LineType::SequenceItem;
    }

    // Flow style mapping (starts with {)
    if trimmed.contains('{') && !trimmed.contains(':') {
        return LineType::FlowMapping;
    }

    // Flow style sequence (starts with [)
    if trimmed.contains('[') {
        return LineType::FlowSequence;
    }

    // Default to mapping key for most content lines
    // This handles "key: value" patterns
    if trimmed.contains(':') {
        return LineType::MappingKey;
    }

    // Fallback to unknown
    LineType::Unknown
}

/// Calculate indentation level from a line
///
/// This function counts the leading whitespace characters in a line.
/// It handles both spaces and tabs, counting each character individually.
///
/// **Note on tab vs space indentation**: YAML specification allows both
/// space and tab indentation, but they should never be mixed in the same
/// document. This function counts tabs and spaces equally (1 tab = 1 level)
/// for basic indentation tracking. For proper validation, use the
/// `IndentationInfo::from_line()` function which provides separate counts
/// and can detect mixed indentation.
///
/// # Arguments
///
/// * `line` - The line to analyze
///
/// # Returns
/// The number of leading whitespace characters
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::calculate_indentation;
///
/// assert_eq!(calculate_indentation("key: value"), 0);
/// assert_eq!(calculate_indentation("  key: value"), 2);
/// assert_eq!(calculate_indentation("\tkey: value"), 1);
/// assert_eq!(calculate_indentation("    key: value"), 4);
/// ```
pub fn calculate_indentation(line: &str) -> usize {
    IndentationInfo::from_line(line).total_level
}

/// Result of parsing a YAML file into lines
///
/// This type contains the complete results of a line-by-line YAML parsing operation.
#[derive(Debug, Clone)]
pub struct LineParseResult {
    /// All parsed lines in order
    pub lines: Vec<YamlLine>,

    /// Total number of lines processed
    pub total_lines: usize,

    /// Number of blank lines found
    pub blank_lines: usize,

    /// Number of comment lines found
    pub comment_lines: usize,

    /// Number of content lines found
    pub content_lines: usize,

    /// Maximum indentation level found
    pub max_indentation: usize,
}

impl LineParseResult {
    /// Create a new line parse result from a vector of lines
    ///
    /// # Arguments
    ///
    /// * `lines` - The parsed YAML lines
    ///
    /// # Returns
    /// A new `LineParseResult` with computed statistics
    pub fn new(lines: Vec<YamlLine>) -> Self {
        let total_lines = lines.len();
        let blank_lines = lines.iter().filter(|l| l.is_blank()).count();
        let comment_lines = lines.iter().filter(|l| l.is_comment()).count();
        let content_lines = lines.iter().filter(|l| l.is_content()).count();
        let max_indentation = lines.iter().map(|l| l.indentation_level()).max().unwrap_or(0);

        Self {
            lines,
            total_lines,
            blank_lines,
            comment_lines,
            content_lines,
            max_indentation,
        }
    }

    /// Check if the parse result is empty
    ///
    /// # Returns
    /// `true` if no lines were parsed
    pub fn is_empty(&self) -> bool {
        self.lines.is_empty()
    }

    /// Get lines by type
    ///
    /// # Arguments
    ///
    /// * `line_type` - The line type to filter by
    ///
    /// # Returns
    /// An iterator over lines matching the specified type
    pub fn lines_by_type(&self, line_type: LineType) -> impl Iterator<Item = &YamlLine> {
        self.lines.iter().filter(move |line| line.line_type() == line_type)
    }

    /// Get content lines (non-structural)
    ///
    /// # Returns
    /// An iterator over content lines
    pub fn content_lines(&self) -> impl Iterator<Item = &YamlLine> {
        self.lines.iter().filter(|line| line.is_content())
    }

    /// Get structural lines (blank, comments, document markers)
    ///
    /// # Returns
    /// An iterator over structural lines
    pub fn structural_lines(&self) -> impl Iterator<Item = &YamlLine> {
        self.lines.iter().filter(|line| line.is_structural())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_line_type_descriptions() {
        assert_eq!(LineType::Blank.description(), "blank line");
        assert_eq!(LineType::Comment.description(), "comment line");
        assert_eq!(LineType::MappingKey.description(), "mapping key");
    }

    #[test]
    fn test_line_type_classification() {
        assert!(LineType::Blank.is_structural());
        assert!(LineType::Comment.is_structural());
        assert!(LineType::MappingKey.is_content());
        assert!(LineType::SequenceItem.is_content());
        assert!(!LineType::MappingKey.is_structural());
    }

    #[test]
    fn test_yaml_line_creation() {
        let line = YamlLine::new(1, "key: value", 0, LineType::MappingKey);
        assert_eq!(line.line_number(), 1);
        assert_eq!(line.raw_content(), "key: value");
        assert_eq!(line.indentation_level(), 0);
        assert_eq!(line.line_type(), LineType::MappingKey);
    }

    #[test]
    fn test_yaml_line_with_content() {
        let content = LineContent::MappingKey {
            key: "name".to_string(),
            value: Some("John".to_string()),
        };
        let line = YamlLine::with_content(1, "name: John", 0, LineType::MappingKey, content);

        assert_eq!(line.line_number(), 1);
        assert!(line.content().is_some());
    }

    #[test]
    fn test_yaml_line_queries() {
        let blank = YamlLine::new(1, "", 0, LineType::Blank);
        assert!(blank.is_blank());
        assert!(!blank.is_mapping_key());

        let key = YamlLine::new(2, "key: value", 0, LineType::MappingKey);
        assert!(key.is_mapping_key());
        assert!(!key.is_blank());
        assert!(key.is_content());
        assert!(!key.is_structural());
    }

    #[test]
    fn test_line_content_display() {
        let comment = LineContent::Comment("This is a comment".to_string());
        assert_eq!(format!("{}", comment), "# This is a comment");

        let mapping = LineContent::MappingKey {
            key: "name".to_string(),
            value: Some("John".to_string()),
        };
        assert_eq!(format!("{}", mapping), "name: John");
    }

    #[test]
    fn test_line_parse_result_statistics() {
        let lines = vec![
            YamlLine::new(1, "", 0, LineType::Blank),
            YamlLine::new(2, "# comment", 0, LineType::Comment),
            YamlLine::new(3, "key: value", 0, LineType::MappingKey),
            YamlLine::new(4, "  nested: value", 2, LineType::MappingKey),
        ];

        let result = LineParseResult::new(lines);
        assert_eq!(result.total_lines, 4);
        assert_eq!(result.blank_lines, 1);
        assert_eq!(result.comment_lines, 1);
        assert_eq!(result.content_lines, 2);
        assert_eq!(result.max_indentation, 2);
    }

    #[test]
    fn test_line_parse_result_filtering() {
        let lines = vec![
            YamlLine::new(1, "", 0, LineType::Blank),
            YamlLine::new(2, "# comment", 0, LineType::Comment),
            YamlLine::new(3, "key: value", 0, LineType::MappingKey),
        ];

        let result = LineParseResult::new(lines);

        let content_lines: Vec<_> = result.content_lines().collect();
        assert_eq!(content_lines.len(), 1);

        let structural_lines: Vec<_> = result.structural_lines().collect();
        assert_eq!(structural_lines.len(), 2);
    }

    // IndentationInfo tests
    #[test]
    fn test_indentation_info_no_indentation() {
        let info = IndentationInfo::from_line("key: value");
        assert_eq!(info.leading_spaces, 0);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 0);
        assert!(info.is_empty());
        assert!(!info.is_mixed());
        assert!(!info.is_tabs_only());
        assert!(!info.is_spaces_only());
    }

    #[test]
    fn test_indentation_info_spaces_only() {
        let info = IndentationInfo::from_line("  key: value");
        assert_eq!(info.leading_spaces, 2);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 2);
        assert!(!info.is_empty());
        assert!(!info.is_mixed());
        assert!(!info.is_tabs_only());
        assert!(info.is_spaces_only());
    }

    #[test]
    fn test_indentation_info_tabs_only() {
        let info = IndentationInfo::from_line("\t\tkey: value");
        assert_eq!(info.leading_spaces, 0);
        assert_eq!(info.leading_tabs, 2);
        assert_eq!(info.total_level, 2);
        assert!(!info.is_empty());
        assert!(!info.is_mixed());
        assert!(info.is_tabs_only());
        assert!(!info.is_spaces_only());
    }

    #[test]
    fn test_indentation_info_mixed() {
        let info = IndentationInfo::from_line(" \t key: value");
        assert_eq!(info.leading_spaces, 1);
        assert_eq!(info.leading_tabs, 1);
        assert_eq!(info.total_level, 2);
        assert!(!info.is_empty());
        assert!(info.is_mixed());
        assert!(!info.is_tabs_only());
        assert!(!info.is_spaces_only());
    }

    #[test]
    fn test_indentation_info_deep_indentation() {
        let info = IndentationInfo::from_line("        key: value");
        assert_eq!(info.leading_spaces, 8);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 8);
    }

    #[test]
    fn test_indentation_info_whitespace_only_line() {
        let info = IndentationInfo::from_line("    ");
        assert_eq!(info.leading_spaces, 4);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 4);
        assert!(!info.is_empty());
    }

    #[test]
    fn test_indentation_info_empty_line() {
        let info = IndentationInfo::from_line("");
        assert_eq!(info.leading_spaces, 0);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 0);
        assert!(info.is_empty());
    }

    #[test]
    fn test_indentation_info_display() {
        let info_spaces = IndentationInfo::from_line("  key: value");
        assert_eq!(format!("{}", info_spaces), "2 spaces");

        let info_tabs = IndentationInfo::from_line("\tkey: value");
        assert_eq!(format!("{}", info_tabs), "1 tabs");

        let info_mixed = IndentationInfo::from_line(" \t key: value");
        assert_eq!(format!("{}", info_mixed), "1 spaces + 1 tabs (MIXED)");

        let info_empty = IndentationInfo::from_line("key: value");
        assert_eq!(format!("{}", info_empty), "no indentation");
    }

    // classify_line_type tests
    #[test]
    fn test_classify_blank_line() {
        assert_eq!(classify_line_type(""), LineType::Blank);
        assert_eq!(classify_line_type("   "), LineType::Blank);
        assert_eq!(classify_line_type("\t\t"), LineType::Blank);
        assert_eq!(classify_line_type(" \t "), LineType::Blank);
    }

    #[test]
    fn test_classify_comment_line() {
        assert_eq!(classify_line_type("# comment"), LineType::Comment);
        assert_eq!(classify_line_type("  # indented comment"), LineType::Comment);
        assert_eq!(classify_line_type("#"), LineType::Comment);
        assert_eq!(classify_line_type("# TODO: fix this"), LineType::Comment);
    }

    #[test]
    fn test_classify_document_markers() {
        assert_eq!(classify_line_type("---"), LineType::DocumentStart);
        assert_eq!(classify_line_type("  ---"), LineType::DocumentStart);
        assert_eq!(classify_line_type("..."), LineType::DocumentEnd);
        assert_eq!(classify_line_type("  ..."), LineType::DocumentEnd);
    }

    #[test]
    fn test_classify_mapping_key() {
        assert_eq!(classify_line_type("key: value"), LineType::MappingKey);
        assert_eq!(classify_line_type("  nested: value"), LineType::MappingKey);
        assert_eq!(classify_line_type("key:"), LineType::MappingKey);
        assert_eq!(classify_line_type("key: "), LineType::MappingKey);
        assert_eq!(classify_line_type("my-key: value"), LineType::MappingKey);
    }

    #[test]
    fn test_classify_sequence_item() {
        assert_eq!(classify_line_type("- item"), LineType::SequenceItem);
        assert_eq!(classify_line_type("  - item"), LineType::SequenceItem);
        assert_eq!(classify_line_type("- key: value"), LineType::SequenceItem);
        assert_eq!(classify_line_type("-"), LineType::SequenceItem);
    }

    #[test]
    fn test_classify_directive() {
        assert_eq!(classify_line_type("%YAML 1.2"), LineType::Directive);
        assert_eq!(classify_line_type("  %YAML 1.2"), LineType::Directive);
        assert_eq!(classify_line_type("%TAG ! tag:example.com,2014:"), LineType::Directive);
    }

    #[test]
    fn test_classify_tag() {
        assert_eq!(classify_line_type("!tag"), LineType::Tag);
        assert_eq!(classify_line_type("  !tag"), LineType::Tag);
        assert_eq!(classify_line_type("!my_tag"), LineType::Tag);
    }

    #[test]
    fn test_classify_anchor() {
        assert_eq!(classify_line_type("&anchor"), LineType::Anchor);
        assert_eq!(classify_line_type("  &anchor"), LineType::Anchor);
        assert_eq!(classify_line_type("&my_anchor"), LineType::Anchor);
    }

    #[test]
    fn test_classify_alias() {
        assert_eq!(classify_line_type("*alias"), LineType::Alias);
        assert_eq!(classify_line_type("  *alias"), LineType::Alias);
        assert_eq!(classify_line_type("*my_alias"), LineType::Alias);
    }

    #[test]
    fn test_classify_literal_block_scalar() {
        assert_eq!(classify_line_type("|"), LineType::LiteralBlockScalar);
        assert_eq!(classify_line_type("  |"), LineType::LiteralBlockScalar);
        assert_eq!(classify_line_type("|-"), LineType::LiteralBlockScalar);
        assert_eq!(classify_line_type("|+"), LineType::LiteralBlockScalar);
    }

    #[test]
    fn test_classify_folded_block_scalar() {
        assert_eq!(classify_line_type(">"), LineType::FoldedBlockScalar);
        assert_eq!(classify_line_type("  >"), LineType::FoldedBlockScalar);
        assert_eq!(classify_line_type(">-"), LineType::FoldedBlockScalar);
        assert_eq!(classify_line_type(">+"), LineType::FoldedBlockScalar);
    }

    #[test]
    fn test_classify_unknown() {
        // Lines without colons or other YAML patterns are Unknown
        assert_eq!(classify_line_type("just some text"), LineType::Unknown);
        assert_eq!(classify_line_type("  random text"), LineType::Unknown);
    }

    // calculate_indentation tests
    #[test]
    fn test_calculate_indentation_no_indent() {
        assert_eq!(calculate_indentation("key: value"), 0);
        assert_eq!(calculate_indentation(""), 0);
        assert_eq!(calculate_indentation("# comment"), 0);
    }

    #[test]
    fn test_calculate_indentation_spaces() {
        assert_eq!(calculate_indentation("  key: value"), 2);
        assert_eq!(calculate_indentation("    key: value"), 4);
        assert_eq!(calculate_indentation("      key: value"), 6);
        assert_eq!(calculate_indentation("        key: value"), 8);
    }

    #[test]
    fn test_calculate_indentation_tabs() {
        assert_eq!(calculate_indentation("\tkey: value"), 1);
        assert_eq!(calculate_indentation("\t\tkey: value"), 2);
        assert_eq!(calculate_indentation("\t\t\tkey: value"), 3);
    }

    #[test]
    fn test_calculate_indentation_mixed() {
        // Mixed tabs and spaces are counted as individual characters
        assert_eq!(calculate_indentation(" \tkey: value"), 2);
        assert_eq!(calculate_indentation("\t key: value"), 2);
        assert_eq!(calculate_indentation("  \t key: value"), 4);
    }

    #[test]
    fn test_calculate_indentation_whitespace_only() {
        assert_eq!(calculate_indentation("  "), 2);
        assert_eq!(calculate_indentation("\t\t"), 2);
        assert_eq!(calculate_indentation("    "), 4);
    }

    #[test]
    fn test_calculate_indentation_edge_cases() {
        // Test that indentation stops at first non-whitespace
        assert_eq!(calculate_indentation("  key: value  "), 2);
        assert_eq!(calculate_indentation("\tkey: value\t"), 1);

        // Test Unicode spaces (if present)
        assert_eq!(calculate_indentation("  key: value"), 2);
    }

    // Integration tests combining classification and indentation
    #[test]
    fn test_classification_and_indentation_integration() {
        // Test various YAML patterns together
        let lines = vec![
            ("", LineType::Blank, 0),
            ("   ", LineType::Blank, 3),
            ("# comment", LineType::Comment, 0),
            ("  # comment", LineType::Comment, 2),
            ("---", LineType::DocumentStart, 0),
            ("  ---", LineType::DocumentStart, 2),
            ("...", LineType::DocumentEnd, 0),
            ("key: value", LineType::MappingKey, 0),
            ("  key: value", LineType::MappingKey, 2),
            ("    key: value", LineType::MappingKey, 4),
            ("- item", LineType::SequenceItem, 0),
            ("  - item", LineType::SequenceItem, 2),
            ("|", LineType::LiteralBlockScalar, 0),
            ("  >", LineType::FoldedBlockScalar, 2),
        ];

        for (line, expected_type, expected_indent) in lines {
            assert_eq!(
                classify_line_type(line),
                expected_type,
                "Failed to classify line: '{}'",
                line
            );
            assert_eq!(
                calculate_indentation(line),
                expected_indent,
                "Failed to calculate indentation for line: '{}'",
                line
            );
        }
    }

    #[test]
    fn test_indentation_info_with_various_yaml_patterns() {
        // Test indentation info with realistic YAML patterns
        let info = IndentationInfo::from_line("  - item");
        assert_eq!(info.leading_spaces, 2);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 2);

        let info = IndentationInfo::from_line("\t- item");
        assert_eq!(info.leading_spaces, 0);
        assert_eq!(info.leading_tabs, 1);
        assert_eq!(info.total_level, 1);

        let info = IndentationInfo::from_line("    key: value");
        assert_eq!(info.leading_spaces, 4);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 4);
    }
}
