# Pluck Debug Flags and Logging Configuration - Complete Guide

## Executive Summary

Pluck is a strand within the NEEDLE system that handles primary bead selection from workspaces. All debug logging is controlled via the `RUST_LOG` environment variable using the standard Rust `tracing` crate syntax.

## Primary Environment Variable: RUST_LOG

**Syntax**: `RUST_LOG=<module>=<level>,<module>=<level>,...`

**Available Log Levels** (in order of verbosity):
- `error` - Errors only
- `warn` - Warnings and errors  
- `info` - High-level operational events
- `debug` - Detailed operations and decisions
- `trace` - Most detailed - all operations including per-item decisions

## Pluck-Specific Debug Targets

### Pluck Strand Module
```
needle::strand::pluck=trace     # Most detailed - all filtering decisions per bead
needle::strand::pluck=debug     # Standard debugging - aggregate filtering stats
needle::strand::pluck=info      # High-level operations only
```

### All Available Strands
```
needle::strand::pluck     # Primary bead selection
needle::strand::mend      # Recovery strand
needle::strand::explore   # Exploration strand
needle::strand::weave     # Gap analysis strand
needle::strand::unravel   # Dependency analysis strand
needle::strand::pulse     # Health check strand
needle::strand::reflect   # Learning consolidation strand
needle::strand::splice    # Integration strand
needle::strand::knot      # Dependency resolution strand
```

## All Available NEEDLE Modules

### Core Modules
```
needle::dispatch          # Agent dispatch operations
needle::worker            # Worker lifecycle and state transitions
needle::telemetry         # Telemetry event logging
needle::bead_store        # Bead store queries and operations
needle::health            # Health checks and heartbeats
needle::learning          # Learning system operations
needle::sanitize          # Secret scanning and sanitization
```

### Strand-Level Modules
```
needle::strand            # All strand operations
needle::strand::pluck     # Individual strand debugging
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

### All Strand Operations
```bash
export RUST_LOG="needle::strand=debug"
```

### Full Workspace Debugging
```bash
export RUST_LOG="debug"
```

### Minimal Pluck Info
```bash
export RUST_LOG="needle::strand::pluck=info"
```

## What Gets Logged at Each Level

### needle::strand::pluck=trace (Most Detailed)
- **Initial strand evaluation**: Exact parameters (exclude_labels, split_threshold)
- **Bead store queries**: Exact filters being applied
- **Per-bead filtering decisions**: Individual bead IDs with labels and exclusion reasons
- **Label filtering**: Each bead excluded with specific labels that matched
- **Status/assignee filtering**: Per-bead status checks and assignee filtering
- **Split trigger checking**: Detailed failure count analysis per bead
- **Candidate sorting**: Sorting operations and final ordering

Example output:
```
DEBUG needle::strand::pluck: Pluck strand evaluation starting exclude_labels=["deferred", "human", "blocked"] split_threshold=3
DEBUG needle::strand::pluck: Querying bead store for ready candidates filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }
DEBUG needle::strand::pluck: bead_id=bf-123 labels=["deferred"] excluded_reasons=["deferred"] Excluded bead due to labels
DEBUG needle::strand::pluck: bead_id=bf-456 failure_count=3 threshold=3 split_triggered=true
```

### needle::strand::pluck=debug (Standard Debugging)
- **Aggregate filtering statistics**: "X beads excluded by labels"
- **Remaining counts**: Candidate counts after each filter stage
- **Split triggers**: Threshold checks and split instruction generation
- **Final candidate lists**: Complete list of selected bead IDs

Example output:
```
DEBUG needle::strand::pluck: Label filtering excluded 3 beads
DEBUG needle::strand::pluck: excluded_count=3 remaining=7 excluded_labels=["deferred", "human", "blocked"]
DEBUG needle::strand::pluck: Split threshold reached, returning Split instruction
INFO needle::strand::pluck: count=5 candidates=["bf-123", "bf-456"] Returning 5 candidates
```

### needle::strand::pluck=info (High-Level Only)
- **Strand start/completion**: High-level lifecycle events
- **Final candidate counts**: Summary statistics
- **Split instruction triggers**: When splits are returned

Example output:
```
INFO needle::worker: worker booted worker=alpha strands=["pluck", "mend", "explore", ...]
INFO needle::strand::pluck: Split threshold reached, returning Split instruction
```

## Key Logging Events and Patterns

### Filtering Decision Logging
```
# Label filtering
DEBUG needle::strand::pluck: Label filtering excluded 3 beads
DEBUG needle::strand::pluck: excluded_count=3 remaining=7 excluded_labels=["deferred", "human", "blocked"]

