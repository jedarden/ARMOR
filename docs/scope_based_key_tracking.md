# Scope-Based Key Tracking Design

## Overview

This document describes a robust scope-based key tracking structure for YAML parsing, addressing the limitations of the current indentation-based approach. The design provides explicit scope management, proper nesting support, and handles complex YAML structures.

## Current Implementation Limitations

The existing implementation in `/home/coding/ARMOR/src/parsers/yaml/syntax_validator.rs` and `/home/coding/ARMOR/src/parsers/yaml/syntax_detector.rs` has several limitations:

### 1. Indentation-Based Scope Detection

```rust
// Current approach (syntax_validator.rs:415-436)
let level = indent / 2;
while self.keys_at_level.len() <= level {
    self.keys_at_level.push(Vec::new());
}
if self.keys_at_level[level].contains(&key.to_string()) {
    // Duplicate key detected
}
```

**Problems:**
- Assumes 2-space indentation (hardcoded `level = indent / 2`)
- Fragile with different indentation sizes (4-space, tabs)
- No relationship between indentation and actual YAML structure
- Cannot handle flow-style contexts (`{key: value}`)

### 2. Simple Context Clearing

```rust
// Current approach (syntax_detector.rs:660-665)
if indent < self.structure_state.prev_indent {
    self.structure_state.current_keys.clear();
}
```

**Problems:**
- Clears all keys when indentation decreases
- Loses parent context when entering nested structures
- Cannot distinguish between sibling keys and returning to parent scope
- Breaks with complex nesting patterns

### 3. No Scope Identity

**Problems:**
- Scopes are implicit (derived from indentation)
- No way to track scope lineage (parent-child relationships)
- Cannot detect when re-entering a previously exited scope
- No support for mixed scalar/mapping values in sequences

## Design Goals

1. **Explicit scope representation**: Track scopes as first-class objects
2. **Per-scope key tracking**: Each scope maintains its own key set
3. **Scope lifecycle management**: Proper creation, entry, and exit
4. **Complex structure support**: Handle sequences, mappings, and flow-style
5. **Clear migration path**: Incremental adoption from current implementation

## Core Data Structures

### 1. Scope Stack

```rust
/// Represents a single YAML scope (mapping context)
#[derive(Debug, Clone)]
pub struct YamlScope {
    /// Unique identifier for this scope
    pub id: ScopeId,
    
    /// Parent scope ID (if any)
    pub parent_id: Option<ScopeId>,
    
    /// Indentation level of this scope
    pub indent_level: usize,
    
    /// Keys seen in this scope
    pub keys: HashSet<String>,
    
    /// Type of scope (mapping, sequence, flow)
    pub scope_type: ScopeType,
    
    /// Line number where this scope was created
    pub created_at_line: usize,
}

/// Unique identifier for a scope
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct ScopeId(pub usize);

/// Type of YAML scope
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ScopeType {
    /// Block-style mapping (indentation-based)
    BlockMapping,
    
    /// Flow-style mapping ({key: value})
    FlowMapping,
    
    /// Flow-style sequence ([items])
    FlowSequence,
    
    /// Block-style sequence (- items)
    BlockSequence,
}

/// Stack of YAML scopes for tracking key context
#[derive(Debug, Clone)]
pub struct ScopeStack {
    /// All scopes in document order
    scopes: Vec<YamlScope>,
    
    /// Current scope stack (top is active scope)
    stack: Vec<ScopeId>,
    
    /// Next scope ID to assign
    next_id: usize,
    
    /// Configuration for scope behavior
    config: ScopeConfig,
}

/// Configuration for scope tracking
#[derive(Debug, Clone)]
pub struct ScopeConfig {
    /// Whether to track duplicate keys
    pub check_duplicates: bool,
    
    /// Whether to ignore keys in flow-style contexts
    pub ignore_flow_keys: bool,
    
    /// Whether to handle keys in sequence items
    pub track_sequence_keys: bool,
}
```

### 2. Key Tracking Within Scope

