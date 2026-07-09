# Pluck Settings That Hide Beads

**Bead:** bf-4351  
**Workspace:** /home/coding/ARMOR  
**Created:** 2026-07-09

## Overview

This document analyzes all Pluck strand settings that can prevent beads from being selected for processing. The Pluck strand is responsible for >90% of bead selection in the NEEDLE system.

---

## Settings That Directly Hide Beads

### 1. `exclude_labels` (Primary Filtering Mechanism)

**Location:** `strands.pluck.exclude_labels`  
**Type:** `Array<String>`  
**Default:** `["deferred", "human", "blocked"]` (applied when config is empty)  
**Current Value:** `[]` (empty array in pluck-config.yaml)

**How it hides beads:**
- Any bead with a label matching an entry in `exclude_labels` is removed from the candidate list
- Filtering occurs at two levels:
  1. **Store level:** Passed to `store.ready(&filters)` - the bead store filters excluded labels
  2. **Strand level:** Defensive filtering in `PluckStrand.evaluate()` (lines 141-186) removes any excluded-label beads that the store may have returned
- The defensive filter prevents the SELECTING→CLAIMING→RETRYING spin loop when the backend omits label data

**Default behavior (when config array is empty):**
```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked"];
```

**Impact:**
- With current config (`exclude_labels: []`), defaults apply: beads labeled `deferred`, `human`, or `blocked` are hidden
- With custom config (e.g., `["wip", "review"]`), only those labels are excluded
- Setting to non-empty array overrides defaults completely

**Example from code:**
```rust
// Line 149 in pluck.rs
candidates.retain(|b| !b.labels.iter().any(|l| self.exclude_labels.contains(l)));
```

---

### 2. Status Filtering (Implicit Hiding)

**Location:** Built into `PluckStrand.evaluate()` (lines 188-210)  
**Not configurable** - always applied

**How it hides beads:**
Two status checks prevent beads from being claimable:

1. **InProgress beads:** Any bead with status `InProgress` is filtered out
   - These beads are actively being processed by another worker
   - Claiming them would fail immediately

2. **Open beads with stale assignee:** Open beads that have an `assignee` field set are filtered out
   - These beads were claimed but not properly released
   - The claimer would reject them every time, causing a hot loop

**Code:**
```rust
// Lines 192-195 in pluck.rs
candidates.retain(|b| {
    !(matches!(b.status, crate::types::BeadStatus::InProgress)
        || (b.status == crate::types::BeadStatus::Open && b.assignee.is_some()))
});
```

**Impact:**
- Prevents selection of already-claimed beads
- Prevents race conditions and claim retry loops

---

### 3. `split_after_failures` (Indirect Hiding via Split Instruction)

**Location:** `strands.pluck.split_after_failures`  
**Type:** `u32`  
**Default:** `3`  
**Current Value:** `0` (disabled in pluck-config.yaml)

**How it affects bead visibility:**
- When enabled (> 0), checks the first candidate's `failure-count:N` label
- If `failure_count >= split_after_failures`, returns a `Split` instruction instead of the bead
- The bead is not processed normally — it's sent to the Mitosis strand for decomposition

**This is not true "hiding"** — the bead is still discovered, but processing is diverted:

**Code:**
```rust
// Lines 229-252 in pluck.rs
if self.split_after_failures > 0 {
    if let Some(first_candidate) = candidates.first() {
        let failure_count = Self::extract_failure_count(first_candidate);
        if failure_count >= self.split_after_failures {
            return StrandResult::Split(Box::new(first_candidate.clone()), failure_count);
        }
    }
}
```

**Impact:**
- When `0` (current config): Disabled — all beads process normally
- When `> 0`: Beads with accumulated failures are split before processing
- Split beads become parent beads that depend on their children, making them non-ready until children complete

---

## Settings That Do NOT Hide Beads

### `sort_order` (Candidate Ordering)

**Location:** `strands.pluck.sort_order` (not in current ARMOR config)  
**Type:** `String`  
**Default:** `"priority"`

**How it works:**
- Sorts candidates in deterministic order: `(priority ASC, created_at ASC, id ASC)`
- Does not filter or hide any beads
- Only affects which bead is processed first

---

## Configuration Hierarchy

Pluck configuration resolution order (later overrides earlier):

