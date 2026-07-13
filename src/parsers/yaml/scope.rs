//! Scope representation data structures
//!
//! This module defines the core data structures for representing hierarchical scopes
//! in YAML documents. These structures enable accurate duplicate key detection by
//! tracking keys within their proper scope contexts.
//!
//! # Overview
//!
//! YAML documents use indentation to define nested scopes. Keys in different scopes
//! can have the same name without being duplicates. For example:
//!
//! ```yaml
//! services:
//!   web:
//!     host: localhost    # "host" in services.web scope
//!     port: 8080
//!   database:
//!     host: db.example.com  # "host" in services.database scope (NOT a duplicate)
//!     port: 5432
//! ```
//!
//! The scope representation system tracks:
//! - **Scope**: A mapping context at a specific nesting level
//! - **ScopeStack**: A hierarchical stack of active scopes during parsing
//! - **KeyContext**: Classification of key types (inline scalar, parent mapping, parent sequence)
//!
//! # Architecture
//!
//! ## Scope
//!
//! A `Scope` represents a single mapping context with:
//! - Indentation level (number of leading spaces)
//! - Set of keys defined within this scope
//! - Line number where this scope started
//! - Parent key that created this scope (e.g., "web" in "services: {...}")
//! - Whether this scope is in flow-style mapping ({key: value})
//!
//! ## ScopeStack
//!
//! A `ScopeStack` manages the hierarchical nature of YAML scopes:
//! - Maintains a stack of active scopes (top = current scope)
//! - Tracks base indentation size (usually 2 or 4 spaces)
//! - Handles scope transitions (enter/exit) as indentation changes
//! - Provides duplicate key detection within proper scope contexts
//!
//! # Examples
//!
//! ```
//! use armor::parsers::yaml::scope::{Scope, ScopeStack};
//!
//! let mut stack = ScopeStack::new(2); // Base indent of 2 spaces
//!
//! // Enter a scope when encountering a parent mapping
//! stack.enter_scope(2, 1, Some("services".to_string()));
//!
//! // Add keys to current scope
//! stack.add_key("web", 2).unwrap();
//! stack.add_key("database", 3).unwrap();
//!
//! // Detect duplicate in same scope
//! let result = stack.add_key("web", 4);
//! assert!(result.is_err()); // Duplicate key error
//! ```

use std::collections::HashSet;
use std::fmt;

#[cfg(debug_assertions)]
use log::debug as log_debug;
#[cfg(debug_assertions)]
use log::warn as log_warn;
#[cfg(debug_assertions)]
use log::trace as log_trace;

/// Classification of scope types in YAML documents
///
/// This enum represents the different types of scopes that can exist
/// during YAML parsing, helping to distinguish between block-style
/// mappings, flow-style collections, sequences, and document root.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ScopeType {
    /// Block-style mapping scope (indentation-based nesting)
    ///
    /// This is the most common YAML scope type, where nesting is indicated
    /// by increased indentation. For example:
    /// ```yaml
    /// parent:
    ///   child: value
    /// ```
    Block,

    /// Flow-style mapping scope (inline `{key: value}` syntax)
    ///
    /// Flow-style mappings use curly braces and can span multiple lines
    /// or be entirely on a single line. For example:
    /// ```yaml
    /// # Single line
    /// mapping: {key1: value1, key2: value2}
    ///
    /// # Multi-line
    /// mapping: {
    ///   key1: value1,
    ///   key2: value2
    /// }
    /// ```
    FlowMapping,

    /// Flow-style sequence scope (inline `[item1, item2]` syntax)
    ///
    /// Flow-style sequences use square brackets. For example:
    /// ```yaml
    /// items: [first, second, third]
    /// ```
    FlowSequence,

    /// Block-style sequence scope (dash-prefixed items)
    ///
    /// Block sequences use dashes to indicate items, with indentation
    /// showing nesting. For example:
    /// ```yaml
    /// items:
    ///   - first
    ///   - second
    ///   - third
    /// ```
    BlockSequence,

    /// Document root scope
    ///
    /// The top-level scope of a YAML document.
    Root,
}

impl fmt::Display for ScopeType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Block => write!(f, "block"),
            Self::FlowMapping => write!(f, "flow-mapping"),
            Self::FlowSequence => write!(f, "flow-sequence"),
            Self::BlockSequence => write!(f, "block-sequence"),
            Self::Root => write!(f, "root"),
        }
    }
}

/// Information about a scope in the scope stack
///
/// `ScopeInfo` captures metadata about scopes during YAML parsing.
/// Unlike the `Scope` struct which tracks the contents of a scope (keys,
/// parent key, etc.), `ScopeInfo` tracks the scope's structural properties:
/// its type and its depth in the nesting hierarchy.
///
/// # Purpose
///
/// This struct serves as a lightweight descriptor for stack entries,
/// enabling efficient scope classification and hierarchy tracking without
/// duplicating the full scope state. It is particularly useful for:
///
/// - Determining scope type transitions (block → flow, etc.)
/// - Tracking nesting depth for validation and diagnostics
/// - Providing context in error messages
/// - Optimizing scope stack operations
///
/// # Fields
///
/// - `scope_type`: The type of scope (block, flow, sequence, root)
/// - `scope_depth`: The nesting depth (0 for root, 1 for top-level keys, etc.)
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::scope::{ScopeInfo, ScopeType};
///
/// // Create a block scope at depth 2
/// let info = ScopeInfo::new(ScopeType::Block, 2);
/// assert_eq!(info.scope_type(), ScopeType::Block);
/// assert_eq!(info.scope_depth(), 2);
///
/// // Create a flow mapping at depth 1
/// let flow_info = ScopeInfo::new(ScopeType::FlowMapping, 1);
/// assert_eq!(flow_info.scope_type(), ScopeType::FlowMapping);
/// assert_eq!(flow_info.scope_depth(), 1);
/// ```
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ScopeInfo {
    /// The type of this scope (block, flow, sequence, root)
    scope_type: ScopeType,
    /// The nesting depth of this scope (0-based, root = 0)
    scope_depth: usize,
}

impl ScopeInfo {
    /// Create a new scope info
    ///
    /// # Arguments
    ///
    /// * `scope_type` - The type of scope
    /// * `scope_depth` - The nesting depth (0 for root, 1 for top-level keys, etc.)
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::{ScopeInfo, ScopeType};
    ///
    /// let info = ScopeInfo::new(ScopeType::Block, 1);
    /// assert_eq!(info.scope_type(), ScopeType::Block);
    /// assert_eq!(info.scope_depth(), 1);
    /// ```
    pub fn new(scope_type: ScopeType, scope_depth: usize) -> Self {
        Self {
            scope_type,
            scope_depth,
        }
    }

    /// Create a root scope info (depth 0)
    ///
    /// # Returns
    ///
    /// A `ScopeInfo` representing the document root
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeInfo;
    ///
    /// let root = ScopeInfo::root();
    /// assert_eq!(root.scope_depth(), 0);
    /// ```
    pub fn root() -> Self {
        Self {
            scope_type: ScopeType::Root,
            scope_depth: 0,
        }
    }

    /// Create a block scope info
    ///
    /// # Arguments
    ///
    /// * `scope_depth` - The nesting depth
    ///
    /// # Returns
    ///
    /// A `ScopeInfo` representing a block-style scope
    pub fn block(scope_depth: usize) -> Self {
        Self {
            scope_type: ScopeType::Block,
            scope_depth,
        }
    }

    /// Create a flow mapping scope info
    ///
    /// # Arguments
    ///
    /// * `scope_depth` - The nesting depth
    ///
    /// # Returns
    ///
    /// A `ScopeInfo` representing a flow-style mapping scope
    pub fn flow_mapping(scope_depth: usize) -> Self {
        Self {
            scope_type: ScopeType::FlowMapping,
            scope_depth,
        }
    }

    /// Create a block sequence scope info
    ///
    /// # Arguments
    ///
    /// * `scope_depth` - The nesting depth
    ///
    /// # Returns
    ///
    /// A `ScopeInfo` representing a block-style sequence scope
    pub fn block_sequence(scope_depth: usize) -> Self {
        Self {
            scope_type: ScopeType::BlockSequence,
            scope_depth,
        }
    }

    /// Get the scope type
    ///
    /// # Returns
    ///
    /// The `ScopeType` of this scope
    pub fn scope_type(&self) -> ScopeType {
        self.scope_type
    }

    /// Get the scope depth
    ///
    /// # Returns
    ///
    /// The nesting depth of this scope (0 for root, 1 for top-level, etc.)
    pub fn scope_depth(&self) -> usize {
        self.scope_depth
    }

    /// Check if this is a root scope
    ///
    /// # Returns
    ///
    /// `true` if this is the document root scope
    pub fn is_root(&self) -> bool {
        self.scope_type == ScopeType::Root
    }

    /// Check if this is a block-style scope
    ///
    /// # Returns
    ///
    /// `true` if this is a block-style mapping or sequence scope
    pub fn is_block(&self) -> bool {
        matches!(self.scope_type, ScopeType::Block | ScopeType::BlockSequence)
    }

    /// Check if this is a flow-style scope
    ///
    /// # Returns
    ///
    /// `true` if this is a flow-style mapping or sequence scope
    pub fn is_flow(&self) -> bool {
        matches!(self.scope_type, ScopeType::FlowMapping | ScopeType::FlowSequence)
    }

    /// Check if this is a sequence scope
    ///
    /// # Returns
    ///
    /// `true` if this is a sequence scope (block or flow)
    pub fn is_sequence(&self) -> bool {
        matches!(self.scope_type, ScopeType::BlockSequence | ScopeType::FlowSequence)
    }

