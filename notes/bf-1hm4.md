# Pluck Configuration Review - ARMOR Workspace

**Date:** 2026-07-08
**Configuration File:** `/home/coding/NEEDLE/.needle.yaml`
**Source Code:** `/home/coding/NEEDLE/src/strand/pluck.rs`
**Bead:** bf-1hm4

## Summary

**CRITICAL FINDING:** Despite `exclude_labels: []` in the configuration, **Pluck DOES filter beads using default exclusions**.

## Pluck Strand Configuration

```yaml
strands:
  pluck:
    exclude_labels: []          # Empty in YAML, but defaults apply!
    split_after_failures: 0    # No automatic splitting
```

## Filter Settings That Could Affect Bead Visibility

### 1. `exclude_labels` - **CRITICAL: Defaults Apply When Empty!**

**Configuration Value:** `[]` (empty array in YAML)

**ACTUAL BEHAVIOR** (from source code `pluck.rs:28-36`):
```rust
// Lines 12-13: Default exclude labels
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked"];

// Lines 28-36: When exclude_labels is empty, DEFAULTS are used
pub fn new(exclude_labels: Vec<String>) -> Self {
    let labels = if exclude_labels.is_empty() {
        DEFAULT_EXCLUDE_LABELS
            .iter()
            .map(|s| (*s).to_string())
            .collect()
    } else {
        exclude_labels
    };
    ...
}
```

**ACTUAL EXCLUDED LABELS** (defaults applied):
- `"deferred"` - Beads marked for later processing
- `"human"` - Beads requiring manual intervention
- `"blocked"` - Beads with unmet dependencies

**Impact:**
- ✅ Beads with these labels ARE filtered out from Pluck selection
- ✅ Empty config `[]` does NOT mean "no filtering" - it means "use defaults"
- ⚠️ To truly disable filtering, you would need to modify source code

**How to Override Defaults:**
To exclude different labels, explicitly set them in YAML:
```yaml
strands:
  pluck:
    exclude_labels: ["deferred", "waiting-approval", "wip"]
```

This would exclude ONLY those 3 labels, NOT the default `"human"` or `"blocked"`.

### 2. `split_after_failures: 0` - Auto-Split Disabled
- **Purpose:** Auto-trigger mitosis (bead splitting) after N consecutive failures
- **Current Value:** `0` (disabled)
- **Failure Tracking:** Via `failure-count:N` labels on beads
- **Impact:** No automatic bead splitting occurs

### 3. Workspace `labels: []` - No Workspace-Level Filtering
```yaml
workspace:
  default: /home/coding/NEEDLE
  home: /home/coding/.needle
  labels: []                   # No workspace-level label filtering
```
- **Purpose:** Workspace-level label filtering (separate from Pluck)
- **Current Value:** Empty list `[]`
- **Impact:** No additional filtering at workspace level

### 4. Other Strand `exclude_*` Settings

All empty:
- `strands.weave.exclude_workspaces: []` - No workspaces excluded from weave strand

## Additional Code-Level Filters (Always Applied)

### 1. Status Filtering (`pluck.rs:127-133`)
```rust
candidates.retain(|b| {
    !(matches!(b.status, crate::types::BeadStatus::InProgress)
        || (b.status == crate::types::BeadStatus::Open && b.assignee.is_some()))
});
```

**Filters out:**
- Beads with status `in_progress` (claimed by another worker)
- Open beads with stale assignee (leftover assignee from previous claim)

### 2. Dependency Filtering (via `store.ready()`)
- Only returns beads whose dependencies are all closed
- Beads blocked by unclosed dependencies are not returned

### 3. Deterministic Sorting (`pluck.rs:75-86`)
```rust
candidates.sort_by(|a, b| {
    a.priority
        .cmp(&b.priority)
        .then_with(|| a.created_at.cmp(&b.created_at))
        .then_with(|| a.id.as_ref().cmp(b.id.as_ref()))
});
```

**Sort order:** `priority ASC, created_at ASC, id ASC`

## Complete List of Settings That Affect Bead Visibility

### Configuration File Settings

