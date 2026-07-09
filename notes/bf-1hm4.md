# Pluck Configuration Review - ARMOR Workspace

**Date:** 2026-07-08  
**Configuration File:** `/home/coding/NEEDLE/.needle.yaml`  
**Bead:** bf-1hm4

## Summary

Pluck configuration has **NO filter settings** that would hide beads. All `exclude_*` settings are empty lists.

## Pluck Strand Configuration

```yaml
strands:
  pluck:
    exclude_labels: []          # No labels excluded
    split_after_failures: 0    # No automatic splitting
```

## Filter Settings That Could Affect Bead Visibility

### 1. `exclude_labels: []` (EMPTY)
- **Purpose:** Labels that Pluck should skip when claiming beads
- **Current Value:** Empty list `[]`
- **Impact:** **NO labels are filtered** - all beads are visible regardless of labels
- **Beads that would be affected if set:** Any bead with the excluded labels

### 2. Workspace `labels: []` (EMPTY)
```yaml
workspace:
  default: /home/coding/NEEDLE
  home: /home/coding/.needle
  labels: []                   # No workspace-level label filtering
```
- **Purpose:** Workspace-level label filtering
- **Current Value:** Empty list `[]`
- **Impact:** **NO filtering at workspace level**

### 3. Other `exclude_*` Settings

All empty:
- `strands.weave.exclude_workspaces: []` - No workspaces excluded from weave strand

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
deferred: 20 beads
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

## Key Finding

**Pluck configuration is NOT hiding any beads.** The `exclude_labels` setting is empty, meaning:
- All 15 open beads are visible to Pluck
- No labels are being filtered
- Beads with `deferred`, `split-child`, or `failure-count:*` labels are NOT excluded

If Pluck cannot find beads, the issue is **NOT** in the filter configuration.

## Conclusion

No filter settings are causing beads to be hidden. The root cause must be elsewhere:
- Check workspace path resolution
- Check if beads are actually "ready" (not blocked by dependencies)
- Verify Pluck is querying the correct workspace directory
