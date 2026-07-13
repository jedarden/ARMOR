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

## Explicit Indent Coverage Gap Analysis

**Bead:** bf-6bai9
**Status:** Coverage gaps identified in Section 12B explicit indent test infrastructure
**Last Updated:** 2026-07-13

### Current Coverage Status

Section 12B includes the following explicit indent test functions:

| Test Function | Line | Modifier Type | Indentation Level | Coverage |
|---------------|------|---------------|-------------------|----------|
| `test_folded_scalar_explicit_indent_modifiers_at_various_levels()` | 8342 | All (>, >-, >+) | All levels | ✅ Comprehensive |
| `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space()` | 8561 | Plain (>) | 2-space only | ⚠️ Single level |
| `test_folded_scalar_strip_explicit_indent_modifiers_at_2_space()` | 8670 | Strip (>-) | 2-space only | ⚠️ Single level |
| `test_folded_scalar_keep_explicit_indent_modifiers_at_2_space()` | 13040 | Keep (>+) | 2-space only | ⚠️ Single level |
| `test_literal_scalar_explicit_indent_modifiers_at_various_levels()` | 8779 | All (|, |-, |+) | All levels | ✅ Comprehensive |

### Missing Level-Specific Test Functions

The following dedicated test functions are **missing** for folded scalars:

#### Plain Modifier (>) - Missing 4 Functions
- ❌ `test_folded_scalar_plain_explicit_indent_modifiers_at_4_space()`
- ❌ `test_folded_scalar_plain_explicit_indent_modifiers_at_6_space()`
- ❌ `test_folded_scalar_plain_explicit_indent_modifiers_at_8_space()`
- ❌ `test_folded_scalar_plain_explicit_indent_modifiers_at_tab()`

#### Strip Modifier (>-) - Missing 4 Functions
- ❌ `test_folded_scalar_strip_explicit_indent_modifiers_at_4_space()`
- ❌ `test_folded_scalar_strip_explicit_indent_modifiers_at_6_space()`
- ❌ `test_folded_scalar_strip_explicit_indent_modifiers_at_8_space()`
- ❌ `test_folded_scalar_strip_explicit_indent_modifiers_at_tab()`

#### Keep Modifier (>+) - Missing 4 Functions
- ❌ `test_folded_scalar_keep_explicit_indent_modifiers_at_4_space()`
- ❌ `test_folded_scalar_keep_explicit_indent_modifiers_at_6_space()`
- ❌ `test_folded_scalar_keep_explicit_indent_modifiers_at_8_space()`
- ❌ `test_folded_scalar_keep_explicit_indent_modifiers_at_tab()`

**Total Missing Functions:** 12 dedicated level-specific test functions

### Skeleton Template Reference

The skeleton template for adding missing test functions is located at:

**File:** `tests/type_like_string_false_positive_test.rs`  
**Function:** `test_folded_scalar_explicit_indent_template_example()`  
**Line:** 12788

### Recommended Additions (Section 12B.3 Pattern)

To add missing test coverage following the Section 12B.3 pattern:

#### Pattern Template
```rust
#[test]
fn test_folded_scalar_<modifier>_explicit_indent_modifiers_at_<level>() {
    // Test folded scalars with <modifier> explicit indent modifiers: <modifier>n for n=1-9
    // At <indentation> indentation level only
    // This provides focused coverage of <modifier> explicit indent specification for folded scalars
    // Follows the pattern established in test_folded_scalar_plain_explicit_indent_modifiers_at_2_space

    let test_cases = vec![
        // ===== Level X: <indentation> indentation with <modifier> explicit indent =====
        // <Modifier> <modifier>n (n=1-9) - main test cases
        ("  <key>1: >1", "<key>1", LineType::MappingKey),
        ("  <key>2: >2", "<key>2", LineType::MappingKey),
        // ... continue for n=1-9

        // Keys with exclamation marks at <indentation> indentation
        ("  key!1: >1", "key!1", LineType::MappingKey),
        // ... exclamation mark variants

        // Additional edge cases
        // ... specific edge cases for this level
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(result, expected_type, "...");

        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(info.is_some(), "...");
            let detected = info.unwrap();
            assert_eq!(detected.key, expected_key, "...");
        }
    }

    // Test continuation lines
    let continuation_lines = vec![
        // Continuation line test cases
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        assert!(expected_types.contains(&result), "...");
    }
}
```

#### Implementation Priority

**High Priority** (Core indentation levels):
1. `test_folded_scalar_plain_explicit_indent_modifiers_at_4_space()` - Most common after 2-space
2. `test_folded_scalar_strip_explicit_indent_modifiers_at_4_space()`
3. `test_folded_scalar_keep_explicit_indent_modifiers_at_4_space()`

