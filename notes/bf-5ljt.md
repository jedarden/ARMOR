# Pluck Debug Flags and Logging Configuration Research

**Bead:** bf-5ljt  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR

## Executive Summary

This research identifies debug flags and logging options for the Pluck strand in the NEEDLE system. Pluck is the primary bead selection strand that handles >90% of all bead processing in NEEDLE workspaces.

## Project Context

- **ARMOR**: Go-based S3-compatible encryption proxy server (this workspace)
- **NEEDLE**: Rust-based task management system located at `/home/coding/NEEDLE`
- **Pluck**: A strand within NEEDLE responsible for bead selection and filtering decisions
- **Relationship**: NEEDLE uses ARMOR as a workspace for processing beads

## Debug Logging Infrastructure

### Environment Variable: `RUST_LOG`

The primary mechanism for controlling debug output in NEEDLE is the `RUST_LOG` environment variable, which is standard for Rust applications using the `tracing` library.

**Key Dependencies:**
- `tracing = "0.1"` - Core tracing instrumentation
- `tracing-subscriber = "0.3"` - With `env-filter` and `json` features enabled
- Uses standard Rust tracing patterns with `tracing::debug!()`, `tracing::info!()`, `tracing::error!()`

### Logging Levels

The `RUST_LOG` variable supports the standard Rust logging levels:
- `error` - Error conditions
- `warn` - Warning conditions  
- `info` - Informational messages
- `debug` - Debug-level detail
- `trace` - Most verbose, trace-level detail

## Pluck-Specific Logging Targets

Based on the source code analysis of `/home/coding/NEEDLE/src/strand/pluck.rs`, the following logging targets are available:

### Primary Target
- `needle::strand::pluck` - Pluck strand execution and filtering decisions

### Supporting Modules
- `needle::strand` - General strand coordination
- `needle::bead_store` - Bead store queries and operations
- `needle::worker` - Worker lifecycle and execution
- `needle::dispatch` - Task dispatching logic
- `needle::claim` - Bead claiming operations

## Recommended Configurations

### 1. Minimal Filtering Decisions
```bash
export RUST_LOG=needle::strand::pluck=info
```
**Output:** High-level strand operations only

### 2. Standard Debug (Recommended)
```bash
export RUST_LOG=needle::strand::pluck=debug
```
**Output:** Filtering decisions, candidate counts, and statistics

### 3. Detailed Trace
```bash
export RUST_LOG=needle::strand::pluck=trace
```
**Output:** Complete execution flow with per-bead details

### 4. Comprehensive Context
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```
**Output:** Pluck trace-level plus supporting modules at debug level

### 5. Full System Debug
```bash
export RUST_LOG=debug
```
**Output:** All NEEDLE modules at debug level

### 6. Maximum Verbosity
```bash
export RUST_LOG=trace
```
**Output:** Everything at trace level (very verbose)

## Pluck Strand Logging Events

Based on source code analysis, Pluck emits the following debug events:

### 1. Evaluation Start
```
DEBUG needle::strand::pluck: Pluck strand evaluation starting
  exclude_labels=["deferred", "human", "blocked"]
  split_threshold=3
```

### 2. Bead Store Query
```
DEBUG needle::strand::pluck: Querying bead store for ready candidates
  filters=Filters { 
    assignee: None, 
    exclude_labels: ["deferred", "human", "blocked"] 
  }
```

### 3. Query Results
```
DEBUG needle::strand::pluck: Bead store returned N candidates
  count=5
```

### 4. Label Filtering
```
DEBUG needle::strand::pluck: Label filtering excluded N beads
  excluded_count=2
  remaining=3
  excluded_labels=["deferred", "human", "blocked"]
```

### 5. Individual Bead Exclusion
```
DEBUG needle::strand::pluck: Excluded bead due to labels
  bead_id="bf-1234"
  labels=["deferred", "blocked"]
  excluded_reasons=["deferred", "blocked"]
```

### 6. Status/Assignee Filtering
```
DEBUG needle::strand::pluck: Status/assignee filtering removed N beads
  filtered_count=1
  remaining=2
```

### 7. Candidate Sorting
```
DEBUG needle::strand::pluck: Sorting N candidates by (priority ASC, created_at ASC, id ASC)
  total=3
  first_bead_id="bf-abcd"
  first_priority=50
  first_created_at="2026-07-09T00:00:00Z"
```

### 8. Split Threshold Check
```
DEBUG needle::strand::pluck: Checking split trigger for first candidate
  bead_id="bf-abcd"
  failure_count=2
  threshold=3
  split_triggered=false
