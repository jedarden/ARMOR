//! YAML Syntax Error Detection Module
//!
//! This module provides comprehensive syntax error detection for YAML files.
//! It detects three main categories of errors:
//!
//! 1. **Indentation Errors**: Mixed spaces/tabs, inconsistent indentation levels
//! 2. **Delimiter Errors**: Missing colons, unbalanced brackets/braces, quote errors
//! 3. **Structure Errors**: Invalid mappings, malformed sequences, duplicate keys
//!
//! ## Architecture
//!
//! The detector follows a line-by-line analysis approach:
//!
//! - LineTracker: Tracks line numbers and content for error reporting
//! - IndentationDetector: Analyzes indentation patterns
//! - DelimiterDetector: Checks delimiter consistency
//! - StructureDetector: Validates YAML structure rules
//!
//! ## Usage
//!
//! ```ignore
//! use armor::parsers::yaml::syntax_detector::{SyntaxDetector, SyntaxError};
//!
//! let detector = SyntaxDetector::new();
//! let yaml_content = "key: value\n  nested: value";
//! let errors = detector.detect_errors(yaml_content);
//!
//! for error in errors {
//!     println!("Line {}: {}", error.line, error.message);
//! }
//! ```

use crate::parsers::yaml::types::{ValidationError, ValidationResult};
use std::collections::{HashMap, HashSet};
use std::fmt;

/// Classification of indentation error types
///
/// This enum provides structured categorization of different indentation
/// error types that can occur in YAML files.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum IndentationErrorType {
    /// Mixed tabs and spaces in the same line
    MixedTabsAndSpaces,
    /// Indentation not a multiple of the base indent size
    InvalidIndentLevel,
    /// Indentation increase is too large (> 2 levels at once)
    ExcessiveIndentIncrease,
    /// Indentation increase is not a multiple of base indent size
    InvalidIndentIncrease,
    /// Tab character detected (when tabs are not allowed)
    TabCharacter,
}

impl IndentationErrorType {
    /// Get a human-readable description of this error type
    pub fn description(&self) -> &'static str {
        match self {
            Self::MixedTabsAndSpaces => "mixed tabs and spaces in indentation",
            Self::InvalidIndentLevel => "indentation level is not a multiple of base indent size",
            Self::ExcessiveIndentIncrease => "indentation increase is too large",
            Self::InvalidIndentIncrease => "indentation increase is not a multiple of base indent size",
            Self::TabCharacter => "tab character detected in indentation",
        }
    }

    /// Get a short code for this error type
    pub fn code(&self) -> &'static str {
        match self {
            Self::MixedTabsAndSpaces => "E001",
            Self::InvalidIndentLevel => "E002",
            Self::ExcessiveIndentIncrease => "E003",
            Self::InvalidIndentIncrease => "E004",
            Self::TabCharacter => "E005",
        }
    }

    /// Get all indentation error types
    pub fn all() -> &'static [Self] {
        &[
            Self::MixedTabsAndSpaces,
            Self::InvalidIndentLevel,
            Self::ExcessiveIndentIncrease,
            Self::InvalidIndentIncrease,
            Self::TabCharacter,
        ]
    }
}

impl fmt::Display for IndentationErrorType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.code(), self.description())
    }
}

/// Classification of delimiter error types
///
/// This enum provides structured categorization of different delimiter
/// error types that can occur in YAML files.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DelimiterErrorType {
    /// Missing colon after a key
    MissingColon,
    /// Unmatched opening bracket '['
    UnmatchedOpeningBracket,
    /// Unmatched closing bracket ']'
    UnmatchedClosingBracket,
    /// Unclosed bracket '['
    UnclosedBracket,
    /// Unmatched opening brace '{'
    UnmatchedOpeningBrace,
    /// Unmatched closing brace '}'
    UnmatchedClosingBrace,
    /// Unclosed brace '{'
    UnclosedBrace,
    /// Mismatched quotes (single inside double or vice versa)
    MismatchedQuotes,
    /// Unclosed single quote
    UnclosedSingleQuote,
    /// Unclosed double quote
    UnclosedDoubleQuote,
}

