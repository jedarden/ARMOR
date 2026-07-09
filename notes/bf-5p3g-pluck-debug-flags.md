# Pluck Debug Flags and Logging Configuration

**Bead:** bf-5p3g  
**Component:** NEEDLE Pluck Strand  
**Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`  
**Date:** 2026-07-09

## Overview

Pluck is the primary bead selection strand in NEEDLE, handling >90% of all bead processing. It uses the Rust `tracing` crate for structured logging with multiple log levels. Debug logging is controlled via the `RUST_LOG` environment variable.

## Available Debug Flags

### Primary Environment Variable: `RUST_LOG`

The `RUST_LOG` environment variable controls which log statements are emitted. The `tracing-subscriber` crate's `env-filter` feature parses this variable at runtime.

#### Syntax
```
RUST_LOG=<target>=<level>,<target2>=<level2>,...
```

#### Available Log Levels
- `error` - Failures that prevent operation
- `warn` - (None currently used in Pluck)
- `info` - Significant events (split trigger, candidate return)
- `debug` - Detailed operation trace (filter stages, counts, decisions)
- `trace` - Most granular details (reserved for future expansion)

#### Target Paths for Pluck
- `needle::strand::pluck` - Pluck strand specific
- `needle::strand` - All strand modules
- `needle::bead_store` - Bead store operations
- `needle::worker` - Worker lifecycle
- `needle::dispatch` - Agent dispatch
- `needle` - All NEEDLE modules
- (no target) - All crates globally

### Common Usage Patterns

#### Pluck-only Debug
```bash
RUST_LOG=needle::strand::pluck=debug
```

#### Pluck-only Trace (Maximum Verbosity)
```bash
RUST_LOG=needle::strand::pluck=trace
```

#### All NEEDLE Debug
```bash
RUST_LOG=needle=debug
```

#### Everything Debug (Very Verbose)
```bash
RUST_LOG=debug
```

#### Comprehensive Pluck Capture (Recommended)
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

## Pluck Debug Events

### 1. Evaluation Start
```
DEBUG Pluck strand evaluation starting
  exclude_labels: ["deferred", "human", "blocked"]
  split_threshold: 3
```

### 2. Bead Store Query
```
DEBUG Querying bead store for ready candidates
  filters: Filters { assignee: None, exclude_labels: [...] }
```

### 3. Store Results
```
DEBUG Bead store returned {count} candidates
  count: 42
```

### 4. Label Filtering
When beads are excluded:
```
DEBUG Label filtering excluded {excluded_count} beads
  excluded_count: 5
  remaining: 37
  excluded_labels: ["deferred", "human", "blocked"]
```

For each excluded bead:
```
DEBUG Excluded bead due to labels
  bead_id: "bf-1234"
  labels: ["deferred", "priority:p2"]
  excluded_reasons: ["deferred"]
```

### 5. Status/Assignee Filtering
```
DEBUG Status/assignee filtering removed {filtered_count} beads
  filtered_count: 2
  remaining: 35
```

### 6. Sorting
```
DEBUG Sorting {total} candidates by (priority ASC, created_at ASC, id ASC)
  total: 35
  first_bead_id: "bf-0001"
  first_priority: 1
  first_created_at: "2026-07-09T00:00:00Z"
```

### 7. Split Trigger Check
```
DEBUG Checking split trigger for first candidate
  bead_id: "bf-0001"
  failure_count: 0
  threshold: 3
  split_triggered: false
```

When split is triggered:
```
INFO Split threshold reached, returning Split instruction
  bead_id: "bf-0001"
  failure_count: 4
  threshold: 3
```

### 8. Result Return
When candidates are found:
```
INFO Returning {count} candidates for processing
  count: 35
  candidates: ["bf-0001", "bf-0002", "bf-0003", ...]
```

When no candidates remain:
```
DEBUG No candidates remaining after filtering, returning NoWork
```

### 9. Error Cases
```
ERROR Bead store query failed
  error: <error details>
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

