# Bead bf-4gk3: Root Cause Analysis of Pluck Bead Discovery Failure

## Investigation Summary

**Date:** 2026-07-06  
**Workspace:** /home/coding/ARMOR  
**Issue:** Pluck not finding open beads despite claim of "5 existing"

## Root Cause Identified

**The original "Pluck discovery failure" was based on incorrect information. There was no configuration issue - the problem was that NO NEEDLE worker was running on the ARMOR workspace when the starvation alert was triggered.**

### Current State (Verified 2026-07-06 11:30 UTC)

**Active NEEDLE workers on ARMOR:**
- `hotel` - running on /home/coding/ARMOR (PID 1378977)
- `alpha` - running on /home/coding/ARMOR (PID 1379781)

**Open beads: 19** (not 5 as originally claimed)

### Historical Problem

When the starvation alert (bf-3b64) was created, the situation was:

1. **No NEEDLE worker assigned to ARMOR workspace**
2. **Pluck strand was configured as `auto`** - only discovers work on workspaces with active workers
3. **Starvation alert misreported bead counts** - claimed "5 open beads" but there were actually 21

## The 19 Open Beads (Current State)

**High Priority (P1):**
- bf-yxq0: Rewrite S3 key paths in all handlers using configured prefix
- bf-32ms: Wire ARMOR_PREFIX into rs-manager and cluster deployments

**Priority 2 (P2) - 17 beads:**
- bf-1daa: Dashboard: verify bucket browser UI acceptance criteria
- bf-668r: Dashboard: verify encryption status + cache statistics display  
- bf-nzm9: Epic: ARMOR web dashboard — finalize in Go, remove Rust scaffold
- bf-3b64: Starvation alert: beads invisible to worker
- bf-1loh: Investigate bead starvation root cause
- bf-17vu: Fix Pluck configuration to discover open beads
- bf-up2e: Verify bead inventory and workspace state
- bf-65nh: List and document all open beads
- bf-1hm4: Review Pluck configuration settings
- bf-43du: Test Pluck filtering logic
- bf-5g60: Extract and review Pluck configuration
- bf-431p: Identify configuration mismatch causing bead invisibility
- bf-24kz: Document root cause and required configuration fix
- bf-1cgd: Test bead
- bf-2y8s: Review Pluck configuration for filter settings
- bf-qagm: Review Pluck configuration settings
- bf-83o2: Document Pluck exclude_labels configuration

## Configuration Analysis

### NEEDLE Configuration (Global)

**File:** `/home/coding/.needle/config.yaml`

```yaml
strands:
  pluck: auto    # Primary work from the auto-discovered workspace
  explore: auto  # Look for work in other workspaces
  mend: true     # Maintenance and cleanup (always on)
```

**Key Points:**
- **No `exclude_labels` configured** - no label-based filtering
- **No workspace-specific filter rules** - no path-based filtering
- **Pluck mode is `auto`** - only processes workspaces with active workers

### ARMOR Beads Configuration

**File:** `/home/coding/ARMOR/.beads/config.yaml`

```yaml
issue_prefix: armor
default_priority: 2
default_type: task
```

**Standard configuration - no special filtering rules that would affect discovery**

## Why Beads Were "Invisible"

### Historical Cause (When Starvation Alert Was Created)

1. **No NEEDLE worker running on ARMOR workspace**
2. **Pluck's `auto` mode** only discovers beads on workspaces with active workers
3. **Result:** No bead discovery occurred, no work was claimed

### Current State (Workers Running)

With workers `hotel` and `alpha` now running on ARMOR:
- **Pluck is functioning normally**
- **Beads are discoverable** (19 open beads visible)
- **No configuration changes were needed**

## Exact Configuration Issue

**The issue was NOT a configuration parameter - it was a worker deployment gap.**

### No Filtering Issues

Verified that these settings did NOT cause the problem:
- ✅ `exclude_labels` - not configured (empty)
- ✅ Workspace path - correct (/home/coding/ARMOR)
- ✅ Filter rules - none configured
- ✅ Beads database - intact and accessible

### The Actual Problem

**Missing worker assignment:**

The NEEDLE fleet was configured with workers for:
- bead-forge (worker: "golf")
- claude-governor (workers: "cgov-1", "charlie")
- kalshi-weather (workers: "kw-1", "sierra")
- NEEDLE (workers: "echo", "tango")
- And others...

**But no worker for ARMOR** at the time the starvation alert was triggered.

## Resolution

**Applied Resolution:**
- NEEDLE workers `hotel` and `alpha` were deployed to ARMOR workspace
- Pluck is now discovering beads normally

**Alternative Resolution (if ARMOR processing not needed):**
- Simply don't assign workers to ARMOR workspace
- Accept that Pluck won't discover beads there (by design)

## Documentation Reference

Full investigation details documented in `/home/coding/ARMOR/notes/bf-4axz.md`

## Acceptance Criteria Met

✅ **Identify the exact configuration parameter causing the issue**
   - Answer: No configuration parameter - missing worker assignment

✅ **Document why the 5 open beads are being filtered out**
   - Answer: There were never 5 beads; there were 19-21 depending on when counted. The "filtering" was due to no worker running on ARMOR workspace

✅ **Propose specific configuration fix needed**
   - Answer: No configuration fix needed; workers have been deployed to ARMOR workspace

## Conclusion

The "Pluck bead discovery failure" was a **false positive** based on:
1. Incorrect bead count in starvation alert (5 vs actual 19-21)
2. Missing worker deployment to ARMOR workspace
3. Misunderstanding of Pluck's `auto` mode behavior

**Pluck configuration is correct and working as designed.** The problem was operational (missing worker), not configuration (incorrect settings).
