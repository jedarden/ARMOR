# exit_to_scope Edge Case Analysis

## Current Implementation (lines 377-448 in scope.rs)

### Logic Flow

1. **Line 390-396**: Prevent exiting to deeper level than current
2. **Line 399-411**: Prevent emptying the scope stack (keep at least root)
3. **Line 415**: Remove scopes deeper than target
4. **Line 424-441**: Ensure we have a scope at target level or create fallback

## Identified Edge Cases and Bugs

### Bug 1: Incorrect adjusted_target logic (lines 424-441)

**Issue**: The code calculates `adjusted_target = target_indent + self.base_indent` and checks if a scope exists at that level. However, this check happens AFTER the retain operation at line 415 has already removed scopes deeper than target.

**Scenario that breaks**:
```yaml
# services:
#   web:
#     host: localhost
#   db:        # <-- exiting from web (indent 4) to services (indent 2)
#     host: db.example.com
```

When we call `exit_to_scope(2)` after being at indent 4:
1. Line 415: Removes scopes deeper than 2 (removes scope at 4)
2. Line 425: Checks if scope exists at 2 → FALSE
3. Line 427: Checks if scope exists at adjusted_target (2+2=4) → FALSE (already removed!)
4. Line 434-441: Creates fallback scope at 2

**But wait!** This might actually be correct behavior! When we exit to indent 2, we want a scope at indent 2. If the parent key "services" created a scope at indent 2, it would still be there (not removed by line 415).

### Bug 2: Fallback scope creation when parent exists

**Issue**: The code creates a fallback scope even when a shallower parent scope exists.

**Scenario that breaks**:
```yaml
services:
  web:
    config:
      debug: true
# db:    # <-- exiting from config (indent 6) to db (indent 2)
```

When we call `exit_to_scope(2)` after being at indent 6:
1. Initial scopes: [root(0), services(2), web(4), config(6)]
2. Line 415: Removes scopes deeper than 2 → [root(0), services(2)]
3. Line 425: Checks if scope exists at 2 → TRUE (services)
4. No fallback needed ✓

This case works correctly!

### Bug 3: Creating unnecessary fallback scopes

**Issue**: When exiting to an indent that has no scope and no shallower scope (except root), we create a fallback. But should we?

**Scenario**:
```rust
stack.enter_scope(4, 1, Some("level4".to_string()));
stack.exit_to_scope(2);
```

1. Initial scopes: [root(0), level4(4)]
2. Line 415: Removes scopes deeper than 2 → [root(0)]
3. Line 425: Checks if scope exists at 2 → FALSE
4. Line 427: Checks if scope exists at 4 → FALSE (already removed)
5. Creates fallback scope at 2

**Question**: Is this correct? The test expects it, but is it semantically correct?

Looking at the test (test_exit_to_non_existent_scope_creates_fallback), it seems the current behavior is intentional. However, it might create weird state where we have a scope with start_line=0 and parent_key=None.

### Bug 4: adjusted_target logic is confusing

**Issue**: The `adjusted_target` calculation at line 424 seems to assume that parent key scopes are at `target_indent + base_indent`, but this is only true when ENTERING a scope, not when exiting.

When we exit to `target_indent`, we're looking for a scope at exactly that indent level, not at `target_indent + base_indent`.

### Bug 5: No verification of target indent validity

**Issue**: The function doesn't validate that `target_indent` is a multiple of `base_indent`. Invalid YAML could have odd indentation that creates weird state.

**Scenario**:
```rust
stack.exit_to_scope(3); // With base_indent=2
```

This could create a scope at indent 3, which breaks the assumption that all indents are multiples of base_indent.

## Recommendations

1. **Remove the adjusted_target logic**: Lines 424-441 have confusing logic. Simplify to:
   - After retain, check if any scope exists at target_indent
   - If yes, done
   - If no, find nearest shallower scope (deepest scope with indent < target_indent)
   - If none except root, create fallback at target_indent (or just use root?)

2. **Add indent validation**: Ensure target_indent is a multiple of base_indent (or 0 for root).

3. **Better state cleanup**: When creating fallback scopes, ensure they have reasonable state (start_line, parent_key).

4. **Clearer semantics**: Document when fallback scopes should be created and when they shouldn't.
