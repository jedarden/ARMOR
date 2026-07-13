# YAML Scope Tracking Mechanism Documentation

## Overview

The NEEDLE YAML parser implements a sophisticated scope tracking system to accurately detect duplicate keys within their proper hierarchical contexts. This documentation explains the current implementation, how scope transitions are detected, and where indent-only detection should be added.

## Core Data Structures

### `Scope` - Single Mapping Context

Located in: `src/parsers/yaml/scope.rs`

```rust
pub struct Scope {
    pub indent_level: usize,           // Indentation level (spaces)
    pub keys: HashSet<String>,         // Keys defined within this scope
    pub start_line: usize,             // Line where scope started (1-indexed)
    pub parent_key: Option<String>,    // Key that created this scope
    pub is_flow_style: bool,           // Whether in {key: value} flow style
    pub in_sequence_context: bool,     // Whether within sequence
    pub sequence_item_id: Option<usize>, // Unique ID for sequence items
}
```

**Purpose**: Represents a single mapping context at a specific nesting level. Each scope maintains its own set of keys, independent of parent or sibling scopes.

**Key Operations**:
- `add_key()` - Adds a key, returns `true` if duplicate
- `contains_key()` - Checks if key exists in this scope

### `ScopeStack` - Hierarchical Scope Management

```rust
pub struct ScopeStack {
    scopes: Vec<Scope>,                // Stack of active scopes (top = current)
    base_indent: usize,               // Base indentation size (usually 2 or 4)
    sequence_item_counter: usize,      // For generating unique sequence IDs
    indent_transitions: Vec<IndentTransition>, // History of indent transitions
    last_indent: usize,               // Last recorded indent level
}
```

**Purpose**: Manages the hierarchical nature of YAML scopes during parsing, maintaining a stack where the top element is the current scope.

**Key Methods**:
- `current_scope()` - Get mutable reference to current scope
- `enter_scope()` - Enter nested scope (indent increase)
- `exit_to_scope()` - Exit to parent scope (indent decrease)
- `add_key()` - Add key to current scope with duplicate detection
- `get_scope_path()` - Get dot-separated path (e.g., "services.web.database")
- `record_indent_transition()` - Track all indent changes
- `process_indent_transition_without_key()` - Handle indent changes without key tokens

### `KeyContext` - Key Token Classification

```rust
pub enum KeyContext {
    InlineScalar { key: String, value: String },  // "key: value"
    ParentMapping { key: String },                // "key:" (creates new scope)
    ParentSequence { key: String },               // "key:\n  - item"
}
```

**Purpose**: Classifies keys based on whether they create new scopes (parent keys) or have inline values.

**Key Methods**:
- `is_parent_key()` - Returns true if key creates a new scope
- `is_inline_scalar()` - Returns true if key has inline value

### `IndentTransition` - Indent Change Tracking

```rust
pub struct IndentTransition {
    pub line_number: usize,       // Where transition occurred
    pub from_indent: usize,       // Previous indent level
    pub to_indent: usize,         // New indent level
    pub has_key: bool,           // Whether line had a key token
    pub raw_line: String,         // Raw line content (for debugging)
}
```

**Purpose**: Records all indentation changes during parsing, whether or not they occur on lines with key tokens. This enables analysis of indent transitions on blank lines, comments, and other non-key lines.

**Key Methods**:
- `is_increase()` - Check if indent increased
- `is_decrease()` - Check if indent decreased
- `is_without_key()` - Check if transition occurred without a key token

## Current Scope Transition Detection

### Detection Trigger: Line-by-Line Processing

Scope transitions are detected during line-by-line parsing in `src/parsers/yaml/parser.rs`:

```rust
for (line_num, line) in content.lines().enumerate() {
    let indent = calculate_indentation(line);
    let trimmed = line.trim();
    
    // Track ALL indent changes for detection purposes
    let indent_changed = indent != scope_stack.get_last_indent();
    if indent_changed {
        let has_key = extract_key_context(line).is_some();
        scope_stack.record_indent_transition(line_num_1index, indent, has_key, line);
    }
    
    // Handle scope transitions based on indentation changes
    match indent.cmp(&scope_stack.current_indent()) {
        Ordering::Greater => { /* Enter scope */ }
        Ordering::Less => { /* Exit scope */ }
        Ordering::Equal => { /* Same scope */ }
    }
}
```

### Three Transition Scenarios

#### 1. Indent Increase → Enter Scope

**Current Logic** (lines 388-436 in parser.rs):