impl DelimiterErrorType {
    /// Get a human-readable description of this error type
    pub fn description(&self) -> &'static str {
        match self {
            Self::MissingColon => "missing colon after key",
            Self::UnmatchedOpeningBracket => "unmatched opening bracket '['",
            Self::UnmatchedClosingBracket => "unmatched closing bracket ']'",
            Self::UnclosedBracket => "unclosed bracket '['",
            Self::UnmatchedOpeningBrace => "unmatched opening brace '{'",
            Self::UnmatchedClosingBrace => "unmatched closing brace '}'",
            Self::UnclosedBrace => "unclosed brace '{'",
            Self::MismatchedQuotes => "mismatched quotes",
            Self::UnclosedSingleQuote => "unclosed single quote",
            Self::UnclosedDoubleQuote => "unclosed double quote",
        }
    }

    /// Get a short code for this error type
    pub fn code(&self) -> &'static str {
        match self {
            Self::MissingColon => "D001",
            Self::UnmatchedOpeningBracket => "D002",
            Self::UnmatchedClosingBracket => "D003",
            Self::UnclosedBracket => "D004",
            Self::UnmatchedOpeningBrace => "D005",
            Self::UnmatchedClosingBrace => "D006",
            Self::UnclosedBrace => "D007",
            Self::MismatchedQuotes => "D008",
            Self::UnclosedSingleQuote => "D009",
            Self::UnclosedDoubleQuote => "D010",
        }
    }

    /// Get all delimiter error types
    pub fn all() -> &'static [Self] {
        &[
            Self::MissingColon,
            Self::UnmatchedOpeningBracket,
            Self::UnmatchedClosingBracket,
            Self::UnclosedBracket,
            Self::UnmatchedOpeningBrace,
            Self::UnmatchedClosingBrace,
            Self::UnclosedBrace,
            Self::MismatchedQuotes,
            Self::UnclosedSingleQuote,
            Self::UnclosedDoubleQuote,
        ]
    }
}

impl fmt::Display for DelimiterErrorType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.code(), self.description())
    }
}

/// Main syntax detector for YAML content
///
/// This struct orchestrates all syntax detection checks and provides
/// a unified interface for error detection.
#[derive(Debug, Clone)]
pub struct SyntaxDetector {
    /// Configuration for detection behavior
    config: DetectorConfig,
    /// Track indentation state across lines
    indentation_state: IndentationState,
    /// Track delimiter state (brackets, braces, quotes)
    delimiter_state: DelimiterState,
    /// Track structure state (keys, sequences, mappings)
    structure_state: StructureState,
}

/// Configuration for syntax detection behavior
#[derive(Debug, Clone)]
pub struct DetectorConfig {
    /// Whether to detect tab/space mixing
    pub detect_mixed_indentation: bool,
    /// Whether to check for consistent indentation (multiples of base)
    pub check_consistent_indentation: bool,
    /// Base indentation size (usually 2 or 4)
    pub base_indent_size: usize,
    /// Whether to validate delimiter balancing
    pub validate_delimiter_balance: bool,
    /// Whether to detect duplicate keys
    pub detect_duplicate_keys: bool,
    /// Whether to detect invalid sequence syntax
    pub detect_invalid_sequences: bool,
    /// Whether to detect invalid mapping syntax
    pub detect_invalid_mappings: bool,
}

impl Default for DetectorConfig {
    fn default() -> Self {
        Self {
            detect_mixed_indentation: true,
            check_consistent_indentation: true,
            base_indent_size: 2,
            validate_delimiter_balance: true,
            detect_duplicate_keys: true,
            detect_invalid_sequences: true,
            detect_invalid_mappings: true,
        }
    }
}

/// State tracking for indentation analysis
#[derive(Debug, Clone, Default)]
struct IndentationState {
    /// Whether tabs have been seen in the file
    has_tabs: bool,
    /// Whether spaces have been seen in the file
    has_spaces: bool,
    /// Previous line's indentation level (in spaces)
    prev_indent_level: usize,
    /// Expected indentation levels for nested contexts
    indent_stack: Vec<usize>,
    /// Lines with mixed indentation
    mixed_lines: Vec<usize>,
}

