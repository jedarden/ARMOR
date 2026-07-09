# Pluck Filter Configurations - Complete Documentation

**Bead:** bf-1jwl  
**Source:** `/home/coding/NEEDLE/src/strand/pluck.rs` and `/home/coding/NEEDLE/src/bead_store/mod.rs`  
**Date:** 2026-07-08

## Overview

Pluck's filter configuration is **compiled into the NEEDLE binary** - there are no external configuration files. All filter settings are hardcoded in Rust source code at `/home/coding/NEEDLE/src/strand/pluck.rs`.

---

## 1. Core Filters Structure

**Location:** `/home/coding/NEEDLE/src/bead_store/mod.rs:76-82`

```rust
#[derive(Debug, Default, Clone)]
pub struct Filters {
    /// Only return beads assigned to this actor. `None` = no filter.
    pub assignee: Option<String>,
    /// Exclude beads that have any of these labels.
    pub exclude_labels: Vec<String>,
}
```

### Filter Settings:

| Setting | Type | Default Value | Purpose |
|---------|------|---------------|---------|
| `assignee` | `Option<String>` | `None` | Filters beads to only those assigned to a specific actor |
| `exclude_labels` | `Vec<String>` | See below | Removes beads with any of these labels from candidate list |

---

## 2. Default Exclude Labels

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:13`

```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked"];
```

### Default Excluded Labels:

| Label | Purpose |
|-------|---------|
| `deferred` | Beads intentionally deferred for later processing |
| `human` | Beads requiring human intervention/manual review |
| `blocked` | Beads blocked by dependencies (not ready to process) |

**Note:** These defaults are **only applied when no custom exclude_labels are provided** to `PluckStrand::new()`.

---

## 3. PluckStrand Configuration

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:16-21`

```rust
pub struct PluckStrand {
    /// Labels to exclude from candidate selection.
    exclude_labels: Vec<String>,
    /// Auto-split beads after this many consecutive failures (0 = disabled).
    split_after_failures: u32,
}
```

### Configuration Parameters:

| Parameter | Type | Default | Purpose |
|-----------|------|---------|---------|
| `exclude_labels` | `Vec<String>` | `["deferred", "human", "blocked"]` | Labels to filter from candidates |
| `split_after_failures` | `u32` | `3` | Consecutive failure threshold before auto-split trigger |

---

## 4. Runtime Filtering Logic

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:118-133`

Pluck applies multiple filter stages during evaluation:

### Stage 1: Store Query Filters (Line 105-108)
```rust
let filters = Filters {
    assignee: None,
    exclude_labels: self.exclude_labels.clone(),
};
```

### Stage 2: Defensive Label Filtering (Line 125)
```rust
candidates.retain(|b| !b.labels.iter().any(|l| self.exclude_labels.contains(l)));
```
**Purpose:** Double-checks that excluded-label beads are never presented, even if the backing store doesn't filter them.

### Stage 3: Status/Assignee Filtering (Line 130-133)
```rust
candidates.retain(|b| {
    !(matches!(b.status, crate::types::BeadStatus::InProgress)
        || (b.status == crate::types::BeadStatus::Open && b.assignee.is_some()))
});
```
**Filters out:**
- Beads with `in_progress` status (claimed by another worker)
- `Open` beads with a stale assignee (would cause hot-loop retry failures)

---

## 5. Sorting Configuration

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:79-86`

```rust
fn sort_candidates(candidates: &mut [Bead]) {
    candidates.sort_by(|a, b| {
        a.priority
            .cmp(&b.priority)
            .then_with(|| a.created_at.cmp(&b.created_at))
            .then_with(|| a.id.as_ref().cmp(b.id.as_ref()))
    });
}
```

### Sort Order (Deterministic):

1. **Priority** (ASC) - Lower numbers first (P1 < P2 < P3)
2. **Created Date** (ASC) - Older beads first
3. **Bead ID** (ASC) - Lexicographic tie-breaker

**Purpose:** Ensures all workers compute identical candidate ordering from the same queue state.

---

## 6. Split Trigger Configuration

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:141-148`

```rust
if self.split_after_failures > 0 {
    if let Some(first_candidate) = candidates.first() {
        let failure_count = Self::extract_failure_count(first_candidate);
        if failure_count >= self.split_after_failures {
            return StrandResult::Split(Box::new(first_candidate.clone()), failure_count);
        }
    }
}
```

### Split Logic:

| Setting | Value | Behavior |
|---------|-------|----------|
| `split_after_failures` | `0` | Split disabled - bead returned for normal processing |
| `split_after_failures` | `> 0` | Split triggered when `failure-count:N` ≥ threshold |

### Failure Count Extraction (Line 66-73):
```rust
fn extract_failure_count(bead: &Bead) -> u32 {
    bead.labels
        .iter()
        .filter_map(|l| l.strip_prefix("failure-count:"))
        .filter_map(|s| s.parse::<u32>().ok())
        .max()
        .unwrap_or(0)
}
```

**Looks for labels matching:** `failure-count:N` (e.g., `failure-count:3`)

---

## 7. How Filters are Applied (Execution Flow)

```
1. Query bead store with Filters {assignee, exclude_labels}
   └─> br ready --json [--assignee <actor>]

2. Apply defensive label filtering
   └─> Remove beads with any label in exclude_labels

3. Apply status/assignee filtering  
   └─> Remove in_progress beads
   └─> Remove Open beads with stale assignee

4. Sort by deterministic priority order
   └─> (priority ASC, created_at ASC, id ASC)

5. Check split trigger
   └─> If failure-count >= split_after_failures: return Split
   └─> Else: return BeadFound
```

---

## 8. Filter Configuration Summary

### Configurable Filters:

| Filter | Configurable Via | Default Value | Location |
|--------|------------------|---------------|----------|
| `exclude_labels` | `PluckStrand::new()` | `["deferred", "human", "blocked"]` | pluck.rs:13 |
| `split_after_failures` | `PluckStrand::with_split_threshold()` | `3` | pluck.rs:39 |
| `assignee` | `Filters.assignee` | `None` | mod.rs:79 |

### Hardcoded (Non-Configurable) Filters:

| Filter | Value | Purpose | Location |
|--------|-------|---------|----------|
| Status filter | Remove `in_progress` | Prevent claiming beads already owned | pluck.rs:131 |
| Assignee filter | Remove `Open + assignee` | Prevent stale assignee hot-loops | pluck.rs:132 |
| Sort order | `(priority, created_at, id)` | Deterministic ordering | pluck.rs:80-85 |

---

## 9. Key Implementation Notes

1. **No External Configuration:** All Pluck settings are compiled into the NEEDLE binary. Changing them requires modifying source code and recompiling.

2. **Defense in Depth:** Pluck applies label filtering twice:
   - First in the store query (`Filters.exclude_labels`)
   - Again in the strand (`candidates.retain()`)
   - This prevents hot-loops when backends omit label data.

3. **Determinism:** Same queue state → same ordering across all workers, guaranteed by sort keys including bead ID as final tie-breaker.

4. **Split Behavior:** Auto-split is **enabled by default** (threshold=3). Setting threshold to 0 disables it entirely.

5. **Empty Labels = Defaults:** If `PluckStrand::new(vec![])` is called with an empty vector, the default exclude labels (`deferred`, `human`, `blocked`) are automatically applied.

---

## 10. Related Beads

- **bf-63ug** - Located Pluck configuration files (found: compiled-in defaults)
- **bf-ogec** - Extract exclude_labels settings from Pluck config  
- **bf-4351** - Analyze which Pluck settings hide beads (depends on this bead)

---

**Status:** ✅ Complete - All filter configurations documented