1. **Built-in defaults:**
   - `exclude_labels: []` → defaults to `["deferred", "human", "blocked"]`
   - `split_after_failures: 3`

2. **Global config:** `~/.config/needle/config.yaml`

3. **Workspace config:** `.needle.yaml` → `strands.pluck` section

4. **Environment variables:** `NEEDLE_STRANDS__PLUCK__EXCLUDE_LABELS`

5. **CLI arguments:** (highest precedence)

---

## Current ARMOR Configuration

From `/home/coding/ARMOR/pluck-config.yaml`:

```yaml
filtering:
  # Empty array means defaults apply: ["deferred", "human", "blocked"]
  exclude_labels: []
  
  # Split disabled (0 = disabled)
  split_after_failures: 0
  
  # Note: sort_order not specified in ARMOR config
```

**Current behavior:**
- Beads labeled `deferred`, `human`, or `blocked` are hidden (default applies)
- No auto-split on failures (disabled)
- Standard priority-based sorting

---

## Combined Filtering Effects

Multiple filters can combine to hide beads:

**Example 1: Deferred + Failed Bead**
```
Bead ID: bf-123
Labels: [deferred, failure-count:5]
Status: Open
Assignee: None

Result: HIDDEN by exclude_labels ("deferred")
```

**Example 2: In-Progress Bead**
```
Bead ID: bf-456
Labels: []
Status: InProgress
Assignee: "worker-1"

Result: HIDDEN by status filter (InProgress)
```

**Example 3: Stale Assignee**
```
Bead ID: bf-789
Labels: []
Status: Open
Assignee: "worker-2" (worker dead or crashed)

Result: HIDDEN by status filter (Open with assignee)
```

**Example 4: Split Candidate (when enabled)**
```
Bead ID: bf-abc
Labels: [failure-count:3]
Status: Open
Assignee: None

Result with split_after_failures=3: DIVERTED to Split (not hidden, but not processed normally)
Result with split_after_failures=0: PROCESSED NORMALLY
```

---

## Detection and Debugging

### Enable Filtering Decision Logging

Current ARMOR config has debug logging enabled:

```yaml
debug:
  level: debug
  log_filtering_decisions: true
  log_bead_store_queries: true
```

**Log location:** `logs/pluck-debug.log`

**What to look for:**
```
# Beads excluded by label filter
tracing::debug!(
    bead_id = %id,
    labels = ?labels,
    excluded_reasons = ?["deferred", "human"],
    "Excluded bead due to labels"
);

# Status/assignee filtering
tracing::debug!(
    filtered_count = 2,
    remaining = 5,
    "Status/assignee filtering removed {} beads"
);

# Split trigger
tracing::info!(
    bead_id = %id,
    failure_count = 5,
    threshold = 3,
    "Split threshold reached, returning Split instruction"
);
```

---

## Summary

All Pluck settings that affect bead visibility:

| Setting | Hides Beads? | Default | Current ARMOR | How to Disable |
|---------|-------------|---------|---------------|----------------|
| `exclude_labels` | **YES** | `["deferred", "human", "blocked"]` | `[]` → defaults apply | Set to `[]` and accept defaults, or set custom labels |
| Status filter (InProgress) | **YES** | Always on | Always on | Cannot disable (built-in safety) |
| Status filter (stale assignee) | **YES** | Always on | Always on | Cannot disable (built-in safety) |
| `split_after_failures` | **NO** (diverts) | `3` | `0` (disabled) | Set to `0` to disable |

**Key insight:** The primary knob for controlling which beads are hidden is `exclude_labels`. The current ARMOR configuration (`exclude_labels: []`) means the default exclusions (`deferred`, `human`, `blocked`) are in effect.

---

## Recommendations

1. **To expose more beads:** Customize `exclude_labels` to only hide truly inappropriate beads:
   ```yaml
   filtering:
     exclude_labels: ["human"]  # Only hide human-required beads
   ```

2. **To hide fewer beads:** Set `exclude_labels` to an empty array and override defaults at the Rust level (requires code change)

3. **To enable auto-split:** Set `split_after_failures` to a positive value:
   ```yaml
   filtering:
     split_after_failures: 3  # Split after 3 consecutive failures
   ```

4. **Monitoring:** Keep `log_filtering_decisions: true` to track which beads are being excluded and why

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-07-09  
**Next Review:** When Pluck configuration changes
