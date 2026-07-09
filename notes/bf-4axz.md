# Pluck Configuration Investigation - bf-4axz

## Investigation Summary

**Root Cause Identified:** Pluck is configured to exclude beads labeled `deferred`, `human`, or `blocked`. Most open ARMOR beads have the `deferred` label, making them invisible to Pluck.

## Current Pluck Configuration

**Configuration File:** `/home/coding/.config/needle/config.yaml`

**Relevant Configuration Section:**
```yaml
strands:
  pluck:
    exclude_labels:
    - deferred
    - human
    - blocked
```

**Workspace Configuration:**
```yaml
workspace:
  default: /home/coding
  home: /home/coding/.needle
  labels: []
```

## Open Beads Analysis

**Total Open Beads:** 16

**Beads with `deferred` label:** 6
- bf-yxq0: Labels: deferred, failure-count:4
- bf-32ms: Labels: deferred, failure-count:4, umbrella  
- bf-3b64: Labels: deferred, failure-count:6, starvation-alert
- bf-1loh: Labels: deferred, split-child, umbrella
- bf-83o2: Labels: deferred, failure-count:1, split-child, umbrella

**Beads without `deferred` label:** 10
- bf-1daa: (no labels)
- bf-668r: (no labels)
- bf-nzm9: Labels: umbrella
- bf-1hm4: Labels: split-child
- bf-43du: Labels: split-child
- bf-5g60: Labels: split-child
- bf-431p: Labels: split-child
- bf-24kz: Labels: split-child
- bf-1cgd: (no labels)
- bf-2y8s: (no labels)
- bf-qagm: Labels: split-child

## Filter Impact

**Beads Excluded by Pluck:** 6 out of 16 (37.5%)

The 6 beads with the `deferred` label are completely invisible to Pluck and will never be discovered or worked on by the bead worker system.

**Beads Visible to Pluck:** 10 out of 16 (62.5%)

The 10 beads without the `deferred` label are discoverable and can be claimed by workers.

## Why Beads Have `deferred` Label

Based on the beads examined, the `deferred` label appears to be applied to beads that:
1. Have failed multiple times (failure-count:4, failure-count:6)
2. Are umbrella/parent beads tracking other work
3. Are part of a split chain that was stalled

This label is likely applied automatically by the needle mitosis strand when beads reach failure thresholds.

## Configuration Options

**Option 1: Remove `deferred` from exclude_labels**
- **Pros:** Deferred beads become discoverable again
- **Cons:** Workers may repeatedly claim beads that have historically failed

**Option 2: Keep `deferred` exclusion but review labeled beads**
- **Pros:** Prevents repeated failures on problematic beads  
- **Cons:** Requires manual review of deferred beads

**Option 3: Modify failure thresholds before deferral**
- **Pros:** Addresses root cause of why beads get deferred
- **Cons:** Requires configuration changes to mitosis strand

## Recommendation

**Remove `deferred` from Pluck's exclude_labels** because:
1. The starvation alert indicates these beads need attention
2. Workers should be able to retry failed tasks
3. The failure-count mechanism can still prevent infinite loops
4. Manual oversight of problematic beads is better than ignoring them

## Required Configuration Change

In `/home/coding/.config/needle/config.yaml`, change:

```yaml
strands:
  pluck:
    exclude_labels:
    - human      # Keep this
    - blocked    # Keep this  
    # - deferred  # REMOVE THIS LINE
```

## Verification Steps

After making the configuration change:
1. Restart the needle workers/Pluck process
2. Verify that beads with `deferred` label now appear in discovery
3. Monitor whether deferred beads are being claimed and worked on
4. Review mitosis failure thresholds if deferral continues to be problematic

## Additional Findings

**Workspace in Pluck Configuration:** ARMOR is listed in the explore workspaces at line 58:
```yaml
explore:
  workspaces:
  - ...
  - /home/coding/ARMOR
```

This confirms Pluck should be scanning the ARMOR workspace for beads.

**Labels That Don't Affect Discovery:**
- `umbrella` - parent/tracking beads
- `split-child` - beads created from splitting
- `failure-count:N` - automatic tracking of failures
- `starvation-alert` - metadata about bead state