    /// Check if this is a mapping scope
    ///
    /// # Returns
    ///
    /// `true` if this is a mapping scope (block or flow)
    pub fn is_mapping(&self) -> bool {
        matches!(self.scope_type, ScopeType::Block | ScopeType::FlowMapping)
    }

    /// Set the scope type
    ///
    /// # Arguments
    ///
    /// * `scope_type` - The new scope type
    pub fn set_scope_type(&mut self, scope_type: ScopeType) {
        self.scope_type = scope_type;
    }

    /// Set the scope depth
    ///
    /// # Arguments
    ///
    /// * `scope_depth` - The new scope depth
    pub fn set_scope_depth(&mut self, scope_depth: usize) {
        self.scope_depth = scope_depth;
    }

    /// Increment the scope depth by 1
    ///
    /// This is useful when entering a nested scope.
    pub fn increment_depth(&mut self) {
        self.scope_depth += 1;
    }

    /// Decrement the scope depth by 1
    ///
    /// This is useful when exiting to a parent scope.
    /// Will not go below 0.
    pub fn decrement_depth(&mut self) {
        if self.scope_depth > 0 {
            self.scope_depth -= 1;
        }
    }

    /// Get a description of this scope info
    ///
    /// # Returns
    ///
    /// A human-readable description of the scope
    pub fn describe(&self) -> String {
        format!("ScopeInfo(type={}, depth={})", self.scope_type, self.scope_depth)
    }
}

impl fmt::Display for ScopeInfo {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "ScopeInfo(type={}, depth={})", self.scope_type, self.scope_depth)
    }
}

/// Classification of indent transition types
///
/// This enum represents the three possible types of indent transitions
/// that can occur during YAML parsing.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum IndentTransitionType {
    /// Indent increased, indicating entry into a deeper scope
    EnterScope,
    /// Indent decreased, indicating exit to a parent scope
    ExitScope,
    /// No indent change, staying at the same scope level
    SameLevel,
}

impl IndentTransitionType {
    /// Classify an indent transition based on from/to indents
    pub fn classify(from_indent: usize, to_indent: usize) -> Self {
        use std::cmp::Ordering;
        match to_indent.cmp(&from_indent) {
            Ordering::Greater => Self::EnterScope,
            Ordering::Less => Self::ExitScope,
            Ordering::Equal => Self::SameLevel,
        }
    }

    /// Check if this is an EnterScope transition
    pub fn is_enter_scope(&self) -> bool {
        matches!(self, Self::EnterScope)
    }

    /// Check if this is an ExitScope transition
    pub fn is_exit_scope(&self) -> bool {
        matches!(self, Self::ExitScope)
    }

    /// Check if this is a SameLevel transition
    pub fn is_same_level(&self) -> bool {
        matches!(self, Self::SameLevel)
    }
}

impl fmt::Display for IndentTransitionType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::EnterScope => write!(f, "enter-scope"),
            Self::ExitScope => write!(f, "exit-scope"),
            Self::SameLevel => write!(f, "same-level"),
        }
    }
}

/// State machine for tracking indent level transitions
///
/// This struct maintains the current state of the parser's indent transition
/// tracking, enabling classification of scope operations during parsing.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IndentTransitionState {
    /// The previous indentation level
    pub from_indent: usize,
    /// The current indentation level
    pub to_indent: usize,
    /// The type of the last transition
    pub last_transition_type: IndentTransitionType,
    /// Whether the last transition had a key token
    pub last_had_key: bool,
}

impl IndentTransitionState {
    /// Create a new transition state in initial state
    pub fn new() -> Self {
        Self {
            from_indent: 0,
            to_indent: 0,
            last_transition_type: IndentTransitionType::SameLevel,
            last_had_key: false,
        }
    }

    /// Update the state with a new indent transition
    ///
    /// # Arguments
    ///
    /// * `from_indent` - The previous indentation level
    /// * `to_indent` - The new indentation level
    /// * `has_key` - Whether this transition occurred on a line with a key token
    pub fn update(&mut self, from_indent: usize, to_indent: usize, has_key: bool) {
        self.from_indent = from_indent;
        self.to_indent = to_indent;
        self.last_transition_type = IndentTransitionType::classify(from_indent, to_indent);
        self.last_had_key = has_key;
    }

    /// Get the current transition type
    pub fn current_transition_type(&self) -> IndentTransitionType {
        self.last_transition_type
    }

    /// Check if the last transition was an enter-scope
    pub fn is_entering_scope(&self) -> bool {
        self.last_transition_type.is_enter_scope()
    }

    /// Check if the last transition was an exit-scope
    pub fn is_exiting_scope(&self) -> bool {
        self.last_transition_type.is_exit_scope()
    }

    /// Check if the last transition was same-level
    pub fn is_same_level(&self) -> bool {
        self.last_transition_type.is_same_level()
    }

    /// Get a description of the current state
    pub fn describe(&self) -> String {
        format!(
            "IndentTransitionState(from={}, to={}, type={}, had_key={})",
            self.from_indent, self.to_indent, self.last_transition_type, self.last_had_key
        )
    }

    /// Reset to initial state
    pub fn reset(&mut self) {
        *self = Self::new();
    }
}

impl Default for IndentTransitionState {
    fn default() -> Self {
        Self::new()
    }
}

impl fmt::Display for IndentTransitionState {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "IndentTransitionState(from={}, to={}, type={}, had_key={})",
            self.from_indent, self.to_indent, self.last_transition_type, self.last_had_key
        )
    }
}

/// A scope representing a mapping context at a specific nesting level
///
/// Scopes are created when the parser encounters a parent mapping (a key whose value
/// is itself a mapping). Each scope maintains its own set of keys, independent of
/// keys in parent or sibling scopes.
#[derive(Debug, Clone)]
pub struct Scope {
    /// Indentation level (number of leading spaces)
    pub indent_level: usize,
    /// Keys defined within this scope
    pub keys: HashSet<String>,
    /// Line number where this scope started (1-indexed)
    pub start_line: usize,
    /// Parent key that created this scope (e.g., "web" in "services: {...}")
    pub parent_key: Option<String>,
    /// Whether this scope is in flow-style mapping ({key: value})
    pub is_flow_style: bool,
    /// Whether this scope is within a sequence context
    pub in_sequence_context: bool,
    /// Unique identifier for sequence items to distinguish them at same indent
    pub sequence_item_id: Option<usize>,
}

impl Scope {
    /// Create a new scope
    ///
    /// # Arguments
    ///
    /// * `indent_level` - Number of leading spaces for this scope
    /// * `start_line` - Line number where this scope started (1-indexed)
    /// * `parent_key` - Optional parent key that created this scope
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::Scope;
    ///
    /// let scope = Scope::new(2, 5, Some("services".to_string()));
    /// assert_eq!(scope.indent_level, 2);
    /// assert_eq!(scope.start_line, 5);
    /// assert_eq!(scope.parent_key, Some("services".to_string()));
    /// ```
    pub fn new(indent_level: usize, start_line: usize, parent_key: Option<String>) -> Self {
        Self {
            indent_level,
            keys: HashSet::new(),
            start_line,
            parent_key,
            is_flow_style: false,
            in_sequence_context: false,
            sequence_item_id: None,
        }
    }

    /// Add a key to this scope, returning true if it's a duplicate
    ///
    /// # Arguments
    ///
    /// * `key` - The key to add
    ///
    /// # Returns
    ///
    /// `true` if the key already exists in this scope (duplicate), `false` otherwise
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::Scope;
    ///
    /// let mut scope = Scope::new(0, 1, None);
    ///
    /// assert_eq!(scope.add_key("first"), false); // First occurrence
    /// assert_eq!(scope.add_key("second"), false); // Different key
    /// assert_eq!(scope.add_key("first"), true);  // Duplicate!
    /// ```
    pub fn add_key(&mut self, key: &str) -> bool {
        !self.keys.insert(key.to_string())
    }

    /// Check if this scope contains a key
    ///
    /// # Arguments
    ///
    /// * `key` - The key to check
    ///
    /// # Returns
    ///
    /// `true` if the key exists in this scope, `false` otherwise
    pub fn contains_key(&self, key: &str) -> bool {
        self.keys.contains(key)
    }

    /// Get the number of keys in this scope
    pub fn key_count(&self) -> usize {
        self.keys.len()
    }

    /// Clear all keys from this scope
    pub fn clear_keys(&mut self) {
        self.keys.clear();
    }
}

impl fmt::Display for Scope {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "Scope(indent={}, ", self.indent_level)?;
        if let Some(ref parent) = self.parent_key {
            write!(f, "parent={}, ", parent)?;
        }
        write!(f, "keys={})", self.keys.len())
    }
}

/// Hierarchical stack of active scopes
///
/// The ScopeStack maintains the hierarchical nature of YAML scopes during parsing.
/// As the parser encounters different indentation levels, it enters and exits scopes,
/// ensuring that duplicate key detection only considers keys within the same scope.
#[derive(Debug, Clone)]
pub struct ScopeStack {
    /// Stack of active scopes (top = current scope)
    pub scopes: Vec<Scope>,
    /// Base indentation size (usually 2 or 4 spaces)
    base_indent: usize,
    /// Sequence item counter for generating unique IDs
    sequence_item_counter: usize,
    /// History of indent transitions (whether or not they have keys)
    indent_transitions: Vec<IndentTransition>,
    /// The last recorded indent level (to detect transitions)
    last_indent: usize,
}