```rust
/// Result of checking a key against the current scope
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum KeyCheckResult {
    /// Key is valid (not seen before in this scope)
    Valid,
    
    /// Key is a duplicate
    Duplicate {
        /// Line number of first occurrence
        first_seen_at: usize,
        /// The scope where it was first seen
        scope_id: ScopeId,
    },
    
    /// Not applicable (comment, flow context, etc.)
    NotApplicable,
}

/// Information about a detected key
#[derive(Debug, Clone)]
pub struct KeyInfo {
    /// The key name
    pub key: String,
    
    /// The scope where this key exists
    pub scope_id: ScopeId,
    
    /// Line number where key was found
    pub line_number: usize,
    
    /// Whether this is a parent key (no inline value)
    pub is_parent_key: bool,
    
    /// The value if present inline
    pub inline_value: Option<String>,
}
```

## Algorithm Design

### 1. Scope Lifecycle

#### Creating Scopes

A new scope is created when:
1. A mapping key is detected with greater indentation than current scope
2. A flow-style context is entered (`{` or `[`)
3. A sequence item starts a nested mapping (`- key: value`)

```rust
impl ScopeStack {
    /// Enter a new scope
    pub fn push_scope(&mut self, scope_type: ScopeType, indent: usize, line: usize) -> ScopeId {
        let parent_id = self.stack.last().copied();
        let scope_id = ScopeId(self.next_id);
        self.next_id += 1;
        
        let scope = YamlScope {
            id: scope_id,
            parent_id,
            indent_level: indent,
            keys: HashSet::new(),
            scope_type,
            created_at_line: line,
        };
        
        self.scopes.push(scope);
        self.stack.push(scope_id);
        
        scope_id
    }
    
    /// Exit the current scope
    pub fn pop_scope(&mut self) -> Option<ScopeId> {
        self.stack.pop()
    }
    
    /// Get the current active scope
    pub fn current_scope(&self) -> Option<&YamlScope> {
        self.stack.last()
            .and_then(|id| self.scopes.iter().find(|s| s.id == *id))
    }
}
```

#### Scope Transitions

Scope transitions are determined by:
1. **Indentation changes**: Increase = new scope, decrease = exit scopes
2. **Flow delimiters**: `{` `[` create flow scopes, `}` `]` exit them
3. **Sequence items**: `-` can create sequence scope or nested mapping

```rust
impl ScopeStack {
    /// Update scope stack based on line indentation
    pub fn update_for_indentation(&mut self, indent: usize, line: usize) {
        let current_indent = self.current_scope()
            .map(|s| s.indent_level)
            .unwrap_or(0);
        
        if indent > current_indent {
            // Enter nested scope
            self.push_scope(ScopeType::BlockMapping, indent, line);
        } else if indent < current_indent {
            // Exit scopes until we reach matching indentation
            while let Some(scope_id) = self.stack.last() {
                let scope = self.get_scope(*scope_id);
                if scope.indent_level > indent {
                    self.stack.pop();
                } else {
                    break;
                }
            }
        }
        // Same indentation: stay in current scope
    }
}
```

### 2. Key Checking Algorithm

```rust
impl ScopeStack {
    /// Check if a key is valid in the current scope
    pub fn check_key(&mut self, key_info: &KeyInfo) -> KeyCheckResult {
        let current = match self.current_scope() {
            Some(scope) => scope,
            None => return KeyCheckResult::NotApplicable,
        };
        
        // Skip if configured to ignore flow keys
        if self.config.ignore_flow_keys {
            if matches!(current.scope_type, 
                ScopeType::FlowMapping | ScopeType::FlowSequence) {
                return KeyCheckResult::NotApplicable;
            }
        }
        
        let scope = self.get_scope_mut(current.id);
        
        // Check for duplicate
        if let Some(&first_line) = scope.key_lines.get(&key_info.key) {
            return KeyCheckResult::Duplicate {
                first_seen_at: first_line,
                scope_id: current.id,
            };
        }
        
        // Add key to scope
        scope.keys.insert(key_info.key.clone());
        scope.key_lines.insert(key_info.key.clone(), key_info.line_number);
        
        KeyCheckResult::Valid
    }
}
```

### 3. Handling Complex Structures

#### Sequences with Mappings

```yaml
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
```

Each `-` creates a new sequence scope that can contain nested mappings:

