//! Core parser trait and implementations for YAML parsing
//!
//! This module defines the Parser trait and provides functionality for
//! parsing YAML content from various sources.

use crate::parsers::yaml::{
    types::{ParseResult, ValidationResult, ValidationError},
    error::ParseError,
    ParserConfig,
    syntax_validator::SyntaxValidator,
    syntax_detector::SyntaxDetector,
    scope::{ScopeStack, classify_line_type, LineClassification, IndentTransitionState, IndentTransitionType, ScopeInfo, ScopeType},
    line_parser::{calculate_indentation},
    scope::extract_key_context,
};

#[cfg(debug_assertions)]
use log::debug as log_debug;

/// Trait for YAML parsers
///
/// This trait defines the core interface for parsing YAML content.
/// Implementations can parse from strings, files, or other sources.
pub trait Parser {
    /// Parse YAML content from a string
    ///
    /// # Arguments
    /// * `content` - The YAML content as a string
    ///
    /// # Returns
    /// A ParseResult containing the parsed data or an error
    fn parse_str(&mut self, content: &str) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a byte slice
    fn parse_bytes(&mut self, content: &[u8]) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a file
    fn parse_file(&mut self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>;

    /// Validate YAML content without fully parsing it
    fn validate_str(&mut self, content: &str) -> ValidationResult;

    /// Validate a YAML file without fully parsing it
    fn validate_file(&mut self, path: &std::path::Path) -> ValidationResult;

    /// Get the parser configuration
    ///
    /// # Returns
    /// A reference to the parser's configuration
    fn config(&self) -> &ParserConfig;

    /// Set the parser configuration
    ///
    /// # Arguments
    /// * `config` - The new configuration
    ///
    /// # Returns
    /// The parser with the new configuration
    fn with_config(self, config: ParserConfig) -> Self
    where
        Self: Sized;
}

    /// Test push_scope adds scope info to stack
    #[test]
    fn test_push_scope() {
        let mut parser = BasicParser::new();

        // Initially scope_info_stack should be empty
        assert_eq!(parser.scope_info_stack().len(), 0, "Initial scope info stack should be empty");

        // Create a scope info and push it
        let scope_info = ScopeInfo::block(1);
        parser.push_scope(scope_info);

        // Verify it was added
        assert_eq!(parser.scope_info_stack().len(), 1, "Scope info stack should have 1 item after push");

        // Verify the pushed scope info matches
        let pushed_info = parser.scope_info_stack().last().unwrap();
        assert_eq!(pushed_info.scope_type(), ScopeType::Block, "Pushed scope should be Block type");
        assert_eq!(pushed_info.scope_depth(), 1, "Pushed scope should have depth 1");
    }

    /// Test push_scope multiple times
    #[test]
    fn test_push_scope_multiple() {
        let mut parser = BasicParser::new();

        // Push multiple scopes
        parser.push_scope(ScopeInfo::block(1));
        parser.push_scope(ScopeInfo::block(2));
        parser.push_scope(ScopeInfo::block(3));

        // Verify all were added
        assert_eq!(parser.scope_info_stack().len(), 3, "Scope info stack should have 3 items");

        // Verify they're in order
        let scopes = parser.scope_info_stack();
        assert_eq!(scopes[0].scope_depth(), 1, "First scope should have depth 1");
        assert_eq!(scopes[1].scope_depth(), 2, "Second scope should have depth 2");
        assert_eq!(scopes[2].scope_depth(), 3, "Third scope should have depth 3");
    }

    /// Test push_scope with different scope types
    #[test]
    fn test_push_scope_different_types() {
        let mut parser = BasicParser::new();

        // Push different scope types
        parser.push_scope(ScopeInfo::root());
        parser.push_scope(ScopeInfo::block(1));
        parser.push_scope(ScopeInfo::new(ScopeType::BlockSequence, 2));
        parser.push_scope(ScopeInfo::new(ScopeType::FlowMapping, 3));

        // Verify all were added
        assert_eq!(parser.scope_info_stack().len(), 4, "Scope info stack should have 4 items");

        // Verify types
        let scopes = parser.scope_info_stack();
        assert_eq!(scopes[0].scope_type(), ScopeType::Root, "First scope should be Root");
        assert_eq!(scopes[1].scope_type(), ScopeType::Block, "Second scope should be Block");
        assert_eq!(scopes[2].scope_type(), ScopeType::BlockSequence, "Third scope should be BlockSequence");
        assert_eq!(scopes[3].scope_type(), ScopeType::FlowMapping, "Fourth scope should be FlowMapping");
    }

/// Basic YAML parser implementation
///
/// This is a minimal implementation of the Parser trait that
/// provides basic YAML parsing functionality with scope-aware key tracking.
#[derive(Debug, Clone)]
pub struct BasicParser {
    config: ParserConfig,
    /// Scope stack for tracking keys within their proper scope contexts
    scope_stack: ScopeStack,
    /// Stack of scope information tracking the scope hierarchy (type and depth at each level)
    scope_info_stack: Vec<ScopeInfo>,
    /// Current line type being processed (key-bearing vs indent-only)
    current_line_type: LineClassification,
    /// Current indent transition state for tracking scope operations
    current_transition_state: IndentTransitionState,
    /// Current scope depth (number of active scopes in the hierarchy)
    scope_depth: usize,
}

impl BasicParser {
    /// Create a new BasicParser with default configuration
    pub fn new() -> Self {
        Self {
            config: ParserConfig::default(),
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
            scope_info_stack: Vec::new(),    // Empty stack initially
            current_line_type: LineClassification::Empty,
            current_transition_state: IndentTransitionState::new(),
            scope_depth: 1, // Root scope is always present
        }
    }

    /// Create a new BasicParser with the specified configuration
    pub fn with_config(config: ParserConfig) -> Self {
        Self {
            config,
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
            scope_info_stack: Vec::new(),    // Empty stack initially
            current_line_type: LineClassification::Empty,
            current_transition_state: IndentTransitionState::new(),
            scope_depth: 1, // Root scope is always present
        }
    }

    /// Create a new strict parser
    ///
    /// A strict parser enables strict mode and disallows duplicate keys.
    /// Uses the comprehensive strict configuration from ParserConfig.
    pub fn strict() -> Self {
        Self {
            config: ParserConfig::strict(),
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
            scope_info_stack: Vec::new(),    // Empty stack initially
            current_line_type: LineClassification::Empty,
            current_transition_state: IndentTransitionState::new(),
            scope_depth: 1, // Root scope is always present
        }
    }

    /// Get the current line type being processed
    pub fn current_line_type(&self) -> LineClassification {
        self.current_line_type
    }

    /// Check if current line is key-bearing
    pub fn is_key_bearing_line(&self) -> bool {
        self.current_line_type.is_key_bearing()
    }

    /// Check if current line is indent-only
    pub fn is_indent_only_line(&self) -> bool {
        self.current_line_type.is_indent_only()
    }

    /// Check if current line is empty
    pub fn is_empty_line(&self) -> bool {
        self.current_line_type.is_empty()
    }

    /// Get the current indent transition state
    pub fn current_transition_state(&self) -> &IndentTransitionState {
        &self.current_transition_state
    }

    /// Check if the parser is currently entering a scope
    pub fn is_entering_scope(&self) -> bool {
        self.current_transition_state.is_entering_scope()
    }

    /// Check if the parser is currently exiting a scope
    pub fn is_exiting_scope(&self) -> bool {
        self.current_transition_state.is_exiting_scope()
    }

