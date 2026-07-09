# Pluck Debug Logging Configuration Guide

**Bead:** bf-3b63
**Task:** Configure Pluck for debug logging output
**Date:** 2026-07-09
**Component:** NEEDLE Pluck Strand

## Overview

This document provides a complete configuration guide for enabling and using Pluck debug logging to capture filtering decisions and candidate selection processes.

## Quick Start Configuration

### For Filtering Decision Debugging (Recommended)

```bash
# Set comprehensive debug logging
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

# Run NEEDLE with debug output
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-output.log
```

### For Maximum Detail

```bash
# Enable trace-level logging for all components
export RUST_LOG="trace"

# Run NEEDLE
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-trace-output.log
```

### For Pluck-Specific Debugging

```bash
# Enable debug logging for Pluck strand only
export RUST_LOG="needle::strand::pluck=debug"

# Run NEEDLE
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-specific-debug.log
```

## Debug Levels Explained

The `RUST_LOG` environment variable controls logging verbosity using standard Rust tracing levels:

| Level | Verbosity | Use Case |
|-------|-----------|----------|
| `error` | Minimal | Only errors and failures |
| `warn` | Low | Warnings and errors |
| `info` | Medium | High-level operations (default) |
| `debug` | High | Detailed operations and decisions |
| `trace` | Maximum | All details including function entry/exit |

## Module Paths for Pluck Debugging

### Primary Pluck Module
- `needle::strand::pluck` - **Core Pluck strand operations**

### Supporting Modules
- `needle::strand` - General strand operations
- `needle::bead_store` - Bead store queries and operations
- `needle::worker` - Worker state machine and processing
- `needle::dispatch` - Agent dispatch and execution
- `needle::claim` - Bead claiming operations

## Configuration Examples

### Comprehensive Configuration (All Filtering Decisions)

```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

**What this captures:**
- Individual bead filtering decisions with labels
- Detailed candidate selection process
- Split trigger evaluation
- Worker state transitions
- Bead claiming operations

### Focused Configuration (Filtering Focus Only)

```bash
export RUST_LOG="needle::strand::pluck=debug,needle::bead_store=debug"
```

**What this captures:**
- Filtering statistics and decisions
- Candidate counts after each filter
- Bead store query details

### Minimal Configuration (High-Level Only)

```bash
export RUST_LOG="needle::strand::pluck=info"
```

**What this captures:**
- Strand start/completion
- Final candidate counts
- Split instruction triggers

## Filtering Decision Logging

Pluck logs the following filtering decisions at debug level:

### 1. Label Filtering
```
DEBUG Label filtering excluded 3 beads
DEBUG excluded_count=3 remaining=7 excluded_labels=["deferred", "human", "blocked"]
DEBUG bead_id=bf-123 labels=["deferred"] excluded_reasons=["deferred"] Excluded bead due to labels
```

### 2. Status/Assignee Filtering
```
DEBUG Status/assignee filtering removed 2 beads
DEBUG filtered_count=2 remaining=5
DEBUG No beads excluded by status/assignee filter
```

### 3. Candidate Sorting
```
DEBUG Sorting 5 candidates by (priority ASC, created_at ASC, id ASC)
DEBUG total=5 first_bead_id=bf-456 first_priority=1 first_created_at=2026-07-09T00:00:00Z
```

### 4. Split Trigger Evaluation
```
DEBUG Checking split trigger for first candidate
DEBUG bead_id=bf-456 failure_count=3 threshold=3 split_triggered=true
INFO Split threshold reached, returning Split instruction
```

### 5. Final Results
```
DEBUG Returning 5 candidates for processing
INFO candidates=["bf-456", "bf-789", "bf-123"] count=3
```

## Using the Capture Script

A helper script is available for automated debug capture:

```bash
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1
```

**Script features:**
- Automatically sets comprehensive RUST_LOG configuration
- Captures all output to timestamped log file
- Provides grep commands for analysis
- Supports workspace targeting and count specification

## Analyzing Debug Output

### Find Filtering Decisions
```bash
grep -i "filter" pluck-debug-output.log
grep -i "exclude" pluck-debug-output.log
```

### Find Specific Beads
```bash
grep "bf-123" pluck-debug-output.log
```

### Find Split Operations
```bash
grep -i "split" pluck-debug-output.log
```

### Find Candidate Selection
```bash
grep -i "candidate" pluck-debug-output.log
```

### Find Label Exclusions
```bash
grep "Excluded bead due to labels" pluck-debug-output.log
```

## Pluck Filtering Process

Based on source code analysis, Pluck applies these filters in order:

1. **Bead Store Query** - Queries ready, unassigned beads with exclude_labels filter
2. **Label Filtering** - Defensive second pass to remove excluded-label beads
3. **Status/Assignee Filtering** - Removes in-progress and stale-assignee beads
4. **Sorting** - Orders by `(priority ASC, created_at ASC, id ASC)`
5. **Split Trigger Check** - Returns Split instruction if failure count >= threshold

## Default Configuration

### Default Exclude Labels
When not configured, Pluck excludes these labels:
- `deferred` - Beads deferred for later processing
- `human` - Beads requiring human intervention
- `blocked` - Beads blocked by dependencies

### Default Split Threshold
- **Default:** 3 failures
- **Disabled:** Set to 0

## Configuration Files

### Global Configuration
`~/.config/needle/config.yaml`

### Workspace Configuration
`.needle.yaml` in workspace root

```yaml
strands:
  pluck:
    exclude_labels:
      - deferred
      - human
      - blocked
    split_threshold: 3