```rust
Ordering::Greater => {
    if let Some(ctx) = extract_key_context(line) {
        if ctx.is_parent_key() {
            // Add parent key to current scope
            scope_stack.add_key(ctx.key_name(), line_num_1index);
            
            // Enter new scope for nested content
            scope_stack.enter_scope(
                indent + scope_stack.base_indent(),
                line_num_1index,
                Some(ctx.key_name().to_string())
            );
        } else {
            // Inline scalar at deeper indent - no scope entry
            scope_stack.add_key(ctx.key_name(), line_num_1index);
        }
    } else {
        // No key context - skip scope entry
    }
}
```

**Key Dependency**: **Key tokens are REQUIRED**. Indent increases without a parent key do NOT trigger scope entry.

**Example**:
```yaml
parent:          # Line 1: Parent key → enters scope
  child: value   # Line 2: Indent increase + parent key → enters scope
```

#### 2. Indent Decrease → Exit Scope

**Current Logic** (lines 437-479 in parser.rs):

```rust
Ordering::Less => {
    scope_stack.exit_to_scope(indent);
    
    // After exiting, check if this line has a key
    if let Some(ctx) = extract_key_context(line) {
        if ctx.is_parent_key() {
            // This is a new parent key at this level
            scope_stack.add_key(ctx.key_name(), line_num_1index);
            scope_stack.enter_scope(
                indent + scope_stack.base_indent(),
                line_num_1index,
                Some(ctx.key_name().to_string())
            );
        }
    }
}
```

**Key Dependency**: **Key tokens are OPTIONAL for exit**. Scope exit occurs on indent decrease regardless of key presence.

**Example**:
```yaml
parent:          # Line 1: Parent key
  child: value   # Line 2: Nested
sibling:         # Line 3: Indent decrease → exits scope
```

#### 3. Same Indent → Check for Sibling Keys

**Current Logic** (lines 480-500 in parser.rs):

```rust
Ordering::Equal => {
    if let Some(ctx) = extract_key_context(line) {
        if ctx.is_parent_key() {
            // Sibling parent key - exit and re-enter
            scope_stack.exit_to_scope(indent);
            scope_stack.add_key(ctx.key_name(), line_num_1index);
            scope_stack.enter_scope(
                indent + scope_stack.base_indent(),
                line_num_1index,
                Some(ctx.key_name().to_string())
            );
        } else {
            // Inline scalar - just add to current scope
            scope_stack.add_key(ctx.key_name(), line_num_1index);
        }
    }
}
```

**Key Dependency**: **Key tokens are REQUIRED**. Same indent with no key is a no-op.

## Role of Key Tokens in Scope Changes

### Critical Role

**Key tokens are the PRIMARY TRIGGER for scope entry**. The parser uses `extract_key_context()` to determine if a line contains a mapping key, and only enters a new scope when:

1. Indent increases AND
2. Line contains a parent mapping key (`ParentMapping` variant)

### Key Context Extraction

Located in: `src/parsers/yaml/scope.rs` (lines 1045-1118)

```rust
pub fn extract_key_context(line: &str) -> Option<KeyContext> {
    let trimmed = line.trim();
    
    // Find colon position
    let colon_pos = trimmed.find(':')?;
    let key_part = &trimmed[..colon_pos];
    let after_colon = &trimmed[colon_pos + 1..];
    
    // Skip if key is empty or contains invalid characters
    let key = key_part.trim();
    if key.is_empty() { return None; }
    if key.contains('{') || key.contains('}') || key.contains('[') || key.contains(']') {
        return None;
    }
    
    // Strip sequence dash from key if present
    let actual_key = if key.starts_with("- ") {
        key[2..].trim()
    } else {
        key
    };
    
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
```

**What It Detects**:
- Mapping keys with colons (`:`)
- Distinguishes parent mappings (no value after colon) from inline scalars
- Strips sequence dash prefix (`- `)
- Rejects flow style mappings (contains `{`, `[`, etc.)

**What It Ignores**:
- Comment lines (starts with `#`)
- Document markers (`---`, `...`)
- Directives (`%YAML`)
- Anchors (`&`), aliases (`*`), tags (`!`)
- Sequence items without keys (lone `-`)

### Key Token Limitations

**Blank lines are ignored** (lines 377-383 in parser.rs):

```rust
if trimmed.is_empty() || trimmed.starts_with('#') {
    // Process indent transitions without keys (e.g., scope exit on blank line)
    if indent_changed && !trimmed.starts_with('#') {
        scope_stack.process_indent_transition_without_key(line_num_1index, indent);
    }
    continue;
}
```

**Current Behavior**:
- Blank lines DO trigger `record_indent_transition()` for tracking
- Blank lines DO call `process_indent_transition_without_key()` for exit
- **Blank lines DO NOT trigger scope entry** even if indent increases

**Comment lines are fully skipped**:
- No indent transition processing
- No scope changes of any kind

## Current Indent Handling Logic

### Indent Calculation

Located in: `src/parsers/yaml/line_parser.rs` (lines 732-763)