```rust
impl ScopeStack {
    /// Handle sequence item with nested mapping
    pub fn handle_sequence_item(&mut self, indent: usize, line: usize) {
        // Sequence item creates a temporary scope for its contents
        let sequence_scope_id = self.push_scope(
            ScopeType::BlockSequence, 
            indent, 
            line
        );
        
        // The sequence item can contain a mapping key
        // which creates another scope at same or greater indentation
    }
}
```

#### Flow-Style Contexts

```yaml
inline: {key1: value1, key2: value2}
list: [item1, item2, {nested: key}]
```

Flow-style contexts are handled as scopes with special rules:

```rust
impl ScopeStack {
    /// Enter flow-style context
    pub fn enter_flow_context(&mut self, flow_type: ScopeType, indent: usize, line: usize) {
        self.push_scope(flow_type, indent, line);
    }
    
    /// Exit flow-style context
    pub fn exit_flow_context(&mut self) {
        // Flow contexts exit when delimiter is closed
        self.pop_scope();
    }
    
    /// Check if we're in a flow context
    pub fn in_flow_context(&self) -> bool {
        self.current_scope()
            .map(|s| matches!(s.scope_type, 
                ScopeType::FlowMapping | ScopeType::FlowSequence))
            .unwrap_or(false)
    }
}
```

## Integration with Current Implementation

### Migration Path

#### Phase 1: Add Scope Tracking Alongside Current Implementation

```rust
pub struct SyntaxDetector {
    // Existing fields...
    config: DetectorConfig,
    indentation_state: IndentationState,
    delimiter_state: DelimiterState,
    structure_state: StructureState,
    
    // NEW: Add scope tracking
    scope_stack: ScopeStack,
}
```

#### Phase 2: Use Scope Tracking for Key Detection

```rust
fn detect_duplicate_key_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
    // Use scope stack instead of simple HashSet
    let trimmed = line.trim();
    let indent = self.get_leading_whitespace_length(line);
    
    // Update scope stack based on indentation
    self.scope_stack.update_for_indentation(indent, line_num);
    
    // Extract key if present
    if let Some(key_info) = self.extract_key_info(line, line_num) {
        match self.scope_stack.check_key(&key_info) {
            KeyCheckResult::Duplicate { first_seen_at, .. } => {
                errors.push(ValidationError::new(
                    format!("key_{}", key_info.key),
                    format!("duplicate key '{}' (first seen at line {})", 
                            key_info.key, first_seen_at)
                ).with_line(line_num));
            }
            KeyCheckResult::Valid => {
                // Key recorded, continue
            }
            KeyCheckResult::NotApplicable => {
                // Skip (comment, flow context, etc.)
            }
        }
    }
}
```

#### Phase 3: Gradual Replacement

1. Keep both implementations during transition
2. A/B test against existing test suite
3. Gradually remove old indentation-based logic
4. Finalize scope-based implementation as default

### Example: Complex Nested Structure

```yaml
services:                    # Scope 0 (root)
  web:                       # Scope 1 (indent 2)
    host: localhost          # Scope 1
    port: 8080               # Scope 1
    ssl:                     # Scope 1 (parent key)
      enabled: true          # Scope 2 (indent 4)
      cert: /path/to/cert    # Scope 2
  database:                  # Scope 1 (sibling to web)
    host: db.example.com     # Scope 1
    port: 5432               # Scope 1
    credentials:             # Scope 1 (parent key)
      username: admin        # Scope 2 (different from ssl scope)
      password: secret      # Scope 2
```

Scope progression:
```
Line 1: "services:"         -> Push Scope 0 (root, indent 0)
Line 2: "  web:"            -> Push Scope 1 (indent 2)
Line 5: "    ssl:"          -> Push Scope 2 (indent 4)
Line 6: "      enabled:"    -> Scope 2 (same indent, no new scope)
Line 9: "  database:"       -> Pop to Scope 1 (indent decreased to 2)
                               "database" is sibling to "web"
Line 12: "    credentials:" -> Push new Scope 2 (indent 4)
                               This is a DIFFERENT scope from ssl's Scope 2
```

## API Design

### Main Entry Point

