# Folded Scalar Explicit Indent Test Infrastructure

**Bead:** bf-63gy6
**Purpose:** Provides reusable patterns and infrastructure for folded scalar explicit indent tests

## Overview

This infrastructure simplifies creating comprehensive tests for YAML folded scalars with explicit indent modifiers. Folded scalars (using `>`) treat newlines as spaces, and explicit indent modifiers (like `>2`, `>-3`, `+4`) specify exactly how many spaces of indentation to use.

## Quick Start

### Using the Helper Macros

```rust
// Generate test cases at a single indentation level
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",           // 2-space indentation
    "level1",      // descriptive name
    [">", ">-", ">+"],  // modifiers
    [1, 2, 3, 4, 5],    // indent levels
    "text"         // key prefix
);

// Run the tests with standard assertions
run_folded_scalar_tests!(test_cases);
```

### Using Helper Functions

```rust
// Single test case
let case = create_folded_scalar_test("  ", "my_key", ">-", 2);

// Bulk generate for multiple levels
let cases = generate_folded_scalar_tests_multi_level(
    &["warning", "error", "info"],
    &[">", ">-", ">+"],
    &[1, 2, 3, 4, 5]
);
```

## Pattern Documentation

### Test Case Structure

Each test case is a tuple with three elements:

```rust
(input_line, expected_key_name, expected_line_type)
```

- **input_line**: The complete YAML line to test
  - Format: `<indent><key_name>: <modifier><indent_level>`
  - Example: `"  warning: >-2"`

- **expected_key_name**: The key that should be extracted
  - Example: `"warning"`

- **expected_line_type**: The line type classification
  - Usually `LineType::MappingKey` for scalar declarations

---

### Pattern 4: Basic Indicator Line Assertions

**Purpose:** Test classification of YAML block scalar indicator lines (folded `>` or literal `|`)

**Structure:** `vec!` of input lines with `assert_eq!` assertions for `MappingKey` classification

**Example from Section 12B:**
```rust
// Section 12B: Multiline String Scenarios with Exclamation Marks (line 7892)
fn test_folded_block_scalar_with_exclamation_marks() {
    let test_cases = vec![
        "description: >",               // Basic folded scalar
        "  folded_text: >",              // Indented folded scalar
        "    note: >",                   // Deep indented folded scalar
        "\tmessage: >",                 // Tab-indented folded scalar
        "\tkey_with_exclamation!: >",   // Tab-indented key with ! followed by folded scalar
        "warning: >-",                  // Folded with strip modifier
        "info: >+",                     // Folded with keep modifier
        "text: >-2",                    // Folded with explicit indent
        "content: >2",                   // Folded with explicit indent
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded block scalar indicator should be MappingKey: '{}'",
            line
        );
    }
}
```

**Assertion Pattern:**
- Single `assert_eq!` comparing result to `LineType::MappingKey`
- Descriptive error message includes the failing line
- Tests the indicator line itself (key followed by `>` or `|`)

**Used in Section 12B:**
- Line 7892: Folded scalar indicators with exclamation marks
- Line 7937: Literal scalar indicators with basic modifiers
- Line 10451: Indicator line classification tests (Section 12B.2)

**When to use this pattern:**
- Testing YAML block scalar indicator lines
- Verifying basic line classification for block scalars
- Simple validation without key extraction checks

---

### Pattern 5: Continuation Line Assertion Patterns with Allowed Types

**Purpose:** Test continuation lines of block scalars where multiple line types may be valid

**Structure:** `vec!` of tuples `(line, vec![allowed_types])` with `assert!(expected_types.contains(&result))`

**Example from Section 12B:**
```rust
// Section 12B: Continuation lines with exclamation marks (line 7916)
fn test_folded_block_scalar_with_exclamation_marks() {
    let continuation_lines = vec![
        "  This is folded text with! exclamation marks",
        "    Multiple! exclamations! in! folded! style",
        "\tMore! content! with! bangs!",
        "  Important! message! continues!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Folded scalar continuation with ! should be MappingKey or Unknown: '{}'",
            line
        );
    }
}
```

**Advanced Example with Multiple Allowed Types:**
```rust
// Section 12B: Literal scalar continuation with Tag types allowed (line 7952)
fn test_literal_block_scalar_with_exclamation_marks() {
    let continuation_lines = vec![
        ("  This is literal text with! exclamation marks", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Multiple! exclamations! in! literal! style", vec![LineType::MappingKey, LineType::Unknown]),
        ("\tMore! content! with! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    !Start! Middle! End!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  !important!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        assert!(
            expected_types.contains(&result),
            "Literal scalar continuation with ! should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}
```

**Assertion Pattern:**
- Tuple structure: `(test_line, vec![allowed_types])`
- `assert!(expected_types.contains(&result))` for flexible matching
- Detailed error message shows all allowed types and actual result
- Handles cases where line starting with `!` could be `Tag`, `MappingKey`, or `Unknown`

**When to use this pattern:**
- Continuation lines of block scalars (indented content following indicator)
- Lines with ambiguous classification (multiple valid types)
- Testing exclamation marks at various positions in continuation lines
- When `Tag` type is possible for lines starting with `!`