/// State tracking for delimiter analysis
#[derive(Debug, Clone, Default)]
struct DelimiterState {
    /// Stack of opening brackets/braces
    bracket_stack: Vec<(char, usize)>, // (bracket_type, line_number)
    /// Current quote state (None, Single, Double)
    quote_state: Option<QuoteType>,
    /// Lines with quote errors
    quote_errors: Vec<usize>,
    /// Whether we're inside a multiline block (|, >)
    in_multiline_block: bool,
    /// Indentation level of the multiline block start
    multiline_block_indent: usize,
    /// Whether we're inside flow-style context (within [] or {})
    in_flow_context: bool,
}

/// Quote type for tracking
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum QuoteType {
    Single,
    Double,
}

/// A scope representing a mapping context at a specific nesting level
#[derive(Debug, Clone)]
struct Scope {
    /// Indentation level (number of leading spaces)
    indent_level: usize,
    /// Keys defined within this scope
    keys: HashSet<String>,
    /// Line number where this scope started (for error reporting)
    start_line: usize,
    /// Parent key that created this scope (e.g., "web" in "services: {...}")
    parent_key: Option<String>,
    /// Whether this scope is in flow-style mapping ({key: value})
    is_flow_style: bool,
}

impl Scope {
    /// Create a new scope
    fn new(indent_level: usize, start_line: usize, parent_key: Option<String>) -> Self {
        Self {
            indent_level,
            keys: HashSet::new(),
            start_line,
            parent_key,
            is_flow_style: false,
        }
    }

    /// Add a key to this scope, returning true if it's a duplicate
    fn add_key(&mut self, key: &str) -> bool {
        !self.keys.insert(key.to_string())
    }

    /// Check if this scope contains a key
    fn contains_key(&self, key: &str) -> bool {
        self.keys.contains(key)
    }
}

/// Hierarchical stack of active scopes
#[derive(Debug, Clone)]
struct ScopeStack {
    /// Stack of active scopes (top = current scope)
    scopes: Vec<Scope>,
    /// Base indentation size (usually 2 or 4 spaces)
    base_indent: usize,
}

impl ScopeStack {
    /// Create a new scope stack
    fn new(base_indent: usize) -> Self {
        Self {
            scopes: vec![Scope::new(0, 0, None)], // Root scope
            base_indent,
        }
    }

    /// Get the current scope (top of stack)
    fn current_scope(&mut self) -> &mut Scope {
        self.scopes.last_mut().expect("Scope stack should never be empty")
    }

    /// Get scope for a specific indentation level
    fn get_scope_at_level(&self, indent_level: usize) -> Option<&Scope> {
        self.scopes.iter().find(|s| s.indent_level == indent_level)
    }

    /// Enter a new scope (when indent increases)
    fn enter_scope(&mut self, indent_level: usize, line: usize, parent_key: Option<String>) {
        // Remove all scopes deeper than this level (fresh start for sibling mappings)
        self.scopes.retain(|s| s.indent_level < indent_level);

        // Create a fresh scope at this level
        let new_scope = Scope::new(indent_level, line, parent_key);
        self.scopes.push(new_scope);
    }

    /// Exit to parent scope (when indent decreases)
    fn exit_to_scope(&mut self, target_indent: usize) {
        // Remove all scopes deeper than target
        self.scopes.retain(|s| s.indent_level <= target_indent);
    }

    /// Check if current scope contains a key
    fn contains_key(&self, key: &str) -> bool {
        self.scopes.last()
            .map(|scope| scope.contains_key(key))
            .unwrap_or(false)
    }

    /// Add a key to current scope
    fn add_key(&mut self, key: &str) {
        let scope = self.current_scope();
        scope.add_key(key);
    }

    /// Get human-readable path to current scope
    fn get_scope_path(&self) -> String {
        let mut path = Vec::new();
        for scope in &self.scopes {
            if let Some(ref key) = scope.parent_key {
                path.push(key.clone());
            }
        }
        path.join(".")
    }

    /// Get current indent level
    fn current_indent(&self) -> usize {
        self.scopes.last()
            .map(|scope| scope.indent_level)
            .unwrap_or(0)
    }
}

impl Default for ScopeStack {
    fn default() -> Self {
        Self::new(2) // Default base indent of 2 spaces
    }
}