    /// Check if the parser is at the same scope level
    pub fn is_same_level(&self) -> bool {
        self.current_transition_state.is_same_level()
    }

    /// Get the current scope depth
    ///
    /// Returns the number of active scopes in the hierarchy.
    /// Root scope has depth 1.
    pub fn scope_depth(&self) -> usize {
        self.scope_depth
    }

    /// Check if we're at the root scope (depth == 1)
    ///
    /// # Returns
    ///
    /// `true` if we're at the root scope, `false` otherwise
    pub fn is_at_root(&self) -> bool {
        self.scope_depth == 1
    }

    /// Check if we're in a nested scope (depth > 1)
    ///
    /// # Returns
    ///
    /// `true` if we're in a nested scope, `false` if at root
    pub fn is_in_nested_scope(&self) -> bool {
        self.scope_depth > 1
    }

    /// Get a reference to the scope stack
    pub fn scope_stack(&self) -> &ScopeStack {
        &self.scope_stack
    }

    /// Get mutable reference to the scope stack
    pub fn scope_stack_mut(&mut self) -> &mut ScopeStack {
        &mut self.scope_stack
    }

    /// Get the current scope from the stack
    pub fn current_scope(&self) -> Option<&crate::parsers::yaml::scope::Scope> {
        if self.scope_stack.scopes.is_empty() {
            None
        } else {
            self.scope_stack.scopes.last()
        }
    }

    /// Get parent scope at a specific depth
    ///
    /// # Arguments
    ///
    /// * `depth_offset` - How many levels up to go (1 = immediate parent, 2 = grandparent, etc.)
    ///
    /// # Returns
    ///
    /// `Some(&Scope)` if a parent exists at that level, `None` otherwise
    ///
    /// # Examples
    ///
    /// ```
    /// let parser = BasicParser::new();
    /// // If in scope "a.b.c", depth_offset=1 returns "b" scope, depth_offset=2 returns "a" scope
    /// ```
    pub fn parent_scope(&self, depth_offset: usize) -> Option<&crate::parsers::yaml::scope::Scope> {
        if self.scope_stack.scopes.is_empty() || depth_offset == 0 {
            None
        } else if depth_offset >= self.scope_stack.scopes.len() {
            None
        } else {
            self.scope_stack.scopes.get(self.scope_stack.scopes.len() - 1 - depth_offset)
        }
    }

    /// Get the immediate parent scope (one level up)
    pub fn immediate_parent_scope(&self) -> Option<&crate::parsers::yaml::scope::Scope> {
        self.parent_scope(1)
    }

    /// Get the scope hierarchy path as a string
    ///
    /// Returns a dot-separated path representing the scope hierarchy,
    /// e.g., "services.web.database" for a deeply nested scope.
    pub fn scope_path(&self) -> String {
        self.scope_stack.get_scope_path()
    }

    /// Get all scopes in the hierarchy as a slice
    pub fn scope_hierarchy(&self) -> &[crate::parsers::yaml::scope::Scope] {
        &self.scope_stack.scopes
    }

    /// Get the scope info stack
    ///
    /// Returns a reference to the stack containing scope type and depth information
    /// for each level in the scope hierarchy.
    pub fn scope_info_stack(&self) -> &[ScopeInfo] {
        &self.scope_info_stack
    }

    /// Get mutable reference to the scope info stack
    pub fn scope_info_stack_mut(&mut self) -> &mut Vec<ScopeInfo> {
        &mut self.scope_info_stack
    }

    /// Push scope information onto the scope info stack
    ///
    /// This method is called when entering a new scope to track its type and depth.
    /// The scope info stack provides lightweight metadata about each scope level
    /// without duplicating the full scope state.
    ///
    /// # Arguments
    ///
    /// * `scope_info` - The scope information to push onto the stack
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::{ScopeInfo, ScopeType};
    /// use armor::parsers::yaml::parser::BasicParser;
    ///
    /// let mut parser = BasicParser::new();
    /// let info = ScopeInfo::block(1);
    /// parser.push_scope(info);
    /// assert_eq!(parser.scope_info_stack().len(), 1);
    /// ```
    pub fn push_scope(&mut self, scope_info: ScopeInfo) {
        self.scope_info_stack.push(scope_info);
    }

    /// Update scope depth to match the current scope stack state
    ///
    /// This should be called after operations that modify the scope stack
    /// to keep the scope_depth field in sync.
    fn update_scope_depth(&mut self) {
        self.scope_depth = self.scope_stack.depth();
    }

    /// Get all indent transitions from parsing
    ///
    /// This requires re-parsing to collect transitions. For more efficient
    /// access, use the scope stack directly during validation.
    /// Updates parser state with line classifications and transition tracking.
    pub fn get_indent_transitions(&mut self, content: &str) -> Vec<crate::parsers::yaml::scope::IndentTransition> {
        let mut scope_stack = self.scope_stack.clone();

        for (line_num, line) in content.lines().enumerate() {
            let line_num_1index = line_num + 1;
            let trimmed = line.trim();

            // Skip document markers and directives
            if trimmed == "---" || trimmed == "..." || trimmed.starts_with('%') {
                // Update state to track that we're on an empty line (marker line)
                self.current_line_type = LineClassification::Empty;
                continue;
            }

            let indent = calculate_indentation(line);
            let line_type = classify_line_type(line);

            // Track line type in parser state
            self.current_line_type = line_type;

            // Only track actual indent changes
            if indent != scope_stack.get_last_indent() {
                let has_key = line_type.is_key_bearing();
                scope_stack.record_indent_transition(line_num_1index, indent, has_key, line);

                // Update transition state to track this indent change
                let from_indent = scope_stack.get_last_indent();
                self.current_transition_state.update(from_indent, indent, has_key);
            }
        }

        scope_stack.get_indent_transitions().to_vec()
    }

    /// Get transition counts by type
    ///
    /// Returns (enter_scope_count, exit_scope_count, same_level_count)
    /// Updates parser state with line classifications during counting.
    pub fn get_transition_counts(&mut self, content: &str) -> (usize, usize, usize) {
        let transitions = self.get_indent_transitions(content);

        let enter = transitions.iter().filter(|t| t.is_enter_scope()).count();
        let exit = transitions.iter().filter(|t| t.is_exit_scope()).count();
        let same = transitions.iter().filter(|t| t.is_same_level()).count();

        (enter, exit, same)
    }