```

## Tracing Architecture

NEEDLE uses the `tracing` crate for structured logging:
- `tracing::debug!()` - Detailed operations
- `tracing::info!()` - High-level events
- `tracing::error!()` - Error conditions
- `tracing::instrument` macro - Span-based tracing

## Debug Output Format

Debug output uses structured logging format:

```
2026-07-09T00:32:15.123Z DEBUG needle::strand::pluck:pluck_strand: Pluck strand evaluation starting
  exclude_labels=["deferred", "human", "blocked"] split_threshold=3
```

## Troubleshooting

### No debug output appearing

1. Verify RUST_LOG is set: `echo $RUST_LOG`
2. Check stderr is being captured: `needle run ... 2>&1 | tee output.log`
3. Ensure tracing subscriber is initialized (automatic in needle CLI)

### Too much output

Narrow the scope to just the Pluck strand:
```bash
export RUST_LOG="needle::strand::pluck=debug"
```

### Missing filtering decisions

Ensure trace level is enabled for detailed filtering logic:
```bash
export RUST_LOG="needle::strand::pluck=trace"
```

## Available Debug Modules

Beyond Pluck, these modules can be debugged:

```
needle::strand::pluck      # Pluck strand (primary selection)
needle::strand::mend       # Mend strand (recovery)
needle::strand::weave      # Weave strand (gap analysis)
needle::strand::explore    # Explore strand (investigation)
needle::strand::unravel    # Unravel strand (debugging)
needle::bead_store         # Bead store operations
needle::worker             # Worker lifecycle
needle::dispatch           # Agent dispatch
needle::claim              # Bead claiming
```

## Key Debug Events

### At TRACE level:
- Initial strand evaluation parameters
- Exact bead store filters being applied
- Per-bead filtering decisions with labels
- Individual excluded bead IDs and reasons
- Detailed split trigger checking
- Candidate sorting operations

### At DEBUG level:
- Aggregate filtering statistics
- Remaining candidate counts after each filter
- Split threshold checks and triggers
- Final candidate list returned

### At INFO level:
- High-level strand start/completion
- Final candidate counts
- Split instruction triggers

## Code Reference

- **Pluck implementation:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Tracing initialization:** `/home/coding/NEEDLE/src/cli/mod.rs`
- **Capture script:** `/home/coding/ARMOR/capture-pluck-debug.sh`

## Summary

- **Primary control:** `RUST_LOG` environment variable
- **Pluck target:** `needle::strand::pluck`
- **Levels:** `trace` (detailed), `debug` (standard), `info` (minimal)
- **Helper script:** `capture-pluck-debug.sh` for comprehensive capture
- **Configuration:** Cannot be set in config files - runtime only via `RUST_LOG`

## Related Documentation

- **Pluck Debug Flags:** `notes/bf-5p3g-pluck-debug-flags.md`
- **Pluck Analysis:** `notes/pluck-debug-configuration.md`
- **Source Code:** `/home/coding/NEEDLE/src/strand/pluck.rs`

---

**Status:** ✅ Complete - Configuration ready for execution