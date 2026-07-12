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
}