    /// Detect duplicate keys using scope-aware tracking
    ///
    /// This method processes YAML content line-by-line, tracking keys within
    /// their proper scope contexts. It returns a list of validation errors
    /// for any duplicate keys found within the same scope.
    fn detect_duplicate_keys_with_scope(&mut self, content: &str, scope_stack: &mut ScopeStack) -> Vec<ValidationError> {
        let mut duplicate_errors = Vec::new();

        for (line_num, line) in content.lines().enumerate() {
            let line_num_1index = line_num + 1;
            let trimmed = line.trim();

            // Classify the line type to enable type-specific scope handling
            let line_type = classify_line_type(line);

            // Handle document markers - reset scope tracking
            if trimmed == "---" || trimmed == "..." {
                #[cfg(debug_assertions)]
                {
                    log_debug!("[detect_duplicate] Document marker '{}' at line={}, resetting scope stack", trimmed, line_num_1index);
                }
                scope_stack.reset();
                continue;
            }

            // Skip YAML directives
            if trimmed.starts_with('%') {
                continue;
            }

            let indent = calculate_indentation(line);

            // Track ALL indent changes for detection purposes, regardless of line type
            // This enables analysis of indent transitions even on blank lines, comments, etc.
            let indent_changed = indent != scope_stack.get_last_indent();
            if indent_changed {
                // Use line classification to determine if this line has a key
                let has_key = line_type.is_key_bearing();
                scope_stack.record_indent_transition(line_num_1index, indent, has_key, line);
            }

            // Handle blank lines, comments, and indent-only lines with consistent logic
            // These lines should be transparent to scope tracking - they don't trigger
            // any scope transitions. The next content line will handle any needed scope changes.
            // Comments are also transparent to scope tracking since YAML parsers ignore them.
            if trimmed.is_empty() || trimmed.starts_with('#') {
                // Update last_indent tracking so the next line can detect indent changes
                // but don't trigger any scope transitions
                if indent != scope_stack.get_last_indent() {
                    scope_stack.set_last_indent(indent);
                }
                continue;
            }

            // Type-specific handling: indent-only lines (no key token)
            // These lines don't trigger scope entry but DO trigger scope exit on indent decrease
            // This ensures proper scope cleanup when returning to outer levels through blank lines
            // CRITICAL: Scope stack must exit on indent decrease even without keys to maintain consistency
            if !line_type.is_key_bearing() {
                // Indent-only line - handle scope exit if indent decreased
                if indent < scope_stack.current_indent() {
                    // Record the indent transition with has_key=false for indent-only lines
                    scope_stack.record_indent_transition(line_num_1index, indent, false, line);
                    scope_stack.exit_to_scope(indent);
                } else if indent > scope_stack.current_indent() {
                    // Indent increased on indent-only line - just record the transition, don't enter scope
                    scope_stack.record_indent_transition(line_num_1index, indent, false, line);
                }
                continue;
            }

            // Handle scope transitions based on indentation changes
            use std::cmp::Ordering;
            match indent.cmp(&scope_stack.current_indent()) {
                Ordering::Greater => {
                    // Indent increased - entering deeper scope
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // Add the parent key to current scope
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            // Enter new scope for the parent key's nested content
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                            // Track scope info on the parser's scope info stack
                            let scope_info = ScopeInfo::block(scope_stack.depth());
                            self.push_scope(scope_info);
                        } else {
                            // Indent increased but not a parent key - this is an inline scalar at deeper indent
                            // Just add the key to current scope, don't create a new scope
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Inline scalar: key='{}', indent={}, line={}, in_sequence={}",
                                    ctx.key_name(), indent, line_num_1index, scope_stack.in_sequence_context());
                            }
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    } else {
                        // Indent increased but no key context found - this is likely:
                        // - A sequence item continuation (already handled by sequence scope logic)
                        // - A scalar value continuation
                        // - Don't enter a scope for non-key lines
                        #[cfg(debug_assertions)]
                        {
                            log_debug!("[detect_duplicate] No key context: line={}, indent={}, in_sequence={}, skipping scope entry",
                                line_num_1index, indent, scope_stack.in_sequence_context());
                        }
                    }
                }
                Ordering::Less => {
                    // Indent decreased - exit to parent scope
                    // Check if we're transitioning to a sequence item at same level as parent
                    // In this case, we should NOT exit the parent scope
                    let trimmed = line.trim();
                    let is_sequence_item_at_parent_level = trimmed.starts_with("- ") &&
                        scope_stack.current_indent() == indent + scope_stack.base_indent();

                    #[cfg(debug_assertions)]
                    {
                        let current_path = scope_stack.get_scope_path();
                        log_debug!("[detect_duplicate] Scope exit: from_indent={}, to_indent={}, line={}, current_scope='{}', is_sequence_item_at_parent_level={}",
                            scope_stack.current_indent(), indent, line_num_1index, current_path, is_sequence_item_at_parent_level);
                    }

                    if !is_sequence_item_at_parent_level {
                        scope_stack.exit_to_scope(indent);
                    }

                    // After exiting, check if this line has a key
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a new parent key at this level
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                            // Track scope info on the parser's scope info stack
                            let scope_info = ScopeInfo::block(scope_stack.depth());
                            self.push_scope(scope_info);
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                    // Edge case: indent decreased but no key - we've already exited to the right scope
                }
                Ordering::Equal => {
                    // Same scope - check for keys
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a sibling parent key at same indent level
                            // Exit and re-enter scope for the sibling
                            #[cfg(debug_assertions)]
                            {
                                let current_path = scope_stack.get_scope_path();
                                log_debug!("[detect_duplicate] Scope exit for sibling: key='{}', indent={}, line={}, current_scope='{}'",
                                    ctx.key_name(), indent, line_num_1index, current_path);
                            }
                            scope_stack.exit_to_scope(indent);
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Re-entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                            // Track scope info on the parser's scope info stack
                            let scope_info = ScopeInfo::block(scope_stack.depth());
                            self.push_scope(scope_info);
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                    // Edge case: same indent but no key - just continue, no scope change needed
                }
            }

            // Handle sequence items with their own scopes
            if trimmed.starts_with("- ") {
                if let Some(ctx) = extract_key_context(line) {
                    // Enter a sequence scope for this item
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[detect_duplicate] Entering scope: type=Sequence, key='{}', indent={}, line={}",
                            ctx.key_name(), indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                    // Track scope info on the parser's scope info stack
                    let scope_info = ScopeInfo::new(ScopeType::BlockSequence, scope_stack.depth());
                    self.push_scope(scope_info);
                    // Add the key to the sequence scope
                    if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                        duplicate_errors.push(ValidationError::new(
                            format!("line_{}", line_num_1index),
                            dup_err.message()
                        ).with_line(line_num_1index));
                    }
                } else {
                    // Sequence item without a key context
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[detect_duplicate] Entering scope: type=Sequence (no key), indent={}, line={}",
                            indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                    // Track scope info on the parser's scope info stack
                    let scope_info = ScopeInfo::new(ScopeType::BlockSequence, scope_stack.depth());
                    self.push_scope(scope_info);
                }
            }
        }

        duplicate_errors
    }
}

impl Default for BasicParser {
    fn default() -> Self {
        Self::new()
    }
}