**Medium Priority** (Deeper indentation):
4. `test_folded_scalar_plain_explicit_indent_modifiers_at_6_space()`
5. `test_folded_scalar_strip_explicit_indent_modifiers_at_6_space()`
6. `test_folded_scalar_keep_explicit_indent_modifiers_at_6_space()`

**Low Priority** (Less common):
7. `test_folded_scalar_plain_explicit_indent_modifiers_at_8_space()`
8. `test_folded_scalar_strip_explicit_indent_modifiers_at_8_space()`
9. `test_folded_scalar_keep_explicit_indent_modifiers_at_8_space()`
10. Tab indentation variants (3 functions)

### Coverage Notes

- **Comprehensive function exists:** `test_folded_scalar_explicit_indent_modifiers_at_various_levels()` provides coverage for all levels in a single test
- **Gap is in dedicated level-specific functions:** The missing functions are isolated tests for each level-modifier combination, which provides better failure isolation and targeted debugging
- **Literal scalars:** Fully covered with dedicated functions for all levels in `test_literal_scalar_explicit_indent_modifiers_at_various_levels()`

### Verification Command

To verify current explicit indent test coverage:
```bash
# List all explicit indent test functions
grep -n "^fn test_.*explicit_indent" tests/type_like_string_false_positive_test.rs | grep -E "(plain|strip|keep|level|space|tab)"

# Run explicit indent tests
cargo test test_folded_scalar_explicit_indent

# Check coverage in Section 12B
cargo test -- section-12b
```

## Section 12B Test Function Index by Pattern

### Pattern 1: Comprehensive Multi-Level Testing
**Functions demonstrating this pattern:**
- `test_folded_scalar_explicit_indent_modifiers_at_various_levels()` (line 8342)
  - Tests all modifier types (>, >-, >+) at all indentation levels (2, 4, 6, 8-space, tab)
  - Covers indent levels 1-9 for each modifier
  - **Pattern:** Comprehensive single-function coverage across all dimensions
  - **See:** Lines 8342-8399 for implementation

### Pattern 2: Single-Indent Level Focused Testing
**Functions demonstrating this pattern:**
- `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space()` (line 8561)
  - Tests plain modifier (>) at 2-space indentation only
  - Indent levels 1-9
  - **Pattern:** Focused coverage for specific indent-modifier combination
  - **See:** Lines 8561-8669 for implementation

- `test_folded_scalar_strip_explicit_indent_modifiers_at_2_space()` (line 8670)
  - Tests strip modifier (>-) at 2-space indentation only
  - Indent levels 1-9
  - **Pattern:** Focused coverage for specific indent-modifier combination
  - **See:** Lines 8670-8778 for implementation

- `test_folded_scalar_keep_explicit_indent_modifiers_at_2_space()` (line 13040)
  - Tests keep modifier (>+) at 2-space indentation only
  - Indent levels 1-9
  - **Pattern:** Focused coverage for specific indent-modifier combination
  - **See:** Lines 13040-13147 for implementation

- `test_literal_scalar_explicit_indent_modifiers_at_various_levels()` (line 8779)
  - Tests all literal scalar modifiers (|, |-, |+) at all levels
  - **Pattern:** Comprehensive coverage for literal scalars
  - **See:** Lines 8779-9229 for implementation

### Pattern 3: Template/Infrastructure Pattern
**Functions demonstrating this pattern:**
- `test_folded_scalar_explicit_indent_template_example()` (line 12788)
  - Demonstrates macro-based test generation
  - **Pattern:** Template for new test development
  - **See:** Lines 12788-12806 for implementation

- `test_folded_scalar_explicit_indent_tab_template()` (line 12809)
  - Demonstrates tab indentation testing
  - **Pattern:** Template for tab-based tests
  - **See:** Lines 12809-12822 for implementation

- `test_folded_scalar_explicit_indent_helper_function_example()` (line 12825)
  - Demonstrates helper function approach
  - **Pattern:** Template for function-based test generation
  - **See:** Lines 12825-12859 for implementation

### Pattern 4: Basic Indicator Line Assertions
**Functions demonstrating this pattern:**
- `test_folded_block_scalar_with_exclamation_marks()` (lines 7901-7959)
  - Section 12B entry point for folded scalars with exclamation marks
  - Tests indicator lines at lines 7904-7919
  - **Pattern:** Basic indicator classification with `assert_eq!` to `MappingKey`
  - **See:** Lines 7904-7919 for indicator tests, 7941-7959 for continuation tests