impl ScopeStack {
    /// Create a new scope stack
    ///
    /// # Arguments
    ///
    /// * `base_indent` - The base indentation size in spaces (usually 2 or 4)
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let stack = ScopeStack::new(2); // 2-space indentation
    /// ```
    pub fn new(base_indent: usize) -> Self {
        Self {
            scopes: Vec::new(), // Empty stack - initialized with no scopes
            base_indent,
            sequence_item_counter: 0,
            indent_transitions: Vec::new(),
            last_indent: 0,
        }
    }

    /// Get the current scope (top of stack)
    ///
    /// # Returns
    ///
    /// `Some(&mut Scope)` if the stack has at least one scope, `None` if empty
    pub fn current_scope(&mut self) -> Option<&mut Scope> {
        self.scopes.last_mut()
    }

    /// Get the current scope as an immutable reference
    ///
    /// # Returns
    ///
    /// `Some(&Scope)` if the stack has at least one scope, `None` if empty
    pub fn current_scope_ref(&self) -> Option<&Scope> {
        self.scopes.last()
    }

    /// Get scope for a specific indentation level
    ///
    /// # Arguments
    ///
    /// * `indent_level` - The indentation level to search for
    ///
    /// # Returns
    ///
    /// `Some(&Scope)` if a scope at this level exists, `None` otherwise
    pub fn get_scope_at_level(&self, indent_level: usize) -> Option<&Scope> {
        self.scopes.iter().find(|s| s.indent_level == indent_level)
    }

    /// Enter a new scope (when indent increases)
    ///
    /// This method is called when the parser encounters increased indentation,
    /// indicating entry into a nested scope.
    ///
    /// # Arguments
    ///
    /// * `indent_level` - The new indentation level
    /// * `line` - The line number where this scope starts (1-indexed)
    /// * `parent_key` - Optional parent key that created this scope
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.enter_scope(2, 5, Some("services".to_string()));
    /// ```
    pub fn enter_scope(&mut self, indent_level: usize, line: usize, parent_key: Option<String>) {
        // Debug logging for scope entry
        #[cfg(debug_assertions)]
        {
            let parent_info = parent_key.as_ref().map(|k| k.as_str()).unwrap_or("<anonymous>");
            let scope_type = "Mapping";
            log_debug!(
                "[SCOPE ENTRY] type={}, line={}, indent={}, parent='{}', current_depth={}, path='{}'",
                scope_type,
                line,
                indent_level,
                parent_info,
                self.depth(),
                self.get_scope_path()
            );
        }

        // Check if we already have a scope at this level
        if let Some(existing) = self.get_scope_at_level(indent_level) {
            // We're re-entering a scope level - clear and reuse
            // This handles sibling mappings correctly
            let mut fresh_scope = Scope::new(indent_level, line, parent_key);
            fresh_scope.is_flow_style = existing.is_flow_style;

            // Remove all scopes deeper than this level
            let before_depth = self.depth();
            let before_path = self.get_scope_path();

            #[cfg(debug_assertions)]
            {
                let scopes_to_remove: Vec<_> = self.scopes.iter()
                    .filter(|s| s.indent_level > indent_level)
                    .map(|s| format!("(indent={}, parent={:?})", s.indent_level, s.parent_key))
                    .collect();
                if !scopes_to_remove.is_empty() {
                    log_debug!("[SCOPE EXIT] type=Mapping reuse cleanup, removing {} scopes: {:?}",
                             scopes_to_remove.len(),
                             scopes_to_remove);
                }
            }

            self.scopes.retain(|s| s.indent_level <= indent_level);

            #[cfg(debug_assertions)]
            {
                let after_retain_depth = self.depth();
                let removed_count = before_depth - after_retain_depth;
                if removed_count > 0 {
                    log_debug!("[SCOPE EXIT] type=Mapping reuse cleanup, removed {} scopes: '{}' -> '{}'",
                             removed_count,
                             before_path,
                             self.get_scope_path());
                }
            }

            self.scopes.push(fresh_scope);

            #[cfg(debug_assertions)]
            {
                log_debug!("[SCOPE ENTRY] type=Mapping (reuse), indent={}, cleared scopes deeper than this level", indent_level);
            }
        } else {
            // Create new scope
            let new_scope = Scope::new(indent_level, line, parent_key);
            self.scopes.push(new_scope);

            #[cfg(debug_assertions)]
            {
                log_debug!("[SCOPE ENTRY] type=Mapping (new), indent={}, created new scope at this level", indent_level);
            }
        }

        #[cfg(debug_assertions)]
        {
            log_debug!("[SCOPE ENTRY] After entry: depth={}, current_indent={}", self.depth(), self.current_indent());
        }
    }

    /// Exit one level to immediate parent scope
    ///
    /// This method exits from the current scope to its immediate parent scope.
    /// It's a convenience method for single-level scope exits.
    ///
    /// # Returns
    ///
    /// `true` if successfully exited to parent, `false` if already at root scope
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.enter_scope(2, 1, Some("parent".to_string()));
    /// stack.enter_scope(4, 2, Some("child".to_string()));
    ///
    /// // Exit from child to parent
    /// let exited = stack.exit_one_level();
    /// assert!(exited);
    /// assert_eq!(stack.current_indent(), 2);
    ///
    /// // Exit from parent to root
    /// let exited = stack.exit_one_level();
    /// assert!(exited);
    /// assert_eq!(stack.current_indent(), 0);
    ///
    /// // Already at root - cannot exit further
    /// let exited = stack.exit_one_level();
    /// assert!(!exited);
    /// ```
    pub fn exit_one_level(&mut self) -> bool {
        // Edge case: already at root scope (depth of 1)
        if self.depth() <= 1 {
            #[cfg(debug_assertions)]
            {
                log_debug!("[SCOPE EXIT ONE LEVEL] Already at root scope (depth={}), cannot exit further", self.depth());
            }
            return false;
        }

        // Get the current scope's indent level
        let current_indent = self.current_indent();

        // Find the parent scope's indent level
        // The parent is the second-to-last scope in the stack
        let parent_indent = if self.depth() >= 2 {
            self.scopes[self.depth() - 2].indent_level
        } else {
            // Should not reach here due to the depth check above,
            // but handle gracefully by defaulting to root indent (0)
            0
        };

        #[cfg(debug_assertions)]
        {
            let before_path = self.get_scope_path();
            log_debug!(
                "[SCOPE EXIT ONE LEVEL] exiting from indent={} to parent indent={}, path='{}'",
                current_indent,
                parent_indent,
                before_path
            );
        }

        // Use the existing exit_to_scope method with the parent's indent
        self.exit_to_scope(parent_indent);

        true
    }