### Method 4: Comprehensive Capture Script
The provided `capture-pluck-debug.sh` script demonstrates a complete capture:
```bash
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug-output.log 1
```

This sets:
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

## Filtering Decision Logging

### Which Events Show Filtering Decisions?

All filter stages emit structured events:

1. **Label filtering**: Logs count of excluded beads and reasons
2. **Status/assignee filtering**: Logs count of removed beads
3. **Individual exclusions**: Each excluded bead logs its ID, labels, and exclusion reasons

### Key Fields for Debugging Filtering

- `exclude_labels` - Labels being filtered
- `excluded_count` - How many beads were excluded
- `remaining` - How many beads remain after filtering
- `excluded_reasons` - Which labels caused exclusion
- `filters` - The filter object passed to the bead store

## CLI Flags

**Note:** NEEDLE does not provide CLI flags for debug logging configuration. All logging is controlled via the `RUST_LOG` environment variable.

However, NEEDLE provides related logging commands:

### `needle logs`
View and query telemetry logs:
```bash
needle logs --follow                    # Stream events in real-time
needle logs --filter 'event_type=bead.*'  # Filter events
needle logs --since 1h                   # Events since 1 hour ago
needle logs --format json               # JSON Lines output
```

### `needle config --dump`
View resolved configuration including logging setup:
```bash
needle config --dump                    # Show all config
needle config --get telemetry.file_sink.enabled  # Get specific value
```

## Tracing Instrumentation Details

### Span Fields
All Pluck events include these fields:
- `strand`: "pluck" - identifies the strand type
- `exclude_labels`: List of labels being filtered
- `split_threshold`: Failure count threshold for auto-split

### Event Types
- `tracing::debug!()` - Detailed diagnostic information
- `tracing::info!()` - Significant operational events
- `tracing::error!()` - Failure events

### Span Name
All Pluck operations are tracked in the `strand.pluck` span.

## Configuration Sources

The tracing subscriber is initialized in `src/cli/mod.rs`:
1. OTLP layer (if enabled in config)
2. fmt layer for stderr output
3. Environment variable filter (via `tracing-subscriber`'s `env-filter` feature)

## Acceptance Criteria Verification

✅ **List of available debug flags/variables found**
- Primary: `RUST_LOG` environment variable
- Levels: error, warn, info, debug, trace
- Targets: needle::strand::pluck, needle::strand, needle, etc.

✅ **Documentation of which flags control filtering decision logging**
- `RUST_LOG=needle::strand::pluck=debug` enables all filtering decision logs
- Individual bead exclusions logged at DEBUG level
- Filter stage counts logged at DEBUG level
- Split trigger decisions logged at DEBUG/INFO level

✅ **Clear instructions on how to enable debug output**
- Multiple methods documented (env var, inline, file capture, script)
- Comprehensive capture script provided
- Examples for all common scenarios

## Related Documentation

- **Pluck Filter Configurations:** See `bf-1jwl.md` for complete filter documentation
- **Pluck Debug Logging Guide:** See `bf-2hvf.md` for detailed event examples
- **Pluck exclude_labels:** See `bf-ogec.md` for exclude_labels extraction
- **NEEDLE Logging:** Check NEEDLE project docs for global logging configuration
- **capture-pluck-debug.sh:** Shell script demonstrating comprehensive debug capture

## Future Enhancements

Possible improvements for future work:

1. **CLI Flags:** Add `--log-level` flag to avoid environment variable manipulation
2. **Filter Configuration:** Add ability to enable/disable specific log categories
3. **Structured JSON Output:** Emit logs in JSON format for easier parsing
4. **Per-Bead Timings:** Add timing information for each filtering stage
5. **Filter Decision Logs:** Emit specific reason for each bead's inclusion/exclusion

---

**Status:** ✅ Complete - Pluck debug flags and logging configuration documented
