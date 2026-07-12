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

/// Information about a detected mapping key
///
/// This struct provides detailed information about a mapping key detected
/// in a YAML line, including the key identifier and metadata.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MappingKeyInfo {
    /// The key identifier (text before the colon, trimmed)
    pub key: String,

    /// The value part, if present on the same line
    pub value: Option<String>,

    /// Whether the key has a value on the same line
    pub has_inline_value: bool,

    /// Whether the line appears to be a parent key (no value on same line)
    pub is_parent_key: bool,
}

impl MappingKeyInfo {
    /// Create a new mapping key info
    ///
    /// # Arguments
    ///
    /// * `key` - The key identifier
    /// * `value` - Optional value from the same line
    ///
    /// # Returns
    /// A new `MappingKeyInfo` instance
    pub fn new(key: String, value: Option<String>) -> Self {
        let has_inline_value = value.is_some();
        let is_parent_key = !has_inline_value;

        Self {
            key,
            value,
            has_inline_value,
            is_parent_key,
        }
    }

    /// Check if this is a valid key (non-empty and follows YAML key rules)
    pub fn is_valid(&self) -> bool {
        !self.key.is_empty() && self.key.chars().next().map_or(false, |c| {
            c.is_alphanumeric() || c == '_' || c == '.' || c == '-'
        })
    }
}

impl fmt::Display for MappingKeyInfo {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if let Some(v) = &self.value {
            write!(f, "{}: {}", self.key, v)
        } else {
            write!(f, "{}:", self.key)
        }
    }
}

/// Check if a line is a YAML comment line
///
/// This function determines if a line is a full-line comment (starts with #)
/// or contains inline content. It handles leading whitespace properly.
///
/// # Arguments
///
/// * `line` - The line to check
///
/// # Returns
///
/// `true` if the line is a comment line, `false` otherwise
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::is_comment_line;
///
/// assert!(is_comment_line("# This is a comment"));
/// assert!(is_comment_line("  # indented comment"));
/// assert!(!is_comment_line("key: value # not a comment line"));
/// assert!(!is_comment_line("key: value"));
/// ```
pub fn is_comment_line(line: &str) -> bool {
    let trimmed = line.trim();
    trimmed.starts_with('#')
}

/// Check if a line contains unquoted brackets or braces
///
/// This helper function checks if `{` or `[` appear outside of quoted strings
/// in a line. This is used to distinguish between flow style mappings/sequences
/// and quoted strings that happen to contain these characters.
///
/// # Arguments
///
/// * `line` - The line to check
///
/// # Returns
///
/// `true` if unquoted `{` or `[` are found, `false` otherwise
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::check_for_unquoted_brackets;
///
/// // Flow style mapping - has unquoted braces
/// assert!(check_for_unquoted_brackets("{key: value}"));
///
/// // Flow style sequence - has unquoted brackets
/// assert!(check_for_unquoted_brackets("[item1, item2]"));
///
/// // Quoted string with brackets - no unquoted brackets
/// assert!(!check_for_unquoted_brackets("key: \"value [with] brackets\""));
///
/// // Mixed - quoted brackets are OK, unquoted are not
/// assert!(check_for_unquoted_brackets("key: [value] \"more\""));
/// ```
fn check_for_unquoted_brackets(line: &str) -> bool {
    let mut in_single_quote = false;
    let mut in_double_quote = false;
    let mut escaped = false;

    for ch in line.chars() {
        if escaped {
            escaped = false;
            continue;
        }

        match ch {
            '\\' => {
                escaped = true;
            }
            '\'' if !in_double_quote => {
                in_single_quote = !in_single_quote;
            }
            '"' if !in_single_quote => {
                in_double_quote = !in_double_quote;
            }
            '[' if !in_single_quote && !in_double_quote => {
                return true; // Found unquoted opening bracket
            }
            '{' if !in_single_quote && !in_double_quote => {
                return true; // Found unquoted opening brace
            }
            _ => {}
        }
    }

    false
}

/// Find the first unquoted colon in a line
///
/// This helper function finds the first colon that is outside of quoted strings
/// and square brackets (for IPv6 addresses). This is used to locate the mapping
/// key separator in YAML lines.
///
/// # Arguments
///
/// * `line` - The line to search
///
/// # Returns
///
/// `Some(usize)` with the position of the first unquoted colon, or `None` if not found
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::find_unquoted_colon;
///
/// // Simple key-value
/// assert_eq!(find_unquoted_colon("key: value"), Some(3));
///
/// // Colon in quoted value
/// assert_eq!(find_unquoted_colon("time: \"12:30:00\""), Some(4));
///
/// // Quoted key with colon
/// assert_eq!(find_unquoted_colon("'key:with:colons': value"), Some(17));
///
/// // IPv6 URL - colons inside brackets are skipped
/// assert_eq!(find_unquoted_colon("url: http://[2001:db8::1]:8080"), Some(3));
///
/// // No colon
/// assert_eq!(find_unquoted_colon("no colon here"), None);
/// ```
fn find_unquoted_colon(line: &str) -> Option<usize> {
    let mut in_single_quote = false;
    let mut in_double_quote = false;
    let mut in_brackets = 0; // Track nested brackets for IPv6 addresses
    let mut escaped = false;

    for (pos, ch) in line.chars().enumerate() {
        if escaped {
            escaped = false;
            continue;
        }

        match ch {
            '\\' => {
                escaped = true;
            }
            '\'' if !in_double_quote => {
                in_single_quote = !in_single_quote;
            }
            '"' if !in_single_quote => {
                in_double_quote = !in_double_quote;
            }
            '[' if !in_single_quote && !in_double_quote => {
                in_brackets += 1;
            }
            ']' if !in_single_quote && !in_double_quote => {
                if in_brackets > 0 {
                    in_brackets -= 1;
                }
            }
            ':' if !in_single_quote && !in_double_quote && in_brackets == 0 => {
                return Some(pos); // Found unquoted colon outside of brackets
            }
            _ => {}
        }
    }

    None
}