```rust
pub fn calculate_indentation(line: &str) -> usize {
    IndentationInfo::from_line(line).total_level
}

pub struct IndentationInfo {
    pub leading_spaces: usize,
    pub leading_tabs: usize,
    pub total_level: usize,
}
```

**Behavior**:
- Counts all leading whitespace characters (spaces + tabs)
- Does NOT distinguish between spaces and tabs for basic tracking
- Returns total count as indent level

### Indent Transition Recording

Located in: `src/parsers/yaml/scope.rs` (lines 707-759)

```rust
pub fn record_indent_transition(&mut self, line_number: usize, new_indent: usize, has_key: bool, raw_line: &str) {
    // Only record if the indent actually changed
    if new_indent != self.last_indent {
        let transition = IndentTransition::new(
            line_number,
            self.last_indent,
            new_indent,
            has_key,
            raw_line,
        );
        self.indent_transitions.push(transition);
        self.last_indent = new_indent;
    }
}
```

**Purpose**: Records ALL indent changes during parsing, creating a complete history of indentation transitions whether or not they occur on lines with key tokens.

**Key Points**:
- Records changes on ANY line type (blank, comment, key, etc.)
- Tracks whether each transition had a key token
- Stores raw line content for debugging
- Maintains `last_indent` state across the entire parse

### Indent-Only Transition Processing

Located in: `src/parsers/yaml/scope.rs` (lines 828-894)

```rust
pub fn process_indent_transition_without_key(&mut self, line_number: usize, new_indent: usize) -> bool {
    let current_indent = self.current_indent();
    
    match new_indent.cmp(&current_indent) {
        Ordering::Greater => {
            // Indent increased - but without a key, this is unusual
            // We don't enter a new scope without a parent key
            // Just record the transition for tracking purposes
            false
        }
        Ordering::Less => {
            // Indent decreased - exit to parent scope
            // This is valid even without a key
            self.exit_to_scope(new_indent);
            true
        }
        Ordering::Equal => {
            // Same indent - no scope change needed
            false
        }
    }
}
```

**Current Limitation**: **Only processes exits, not entries**. Indent increases without keys are logged but don't trigger scope entry.

## Where Indent-Only Detection Should Be Added

### Current Gap

The parser has the infrastructure to detect and process indent-only transitions (via `record_indent_transition()` and `process_indent_transition_without_key()`), but it only uses this for scope **exit**, not scope **entry**.

### Implementation Location

**File**: `src/parsers/yaml/parser.rs`
**Function**: `parse_str()` or `detect_duplicate_keys_with_scope()`
**Lines**: ~388-436 (indent increase handling)

### Proposed Enhancement

Add logic to handle indent increases on blank lines:

```rust
Ordering::Greater => {
    if let Some(ctx) = extract_key_context(line) {
        if ctx.is_parent_key() {
            // EXISTING: Parent key → enter scope
            scope_stack.add_key(ctx.key_name(), line_num_1index);
            scope_stack.enter_scope(
                indent + scope_stack.base_indent(),
                line_num_1index,
                Some(ctx.key_name().to_string())
            );
        } else {
            // EXISTING: Inline scalar → no scope entry
            scope_stack.add_key(ctx.key_name(), line_num_1index);
        }
    } else {
        // NEW: Indent increased but no key context
        // Check if this is a blank line with significant indent increase
        if trimmed.is_empty() {
            // Enter scope for blank line indent increase
            // Next content line will populate this scope
            scope_stack.enter_scope(
                indent,
                line_num_1index,
                None  // No parent key yet
            );
        } else {
            // Non-key, non-blank line - skip scope entry
        }
    }
}
```

### Use Cases for Indent-Only Entry

1. **Blank lines before nested content**:
   ```yaml
   parent:
   
     child: value   # Blank line + indent increase should enter scope
   ```

2. **Comment blocks before nested content**:
   ```yaml
   parent:
     # Comment block
   
     child: value   # Comment indent + blank line should enter scope
   ```

3. **Multi-line nested structures**:
   ```yaml
   services:
   
   web:
     host: localhost
   
   database:
     host: db.example.com
   ```

### Testing Strategy

Test files demonstrate the gap:
- `test_blank_line_yaml.rs` - Tests blank line handling
- `test_sequence_scope.rs` - Tests sequence scope detection
- `examples/scope_key_tracking_demo.rs` - Demonstrates scope tracking