- `test_literal_block_scalar_with_exclamation_marks()` (lines 7962-8009)
  - Tests literal scalar indicators at lines 7965-7974
  - **Pattern:** Basic indicator classification for literal scalars
  - **See:** Lines 7965-7974 for indicator tests, 7988-8008 for continuation tests

- `test_folded_scalar_indicator_classification()` (line 10730)
  - Section 12B.1 comprehensive indicator tests
  - Tests at lines 10733-10766
  - **Pattern:** Comprehensive indicator classification across all variants
  - **See:** Lines 10730-10779 for full implementation

- `test_folded_scalar_indicator_lines()` (line 10524)
  - Section 12B.2 basic indicator tests
  - Tests at lines 10528-10535
  - **Pattern:** Simple indicator line validation
  - **See:** Lines 10524-10546 for implementation

### Pattern 5: Continuation Line Assertions with Allowed Types
**Functions demonstrating this pattern:**
- `test_folded_block_scalar_with_exclamation_marks()` (lines 7941-7959)
  - Continuation lines at lines 7942-7958
  - Uses basic pattern: `assert!(result == MappingKey || result == Unknown)`
  - **Pattern:** Binary allowed types without tuple structure
  - **See:** Lines 7941-7959 for implementation

- `test_literal_block_scalar_with_exclamation_marks()` (lines 7988-8008)
  - Continuation lines at lines 7989-7997
  - Uses tuple pattern: `(line, vec![allowed_types])`
  - Includes Tag type support for lines starting with `!`
  - **Pattern:** Tuple-based with multiple allowed types including Tag
  - **See:** Lines 7988-8008 for implementation

- `test_folded_scalar_basic_modifiers()` (line 10549)
  - Continuation lines for strip (>-) and keep (>+) modifiers
  - **Pattern:** Tuple-based continuation testing for modified scalars
  - **See:** Lines 10549-10594 for implementation

### Pattern 6: Key Extraction Assertions
**Functions demonstrating this pattern:**
- `test_folded_scalar_explicit_indent_modifiers_at_various_levels()` (line 8342)
  - Key extraction tests at lines 8390-8399
  - **Pattern:** `detect_mapping_key()` followed by key assertion
  - **See:** Lines 8390-8399 for implementation

- `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space()` (line 8561)
  - Key extraction tests throughout
  - **Pattern:** Verifies correct key extraction for explicit indent cases
  - **See:** Lines 8561-8669 for implementation

**Note:** Key extraction assertions typically follow indicator line assertions in the same test function, providing complete validation of both classification and parsing.

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

**Concrete Implementations in Section 12B:**

