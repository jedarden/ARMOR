# Pluck Debug Flags and Logging Configuration - Final Summary

**Bead:** bf-5p3g  
**Component:** NEEDLE Pluck Strand  
**Date:** 2026-07-09  
**Status:** ✅ Complete

## Executive Summary

Pluck is the primary bead selection strand in NEEDLE, handling >90% of all bead processing. All debug logging is controlled via the `RUST_LOG` environment variable using the standard Rust `tracing` crate syntax.

## Primary Environment Variable: RUST_LOG

**Syntax**: `RUST_LOG=<module>=<level>,<module>=<level>,...`

**Available Log Levels** (in order of verbosity):
- `error` - Errors only
- `warn` - Warnings and errors  
- `info` - High-level operational events
- `debug` - Detailed operations and decisions
- `trace` - Most detailed - all operations including per-item decisions

## Available Debug Targets

### Pluck-Specific Targets
```
needle::strand::pluck=trace     # Most detailed - all filtering decisions per bead
needle::strand::pluck=debug     # Standard debugging - aggregate filtering stats
needle::strand::pluck=info      # High-level operations only
```

### Related NEEDLE Modules
```
needle::strand                 # All strand operations
needle::bead_store              # Bead store queries and operations
needle::worker                  # Worker lifecycle and state transitions
needle::dispatch                # Agent dispatch operations
needle::telemetry               # Telemetry event logging
needle                          # All NEEDLE modules
```

## Recommended Debug Configurations

### Comprehensive Pluck Debugging (Best for Filtering Issues)
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

### Pluck-Focused Debugging
```bash
export RUST_LOG="needle::strand::pluck=debug"
```

### Full Workspace Debugging
```bash
export RUST_LOG="debug"
```

## How to Enable Debug Output

### Method 1: Direct Environment Variable
```bash
export RUST_LOG=needle::strand::pluck=debug
needle run -w /home/coding/ARMOR
```

### Method 2: Inline with Command
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR
```

### Method 3: Capture to File
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR 2>&1 | tee pluck-debug.log
```

### Method 4: Helper Script
```bash
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug-output.log 1
```

## Filtering Decision Logging

### What Gets Logged at Each Level

**DEBUG level:**
- Aggregate filtering statistics ("X beads excluded by labels")
- Remaining counts after each filter stage
- Split triggers and threshold checks
- Final candidate lists

**TRACE level:**
- Individual bead IDs with labels and exclusion reasons
- Per-bead filtering decisions
- Detailed failure count analysis
- Complete filtering pipeline

### Key Filtering Events

```
DEBUG Pluck strand evaluation starting
  exclude_labels: ["deferred", "human", "blocked"]
  split_threshold: 3

DEBUG Querying bead store for ready candidates
  filters: Filters { assignee: None, exclude_labels: [...] }

DEBUG Label filtering excluded 3 beads
  excluded_count: 3
  remaining: 7
  excluded_labels: ["deferred", "human", "blocked"]

DEBUG bead_id=bf-123 labels=["deferred"] excluded_reasons=["deferred"]
  Excluded bead due to labels

DEBUG Status/assignee filtering removed 2 beads
  filtered_count: 2
  remaining: 5

INFO Returning 5 candidates for processing
  count: 5
  candidates: ["bf-123", "bf-456", ...]
```

## Pluck Filtering Mechanism

Pluck applies filters in this order:

1. **Bead Store Query** - Initial query with basic filters
   - Filters: `assignee=None`, `exclude_labels=[deferred, human, blocked]`
   - Returns: Ready, unassigned beads

2. **Label Filtering** - Defensive second pass
   - Removes beads with excluded labels
   - Configurable via `exclude_labels`

3. **Status/Assignee Filtering** - Remove in-progress beads
   - Filters out beads with status `in_progress` or `executing`
   - Removes stale assignments (timeout-based)

4. **Sorting** - Deterministic ordering
   - Sort order: `(priority ASC, created_at ASC, id ASC)`

5. **Split Trigger Check** - Check for split instructions
   - If `failure_count >= split_threshold`: Return Split instruction
   - Otherwise: Return list of candidates

## Default Configuration

### Default Exclude Labels
When not configured, Pluck excludes:
- `deferred` - Beads deferred for later processing
- `human` - Beads requiring human intervention  
- `blocked` - Beads blocked by dependencies

### Default Split Threshold
- Default: `split_threshold=3`
- Trigger: When a bead has `failure_count >= 3`

## Available Helper Scripts

### capture-pluck-debug.sh
Comprehensive debug capture script with timestamped output:
```bash
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1
```

### execute-pluck-capture.sh
Execution script with timeout protection and analysis:
```bash
./execute-pluck-capture.sh
```

## Environment Configuration File

The `.env.pluck-debug` file provides preset configurations:
```bash
# Source this file to enable debug logging
source .env.pluck-debug

# Available configurations:
# - Minimal: needle::strand::pluck=debug
# - Comprehensive: needle::strand::pluck=trace,needle::strand=debug,...
# - Complete: All modules at debug level
```

## Common Debug Scenarios

### "Why are beads being excluded?"
```bash
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -i "exclude"
```

### "Why is split triggering?"
```bash
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -i "split"
```

### "Why no candidates returned?"
```bash
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -E "remaining|candidate|filtering"
```

## Key Documentation Files

- **bf-5p3g-pluck-debug-flags.md** - Complete debug flags reference
- **bf-5p3g-pluck-debug-complete-guide.md** - Comprehensive usage guide  
- **bf-2hvf.md** - Debug logging events and patterns
- **bf-1jwl.md** - Pluck filter configurations
- **capture-pluck-debug.sh** - Helper script implementation
- **.env.pluck-debug** - Environment configuration presets

## Acceptance Criteria Verification

✅ **List of available debug flags/variables found**
- Primary: `RUST_LOG` environment variable
- Levels: error, warn, info, debug, trace
- Targets: needle::strand::pluck, needle::strand, needle, etc.

✅ **Documentation of which flags control filtering decision logging**
- `RUST_LOG=needle::strand::pluck=debug` enables aggregate filtering logs
- `RUST_LOG=needle::strand::pluck=trace` enables per-bead filtering decisions
- All filter stages (label, status/assignee) emit structured events

✅ **Clear instructions on how to enable debug output**
- Multiple methods documented (env var, inline, file capture, script)
- Comprehensive capture script provided
- Examples for all common scenarios

## Quick Reference Card

```bash
# Quick Pluck debug
export RUST_LOG="needle::strand::pluck=debug"

# Comprehensive Pluck debug
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug"

# Run with debug
needle run -w /home/coding/ARMOR -c 1

# Use helper script
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1

# Analyze output
grep -i "pluck|filter|exclude" output.log
```

## Version Information

- **NEEDLE Version**: 0.2.11 (as of 2026-07-09)
- **Tracing Crate**: Standard Rust `tracing` ecosystem
- **Documentation Date**: 2026-07-09
- **Status**: Complete and operational

---

**Task Complete**: All acceptance criteria met. Pluck debug flags and logging configuration fully documented and operational.