    /// Exit to parent scope (when indent decreases)
    ///
    /// This method is called when the parser encounters decreased indentation,
    /// indicating exit from a nested scope back to its parent.
    ///
    /// # Arguments
    ///
    /// * `target_indent` - The indentation level to exit to
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.enter_scope(2, 1, Some("parent".to_string()));
    /// stack.exit_to_scope(0); // Exit back to root
    /// ```
    pub fn exit_to_scope(&mut self, target_indent: usize) {
        #[cfg(debug_assertions)]
        {
            log_debug!(
                "[SCOPE EXIT] target_indent={}, current_depth={}, current_indent={}, path='{}'",
                target_indent,
                self.depth(),
                self.current_indent(),
                self.get_scope_path()
            );
        }

        // Edge case: can't exit to a deeper level than current
        if target_indent > self.current_indent() {
            #[cfg(debug_assertions)]
            {
                log_warn!("[SCOPE EXIT] WARNING: target_indent={} > current_indent={}, ignoring exit request", target_indent, self.current_indent());
            }
            return;
        }

        // Edge case: can't exit if stack would become empty
        let would_be_empty = self.scopes.iter()
            .filter(|s| s.indent_level <= target_indent)
            .count() == 0;

        if would_be_empty {
            #[cfg(debug_assertions)]
            {
                log_warn!("[SCOPE EXIT] WARNING: exit would empty scope stack, keeping at least root scope");
            }
            // Keep at least the root scope
            self.scopes.retain(|s| s.indent_level == 0);
            return;
        }

        // Remove all scopes deeper than target
        let before_depth = self.depth();
        self.scopes.retain(|s| s.indent_level <= target_indent);

        #[cfg(debug_assertions)]
        {
            log_debug!("[SCOPE EXIT] Removed {} scopes deeper than indent={}", before_depth - self.depth(), target_indent);
        }

        // Search for target scope in hierarchy
        // This handles cases where the exact target indent doesn't exist in the stack
        // by finding the closest parent scope or creating a fallback if needed
        if !self.scopes.iter().any(|s| s.indent_level == target_indent) {
            // No exact match at target indent - search for closest parent
            #[cfg(debug_assertions)]
            {
                log_warn!("[SCOPE EXIT] WARNING: No scope found at target_indent={}, searching for closest parent scope",
                         target_indent);
            }

            // Find the closest scope with indent <= target_indent
            let closest_scope = self.scopes.iter()
                .filter(|s| s.indent_level <= target_indent)
                .max_by_key(|s| s.indent_level);

            match closest_scope {
                Some(scope) => {
                    // Found a parent scope - use that instead
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[SCOPE EXIT] Found closest parent scope at indent={}, path='{}', using existing scope",
                                 scope.indent_level, self.get_scope_path());
                    }
                    // We've already retained scopes <= target_indent, so we're at the closest scope
                }
                None => {
                    // No suitable parent scope found - this shouldn't happen in normal YAML
                    // but we handle it gracefully by creating a fallback scope
                    #[cfg(debug_assertions)]
                    {
                        log_warn!("[SCOPE EXIT] WARNING: No suitable parent scope found, creating fallback scope at indent={}",
                                 target_indent);
                    }
                    let fallback_scope = Scope::new(target_indent, 0, None);
                    self.scopes.push(fallback_scope);
                }
            }
        }

        #[cfg(debug_assertions)]
        {
            log_debug!("[SCOPE EXIT] After exit: depth={}, current_indent={}, path='{}'", self.depth(), self.current_indent(), self.get_scope_path());
        }
    }

    /// Check if current scope contains a key
    ///
    /// # Arguments
    ///
    /// * `key` - The key to check for
    ///
    /// # Returns
    ///
    /// `true` if the key exists in the current scope, `false` otherwise
    pub fn contains_key(&self, key: &str) -> bool {
        self.scopes.last()
            .and_then(|scope| Some(scope.contains_key(key)))
            .unwrap_or(false)
    }

    /// Check if any scope in the hierarchy contains a key
    ///
    /// This searches through all scopes in the stack, not just the current one.
    ///
    /// # Arguments
    ///
    /// * `key` - The key to search for
    ///
    /// # Returns
    ///
    /// `true` if the key exists in any scope, `false` otherwise
    pub fn contains_key_in_any_scope(&self, key: &str) -> bool {
        self.scopes.iter().any(|scope| scope.contains_key(key))
    }

    /// Add a key to current scope
    ///
    /// # Arguments
    ///
    /// * `key` - The key to add
    /// * `line` - The line number where the key appears (1-indexed)
    ///
    /// # Returns
    ///
    /// `Ok(())` if the key was added successfully
    /// `Err(DuplicateKeyError)` if the key already exists in the current scope
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    ///
    /// // First occurrence succeeds
    /// assert!(stack.add_key("host", 1).is_ok());
    ///
    /// // Second occurrence fails
    /// assert!(stack.add_key("host", 2).is_err());
    /// ```
    pub fn add_key(&mut self, key: &str, line: usize) -> Result<(), DuplicateKeyError> {
        // Auto-create root scope if stack is empty
        if self.scopes.is_empty() {
            self.scopes.push(Scope::new(0, 0, None));
        }

        let scope_path = self.get_scope_path();

        // Check if key exists before adding
        if self.contains_key(key) {
            let scope = self.current_scope().unwrap();
            Err(DuplicateKeyError {
                key: key.to_string(),
                scope_path,
                first_line: scope.start_line,
                duplicate_line: line,
            })
        } else {
            let scope = self.current_scope().unwrap();
            scope.add_key(key);
            Ok(())
        }
    }

    /// Get human-readable path to current scope
    ///
    /// Returns a dot-separated path representing the scope hierarchy,
    /// e.g., "services.web.database" for a deeply nested scope.
    ///
    /// # Returns
    ///
    /// A string representing the scope path
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.enter_scope(2, 1, Some("services".to_string()));
    /// stack.enter_scope(4, 2, Some("web".to_string()));
    /// assert_eq!(stack.get_scope_path(), "services.web");
    /// ```
    pub fn get_scope_path(&self) -> String {
        let mut path = Vec::new();
        for scope in &self.scopes {
            if let Some(ref key) = scope.parent_key {
                path.push(key.clone());
            }
        }
        path.join(".")
    }

    /// Get current indent level
    ///
    /// # Returns
    ///
    /// The indentation level of the current scope, or 0 if stack is empty
    pub fn current_indent(&self) -> usize {
        self.scopes.last()
            .map(|scope| scope.indent_level)
            .unwrap_or(0)
    }

    /// Get the number of active scopes in the stack
    pub fn depth(&self) -> usize {
        self.scopes.len()
    }

    /// Clear all scopes and reset to empty
    ///
    /// This resets the scope stack to its initial state with no scopes.
    /// The next add_key() will auto-create a root scope.
    pub fn reset(&mut self) {
        self.scopes.clear();
        self.clear_indent_transitions();
    }

    /// Get the base indentation size
    pub fn base_indent(&self) -> usize {
        self.base_indent
    }

    /// Set the base indentation size
    ///
    /// # Arguments
    ///
    /// * `base_indent` - The new base indentation size in spaces
    pub fn set_base_indent(&mut self, base_indent: usize) {
        self.base_indent = base_indent;
    }

    /// Enter a sequence context (when we see a `-` item)
    ///
    /// This method is called when the parser encounters a sequence item,
    /// creating a new scope for that item to prevent false duplicate detection
    /// between items at the same indentation level.
    ///
    /// # Arguments
    ///
    /// * `indent_level` - The indentation level of the sequence item
    /// * `line` - The line number where this sequence item starts (1-indexed)
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.enter_sequence_scope(2, 5); // Enter sequence item at indent 2
    /// ```
    pub fn enter_sequence_scope(&mut self, indent_level: usize, line: usize) {
        #[cfg(debug_assertions)]
        {
            log_debug!(
                "[SCOPE ENTRY] type=Sequence, line={}, indent={}, current_depth={}, path='{}'",
                line,
                indent_level,
                self.depth(),
                self.get_scope_path()
            );
        }

        // Remove all scopes at or deeper than this level
        // When entering a sequence scope, we clear any existing scope at this level
        // (including parent mappings) and all deeper scopes, then add a new sequence scope
        let before_depth = self.depth();
        let before_path = self.get_scope_path();

        #[cfg(debug_assertions)]
        {
            let scopes_to_remove: Vec<_> = self.scopes.iter()
                .filter(|s| s.indent_level >= indent_level)
                .map(|s| format!("(indent={}, parent={:?})", s.indent_level, s.parent_key))
                .collect();
            if !scopes_to_remove.is_empty() {
                log_debug!("[SCOPE EXIT] type=Sequence entry cleanup, removing {} scopes at or deeper than indent={}: {:?}",
                         scopes_to_remove.len(),
                         indent_level,
                         scopes_to_remove);
            }
        }

        // Only retain scopes shallower than indent_level
        // Sequence scopes completely replace any mapping at the same level
        // (unlike sibling mappings which preserve the parent)
        self.scopes.retain(|s| s.indent_level < indent_level);

        #[cfg(debug_assertions)]
        {
            let removed_count = before_depth - self.depth();
            if removed_count > 0 {
                log_debug!("[SCOPE EXIT] type=Sequence entry cleanup, removed {} scopes: '{}' -> '{}'",
                         removed_count,
                         before_path,
                         self.get_scope_path());
            }
            log_debug!("[SCOPE ENTRY] type=Sequence, indent={}, cleared scopes at or deeper than this level (replaces any parent mapping)", indent_level);
        }

        // Always create a new scope for each sequence item
        // Each sequence item needs a fresh scope - we don't reuse existing sequence scopes
        let mut new_scope = Scope::new(indent_level, line, None);
        new_scope.in_sequence_context = true;
        self.sequence_item_counter += 1;
        new_scope.sequence_item_id = Some(self.sequence_item_counter);
        self.scopes.push(new_scope);

        #[cfg(debug_assertions)]
        {
            log_debug!("[SCOPE ENTRY] type=Sequence (new), item_id={}, created new sequence scope", self.sequence_item_counter);
        }

        #[cfg(debug_assertions)]
        {
            log_debug!("[SCOPE ENTRY] type=Sequence, after entry: depth={}, in_sequence={}", self.depth(), self.in_sequence_context());
        }
    }

    /// Check if we're in a sequence context
    ///
    /// # Returns
    ///
    /// `true` if the current scope is within a sequence context, `false` otherwise
    pub fn in_sequence_context(&self) -> bool {
        self.scopes.last()
            .map(|scope| scope.in_sequence_context)
            .unwrap_or(false)
    }

    /// Record an indent transition
    ///
    /// This method tracks indentation level changes during parsing, whether or not
    /// they occur on lines with key tokens. This enables detection of indent changes
    /// on blank lines, comments, or other non-key lines.
    ///
    /// # Arguments
    ///
    /// * `line_number` - The line number where this transition occurred (1-indexed)
    /// * `new_indent` - The new indentation level
    /// * `has_key` - Whether this transition occurred on a line with a key token
    /// * `raw_line` - The raw line content (for debugging)
    ///
    /// Record an indent transition
    ///
    /// This method tracks indentation level changes during parsing, whether or not
    /// they occur on lines with key tokens. This enables detection of indent changes
    /// on blank lines, comments, or other non-key lines.
    ///
    /// # Arguments
    ///
    /// * `line_number` - The line number where this transition occurred (1-indexed)
    /// * `new_indent` - The new indentation level
    /// * `has_key` - Whether this transition occurred on a line with a key token
    /// * `raw_line` - The raw line content (for debugging)
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.record_indent_transition(5, 2, false, "  # blank line with indent");
    /// ```
    pub fn record_indent_transition(&mut self, line_number: usize, new_indent: usize, has_key: bool, raw_line: &str) {
        // Only record if the indent actually changed
        if new_indent != self.last_indent {
            // Store the old indent before updating
            let old_indent = self.last_indent;

            // Classify the line type
            let line_classification = classify_line_type(raw_line);

            let transition = IndentTransition::new(
                line_number,
                old_indent,
                new_indent,
                has_key,
                raw_line,
                line_classification,
            );
            self.indent_transitions.push(transition);
            self.last_indent = new_indent;

            #[cfg(debug_assertions)]
            {
                let direction = if new_indent > old_indent {
                    "increase"
                } else {
                    "decrease"
                };
                let key_status = if has_key { "with-key" } else { "without-key" };
                log_debug!(
                    "[INDENT TRANSITION] line={}, {}→{}, {}, {}, raw='{}'",
                    line_number,
                    old_indent,
                    new_indent,
                    key_status,
                    line_classification,
                    raw_line.trim()
                );
            }
        }
    }

    /// Get all recorded indent transitions
    ///
    /// # Returns
    ///
    /// A slice of all indent transitions recorded during parsing
    pub fn get_indent_transitions(&self) -> &[IndentTransition] {
        &self.indent_transitions
    }

    /// Get indent transitions without keys
    ///
    /// Returns only those transitions that occurred on lines without key tokens,
    /// such as blank lines or comments.
    ///
    /// # Returns
    ///
    /// A vector of indent transitions that occurred without keys
    pub fn get_transitions_without_keys(&self) -> Vec<&IndentTransition> {
        self.indent_transitions
            .iter()
            .filter(|t| t.is_without_key())
            .collect()
    }

    /// Get indent transitions with keys
    ///
    /// Returns only those transitions that occurred on lines with key tokens.
    ///
    /// # Returns
    ///
    /// A vector of indent transitions that occurred with keys
    pub fn get_transitions_with_keys(&self) -> Vec<&IndentTransition> {
        self.indent_transitions
            .iter()
            .filter(|t| t.has_key)
            .collect()
    }

    /// Get enter-scope transitions
    ///
    /// Returns only those transitions that represent entering a deeper scope.
    ///
    /// # Returns
    ///
    /// A vector of enter-scope transitions
    pub fn get_enter_scope_transitions(&self) -> Vec<&IndentTransition> {
        self.indent_transitions
            .iter()
            .filter(|t| t.is_enter_scope())
            .collect()
    }

    /// Get exit-scope transitions
    ///
    /// Returns only those transitions that represent exiting to a parent scope.
    ///
    /// # Returns
    ///
    /// A vector of exit-scope transitions
    pub fn get_exit_scope_transitions(&self) -> Vec<&IndentTransition> {
        self.indent_transitions
            .iter()
            .filter(|t| t.is_exit_scope())
            .collect()
    }

    /// Get same-level transitions
    ///
    /// Returns only those transitions that represent staying at the same level.
    ///
    /// # Returns
    ///
    /// A vector of same-level transitions
    pub fn get_same_level_transitions(&self) -> Vec<&IndentTransition> {
        self.indent_transitions
            .iter()
            .filter(|t| t.is_same_level())
            .collect()
    }

    /// Get transition count by type
    ///
    /// Returns a tuple of (enter_count, exit_count, same_level_count).
    ///
    /// # Returns
    ///
    /// A tuple with counts of each transition type
    pub fn get_transition_counts(&self) -> (usize, usize, usize) {
        let enter = self.get_enter_scope_transitions().len();
        let exit = self.get_exit_scope_transitions().len();
        let same = self.get_same_level_transitions().len();
        (enter, exit, same)
    }

    /// Clear all indent transitions
    ///
    /// This is typically called when resetting the scope stack for a new document.
    pub fn clear_indent_transitions(&mut self) {
        self.indent_transitions.clear();
        self.last_indent = 0;
    }

    /// Get the last recorded indent level
    ///
    /// # Returns
    ///
    /// The most recent indent level recorded
    pub fn get_last_indent(&self) -> usize {
        self.last_indent
    }

    /// Set the last indent level directly
    ///
    /// This can be used to initialize the indent tracking or to correct it after
    /// document markers or other special constructs.
    ///
    /// # Arguments
    ///
    /// * `indent` - The indent level to set
    pub fn set_last_indent(&mut self, indent: usize) {
        self.last_indent = indent;
    }

    /// Process an indent transition without a key token
    ///
    /// This method handles indentation changes that occur on lines without key tokens,
    /// such as blank lines or comments. It determines if the indent change represents
    /// a valid scope transition and triggers the appropriate scope entry/exit.
    ///
    /// # Arguments
    ///
    /// * `line_number` - The line number where this transition occurred (1-indexed)
    /// * `new_indent` - The new indentation level
    ///
    /// # Returns
    ///
    /// `true` if a scope transition occurred, `false` otherwise
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::yaml::scope::ScopeStack;
    ///
    /// let mut stack = ScopeStack::new(2);
    /// stack.enter_scope(2, 1, Some("parent".to_string()));
    /// // Process indent decrease on blank line
    /// let transitioned = stack.process_indent_transition_without_key(5, 0);
    /// assert!(transitioned); // Should exit to root scope
    /// ```
    pub fn process_indent_transition_without_key(&mut self, line_number: usize, new_indent: usize) -> bool {
        use std::cmp::Ordering;

        // Compare new indent with current scope indent
        let current_indent = self.current_indent();

        match new_indent.cmp(&current_indent) {
            Ordering::Greater => {
                // Indent increased - but without a key, this is unusual
                // We don't enter a new scope without a parent key
                // Just record the transition for tracking purposes
                #[cfg(debug_assertions)]
                {
                    log_debug!(
                        "[INDENT WITHOUT KEY] line={}, indent={}→{} (increase), NO SCOPE ENTRY (no parent key)",
                        line_number, current_indent, new_indent
                    );
                }
                false
            }
            Ordering::Less => {
                // Indent decreased - exit to parent scope
                // This is valid even without a key (e.g., blank line at end of nested block)
                #[cfg(debug_assertions)]
                {
                    let before_path = self.get_scope_path();
                    log_debug!(
                        "[INDENT WITHOUT KEY] line={}, indent={}→{} (decrease), exiting scope: '{}'",
                        line_number, current_indent, new_indent, before_path
                    );
                }

                self.exit_to_scope(new_indent);
                true
            }
            Ordering::Equal => {
                // Same indent - no scope change needed
                false
            }
        }
    }
}

