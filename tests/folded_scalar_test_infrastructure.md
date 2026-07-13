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