```rust
impl ScopeStack {
    /// Create a new scope stack
    pub fn new(config: ScopeConfig) -> Self {
        Self {
            scopes: Vec::new(),
            stack: Vec::new(),
            next_id:0,
            config,
        }
    }
    
    /// Process a YAML line and update scope stack
    pub fn process_line(&mut self, line: &str, line_num: usize) -> Vec<KeyError> {
        let mut errors = Vec::new();
        let indent = calculate_indentation(line);
        let trimmed = line.trim();
        
        // Skip empty lines and comments
        if trimmed.is_empty() || trimmed.starts_with('#') {
            return errors;
        }
        
        // Update scope stack for indentation
        self.update_for_indentation(indent, line_num);
        
        // Handle flow delimiters
        if trimmed.contains('{') || trimmed.contains('[') {
            self.handle_flow_delimiters(trimmed, indent, line_num);
        }
        
        // Check for mapping key
        if let Some(key_info) = self.extract_key_info(line, line_num) {
            if let KeyCheckResult::Duplicate { first_seen_at, .. } = 
                self.check_key(&key_info) {
                errors.push(KeyError {
                    key: key_info.key,
                    line: line_num,
                    first_seen_at,
                });
            }
        }
        
        errors
    }
    
    /// Get the current scope depth (for debugging)
    pub fn depth(&self) -> usize {
        self.stack.len()
    }
    
    /// Get all scopes in document order
    pub fn all_scopes(&self) -> &[YamlScope] {
        &self.scopes
    }
}
```

### Error Reporting

```rust
/// Key error detected during parsing
#[derive(Debug, Clone)]
pub struct KeyError {
    /// The duplicate key name
    pub key: String,
    
    /// Line where duplicate was found
    pub line: usize,
    
    /// Line where key was first seen
    pub first_seen_at: usize,
}

impl fmt::Display for KeyError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f, 
            "duplicate key '{}' at line {} (first seen at line {})",
            self.key, self.line, self.first_seen_at
        )
    }
}
```

## Testing Strategy

### Unit Tests

```rust
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_simple_scope_stack() {
        let config = ScopeConfig::default();
        let mut stack = ScopeStack::new(config);
        
        // Root scope
        stack.push_scope(ScopeType::BlockMapping, 0, 1);
        assert_eq!(stack.depth(), 1);
        
        // Nested scope
        stack.push_scope(ScopeType::BlockMapping, 2, 2);
        assert_eq!(stack.depth(), 2);
        
        // Exit nested scope
        stack.pop_scope();
        assert_eq!(stack.depth(), 1);
    }
    
    #[test]
    fn test_key_tracking_in_scope() {
        let config = ScopeConfig::default();
        let mut stack = ScopeStack::new(config);
        
        stack.push_scope(ScopeType::BlockMapping, 0, 1);
        
        let key1 = KeyInfo {
            key: "name".to_string(),
            scope_id: stack.current_scope().unwrap().id,
            line_number: 2,
            is_parent_key: false,
            inline_value: Some("value".to_string()),
        };
        
        assert!(matches!(
            stack.check_key(&key1),
            KeyCheckResult::Valid
        ));
        
        // Duplicate key
        assert!(matches!(
            stack.check_key(&key1),
            KeyCheckResult::Duplicate { .. }
        ));
    }
    
    #[test]
    fn test_nested_scope_key_isolation() {
        let mut stack = ScopeStack::new(ScopeConfig::default());
        
        // Parent scope
        stack.push_scope(ScopeType::BlockMapping, 0, 1);
        
        let key1 = KeyInfo {
            key: "host".to_string(),
            scope_id: stack.current_scope().unwrap().id,
            line_number: 2,
            is_parent_key: false,
            inline_value: Some("localhost".to_string()),
        };
        
        stack.check_key(&key1);
        
        // Child scope
        stack.push_scope(ScopeType::BlockMapping, 2, 3);
        
        let key2 = KeyInfo {
            key: "host".to_string(),
            scope_id: stack.current_scope().unwrap().id,
            line_number: 4,
            is_parent_key: false,
            inline_value: Some("db.example.com".to_string()),
        };
        
        // Same key name, different scope - should be valid
        assert!(matches!(
            stack.check_key(&key2),
            KeyCheckResult::Valid
        ));
    }
}
```

### Integration Tests