impl fmt::Display for ScopeStack {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "ScopeStack(depth={}, base_indent={}, current_path={})",
               self.depth(), self.base_indent, self.get_scope_path())
    }
}

/// Error when a duplicate key is detected
///
/// This error is returned when attempting to add a key that already exists
/// in the current scope.
#[derive(Debug, Clone)]
pub struct DuplicateKeyError {
    /// The duplicate key
    pub key: String,
    /// The scope path where the duplicate was found
    pub scope_path: String,
    /// The line number where the key was first defined
    pub first_line: usize,
    /// The line number where the duplicate was found
    pub duplicate_line: usize,
}

impl DuplicateKeyError {
    /// Create a new duplicate key error
    ///
    /// # Arguments
    ///
    /// * `key` - The duplicate key name
    /// * `scope_path` - The scope path where the duplicate was found
    /// * `first_line` - Line number of first occurrence
    /// * `duplicate_line` - Line number of duplicate occurrence
    pub fn new(key: String, scope_path: String, first_line: usize, duplicate_line: usize) -> Self {
        Self {
            key,
            scope_path,
            first_line,
            duplicate_line,
        }
    }

    /// Get a formatted error message
    ///
    /// # Returns
    ///
    /// A human-readable error message
    pub fn message(&self) -> String {
        format!(
            "Line {}: duplicate key '{}' in scope '{}'\n  First defined at: Line {}",
            self.duplicate_line, self.key, self.scope_path, self.first_line
        )
    }
}

impl fmt::Display for DuplicateKeyError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Line {}: duplicate key '{}' in scope '{}'\n  First defined at: Line {}",
            self.duplicate_line, self.key, self.scope_path, self.first_line
        )
    }
}

impl std::error::Error for DuplicateKeyError {}

/// Key context classification
///
/// Enumerates the different contexts in which a key can appear in a YAML document.
/// This classification helps the parser understand how to handle different key types.
#[derive(Debug, Clone, PartialEq)]
pub enum KeyContext {
    /// Key with inline scalar value: "key: value"
    ///
    /// The key has a simple scalar value on the same line
    InlineScalar {
        /// The key name
        key: String,
        /// The scalar value
        value: String,
    },

    /// Parent key with nested mapping: "key:\n  nested: value"
    ///
    /// The key's value is a nested mapping (indentation increases after this key)
    ParentMapping {
        /// The parent key name
        key: String,
    },

    /// Key with nested sequence: "key:\n  - item1"
    ///
    /// The key's value is a sequence of items
    ParentSequence {
        /// The parent key name
        key: String,
    },
}

impl KeyContext {
    /// Get the key name from any key context variant
    ///
    /// # Returns
    ///
    /// The key name as a string slice
    pub fn key_name(&self) -> &str {
        match self {
            KeyContext::InlineScalar { key, .. } => key,
            KeyContext::ParentMapping { key } => key,
            KeyContext::ParentSequence { key } => key,
        }
    }

    /// Check if this is a parent key (creates a new scope)
    ///
    /// # Returns
    ///
    /// `true` if this key creates a new scope (mapping or sequence)
    pub fn is_parent_key(&self) -> bool {
        matches!(self, KeyContext::ParentMapping { .. } | KeyContext::ParentSequence { .. })
    }

    /// Check if this is an inline scalar (does not create a new scope)
    ///
    /// # Returns
    ///
    /// `true` if this key has an inline scalar value
    pub fn is_inline_scalar(&self) -> bool {
        matches!(self, KeyContext::InlineScalar { .. })
    }
}

impl fmt::Display for KeyContext {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            KeyContext::InlineScalar { key, value } => {
                write!(f, "InlineScalar(key='{}', value='{}')", key, value)
            }
            KeyContext::ParentMapping { key } => {
                write!(f, "ParentMapping(key='{}')", key)
            }
            KeyContext::ParentSequence { key } => {
                write!(f, "ParentSequence(key='{}')", key)
            }
        }
    }
}