| Setting | Location | Current Value | Actual Effect |
|---------|----------|---------------|---------------|
| `exclude_labels` | `strands.pluck.exclude_labels` | `[]` | **Uses defaults:** `["deferred", "human", "blocked"]` |
| `split_after_failures` | `strands.pluck.split_after_failures` | `0` | Auto-split disabled |
| `workspace.labels` | `workspace.labels` | `[]` | No workspace-level filtering |
| `exclude_workspaces` | `strands.weave.exclude_workspaces` | `[]` | No workspaces excluded from weave |

### Code-Level Defaults (Always Applied)

| Filter | Source | What's Filtered |
|--------|--------|-----------------|
| Label exclusion | `DEFAULT_EXCLUDE_LABELS` | `deferred`, `human`, `blocked` |
| Status filtering | `pluck.rs:127-133` | `in_progress`, stale assignee |
| Dependency filtering | `store.ready()` | Beads with unclosed dependencies |

## Current Workspace State

```
Total beads: 115
  Open: 15
  Blocked: 21
  In-progress: 1
  Closed: 72
  Completed: 6
```

## Labels Currently in Use

```
split-child: 47 beads
deferred: 20 beads              ← EXCLUDED by Pluck (default)
failure-count:1: 8 beads
failure-count:2: 5 beads
umbrella: 5 beads
backend: 3 beads
failure-count:4: 2 beads
b2-cap: 1 beads
deployment: 1 beads
external-repo: 1 beads
failure-count:37: 1 beads
failure-count:39: 1 beads
failure-count:6: 1 beads
failure-count:9: 1 beads
observability: 1 beads
server: 1 beads
starvation-alert: 1 beads
test: 1 beads
```

## Key Findings

### ✅ **Confirmed Settings**

1. **`exclude_labels` defaults ARE active**: Despite `[]` in config, defaults `["deferred", "human", "blocked"]` are applied
2. **20 beads with `deferred` label are filtered out** from Pluck selection
3. **No custom exclusions configured**: Empty array means "use defaults", not "no filtering"
4. **Auto-split disabled**: `split_after_failures: 0`

### ⚠️ **Potential Issues**

1. **`deferred` beads (20 total) are invisible to Pluck** - This is intentional but worth noting
2. **If workers need to see `deferred` beads**, the config must explicitly exclude only `["human", "blocked"]`
3. **Stale assignee filtering** prevents retry loops on abandoned claims

### 🔍 **Debugging Commands**

```bash
# View actual Pluck configuration (with defaults applied)
grep -A 10 "strands:" /home/coding/NEEDLE/.needle.yaml | grep -A 5 "pluck:"

# View all beads with their labels
br list --format json | jq -r '.[] | "\(.id): \(.labels | join(", "))"'

# View beads that ARE excluded (with defaults)
br list --format json | jq '[.[] | select(.labels[] | IN("deferred","human","blocked")) | .id]'

# View beads available to Pluck (not excluded, not in_progress, ready)
br list --format json | jq '[.[] | select(
  (.labels | IN("deferred","human","blocked") | not) and
  (.status != "in_progress") and
  (.assignee == null)
) | .id]'
```

## Conclusion

**Pluck configuration DOES filter beads, but using DEFAULT exclusions:**

1. ✅ **Default label filtering IS active**: `deferred`, `human`, `blocked` beads are excluded
2. ✅ **Status filtering IS active**: `in_progress` and stale-assignee beads are excluded
3. ✅ **Dependency filtering IS active**: Only ready beads (unblocked) are returned
4. ⚠️ **20 `deferred` beads are NOT visible to Pluck** - This is expected behavior
5. ✅ **No custom exclusions**: Empty config `[]` means "use defaults"

**If Pluck appears to skip beads, check:**
- Are they labeled `deferred`, `human`, or `blocked`? (excluded by default)
- Are they `in_progress` or have stale assignee? (excluded by status filter)
- Do they have unclosed dependencies? (excluded by dependency filter)
- Is the workspace path correct?

**The configuration is working as designed** - defaults provide sensible filtering to prevent workers from claiming inappropriate beads.
