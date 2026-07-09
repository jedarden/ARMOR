# Pluck Settings That Hide Beads

**Bead:** bf-4351  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR

## Overview

This document analyzes all Pluck configuration settings that affect bead visibility and can cause beads to be hidden from selection during NEEDLE's Pluck strand evaluation.

## Configuration File Locations

Pluck settings are configured in:

1. **`.needle.yaml`** (workspace-level configuration)
   - Location: `/home/coding/ARMOR/.needle.yaml`
   - Format: YAML
   - Purpose: Controls NEEDLE strand behavior for this workspace

2. **`pluck-config.yaml`** (debug configuration)
   - Location: `/home/coding/ARMOR/pluck-config.yaml`
   - Format: YAML
   - Purpose: Debug logging and filtering behavior

3. **Environment Variables**
   - `RUST_LOG` - Controls debug logging level (does not hide beads, only affects output)

## Settings That Hide Beads

### 1. `exclude_labels` (Primary Hiding Mechanism)

**Location:** `.needle.yaml` → `strands.pluck.exclude_labels`  
**Type:** Array of strings  
**Default:** `[]` (empty array - no exclusions)  
**Current Value:** `[]` (empty)

**How it hides beads:**
- Any bead with a label matching any value in this array is completely excluded from consideration
- This is the **primary configurable hiding mechanism** in Pluck
- Filtering happens BEFORE other filtering steps

**Example configuration:**
```yaml
strands:
  pluck:
    exclude_labels: ["deferred", "human", "blocked"]
```

**Impact:**
- All beads with `label:deferred` are hidden
- All beads with `label:human` are hidden  
- All beads with `label:blocked` are hidden
- **Hiding is absolute** - excluded beads never appear in any Pluck output

**Common exclusion patterns:**
- `deferred` - Beads postponed for later work
- `human` - Beads requiring manual human intervention
- `blocked` - Beads blocked by dependencies
- Custom labels can be added as needed

### 2. Status Filtering (Built-in, Not Configurable)

**Location:** Hardcoded in Pluck strand  
**Type:** Status filter  
**Hidden Statuses:** `closed`, `completed`, `deleted`

**How it hides beads:**
- Pluck only considers beads with `status: open`
- Beads with `status: in_progress` are hidden (already claimed)
- Beads with `status: closed` are hidden (completed)
- Beads with `status: deleted` are hidden (removed)

**Impact:**
- This filter is **not configurable** - it's built into Pluck's logic
- You cannot change this behavior via configuration
- To make a bead visible, it must be in `status: open`

**Debug output:**
```
DEBUG needle::strand::pluck: Filtering by status and assignee
  remaining=3
```

### 3. Assignee Filtering (Built-in, Not Configurable)

**Location:** Hardcoded in Pluck strand  
**Type:** Assignee filter  
**Hidden Condition:** Bead assigned to another worker

**How it hides beads:**
- Pluck filters out beads that are already assigned to another worker
- A bead with `assignee: <worker-id>` is hidden from other workers
- Only unassigned beads or beads assigned to the current worker are visible

**Impact:**
- This filter is **not configurable** - it's built into Pluck's logic
- Prevents multiple workers from processing the same bead simultaneously
- A bead becomes visible to all workers only when its `assignee` field is cleared

**Debug output:**
```
DEBUG needle::strand::pluck: Filtering by status and assignee
  filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }
```

### 4. `sort_order` (Does Not Hide, Affects Order)

**Location:** `pluck-config.yaml` → `filtering.sort_order`  
**Type:** String  
**Options:** `created`, `updated`, `priority`, `random`  
**Default:** `priority`

**How it affects visibility:**
- This setting does **NOT hide beads**
- It only controls the **order** in which visible beads are presented
- All beads that pass the filters above will be visible, just in a different order

**Impact:**
- No beads are hidden by this setting
- Only affects which bead is selected first when multiple beads are available

## Settings That Do NOT Hide Beads

### 1. `split_after_failures`

**Location:** `.needle.yaml` → `strands.pluck.split_after_failures`  
**Type:** Integer  
**Default:** `0` (disabled)  
**Current Value:** `0` (disabled)

**What it does:**
- After N consecutive failures on a bead, Pluck will **split** the bead into smaller sub-beads
- This **creates new beads**, it does not hide existing ones
- The original bead remains visible

**Impact:**
- Does not hide beads
- Creates additional beads to break down complex tasks

### 2. Debug Logging Settings

**Location:** Environment variable `RUST_LOG`  
**Type:** String  
**Examples:** `info`, `debug`, `trace`