/// Extract key context from a line
///
/// Analyzes a YAML line to determine the context of a key.
/// Returns `None` if the line doesn't contain a valid key.
///
/// # Arguments
///
/// * `line` - The YAML line to analyze
///
/// # Returns
///
/// `Some(KeyContext)` if a key is found, `None` otherwise
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::scope::{extract_key_context, KeyContext};
///
/// // Inline scalar
/// let ctx = extract_key_context("host: localhost").unwrap();
/// assert!(matches!(ctx, KeyContext::InlineScalar { .. }));
///
/// // Parent mapping
/// let ctx = extract_key_context("services:").unwrap();
/// assert!(matches!(ctx, KeyContext::ParentMapping { .. }));
/// ```
pub fn extract_key_context(line: &str) -> Option<KeyContext> {
    let trimmed = line.trim();

    // Find colon position
    let colon_pos = trimmed.find(':')?;
    let key_part = &trimmed[..colon_pos];
    let after_colon = &trimmed[colon_pos + 1..];

    // Skip if key is empty
    let key = key_part.trim();
    if key.is_empty() {
        return None;
    }

    // Skip if key contains invalid characters (like in flow style)
    if key.contains('{') || key.contains('}') || key.contains('[') || key.contains(']') {
        return None;
    }

    // Strip sequence dash from key if present
    // Handles: "- name: value" -> key should be "name", not "- name"
    let actual_key = if key.starts_with("- ") {
        // Remove the "- " prefix (dash and space)
        key[2..].trim()
    } else if key.starts_with('-') && key.len() > 1 {
        // Dash followed by non-space (invalid, but handle gracefully)
        &key[1..]
    } else {
        key
    };

    // Skip if actual key is empty after stripping dash
    if actual_key.is_empty() {
        return None;
    }

    // Classify based on what comes after the colon
    let context = if after_colon.trim().is_empty() {
        KeyContext::ParentMapping { key: actual_key.to_string() }
    } else {
        KeyContext::InlineScalar {
            key: actual_key.to_string(),
            value: after_colon.trim().to_string(),
        }
    };

    Some(context)
}

/// Get leading whitespace length from a line
///
/// # Arguments
///
/// * `line` - The line to analyze
///
/// # Returns
///
/// The number of leading whitespace characters
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::scope::get_leading_whitespace_length;
///
/// assert_eq!(get_leading_whitespace_length("  key: value"), 2);
/// assert_eq!(get_leading_whitespace_length("key: value"), 0);
/// assert_eq!(get_leading_whitespace_length("    nested: value"), 4);
/// ```
pub fn get_leading_whitespace_length(line: &str) -> usize {
    line.chars().take_while(|c| c.is_whitespace()).count()
}

/// Classify a YAML line as key-bearing or indent-only
///
/// This function analyzes a line to determine if it contains a key token,
/// which is essential for proper scope transition handling.
///
/// # Arguments
///
/// * `line` - The YAML line to classify
///
/// # Returns
///
/// The `LineClassification` for the line
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::scope::classify_line_type;
///
/// // Key-bearing lines
/// assert!(matches!(classify_line_type("key: value"), armor::parsers::yaml::scope::LineClassification::KeyBearing));
/// assert!(matches!(classify_line_type("  nested:"), armor::parsers::yaml::scope::LineClassification::KeyBearing));
/// assert!(matches!(classify_line_type("- item"), armor::parsers::yaml::scope::LineClassification::KeyBearing));
///
/// // Indent-only lines (no key token)
/// assert!(matches!(classify_line_type("  some value"), armor::parsers::yaml::scope::LineClassification::IndentOnly));
/// assert!(matches!(classify_line_type("    more text"), armor::parsers::yaml::scope::LineClassification::IndentOnly));
///
/// // Empty lines
/// assert!(matches!(classify_line_type(""), armor::parsers::yaml::scope::LineClassification::Empty));
/// assert!(matches!(classify_line_type("    "), armor::parsers::yaml::scope::LineClassification::Empty));
/// ```
pub fn classify_line_type(line: &str) -> LineClassification {
    let trimmed = line.trim();

    // Empty or whitespace-only lines
    if trimmed.is_empty() {
        return LineClassification::Empty;
    }

    // Check if line has a key token by using existing extract_key_context
    // This handles all YAML key patterns including:
    // - Simple keys: "key: value"
    // - Parent keys: "key:"
    // - Sequence items with keys: "- key: value"
    // - Nested keys with proper indentation
    if extract_key_context(line).is_some() {
        LineClassification::KeyBearing
    } else {
        LineClassification::IndentOnly
    }
}

/// Check if a line contains key tokens
///
/// This is a convenience function that returns true if the line is key-bearing.
///
/// # Arguments
///
/// * `line` - The YAML line to check
///
/// # Returns
///
/// `true` if the line contains key tokens, `false` otherwise
///
/// # Examples
///
/// ```
/// use armor::parsers::yaml::scope::has_key_token;
///
/// assert!(has_key_token("key: value"));
/// assert!(has_key_token("  nested: value"));
/// assert!(!has_key_token("  some text"));
/// assert!(!has_key_token(""));
/// ```
pub fn has_key_token(line: &str) -> bool {
    classify_line_type(line).is_key_bearing()
}

/// Line classification for YAML parsing
///
/// This enum categorizes lines based on whether they contain key tokens,
/// which is essential for proper scope transition handling.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LineClassification {
    /// Key-bearing line (contains a YAML key token like "key:", "- key:", etc.)
    KeyBearing,

    /// Indent-only line (no key token, only whitespace/content)
    IndentOnly,

    /// Empty or whitespace-only line
    Empty,
}

impl LineClassification {
    /// Check if this line type is key-bearing
    pub fn is_key_bearing(&self) -> bool {
        matches!(self, Self::KeyBearing)
    }

    /// Check if this line type is indent-only
    pub fn is_indent_only(&self) -> bool {
        matches!(self, Self::IndentOnly)
    }

    /// Check if this line type is empty
    pub fn is_empty(&self) -> bool {
        matches!(self, Self::Empty)
    }
}

impl fmt::Display for LineClassification {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::KeyBearing => write!(f, "key-bearing"),
            Self::IndentOnly => write!(f, "indent-only"),
            Self::Empty => write!(f, "empty"),
        }
    }
}

/// Indent transition event recorded during parsing
///
/// This struct captures information about indentation level changes that occur
/// during YAML parsing, whether or not a key token is present.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IndentTransition {
    /// The line number where this transition occurred (1-indexed)
    pub line_number: usize,
    /// The previous indentation level
    pub from_indent: usize,
    /// The new indentation level
    pub to_indent: usize,
    /// Whether this transition occurred on a line with a key token
    pub has_key: bool,
    /// The raw line content (for debugging)
    pub raw_line: String,
    /// Line classification (key-bearing vs indent-only)
    pub line_classification: LineClassification,
    /// Classification of the transition type (enter-scope, exit-scope, same-level)
    pub transition_type: IndentTransitionType,
}

impl IndentTransition {
    /// Create a new indent transition record
    ///
    /// # Arguments
    ///
    /// * `line_number` - The line number where this transition occurred (1-indexed)
    /// * `from_indent` - The previous indentation level
    /// * `to_indent` - The new indentation level
    /// * `has_key` - Whether this transition occurred on a line with a key token
    /// * `raw_line` - The raw line content
    /// * `line_classification` - Classification of the line type
    pub fn new(line_number: usize, from_indent: usize, to_indent: usize, has_key: bool, raw_line: &str, line_classification: LineClassification) -> Self {
        let transition_type = IndentTransitionType::classify(from_indent, to_indent);
        Self {
            line_number,
            from_indent,
            to_indent,
            has_key,
            raw_line: raw_line.to_string(),
            line_classification,
            transition_type,
        }
    }

    /// Check if this is an indent increase
    pub fn is_increase(&self) -> bool {
        self.to_indent > self.from_indent
    }

    /// Check if this is an indent decrease
    pub fn is_decrease(&self) -> bool {
        self.to_indent < self.from_indent
    }

    /// Get the indent change amount (positive for increase, negative for decrease)
    pub fn change_amount(&self) -> isize {
        self.to_indent as isize - self.from_indent as isize
    }

    /// Check if this transition occurred without a key
    pub fn is_without_key(&self) -> bool {
        !self.has_key
    }

    /// Get the line classification for this transition
    pub fn line_classification(&self) -> LineClassification {
        self.line_classification
    }

    /// Get the transition type (enter-scope, exit-scope, same-level)
    pub fn transition_type(&self) -> IndentTransitionType {
        self.transition_type
    }

    /// Check if this is an enter-scope transition
    pub fn is_enter_scope(&self) -> bool {
        self.transition_type.is_enter_scope()
    }

    /// Check if this is an exit-scope transition
    pub fn is_exit_scope(&self) -> bool {
        self.transition_type.is_exit_scope()
    }

    /// Check if this is a same-level transition
    pub fn is_same_level(&self) -> bool {
        self.transition_type.is_same_level()
    }

    /// Map this indent transition to a scope operation description
    ///
    /// Returns a human-readable description of what scope operation
    /// this transition represents.
    pub fn scope_operation(&self) -> &'static str {
        match self.transition_type {
            IndentTransitionType::EnterScope => "enter-scope",
            IndentTransitionType::ExitScope => "exit-scope",
            IndentTransitionType::SameLevel => "stay-in-scope",
        }
    }
}

impl fmt::Display for IndentTransition {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let direction = if self.is_increase() {
            "increase"
        } else if self.is_decrease() {
            "decrease"
        } else {
            "no-change"
        };
        let key_status = if self.has_key { "with-key" } else { "without-key" };
        write!(
            f,
            "IndentTransition(line={}, {}→{}, {}, {}, {}, type={})",
            self.line_number, self.from_indent, self.to_indent, direction, key_status, self.line_classification, self.transition_type
        )
    }
}