#### Function: `test_folded_block_scalar_with_exclamation_marks()` (line 7901)
```rust
// Section 12B: Multiline String Scenarios with Exclamation Marks (line 7901)
// Indicator line tests at lines 7904-7919
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
        // ... more test cases
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

#### Function: `test_literal_block_scalar_with_exclamation_marks()` (line 7962)
```rust
// Section 12B: Literal scalars with exclamation marks (line 7962)
// Indicator line tests at lines 7965-7974
fn test_literal_block_scalar_with_exclamation_marks() {
    let test_cases = vec![
        "description: |",               // Basic literal scalar
        "  literal_text: |",             // Indented literal scalar
        "    note: |",                   // Deep indented literal scalar
        "\tmessage: |",                 // Tab-indented literal scalar
        "warning: |-",                  // Literal with strip modifier
        "info: |+",                     // Literal with keep modifier
        "text: |-2",                    // Literal with explicit indent
        "content: |2",                  // Literal with explicit indent
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Literal block scalar indicator should be MappingKey: '{}'",
            line
        );
    }
}
```

#### Function: `test_folded_scalar_indicator_classification()` (line 10730)
```rust
// Section 12B.1: Comprehensive indicator classification (line 10730)
// Indicator line tests at lines 10733-10766
fn test_folded_scalar_indicator_classification() {
    let test_cases = vec![
        // Basic folded scalar indicator
        "description: >",
        "  folded_text: >",
        "    note: >",
        "\tmessage: >",

        // Folded with strip modifier (-)
        "warning: >-",
        "  alert: >-",
        "    info: >-",

        // Folded with keep modifier (+)
        "log: >+",
        "  output: >+",
        "    data: >+",

        // Folded with explicit indent (2)
        "text: >-2",
        "content: >2",
        "  field: >-2",
        "    value: >2",

        // Folded with explicit indent (4)
        "doc: >-4",
        "info: >4",
        "  body: >-4",
        "    detail: >4",

        // Tab-indented folded scalars
        "\tfolded: >",
        "\t  note: >",
        "\t    text: >",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded scalar indicator should be classified as MappingKey: '{}'",
            line
        );
    }
}
```

#### Function: `test_folded_scalar_indicator_lines()` (line 10524)
```rust
// Section 12B.2: Basic indicator line tests (line 10524)
// Indicator line tests at lines 10528-10535
fn test_folded_scalar_indicator_lines() {
    let test_cases = vec![
        // Basic folded scalar indicators (>)
        "description: >",
        "content: >",
        "message: >",
        "text: >",
        "note: >",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Basic folded scalar indicator (>) should be MappingKey: '{}'",
            line
        );
    }
}
```

**Assertion Pattern:**
- Single `assert_eq!` comparing result to `LineType::MappingKey`
- Descriptive error message includes the failing line
- Tests the indicator line itself (key followed by `>` or `|`)

**When to use this pattern:**
- Testing YAML block scalar indicator lines
- Verifying basic line classification for block scalars
- Simple validation without key extraction checks

**Cross-reference:** See Pattern 2 (below) for key extraction assertions that often follow indicator classification

---

### Pattern 5: Continuation Line Assertion Patterns with Allowed Types

**Purpose:** Test continuation lines of block scalars where multiple line types may be valid

**Structure:** Two variants:
1. **Basic pattern:** `vec!` of lines with `assert!(result == Type1 || result == Type2)`
2. **Tuple pattern:** `vec!` of tuples `(line, vec![allowed_types])` with `assert!(expected_types.contains(&result))`

**Concrete Implementations in Section 12B:**

#### Function: `test_folded_block_scalar_with_exclamation_marks()` - Basic Pattern (line 7901)
```rust
// Section 12B: Folded scalar continuation lines (line 7901)
// Continuation line tests at lines 7941-7959
fn test_folded_block_scalar_with_exclamation_marks() {
    // ... indicator line tests ...

    // Continuation lines - Basic pattern without tuple structure
    let continuation_lines = vec![
        "  This is folded text with! exclamation marks",
        "    Multiple! exclamations! in! folded! style",
        "\tMore! content! with! bangs!",
        "  Important! message! continues!",
        "    Another! line! with! emphasis!",
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

#### Function: `test_literal_block_scalar_with_exclamation_marks()` - Tuple Pattern (line 7962)
```rust
// Section 12B: Literal scalar continuation with Tag support (line 7962)
// Continuation line tests at lines 7988-8008
fn test_literal_block_scalar_with_exclamation_marks() {
    // ... indicator line tests ...

    // Continuation lines - Tuple pattern with multiple allowed types
    let continuation_lines = vec![
        ("  This is literal text with! exclamation marks", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Multiple! exclamations! in! literal! style", vec![LineType::MappingKey, LineType::Unknown]),
        ("\tMore! content! with! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Important! message! continues!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Another! line! with! emphasis!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Lines with! at! various! positions!", vec![LineType::MappingKey, LineType::Unknown]),
        // Lines starting with '!' can also be Tag type
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

#### Function: `test_folded_scalar_basic_modifiers()` (line 10549)
```rust
// Section 12B.2: Basic modifier continuation tests (line 10549)
// Demonstrates continuation lines for strip (>-) and keep (>+) modifiers
fn test_folded_scalar_basic_modifiers() {
    // ... indicator line tests for >-, >+ ...

    // Continuation lines for modified scalars
    let continuation_lines = vec![
        ("  Some content after >", vec![LineType::MappingKey, LineType::Unknown]),
        ("    More indented content", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        assert!(
            expected_types.contains(&result),
            "Continuation line should be one of {:?}: '{}'",
            expected_types, line
        );
    }
}
```

**Assertion Pattern Comparison:**

| Pattern | Pros | Cons | When to Use |
|---------|------|------|-------------|
| **Basic** (`assert!(A \|\| B)`) | Simpler, less code | Fixed to 2 types, harder to extend | When you only need 2 allowed types |
| **Tuple** (`vec![A, B, C]`) | Flexible, any number of types | More verbose | When you need 3+ types or Tag support |

**Allowed Types:**
- `vec![LineType::MappingKey, LineType::Unknown]` - Most continuation lines (lines with `!` not at start)
- `vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]` - Lines starting with `!` (could be a Tag)

**When to use this pattern:**
- Continuation lines of block scalars (indented content following indicator)
- Lines with ambiguous classification (multiple valid types)
- Testing exclamation marks at various positions in continuation lines
- When `Tag` type is possible for lines starting with `!`

**Cross-reference:** This pattern complements Pattern 4 (Indicator Line Assertions) - first test the indicator line, then test its continuation lines

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