**Test Cases to Add**:
1. Blank line with indent increase → should enter scope
2. Comment line with indent increase → should NOT enter scope (current behavior is correct)
3. Blank line then indent decrease → should exit scope (already works)
4. Multiple blank lines at varying indents → should track properly

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        YAML Parser                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────┐                                              │
│  │ Line-by-Line   │                                              │
│  │ Processing     │                                              │
│  └───────┬────────┘                                              │
│          │                                                       │
│          ▼                                                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Indent Change Detection                      │   │
│  │  calculate_indentation() → compare with current_indent   │   │
│  └───────┬──────────────────────────────────────────────────┘   │
│          │                                                       │
│          ▼                                                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Key Context Extraction                       │   │
│  │  extract_key_context() → Some(KeyContext) or None        │   │
│  └───────┬──────────────────────────────────────────────────┘   │
│          │                                                       │
│          ▼                                                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │           Indent Transition Recording                     │   │
│  │  record_indent_transition(line, indent, has_key, raw)    │   │
│  └───────┬──────────────────────────────────────────────────┘   │
│          │                                                       │
│          ▼                                                       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Scope Stack Management                       │   │
│  │  ┌────────────────────────────────────────────────────┐ │   │
│  │  │ ScopeStack { scopes: Vec<Scope> }                   │ │   │
│  │  │                                                       │ │   │
│  │  │ enter_scope()    → push new Scope                   │ │   │
│  │  │ exit_to_scope()  → pop to target indent             │ │   │
│  │  │ add_key()        → add to current scope              │ │   │
│  │  └────────────────────────────────────────────────────┘ │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Duplicate Key Detection                       │   │
│  │  check if key exists in current_scope.keys                │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Flow Diagram: Scope Transitions

```
                    Line Processing
                          │
                          ▼
              ┌─────────────────────┐
              │  Calculate Indent   │
              └──────────┬──────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │  Indent Changed?     │
              └──────────┬──────────┘
                    │/│
           ┌────────┴────────┐
           │                 │
          Yes               No
           │                 │
           ▼                 │
┌─────────────────────┐     │
│  Extract Key Context│     │
└──────────┬──────────┘     │
           │                │
   ┌───────┴────────┐      │
   │                │      │
Parent Key       No Key
   │                │      │
   ▼                ▼      │
┌─────────┐   ┌────────┐  │
│ Compare │   │ Record │  │
│ Indents │   │ Only   │  │
└────┬────┘   │ Trans  │  │
     │        └───┬────┘  │
     │            │       │
     ▼            ▼       │
  ┌─────────────────┐     │
  │  Indent Change   │     │
  └────────┬─────────┘     │
           │
     ┌─────┼─────┬─────┐
     │     │     │     │
  Increase Equal Decrease
     │     │     │     │
     ▼     │     ▼     │
┌──────┐  │  ┌──────┐ │
│Enter │  │  │Exit  │ │
│Scope │  │  │Scope │ │
└───┬──┘  │  └───┬──┘ │
    │     │      │    │
    └─────┴──────┴────┘
           │
           ▼
   ┌─────────────┐
   │ Add Key to  │
   │Current Scope│
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ Check for   │
   │ Duplicates  │
   └──────┬──────┘
          │
          ▼
   ┌─────────────┐
   │ Next Line   │
   └─────────────┘
```

## Summary

### Current State

1. **Scope transitions are key-token dependent** - Indent increases only trigger scope entry when accompanied by a parent mapping key
2. **Indent decreases work independently** - Scope exit occurs on indent decrease regardless of key presence
3. **Blank lines partially processed** - Blank lines trigger indent tracking and scope exit, but NOT scope entry
4. **Comprehensive tracking infrastructure exists** - `record_indent_transition()` and `IndentTransition` provide full history

### Key Token Dependencies

- **Scope Entry**: REQUIRES parent key (`ParentMapping`)
- **Scope Exit**: Does NOT require key
- **Scope Re-entry (sibling)**: REQUIRES parent key
- **Indent Tracking**: Does NOT require key (tracks all changes)

### Where to Add Indent-Only Detection

**Location**: `src/parsers/yaml/parser.rs`, lines ~388-436

**Enhancement**: Add logic to `Ordering::Greater` branch to enter scopes on blank lines with significant indent increases, even without parent keys.

**Impact**: Would enable proper scope tracking for structures with blank lines between parent keys and nested content, improving duplicate key detection accuracy.

## Related Files

- `src/parsers/yaml/scope.rs` - Core scope data structures and management
- `src/parsers/yaml/parser.rs` - Main parsing logic with scope transitions
- `src/parsers/yaml/line_parser.rs` - Line classification and indent calculation
- `src/parsers/yaml/syntax_detector.rs` - YAML syntax detection
- `src/parsers/yaml/syntax_validator.rs` - Validation logic
- `test_blank_line_yaml.rs` - Test cases for blank line handling
- `test_sequence_scope.rs` - Test cases for sequence scope detection
- `examples/scope_key_tracking_demo.rs` - Demonstration of scope tracking

## Version

Documentation created for: NEEDLE Parser (ARMOR project)
Date: 2026-07-13
Related Bead: bf-4vdifj
