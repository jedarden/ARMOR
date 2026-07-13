# Scope-Based Key Tracking Design for ARMOR YAML Parser

## Executive Summary

This document outlines the design for a scope-based key tracking system that properly handles YAML mapping keys within their correct scope context. The current implementation has limitations in handling nested structures and scope transitions. This design provides a hierarchical scope system with proper key isolation and comprehensive duplicate detection.

## Current Implementation Analysis

### Existing Structure (from `syntax_detector.rs`)

```rust
struct StructureState {
    context_stack: Vec<StructureContext>,  // Tracks structure types only
    current_keys: HashSet<String>,          // Single-level key tracking
    prev_indent: usize,                     // Previous indentation level
}
```

### Current Limitations

1. **No Scope Hierarchy**: Only tracks keys at one level at a time
2. **Aggressive Key Clearing**: Clears all keys when indentation decreases
3. **No Scope Preservation**: Loses parent scope keys when entering nested structures
4. **Context Type Only**: `context_stack` only tracks structure types, not keys
5. **False Positives**: May report duplicates when keys appear in different scopes

### Current Behavior

```yaml
# Current implementation behavior:
services:
  web:
    host: localhost      # Key "host" added to current_keys
    port: 8080           # Key "port" added to current_keys
  database:
    host: db.example.com  # ERROR: "host" reported as duplicate!
    port: 5432            # ERROR: "port" reported as duplicate!
```

The current implementation incorrectly reports duplicates because it doesn't distinguish between different mapping scopes (`services.web` vs `services.database`).

## Proposed Design

### Core Concept: Hierarchical Scope Stack

A **scope** represents a mapping context where keys are defined and must be unique. Each scope is associated with a specific indentation level and maintains its own set of keys.

#### Scope Definition

```rust
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
```

#### Scope Stack

```rust
/// Hierarchical stack of active scopes
#[derive(Debug, Clone)]
struct ScopeStack {
    /// Stack of active scopes (top = current scope)
    scopes: Vec<Scope>,
    /// Base indentation size (usually 2 or 4 spaces)
    base_indent: usize,
}

impl ScopeStack {
    /// Get the scope for a specific indentation level
    fn get_scope_at_level(&self, indent_level: usize) -> Option<&Scope>;
    
    /// Get the current scope (top of stack)
    fn current_scope(&self) -> Option<&Scope>;
    
    /// Enter a new scope (when indent increases)
    fn enter_scope(&mut self, indent_level: usize, line: usize, parent_key: Option<String>);
    
    /// Exit to parent scope (when indent decreases)
    fn exit_to_scope(&mut self, indent_level: usize);
    
    /// Check if a key already exists in current scope
    fn contains_key(&self, key: &str) -> bool;
    
    /// Add a key to current scope
    fn add_key(&mut self, key: String, line: usize) -> Result<(), DuplicateKeyError>;
}
```

### Scope Transition Algorithm

#### 1. Entering a Scope (Indentation Increase)

```yaml
services:
  web:              # ← ENTER scope for "services.web"
    host: localhost
    port: 8080
```

**Algorithm:**
```rust
fn on_indent_increase(indent_level: usize, line: usize, parent_key: Option<String>) {
    // Calculate scope level from indentation
    let scope_level = indent_level / base_indent;
    
    // Check if we already have a scope at this level
    if let Some(existing_scope) = get_scope_at_level(indent_level) {
        // Re-enter existing scope (e.g., sibling mapping)
        // Don't clear keys - they persist within the scope
        set_current_scope(existing_scope);
    } else {
        // Create new scope for this indentation level
        let new_scope = Scope {
            indent_level,
            keys: HashSet::new(),
            start_line: line,
            parent_key,
            is_flow_style: false,
        };
        push_scope(new_scope);
    }
}
```

#### 2. Exiting a Scope (Indentation Decrease)

```yaml
services:
  web:
    host: localhost
    port: 8080
database:           # ← EXIT scope for "services.web"
  host: db.example.com
```

**Algorithm:**
```rust
fn on_indent_decrease(new_indent_level: usize) {
    // Pop scopes until we find the target scope
    while current_scope().indent_level > new_indent_level {
        pop_scope();
    }
    
    // Set current to the scope matching new_indent_level
    set_current_scope(get_scope_at_level(new_indent_level));
}
```

**Key Preservation**: When exiting a scope, we **preserve** the scope and its keys. If we re-enter that scope later (e.g., a sibling mapping), the keys are still available for duplicate checking.

### Key Detection and Classification

#### Key Context Types