# Per-bead exclusion (trace level only)
DEBUG needle::strand::pluck: bead_id=bf-123 labels=["deferred"] excluded_reasons=["deferred"] Excluded bead due to labels

# Status/assignee filtering
DEBUG needle::strand::pluck: Status/assignee filtering removed 2 beads
DEBUG needle::strand::pluck: filtered_count=2 remaining=5
```

### Split Trigger Logging
```
DEBUG needle::strand::pluck: bead_id=bf-456 failure_count=3 threshold=3 split_triggered=true
INFO needle::strand::pluck: Split threshold reached, returning Split instruction
```

### Candidate Selection Logging
```
INFO needle::strand::pluck: count=5 candidates=["bf-123", "bf-456", "bf-789"] Returning 5 candidates for processing
```

### Error Logging
```
ERROR needle::strand::pluck: Bead store query failed error=bf list failed
WARN needle::strand: strand error, continuing to next strand strand=pluck error=bead store error: bf list failed elapsed_ms=2
```

## Pluck Filtering Mechanism

Based on code analysis, Pluck applies filters in this order:

1. **Bead Store Query** - Initial query with basic filters
   - Filters: `assignee=None`, `exclude_labels=[deferred, human, blocked]`
   - Returns: Ready, unassigned beads

2. **Label Filtering** - Defensive second pass
   - Removes beads with excluded labels (in case store query was incomplete)
   - Configurable via `exclude_labels`

3. **Status/Assignee Filtering** - Remove in-progress beads
   - Filters out beads with status `in_progress` or `executing`
   - Removes stale assignments (timeout-based)

4. **Sorting** - Deterministic ordering
   - Sort order: `(priority ASC, created_at ASC, id ASC)`
   - Ensures consistent selection across runs

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

## Configuration Methods

### Pluck Configuration (Not Debug Levels)
Pluck's `exclude_labels` and `split_threshold` can be configured via:

1. **Global Config**: `~/.config/needle/config.yaml`
   ```yaml
   strands:
     pluck:
       exclude_labels:
         - deferred
         - human
         - blocked
       split_threshold: 3
   ```

2. **Workspace Config**: `.needle.yaml` in workspace root
   ```yaml
   strands:
     pluck:
       exclude_labels:
         - testing
       split_threshold: 5
   ```

### Debug Logging Configuration (RUST_LOG Only)
**Important**: Debug logging levels cannot be set in config files - they are **only** controlled via the `RUST_LOG` environment variable at runtime.

## Helper Script: capture-pluck-debug.sh

A helper script exists for comprehensive debug capture:

```bash
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1
```

**What the script does:**
1. Sets comprehensive RUST_LOG configuration
2. Runs NEEDLE with the specified workspace and count
3. Captures all output to a timestamped log file
4. Provides analysis grep commands for common patterns

**Script location**: `/home/coding/ARMOR/capture-pluck-debug.sh`

## Analyzing Debug Output

### Common Analysis Patterns

**Find filtering decisions:**
```bash
grep -i "filter" output.log
grep -i "exclude" output.log
```

**Find specific beads:**
```bash
grep "bf-123" output.log
```

**Find split operations:**
```bash
grep -i "split" output.log
```

**Find candidate selection:**
```bash
grep -i "candidate" output.log
```

**Find errors:**
```bash
grep -i "error\|failed" output.log
```

## NEEDLE Logs Command

The `needle logs` command provides powerful filtering for telemetry logs:

```bash
# View recent Pluck-related events
needle logs --filter 'event_type~strand.*'