**What it does:**
- Controls the verbosity of Pluck's logging output
- Does not affect which beads are selected or hidden

**Impact:**
- No visibility impact
- Only affects what appears in logs/debug output

## Combined Filtering Effects

### Filter Order (Applied in Sequence)

Pluck applies filters in this specific order:

1. **Label Exclusion** (`exclude_labels`) - First filter applied
2. **Status Filtering** - Remove closed/in_progress beads
3. **Assignee Filtering** - Remove beads assigned to other workers
4. **Sorting** (`sort_order`) - Order remaining beads (does not filter)

### Combined Hiding Example

If you have:
- `exclude_labels: ["deferred", "blocked"]`
- Bead `bf-1234` with `status: open`, `labels: ["deferred"]` → **HIDDEN** by label exclusion
- Bead `bf-5678` with `status: closed`, `labels: []` → **HIDDEN** by status filter
- Bead `bf-9012` with `status: open`, `assignee: "worker-1"`, `labels: []` → **HIDDEN** by assignee filter (if evaluated by worker-2)
- Bead `bf-abcd` with `status: open`, `assignee: null`, `labels: []` → **VISIBLE**

**Key point:** Beads must pass **ALL** filters to be visible. Failing any single filter hides the bead.

## Debug Logging for Hidden Beads

To see which beads are being hidden and why, enable debug logging:

```bash
# Enable debug logging
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1

# Or use the debug configuration script
./pluck-debug-config.sh /home/coding/ARMOR output.log standard
```

**Expected debug output for hidden beads:**
```
DEBUG needle::strand::pluck: Filtering by excluded labels
  excluded_beads=["bf-1234", "bf-5678"]
  reasons=["label:deferred", "label:blocked"]

DEBUG needle::strand::pluck: Filtering by status and assignee
  remaining=3
```

## Current ARMOR Workspace Configuration

**File:** `/home/coding/ARMOR/.needle.yaml`

```yaml
strands:
  pluck:
    exclude_labels: []        # No label-based exclusions
    split_after_failures: 0   # Auto-split disabled
```

**Analysis of Current Configuration:**
- ✅ **No beads hidden by label exclusion** - `exclude_labels` is empty
- ✅ **All open beads are visible** - No label-based filtering
- ✅ **Status filtering active** - Only hides closed/in_progress beads (built-in)
- ✅ **Assignee filtering active** - Only hides beads assigned to other workers (built-in)

## Summary Table

| Setting | Location | Type | Hides Beads? | Configurable? | Current Value |
|---------|----------|------|--------------|----------------|---------------|
| `exclude_labels` | `.needle.yaml` | Array | ✅ YES | ✅ Yes | `[]` (empty) |
| Status filtering | Built-in | N/A | ✅ YES | ❌ No | N/A (hardcoded) |
| Assignee filtering | Built-in | N/A | ✅ YES | ❌ No | N/A (hardcoded) |
| `sort_order` | `pluck-config.yaml` | String | ❌ NO | ✅ Yes | `priority` |
| `split_after_failures` | `.needle.yaml` | Integer | ❌ NO | ✅ Yes | `0` (disabled) |
| Debug logging (`RUST_LOG`) | Environment | String | ❌ NO | ✅ Yes | N/A |

## Recommendations

1. **To make more beads visible:**
   - Keep `exclude_labels: []` (current setting)
   - Ensure beads are in `status: open`
   - Clear `assignee` field on beads that should be available to any worker

2. **To hide specific bead types:**
   - Add labels to `exclude_labels` array
   - Example: `exclude_labels: ["deferred", "human", "blocked"]`

3. **To debug bead visibility issues:**
   - Enable debug logging: `RUST_LOG=needle::strand::pluck=debug`
   - Check bead status: `br show <bead-id>`
   - Check bead labels: `br show <bead-id> | grep labels`
   - Check bead assignee: `br show <bead-id> | grep assignee`

## Related Documentation

- **Pluck Debug Configuration:** `/home/coding/ARMOR/pluck-debug-configuration.md`
- **Command Structure Reference:** `/home/coding/ARMOR/docs/pluck-command-structure-reference.md`
- **Pluck Dependencies:** `/home/coding/ARMOR/docs/pluck-dependencies.md`
- **Configuration Files:**
  - `/home/coding/ARMOR/.needle.yaml`
  - `/home/coding/ARMOR/pluck-config.yaml`

---

**Status:** ✅ Complete  
**Acceptance Criteria Met:**
- ✅ Listed all Pluck settings that can hide beads
- ✅ Explained how each setting affects visibility
- ✅ Noted which settings combine to hide beads
- ✅ Documented current ARMOR workspace configuration