```rust
/// Classification of how a key is used in the YAML document
#[derive(Debug, Clone, PartialEq, Eq)]
enum KeyContext {
    /// Key with inline scalar value: "key: value"
    InlineScalar {
        key: String,
        value: String,
    },
    /// Parent key with nested mapping: "key:\n  nested: value"
    ParentMapping {
        key: String,
    },
    /// Key with nested sequence: "key:\n  - item1"
    ParentSequence {
        key: String,
    },
    /// Key in flow mapping: "{key: value, key2: value2}"
    FlowMapping {
        key: String,
    },
    /// Key with flow sequence value: "key: [val1, val2]"
    FlowSequenceValue {
        key: String,
    },
    /// Key in explicit mapping: "? key\n: value"
    ExplicitMapping {
        key: String,
    },
}
```

#### Key Extraction Algorithm

```rust
fn extract_key_context(line: &str, line_num: usize) -> Option<KeyContext> {
    let trimmed = line.trim();
    
    // Skip if not a key-value line
    if !trimmed.contains(':') {
        return None;
    }
    
    let colon_pos = trimmed.find(':').unwrap();
    let key_part = &trimmed[..colon_pos];
    let after_colon = &trimmed[colon_pos+1..];
    
    // Skip if key is empty or contains special characters
    let key = key_part.trim();
    if key.is_empty() || key.contains(['?', '&', '*', '!', '[', ']', '{', '}']) {
        return None;
    }
    
    // Classify based on what comes after the colon
    let context = if after_colon.trim().is_empty() {
        // "key:\n  ..." → Parent key with nested content
        KeyContext::ParentMapping { key: key.to_string() }
    } else if after_colon.contains('[') {
        // "key: [val1, val2]" → Key with flow sequence
        KeyContext::FlowSequenceValue { key: key.to_string() }
    } else if after_colon.contains('{') {
        // "key: {nested: value}" → Key with flow mapping
        KeyContext::FlowMapping { key: key.to_string() }
    } else {
        // "key: value" → Inline scalar value
        KeyContext::InlineScalar {
            key: key.to_string(),
            value: after_colon.trim().to_string(),
        }
    };
    
    Some(context)
}
```

### Duplicate Detection Algorithm

```rust
fn detect_duplicate_keys(
    scope_stack: &mut ScopeStack,
    line: &str,
    line_num: usize,
) -> Vec<ValidationError> {
    let mut errors = Vec::new();
    
    // Skip flow-style contexts (handled separately)
    if is_in_flow_context(line) {
        return errors;
    }
    
    let indent = get_leading_whitespace_length(line);
    
    // Handle scope transitions
    match indent.cmp(&current_indent_level()) {
        Ordering::Greater => {
            // Entering deeper scope
            if let Some(KeyContext::ParentMapping { key }) = extract_key_context(line, line_num) {
                scope_stack.enter_scope(indent, line_num, Some(key));
            }
            return errors; // Parent keys don't create duplicate checks
        }
        Ordering::Less => {
            // Exiting to parent scope
            scope_stack.exit_to_scope(indent);
        }
        Ordering::Equal => {
            // Same scope - continue checking for duplicates
        }
    }
    
    // Extract and check key
    if let Some(key_context) = extract_key_context(line, line_num) {
        match key_context {
            KeyContext::ParentMapping { .. } => {
                // Parent keys create new scope, handled above
            }
            KeyContext::InlineScalar { key, .. } |
            KeyContext::FlowMapping { key, .. } |
            KeyContext::FlowSequenceValue { key, .. } => {
                // Check for duplicate in current scope
                if scope_stack.contains_key(&key) {
                    errors.push(ValidationError::new(
                        format!("key_{}", key),
                        format!("duplicate key '{}' in mapping scope", key)
                    ).with_line(line_num).with_code(ErrorCode::KEY_DUPLICATE));
                } else {
                    scope_stack.add_key(key, line_num);
                }
            }
            _ => {}
        }
    }
    
    errors
}
```

## Handling Complex Scenarios

### Scenario 1: Deeply Nested Mappings

```yaml
level1:
  level2:
    level3:
      key1: value1
      key2: value2
    key3: value3
  key4: value4
key5: value5
```

**Scope Stack Evolution:**

1. Line 1 (`level1:`): Enter scope level 0 with parent_key="level1"
2. Line 2 (`  level2:`): Enter scope level 2 with parent_key="level2"
3. Line 3 (`    level3:`): Enter scope level 4 with parent_key="level3"
4. Line 4 (`      key1:`): Add "key1" to scope at level 4
5. Line 5 (`      key2:`): Add "key2" to scope at level 4
6. Line 6 (`    key3:`): Exit to scope level 2, add "key3"
7. Line 7 (`  key4:`): Exit to scope level 0, add "key4"
8. Line 8 (`key5:`): Exit to root scope, add "key5"

**Result**: Each "key" is tracked in its appropriate scope, no false duplicates.

### Scenario 2: Sibling Mappings with Same Keys

```yaml
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
```

**Scope Stack Evolution:**

