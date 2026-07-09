# Pluck Debug Flags and Logging Configuration

## Overview

Pluck is a strand within NEEDLE that handles primary bead selection from the workspace. It processes >90% of all beads by querying the bead store for unassigned, ready beads, filtering by excluded labels, and sorting them in deterministic priority order.

## Debug Logging Control

### Primary Environment Variable: `RUST_LOG`

All Pluck debug output is controlled via the `RUST_LOG` environment variable, which uses the standard Rust `tracing` crate's log filtering syntax.

### Pluck-Specific Debug Targets

```
needle::strand::pluck=trace     # Most detailed - all Pluck operations
needle::strand::pluck=debug     # Detailed filtering decisions  
needle::strand::pluck=info      # High-level operations only
```

### Recommended Debug Configurations

**Comprehensive Pluck debugging:**
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

**Pluck-focused debugging:**
```bash
export RUST_LOG="needle::strand::pluck=debug"
```

**Minimal Pluck info:**
```bash
export RUST_LOG="needle::strand::pluck=info"
```

**Full workspace debugging:**
```bash
export RUST_LOG="debug"
```

## What Gets Logged

### At `trace` level (most detailed):
- Initial strand evaluation parameters
- Exact bead store filters being applied
- Per-bead filtering decisions with labels
- Individual excluded bead IDs and reasons
- Detailed split trigger checking
- Candidate sorting operations

### At `debug` level:
- Aggregate filtering statistics (e.g., "5 beads excluded")
- Remaining candidate counts after each filter
- Split threshold checks and triggers
- Final candidate list returned

### At `info` level:
- High-level strand start/completion
- Final candidate counts
- Split instruction triggers

## Key Logging Events

### Filtering Decisions
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

## How Pluck Filtering Works

Based on the code analysis, Pluck applies these filters in order:

1. **Bead store query** - Gets ready, unassigned beads with exclude_labels filter
2. **Label filtering** - Defensive second pass to remove excluded-label beads
3. **Status/assignee filtering** - Removes in-progress and stale-assignee beads  
4. **Sorting** - Deterministic order: `(priority ASC, created_at ASC, id ASC)`
5. **Split trigger check** - Returns Split instruction if failure count >= threshold

## Default Exclude Labels

When not configured, Pluck excludes these labels:
- `deferred` - Beads deferred for later processing
- `human` - Beads requiring human intervention
- `blocked` - Beads blocked by dependencies

## Using the Debug Capture Script

A helper script exists for comprehensive debug capture:

```bash
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1
```

This script:
- Sets comprehensive RUST_LOG configuration
- Runs NEEDLE with the specified workspace
- Captures all output to a timestamped log file
- Provides analysis grep commands for common patterns

## Analyzing Debug Output

### Find filtering decisions:
```bash
grep -i "filter" output.log
grep -i "exclude" output.log
```

### Find specific beads:
```bash
grep "bf-123" output.log
```

### Find split operations:
```bash
grep -i "split" output.log
```

### Find candidate selection:
```bash
grep -i "candidate" output.log
```

## Code Locations

- **Pluck implementation**: `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Tracing initialization**: `/home/coding/NEEDLE/src/cli/mod.rs` (init_tracing_subscriber function)
- **Capture script**: `/home/coding/ARMOR/capture-pluck-debug.sh`

## Tracing Architecture

NEEDLE uses the `tracing` crate (not `log`) for structured logging:
- `tracing::debug!()` - Detailed operations
- `tracing::info!()` - High-level events
- `tracing::error!()` - Error conditions
- `tracing::instrument` macro - Span-based tracing for function entry/exit

The tracing subscriber is initialized in the CLI layer with optional OTLP support for distributed tracing.

## Configuration vs Runtime

Pluck's exclude_labels and split_threshold are configurable via:
1. Global config: `~/.config/needle/config.yaml`
2. Workspace config: `.needle.yaml` in workspace root
3. Environment variables: `NEEDLE_STRANDS_PLUCK_EXCLUDE_LABELS`

However, debug logging levels are **only** controlled via `RUST_LOG` and cannot be set in config files.

## Common Debug Patterns

### Why are beads being excluded?
Set `RUST_LOG=needle::strand::pluck=debug` and look for:
- "Label filtering excluded" messages
- "Excluded bead due to labels" with bead IDs and specific labels
- Check if exclude_labels match what you expect

### Why is split triggering?
Set `RUST_LOG=needle::strand::pluck=debug` and look for:
- "Split threshold reached" messages
- Check failure_count vs threshold values
- Look for "failure-count:N" labels on beads

### Why no candidates returned?
Set `RUST_LOG=needle::strand::pluck=debug` and look for:
- "No candidates remaining after filtering"
- Track how many beads removed at each filtering stage
- Check if all beads are excluded by labels, status, or assignee

## Available Debug Modules

Beyond Pluck, these modules can be debugged:

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

- **Primary control**: `RUST_LOG` environment variable
- **Pluck target**: `needle::strand::pluck`
- **Levels**: `trace` (detailed), `debug` (standard), `info` (minimal)
- **Helper script**: `capture-pluck-debug.sh` for comprehensive capture
- **No config file support** - debug levels are runtime-only via `RUST_LOG`