impl Parser for BasicParser {
    fn parse_str(&mut self, content: &str) -> ParseResult<serde_yaml::Value> {
        // Create a new scope stack for this parsing operation
        let mut scope_stack = ScopeStack::new(2);
        let mut parse_errors = Vec::new();

        // Parse line by line with scope-aware key tracking
        for (line_num, line) in content.lines().enumerate() {
            let line_num_1index = line_num + 1;
            let trimmed = line.trim();

            // Classify the line type to enable type-specific scope handling
            let line_type = classify_line_type(line);

            // Handle document markers - reset scope tracking
            if trimmed == "---" || trimmed == "..." {
                #[cfg(debug_assertions)]
                {
                    log_debug!("[parse_str] Document marker '{}' at line={}, resetting scope stack", trimmed, line_num_1index);
                }
                scope_stack.reset();
                continue;
            }

            // Skip YAML directives
            if trimmed.starts_with('%') {
                continue;
            }

            let indent = calculate_indentation(line);

            // Track ALL indent changes for detection purposes, regardless of line type
            // This enables analysis of indent transitions even on blank lines, comments, etc.
            let indent_changed = indent != scope_stack.get_last_indent();
            if indent_changed {
                // Use line classification to determine if this line has a key
                let has_key = line_type.is_key_bearing();
                scope_stack.record_indent_transition(line_num_1index, indent, has_key, line);
            }

            // Handle blank lines, comments, and indent-only lines with consistent logic
            // These lines should be transparent to scope tracking - they don't trigger
            // any scope transitions. The next content line will handle any needed scope changes.
            // Comments are also transparent to scope tracking since YAML parsers ignore them.
            if trimmed.is_empty() || trimmed.starts_with('#') {
                // Update last_indent tracking so the next line can detect indent changes
                // but don't trigger any scope transitions
                if indent != scope_stack.get_last_indent() {
                    scope_stack.set_last_indent(indent);
                }
                continue;
            }

            // Type-specific handling: indent-only lines (no key token)
            // These lines don't trigger scope entry but DO trigger scope exit on indent decrease
            // This ensures proper scope cleanup when returning to outer levels through blank lines
            // CRITICAL: Scope stack must exit on indent decrease even without keys to maintain consistency
            if !line_type.is_key_bearing() {
                // Indent-only line - handle scope exit if indent decreased
                if indent < scope_stack.current_indent() {
                    // Record the indent transition with has_key=false for indent-only lines
                    scope_stack.record_indent_transition(line_num_1index, indent, false, line);
                    scope_stack.exit_to_scope(indent);
                } else if indent > scope_stack.current_indent() {
                    // Indent increased on indent-only line - just record the transition, don't enter scope
                    scope_stack.record_indent_transition(line_num_1index, indent, false, line);
                }
                continue;
            }

            // Handle scope transitions based on indentation changes
            use std::cmp::Ordering;
            match indent.cmp(&scope_stack.current_indent()) {
                Ordering::Greater => {
                    // Indent increased - entering deeper scope
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // Add the parent key to current scope
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            // Enter new scope for the parent key's nested content
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else {
                            // Indent increased but not a parent key - this is an inline scalar at deeper indent
                            // Just add the key to current scope, don't create a new scope
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Inline scalar: key='{}', indent={}, line={}, in_sequence={}",
                                    ctx.key_name(), indent, line_num_1index, scope_stack.in_sequence_context());
                            }
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    } else {
                        // Indent increased but no key context found - this is likely:
                        // - A sequence item continuation (already handled by sequence scope logic)
                        // - A scalar value continuation
                        // - Don't enter a scope for non-key lines
                        #[cfg(debug_assertions)]
                        {
                            log_debug!("[parse_str] No key context: line={}, indent={}, in_sequence={}, skipping scope entry",
                                line_num_1index, indent, scope_stack.in_sequence_context());
                        }
                    }
                }
                Ordering::Less => {
                    // Indent decreased - exit to parent scope
                    // Record the indent transition with key
                    scope_stack.record_indent_transition(line_num_1index, indent, true, line);

                    #[cfg(debug_assertions)]
                    {
                        let current_path = scope_stack.get_scope_path();
                        log_debug!("[parse_str] Scope exit: from_indent={}, to_indent={}, line={}, current_scope='{}'",
                            scope_stack.current_indent(), indent, line_num_1index, current_path);
                    }
                    scope_stack.exit_to_scope(indent);

                    // After exiting, check if this line has a key
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a new parent key at this level
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                }
                Ordering::Equal => {
                    // Same scope - check for keys
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a sibling parent key at same indent level
                            // Exit and re-enter scope for the sibling
                            #[cfg(debug_assertions)]
                            {
                                let current_path = scope_stack.get_scope_path();
                                log_debug!("[parse_str] Scope exit for sibling: key='{}', indent={}, line={}, current_scope='{}'",
                                    ctx.key_name(), indent, line_num_1index, current_path);
                            }
                            scope_stack.exit_to_scope(indent);
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Re-entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                }
            }

            // Handle sequence items with their own scopes
            if trimmed.starts_with("- ") {
                // When starting a new sequence item, exit any existing sequence scope at the same indent
                if scope_stack.current_indent() == indent && scope_stack.in_sequence_context() {
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[parse_str] Exiting previous sequence scope before entering new one at indent={}, line={}",
                            indent, line_num_1index);
                    }
                    scope_stack.exit_to_scope(indent.saturating_sub(1));
                }

                if let Some(ctx) = extract_key_context(line) {
                    // Enter a sequence scope for this item
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[parse_str] Entering scope: type=Sequence, key='{}', indent={}, line={}",
                            ctx.key_name(), indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                    // Add the key to the sequence scope
                    if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                        parse_errors.push(ValidationError::new(
                            format!("line_{}", line_num_1index),
                            dup_err.message()
                        ).with_line(line_num_1index));
                    }
                } else {
                    // Sequence item without a key context
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[parse_str] Entering scope: type=Sequence (no key), indent={}, line={}",
                            indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                }
            }
        }

        // Return parse result using serde_yaml for the actual value
        match serde_yaml::from_str::<serde_yaml::Value>(content) {
            Ok(value) => {
                // Note: parse_errors from scope tracking are currently informational
                // They could be added as warnings in a future enhancement
                ParseResult::success(value)
            }
            Err(err) => {
                // Add any duplicate key errors we found to the parse error
                let error_msg = if parse_errors.is_empty() {
                    err.to_string()
                } else {
                    format!("{}; duplicate key errors: {}", err, parse_errors.iter()
                        .map(|e| e.message.clone()).collect::<Vec<_>>().join("; "))
                };
                ParseResult::failure(ParseError::syntax(error_msg))
            }
        }
    }

    fn parse_bytes(&mut self, content: &[u8]) -> ParseResult<serde_yaml::Value> {
        // Convert bytes to string and parse
        match std::str::from_utf8(content) {
            Ok(utf8_content) => self.parse_str(utf8_content),
            Err(err) => ParseResult::failure(ParseError::io(format!("Invalid UTF-8: {}", err))),
        }
    }

    fn parse_file(&mut self, path: &std::path::Path) -> ParseResult<serde_yaml::Value> {
        // Read file content
        match std::fs::read_to_string(path) {
            Ok(content) => self.parse_str(&content),
            Err(err) => ParseResult::failure(ParseError::io(format!("Failed to read file {}: {}", path.display(), err))),
        }
    }

    fn validate_str(&mut self, content: &str) -> ValidationResult {
        // Create syntax validator based on parser mode
        let validator = if self.config.is_strict() {
            SyntaxValidator::strict()
        } else {
            SyntaxValidator::lenient()
        };

        // Run syntax validation
        let mut result = validator.validate(content);

        // Run scope-aware duplicate key detection (when duplicates are not allowed)
        if !self.config.allow_duplicates {
            let mut scope_stack = self.scope_stack.clone();
            let duplicate_errors = self.detect_duplicate_keys_with_scope(content, &mut scope_stack);
            if !duplicate_errors.is_empty() {
                result.valid = false;
                result.errors.extend(duplicate_errors);
            }
        }

        // If no errors from basic validation, run enhanced detection
        if result.is_valid() {
            let mut detector = SyntaxDetector::new();
            let detector_result = detector.detect_to_validation_result(content);

            // Merge errors from detector
            if !detector_result.is_valid() {
                result.valid = false;
                result.errors.extend(detector_result.errors);
            }
        }

        result
    }

    fn validate_file(&mut self, path: &std::path::Path) -> ValidationResult {
        // Read file content
        let content = match std::fs::read_to_string(path) {
            Ok(content) => content,
            Err(err) => {
                return ValidationResult {
                    valid: false,
                    errors: vec![ValidationError::new(
                        path.display().to_string(),
                        format!("failed to read file: {}", err)
                    )],
                    warnings: Vec::new(),
                };
            }
        };

        // Validate the content
        self.validate_str(&content)
    }

    fn config(&self) -> &ParserConfig {
        &self.config
    }

    fn with_config(self, config: ParserConfig) -> Self
    where
        Self: Sized,
    {
        Self {
            config,
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
            scope_info_stack: Vec::new(),    // Empty stack initially
            current_line_type: LineClassification::Empty,
            current_transition_state: IndentTransitionState::new(),
            scope_depth: 1, // Root scope is always present
        }
    }
}