/// Strip inline comment from a YAML line
///
/// This function removes inline comments from a YAML line while preserving:
/// - Hash characters (#) in URLs (http://example.com#anchor)
/// - Hash characters in quoted strings
/// - Hash characters in values
///
/// The function intelligently handles quoted strings and only removes comments
/// that are outside of quotes AND preceded by whitespace (per YAML spec).
///
/// # Arguments
///
/// * `line` - The line to process
///
/// # Returns
///
/// The line with inline comments removed, or the original line if no comments found
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::strip_inline_comment;
///
/// // Basic inline comment
/// assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");
///
/// // Comment without leading space
/// assert_eq!(strip_inline_comment("key: value#comment"), "key: value");
///
/// // Hash in URL should be preserved
/// assert_eq!(strip_inline_comment("url: http://example.com#anchor"), "url: http://example.com#anchor");
///
/// // Hash in quoted string should be preserved
/// assert_eq!(strip_inline_comment("key: \"value with # hash\" # comment"), "key: \"value with # hash\" ");
/// ```
pub fn strip_inline_comment(line: &str) -> String {
    let mut result = String::with_capacity(line.len());
    let mut chars = line.chars().peekable();
    let mut in_single_quote = false;
    let mut in_double_quote = false;
    let mut escaped = false;
    let mut prev_char: Option<char> = None;

    while let Some(ch) = chars.next() {
        if escaped {
            // After escape character, preserve everything
            result.push(ch);
            prev_char = Some(ch);
            escaped = false;
            continue;
        }

        match ch {
            '\\' => {
                result.push(ch);
                prev_char = Some(ch);
                escaped = true;
            }
            '\'' if !in_double_quote => {
                in_single_quote = !in_single_quote;
                result.push(ch);
                prev_char = Some(ch);
            }
            '"' if !in_single_quote => {
                in_double_quote = !in_double_quote;
                result.push(ch);
                prev_char = Some(ch);
            }
            '#' if !in_single_quote && !in_double_quote => {
                // Check if this # is preceded by whitespace (YAML comment rule)
                // A # only starts a comment if it's preceded by whitespace or at line start
                let is_comment = prev_char.map_or(true, |c| c.is_whitespace());

                if is_comment {
                    // Found comment start - stop processing
                    break;
                } else {
                    // # is part of the value (like in URL), preserve it
                    result.push(ch);
                    prev_char = Some(ch);
                }
            }
            _ => {
                result.push(ch);
                prev_char = Some(ch);
            }
        }
    }

    result
}

/// Detect if a line is a YAML mapping key and extract key information
///
/// This function analyzes a YAML line to determine if it contains a mapping key,
/// and extracts the key identifier and optional value. It handles various edge cases:
///
/// - Lines with colons in values (URLs, timestamps, etc.)
/// - Comment lines with colons (excluded from detection)
/// - Nested mappings (proper indentation)
/// - Parent keys (keys without values on the same line)
/// - Flow style mappings (excluded)
/// - Special YAML constructs (anchors, aliases, tags - excluded)
///
/// # Arguments
///
/// * `line` - The YAML line to analyze
/// * `parent_indent` - The indentation level of the parent context (for nested mappings)
///
/// # Returns
///
/// - `Some(MappingKeyInfo)` if the line is a valid mapping key
/// - `None` if the line is not a mapping key
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::detect_mapping_key;
///
/// // Simple key-value pair
/// let info = detect_mapping_key("name: John", 0);
/// assert!(info.is_some());
/// assert_eq!(info.unwrap().key, "name");
///
/// // Comment line with colon - should not be detected as key
/// let info = detect_mapping_key("# This: is a comment", 0);
/// assert!(info.is_none());
///
/// // Key with colon in value (URL)
/// let info = detect_mapping_key("url: http://example.com", 0);
/// assert!(info.is_some());
/// assert_eq!(info.unwrap().key, "url");
///
/// // Parent key (no value on same line)
/// let info = detect_mapping_key("nested:", 0);
/// assert!(info.is_some());
/// assert_eq!(info.unwrap().key, "nested");
/// assert!(info.unwrap().is_parent_key);
/// ```
pub fn detect_mapping_key(line: &str, parent_indent: usize) -> Option<MappingKeyInfo> {
    // Check if this is a full-line comment before processing
    if is_comment_line(line) {
        return None;
    }

    // Strip inline comments before processing
    let line_without_comments = strip_inline_comment(line);
    let trimmed = line_without_comments.trim();

    // Skip empty lines
    if trimmed.is_empty() {
        return None;
    }

    // Skip document markers
    if trimmed == "---" || trimmed == "..." {
        return None;
    }

    // Skip YAML directives
    if trimmed.starts_with('%') {
        return None;
    }

    // Skip tags (start with !)
    if trimmed.starts_with('!') {
        return None;
    }

    // Skip anchors (start with &)
    if trimmed.starts_with('&') {
        return None;
    }

    // Skip aliases (start with *)
    if trimmed.starts_with('*') {
        return None;
    }

    // Skip sequence items (start with -)
    if trimmed.starts_with('-') {
        return None;
    }

    // Skip explicit key indicators (start with ?)
    if trimmed.starts_with('?') {
        return None;
    }

    // Skip block scalar indicators
    if trimmed.starts_with('|') || trimmed.starts_with('>') {
        return None;
    }

    // Find the first unquoted colon in the line
    // This skips colons inside quotes and brackets (for IPv6 addresses)
    let colon_pos = find_unquoted_colon(&trimmed)?;

    // Now check for flow style mappings/sequences more carefully
    // We need to allow URLs with IPv6 addresses like "key: http://[2001:db8::1]:8080"
    // But reject flow style like "key: {value}" or "key: [items]"

    // Get the key and value parts
    let key_part = &trimmed[..colon_pos];
    let value_part = if colon_pos + 1 < trimmed.len() {
        Some(trimmed[colon_pos + 1..].trim())
    } else {
        None
    };

    // Check if key part contains { or [ (invalid for keys)
    if check_for_unquoted_brackets(key_part) {
        return None;
    }

    // Check if value is flow style (starts with { or [)
    // But allow it if it looks like a URL (contains ://)
    if let Some(value) = value_part {
        let trimmed_value = value.trim();
        if (trimmed_value.starts_with('{') || trimmed_value.starts_with('[')) && !trimmed_value.contains("://") {
            return None;
        }
    }

    // Convert value_part to String for the final result
    let value_part_string = value_part.map(|v| v.to_string());

    // Trim the key part
    let key = key_part.trim();

    // Key must not be empty
    if key.is_empty() {
        return None;
    }

    // Check if this is a quoted key (single or double quotes)
    let is_quoted_key = (key.starts_with('\'') && key.ends_with('\'') && key.len() > 1) ||
                        (key.starts_with('"') && key.ends_with('"') && key.len() > 1);

    // For non-quoted keys, validate that they only contain valid characters
    if !is_quoted_key {
        for ch in key.chars() {
            if !ch.is_alphanumeric() && ch != '_' && ch != '.' && ch != '-' {
                return None; // Invalid key character for unquoted key
            }
        }
    }
    // Quoted keys can contain any characters, so we skip validation for them

    // Check for proper indentation relative to parent
    let current_indent = calculate_indentation(line);

    // Indentation validation rules:
    // - current_indent < parent_indent: Invalid (exiting parent's context - not a child)
    // - current_indent == parent_indent: Valid (sibling key at same level)
    // - current_indent > parent_indent: Valid nested key IF:
    //   - parent_indent > 0 AND indent_increase >= 2 (proper nesting)
    //   - OR parent_indent == 0 (root level, any indentation is valid)

    if current_indent < parent_indent {
        // Decreasing indentation means we've exited the parent's context
        // This line is not a child of the current parent
        return None;
    } else if current_indent > parent_indent {
        let indent_increase = current_indent - parent_indent;

        // When parent is at root level (0), be lenient - allow any positive indent
        // When nested, require at least 2 spaces for proper nesting
        if parent_indent > 0 && indent_increase < 2 {
            return None; // Insufficient indentation for nested key
        }
    }
    // current_indent == parent_indent is valid - sibling key at same level

    // Check if this is a parent key (no value or just whitespace after colon)
    let is_parent = value_part_string.as_ref().map_or(true, |v| v.is_empty() || v.starts_with('#'));
    let value = if is_parent {
        None
    } else {
        value_part_string
    };

    Some(MappingKeyInfo::new(key.to_string(), value))
}

