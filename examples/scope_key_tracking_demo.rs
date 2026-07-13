//! Scope-Based Key Tracking Demonstration
//!
//! This example demonstrates the hierarchical scope-based key tracking
//! system for YAML parsing, showing how it properly handles nested
//! mappings and avoids false duplicate positives.

use std::collections::{HashMap, HashSet};
use std::fmt;

/// A scope representing a mapping context at a specific nesting level
#[derive(Debug, Clone)]
struct Scope {
    /// Indentation level (number of leading spaces)
    indent_level: usize,
    /// Keys defined within this scope
    keys: HashSet<String>,
    /// Line number where this scope started
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
        // Calculate scope level from indentation
        let scope_level = indent_level / self.base_indent;

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
    fn exit_to_scope(&mut self, target_indent: usize) {
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
    fn contains_key(&self, key: &str) -> bool {
        self.scopes.last()
            .and_then(|scope| Some(scope.contains_key(key)))
            .unwrap_or(false)
    }

    /// Add a key to current scope
    fn add_key(&mut self, key: &str, line: usize) -> Result<(), DuplicateKeyError> {
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

/// Error when a duplicate key is detected
#[derive(Debug)]
struct DuplicateKeyError {
    key: String,
    scope_path: String,
    first_line: usize,
    duplicate_line: usize,
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

/// Key context classification
#[derive(Debug, Clone, PartialEq)]
enum KeyContext {
    /// Key with inline scalar value: "key: value"
    InlineScalar { key: String, value: String },
    /// Parent key with nested mapping: "key:\n  nested: value"
    ParentMapping { key: String },
    /// Key with nested sequence: "key:\n  - item1"
    ParentSequence { key: String },
}

/// Extract key context from a line
fn extract_key_context(line: &str) -> Option<KeyContext> {
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

/// Get leading whitespace length
fn get_leading_whitespace_length(line: &str) -> usize {
    line.chars().take_while(|c| c.is_whitespace()).count()
}

/// Process YAML lines and detect duplicate keys
fn process_yaml(yaml: &str) -> Result<Vec<String>, DuplicateKeyError> {
    let mut scope_stack = ScopeStack::new(2); // Base indent of 2 spaces
    let mut processed_keys = Vec::new();

    for (line_num, line) in yaml.lines().enumerate() {
        let line_num_1index = line_num + 1;
        let trimmed = line.trim();

        // Skip empty lines and comments
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }

        let indent = get_leading_whitespace_length(line);

        // Handle scope transitions
        use std::cmp::Ordering;
        match indent.cmp(&scope_stack.current_indent()) {
            Ordering::Greater => {
                // Entering deeper scope
                if let Some(KeyContext::ParentMapping { ref key }) = extract_key_context(line) {
                    // Add the parent key to current scope first
                    if let Err(e) = scope_stack.add_key(key, line_num_1index) {
                        return Err(e);
                    }
                    // Then enter the deeper scope
                    scope_stack.enter_scope(indent, line_num_1index, Some(key.clone()));
                    processed_keys.push(format!("Line {}: Enter scope '{}'", line_num_1index, key));
                    continue; // Parent keys already processed above
                } else {
                    // Not a parent mapping, just indent increase - enter anonymous scope
                    scope_stack.enter_scope(indent, line_num_1index, None);
                    processed_keys.push(format!("Line {}: Enter scope at indent {}", line_num_1index, indent));
                }
            }
            Ordering::Less => {
                // Exiting to parent scope
                scope_stack.exit_to_scope(indent);
                processed_keys.push(format!("Line {}: Exit to scope at indent {}", line_num_1index, indent));
            }
            Ordering::Equal => {
                // Same scope - continue checking
            }
        }

        // Extract and check key
        if let Some(key_context) = extract_key_context(line) {
            match key_context {
                KeyContext::ParentMapping { .. } => {
                    // Handled in scope transition above
                }
                KeyContext::InlineScalar { key, .. } | KeyContext::ParentSequence { key } => {
                    // Check for duplicate
                    scope_stack.add_key(&key, line_num_1index)?;
                    processed_keys.push(format!("Line {}: Added key '{}'", line_num_1index, key));
                }
            }
        }
    }

    Ok(processed_keys)
}

fn main() {
    println!("=== Scope-Based Key Tracking Demonstration ===\n");

    // Example 1: Sibling mappings with same keys (should be OK)
    println!("--- Example 1: Sibling Mappings ---");
    let yaml1 = "
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
";

    match process_yaml(yaml1) {
        Ok(actions) => {
            println!("✓ No duplicate key errors (expected)");
            for action in actions {
                println!("  {}", action);
            }
        }
        Err(e) => println!("✗ Unexpected error: {}", e),
    }

    println!();

    // Example 2: Actual duplicate in same scope (should fail)
    println!("--- Example 2: Actual Duplicate ---");
    let yaml2 = "
config:
  host: localhost
  host: duplicate
";

    match process_yaml(yaml2) {
        Ok(_) => println!("✗ Should have detected duplicate key"),
        Err(e) => println!("✓ Correctly detected duplicate:\n  {}", e),
    }

    println!();

    // Example 3: Deeply nested mappings
    println!("--- Example 3: Deep Nesting ---");
    let yaml3 = "
level1:
  level2:
    level3:
      key: value1
    key: value2
  key: value3
key: value4
";

    match process_yaml(yaml3) {
        Ok(actions) => {
            println!("✓ No duplicate key errors (expected)");
            for action in actions {
                println!("  {}", action);
            }
        }
        Err(e) => println!("✗ Unexpected error: {}", e),
    }

    println!();

    // Example 4: Mixed scalar and mapping values
    println!("--- Example 4: Mixed Values ---");
    let yaml4 = "
scalar_key: simple_value
mapping_key:
  nested: value
another_scalar: value2
";

    match process_yaml(yaml4) {
        Ok(actions) => {
            println!("✓ No duplicate key errors (expected)");
            for action in actions {
                println!("  {}", action);
            }
        }
        Err(e) => println!("✗ Unexpected error: {}", e),
    }

    println!();
    println!("=== Demonstration Complete ===");
}