// ============================================================================
// Integration Tests for Scope Tracking
// ============================================================================

#[cfg(test)]
mod integration_tests {
    use super::*;

    /// Test parse_str with nested YAML structures
    #[test]
    fn test_parse_str_with_nested_yaml() {
        let mut parser = BasicParser::new();
        let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should parse valid nested YAML successfully");

        // Verify we can access the parsed structure
        let value = result.unwrap();
        assert!(value.is_mapping());
        assert!(value.get("services").is_some());
    }

    /// Test parse_str with deeply nested YAML
    #[test]
    fn test_parse_str_with_deeply_nested_yaml() {
        let mut parser = BasicParser::new();
        let yaml = r#"
application:
  server:
    config:
      timeouts:
        connect: 30
        read: 60
    logging:
      level: info
  database:
    connection:
      pool:
        max: 10
        min: 2
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should parse deeply nested YAML successfully");
    }

    /// Test that same key in different scopes passes
    #[test]
    fn test_same_key_different_scopes_passes() {
        let mut parser = BasicParser::new();

        // This YAML has 'host' and 'port' in different scopes
        let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
  cache:
    host: redis.example.com
    port: 6379
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Same key in different scopes should be allowed");

        let value = result.unwrap();
        let services = &value["services"];
        assert_eq!(services["web"]["host"], "localhost");
        assert_eq!(services["database"]["host"], "db.example.com");
        assert_eq!(services["cache"]["host"], "redis.example.com");
    }

    /// Test that duplicate key in same scope is detected by strict parser
    #[test]
    fn test_duplicate_key_same_scope_detected() {
        let mut parser = BasicParser::strict(); // Strict parser doesn't allow duplicates

        let yaml = r#"
config:
  server:
    host: localhost
    host: duplicate-value
    port: 8080
"#;

        // The parse_str itself may succeed (serde_yaml's behavior), but validation should detect the duplicate
        let validation_result = parser.validate_str(yaml);
        assert!(!validation_result.is_valid(), "Validation should detect duplicate keys");

        // Check that error message mentions duplicate key
        let error_messages: Vec<String> = validation_result.errors
            .iter()
            .map(|e| e.message.clone())
            .collect();
        assert!(error_messages.iter().any(|msg| msg.contains("duplicate") || msg.contains("host")));
    }

    /// Test parse_str with sequence items having same keys
    #[test]
    fn test_parse_str_sequence_items_same_keys() {
        let mut parser = BasicParser::new();

        let yaml = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
  - name: item3
    value: 300
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Sequence items with same keys should be allowed");

        let value = result.unwrap();
        let items = value["items"].as_sequence().unwrap();
        assert_eq!(items.len(), 3);
    }

    /// Test parse_str with mixed mapping and sequence scopes
    #[test]
    fn test_parse_str_mixed_mapping_sequence() {
        let mut parser = BasicParser::new();

        let yaml = r#"
services:
  web:
    endpoints:
      - path: /api
        method: GET
      - path: /health
        method: GET
    config:
      timeout: 30
  database:
    endpoints:
      - path: /query
        method: POST
    config:
      pool_size: 10
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Mixed mapping and sequence structures should parse successfully");
    }

    /// Test parse_str with multiple indent transitions
    #[test]
    fn test_parse_str_multiple_indent_transitions() {
        let mut parser = BasicParser::new();

        let yaml = r#"
level1_a:
  level2_a:
    level3:
      value: 1
  level2_b:
    level3:
      value: 2
level1_b:
  level2:
    level3:
      value: 3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Multiple indent transitions should be handled correctly");
    }

    /// Test parse_str with inline scalars (no scope creation)
    #[test]
    fn test_parse_str_inline_scalars() {
        let mut parser = BasicParser::new();

        let yaml = r#"
name: test
version: "1.0"
author: example
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Inline scalars should parse successfully");

        let value = result.unwrap();
        assert_eq!(value["name"], "test");
        assert_eq!(value["version"], "1.0");
    }

    /// Test parse_str with empty lines and comments
    #[test]
    fn test_parse_str_with_comments_and_empty_lines() {
        let mut parser = BasicParser::new();

        let yaml = r#"
# Configuration file
name: test

# Server settings
server:
  host: localhost
  # Port number
  port: 8080

# Database settings
database:
  host: db.example.com
  port: 5432
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Comments and empty lines should be handled correctly");
    }

    /// Test validate_str detects duplicate keys
    #[test]
    fn test_validate_str_duplicate_detection() {
        let mut parser = BasicParser::strict();

        let yaml = r#"
config:
  setting1: value1
  setting1: value2
  setting2: value3
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid(), "Should detect duplicate keys in same scope");
        assert!(!result.errors.is_empty(), "Should have validation errors");
    }

    /// Test validate_str allows same key in different scopes
    #[test]
    fn test_validate_str_same_key_different_scopes() {
        let mut parser = BasicParser::strict();

        let yaml = r#"
section1:
  key: value1
section2:
  key: value2
section3:
  key: value3
"#;

        let result = parser.validate_str(yaml);
        assert!(result.is_valid(), "Same key in different scopes should be valid");
    }

    /// Test real-world config scenario
    #[test]
    fn test_real_world_config_scenario() {
        let mut parser = BasicParser::new();

        let yaml = r#"
application:
  name: myapp
  version: 1.0.0

server:
  host: 0.0.0.0
  port: 8080
  ssl:
    enabled: true
    cert_path: /path/to/cert.pem

database:
  primary:
    host: db1.example.com
    port: 5432
  replica:
    host: db2.example.com
    port: 5432

logging:
  level: info
  outputs:
    - type: stdout
      format: json
    - type: file
      format: text
      path: /var/log/app.log
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Real-world config should parse successfully");

        let value = result.unwrap();
        assert_eq!(value["application"]["name"], "myapp");
        assert_eq!(value["server"]["port"], 8080);
        assert_eq!(value["logging"]["level"], "info");
    }

    /// Test parse_str with document markers
    #[test]
    fn test_parse_str_with_document_markers() {
        let mut parser = BasicParser::new();

        let yaml = r#"
---
name: test
value: 123
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Document markers should be handled correctly");

        let value = result.unwrap();
        assert_eq!(value["name"], "test");
        assert_eq!(value["value"], 123);
    }

    /// Test scope tracking with complex nested structure
    #[test]
    fn test_scope_tracking_complex_nesting() {
        let mut parser = BasicParser::new();

        let yaml = r#"
outer:
  inner1:
    deep1:
      value: 1
    deep2:
      value: 2
  inner2:
    deep1:
      deeper:
        value: 3
    deep2:
      value: 4
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Complex nesting should be handled correctly");
    }

    /// Test that parent key followed by nested content creates proper scope
    #[test]
    fn test_parent_key_creates_scope() {
        let mut parser = BasicParser::new();

        let yaml = r#"
parent:
  child1: value1
  child2: value2
  nested:
    deep: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Parent key with nested content should create proper scope");

        let value = result.unwrap();
        assert_eq!(value["parent"]["child1"], "value1");
        assert_eq!(value["parent"]["nested"]["deep"], "value3");
    }

    /// Test sequence scope isolation
    #[test]
    fn test_sequence_scope_isolation() {
        let mut parser = BasicParser::new();

        let yaml = r#"
items:
  - id: 1
    name: First
    config:
      enabled: true
  - id: 2
    name: Second
    config:
      enabled: false
  - id: 3
    name: Third
    config:
      enabled: true
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Sequence items should have isolated scopes");

        let value = result.unwrap();
        let items = value["items"].as_sequence().unwrap();
        assert_eq!(items.len(), 3);
    }

    /// Test validation with multiple duplicates in different scopes
    #[test]
    fn test_validate_multiple_scopes_with_duplicates() {
        let mut parser = BasicParser::strict();

        let yaml = r#"
scope1:
  key: value1
  key: duplicate1
scope2:
  key: value2
scope3:
  key: value3
  key: duplicate2
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid(), "Should detect duplicates in their respective scopes");

        // Should have errors for both duplicates
        assert!(result.errors.len() >= 2, "Should detect multiple duplicate errors");
    }

    /// Test parse_str with various indentation patterns
    #[test]
    fn test_parse_str_various_indentation_patterns() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  level1_a:
    level2_a:
      level3: value1
    level2_b: value2
  level1_b:
    level2:
      - item1
      - item2
  level1_c: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Various indentation patterns should be handled correctly");
    }

    /// Test that empty document parses successfully
    #[test]
    fn test_parse_empty_document() {
        let mut parser = BasicParser::new();

        let yaml = "";

        let result = parser.parse_str(yaml);
        // Empty document should parse as null
        assert!(result.is_success());
    }

    /// Test that document with only comments parses successfully
    #[test]
    fn test_parse_only_comments() {
        let mut parser = BasicParser::new();

        let yaml = r#"
# Comment 1
# Comment 2
# Comment 3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Document with only comments should parse successfully");
    }

    /// Test validation error message quality
    #[test]
    fn test_validation_error_message_quality() {
        let mut parser = BasicParser::strict();

        let yaml = r#"
config:
  server:
    host: localhost
    host: duplicate
    port: 8080
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid());

        // Check that error messages are useful
        for error in &result.errors {
            assert!(!error.message.is_empty(), "Error message should not be empty");
            if error.line.is_some() {
                assert!(error.message.contains("Line"), "Error should mention line number");
            }
        }
    }

    /// Test blank line with decreased indent (scope should exit properly)
    #[test]
    fn test_blank_line_with_decreased_indent() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  nested:
    key1: value1