/// State tracking for structure analysis
#[derive(Debug, Clone, Default)]
struct StructureState {
    /// Stack of nested structures (mapping, sequence, etc.)
    context_stack: Vec<StructureContext>,
    /// Hierarchical scope-based key tracking
    scope_stack: ScopeStack,
    /// Previous line's indentation level
    prev_indent: usize,
}

/// Current structure context
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum StructureContext {
    Mapping,
    Sequence,
    FlowMapping,
    FlowSequence,
}

impl SyntaxDetector {
    /// Create a new syntax detector with default configuration
    pub fn new() -> Self {
        Self::with_config(DetectorConfig::default())
    }

    /// Create a new syntax detector with custom configuration
    pub fn with_config(config: DetectorConfig) -> Self {
        Self {
            config,
            indentation_state: IndentationState::default(),
            delimiter_state: DelimiterState::default(),
            structure_state: StructureState::default(),
        }
    }

    /// Detect all syntax errors in YAML content
    ///
    /// This is the main entry point for syntax detection. It runs all
    /// enabled detectors and returns a comprehensive list of errors.
    ///
    /// # Arguments
    /// * `content` - The YAML content to analyze
    ///
    /// # Returns
    /// A vector of validation errors representing all detected syntax issues
    pub fn detect_errors(&mut self, content: &str) -> Vec<ValidationError> {
        let mut errors = Vec::new();
        let lines: Vec<&str> = content.lines().collect();

        // Reset state for new detection
        self.reset_state();

        for (line_num, line) in lines.iter().enumerate() {
            let line_num_1index = line_num + 1; // Convert to 1-indexed

            // Skip empty lines and comments for most checks
            let is_meaningful = !line.trim().is_empty() && !line.trim().starts_with('#');

            if is_meaningful {
                // Run all enabled detectors
                if self.config.detect_mixed_indentation || self.config.check_consistent_indentation {
                    self.detect_indentation_errors(line, line_num_1index, &mut errors);
                }

                if self.config.validate_delimiter_balance {
                    self.detect_delimiter_errors(line, line_num_1index, &mut errors);
                }

                if self.config.detect_invalid_sequences || self.config.detect_invalid_mappings {
                    self.detect_structure_errors(line, line_num_1index, &mut errors);
                }

                if self.config.detect_duplicate_keys {
                    self.detect_duplicate_key_errors(line, line_num_1index, &mut errors);
                }
            }
        }

        // Final validation checks
        self.finalize_delimiter_checks(&mut errors);
        self.finalize_structure_checks(&mut errors);

        errors
    }

    /// Detect syntax errors and convert to ValidationResult
    pub fn detect_to_validation_result(&mut self, content: &str) -> ValidationResult {
        let errors = self.detect_errors(content);

        if errors.is_empty() {
            ValidationResult::success()
        } else {
            ValidationResult::failure(errors)
        }
    }

    /// Reset all state tracking for a new detection run
    fn reset_state(&mut self) {
        self.indentation_state = IndentationState::default();
        self.delimiter_state = DelimiterState::default();
        self.structure_state = StructureState::default();
    }

    /// Detect indentation-related errors
    fn detect_indentation_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
        let leading_whitespace_len = self.get_leading_whitespace_length(line);

        // Check for mixed tabs and spaces in leading whitespace
        if self.config.detect_mixed_indentation && leading_whitespace_len > 0 {
            let leading_chars: Vec<char> = line.chars().take(leading_whitespace_len).collect();
            let has_tabs = leading_chars.contains(&'\t');
            let has_spaces = leading_chars.contains(&' ');

            if has_tabs && has_spaces {
                self.indentation_state.has_tabs = true;
                self.indentation_state.has_spaces = true;
                self.indentation_state.mixed_lines.push(line_num);

                let error_type = IndentationErrorType::MixedTabsAndSpaces;
                errors.push(ValidationError::new(
                    format!("line_{}", line_num),
                    error_type.description()
                ).with_line(line_num).with_indentation_error_type(error_type));
            } else if has_tabs {
                self.indentation_state.has_tabs = true;
                // Optionally report tab-only indentation
                if self.config.detect_mixed_indentation {
                    let error_type = IndentationErrorType::TabCharacter;
                    errors.push(ValidationError::new(
                        format!("line_{}", line_num),
                        format!("{}: {}", error_type.code(), error_type.description())
                    ).with_line(line_num).with_indentation_error_type(error_type));
                }
            } else if has_spaces {
                self.indentation_state.has_spaces = true;
            }
        }

