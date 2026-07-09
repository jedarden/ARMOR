# Pluck Settings That Hide Beads - Verification Summary

**Date:** 2026-07-09  
**Bead:** bf-4351  
**Status:** ✅ VERIFIED - Analysis accurate and complete

## Verification Results

### Source Materials Reviewed
1. **Existing Analysis:** `/home/coding/ARMOR/notes/bf-4351-pluck-settings-that-hide-beads.md`
2. **Current Implementation:** `/home/coding/NEEDLE/src/strand/pluck.rs`
3. **ARMOR Configuration:** `/home/coding/ARMOR/.needle.yaml`

### Accuracy Assessment: ✅ CONFIRMED ACCURATE

All 7 settings identified in the original analysis are **verified** against the current Pluck implementation:

| Setting | Line in pluck.rs | Analysis Status |
|---------|------------------|-----------------|
| `exclude_labels` | 13, 18, 28-41 | ✅ Accurate |
| `assignee` filter | 113-114 | ✅ Accurate |
| Status filter (in_progress) | 193 | ✅ Accurate |
| Stale assignee filter | 194 | ✅ Accurate |
| `split_after_failures` | 20, 39, 229-252 | ✅ Accurate |
| Sort order | 80-86 | ✅ Accurate |
| Empty queue condition | 255-257 | ✅ Accurate |

## ARMOR-Specific Configuration

### Current ARMOR Settings (`.needle.yaml`)
```yaml
strands:
  pluck:
    exclude_labels: []      # Empty = activates DEFAULTS
    split_after_failures: 0 # Disabled
```

### Practical Implications for ARMOR

1. **Label Exclusions Active**: Despite `exclude_labels: []` in config, ARMOR **is using** the default exclusions:
   - `deferred` beads are hidden
   - `human` beads are hidden  
   - `blocked` beads are hidden

2. **Split Disabled**: `split_after_failures: 0` means:
   - No automatic bead splitting on repeated failures
   - Beads with `failure-count:N` labels are processed normally
   - No diversion to split handler

3. **Hardcoded Filters Active**: Non-configurable filters still apply:
   - `InProgress` status beads hidden (prevents race conditions)
   - Open beads with stale assignees hidden (prevents retry loops)

## Key Findings Summary

### Direct Hiding (4 settings)
1. **exclude_labels** - PRIMARY filter mechanism
   - Default: `["deferred", "human", "blocked"]`
   - Impact: HIGH
   - Applied TWICE for defense in depth

2. **assignee filter** - Actor-specific filtering
   - Default: `None` (no filter)
   - Impact: MEDIUM
   - Hides beads NOT assigned to specified actor

3. **Status filter** - In-progress hiding
   - Hardcoded, cannot be disabled
   - Impact: CRITICAL
   - Prevents concurrent processing

4. **Stale assignee filter** - Orphaned bead prevention
   - Hardcoded, cannot be disabled
   - Impact: HIGH
   - Hides `Open` beads with leftover assignees

### Indirect Hiding (3 settings)
5. **split_after_failures** - Auto-split trigger
   - Default: `3`
   - Impact: MEDIUM
   - Diverts beads to split handler instead of normal processing
   - DISABLED in ARMOR (set to `0`)

6. **Sort order** - Candidate prioritization
   - Hardcoded: `(priority ASC, created_at ASC, id ASC)`
   - Impact: LOW
   - Effectively hides low-priority beads until higher-priority work done

7. **Empty queue** - Runtime condition
   - Impact: VARIABLE
   - All beads hidden when filter combination returns empty list

## Most Common Reasons Beads Are Hidden in ARMOR

Based on current configuration:

1. **Has excluded label** (`deferred`, `human`, `blocked`) - Most common
2. **Status is InProgress** - Claimed by another worker
3. **Open with stale assignee** - Failed to release after crash
4. **Low priority** - Behind higher-priority beads in queue

## Debug Commands for ARMOR

```bash
# Check why a specific bead is hidden
br show <bead-id> | grep -E "(Labels|Status|Assignee|Priority):"

# See all beads that passed filters (ready queue)
br ready --json

# Check current Pluck configuration
cat .needle.yaml | grep -A 5 "pluck:"

# Verify defaults are active (empty list = defaults)
br show <bead-with-deferred-label>  # Should be hidden
```

## Conclusion

✅ **The original analysis is complete and accurate.**  
✅ **All 7 hiding settings verified against current implementation.**  
✅ **ARMOR configuration implications documented.**

### Recommendations

1. **For debugging hidden beads**: Always check labels first - they're the most common hiding mechanism
2. **For maximum visibility**: Modify code to use truly empty defaults, or pass custom exclude list
3. **For distributed systems**: Hardcoded filters (status, stale assignee) are essential - never disable

---

**Analysis Source:** `/home/coding/ARMOR/notes/bf-4351-pluck-settings-that-hide-beads.md`  
**Implementation Verified:** 2026-07-09 against NEEDLE commit a47c0e8  
**ARMOR Config:** Current as of 2026-07-09
