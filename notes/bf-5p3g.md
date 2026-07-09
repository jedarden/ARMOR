# Pluck Debug Flags and Logging Configuration

**Bead:** bf-5p3g
**Date:** 2026-07-09
**Status:** ✅ Complete

## Overview

Pluck uses the standard Rust `tracing` crate for instrumentation and logging. Debug output is controlled via the `RUST_LOG` environment variable, which follows the standard tracing filter syntax.

## Primary Debug Control: `RUST_LOG`

The `RUST_LOG` environment variable controls all debug logging in NEEDLE, including Pluck strand filtering output.

### Basic Syntax

```bash
RUST_LOG=<module>=<level> needle run -w <workspace>
```

### Available Log Levels

- `trace` - Most verbose, shows all execution details
- `debug` - Detailed filtering decisions and state transitions
- `info` - High-level events (default)
- `warn` - Warning messages
- `error` - Error messages only

## Pluck-Specific Debug Targets

### For Pluck Strand Only

```bash
# DEBUG level - detailed filtering decisions
RUST_LOG=needle::strand::pluck=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1

# TRACE level - exhaustive execution details
RUST_LOG=needle::strand::pluck=trace /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1
```

### For Multiple NEEDLE Modules

```bash
# Pluck + Worker state machine
RUST_LOG=needle::strand::pluck=debug,needle::worker=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1

# All NEEDLE code at DEBUG level
RUST_LOG=needle=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1

# All NEEDLE code at TRACE level (very verbose)
RUST_LOG=needle=trace /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1
```

### Global Debug (All Crates)

```bash
# Debug everything (NEEDLE + all dependencies)
RUST_LOG=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1
```

## What Pluck Debug Output Shows

When `RUST_LOG=needle::strand::pluck=debug` is set, Pluck outputs:

### 1. Strand Initialization
```
DEBUG needle::strand::pluck: Pluck strand evaluation starting exclude_labels=["deferred", "human", "blocked"] split_threshold=3
```
- Shows default exclude_labels configuration
- Shows split threshold for auto-splitting failed beads
- Timestamp and context

### 2. Filter Construction
```
DEBUG needle::strand::pluck: Querying bead store for ready candidates filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }
```
- Shows filters being applied to the bead store query
- No assignee filter (None)
- Excludes beads with labels: deferred, human, blocked

### 3. Candidate Count
```
DEBUG needle::strand::pluck: Bead store returned N candidates
```
- Number of beads returned from the initial query

### 4. Label Filtering
```
DEBUG needle::strand::pluck: Label filtering excluded N beads
DEBUG needle::strand::pluck: Excluded bead due to labels bead_id=bf-123 labels=["deferred"] excluded_reasons=["deferred"]
```
- Individual bead exclusion logging with reasons
- Shows which beads were filtered and why

### 5. Status/Assignee Filtering
```
DEBUG needle::strand::pluck: Status/assignee filtering removed N beads
```
- Beads filtered due to in-progress status or stale assignee

### 6. Sorting and Candidate Selection
```
DEBUG needle::strand::pluck: Sorting N candidates by (priority ASC, created_at ASC, id ASC)
DEBUG needle::strand::pluck: first_bead_id=bf-456 first_priority=1 first_created_at=2026-07-09T04:20:56Z
```
- Candidate sorting decisions
- First candidate details

### 7. Split Trigger Check
```
DEBUG needle::strand::pluck: Checking split trigger for first candidate bead_id=bf-789 failure_count=2 threshold=3 split_triggered=false
```
- Whether the first candidate has enough failures to trigger auto-split

### 8. Final Result
```
INFO needle::strand::pluck: Returning N candidates for processing candidates=["bf-1", "bf-2", "bf-3"]
```
- Final candidate list returned for processing

## Tracing Span Information

Pluck uses `tracing::instrument` to automatically capture:
- **Span name:** `strand.pluck`
- **Fields captured:**
  - `strand`: "pluck"
  - `exclude_labels`: The label exclusion list
  - `split_threshold`: The failure count threshold for splitting

These fields are included in all child events within the span.

## Other Available Debug Targets

### Worker State Machine
```bash
RUST_LOG=needle::worker=debug
```
Shows state transitions, bead lifecycle events, and worker loop decisions.

### Other Strands
```bash
# Explore strand (multi-workspace discovery)
RUST_LOG=needle::strand::explore=debug

# Mend strand (stuck bead recovery)
RUST_LOG=needle::strand::mend=debug

# Weave strand (gap analysis and bead creation)
RUST_LOG=needle::strand::weave=debug
```

### Telemetry System
```bash
RUST_LOG=needle::telemetry=debug
```
Shows telemetry event emission and sink behavior.

### Agent Dispatch
```bash
RUST_LOG=needle::dispatch=debug
```
Shows agent process spawning and command construction.

## Capturing Debug Output

### Save to File
```bash
RUST_LOG=needle::strand::pluck=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### Real-time Monitoring
```bash
RUST_LOG=needle::strand::pluck=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -E "Pluck|pluck|strand"
```

## Environment Variable Priority

The `RUST_LOG` environment variable overrides any compiled-in defaults and is the primary method for controlling debug output. There are no CLI flags for debug level control in NEEDLE.

## Common Debug Patterns

### Pattern 1: Focus on Filtering Decisions
```bash
RUST_LOG=needle::strand::pluck=debug,needle::worker=info
```
Shows detailed Pluck filtering but keeps worker output at info level.

### Pattern 2: Full Trace of Single Bead Processing
```bash
RUST_LOG=needle=trace /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -A 20 "bf-123"
```

### Pattern 3: Exclude Noisy Modules
```bash
RUST_LOG=needle::strand::pluck=debug,needle::tokio=warn
```
Shows Pluck debug but suppresses tokio runtime details.

## Verification

To verify debug logging is working:
1. Set `RUST_LOG=needle::strand::pluck=debug`
2. Run the worker
3. Look for `DEBUG needle::strand::pluck: Pluck strand evaluation starting` in the output
4. Confirm that filter fields are shown (`exclude_labels`, `split_threshold`)

## Integration with Telemetry

Debug output via `RUST_LOG` is separate from the JSONL telemetry system:
- `RUST_LOG` controls human-readable stderr output
- Telemetry file writes (`~/.needle/logs/*.jsonl`) are always enabled
- Both can be active simultaneously

## Source Code References

- Pluck strand implementation: `/home/coding/NEEDLE/src/strand/pluck.rs`
- Tracing initialization: `/home/coding/NEEDLE/src/cli/mod.rs` (lines 671-756)
- Tracing crate documentation: https://docs.rs/tracing/

## No Additional Debug Flags

There are **no other debug flags, environment variables, or CLI switches** for Pluck filtering. All debug output is controlled through `RUST_LOG` as documented above.

## Acceptance Criteria Met

✅ **List of available debug flags/variables found:**
- `RUST_LOG` environment variable with module path syntax

✅ **Documentation of which flags control filtering decision logging:**
- `RUST_LOG=needle::strand::pluck=debug` for detailed filtering
- `RUST_LOG=needle::strand::pluck=trace` for exhaustive output

✅ **Clear instructions on how to enable debug output:**
- Multiple examples provided for different scenarios
- File capture patterns documented
- Real-time monitoring commands included