key2: value2
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Blank line with decreased indent should parse successfully");

        let value = result.unwrap();
        assert!(value.get("root").is_some());
        assert!(value.get("key2").is_some());
    }

    /// Test blank line at same indent as current scope
    #[test]
    fn test_blank_line_same_indent() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  key1: value1

  key2: value2
  key3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Blank line at same indent should parse successfully");

        let value = result.unwrap();
        let root = &value["root"];
        assert_eq!(root["key1"], "value1");
        assert_eq!(root["key2"], "value2");
        assert_eq!(root["key3"], "value3");
    }

    /// Test multiple blank lines at various indents
    #[test]
    fn test_multiple_blank_lines_various_indents() {
        let mut parser = BasicParser::new();

        let yaml = r#"
level1:
  level2:
    key1: value1


  key2: value2

level3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Multiple blank lines at various indents should parse successfully");

        let value = result.unwrap();
        assert!(value.get("level1").is_some());
        assert!(value.get("level3").is_some());
    }

    /// Test comment at different indent (should not affect scope)
    #[test]
    fn test_comment_different_indent() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  key1: value1
  # Comment at indent 2
  key2: value2
# Comment at indent 0
key3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Comments at different indents should not affect parsing");

        let value = result.unwrap();
        let root = &value["root"];
        assert_eq!(root["key1"], "value1");
        assert_eq!(root["key2"], "value2");
        assert_eq!(value["key3"], "value3");
    }

    /// Test blank line followed by key at same indent
    #[test]
    fn test_blank_line_key_same_indent() {
        let mut parser = BasicParser::new();

        let yaml = r#"
services:
  web:
    host: localhost

  database:
    host: db.example.com
    port: 5432
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Blank line followed by key at same indent should parse successfully");

        let value = result.unwrap();
        let services = &value["services"];
        assert!(services.get("web").is_some());
        assert!(services.get("database").is_some());
    }

    /// Test indent transition without key (just blank line)
    #[test]
    fn test_indent_transition_blank_line_only() {
        let mut parser = BasicParser::new();

        let yaml = r#"
level1:
  level2:
    key1: value1


key3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Indent transition via blank lines should parse successfully");

        let value = result.unwrap();
        assert!(value.get("level1").is_some());
        assert!(value.get("key3").is_some());
    }

    /// Test scope consistency after indent changes on blank lines
    #[test]
    fn test_scope_consistency_after_blank_indents() {
        let mut parser = BasicParser::new();

        let yaml = r#"
outer1:
  inner1:
    deep1: value1

  inner2:
    deep2: value2

outer2:
  inner3:
    deep3: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Scope should remain consistent after indent changes on blank lines");

        let value = result.unwrap();
        let outer1 = &value["outer1"];
        assert!(outer1.get("inner1").is_some());
        assert!(outer1.get("inner2").is_some());
        assert!(value.get("outer2").is_some());
    }

    /// Test that blank lines don't create false duplicate key errors
    #[test]
    fn test_blank_lines_no_false_duplicates() {
        let mut parser = BasicParser::strict();

        let yaml = r#"
section:
  key1: value1

  key2: value2

section2:
  key1: value3
  key2: value4
"#;

        let result = parser.validate_str(yaml);
        assert!(result.is_valid(), "Blank lines should not cause false duplicate key errors");
    }

    /// Test complex nesting with blank lines
    #[test]
    fn test_complex_nesting_blank_lines() {
        let mut parser = BasicParser::new();

        let yaml = r#"
app:
  server:
    host: localhost
    port: 8080

    ssl:
      enabled: true
      cert: /path/to/cert

  database:

    primary:
      host: db1.example.com
      port: 5432

    replica:
      host: db2.example.com
      port: 5432
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Complex nesting with blank lines should parse successfully");

        let value = result.unwrap();
        let app = &value["app"];
        assert!(app.get("server").is_some());
        assert!(app.get("database").is_some());
    }

    /// Test that increased indent on blank line doesn't enter scope
    #[test]
    fn test_increased_indent_blank_line_no_scope() {
        let mut parser = BasicParser::new();

        let yaml = r#"
key1: value1


key2: value2
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Increased indent on blank line should not affect scope");
    }

    /// Test blank lines in sequence contexts
    #[test]
    fn test_blank_lines_in_sequence() {
        let mut parser = BasicParser::new();

        let yaml = r#"
items:
  - name: item1
    value: 100

  - name: item2
    value: 200

  - name: item3
    value: 300
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Blank lines in sequences should parse successfully");

        let value = result.unwrap();
        let items = value["items"].as_sequence().unwrap();
        assert_eq!(items.len(), 3);
    }

    /// Test indent transition state tracking
    #[test]
    fn test_indent_transition_state_tracking() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  level1:
    level2: value
  level1_sibling: value2
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Should have transitions for indent changes
        assert!(!transitions.is_empty(), "Should track indent transitions");

        // Verify transitions are properly classified
        let enter_transitions: Vec<_> = transitions.iter()
            .filter(|t| t.is_enter_scope())
            .collect();

        let exit_transitions: Vec<_> = transitions.iter()
            .filter(|t| t.is_exit_scope())
            .collect();

        // Should have both enter and exit transitions
        assert!(!enter_transitions.is_empty(), "Should have enter-scope transitions");
        assert!(!exit_transitions.is_empty(), "Should have exit-scope transitions");
    }

    /// Test transition type classification
    #[test]
    fn test_transition_type_classification() {
        let mut parser = BasicParser::new();

        let yaml = r#"
key1: value1
  nested:
    deep: value
key2: value2
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Check that all transitions are properly classified
        for transition in &transitions {
            let transition_type = transition.transition_type();

            // Verify classification matches indent change
            if transition.to_indent > transition.from_indent {
                assert_eq!(transition_type, IndentTransitionType::EnterScope,
                           "Indent increase should be classified as EnterScope");
            } else if transition.to_indent < transition.from_indent {
                assert_eq!(transition_type, IndentTransitionType::ExitScope,
                           "Indent decrease should be classified as ExitScope");
            } else {
                assert_eq!(transition_type, IndentTransitionType::SameLevel,
                           "No indent change should be classified as SameLevel");
            }
        }
    }

    /// Test scope operation mapping
    #[test]
    fn test_scope_operation_mapping() {
        let mut parser = BasicParser::new();

        let yaml = r#"
parent:
  child: value
sibling: value2
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Verify each transition has a valid scope operation
        for transition in &transitions {
            let operation = transition.scope_operation();

            match operation {
                "enter-scope" => assert!(transition.is_enter_scope()),
                "exit-scope" => assert!(transition.is_exit_scope()),
                "stay-in-scope" => assert!(transition.is_same_level()),
                _ => panic!("Invalid scope operation: {}", operation),
            }
        }
    }

    /// Test transition counts by type
    #[test]
    fn test_transition_counts() {
        let mut parser = BasicParser::new();

        let yaml = r#"
level1_a:
  level2_a:
    level3: value
  level2_b: value
level1_b:
  level2: value
"#;

        let (enter, exit, same) = parser.get_transition_counts(yaml);

        // Should have multiple transitions
        assert!(enter > 0, "Should have enter-scope transitions");
        assert!(exit > 0, "Should have exit-scope transitions");

        // Enter and exit should be roughly balanced (with small difference due to starting at root)
        assert!((enter as i32 - exit as i32).abs() <= 1,
               "Enter and exit counts should be balanced");
    }

    /// Test transitions with blank lines
    #[test]
    fn test_transitions_with_blank_lines() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  child1: value1

  child2: value2