# Follow logs in real-time
needle logs --follow

# Filter by specific event types
needle logs --filter 'event_type=strand.started'

# Time-based filtering
needle logs --since 1h --filter 'event_type~pluck'
```

## Common Debug Scenarios

### Scenario: "Why are beads being excluded?"

**Set**: `RUST_LOG=needle::strand::pluck=debug`

**Look for**:
- "Label filtering excluded" messages
- "Excluded bead due to labels" with bead IDs and specific labels (at trace level)
- Check if `exclude_labels` match what you expect

**Example**:
```bash
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -i "exclude"
```

### Scenario: "Why is split triggering?"

**Set**: `RUST_LOG=needle::strand::pluck=debug`

**Look for**:
- "Split threshold reached" messages
- Check `failure_count` vs `threshold` values
- Look for "failure-count:N" labels on beads

**Example**:
```bash
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -i "split"
```

### Scenario: "Why no candidates returned?"

**Set**: `RUST_LOG=needle::strand::pluck=debug`

**Look for**:
- "No candidates remaining after filtering"
- Track how many beads removed at each filtering stage
- Check if all beads are excluded by labels, status, or assignee

**Example**:
```bash
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -E "remaining|candidate|filtering"
```

### Scenario: "Bead store query failed"

**Set**: `RUST_LOG=needle::strand::pluck=debug,needle::bead_store=debug`

**Look for**:
- "Bead store query failed" errors
- Bead store connection issues
- Check bead store health

**Example**:
```bash
export RUST_LOG="needle::strand::pluck=debug,needle::bead_store=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -E "bead.*store|query"
```

## Environment Variable Reference

### Primary Variables
```bash
RUST_LOG=needle::strand::pluck=debug    # Main debug control
```

### Secondary Variables (observed in environment)
```bash
RUST_TEST_THREADS=2                      # Rust test threading
RUSTFLAGS=-C codegen-units=1            # Rust compilation flags
NEEDLE_INNER=1                           # NEEDLE internal flag
```

## Code Architecture Reference

### Key Locations
- **Pluck implementation**: `~/.local/bin/needle` (binary)
- **Tracing subscriber**: Initialized in NEEDLE binary at startup
- **Workspace config**: `.needle.yaml` (if present)
- **Global config**: `~/.config/needle/config.yaml`

### Tracing Architecture
NEEDLE uses the `tracing` crate (not `log`) for structured logging:
- `tracing::debug!()` - Detailed operations
- `tracing::info!()` - High-level events
- `tracing::error!()` - Error conditions
- `tracing::warn!()` - Warning conditions
- `tracing::instrument` macro - Span-based tracing

## Summary Table

| Module Target | Purpose | Recommended Level |
|---------------|---------|------------------|
| `needle::strand::pluck` | Pluck strand filtering | `debug` (standard), `trace` (detailed) |
| `needle::strand` | All strand operations | `debug` |
| `needle::bead_store` | Bead store queries | `debug` (when debugging bead queries) |
| `needle::worker` | Worker lifecycle | `debug` (when debugging worker issues) |
| `needle::dispatch` | Agent dispatch | `debug` (when debugging dispatch issues) |
| `needle::telemetry` | Telemetry events | `debug` (rarely needed) |
| `needle::health` | Health checks | `info` (usually sufficient) |

## Quick Reference Card

```bash
# Quick Pluck debug
export RUST_LOG="needle::strand::pluck=debug"

# Comprehensive Pluck debug
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug"

# Full NEEDLE debug
export RUST_LOG="debug"

# Run with debug
needle run -w /home/coding/ARMOR -c 1

# Use helper script
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1

# Analyze output
grep -i "pluck\|filter\|exclude" output.log
```

## Version Information

- **NEEDLE Version**: 0.2.11 (as of 2026-07-09)
- **Tracing Crate**: Standard Rust `tracing` ecosystem
- **Log Levels**: error, warn, info, debug, trace

---

**Document Created**: 2026-07-09  
**Bead ID**: bf-5p3g  
**Purpose**: Comprehensive guide to Pluck debug flags and logging configuration