// Tests for indent transition tracking
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_indent_transition_type_classify() {
        // Test enter-scope (indent increase)
        let trans_type = IndentTransitionType::classify(0, 2);
        assert_eq!(trans_type, IndentTransitionType::EnterScope);
        assert!(trans_type.is_enter_scope());
        assert!(!trans_type.is_exit_scope());
        assert!(!trans_type.is_same_level());

        // Test exit-scope (indent decrease)
        let trans_type = IndentTransitionType::classify(4, 2);
        assert_eq!(trans_type, IndentTransitionType::ExitScope);
        assert!(!trans_type.is_enter_scope());
        assert!(trans_type.is_exit_scope());
        assert!(!trans_type.is_same_level());

        // Test same-level (no indent change)
        let trans_type = IndentTransitionType::classify(2, 2);
        assert_eq!(trans_type, IndentTransitionType::SameLevel);
        assert!(!trans_type.is_enter_scope());
        assert!(!trans_type.is_exit_scope());
        assert!(trans_type.is_same_level());
    }

    #[test]
    fn test_indent_transition_type_display() {
        assert_eq!(format!("{}", IndentTransitionType::EnterScope), "enter-scope");
        assert_eq!(format!("{}", IndentTransitionType::ExitScope), "exit-scope");
        assert_eq!(format!("{}", IndentTransitionType::SameLevel), "same-level");
    }

    #[test]
    fn test_indent_transition_state() {
        let mut state = IndentTransitionState::new();

        // Initial state
        assert_eq!(state.from_indent, 0);
        assert_eq!(state.to_indent, 0);
        assert_eq!(state.current_transition_type(), IndentTransitionType::SameLevel);
        assert!(!state.last_had_key);

        // Update with enter-scope transition
        state.update(0, 2, true);
        assert_eq!(state.from_indent, 0);
        assert_eq!(state.to_indent, 2);
        assert!(state.is_entering_scope());
        assert!(!state.is_exiting_scope());
        assert!(!state.is_same_level());
        assert!(state.last_had_key);

        // Update with exit-scope transition
        state.update(2, 0, false);
        assert_eq!(state.from_indent, 2);
        assert_eq!(state.to_indent, 0);
        assert!(!state.is_entering_scope());
        assert!(state.is_exiting_scope());
        assert!(!state.is_same_level());
        assert!(!state.last_had_key);

        // Update with same-level transition
        state.update(0, 0, true);
        assert!(state.is_same_level());
    }

    #[test]
    fn test_indent_transition_state_describe() {
        let mut state = IndentTransitionState::new();
        state.update(0, 2, true);

        let description = state.describe();
        assert!(description.contains("from=0"));
        assert!(description.contains("to=2"));
        assert!(description.contains("enter-scope"));
        assert!(description.contains("had_key=true"));

        let display = format!("{}", state);
        assert!(display.contains("IndentTransitionState"));
    }

    #[test]
    fn test_indent_transition_state_reset() {
        let mut state = IndentTransitionState::new();
        state.update(0, 4, true);
        state.reset();

        assert_eq!(state.from_indent, 0);
        assert_eq!(state.to_indent, 0);
        assert_eq!(state.current_transition_type(), IndentTransitionType::SameLevel);
        assert!(!state.last_had_key);
    }

    #[test]
    fn test_indent_transition_with_type() {
        let transition = IndentTransition::new(
            5,
            0,
            2,
            true,
            "key: value",
            LineClassification::KeyBearing,
        );

        assert_eq!(transition.line_number, 5);
        assert_eq!(transition.from_indent, 0);
        assert_eq!(transition.to_indent, 2);
        assert!(transition.has_key);
        assert!(transition.is_increase());
        assert!(!transition.is_decrease());
        assert!(transition.is_enter_scope());
        assert!(!transition.is_exit_scope());
        assert!(transition.is_without_key() == false);
        assert_eq!(transition.transition_type(), IndentTransitionType::EnterScope);
        assert_eq!(transition.scope_operation(), "enter-scope");
    }

    #[test]
    fn test_indent_transition_exit_scope() {
        let transition = IndentTransition::new(
            10,
            4,
            2,
            false,
            "  # comment",
            LineClassification::IndentOnly,
        );

        assert!(transition.is_decrease());
        assert!(!transition.is_increase());
        assert!(transition.is_exit_scope());
        assert!(!transition.is_enter_scope());
        assert!(transition.is_without_key());
        assert_eq!(transition.transition_type(), IndentTransitionType::ExitScope);
        assert_eq!(transition.scope_operation(), "exit-scope");
    }

    #[test]
    fn test_indent_transition_same_level() {
        let transition = IndentTransition::new(
            15,
            2,
            2,
            true,
            "  another: value",
            LineClassification::KeyBearing,
        );

        assert!(!transition.is_increase());
        assert!(!transition.is_decrease());
        assert!(transition.is_same_level());
        assert_eq!(transition.change_amount(), 0);
        assert_eq!(transition.transition_type(), IndentTransitionType::SameLevel);
        assert_eq!(transition.scope_operation(), "stay-in-scope");
    }

    #[test]
    fn test_indent_transition_display() {
        let transition = IndentTransition::new(
            5,
            0,
            2,
            true,
            "key: value",
            LineClassification::KeyBearing,
        );

        let display = format!("{}", transition);
        assert!(display.contains("line=5"));
        assert!(display.contains("0→2"));
        assert!(display.contains("increase"));
        assert!(display.contains("with-key"));
        assert!(display.contains("key-bearing"));
        assert!(display.contains("type=enter-scope"));
    }

    #[test]
    fn test_scope_stack_transition_tracking() {
        let mut stack = ScopeStack::new(2);

        // Record some transitions
        stack.record_indent_transition(1, 2, true, "  key: value");
        stack.record_indent_transition(3, 4, true, "    nested:");
        stack.record_indent_transition(5, 2, false, "  ");

        let transitions = stack.get_indent_transitions();
        assert_eq!(transitions.len(), 3);

        // Check first transition (enter-scope)
        assert!(transitions[0].is_enter_scope());
        assert_eq!(transitions[0].line_number, 1);

        // Check second transition (enter-scope)
        assert!(transitions[1].is_enter_scope());
        assert_eq!(transitions[1].line_number, 3);

        // Check third transition (exit-scope)
        assert!(transitions[2].is_exit_scope());
        assert_eq!(transitions[2].line_number, 5);
    }

    #[test]
    fn test_scope_stack_transition_filtering() {
        let mut stack = ScopeStack::new(2);

        // Record transitions
        stack.record_indent_transition(1, 2, true, "  key: value");
        stack.record_indent_transition(2, 4, true, "    nested:");
        stack.record_indent_transition(3, 2, false, "  ");
        // Note: line 4 (2→2) won't be recorded since indent doesn't change
        stack.record_indent_transition(4, 2, true, "  sibling: value");

        let enter_transitions = stack.get_enter_scope_transitions();
        assert_eq!(enter_transitions.len(), 2); // 0→2, 2→4

        let exit_transitions = stack.get_exit_scope_transitions();
        assert_eq!(exit_transitions.len(), 1); // 4→2

        let same_level = stack.get_same_level_transitions();
        assert_eq!(same_level.len(), 0); // No same-level transitions

        let with_keys = stack.get_transitions_with_keys();
        assert_eq!(with_keys.len(), 2); // Lines 1 and 2 only (line 4 not recorded due to no indent change)

        let without_keys = stack.get_transitions_without_keys();
        assert_eq!(without_keys.len(), 1); // Only line 3
    }

    #[test]
    fn test_scope_stack_transition_counts() {
        let mut stack = ScopeStack::new(2);

        // Record transitions
        stack.record_indent_transition(1, 2, true, "  key: value");
        stack.record_indent_transition(2, 4, true, "    nested:");
        stack.record_indent_transition(3, 2, false, "  ");
        stack.record_indent_transition(4, 2, true, "  sibling:");

        let (enter, exit, same) = stack.get_transition_counts();
        assert_eq!(enter, 2);
        assert_eq!(exit, 1);
        assert_eq!(same, 0);
    }

    #[test]
    fn test_scope_stack_clear_transitions() {
        let mut stack = ScopeStack::new(2);

        stack.record_indent_transition(1, 2, true, "  key: value");
        stack.record_indent_transition(2, 4, true, "    nested:");

        assert_eq!(stack.get_indent_transitions().len(), 2);

        stack.clear_indent_transitions();

        assert_eq!(stack.get_indent_transitions().len(), 0);
        assert_eq!(stack.get_last_indent(), 0);
    }

    #[test]
    fn test_exit_one_level_nested_scopes() {
        let mut stack = ScopeStack::new(2);

        // Auto-create root scope by adding a key
        assert!(stack.add_key("root_key", 0).is_ok());
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);

        // Enter parent scope
        stack.enter_scope(2, 1, Some("parent".to_string()));
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);

        // Enter child scope
        stack.enter_scope(4, 2, Some("child".to_string()));
        assert_eq!(stack.depth(), 3);
        assert_eq!(stack.current_indent(), 4);

        // Exit from child to parent
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "parent");

        // Exit from parent to root
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);
        assert_eq!(stack.get_scope_path(), "");
    }

    #[test]
    fn test_exit_one_level_at_root() {
        let mut stack = ScopeStack::new(2);

        // Auto-create root scope
        assert!(stack.add_key("root_key", 0).is_ok());
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);

        // Attempting to exit from root should return false
        let exited = stack.exit_one_level();
        assert!(!exited);
        assert_eq!(stack.depth(), 1); // Should still be at root
        assert_eq!(stack.current_indent(), 0);
    }

    #[test]
    fn test_exit_one_level_single_level_nesting() {
        let mut stack = ScopeStack::new(2);

        // Auto-create root scope
        assert!(stack.add_key("root_key", 0).is_ok());
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);

        // Enter a single level of nesting
        stack.enter_scope(2, 1, Some("level1".to_string()));
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);

        // Exit back to root
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);

        // Attempting to exit again should return false
        let exited = stack.exit_one_level();
        assert!(!exited);
        assert_eq!(stack.depth(), 1);
    }

    #[test]
    fn test_exit_one_level_deeply_nested() {
        let mut stack = ScopeStack::new(2);

        // Auto-create root scope
        assert!(stack.add_key("root_key", 0).is_ok());

        // Create deeply nested structure (5 levels)
        stack.enter_scope(2, 1, Some("level1".to_string()));
        stack.enter_scope(4, 2, Some("level2".to_string()));
        stack.enter_scope(6, 3, Some("level3".to_string()));
        stack.enter_scope(8, 4, Some("level4".to_string()));
        stack.enter_scope(10, 5, Some("level5".to_string()));

        assert_eq!(stack.depth(), 6); // Root + 5 nested levels
        assert_eq!(stack.current_indent(), 10);
        assert_eq!(stack.get_scope_path(), "level1.level2.level3.level4.level5");

        // Exit one level at a time and verify
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 5);
        assert_eq!(stack.current_indent(), 8);
        assert_eq!(stack.get_scope_path(), "level1.level2.level3.level4");

        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 4);
        assert_eq!(stack.current_indent(), 6);
        assert_eq!(stack.get_scope_path(), "level1.level2.level3");

        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 3);
        assert_eq!(stack.current_indent(), 4);
        assert_eq!(stack.get_scope_path(), "level1.level2");

        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "level1");

        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);
        assert_eq!(stack.get_scope_path(), "");

        // One more attempt should fail
        let exited = stack.exit_one_level();
        assert!(!exited);
        assert_eq!(stack.depth(), 1);
    }

    #[test]
    fn test_exit_one_level_with_sequence_scope() {
        let mut stack = ScopeStack::new(2);

        // Auto-create root scope
        assert!(stack.add_key("root_key", 0).is_ok());

        // Enter a mapping scope
        stack.enter_scope(2, 1, Some("mapping".to_string()));

        // Enter a sequence scope
        stack.enter_sequence_scope(4, 2);

        assert_eq!(stack.depth(), 3); // Root, mapping, sequence
        assert_eq!(stack.current_indent(), 4);

        // Exit from sequence scope
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);

        // Exit from mapping scope
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.depth(), 1);
        assert_eq!(stack.current_indent(), 0);
    }

    #[test]
    fn test_exit_one_level_mixed_indent_sizes() {
        let mut stack = ScopeStack::new(2);

        // Auto-create root scope
        assert!(stack.add_key("root_key", 0).is_ok());

        // Test with various indent sizes (not just multiples of 2)
        stack.enter_scope(3, 1, Some("indent3".to_string()));
        assert_eq!(stack.current_indent(), 3);

        stack.enter_scope(7, 2, Some("indent7".to_string()));
        assert_eq!(stack.current_indent(), 7);

        // Exit from indent7 to indent3
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.current_indent(), 3);

        // Exit from indent3 to root (indent 0)
        let exited = stack.exit_one_level();
        assert!(exited);
        assert_eq!(stack.current_indent(), 0);
    }

    #[test]
    fn test_scope_type_display() {
        assert_eq!(format!("{}", ScopeType::Block), "block");
        assert_eq!(format!("{}", ScopeType::FlowMapping), "flow-mapping");
        assert_eq!(format!("{}", ScopeType::FlowSequence), "flow-sequence");
        assert_eq!(format!("{}", ScopeType::BlockSequence), "block-sequence");
        assert_eq!(format!("{}", ScopeType::Root), "root");
    }

    #[test]
    fn test_scope_info_new() {
        let info = ScopeInfo::new(ScopeType::Block, 2);
        assert_eq!(info.scope_type(), ScopeType::Block);
        assert_eq!(info.scope_depth(), 2);
    }

    #[test]
    fn test_scope_info_root() {
        let root = ScopeInfo::root();
        assert_eq!(root.scope_type(), ScopeType::Root);
        assert_eq!(root.scope_depth(), 0);
        assert!(root.is_root());
    }

    #[test]
    fn test_scope_info_block() {
        let block = ScopeInfo::block(1);
        assert_eq!(block.scope_type(), ScopeType::Block);
        assert_eq!(block.scope_depth(), 1);
        assert!(block.is_block());
        assert!(!block.is_flow());
        assert!(block.is_mapping());
        assert!(!block.is_sequence());
    }

    #[test]
    fn test_scope_info_flow_mapping() {
        let flow = ScopeInfo::flow_mapping(1);
        assert_eq!(flow.scope_type(), ScopeType::FlowMapping);
        assert_eq!(flow.scope_depth(), 1);
        assert!(flow.is_flow());
        assert!(!flow.is_block());
        assert!(flow.is_mapping());
        assert!(!flow.is_sequence());
    }

    #[test]
    fn test_scope_info_block_sequence() {
        let seq = ScopeInfo::block_sequence(2);
        assert_eq!(seq.scope_type(), ScopeType::BlockSequence);
        assert_eq!(seq.scope_depth(), 2);
        assert!(seq.is_block());
        assert!(!seq.is_flow());
        assert!(seq.is_sequence());
        assert!(!seq.is_mapping());
    }

    #[test]
    fn test_scope_info_flow_sequence() {
        // This would require adding a flow_sequence constructor
        // For now, test via new()
        let seq = ScopeInfo::new(ScopeType::FlowSequence, 2);
        assert_eq!(seq.scope_type(), ScopeType::FlowSequence);
        assert_eq!(seq.scope_depth(), 2);
        assert!(seq.is_flow());
        assert!(!seq.is_block());
        assert!(seq.is_sequence());
        assert!(!seq.is_mapping());
    }

    #[test]
    fn test_scope_info_setters() {
        let mut info = ScopeInfo::new(ScopeType::Block, 1);

        // Test set_scope_type
        info.set_scope_type(ScopeType::FlowMapping);
        assert_eq!(info.scope_type(), ScopeType::FlowMapping);
        assert!(info.is_flow());

        // Test set_scope_depth
        info.set_scope_depth(3);
        assert_eq!(info.scope_depth(), 3);
    }

    #[test]
    fn test_scope_info_increment_depth() {
        let mut info = ScopeInfo::new(ScopeType::Block, 1);
        info.increment_depth();
        assert_eq!(info.scope_depth(), 2);
        info.increment_depth();
        assert_eq!(info.scope_depth(), 3);
    }

    #[test]
    fn test_scope_info_decrement_depth() {
        let mut info = ScopeInfo::new(ScopeType::Block, 3);
        info.decrement_depth();
        assert_eq!(info.scope_depth(), 2);
        info.decrement_depth();
        assert_eq!(info.scope_depth(), 1);

        // Decrementing at 0 should stay at 0
        info.set_scope_depth(0);
        info.decrement_depth();
        assert_eq!(info.scope_depth(), 0);
    }

    #[test]
    fn test_scope_info_equality() {
        let info1 = ScopeInfo::new(ScopeType::Block, 2);
        let info2 = ScopeInfo::new(ScopeType::Block, 2);
        let info3 = ScopeInfo::new(ScopeType::FlowMapping, 2);
        let info4 = ScopeInfo::new(ScopeType::Block, 3);

        // Same type and depth should be equal
        assert_eq!(info1, info2);

        // Different type should not be equal
        assert_ne!(info1, info3);

        // Different depth should not be equal
        assert_ne!(info1, info4);
    }

    #[test]
    fn test_scope_info_clone() {
        let info1 = ScopeInfo::new(ScopeType::Block, 2);
        let info2 = info1.clone();
        assert_eq!(info1, info2);
    }

    #[test]
    fn test_scope_info_describe() {
        let info = ScopeInfo::new(ScopeType::Block, 2);
        let description = info.describe();
        assert!(description.contains("type=block"));
        assert!(description.contains("depth=2"));
    }

    #[test]
    fn test_scope_info_display() {
        let info = ScopeInfo::new(ScopeType::FlowMapping, 3);
        let display = format!("{}", info);
        assert!(display.contains("ScopeInfo"));
        assert!(display.contains("type=flow-mapping"));
        assert!(display.contains("depth=3"));
    }

    #[test]
    fn test_scope_info_depth_tracking() {
        // Simulate entering nested scopes
        let mut info = ScopeInfo::root();
        assert_eq!(info.scope_depth(), 0);

        info.increment_depth();
        assert_eq!(info.scope_depth(), 1);

        info.increment_depth();
        assert_eq!(info.scope_depth(), 2);

        // Simulate exiting scopes
        info.decrement_depth();
        assert_eq!(info.scope_depth(), 1);

        info.decrement_depth();
        assert_eq!(info.scope_depth(), 0);
    }

    #[test]
    fn test_scope_info_type_transitions() {
        let mut info = ScopeInfo::block(1);
        assert!(info.is_block());
        assert!(!info.is_flow());

        // Transition from block to flow
        info.set_scope_type(ScopeType::FlowMapping);
        assert!(info.is_flow());
        assert!(!info.is_block());
    }

    #[test]
    fn test_scope_info_all_scope_types() {
        let types = vec![
            (ScopeType::Root, ScopeInfo::root()),
            (ScopeType::Block, ScopeInfo::block(1)),
            (ScopeType::FlowMapping, ScopeInfo::flow_mapping(1)),
            (ScopeType::BlockSequence, ScopeInfo::block_sequence(1)),
        ];

        for (scope_type, info) in types {
            assert_eq!(info.scope_type(), scope_type);
        }
    }
}
