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
    scopes: Vec<Scope>,
    /// Base indentation size (usually 2 or 4 spaces)
    base_indent: usize,
    /// Sequence item counter for generating unique IDs
    sequence_item_counter: usize,
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
            scopes: vec![Scope::new(0, 0, None)], // Root scope
            base_indent,
            sequence_item_counter: 0,
        }
    }

    /// Get the current scope (top of stack)
    ///
    /// # Panics
    ///
    /// Panics if the scope stack is empty (should never happen in normal operation)
    pub fn current_scope(&mut self) -> &mut Scope {
        self.scopes.last_mut().expect("Scope stack should never be empty")
    }

    /// Get the current scope as an immutable reference
    ///
    /// # Panics
    ///
    /// Panics if the scope stack is empty
    pub fn current_scope_ref(&self) -> &Scope {
        self.scopes.last().expect("Scope stack should never be empty")
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
        // Check if we already have a scope at this level
        if let Some(existing) = self.get_scope_at_level(indent_level) {
            // We're re-entering a scope level - clear and reuse
            // This handles sibling mappings correctly
            let mut fresh_scope = Scope::new(indent_level, line, parent_key);
            fresh_scope.is_flow_style = existing.is_flow_style;

            // Remove all scopes deeper than this level
            self.scopes.retain(|s| s.indent_level <= indent_level);
            self.scopes.push(fresh_scope);
        } else {
            // Create new scope
            let new_scope = Scope::new(indent_level, line, parent_key);
            self.scopes.push(new_scope);
        }
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
        // Remove all scopes deeper than target
        self.scopes.retain(|s| s.indent_level <= target_indent);

        // Ensure we have a scope at the target level
        if !self.scopes.iter().any(|s| s.indent_level == target_indent) {
            // This shouldn't happen in valid YAML, but handle gracefully
            let fallback_scope = Scope::new(target_indent, 0, None);
            self.scopes.push(fallback_scope);
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
        let scope_path = self.get_scope_path();

        // Check if key exists before adding
        if self.contains_key(key) {
            let scope = self.current_scope();
            Err(DuplicateKeyError {
                key: key.to_string(),
                scope_path,
                first_line: scope.start_line,
                duplicate_line: line,
            })
        } else {
            let scope = self.current_scope();
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
    /// The indentation level of the current scope
    pub fn current_indent(&self) -> usize {
        self.scopes.last()
            .map(|scope| scope.indent_level)
            .unwrap_or(0)
    }

    /// Get the number of active scopes in the stack
    pub fn depth(&self) -> usize {
        self.scopes.len()
    }

    /// Clear all scopes and reset to root
    ///
    /// This resets the scope stack to its initial state with only the root scope.
    pub fn reset(&mut self) {
        self.scopes = vec![Scope::new(0, 0, None)];
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
        // Remove all scopes deeper than this level
        self.scopes.retain(|s| s.indent_level < indent_level);

        // Check if there's already a scope at this level that's in a sequence context
        let needs_new_scope = self.scopes.last()
            .map(|scope| scope.indent_level != indent_level || !scope.in_sequence_context)
            .unwrap_or(true);

        if needs_new_scope {
            // Create a new scope for this sequence item
            let mut new_scope = Scope::new(indent_level, line, None);
            new_scope.in_sequence_context = true;
            self.sequence_item_counter += 1;
            new_scope.sequence_item_id = Some(self.sequence_item_counter);
            self.scopes.push(new_scope);
        } else {
            // Reset the existing scope for a new sequence item
            if let Some(scope) = self.scopes.last_mut() {
                scope.keys.clear();
                scope.start_line = line;
                self.sequence_item_counter += 1;
                scope.sequence_item_id = Some(self.sequence_item_counter);
            }
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

    // Classify based on what comes after the colon
    let context = if after_colon.trim().is_empty() {
        KeyContext::ParentMapping { key: key.to_string() }
    } else {
        KeyContext::InlineScalar {
            key: key.to_string(),
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_scope_creation() {
        let scope = Scope::new(2, 5, Some("parent".to_string()));
        assert_eq!(scope.indent_level, 2);
        assert_eq!(scope.start_line, 5);
        assert_eq!(scope.parent_key, Some("parent".to_string()));
        assert_eq!(scope.key_count(), 0);
        assert!(!scope.is_flow_style);
    }

    #[test]
    fn test_scope_add_key() {
        let mut scope = Scope::new(0, 1, None);

        assert_eq!(scope.add_key("first"), false);
        assert_eq!(scope.add_key("second"), false);
        assert_eq!(scope.key_count(), 2);

        // Duplicate detection
        assert_eq!(scope.add_key("first"), true);
        assert_eq!(scope.key_count(), 2); // Not added
    }

    #[test]
    fn test_scope_contains_key() {
        let mut scope = Scope::new(0, 1, None);

        assert!(!scope.contains_key("test"));
        scope.add_key("test");
        assert!(scope.contains_key("test"));
    }

    #[test]
    fn test_scope_stack_creation() {
        let stack = ScopeStack::new(2);
        assert_eq!(stack.base_indent(), 2);
        assert_eq!(stack.depth(), 1); // Root scope
        assert_eq!(stack.current_indent(), 0);
    }

    #[test]
    fn test_scope_stack_enter_exit() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("parent".to_string()));
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "parent");

        stack.enter_scope(4, 2, Some("child".to_string()));
        assert_eq!(stack.depth(), 3);
        assert_eq!(stack.get_scope_path(), "parent.child");

        stack.exit_to_scope(2);
        assert_eq!(stack.depth(), 2);
        assert_eq!(stack.current_indent(), 2);
        assert_eq!(stack.get_scope_path(), "parent");
    }

    #[test]
    fn test_scope_stack_add_key() {
        let mut stack = ScopeStack::new(2);

        // Add to root scope
        assert!(stack.add_key("root_key", 1).is_ok());
        assert!(stack.contains_key("root_key"));

        // Duplicate detection in same scope
        assert!(stack.add_key("root_key", 2).is_err());

        // Different key succeeds
        assert!(stack.add_key("another_key", 3).is_ok());
    }

    #[test]
    fn test_scope_stack_add_key_in_nested_scope() {
        let mut stack = ScopeStack::new(2);

        // Add to root
        stack.add_key("host", 1).unwrap();

        // Enter nested scope
        stack.enter_scope(2, 2, Some("services".to_string()));

        // Same key name is OK in different scope
        assert!(stack.add_key("host", 3).is_ok());
        assert!(stack.contains_key("host"));
    }

    #[test]
    fn test_scope_stack_reset() {
        let mut stack = ScopeStack::new(2);

        stack.enter_scope(2, 1, Some("parent".to_string()));
        stack.add_key("key", 2).unwrap();

        assert_eq!(stack.depth(), 2);

        stack.reset();

        assert_eq!(stack.depth(), 1);
        assert!(!stack.contains_key("key"));
    }

    #[test]
    fn test_duplicate_key_error() {
        let error = DuplicateKeyError::new(
            "host".to_string(),
            "services.web".to_string(),
            5,
            10,
        );

        assert_eq!(error.key, "host");
        assert_eq!(error.scope_path, "services.web");
        assert_eq!(error.first_line, 5);
        assert_eq!(error.duplicate_line, 10);

        let msg = error.message();
        assert!(msg.contains("duplicate key"));
        assert!(msg.contains("host"));
        assert!(msg.contains("services.web"));
        assert!(msg.contains("Line 10"));
        assert!(msg.contains("Line 5"));
    }

    #[test]
    fn test_key_context_inline_scalar() {
        let ctx = extract_key_context("host: localhost").unwrap();
        assert!(matches!(ctx, KeyContext::InlineScalar { .. }));
        assert_eq!(ctx.key_name(), "host");
        assert!(ctx.is_inline_scalar());
        assert!(!ctx.is_parent_key());
    }

    #[test]
    fn test_key_context_parent_mapping() {
        let ctx = extract_key_context("services:").unwrap();
        assert!(matches!(ctx, KeyContext::ParentMapping { .. }));
        assert_eq!(ctx.key_name(), "services");
        assert!(!ctx.is_inline_scalar());
        assert!(ctx.is_parent_key());
    }

    #[test]
    fn test_extract_key_context_invalid() {
        // No colon
        assert!(extract_key_context("no key here").is_none());

        // Empty key
        assert!(extract_key_context(": value").is_none());

        // Flow style (contains braces)
        assert!(extract_key_context("{key: value}").is_none());
    }

    #[test]
    fn test_get_leading_whitespace_length() {
        assert_eq!(get_leading_whitespace_length("key: value"), 0);
        assert_eq!(get_leading_whitespace_length("  key: value"), 2);
        assert_eq!(get_leading_whitespace_length("    key: value"), 4);
        assert_eq!(get_leading_whitespace_length("\tkey: value"), 1); // Tab counts as 1
    }

    #[test]
    fn test_scope_stack_sibling_mappings() {
        let mut stack = ScopeStack::new(2);

        // First sibling
        stack.enter_scope(2, 1, Some("web".to_string()));
        stack.add_key("host", 2).unwrap();
        stack.exit_to_scope(0);

        // Second sibling at same level
        stack.enter_scope(2, 5, Some("database".to_string()));
        // Same key should be OK (different scope)
        assert!(stack.add_key("host", 6).is_ok());
    }

    #[test]
    fn test_scope_stack_display() {
        let stack = ScopeStack::new(2);
        let display = format!("{}", stack);
        assert!(display.contains("ScopeStack"));
        assert!(display.contains("depth="));
        assert!(display.contains("base_indent="));
    }

    #[test]
    fn test_scope_display() {
        let scope = Scope::new(2, 5, Some("parent".to_string()));
        let display = format!("{}", scope);
        assert!(display.contains("Scope"));
        assert!(display.contains("indent=2"));
        assert!(display.contains("parent=parent"));
    }

    #[test]
    fn test_enter_sequence_scope() {
        let mut stack = ScopeStack::new(2);

        // Enter first sequence item
        stack.enter_sequence_scope(2, 1);
        assert!(stack.in_sequence_context());
        assert_eq!(stack.current_indent(), 2);

        // Add a key to the first sequence item
        stack.add_key("name", 2).unwrap();

        // Enter second sequence item (same indent level)
        stack.enter_sequence_scope(2, 3);
        assert!(stack.in_sequence_context());

        // Same key should be OK in different sequence item
        stack.add_key("name", 4).unwrap();

        // But duplicate in same sequence item should fail
        assert!(stack.add_key("name", 5).is_err());
    }

    #[test]
    fn test_sequence_scope_with_unique_ids() {
        let mut stack = ScopeStack::new(2);

        // Enter first sequence item
        stack.enter_sequence_scope(2, 1);
        let first_id = stack.current_scope_ref().sequence_item_id;
        assert_eq!(first_id, Some(1));

        // Enter second sequence item
        stack.enter_sequence_scope(2, 3);
        let second_id = stack.current_scope_ref().sequence_item_id;
        assert_eq!(second_id, Some(2));

        // IDs should be different
        assert_ne!(first_id, second_id);
    }

    #[test]
    fn test_sequence_scope_clears_keys() {
        let mut stack = ScopeStack::new(2);

        // Enter first sequence item and add keys
        stack.enter_sequence_scope(2, 1);
        stack.add_key("first", 2).unwrap();
        stack.add_key("second", 3).unwrap();
        assert_eq!(stack.current_scope_ref().key_count(), 2);

        // Enter second sequence item (should clear keys from first)
        stack.enter_sequence_scope(2, 5);
        assert_eq!(stack.current_scope_ref().key_count(), 0);

        // Keys from first item should not be in second
        stack.add_key("first", 6).unwrap();
        stack.add_key("second", 7).unwrap();
    }

    #[test]
    fn test_mixed_regular_and_sequence_scopes() {
        let mut stack = ScopeStack::new(2);

        // Enter regular scope
        stack.enter_scope(2, 1, Some("items".to_string()));
        stack.add_key("item1", 2).unwrap();

        // Enter sequence scope within regular scope
        stack.enter_sequence_scope(4, 3);
        stack.add_key("name", 4).unwrap();

        // Exit sequence scope back to regular
        stack.exit_to_scope(2);
        assert!(!stack.in_sequence_context());

        // Add another key to regular scope
        stack.add_key("item2", 5).unwrap();
    }
}