1. Line 1 (`services:`): Root scope (level 0)
2. Line 2 (`  web:`): Enter scope level 2, parent_key="web"
3. Line 3 (`    host:`): Add "host" to scope level 4
4. Line 4 (`    port:`): Add "port" to scope level 4
5. Line 5 (`  database:`): Exit to scope level 2, **preserve scope level 4 keys**
6. Line 6 (`    host:`): **Enter existing scope level 4**, check "host" against **fresh keys**
7. Line 7 (`    port:`): Add "port" to scope level 4

**Critical Decision**: When exiting scope level 4 (going to level 2), do we:
- **Option A**: Clear the scope level 4 keys (current behavior)
- **Option B**: Preserve the scope level 4 keys for potential re-entry

**Correct Behavior is Option A**: Each sibling mapping is a **new scope**. The "host" in `services.web` and the "host" in `services.database` are in **different scopes**, so they should both be allowed.

**Revised Algorithm**:
```rust
fn on_indent_decrease(new_indent_level: usize) {
    // Clear all scopes deeper than new_indent_level
    scopes.retain(|scope| scope.indent_level <= new_indent_level);
    set_current_scope(new_indent_level);
}

fn on_indent_increase(indent_level: usize, line: usize, parent_key: Option<String>) {
    // Always create a FRESH scope when entering a deeper level
    let new_scope = Scope {
        indent_level,
        keys: HashSet::new(),  // Fresh keys
        start_line: line,
        parent_key,
        is_flow_style: false,
    };
    push_scope(new_scope);
}
```

### Scenario 3: Mixed Scalar and Mapping Values

```yaml
simple_key: scalar_value
mapping_key:
  nested1: value1
  nested2: value2
sequence_key:
  - item1
  - item2
flow_key: {inline: value, another: value2}
```

**Handling**:
- `simple_key`: Add "simple_key" to root scope
- `mapping_key`: Add "mapping_key" to root scope, **then** enter nested scope
- `sequence_key`: Add "sequence_key" to root scope, enter sequence scope
- `flow_key`: Add "flow_key" to root scope, parse flow mapping separately

### Scenario 4: Flow-Style Mappings

```yaml
inline: {key1: value1, key2: value2}
multiline: {
  key3: value3,
  key4: value4
}
```

**Handling**:
- Detect flow context by checking for `{` character
- Extract keys using specialized flow parser
- Track duplicates within the flow mapping scope only
- Don't mix flow-style keys with block-style keys

## Migration Path from Current Implementation

### Phase 1: Add Scope Stack Alongside Current Implementation

**Changes to `StructureState`:**

```rust
struct StructureState {
    // NEW: Hierarchical scope tracking
    scope_stack: ScopeStack,
    
    // LEGACY: Keep for backward compatibility during migration
    context_stack: Vec<StructureContext>,
    current_keys: HashSet<String>,
    prev_indent: usize,
    
    // Migration flag
    use_legacy_tracking: bool,
}
```

**Dual Implementation:**
```rust
fn detect_duplicate_key_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
    if self.use_legacy_tracking {
        // Use current implementation
        self.legacy_detect_duplicates(line, line_num, errors);
    } else {
        // Use new scope-based implementation
        self.scope_based_detect_duplicates(line, line_num, errors);
    }
}
```

### Phase 2: Feature Flag for New Implementation

**Configuration Update:**

```rust
pub struct DetectorConfig {
    // ... existing config ...
    
    /// Enable scope-based key tracking (Phase 2 feature)
    pub use_scope_based_key_tracking: bool,
}

impl Default for DetectorConfig {
    fn default() -> Self {
        Self {
            // ... existing defaults ...
            use_scope_based_key_tracking: false,  // Disabled by default during migration
        }
    }
}
```

### Phase 3: Validation and Testing

**Test Coverage:**

```rust
#[cfg(test)]
mod scope_based_tests {
    #[test]
    fn test_sibling_mappings_same_keys() {
        let yaml = "
services:
  web:
    host: localhost
  database:
    host: db.example.com
";
        let mut detector = SyntaxDetector::with_config(DetectorConfig {
            use_scope_based_key_tracking: true,
            ..Default::default()
        });
        let errors = detector.detect_errors(yaml);
        assert!(errors.is_empty(), "Should not report duplicate across siblings");
    }
    
    #[test]
    fn test_actual_duplicate_in_same_scope() {
        let yaml = "
config:
  host: localhost
  host: duplicate
";
        let mut detector = SyntaxDetector::with_config(DetectorConfig {
            use_scope_based_key_tracking: true,
            ..Default::default()
        });
        let errors = detector.detect_errors(yaml);
        assert!(errors.iter().any(|e| e.code == ErrorCode::KEY_DUPLICATE));
    }
    
    #[test]
    fn test_deeply_nested_mappings() {
        let yaml = "
level1:
  level2:
    level3:
      key: value
  key: value2
key: value3
";
        let mut detector = SyntaxDetector::with_config(DetectorConfig {
            use_scope_based_key_tracking: true,
            ..Default::default()
        });
        let errors = detector.detect_errors(yaml);
        assert!(errors.is_empty(), "Should handle deep nesting correctly");
    }
}
```

