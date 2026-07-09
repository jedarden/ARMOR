# Pluck Debug Flags and Logging Configuration - Final Summary

## Task Completion Status: COMPLETE

**Date:** 2026-07-09  
**Bead ID:** bf-5p3g  
**Workspace:** /home/coding/ARMOR  
**NEEDLE Project:** /home/coding/NEEDLE

## Executive Summary

Comprehensive documentation for Pluck debug flags and logging configuration has been **verified and confirmed**. All necessary documentation already exists in the ARMOR repository and is accurate based on source code verification.

## Available Documentation Locations

The following comprehensive documentation files exist:

1. **`/home/coding/ARMOR/docs/pluck-debug-configuration.md`** - Primary reference document (271 lines)
2. **`/home/coding/ARMOR/notes/pluck-debug-configuration.md`** - Supplementary reference (211 lines)
3. **`/home/coding/ARMOR/notes/bf-5p3g-pluck-debug-flags.md`** - Technical deep dive (211 lines)

## Key Findings

### Primary Debug Control

**Environment Variable:** `RUST_LOG`

All Pluck debug output is controlled via the standard Rust `tracing` crate's `RUST_LOG` environment variable.

### Exact Module Path

```
needle::strand::pluck
```

This is the precise module path for filtering decision logging.

### Recommended Debug Configurations

#### Standard Filtering Debug
```bash
export RUST_LOG="needle::strand::pluck=debug"
```

#### Comprehensive Debug
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

#### Maximum Verbosity
```bash
export RUST_LOG="trace"
```

### Available Log Levels

| Level | Purpose | Verbosity |
|-------|---------|-----------|
| `error` | Errors only | Minimal |
| `warn` | Warnings and errors | Low |
| `info` | Informational messages | Medium |
| `debug` | Detailed debugging information | High |
| `trace` | Extremely detailed execution trace | Maximum |

### Filtering Decision Logging

Pluck logs detailed information about these filtering operations:

1. **Label Exclusion** - Default: `["deferred", "human", "blocked"]`
2. **Status Filtering** - Removes `in_progress` beads
3. **Assignee Filtering** - Removes Open beads with stale assignees
4. **Sorting** - `(priority ASC, created_at ASC, id ASC)`
5. **Split Triggers** - Based on `failure-count:N` labels

### Helper Script

**Location:** `/home/coding/ARMOR/capture-pluck-debug.sh`

**Usage:**
```bash
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1
```

This script automatically:
- Sets comprehensive RUST_LOG configuration
- Runs NEEDLE with specified workspace
- Captures all output to timestamped log file
- Provides analysis commands for common patterns

## Source Code Verification

**Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs` (917 lines)

Source code verification confirms:
- All debug events use `tracing::debug!()` macro
- Module path is exactly `needle::strand::pluck`
- Filtering decisions are logged at `debug` level
- Split trigger evaluation is logged at `debug` and `info` levels
- Tracing instrumentation uses `#[tracing::instrument]` macro

## Expected Debug Output Examples

### Label Filtering
```
DEBUG Label filtering excluded 3 beads
DEBUG excluded_count=3 remaining=7 excluded_labels=["deferred", "human", "blocked"]
DEBUG bead_id=bf-123 labels=["deferred"] excluded_reasons=["deferred"] Excluded bead due to labels
```

### Status/Assignee Filtering
```
DEBUG Status/assignee filtering removed 2 beads
DEBUG filtered_count=2 remaining=5
```

### Split Triggers
```
DEBUG bead_id=bf-456 failure_count=3 threshold=3 split_triggered=true
INFO Split threshold reached, returning Split instruction
```

### Candidate Selection
```
INFO count=5 candidates=["bf-123", "bf-456", "bf-789"] Returning 5 candidates for processing
```

## Analysis Commands

```bash
# Find filtering decisions
grep -i "filter" output.log
grep -i "exclude" output.log

# Find specific beads
grep "bf-123" output.log

# Find split operations
grep -i "split" output.log

# Find candidate selection
grep -i "candidate" output.log
```

## Configuration Files

Pluck behavior can be configured via:

1. **Global Config:** `~/.config/needle/config.yaml`
2. **Workspace Config:** `.needle.yaml` in workspace root
3. **Environment Variables:** `NEEDLE_STRANDS_PLUCK_EXCLUDE_LABELS`

**However**, debug logging levels are **only** controlled via `RUST_LOG` and cannot be set in config files.

## Troubleshooting Guide

### No debug output appearing

1. **Verify RUST_LOG is set:** `echo $RUST_LOG`
2. **Check stderr capture:** Ensure stderr is being captured (`2>&1`)
3. **Verify tracing subscriber:** Look for "tracing subscriber initialized" in output
4. **Check module path:** Must be exactly `needle::strand::pluck`
5. **Release claimed beads:** Use `br release <bead_id>` if worker has claimed work

### Too much output

Narrow scope to just Pluck:
```bash
export RUST_LOG="needle::strand::pluck=debug"
```

### Missing filtering decisions

Use trace level for maximum detail:
```bash
export RUST_LOG="needle::strand::pluck=trace"
```

## Related Debug Modules

```
needle::strand::debug         # All strand operations
needle::strand::pluck=debug   # Pluck strand only
needle::strand::mend=debug    # Mend strand (recovery)
needle::strand::weave=debug   # Weave strand (gap analysis)
needle::bead_store=debug      # Bead store operations
needle::worker=debug          # Worker lifecycle
needle::dispatch=debug        # Agent dispatch
needle::claim=debug           # Bead claiming
```

## Summary

✅ **Task Complete:** All Pluck debug flags and logging configuration have been identified and documented.

**Key Points:**
1. **Primary Control:** `RUST_LOG` environment variable
2. **Module Path:** `needle::strand::pluck`
3. **Documentation:** Comprehensive documentation already exists
4. **Helper Script:** `capture-pluck-debug.sh` available
5. **Source Verified:** Documentation matches actual implementation

## Next Steps

No additional documentation required. Existing documentation is comprehensive, accurate, and based on actual source code verification.

**Documentation Files:**
- `/home/coding/ARMOR/docs/pluck-debug-configuration.md` ← Primary reference
- `/home/coding/ARMOR/notes/pluck-debug-configuration.md` ← Supplementary reference
- `/home/coding/ARMOR/notes/bf-5p3g-pluck-debug-flags.md` ← Technical deep dive
- `/home/coding/ARMOR/capture-pluck-debug.sh` ← Helper script

---

**Bead ID:** bf-5p3g  
**Status:** Document verified as complete and accurate  
**Date:** 2026-07-09  