        // Check for consistent indentation levels
        if self.config.check_consistent_indentation && leading_whitespace_len > 0 {
            let indent_level = leading_whitespace_len;

            // Check if indentation is a multiple of base_indent_size
            if indent_level % self.config.base_indent_size != 0 {
                let error_type = IndentationErrorType::InvalidIndentLevel;
                errors.push(ValidationError::new(
                    format!("line_{}", line_num),
                    format!("{}: {} spaces is not a multiple of {}",
                            error_type.code(), indent_level, self.config.base_indent_size)
                ).with_line(line_num).with_indentation_error_type(error_type));
            }

            // Check if indentation increase is consistent
            if indent_level > self.indentation_state.prev_indent_level {
                let increase = indent_level - self.indentation_state.prev_indent_level;

                // Check if increase is too large (more than 2 levels at once)
                if increase > self.config.base_indent_size * 2 {
                    let error_type = IndentationErrorType::ExcessiveIndentIncrease;
                    errors.push(ValidationError::new(
                        format!("line_{}", line_num),
                        format!("{}: increase of {} spaces exceeds maximum of {} levels",
                                error_type.code(), increase, self.config.base_indent_size * 2)
                    ).with_line(line_num).with_indentation_error_type(error_type));
                }

                if increase % self.config.base_indent_size != 0 {
                    let error_type = IndentationErrorType::InvalidIndentIncrease;
                    errors.push(ValidationError::new(
                        format!("line_{}", line_num),
                        format!("{}: increase of {} spaces is not a multiple of {}",
                                error_type.code(), increase, self.config.base_indent_size)
                    ).with_line(line_num).with_indentation_error_type(error_type));
                } else {
                    // Valid increase - push to stack
                    self.indentation_state.indent_stack.push(indent_level);
                }
            } else if indent_level < self.indentation_state.prev_indent_level {
                // Decrease in indentation - pop from stack
                while !self.indentation_state.indent_stack.is_empty()
                      && *self.indentation_state.indent_stack.last().unwrap() > indent_level {
                    self.indentation_state.indent_stack.pop();
                }
            }

            self.indentation_state.prev_indent_level = indent_level;
        }
    }

    /// Detect delimiter-related errors
    fn detect_delimiter_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
        let leading_whitespace_len = self.get_leading_whitespace_length(line);
        let trimmed = line.trim();

        // Check if we're inside a multiline block
        if self.delimiter_state.in_multiline_block {
            // Multiline blocks continue until we find a line with less or equal indentation
            // that's not empty or a comment
            if leading_whitespace_len > self.delimiter_state.multiline_block_indent ||
               (leading_whitespace_len == self.delimiter_state.multiline_block_indent && (trimmed.is_empty() || trimmed.starts_with('#'))) {
                return; // Skip lines inside the multiline block
            } else {
                // Exit multiline block when indentation decreases
                self.delimiter_state.in_multiline_block = false;
                self.delimiter_state.multiline_block_indent = 0;
            }
        }

        // Skip empty lines and comments (but not when inside multiline blocks - handled above)
        if trimmed.is_empty() || trimmed.starts_with('#') {
            return;
        }

        // Handle document markers (---, ...)
        if trimmed == "---" || trimmed == "..." {
            return; // Document markers should not be flagged for missing colons
        }

        // Handle multiline block scalars (|, >, |-, |- , >-, >+, |+)
        // These appear after a colon, like "key: |" or "key: >"
        if trimmed.contains(": |") || trimmed.contains(": >") ||
           trimmed.contains(":|-") || trimmed.contains(":>-") ||
           trimmed.contains(":|+") || trimmed.contains(":>+") {
            self.delimiter_state.in_multiline_block = true;
            self.delimiter_state.multiline_block_indent = leading_whitespace_len; // Content must be MORE indented than this line
        }

        // Check for missing colons after keys
        if !trimmed.starts_with('-') && // Not a sequence item
           !trimmed.starts_with('?') && // Not an explicit key
           !trimmed.starts_with(':') && // Not a value
           !trimmed.starts_with('&') && // Not an anchor
           !trimmed.starts_with('*') && // Not an alias
           !trimmed.starts_with('!') && // Not a tag
           !trimmed.starts_with('|') && // Not a literal block scalar
           !trimmed.starts_with('>') && // Not a folded block scalar
           !trimmed.contains('{') && // Not flow style mapping
           !trimmed.contains('[') && // Not flow style sequence
           !trimmed.contains(':') && // No colon at all
           !trimmed.is_empty() &&
           self.looks_like_key(trimmed) {

            let error_type = DelimiterErrorType::MissingColon;
            errors.push(ValidationError::new(
                format!("line_{}", line_num),
                format!("{}: {}", error_type.code(), error_type.description())
            ).with_line(line_num).with_delimiter_error_type(error_type));
        }

        // Track bracket/brace balance and flow context
        for (char_pos, ch) in line.chars().enumerate() {
            match ch {
                '[' => {
                    self.delimiter_state.bracket_stack.push(('[', line_num));
                    self.delimiter_state.in_flow_context = true;
                }
                ']' => {
                    if let Some(('[', _)) = self.delimiter_state.bracket_stack.last() {
                        self.delimiter_state.bracket_stack.pop();
                        // Update flow context when stack becomes empty
                        self.delimiter_state.in_flow_context = !self.delimiter_state.bracket_stack.is_empty();
                    } else {
                        let error_type = DelimiterErrorType::UnmatchedClosingBracket;
                        errors.push(ValidationError::new(
                            format!("line_{}", line_num),
                            format!("{}: {}", error_type.code(), error_type.description())
                        ).with_line(line_num).with_delimiter_error_type(error_type));
                    }
                }
                '{' => {
                    self.delimiter_state.bracket_stack.push(('{', line_num));
                    self.delimiter_state.in_flow_context = true;
                }
                '}' => {
                    if let Some(('{', _)) = self.delimiter_state.bracket_stack.last() {
                        self.delimiter_state.bracket_stack.pop();
                        // Update flow context when stack becomes empty
                        self.delimiter_state.in_flow_context = !self.delimiter_state.bracket_stack.is_empty();
                    } else {
                        let error_type = DelimiterErrorType::UnmatchedClosingBrace;
                        errors.push(ValidationError::new(
                            format!("line_{}", line_num),
                            format!("{}: {}", error_type.code(), error_type.description())
                        ).with_line(line_num).with_delimiter_error_type(error_type));
                    }
                }
                '\'' => {
                    match self.delimiter_state.quote_state {
                        None => self.delimiter_state.quote_state = Some(QuoteType::Single),
                        Some(QuoteType::Single) => self.delimiter_state.quote_state = None,
                        Some(QuoteType::Double) => {
                            // Mismatched quote
                            let error_type = DelimiterErrorType::MismatchedQuotes;
                            errors.push(ValidationError::new(
                                format!("line_{}", line_num),
                                format!("{}: {} (single inside double)", error_type.code(), error_type.description())
                            ).with_line(line_num).with_delimiter_error_type(error_type));
                            self.delimiter_state.quote_errors.push(line_num);
                        }
                    }
                }
                '"' => {
                    match self.delimiter_state.quote_state {
                        None => self.delimiter_state.quote_state = Some(QuoteType::Double),
                        Some(QuoteType::Double) => self.delimiter_state.quote_state = None,
                        Some(QuoteType::Single) => {
                            // Mismatched quote
                            let error_type = DelimiterErrorType::MismatchedQuotes;
                            errors.push(ValidationError::new(
                                format!("line_{}", line_num),
                                format!("{}: {} (double inside single)", error_type.code(), error_type.description())
                            ).with_line(line_num).with_delimiter_error_type(error_type));
                            self.delimiter_state.quote_errors.push(line_num);
                        }
                    }
                }
                _ => {}
            }
        }
    }

    /// Detect structure-related errors
    fn detect_structure_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
        let trimmed = line.trim();

        // Detect invalid sequence syntax
        if self.config.detect_invalid_sequences {
            if trimmed.starts_with("- ") {
                // Valid sequence item
                if self.structure_state.context_stack.is_empty() ||
                   !matches!(self.structure_state.context_stack.last(), Some(StructureContext::Sequence)) {
                    self.structure_state.context_stack.push(StructureContext::Sequence);
                }
            } else if trimmed.starts_with('-') && trimmed.len() > 1 {
                // Dash followed by something other than space
                if let Some(next_char) = trimmed.chars().nth(1) {
                    if !next_char.is_whitespace() {
                        errors.push(ValidationError::new(
                            format!("line_{}", line_num),
                            "sequence dash '-' must be followed by whitespace"
                        ).with_line(line_num));
                    }
                }
            }
        }

        // Detect invalid mapping syntax
        if self.config.detect_invalid_mappings {
            if let Some(colon_pos) = trimmed.find(':') {
                let before_colon = &trimmed[..colon_pos];

                // Check if colon is properly used after a key
                if !before_colon.is_empty() && !before_colon.ends_with(' ') {
                    // Valid key: value pair
                    if self.structure_state.context_stack.is_empty() ||
                       !matches!(self.structure_state.context_stack.last(), Some(StructureContext::Mapping)) {
                        self.structure_state.context_stack.push(StructureContext::Mapping);
                    }
                } else if before_colon.is_empty() {
                    // Colon at start - might be a value or error
                    if !trimmed.starts_with(": ") && trimmed != ":" {
                        errors.push(ValidationError::new(
                            format!("line_{}", line_num),
                            "colon ':' at start must be followed by space"
                        ).with_line(line_num));
                    }
                }
            }
        }

        // Check for flow syntax indicators
        if trimmed.contains('{') || trimmed.contains('[') {
            self.structure_state.context_stack.push(if trimmed.contains('{') {
                StructureContext::FlowMapping
            } else {
                StructureContext::FlowSequence
            });
        }
    }

    /// Detect duplicate key errors using scope-aware tracking
    fn detect_duplicate_key_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
        // Skip duplicate key detection when inside flow-style contexts ([] or {})
        // Flow-style YAML uses {key: value} syntax which should not be treated as duplicate keys
        if self.delimiter_state.in_flow_context {
            return;
        }

        let trimmed = line.trim();
        let indent = self.get_leading_whitespace_length(line);

        // Extract key if this is a key-value pair
        if let Some(colon_pos) = trimmed.find(':') {
            let key_part = &trimmed[..colon_pos];

            // Check if this is a parent key (ends with colon and no value on same line)
            // A parent key typically has nothing after the colon or just whitespace/comments
            let after_colon = trimmed[colon_pos + 1..].trim();
            let is_parent_key = after_colon.is_empty() ||
                                after_colon.starts_with('#');

            if is_parent_key {
                // This is a parent key - extract the key name and enter a new scope
                if !key_part.is_empty() && !key_part.contains('-') && !key_part.contains('?') {
                    let key = key_part.trim();
                    if !key.starts_with('\'') && !key.starts_with('"') && !key.contains('#') {
                        // Enter a new scope for the parent key's nested content
                        self.structure_state.scope_stack.enter_scope(
                            indent + self.config.base_indent_size,
                            line_num,
                            Some(key.to_string())
                        );
                    }
                }
                return;
            }

            // Skip if it's a value or not a simple key
            if key_part.is_empty() || key_part.contains('-') || key_part.contains('?') {
                return;
            }

            let key = key_part.trim();

            // Skip quoted keys and special keys
            if key.starts_with('\'') || key.starts_with('"') || key.contains('#') {
                return;
            }

            // Handle scope transitions based on indentation changes
            use std::cmp::Ordering;
            match indent.cmp(&self.structure_state.scope_stack.current_indent()) {
                Ordering::Greater => {
                    // Indent increased - this should be a parent key (handled above)
                    // If we get here, it's a regular key at deeper indent without a parent
                    // Enter an anonymous scope
                    self.structure_state.scope_stack.enter_scope(indent, line_num, None);
                }
                Ordering::Less => {
                    // Indent decreased - exit to the appropriate parent scope
                    self.structure_state.scope_stack.exit_to_scope(indent);
                }
                Ordering::Equal => {
                    // Same scope level - continue checking for duplicates
                }
            }

            // Check for duplicates in the current scope
            if self.structure_state.scope_stack.contains_key(key) {
                let scope_path = self.structure_state.scope_stack.get_scope_path();
                let error_message = if scope_path.is_empty() {
                    format!("duplicate key '{}' in mapping scope", key)
                } else {
                    format!("duplicate key '{}' in mapping scope '{}'", key, scope_path)
                };
                errors.push(ValidationError::new(
                    format!("key_{}", key),
                    error_message
                ).with_line(line_num));
            } else {
                self.structure_state.scope_stack.add_key(key);
            }
        }

        // Update previous indentation for next comparison
        self.structure_state.prev_indent = indent;
    }

    /// Finalize delimiter checks after processing all lines
    fn finalize_delimiter_checks(&mut self, errors: &mut Vec<ValidationError>) {
        // Check for unclosed brackets
        for (bracket, line_num) in &self.delimiter_state.bracket_stack {
            let error_type = match *bracket {
                '[' => DelimiterErrorType::UnclosedBracket,
                '{' => DelimiterErrorType::UnclosedBrace,
                _ => continue, // Skip unknown bracket types
            };
            errors.push(ValidationError::new(
                "delimiter_balance".to_string(),
                format!("{}: {}", error_type.code(), error_type.description())
            ).with_line(*line_num).with_delimiter_error_type(error_type));
        }

        // Check for unclosed quotes
        if self.delimiter_state.quote_state.is_some() {
            let (error_type, quote_char) = match self.delimiter_state.quote_state {
                Some(QuoteType::Single) => (DelimiterErrorType::UnclosedSingleQuote, "'"),
                Some(QuoteType::Double) => (DelimiterErrorType::UnclosedDoubleQuote, "\""),
                None => unreachable!(),
            };

            errors.push(ValidationError::new(
                "quote_balance".to_string(),
                format!("{}: {}", error_type.code(), error_type.description())
            ).with_line(0).with_delimiter_error_type(error_type)); // Line 0 since we don't track where it opened
        }
    }

    /// Finalize structure checks after processing all lines
    fn finalize_structure_checks(&mut self, _errors: &mut Vec<ValidationError>) {
        // No-op - same-level duplicate detection is handled in detect_duplicate_key_errors
    }

    /// Get leading whitespace length from a line
    fn get_leading_whitespace_length(&self, line: &str) -> usize {
        line.chars().take_while(|c| c.is_whitespace()).count()
    }

    /// Check if a line looks like a key definition
    fn looks_like_key(&self, trimmed: &str) -> bool {
        // Keys are typically alphanumeric with underscores, dashes, or dots
        // They don't start with special YAML characters
        if trimmed.is_empty() {
            return false;
        }

        let first_char = trimmed.chars().next().unwrap();

        // Exclude lines that start with YAML special characters
        if matches!(first_char, '-' | ':' | '?' | '|' | '>' | '#' | '[' | ']' | '{' | '}' | '"' | '\'' | '!' | '&' | '*' | '%' | '@' | '`') {
            return false;
        }

        // Look for word characters at the start
        trimmed.chars().next().map_or(false, |c| c.is_alphanumeric() || c == '_' || c == '.')
    }
}

impl Default for SyntaxDetector {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_detect_mixed_indentation() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n\t  bad: value";
        let errors = detector.detect_errors(yaml);

        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("mixed tabs and spaces")));
    }

    #[test]
    fn test_detect_missing_colon() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key value\n  key2: value2";
        let errors = detector.detect_errors(yaml);

        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("missing colon")));
    }

    #[test]
    fn test_detect_unmatched_bracket() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: [value1, value2";
        let errors = detector.detect_errors(yaml);

        assert!(!errors.is_empty());
        assert!(errors.iter().any(|e| e.message.contains("unclosed")));
    }

    #[test]
    fn test_valid_yaml_no_errors() {
        let mut detector = SyntaxDetector::new();
        let yaml = "key: value\n  nested:\n    - item1\n    - item2";
        let errors = detector.detect_errors(yaml);

        assert!(errors.is_empty());
    }
}