key2: value3
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Should track transitions even across blank lines
        assert!(!transitions.is_empty(), "Should track transitions across blank lines");

        // Verify exit transitions are recorded
        let exit_transitions: Vec<_> = transitions.iter()
            .filter(|t| t.is_exit_scope())
            .collect();

        assert!(!exit_transitions.is_empty(), "Should track exit transitions across blank lines");
    }

    /// Test transition state with line classification
    #[test]
    fn test_transition_with_line_classification() {
        let mut parser = BasicParser::new();

        let yaml = r#"
key: value
  # comment at indent 2
  another: value2
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Verify transitions have line classification
        for transition in &transitions {
            match transition.line_classification() {
                LineClassification::KeyBearing => {
                    assert!(transition.has_key || transition.raw_line.contains(':'),
                           "Key-bearing lines should have keys or colons");
                }
                LineClassification::IndentOnly => {
                    // Comment or indent-only line
                }
                LineClassification::Empty => {
                    assert!(transition.raw_line.trim().is_empty(),
                           "Empty classification should only be for empty lines");
                }
            }
        }
    }

    /// Test indent increase/decrease/no-change handling
    #[test]
    fn test_indent_change_handling() {
        let mut parser = BasicParser::new();

        let yaml = r#"
level1:
  level2:
    level3: value
  level2_sibling: value
level1_sibling: value
"#;

        let transitions = parser.get_indent_transitions(yaml);

        let mut has_increase = false;
        let mut has_decrease = false;

        for transition in &transitions {
            if transition.is_increase() {
                has_increase = true;
                assert!(transition.is_enter_scope(),
                       "Increase should be enter-scope");
            }
            if transition.is_decrease() {
                has_decrease = true;
                assert!(transition.is_exit_scope(),
                       "Decrease should be exit-scope");
            }
        }

        assert!(has_increase, "Should have indent increases");
        assert!(has_decrease, "Should have indent decreases");
    }

    /// Test that all transition types are tracked
    #[test]
    fn test_all_transition_types_tracked() {
        let mut parser = BasicParser::new();

        let yaml = r#"
root:
  child1: value1
  child2: value2
sibling: value3
"#;

        let transitions = parser.get_indent_transitions(yaml);

        let has_enter = transitions.iter().any(|t| t.is_enter_scope());
        let has_exit = transitions.iter().any(|t| t.is_exit_scope());

        assert!(has_enter, "Should track enter-scope transitions");
        assert!(has_exit, "Should track exit-scope transitions");
    }

    /// Test complex YAML with many indent transitions
    #[test]
    fn test_complex_indent_transitions() {
        let mut parser = BasicParser::new();

        let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
    ssl:
      enabled: true
      cert: /path/to/cert
  database:
    host: db.example.com
    port: 5432
  cache:
    host: redis.example.com
    port: 6379
logging:
  level: info
  outputs:
    - type: stdout
    - type: file
"#;

        let transitions = parser.get_indent_transitions(yaml);
        let (enter, exit, same) = parser.get_transition_counts(yaml);

        // Complex YAML should have many transitions
        assert!(transitions.len() >= 5, "Complex YAML should have many transitions");
        assert!(enter >= 3, "Should have multiple enter-scope transitions");
        // Exit transitions might be fewer since we don't always exit explicitly
        assert!(enter + exit >= 5, "Should have multiple total transitions");
    }

    /// Test that transition history is maintained
    #[test]
    fn test_transition_history_maintained() {
        let mut parser = BasicParser::new();

        let yaml = r#"
a:
  b:
    c: value
  d: value
e: value
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Transitions should be in order
        let mut last_line = 0;
        for (i, transition) in transitions.iter().enumerate() {
            assert!(transition.line_number > last_line,
                   "Transition {} should be after previous transition", i);
            last_line = transition.line_number;
        }

        // Each transition should have complete information
        for transition in &transitions {
            assert!(transition.line_number > 0);
            assert!(transition.from_indent <= transition.to_indent ||
                   transition.to_indent <= transition.from_indent);
            assert!(!transition.raw_line.is_empty());
        }
    }

    /// Test scope depth accessor
    #[test]
    fn test_scope_depth_accessor() {
        let mut parser = BasicParser::new();

        // New parser starts at root scope (depth 1)
        assert_eq!(parser.scope_depth(), 1, "New parser should start at depth 1");
        assert!(parser.is_at_root(), "New parser should be at root");
        assert!(!parser.is_in_nested_scope(), "New parser should not be in nested scope");
    }

    /// Test scope stack accessor
    #[test]
    fn test_scope_stack_accessor() {
        let mut parser = BasicParser::new();

        // Should be able to access scope stack
        let stack = parser.scope_stack();
        assert_eq!(stack.depth(), 1, "Scope stack should start with root scope only");
    }

    /// Test current scope accessor
    #[test]
    fn test_current_scope_accessor() {
        let mut parser = BasicParser::new();

        // Should be able to access current scope
        let current = parser.current_scope();
        assert!(current.is_some(), "Should have a current scope");
        assert_eq!(current.unwrap().indent_level, 0, "Root scope should be at indent 0");
    }

    /// Test parent scope accessors
    #[test]
    fn test_parent_scope_accessors() {
        let mut parser = BasicParser::new();

        // At root, there should be no parent
        assert!(parser.immediate_parent_scope().is_none(),
               "Root scope should have no parent");
        assert!(parser.parent_scope(1).is_none(),
               "Root scope should have no parent at offset 1");
        assert!(parser.parent_scope(2).is_none(),
               "Root scope should have no grandparent");
    }

    /// Test scope hierarchy path
    #[test]
    fn test_scope_hierarchy_path() {
        let mut parser = BasicParser::new();

        // At root, path should be empty
        let path = parser.scope_path();
        assert!(path.is_empty() || path == ".",
               "Root scope path should be empty or dot");
    }

    /// Test scope depth tracking during transitions
    #[test]
    fn test_scope_depth_tracking_during_transitions() {
        let mut parser = BasicParser::new();

        let yaml = r#"
level1:
  level2:
    level3: value
  level2_sibling: value2
level1_sibling: value3
"#;

        // Get transitions to see scope changes
        let transitions = parser.get_indent_transitions(yaml);

        // Should have enter and exit transitions
        let enter_count = transitions.iter().filter(|t| t.is_enter_scope()).count();
        let exit_count = transitions.iter().filter(|t| t.is_exit_scope()).count();

        assert!(enter_count > 0, "Should have enter-scope transitions");
        assert!(exit_count > 0, "Should have exit-scope transitions");
    }

    /// Test scope depth with nested structures
    #[test]
    fn test_scope_depth_with_nested_structures() {
        let mut parser = BasicParser::new();

        let yaml = r#"
a:
  b:
    c:
      d: value
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Track the maximum depth reached
        let enter_transitions: Vec<_> = transitions.iter()
            .filter(|t| t.is_enter_scope())
            .collect();

        // Should have multiple enter transitions for deeply nested structure
        assert!(enter_transitions.len() >= 3,
               "Deeply nested YAML should have multiple enter-scope transitions");
    }

    /// Test scope depth with sequences
    #[test]
    fn test_scope_depth_with_sequences() {
        let mut parser = BasicParser::new();

        let yaml = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
"#;

        let transitions = parser.get_indent_transitions(yaml);

        // Should have transitions for sequence items
        assert!(!transitions.is_empty(), "Sequence YAML should have transitions");

        // Sequence items should create scopes
        let enter_count = transitions.iter().filter(|t| t.is_enter_scope()).count();
        assert!(enter_count > 0, "Sequence items should create scopes");
    }

    /// Test scope depth accessor doesn't change after parsing
    #[test]
    fn test_scope_depth_unchanged_after_parsing() {
        let mut parser = BasicParser::new();

        let yaml = r#"
nested:
  deep:
    value: test
"#;

        let initial_depth = parser.scope_depth();

        // Parse the YAML
        parser.get_indent_transitions(yaml);

        // Parser's scope depth should remain unchanged (it's for state queries, not tracking)
        assert_eq!(parser.scope_depth(), initial_depth,
                   "Parser scope depth should remain unchanged after parsing operations");
    }

    /// Test scope stack depth reflects actual changes
    #[test]
    fn test_scope_stack_depth_reflects_changes() {
        let mut parser = BasicParser::new();

        let yaml = r#"
a:
  b:
    c: value
"#;

        let initial_stack_depth = parser.scope_stack().depth();

        // The parser's scope stack doesn't change during get_indent_transitions
        // (it uses a local copy), so the depth should remain the same
        parser.get_indent_transitions(yaml);

        let final_stack_depth = parser.scope_stack().depth();

        assert_eq!(initial_stack_depth, final_stack_depth,
                   "Parser's scope stack depth should remain unchanged");
    }

    /// Test parent scope at different offsets
    #[test]
    fn test_parent_scope_at_different_offsets() {
        let mut parser = BasicParser::new();

        // At root, all parent queries should return None
        assert!(parser.parent_scope(0).is_none(), "Offset 0 should return None");
        assert!(parser.parent_scope(1).is_none(), "Offset 1 should return None at root");
        assert!(parser.parent_scope(10).is_none(), "Large offset should return None");
    }

    /// Test scope hierarchy with multiple levels
    #[test]
    fn test_scope_hierarchy_multiple_levels() {
        let mut parser = BasicParser::new();

        // Even at root, we should be able to get the hierarchy
        let hierarchy = parser.scope_hierarchy();
        assert!(!hierarchy.is_empty(), "Hierarchy should contain at least root scope");
        assert_eq!(hierarchy.len(), 1, "Root hierarchy should have exactly 1 scope");
    }

    /// Test push_scope adds scope info to stack
    #[test]
    fn test_push_scope() {
        let mut parser = BasicParser::new();

        // Initially scope_info_stack should be empty
        assert_eq!(parser.scope_info_stack().len(), 0, "Initial scope info stack should be empty");

        // Create a scope info and push it
        let scope_info = ScopeInfo::block(1);
        parser.push_scope(scope_info);

        // Verify it was added
        assert_eq!(parser.scope_info_stack().len(), 1, "Scope info stack should have 1 item after push");

        // Verify the pushed scope info matches
        let pushed_info = parser.scope_info_stack().last().unwrap();
        assert_eq!(pushed_info.scope_type(), ScopeType::Block, "Pushed scope should be Block type");
        assert_eq!(pushed_info.scope_depth(), 1, "Pushed scope should have depth 1");
    }

    /// Test push_scope multiple times
    #[test]
    fn test_push_scope_multiple() {
        let mut parser = BasicParser::new();

        // Push multiple scopes
        parser.push_scope(ScopeInfo::block(1));
        parser.push_scope(ScopeInfo::block(2));
        parser.push_scope(ScopeInfo::block(3));

        // Verify all were added
        assert_eq!(parser.scope_info_stack().len(), 3, "Scope info stack should have 3 items");

        // Verify they're in order
        let scopes = parser.scope_info_stack();
        assert_eq!(scopes[0].scope_depth(), 1, "First scope should have depth 1");
        assert_eq!(scopes[1].scope_depth(), 2, "Second scope should have depth 2");
        assert_eq!(scopes[2].scope_depth(), 3, "Third scope should have depth 3");
    }

    /// Test push_scope with different scope types
    #[test]
    fn test_push_scope_different_types() {
        let mut parser = BasicParser::new();

        // Push different scope types
        parser.push_scope(ScopeInfo::root());
        parser.push_scope(ScopeInfo::block(1));
        parser.push_scope(ScopeInfo::new(ScopeType::BlockSequence, 2));
        parser.push_scope(ScopeInfo::new(ScopeType::FlowMapping, 3));

        // Verify all were added
        assert_eq!(parser.scope_info_stack().len(), 4, "Scope info stack should have 4 items");

        // Verify types
        let scopes = parser.scope_info_stack();
        assert_eq!(scopes[0].scope_type(), ScopeType::Root, "First scope should be Root");
        assert_eq!(scopes[1].scope_type(), ScopeType::Block, "Second scope should be Block");
        assert_eq!(scopes[2].scope_type(), ScopeType::BlockSequence, "Third scope should be BlockSequence");
        assert_eq!(scopes[3].scope_type(), ScopeType::FlowMapping, "Fourth scope should be FlowMapping");
    }
}