### Phase 4: Gradual Rollout

1. **Week 1-2**: Feature flag = false (current behavior)
2. **Week 3-4**: Feature flag = true in test environments only
3. **Week 5-6**: Monitor error reports, tune scope detection
4. **Week 7-8**: Feature flag = true in production
5. **Week 9+**: Remove legacy code, set feature flag to always true

### Phase 5: Remove Legacy Implementation

```rust
// Final StructureState (post-migration)
struct StructureState {
    scope_stack: ScopeStack,
    // No legacy fields
}
```

## Data Structure Summary

### Complete Scope Tracking Structure

```rust
/// Hierarchical key tracking with proper scope isolation
pub struct ScopeBasedKeyTracker {
    /// Stack of active scopes
    scopes: Vec<Scope>,
    /// Base indentation size
    base_indent: usize,
    /// Current flow-style context (if any)
    flow_context: Option<FlowContext>,
}

/// A single scope in the hierarchy
pub struct Scope {
    /// Indentation level for this scope
    pub indent_level: usize,
    /// Keys defined in this scope
    pub keys: HashSet<String>,
    /// Line where this scope started
    pub start_line: usize,
    /// Parent key name
    pub parent_key: Option<String>,
    /// Whether this is a flow-style mapping
    pub is_flow_style: bool,
    /// Keys seen in flow context (if is_flow_style)
    pub flow_keys: HashSet<String>,
}

/// Flow-style context tracking
pub struct FlowContext {
    /// Brackets/braces depth
    depth: usize,
    /// Keys seen in current flow mapping
    keys: HashSet<String>,
    /// Opening line number
    start_line: usize,
}
```

## Algorithm Complexity

### Time Complexity

- **Scope lookup**: O(1) using indent_level as index
- **Key duplicate check**: O(1) using HashSet
- **Scope transition**: O(k) where k = number of scopes to pop (typically small)
- **Overall line processing**: O(1) amortized per line

### Space Complexity

- **Per scope**: O(m) where m = number of keys in that scope
- **Total space**: O(n * m_avg) where n = number of active scopes
- **Typical case**: O(depth * keys_per_scope) where depth ≤ 10 for most YAML files

## Error Messages

### Duplicate Key Error

```
Line 5: duplicate key 'host' in mapping scope 'services.web'
  First occurrence: Line 3
  Current occurrence: Line 5
  Scope path: services.web
```

### Enhanced Error Context

```rust
pub struct DuplicateKeyError {
    pub key: String,
    pub scope_path: String,        // e.g., "services.web"
    pub first_line: usize,
    pub duplicate_line: usize,
    pub code: ErrorCode,
}

impl fmt::Display for DuplicateKeyError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Line {}: duplicate key '{}' in scope '{}'\n  First defined at: Line {}\n  Current location: Line {}",
            self.duplicate_line, self.key, self.scope_path, self.first_line, self.duplicate_line
        )
    }
}
```

## Configuration Options

### DetectorConfig Extensions

```rust
pub struct DetectorConfig {
    // ... existing fields ...
    
    /// Enable scope-based key tracking
    pub use_scope_based_key_tracking: bool,
    
    /// Base indentation size (2 or 4)
    pub base_indent_size: usize,
    
    /// How strict duplicate detection should be
    pub duplicate_key_strictness: DuplicateKeyStrictness,
}

pub enum DuplicateKeyStrictness {
    /// Only check within the same immediate scope
    SameScopeOnly,
    /// Check across entire scope hierarchy
    FullHierarchy,
    /// Allow duplicates if types differ
    AllowDifferentTypes,
}
```

## Conclusion

This design provides a robust, hierarchical scope-based key tracking system that:

1. **Properly isolates keys by scope**: Each mapping maintains its own key set
2. **Handles complex nesting**: Deep hierarchies are tracked correctly
3. **Supports mixed content**: Scalar values, mappings, and sequences coexist
4. **Provides clear migration path**: Feature flag and phased rollout
5. **Maintains performance**: O(1) operations per line for typical cases
6. **Enhances error messages**: Clear scope path in duplicate reports

The implementation will be rolled out in phases to ensure backward compatibility and thorough testing before making it the default behavior.

## Next Steps

1. Implement `Scope` and `ScopeStack` structs
2. Add `ScopeBasedKeyTracker` to `StructureState`
3. Implement scope transition logic
4. Add comprehensive test coverage
5. Create migration plan with feature flags
6. Monitor and validate in test environments
7. Gradual rollout to production
8. Remove legacy implementation