```

### 9. Split Triggered
```
INFO needle::strand::pluck: Split threshold reached, returning Split instruction
  bead_id="bf-abcd"
  failure_count=3
  threshold=3
```

## Usage Examples

### Running NEEDLE with Debug Output
```bash
# Set environment variable
export RUST_LOG=needle::strand::pluck=debug

# Run NEEDLE against ARMOR workspace
cd /home/coding/NEEDLE
cargo run -- run -w /home/coding/ARMOR -c 1

# Or with output capture
RUST_LOG=needle::strand::pluck=debug cargo run -- run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### Using Release Binary
```bash
# Build release version
cargo build --release

# Run with debug logging
RUST_LOG=needle::strand::pluck=trace /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1
```

## Additional Environment Variables

While `RUST_LOG` is the primary control, the following may also affect logging:

### `RUST_BACKTRACE`
```bash
export RUST_BACKTRACE=1  # Enable backtraces on errors
export RUST_BACKTRACE=full  # Full backtraces
```

### Tracing Subscriber Features
The tracing subscriber is configured with:
- `env-filter` feature - Enables `RUST_LOG` environment variable parsing
- `json` feature - Enables JSON-formatted log output
- ANSI color support for stderr output

## Analysis Commands

Once debug logs are captured, analyze them with:

```bash
# View all Pluck-related events
grep -i "pluck\|needle::strand::pluck" pluck-debug.log

# Filter specific decision types
grep "Label filtering excluded" pluck-debug.log
grep "Status/assignee filtering" pluck-debug.log
grep "Checking split trigger" pluck-debug.log

# Check excluded beads
grep "Excluded bead due to labels" pluck-debug.log

# View candidate processing
grep "Sorting.*candidates" pluck-debug.log
grep "Bead store returned" pluck-debug.log

# Count events by type
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "Split threshold reached" pluck-debug.log
```

## Integration with ARMOR Workspace

The existing debug infrastructure in the ARMOR workspace includes:

### Configuration Files
- `/home/coding/ARMOR/pluck-debug-config.sh` - Debug level configuration script
- `/home/coding/ARMOR/.env.pluck-debug` - Environment variable presets
- `/home/coding/ARMOR/pluck-debug-configuration.md` - Detailed usage documentation

### Preset Configurations Available
1. **minimal** - INFO level only
2. **standard** - DEBUG level (recommended)
3. **detailed** - TRACE level
4. **comprehensive** - Multi-module TRACE/DEBUG
5. **full** - All NEEDLE modules
6. **maximum** - Global TRACE

## Tracing Instrumentation

Pluck uses OpenTelemetry-compatible tracing with structured fields:

```rust
#[tracing::instrument(
    name = "strand.pluck",
    skip(self, store),
    fields(
        strand = "pluck",
        exclude_labels = ?self.exclude_labels,
        split_threshold = self.split_after_failures,
    )
)]
```

This provides automatic span creation with context propagation for distributed tracing.

## Key Findings

1. **No Custom CLI Flags**: NEEDLE does not implement custom command-line flags for debug control; it relies on the standard `RUST_LOG` environment variable
2. **Standard Rust Tracing**: Uses the conventional Rust tracing ecosystem with `tracing` and `tracing-subscriber` crates
3. **Module-Level Control**: Logging can be controlled at the module level using Rust's target syntax (e.g., `needle::strand::pluck=debug`)
4. **Comprehensive Coverage**: Pluck strand has extensive debug instrumentation covering all filtering decisions
5. **Structured Logging**: All logging events use structured fields for machine parsing and filtering

## Verification

To verify debug logging is working:

```bash
# Set trace level
export RUST_LOG=needle::strand::pluck=trace

# Run NEEDLE and capture output
/home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | grep -i "pluck\|filtering\|candidates"

# Expected output should show:
# - "Pluck strand evaluation starting"
# - "Querying bead store for ready candidates"
# - "Label filtering excluded" or "No beads excluded by label filter"
# - "Sorting.*candidates"
```

## Documentation Status

✅ **Debug flags identified**: RUST_LOG environment variable  
✅ **Logging targets documented**: needle::strand::pluck and supporting modules  
✅ **Configuration levels defined**: 6 preset configurations  
✅ **Filtering decision logging**: Comprehensive coverage  
✅ **Usage examples provided**: Command-line examples  
✅ **Source code verification**: Confirmed in /home/coding/NEEDLE/src/strand/pluck.rs

## Notes

- The ARMOR workspace contains existing Pluck debug documentation from prior research (bf-3b63)
- This research confirms and expands upon that documentation with source code verification
- The existing debug configuration scripts and environment files remain accurate and applicable
- No additional command-line flags or environment variables were found beyond standard RUST_LOG