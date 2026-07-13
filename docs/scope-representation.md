# Scope Representation Data Structures

## Overview

The scope representation module (`src/parsers/yaml/scope.rs`) provides hierarchical scope tracking for YAML parsing, enabling accurate duplicate key detection within proper scope contexts.

## Architecture

### Core Components

1. **Scope** - Represents a mapping context at a specific nesting level
2. **ScopeStack** - Hierarchical stack managing active scopes during parsing
3. **KeyContext** - Classification of key types (inline scalar, parent mapping, parent sequence)
4. **DuplicateKeyError** - Error type for duplicate key detection

### Scope Structure

```rust
pub struct Scope {
    pub indent_level: usize,        // Leading spaces
    pub keys: HashSet<String>,      // Keys in this scope
    pub start_line: usize,           // Where scope started
    pub parent_key: Option<String>, // Parent key name
    pub is_flow_style: bool,         // Flow-style mapping
}
```

### ScopeStack Operations

- `enter_scope(indent, line, parent_key)` - Enter nested scope
- `exit_to_scope(target_indent)` - Exit to parent scope
- `add_key(key, line)` - Add key to current scope (detects duplicates)
- `get_scope_path()` - Get dot-separated path (e.g., "services.web")

## Usage Example

```rust
use armor::parsers::yaml::scope::{ScopeStack, DuplicateKeyError};

let mut stack = ScopeStack::new(2); // 2-space indentation

// Enter scope when encountering parent mapping
stack.enter_scope(2, 1, Some("services".to_string()));

// Add keys to current scope
stack.add_key("web", 2)?;
stack.add_key("database", 3)?;

// Detect duplicate in same scope
let result = stack.add_key("web", 4);
assert!(result.is_err()); // DuplicateKeyError
```

## Key Benefits

1. **Accurate Duplicate Detection** - Only detects true duplicates within the same scope
2. **Nested Scope Support** - Handles deeply nested YAML structures correctly
3. **Scope Path Tracking** - Provides human-readable scope paths for error messages
4. **Flow Style Support** - Handles both block and flow-style YAML mappings

## Integration Points

The scope module integrates with:
- `line_parser` - For extracting key context from YAML lines
- `syntax_validator` - For validation during parsing
- `parser` - For use during YAML parsing operations

## Testing

The module includes 16 comprehensive tests covering:
- Scope creation and key management
- Stack enter/exit operations
- Duplicate detection in same vs. different scopes
- Sibling mapping handling
- Key context extraction
- Error handling and display