**Allowed Types:**
- `vec![LineType::MappingKey, LineType::Unknown]` - Most continuation lines
- `vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]` - Lines starting with `!`

**Used in Section 12B:**
- Line 7916: Folded scalar continuation lines (basic pattern)
- Line 7948: Literal scalar continuation lines (with Tag type support)
- Line 7965: Tuple-based tests with comprehensive allowed types
- Line 10708: Folded scalar continuation with exclamation (Section 12B.1)

---

### Modifiers

YAML folded scalars support three modifiers:

| Modifier | Name | Description |
|----------|------|-------------|
| `>` | Plain | Standard folded scalar |
| `>-` | Strip | Remove leading/trailing blank lines |
| `>+` | Keep | Preserve all blank lines |

### Explicit Indent Levels

Each modifier can be combined with an explicit indent level (1-9):

- `>1`, `>2`, `>3`, ..., `>9`
- `>-1`, `>-2`, `>-3`, ..., `>-9`
- `>+1`, `>+2`, `>+3`, ..., `>+9`

The number specifies exactly how many spaces of indentation to expect in the continuation lines.

### Indentation Levels

Tests cover multiple base indentation levels to ensure robustness:

| Level | Indentation | Name |
|-------|-------------|------|
| 1 | `"  "` | level1 |
| 2 | `"    "` | level2 |
| 3 | `"      "` | level3 |
| 4 | `"        "` | level4 |
| Tab | `"\t"` | tab |

Mixed indentation (tab + spaces) is also tested:
- `"\t  "` - tab + 2 spaces
- `"\t    "` - tab + 4 spaces

## Complete Example

Here's a complete test function using the infrastructure:

```rust
#[test]
fn test_folded_scalar_my_custom_tests() {
    // Generate test cases for Level 1 (2-space) indentation
    let test_cases = vec![
        // Plain >n modifiers
        create_folded_scalar_test("  ", "text1", ">", 1),
        create_folded_scalar_test("  ", "text2", ">", 2),
        create_folded_scalar_test("  ", "text3", ">", 3),

        // Strip >-n modifiers
        create_folded_scalar_test("  ", "strip1", ">-", 1),
        create_folded_scalar_test("  ", "strip2", ">-", 2),
        create_folded_scalar_test("  ", "strip3", ">-", 3),

        // Keep >+n modifiers
        create_folded_scalar_test("  ", "keep1", ">+", 1),
        create_folded_scalar_test("  ", "keep2", ">+", 2),
        create_folded_scalar_test("  ", "keep3", ">+", 3),

        // Keys with exclamation marks
        create_folded_scalar_test("  ", "warn!1", ">", 1),
        create_folded_scalar_test("  ", "error!2", ">-", 2),
        create_folded_scalar_test("  ", "info!3", ">+", 3),
    ];

    // Run all test cases with standard assertions
    run_folded_scalar_tests!(test_cases);
}
```

## Pattern for Child Beads

When adding new folded scalar explicit indent tests as child beads, follow this pattern:

### 1. Choose Your Test Scope

Decide what specific aspect you're testing:
- Specific indentation level?
- Specific modifier combination?
- Keys with special characters?
- Edge cases?

### 2. Create Test Function

```rust
#[test]
fn test_folded_scalar_<descriptive_name>() {
    // Clear description of what this tests
    let test_cases = vec![
        // Your test cases here
        create_folded_scalar_test(...),
        // Or manually: (line, key, type)
    ];

    run_folded_scalar_tests!(test_cases);
}
```

### 3. Add to Appropriate Section

Place your test in the appropriate section:
- Section 12B: For tests with exclamation marks
- Or create a new section if warranted

### 4. Document Any Special Cases

If your test covers unusual cases, add comments explaining:
- Why this case is special
- What YAML behavior is expected
- Any edge cases being tested

## Assertion Pattern

The `run_folded_scalar_tests!` macro performs two assertions:

1. **Line Type Assertion**
   ```rust
   assert_eq!(result, expected_type, "...");
   ```

2. **Key Detection Assertion** (only for MappingKey types)
   ```rust
   let info = detect_mapping_key(line, 0);
   assert!(info.is_some(), "...");
   assert_eq!(detected.key, expected_key, "...");
   ```

This ensures both:
- The line is correctly classified
- The key name is correctly extracted

## YAML Specification Reference

According to YAML 1.2 spec:
- Folded scalars use `>` indicator
- Explicit indent is specified with a number after the modifier
- The number defines the indentation level for continuation lines
- Modifiers affect handling of leading/trailing blank lines

## Related Beads

- **bf-57cf0**: Initial implementation of folded scalar explicit indent tests at various indentation levels
- **bf-63gy6**: Infrastructure and pattern documentation (this bead)

## Future Extensions

Potential additions to this infrastructure:

1. Continuation line testing macros
2. Multi-level YAML document testing
3. Performance benchmarking helpers
4. YAML serialization/deserialization round-trip tests

## Maintenance Notes

When modifying this infrastructure:
1. Keep macros backward compatible
2. Update this documentation
3. Add examples for new patterns
4. Test both macro and function-based approaches