```rust
#[test]
fn test_complex_nested_yaml() {
    let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
    ssl:
      enabled: true
      cert: /path/to/cert.pem
  database:
    host: db.example.com
    port: 5432
    credentials:
      username: admin
      password: secret
"#;
    
    let config = ScopeConfig::default();
    let mut stack = ScopeStack::new(config);
    
    let mut errors = Vec::new();
    for (line_num, line) in yaml.lines().enumerate() {
        let line_num = line_num + 1;
        errors.extend(stack.process_line(line, line_num));
    }
    
    assert!(errors.is_empty(), "No duplicates should be found");
}

#[test]
fn test_duplicate_key_detection() {
    let yaml = r#"
config:
  host: localhost
  host: duplicate
  port: 8080
"#;
    
    let config = ScopeConfig {
        check_duplicates: true,
        ..Default::default()
    };
    let mut stack = ScopeStack::new(config);
    
    let mut errors = Vec::new();
    for (line_num, line) in yaml.lines().enumerate() {
        let line_num = line_num + 1;
        errors.extend(stack.process_line(line, line_num));
    }
    
    assert_eq!(errors.len(), 1);
    assert_eq!(errors[0].key, "host");
    assert_eq!(errors[0].line, 3);
    assert_eq!(errors[0].first_seen_at, 2);
}
```

## Performance Considerations

### Memory Usage

- **Scopes**: O(depth) where depth is maximum nesting level (typically < 10)
- **Keys per scope**: O(keys) where keys is number of unique keys in that scope
- **Total memory**: O(total_keys) for all scopes

### Computational Complexity

- **Scope push/pop**: O(1) amortized
- **Key check**: O(1) average (HashSet lookup)
- **Indentation update**: O(depth) in worst case (popping multiple scopes)
- **Overall parsing**: O(n) where n is number of lines

### Optimization Opportunities

1. **Scope pooling**: Reuse scope objects for common patterns
2. **Key internment**: Use string interning for common key names
3. **Lazy scope creation**: Only create scopes when keys are detected
4. **Incremental validation**: Stop processing after first N errors

## Future Enhancements

### 1. Schema Integration

```rust
impl ScopeStack {
    /// Set schema for a scope
    pub fn set_schema(&mut self, scope_id: ScopeId, schema: Schema) {
        if let Some(scope) = self.get_scope_mut(scope_id) {
            scope.schema = Some(schema);
        }
    }
    
    /// Validate key against schema
    pub fn validate_key_against_schema(&self, key_info: &KeyInfo) -> Vec<ValidationError> {
        let current = match self.current_scope() {
            Some(scope) => scope,
            None => return Vec::new(),
        };
        
        if let Some(schema) = &current.schema {
            schema.validate_key(&key_info.key)
        } else {
            Vec::new()
        }
    }
}
```

### 2. Cross-Scope References

```rust
#[derive(Debug, Clone)]
pub struct ScopeReference {
    pub from_scope: ScopeId,
    pub to_scope: ScopeId,
    pub reference_type: ReferenceType,
}

pub enum ReferenceType {
    Anchor,    // &anchor
    Alias,     // *alias
    Merge,     // << key
}
```

### 3. Visualization and Debugging

```rust
impl ScopeStack {
    /// Generate a tree representation of scopes
    pub fn to_tree(&self) -> String {
        let mut output = String::new();
        for scope in &self.scopes {
            let indent = "  ".repeat(scope.indent_level / 2);
            writeln!(output, "{}Scope {} ({:?})", indent, scope.id.0, scope.scope_type);
            for key in &scope.keys {
                writeln!(output, "  {}  - {}", indent, key);
            }
        }
        output
    }
}
```

## Conclusion

This scope-based key tracking design provides:

1. **Explicit scope management**: Scopes are first-class objects with identity
2. **Proper nesting support**: Parent-child relationships tracked accurately
3. **Complex structure handling**: Sequences, flow-style, and mixed contexts supported
4. **Clear migration path**: Can be incrementally integrated into existing codebase
5. **Extensibility**: Foundation for schema validation, cross-scope references, and more

The design addresses all limitations of the current indentation-based approach while maintaining compatibility with existing YAML parsing infrastructure.
