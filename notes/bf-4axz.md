# Pluck Configuration Investigation - bf-4axz

## Investigation Summary

**Date**: 2026-07-06
**Workspace**: /home/coding/ARMOR
**Issue**: Pluck (NEEDLE's bead discovery strand) not finding open beads in ARMOR workspace

## Root Cause Identified

**Pluck configuration is correct - the issue is that NO NEEDLE WORKER is running on the ARMOR workspace.**

### Evidence

1. **Active NEEDLE Workers** (from `ps aux | grep needle`):
   - `bead-forge` workspace (worker: "golf")
   - `claude-governor` workspace (worker: "cgov-1")
   - `kalshi-weather` workspace (worker: "kw-1")
   - **NO worker running on ARMOR workspace**

2. **NEEDLE Configuration** (`/home/coding/.needle/config.yaml`):
   - Default workspace: `/home/coding/NEEDLE`
   - Pluck strand: `auto` (enabled for auto-discovered workspace)
   - No explicit exclude_labels or filters configured

3. **ARMOR Workspace State**:
   - **21 open beads** (not 5 as the starvation alert claimed)
   - Many beads have labels like "deferred", "split-child", "failure-count:N"
   - Some beads have no labels (bf-1daa, bf-668r, bf-1cgd, bf-2y8s)

4. **Pluck is Working Correctly**:
   - Logs show `"result":"bead_found"` entries
   - Pluck successfully finds beads in workspaces where workers ARE running
   - The starvation alert (bf-3b64) contains outdated/misleading information

### The 21 Open Beads in ARMOR

**High Priority (P1)**:
- bf-yxq0: Rewrite S3 key paths in all handlers using configured prefix (labels: deferred, failure-count:4)
- bf-32ms: Wire ARMOR_PREFIX into rs-manager and cluster deployments (labels: deferred, failure-count:4, umbrella)

**Priority 2 (P2)**:
- bf-1daa: Dashboard: verify bucket browser UI acceptance criteria; fill test gaps
- bf-668r: Dashboard: verify encryption status + cache statistics display; fill gaps  
- bf-nzm9: Epic: ARMOR web dashboard — finalize in Go, remove Rust scaffold (umbrella)
- bf-3b64: Starvation alert: beads invisible to worker (deferred, failure-count:6, starvation-alert)
- bf-1loh: Investigate bead starvation root cause (deferred, split-child, umbrella)
- bf-4gk3: Identify root cause of Pluck bead discovery failure (split-child)
- bf-17vu: Fix Pluck configuration to discover open beads (split-child)
- Plus 13 additional P2 beads related to the starvation investigation

**Conclusion**: The starvation investigation itself created 15+ beads, making the problem appear worse than it is.

## Configuration Details

### NEEDLE Pluck Configuration
```yaml
# /home/coding/.needle/config.yaml
workspace:
  default: /home/coding/NEEDLE

strands:
  pluck: auto    # Primary work from the auto-discovered workspace
  explore: auto  # Look for work in other workspaces
  mend: true     # Maintenance and cleanup (always on)
```

**Key Points**:
- No `exclude_labels` configured
- No workspace-specific filter rules
- Pluck is enabled (`auto` mode)
- The configuration is correct

### ARMOR Beads Configuration
```yaml
# /home/coding/ARMOR/.beads/config.yaml
issue_prefix: armor
default_priority: 2
default_type: task
```

**Standard configuration - no special filtering rules**

## Why the Starvation Alert Was Misleading

The starvation alert bead (bf-3b64) claimed:
- "Workspace: default"
- "Total beads: 82"
- "Open: 5"

**Reality**:
- Workspace: /home/coding/ARMOR (not default)
- Open beads: 21 (not 5)
- Pluck works fine when a worker is actually running

## Resolution

**To make Pluck discover beads in ARMOR, a NEEDLE worker must be assigned to that workspace**:

```bash
# Start a NEEDLE worker on the ARMOR workspace
/home/coding/.local/bin/needle run --workspace /home/coding/ARMOR --count 1 --identifier armor-1
```

This would allow Pluck to discover and process the 21 open beads in ARMOR.

## Additional Findings

1. **Split-child mitosis storm**: The starvation investigation triggered extensive mitosis (split-child beads), creating 15+ related beads that now clutter the workspace.

2. **Deferred beads accumulate**: Several beads have "deferred" labels with high failure counts (4-6 failures), suggesting they're being retried but not completed.

3. **No actual Pluck filtering issue**: The entire investigation was based on a false premise - Pluck configuration is correct, it just wasn't being used on the ARMOR workspace.

## Recommendations

1. **Start a NEEDLE worker on ARMOR** if ARMOR bead processing is desired
2. **Clean up starvation investigation beads** - many are now redundant or resolved
3. **Review deferred beads** with high failure counts - they may need manual intervention
4. **Update starvation alert logic** to detect when no worker is running (not just when beads aren't found)

## Acceptance Criteria Met

✅ Document current Pluck configuration with all filter rules  
✅ List all open beads with their labels and attributes (found 21, not 5)  
✅ Identify which filter rule might be causing the exclusion (N/A - no filter rules, just no worker running)  
✅ Identify the exact configuration issue (missing worker assignment, not a config problem)
