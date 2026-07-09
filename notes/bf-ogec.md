# Pluck exclude_labels Configuration Extraction

**Date:** 2026-07-08  
**Workspace:** `/home/coding/ARMOR`  
**Bead:** bf-ogec  
**Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`

## Complete exclude_labels Settings

### 1. Default Exclude Labels (Hardcoded)

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:13`

```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked"];
```

**Values:**
- `deferred` - Beads marked for later processing
- `human` - Beads requiring human intervention
- `blocked` - Beads blocked by dependencies

**Application:** Applied automatically when `exclude_labels` parameter is empty in `PluckStrand::new()` or `PluckStrand::with_split_threshold()`.

### 2. Custom Exclude Labels (Override)

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:28-36`

When custom exclude_labels are provided (non-empty vector), they **completely replace** the defaults.

```rust
pub fn new(exclude_labels: Vec<String>) -> Self {
    let labels = if exclude_labels.is_empty() {
        DEFAULT_EXCLUDE_LABELS
            .iter()
            .map(|s| (*s).to_string())
            .collect()
    } else {
        exclude_labels  // Custom labels override defaults
    };
    // ...
}
```

**Behavior:**
- Empty `exclude_labels` → Uses defaults: `["deferred", "human", "blocked"]`
- Non-empty `exclude_labels` → Uses only provided labels (defaults ignored)

### 3. Filtering Implementation

**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:105-125`

Filtering occurs in **two layers** for defense in depth:

**Layer 1 - Bead Store Query:**
```rust
let filters = Filters {
    assignee: None,
    exclude_labels: self.exclude_labels.clone(),
};
let mut candidates = store.ready(&filters).await?;
```

**Layer 2 - Defensive Guard (in PluckStrand):**
```rust
// Defensive guard — store.ready() passes exclude_labels in its Filters,
// but the backing CLI may not include label data in every query type.
candidates.retain(|b| !b.labels.iter().any(|l| self.exclude_labels.contains(l)));
```

This dual-layer filtering prevents the SELECTING→CLAIMING→RETRYING spin loop when the backing `br ready --json` omits label fields.

## Patterns in Excluded Labels

### Semantic Categories
1. **Temporal deferral:** `deferred` - "not now, later"
2. **Human gating:** `human` - "requires human judgment/intervention"
3. **Dependency blocking:** `blocked` - "waiting on other beads"

### Common Use Cases
- **deferred:** Lower-priority work, exploratory tasks, backlog items
- **human:** Code reviews, manual testing, design decisions
- **blocked:** Tasks with unmet dependencies (see `bf-520v` for context)

## Configuration Notes

### No External Configuration Files
Pluck has **no external config files** (`.pluck.yml`, `pluckconfig`, etc.). All configuration is compiled into the NEEDLE binary at `/home/coding/NEEDLE/src/strand/pluck.rs`.

### Runtime Behavior
- Worker instantiates `PluckStrand` with exclude_labels at startup
- Labels cannot be changed without restarting NEEDLE
- Custom exclude_labels are passed via command-line or worker configuration (not in Pluck config file)

## Related Findings

- **bf-63ug:** Confirmed no external Pluck config files exist
- **bf-1hm4:** Previous Pluck configuration investigation  
- **bf-4axz:** Pluck configuration investigation
- The commit message for bf-63ug mentioned `starvation-alert` as a default label, but **source code shows only 3 defaults**: `deferred`, `human`, `blocked`. The fourth label may have been removed or never existed in this version.

## Verification

Test cases at lines 516-561 verify:
1. Default labels filter correctly (`beads_with_excluded_labels_are_filtered`)
2. Custom labels override defaults (`custom_exclude_labels_override_defaults`)
3. Empty list of excluded labels returns NoWork (`all_excluded_labels_returns_no_work_via_strand_filter`)