/// Combined function to detect mapping key and return key with child indentation
///
/// This is a convenience wrapper that integrates all detection logic and returns
/// a simple `(String, usize)` tuple containing the key name and the child's indentation level.
///
/// This function handles all edge cases including:
/// - Quoted values with colons: `message: "Hello: World"`
/// - URLs with colons: `url: http://example.com`
/// - Time values: `time: 10:30:00`
/// - Nested mappings: `parent:\n  child: value`
/// - Comment filtering: `# comment: text` is rejected
/// - Special constructs: sequences, anchors, aliases are rejected
///
/// # Arguments
///
/// * `line` - The YAML line to analyze
/// * `parent_indent` - The indentation level of the parent context
///
/// # Returns
///
/// - `Some((String, usize))` where the String is the key name and usize is the child's indentation level
/// - `None` if the line is not a valid mapping key
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::line_parser::detect_mapping_key_simple;
///
/// // Simple key-value pair
/// let result = detect_mapping_key_simple("name: John", 0);
/// assert_eq!(result, Some(("name".to_string(), 0)));
///
/// // Nested key
/// let result = detect_mapping_key_simple("  child: value", 0);
/// assert_eq!(result, Some(("child".to_string(), 2)));
///
/// // Comment line - rejected
/// let result = detect_mapping_key_simple("# comment: text", 0);
/// assert_eq!(result, None);
///
/// // URL with colon
/// let result = detect_mapping_key_simple("url: http://example.com", 0);
/// assert_eq!(result, Some(("url".to_string(), 0)));
/// ```
pub fn detect_mapping_key_simple(line: &str, parent_indent: usize) -> Option<(String, usize)> {
    detect_mapping_key(line, parent_indent).map(|info| {
        let child_indent = calculate_indentation(line);
        (info.key, child_indent)
    })
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
        // The string " \t key: value" has: space, tab, space, then "key"
        // So leading whitespace is: 2 spaces + 1 tab = 3 total
        assert_eq!(info.leading_spaces, 2);
        assert_eq!(info.leading_tabs, 1);
        assert_eq!(info.total_level, 3);
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
        assert_eq!(format!("{}", info_mixed), "2 spaces + 1 tabs (MIXED)");

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

    // Mapping key detection tests

    #[test]
    fn test_detect_mapping_key_simple_pair() {
        // Simple key: value pair
        let info = detect_mapping_key("name: John", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "name");
        assert_eq!(info.value, Some("John".to_string()));
        assert!(info.has_inline_value);
        assert!(!info.is_parent_key);
    }

    #[test]
    fn test_detect_mapping_key_nested() {
        // Nested key with proper indentation
        let info = detect_mapping_key("  nested: value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "nested");
        assert_eq!(info.value, Some("value".to_string()));
        assert!(info.has_inline_value);
    }

    #[test]
    fn test_detect_mapping_key_nested_with_parent_indent() {
        // Nested key with parent indentation
        let info = detect_mapping_key("    child: value", 2);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "child");
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_insufficient_indent() {
        // Insufficient indentation for nested key (less than 2 spaces increase)
        let info = detect_mapping_key(" child: value", 0);
        assert!(info.is_some()); // Detected as key at root level (1 space indent is valid)

        // For parent_indent=2, a line with 1 space has decreased indent (exiting context)
        let info = detect_mapping_key(" child: value", 2);
        assert!(info.is_none()); // Not valid when parent has more indent
    }

    #[test]
    fn test_detect_mapping_key_colon_in_value_url() {
        // Key with colon in value (URL)
        let info = detect_mapping_key("url: http://example.com", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "url");
        // Value should be "http://example.com"
        assert_eq!(info.value, Some("http://example.com".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_colon_in_value_timestamp() {
        // Key with colon in value (timestamp)
        let info = detect_mapping_key("time: 12:30:45", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "time");
        // Value should be "12:30:45"
        assert_eq!(info.value, Some("12:30:45".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_colon_in_value_multiple() {
        // Key with multiple colons in value
        let info = detect_mapping_key("api: http://api.example.com:8080/v1", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "api");
        // Value should include everything after first colon
        assert_eq!(info.value, Some("http://api.example.com:8080/v1".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_comment_line_with_colon() {
        // Comment line with colon - should NOT be detected as key
        let info = detect_mapping_key("# This: is a comment", 0);
        assert!(info.is_none(), "Comment lines should not be detected as keys");
    }

    #[test]
    fn test_detect_mapping_key_indented_comment_with_colon() {
        // Indented comment line with colon
        let info = detect_mapping_key("  # TODO: fix this later", 0);
        assert!(info.is_none(), "Indented comment lines should not be detected as keys");
    }

    #[test]
    fn test_detect_mapping_key_parent_key_no_value() {
        // Parent key (no value on same line)
        let info = detect_mapping_key("nested:", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "nested");
        assert!(info.value.is_none());
        assert!(info.is_parent_key);
        assert!(!info.has_inline_value);
    }

    #[test]
    fn test_detect_mapping_key_parent_key_with_whitespace() {
        // Parent key with whitespace after colon
        let info = detect_mapping_key("nested:   ", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "nested");
        assert!(info.value.is_none());
        assert!(info.is_parent_key);
    }

    #[test]
    fn test_detect_mapping_key_parent_key_with_comment() {
        // Parent key with inline comment
        let info = detect_mapping_key("nested: # comment here", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "nested");
        assert!(info.is_parent_key);
        assert!(!info.has_inline_value);
    }

    #[test]
    fn test_detect_mapping_key_empty_line() {
        // Empty line - should not be detected as key
        let info = detect_mapping_key("", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_whitespace_only() {
        // Whitespace-only line - should not be detected as key
        let info = detect_mapping_key("   ", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_document_start() {
        // Document start marker - should not be detected as key
        let info = detect_mapping_key("---", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_document_end() {
        // Document end marker - should not be detected as key
        let info = detect_mapping_key("...", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_sequence_item() {
        // Sequence item - should not be detected as key
        let info = detect_mapping_key("- item", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_sequence_with_key_value() {
        // Sequence item with key-value - should not be detected as key
        let info = detect_mapping_key("- key: value", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_anchor() {
        // Anchor definition - should not be detected as key
        let info = detect_mapping_key("&anchor", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_alias() {
        // Alias reference - should not be detected as key
        let info = detect_mapping_key("*alias", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_tag() {
        // Tag directive - should not be detected as key
        let info = detect_mapping_key("!tag", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_directive() {
        // YAML directive - should not be detected as key
        let info = detect_mapping_key("%YAML 1.2", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_explicit_key() {
        // Explicit key indicator - should not be detected as key
        let info = detect_mapping_key("? explicit_key", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_flow_mapping() {
        // Flow style mapping - should not be detected as key
        let info = detect_mapping_key("{key: value}", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_flow_sequence() {
        // Flow style sequence - should not be detected as key
        let info = detect_mapping_key("[item1, item2]", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_literal_block_scalar() {
        // Literal block scalar - should not be detected as key
        let info = detect_mapping_key("|", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_folded_block_scalar() {
        // Folded block scalar - should not be detected as key
        let info = detect_mapping_key(">", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_empty_key() {
        // Empty key (colon at start) - should not be detected as key
        let info = detect_mapping_key(": value", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_with_dash() {
        // Key with dash character
        let info = detect_mapping_key("my-key: value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "my-key");
    }

    #[test]
    fn test_detect_mapping_key_with_underscore() {
        // Key with underscore character
        let info = detect_mapping_key("my_key: value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "my_key");
    }

    #[test]
    fn test_detect_mapping_key_with_dot() {
        // Key with dot character
        let info = detect_mapping_key("my.key: value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "my.key");
    }

    #[test]
    fn test_detect_mapping_key_quoted_single_quotes() {
        // Quoted key with single quotes
        let info = detect_mapping_key("'my-key': value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "'my-key'");
    }

    #[test]
    fn test_detect_mapping_key_quoted_double_quotes() {
        // Quoted key with double quotes
        let info = detect_mapping_key("\"my-key\": value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "\"my-key\"");
    }

    #[test]
    fn test_detect_mapping_key_invalid_characters() {
        // Key with invalid characters (should not be detected)
        let info = detect_mapping_key("key@value: something", 0);
        assert!(info.is_none(), "Keys with @ character should not be detected");
    }

    #[test]
    fn test_detect_mapping_key_with_spaces_around_colon() {
        // Key with spaces around colon
        let info = detect_mapping_key("key : value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
    }

    #[test]
    fn test_detect_mapping_key_no_space_after_colon() {
        // Key with no space after colon
        let info = detect_mapping_key("key:value", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_multiple_colons_value_has_spaces() {
        // Multiple colons with spaces in value
        let info = detect_mapping_key("key: value: with: colons", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        assert_eq!(info.value, Some("value: with: colons".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_valid_key_is_valid() {
        // Test MappingKeyInfo::is_valid()
        let info = MappingKeyInfo::new("valid_key".to_string(), Some("value".to_string()));
        assert!(info.is_valid());

        let info = MappingKeyInfo::new("123key".to_string(), None);
        assert!(info.is_valid());

        let info = MappingKeyInfo::new("".to_string(), Some("value".to_string()));
        assert!(!info.is_valid(), "Empty key should not be valid");

        let info = MappingKeyInfo::new("@invalid".to_string(), None);
        assert!(!info.is_valid(), "Key starting with @ should not be valid");
    }

    #[test]
    fn test_detect_mapping_key_display() {
        // Test Display trait implementation
        let info_with_value = MappingKeyInfo::new("key".to_string(), Some("value".to_string()));
        assert_eq!(format!("{}", info_with_value), "key: value");

        let info_parent = MappingKeyInfo::new("parent".to_string(), None);
        assert_eq!(format!("{}", info_parent), "parent:");
    }

    #[test]
    fn test_detect_mapping_key_complex_nested_structure() {
        // Test complex nested structure
        let lines = vec![
            ("root:", 0, true, false),    // parent key
            ("  child1: value1", 0, true, true),
            ("  child2:", 0, true, false), // parent key
            ("    grandchild: value2", 2, true, true),
            ("  child3: value3", 0, true, true),
        ];

        for (line, parent_indent, expected_some, expected_has_value) in lines {
            let info = detect_mapping_key(line, parent_indent);
            assert_eq!(info.is_some(), expected_some, "Failed for line: {}", line);
            if expected_some {
                let info = info.unwrap();
                assert_eq!(info.has_inline_value, expected_has_value, "Failed for line: {}", line);
            }
        }
    }

    #[test]
    fn test_detect_mapping_key_sibling_keys() {
        // Test sibling keys at same indentation level
        let parent_indent = 0;

        let info1 = detect_mapping_key("sibling1: value1", parent_indent);
        assert!(info1.is_some());

        let info2 = detect_mapping_key("sibling2: value2", parent_indent);
        assert!(info2.is_some());

        let info3 = detect_mapping_key("sibling3:", parent_indent);
        assert!(info3.is_some());
        assert!(info3.unwrap().is_parent_key);
    }

    #[test]
    fn test_detect_mapping_key_decreasing_indentation() {
        // Test decreasing indentation (exiting nested context)
        let parent_indent = 4;

        // When parent_indent is 4 and current line has 2 spaces, we're exiting context
        // This should NOT be detected as a key in the current parent context
        let info = detect_mapping_key("  back_up: value", parent_indent);
        assert!(info.is_none(), "Should not detect key when exiting context (decreasing indent)");

        // But if we call it with parent_indent=0 (root level), it should be detected
        let info = detect_mapping_key("  back_up: value", 0);
        assert!(info.is_some(), "Should detect key at appropriate indentation level");
    }

    // Comment filtering tests

    #[test]
    fn test_is_comment_line_full_comment() {
        // Full comment line
        assert!(is_comment_line("# This is a comment"));
        assert!(is_comment_line("#"));
        assert!(is_comment_line("#TODO: fix this"));
    }

    #[test]
    fn test_is_comment_line_indented_comment() {
        // Indented comment lines
        assert!(is_comment_line("  # indented comment"));
        assert!(is_comment_line("    # deeply indented comment"));
        assert!(is_comment_line("\t# tab-indented comment"));
        assert!(is_comment_line("  \t  # mixed whitespace comment"));
    }

    #[test]
    fn test_is_comment_line_inline_comment_not_full_line() {
        // Lines with inline comments are NOT full-line comments
        assert!(!is_comment_line("key: value # comment"));
        assert!(!is_comment_line("key: value#comment"));
        assert!(!is_comment_line("  key: value # with inline comment"));
    }

    #[test]
    fn test_is_comment_line_regular_lines() {
        // Regular lines are not comments
        assert!(!is_comment_line("key: value"));
        assert!(!is_comment_line("  key: value"));
        assert!(!is_comment_line("- item"));
        assert!(!is_comment_line("---"));
        assert!(!is_comment_line(""));
        assert!(!is_comment_line("   "));
    }

    #[test]
    fn test_strip_inline_comment_basic() {
        // Basic inline comment (hash preceded by whitespace)
        assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");
        assert_eq!(strip_inline_comment("key: value # comment with more text"), "key: value ");
        // Hash without preceding whitespace is part of value, not comment
        assert_eq!(strip_inline_comment("key: value#comment"), "key: value#comment");
    }

    #[test]
    fn test_strip_inline_comment_no_comment() {
        // No comment to strip
        assert_eq!(strip_inline_comment("key: value"), "key: value");
        assert_eq!(strip_inline_comment("  key: value  "), "  key: value  ");
        assert_eq!(strip_inline_comment("just text"), "just text");
    }

    #[test]
    fn test_strip_inline_comment_hash_in_url() {
        // Hash in URL should be preserved
        assert_eq!(strip_inline_comment("url: http://example.com#anchor"), "url: http://example.com#anchor");
        assert_eq!(strip_inline_comment("url: https://example.com/path#section"), "url: https://example.com/path#section");
        assert_eq!(strip_inline_comment("link: ftp://files.example.com#dir"), "link: ftp://files.example.com#dir");
    }

    #[test]
    fn test_strip_inline_comment_hash_in_quoted_string() {
        // Hash in quoted string should be preserved
        assert_eq!(strip_inline_comment("key: \"value with # hash\""), "key: \"value with # hash\"");
        assert_eq!(strip_inline_comment("key: 'value with # hash'"), "key: 'value with # hash'");
        assert_eq!(strip_inline_comment("key: \"value #1\" # comment"), "key: \"value #1\" ");
    }

    #[test]
    fn test_strip_inline_comment_double_quoted_with_comment() {
        // Double quoted string followed by comment
        assert_eq!(strip_inline_comment("key: \"value\" # comment"), "key: \"value\" ");
        assert_eq!(strip_inline_comment("key: \"# not a comment\" # this is a comment"), "key: \"# not a comment\" ");
    }

    #[test]
    fn test_strip_inline_comment_single_quoted_with_comment() {
        // Single quoted string followed by comment
        assert_eq!(strip_inline_comment("key: 'value' # comment"), "key: 'value' ");
        assert_eq!(strip_inline_comment("key: '# not a comment' # this is a comment"), "key: '# not a comment' ");
    }

    #[test]
    fn test_strip_inline_comment_mixed_quotes() {
        // Mixed quote handling
        assert_eq!(strip_inline_comment("key: \"double 'inner' # hash\" # comment"), "key: \"double 'inner' # hash\" ");
        assert_eq!(strip_inline_comment("key: 'single \"inner\" # hash' # comment"), "key: 'single \"inner\" # hash' ");
    }

    #[test]
    fn test_strip_inline_comment_escaped_quotes() {
        // Escaped quotes should be handled
        assert_eq!(strip_inline_comment("key: \"value with \\\" escaped quote\" # comment"), "key: \"value with \\\" escaped quote\" ");
        assert_eq!(strip_inline_comment("key: 'value with \\' escaped quote' # comment"), "key: 'value with \\' escaped quote' ");
    }

    #[test]
    fn test_strip_inline_comment_multiple_hashes() {
        // Multiple hash characters without preceding whitespace are part of value
        assert_eq!(strip_inline_comment("key: value#hash#in#value"), "key: value#hash#in#value");
        // Hash preceded by whitespace starts comment, strips everything after
        assert_eq!(strip_inline_comment("key: value # comment # with # multiple # hashes"), "key: value ");
    }

    #[test]
    fn test_strip_inline_comment_edge_cases() {
        // Edge cases
        // Full comment line - hash at start with no preceding content
        assert_eq!(strip_inline_comment("#"), "");
        assert_eq!(strip_inline_comment("# comment"), "");
        // Empty line
        assert_eq!(strip_inline_comment(""), "");
        // Whitespace only
        assert_eq!(strip_inline_comment("   "), "   ");
        // Hash at start after whitespace (still a comment)
        assert_eq!(strip_inline_comment("  # comment"), "  ");
    }

    #[test]
    fn test_strip_inline_comment_preserves_leading_whitespace() {
        // Leading whitespace should be preserved
        assert_eq!(strip_inline_comment("  key: value # comment"), "  key: value ");
        assert_eq!(strip_inline_comment("\tkey: value # comment"), "\tkey: value ");
        assert_eq!(strip_inline_comment("    key: value # comment"), "    key: value ");
    }

    #[test]
    fn test_detect_mapping_key_with_inline_comment() {
        // Key detection should work with inline comments
        let info = detect_mapping_key("key: value # this is a comment", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_with_inline_comment_no_space() {
        // Hash without preceding whitespace is part of value, not a comment
        let info = detect_mapping_key("key: value#comment", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        // value#comment is the actual value since # wasn't preceded by whitespace
        assert_eq!(info.value, Some("value#comment".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_with_url_and_comment() {
        // URL with hash and inline comment
        let info = detect_mapping_key("url: http://example.com#anchor # this is a comment", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "url");
        // Value should preserve the URL hash
        assert_eq!(info.value, Some("http://example.com#anchor".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_with_quoted_value_and_comment() {
        // Quoted value with hash characters and inline comment
        let info = detect_mapping_key("key: \"value #1\" # this is a comment", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        // Value should preserve the quoted hash
        assert_eq!(info.value, Some("\"value #1\"".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_nested_with_comment() {
        // Nested key with inline comment
        let info = detect_mapping_key("  nested: value # comment", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "nested");
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_detect_mapping_key_full_comment_line_rejected() {
        // Full comment line should still be rejected
        let info = detect_mapping_key("# This: is a comment", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_indented_full_comment_rejected() {
        // Indented full comment line should be rejected
        let info = detect_mapping_key("  # TODO: fix this later", 0);
        assert!(info.is_none());
    }

    #[test]
    fn test_detect_mapping_key_multiple_inline_comments() {
        // Line with inline comment - only first # preceded by whitespace starts comment
        let info = detect_mapping_key("key: value # not # comments", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        // Should strip everything after first # (which is preceded by whitespace)
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_strip_inline_comment_complex_yaml_line() {
        // Complex real-world YAML line
        let line = "  database: \"postgresql://localhost:5432/db#schema\" # production database";
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, "  database: \"postgresql://localhost:5432/db#schema\" ");
    }

    #[test]
    fn test_detect_mapping_key_complex_real_world_line() {
        // Complex real-world YAML line with URL and comment
        let info = detect_mapping_key("api: \"https://api.example.com/v1#endpoint\" # production API", 0);
        assert!(info.is_some());
        let info = info.unwrap();
        assert_eq!(info.key, "api");
        // Value should preserve the URL hash in quoted string
        assert_eq!(info.value, Some("\"https://api.example.com/v1#endpoint\"".to_string()));
    }

    // Edge case tests for indentation parsing
    #[test]
    fn test_calculate_indentation_very_long_indentation() {
        // Very long indentation (100+ spaces)
        let long_indent = " ".repeat(100) + "key: value";
        assert_eq!(calculate_indentation(&long_indent), 100);

        let very_long_indent = " ".repeat(200) + "key: value";
        assert_eq!(calculate_indentation(&very_long_indent), 200);

        let extremely_long_indent = " ".repeat(500) + "key: value";
        assert_eq!(calculate_indentation(&extremely_long_indent), 500);
    }

    #[test]
    fn test_classify_line_type_hash_only() {
        // Lines with only "#" symbol (no text after)
        assert_eq!(classify_line_type("#"), LineType::Comment);
        assert_eq!(classify_line_type("  #"), LineType::Comment);
        assert_eq!(classify_line_type("\t#"), LineType::Comment);
        assert_eq!(classify_line_type("    #"), LineType::Comment);
    }

    #[test]
    fn test_classify_line_type_whitespace_and_hash() {
        // Lines with only whitespace and "#"
        assert_eq!(classify_line_type(" #"), LineType::Comment);
        assert_eq!(classify_line_type("  #"), LineType::Comment);
        assert_eq!(classify_line_type("   #"), LineType::Comment);
        assert_eq!(classify_line_type("\t#"), LineType::Comment);
        assert_eq!(classify_line_type("  \t #"), LineType::Comment);
        assert_eq!(classify_line_type(" \t  #"), LineType::Comment);
    }

    #[test]
    fn test_calculate_indentation_unicode_content() {
        // Unicode content with indentation
        let unicode_line = "  clé: valeur";
        assert_eq!(calculate_indentation(unicode_line), 2);

        let unicode_line2 = "    名前: 値";
        assert_eq!(calculate_indentation(unicode_line2), 4);

        let unicode_line3 = "\t用户名: 密码";
        assert_eq!(calculate_indentation(unicode_line3), 1);

        let unicode_line4 = "      ελληνικά: Greek";
        assert_eq!(calculate_indentation(unicode_line4), 6);

        let unicode_line5 = "    🎨: emoji";
        assert_eq!(calculate_indentation(unicode_line5), 4);

        let unicode_line6 = "  ñoño: value";
        assert_eq!(calculate_indentation(unicode_line6), 2);

        let unicode_line7 = "    -key-with-accents: value";
        assert_eq!(calculate_indentation(unicode_line7), 4);
    }

    #[test]
    fn test_calculate_indentation_mixed_whitespace() {
        // Mixed whitespace in comment lines
        assert_eq!(calculate_indentation(" \t# comment"), 2);
        assert_eq!(calculate_indentation("  \t# comment"), 3);
        assert_eq!(calculate_indentation("\t # comment"), 2);
        assert_eq!(calculate_indentation(" \t # comment"), 3);
        assert_eq!(calculate_indentation("  \t  # comment"), 5);
        assert_eq!(calculate_indentation("\t \t# comment"), 3);
    }

    #[test]
    fn test_indentation_info_very_long_indentation() {
        // Very long indentation (100+ spaces)
        let long_indent = " ".repeat(100) + "key: value";
        let info = IndentationInfo::from_line(&long_indent);
        assert_eq!(info.leading_spaces, 100);
        assert_eq!(info.leading_tabs, 0);
        assert_eq!(info.total_level, 100);
        assert!(info.is_spaces_only());
        assert!(!info.is_mixed());
    }

    #[test]
    fn test_indentation_info_mixed_whitespace_variations() {
        // Various mixed whitespace patterns
        let info1 = IndentationInfo::from_line(" \t#");
        assert_eq!(info1.leading_spaces, 1);
        assert_eq!(info1.leading_tabs, 1);
        assert!(info1.is_mixed());

        let info2 = IndentationInfo::from_line("  \t#");
        assert_eq!(info2.leading_spaces, 2);
        assert_eq!(info2.leading_tabs, 1);
        assert!(info2.is_mixed());

        let info3 = IndentationInfo::from_line("\t #");
        assert_eq!(info3.leading_spaces, 1);
        assert_eq!(info3.leading_tabs, 1);
        assert!(info3.is_mixed());

        let info4 = IndentationInfo::from_line("\t\t  #");
        assert_eq!(info4.leading_spaces, 2);
        assert_eq!(info4.leading_tabs, 2);
        assert!(info4.is_mixed());
    }

    #[test]
    fn test_classify_line_type_unicode_with_indentation() {
        // Unicode content classification with various indentation levels
        assert_eq!(classify_line_type("  clé: valeur"), LineType::MappingKey);
        assert_eq!(classify_line_type("    名前: 値"), LineType::MappingKey);
        assert_eq!(classify_line_type("\t用户名: 密码"), LineType::MappingKey);
        assert_eq!(classify_line_type("  # комментарий"), LineType::Comment);
        assert_eq!(classify_line_type("    # תגובה"), LineType::Comment);
    }

    #[test]
    fn test_classify_line_type_mixed_whitespace_with_content() {
        // Mixed whitespace with different content types
        assert_eq!(classify_line_type(" \t key: value"), LineType::MappingKey);
        assert_eq!(classify_line_type(" \t # comment"), LineType::Comment);
        assert_eq!(classify_line_type("  \t - item"), LineType::SequenceItem);
        assert_eq!(classify_line_type("\t #"), LineType::Comment);
        assert_eq!(classify_line_type(" \t  "), LineType::Blank);
    }

    #[test]
    fn test_calculate_indentation_empty_lines_edge_cases() {
        // Empty and whitespace-only edge cases
        assert_eq!(calculate_indentation(""), 0);
        assert_eq!(calculate_indentation(" "), 1);
        assert_eq!(calculate_indentation("  "), 2);
        assert_eq!(calculate_indentation("\t"), 1);
        assert_eq!(calculate_indentation("\t\t"), 2);
        assert_eq!(calculate_indentation(" \t"), 2);
        assert_eq!(calculate_indentation("\t "), 2);
        assert_eq!(calculate_indentation(" \t "), 3);
    }

    #[test]
    fn test_calculate_indentation_only_comments_edge_cases() {
        // Comments with various indentation patterns
        assert_eq!(calculate_indentation("#"), 0);
        assert_eq!(calculate_indentation(" #"), 1);
        assert_eq!(calculate_indentation("  #"), 2);
        assert_eq!(calculate_indentation("\t#"), 1);
        assert_eq!(calculate_indentation(" \t#"), 2);
        assert_eq!(calculate_indentation("\t #"), 2);
        assert_eq!(calculate_indentation("    # comment"), 4);
        assert_eq!(calculate_indentation("  \t # deep comment"), 4);
    }

    #[test]
    fn test_indentation_info_unicode_edge_cases() {
        // Unicode with various edge cases
        let info = IndentationInfo::from_line("  👍: thumbs up");
        assert_eq!(info.leading_spaces, 2);
        assert_eq!(info.total_level, 2);

        let info = IndentationInfo::from_line("    🔥: fire");
        assert_eq!(info.leading_spaces, 4);
        assert_eq!(info.total_level, 4);

        let info = IndentationInfo::from_line("\t🌍: world");
        assert_eq!(info.leading_tabs, 1);
        assert_eq!(info.total_level, 1);
    }

    #[test]
    fn test_classify_line_type_combined_edge_cases() {
        // Combined edge case testing
        // Very long indentation with various content types
        let very_long_indent = " ".repeat(100);
        assert_eq!(classify_line_type(&format!("{}key: value", very_long_indent)), LineType::MappingKey);
        assert_eq!(classify_line_type(&format!("{}# comment", very_long_indent)), LineType::Comment);
        assert_eq!(classify_line_type(&format!("{}- item", very_long_indent)), LineType::SequenceItem);

        // Hash only with various indentation
        assert_eq!(classify_line_type(&format!("{}#", " ".repeat(50))), LineType::Comment);
        assert_eq!(classify_line_type(&format!("{}#", "\t".repeat(10))), LineType::Comment);

        // Mixed whitespace with unicode
        assert_eq!(classify_line_type(" \t clé: valeur"), LineType::MappingKey);
        assert_eq!(classify_line_type("  \t 用户名: 密码"), LineType::MappingKey);
        assert_eq!(classify_line_type("\t \t # 注释"), LineType::Comment);
    }

    // Integration tests for comment filtering in key detection

    #[test]
    fn test_key_detection_skips_full_line_comments() {
        // Full-line comments should be ignored
        let info = detect_mapping_key("# This is a comment", 0);
        assert!(info.is_none(), "Full-line comment should not be detected as key");

        let info = detect_mapping_key("  # Indented comment", 0);
        assert!(info.is_none(), "Indented full-line comment should not be detected as key");

        let info = detect_mapping_key("\t# Tab-indented comment", 0);
        assert!(info.is_none(), "Tab-indented comment should not be detected as key");
    }

    #[test]
    fn test_key_detection_strips_inline_comments() {
        // Inline comments should be stripped before processing
        let info = detect_mapping_key("key: value # inline comment", 0);
        assert!(info.is_some(), "Key with inline comment should still be detected");
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        assert_eq!(info.value, Some("value".to_string()));

        let info = detect_mapping_key("  nested: value # comment here", 0);
        assert!(info.is_some(), "Nested key with inline comment should be detected");
        let info = info.unwrap();
        assert_eq!(info.key, "nested");
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_key_detection_preserves_hashes_in_values() {
        // Hashes in URLs and values should be preserved
        let info = detect_mapping_key("url: http://example.com#anchor", 0);
        assert!(info.is_some(), "Key with URL hash should be detected");
        let info = info.unwrap();
        assert_eq!(info.key, "url");
        assert_eq!(info.value, Some("http://example.com#anchor".to_string()));

        let info = detect_mapping_key("link: https://api.example.com/v1#section # comment", 0);
        assert!(info.is_some(), "Key with URL hash and inline comment should be detected");
        let info = info.unwrap();
        assert_eq!(info.key, "link");
        assert_eq!(info.value, Some("https://api.example.com/v1#section".to_string()));

        let info = detect_mapping_key("text: value # hash # in # value", 0);
        assert!(info.is_some(), "Key with hash characters in value should be detected");
        let info = info.unwrap();
        // The first # preceded by whitespace starts the comment, so value should be "value"
        assert_eq!(info.value, Some("value".to_string()));
    }

    #[test]
    fn test_key_detection_preserves_quoted_hashes() {
        // Hashes in quoted strings should be preserved
        let info = detect_mapping_key("key: \"value with # hash\"", 0);
        assert!(info.is_some(), "Key with quoted hash should be detected");
        let info = info.unwrap();
        assert_eq!(info.value, Some("\"value with # hash\"".to_string()));

        let info = detect_mapping_key("key: 'value #2' # inline comment", 0);
        assert!(info.is_some(), "Key with quoted hash and inline comment should be detected");
        let info = info.unwrap();
        assert_eq!(info.value, Some("'value #2'".to_string()));
    }

    #[test]
    fn test_key_detection_comment_lines_with_colons_ignored() {
        // Comment lines containing colons should be ignored
        let info = detect_mapping_key("# TODO: fix this bug", 0);
        assert!(info.is_none(), "Comment line with colon should not be detected as key");

        let info = detect_mapping_key("  # Note: this is important", 0);
        assert!(info.is_none(), "Indented comment line with colon should not be detected as key");

        let info = detect_mapping_key("# http://example.com#anchor", 0);
        assert!(info.is_none(), "Comment line with URL should not be detected as key");
    }

    #[test]
    fn test_key_detection_non_comment_lines_unchanged() {
        // Regular non-comment lines should work as before
        let info = detect_mapping_key("key: value", 0);
        assert!(info.is_some(), "Regular key-value pair should be detected");
        let info = info.unwrap();
        assert_eq!(info.key, "key");
        assert_eq!(info.value, Some("value".to_string()));

        let info = detect_mapping_key("  nested: value", 0);
        assert!(info.is_some(), "Nested key should be detected");

        let info = detect_mapping_key("parent:", 0);
        assert!(info.is_some(), "Parent key should be detected");
        let info = info.unwrap();
        assert!(info.is_parent_key);
    }
}
