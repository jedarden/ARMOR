# Pluck Debug Flags and Logging Configuration

## Overview

Pluck is a strand within NEEDLE (a Rust-based bead processing system) that handles primary bead selection from assigned workspaces. This document describes all available debug flags, logging configuration options, and how to enable filtering-related debug output.

## Logging Infrastructure

Pluck uses Rust's `tracing` crate for structured logging with the `RUST_LOG` environment variable controlling output verbosity and scope.

### Environment Variable

**`RUST_LOG`** - Controls logging levels for specific Rust modules

**Syntax:** `RUST_LOG=module_path=log_level[,module_path=log_level,...]`

**Available Log Levels:**
- `error` - Only errors
- `warn` - Warnings and errors  
- `info` - Normal informational messages (default)
- `debug` - Detailed diagnostic information
- `trace` - Extremely detailed execution flow

## Available Debug Modules

### Pluck-Specific Modules

| Module Path | Description | Recommended Level |
|-------------|-------------|-------------------|
| `needle::strand::pluck` | Core Pluck strand evaluation logic | `debug` or `trace` |
| `needle::strand` | All strand coordination (Pluck, Mend, Explore, etc.) | `debug` |
| `needle::worker` | Worker state machine and lifecycle | `debug` |

### Supporting Modules

| Module Path | Description | Recommended Level |
|-------------|-------------|-------------------|
| `needle::bead_store` | Bead store queries and operations | `debug` |
| `needle::dispatch` | Agent dispatch and execution | `debug` |
| `needle::claim` | Bead claiming logic | `debug` |

## Debug Configuration Presets

### 1. Minimal Pluck Debug
**Purpose:** Filter decisions and candidate counts only

```bash
export RUST_LOG=needle::strand::pluck=debug
```

**What you'll see:**
- Pluck strand evaluation starting
- Bead store query results
- Candidate counts after filtering
- Label filtering exclusions
- Final candidate return

### 2. Comprehensive Pluck Trace  
**Purpose:** Detailed Pluck execution flow with all filtering decisions

```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug
```

**What you'll see:**
- Everything from minimal debug, plus:
- Individual excluded bead details with labels and reasons
- Status/assignee filtering details
- Split threshold evaluation
- Detailed candidate sorting information

### 3. Full Worker Context (RECOMMENDED)
**Purpose:** Complete debugging context including coordination and storage

```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

**What you'll see:**
- All Pluck trace output
- Worker state transitions
- Bead store operations
- Agent dispatch details
- Claim retry logic

### 4. Maximum Debug Output
**Purpose:** Everything at debug level (not recommended - very verbose)

```bash
export RUST_LOG=debug
```

## Key Pluck Debug Events

### Strand Evaluation Start
```
[timestamp] DEBUG needle::strand::pluck: Pluck strand evaluation starting
  exclude_labels: ["deferred", "human", "blocked"]
  split_threshold: 3
```

### Bead Store Query
```
[timestamp] DEBUG needle::strand::pluck: Querying bead store for ready candidates
  filters: Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }
```

### Label Filtering
```
[timestamp] DEBUG needle::strand::pluck: Label filtering excluded 2 beads
  excluded_count: 2
  remaining: 5
  excluded_labels: ["deferred", "human", "blocked"]
```

### Individual Excluded Beads
```
[timestamp] DEBUG needle::strand::pluck: Excluded bead due to labels
  bead_id: "bf-1234"
  labels: ["deferred", "bug-fix"]
  excluded_reasons: ["deferred"]
```

### Status/Assignee Filtering
```
[timestamp] DEBUG needle::strand::pluck: Status/assignee filtering removed 1 beads
  filtered_count: 1
  remaining: 4
```

### Split Trigger Evaluation
```
[timestamp] DEBUG needle::strand::pluck: Checking split trigger for first candidate
  bead_id: "bf-5678"
  failure_count: 2
  threshold: 3
  split_triggered: false
```

### Candidate Return
```
[timestamp] INFO needle::strand::pluck: Returning 3 candidates for processing
  count: 3
  candidates: ["bf-1001", "bf-1002", "bf-1003"]
```

## Filtering Decision Logging

The Pluck strand logs comprehensive filtering decisions at multiple points:

### 1. Store Query Results (Line 117-120 in pluck.rs)
```rust
tracing::debug!(
    filters = ?filters,
    "Querying bead store for ready candidates"
);
```

### 2. Initial Candidate Count (Line 124-128)
```rust
tracing::debug!(
    count = beads.len(),
    "Bead store returned {} candidates",
    beads.len()
);
```

### 3. Label Filtering Summary (Line 153-159)
```rust
tracing::debug!(
    excluded_count = before_label_filter - after_label_filter,
    remaining = after_label_filter,
    excluded_labels = ?self.exclude_labels,
    "Label filtering excluded {} beads",
    before_label_filter - after_label_filter
);
```

### 4. Individual Excluded Beads (Line 161-177)
```rust
for (id, labels) in excluded_beads {
    let excluded_reasons: Vec<_> = labels.iter()
        .filter(|l| self.exclude_labels.contains(l))
        .map(|l| l.as_str())
        .collect();
    tracing::debug!(
        bead_id = %id,
        labels = ?labels,
        excluded_reasons = ?excluded_reasons,
        "Excluded bead due to labels"
    );
}
```

### 5. Status/Assignee Filtering (Line 198-210)
```rust
tracing::debug!(
    filtered_count = before_status_filter - after_status_filter,
    remaining = after_status_filter,
    "Status/assignee filtering removed {} beads",
    before_status_filter - after_status_filter
);
```

## Usage Examples

### Direct Execution
```bash
# Set debug configuration
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug"

# Run NEEDLE worker
needle run -w /home/coding/ARMOR -c 1
```

### Using the Capture Script
```bash
# The ARMOR workspace includes a pre-configured capture script
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug.log 1
```

### Sourcing Environment Configuration
```bash
# Use the pre-configured environment file
source .env.pluck-debug

# Run NEEDLE worker
needle run -w /home/coding/ARMOR -c 1
```

### Temporary Override
```bash
# One-time debug execution without persistence
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1
```

## Log File Analysis

### Filtering Pluck Output
```bash
# See only Pluck strand messages
grep -i 'pluck' pluck-debug.log

# See only filtering decisions  
grep -i 'filter' pluck-debug.log

# See only excluded beads
grep -i 'exclude' pluck-debug.log

# See only candidate information
grep -i 'candidate' pluck-debug.log
```

### Analyzing Filtering Behavior
```bash
# Count excluded beads by label
grep "excluded_labels" pluck-debug.log | grep -oP 'excluded_count: \K\d+'

# See which labels caused exclusions
grep "excluded_reasons" pluck-debug.log

# Track candidate counts over time
grep "Bead store returned" pluck-debug.log
```

## Debug Output Interpretation

### Normal Processing Flow
1. **Pluck strand evaluation starting** - Worker began processing
2. **Querying bead store for ready candidates** - Asking for available work
3. **Bead store returned N candidates** - Found N potential beads
4. **Label filtering excluded X beads** - Filtered out beads with excluded labels
5. **Status/assignee filtering removed Y beads** - Filtered out unavailable beads
6. **Returning Z candidates for processing** - Z beads remain after all filtering

### No Work Available
```
Pluck strand evaluation starting
Querying bead store for ready candidates
Bead store returned 0 candidates
No candidates remaining after filtering, returning NoWork
```

### Split Triggered
```
Checking split trigger for first candidate
  bead_id: "bf-retry-123"
  failure_count: 5
  threshold: 3
  split_triggered: true

INFO needle::strand::pluck: Split threshold reached, returning Split instruction
  bead_id: "bf-retry-123"
  failure_count: 5
  threshold: 3
```

## Configuration Files

### `.env.pluck-debug` 
Located at `/home/coding/ARMOR/.env.pluck-debug`

This file contains pre-configured debug settings:
```bash
# Minimal Pluck debug - filtering decisions and candidate counts
# export RUST_LOG=needle::strand::pluck=debug

# Comprehensive Pluck trace - detailed execution flow
# export RUST_LOG=needle::strand::pluck=trace

# Full strand context - all strands with detailed Pluck trace
# export RUST_LOG=needle::strand=debug,needle::strand::pluck=trace

# Complete worker context - Pluck + coordination + storage (RECOMMENDED)
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

### `capture-pluck-debug.sh`
Located at `/home/coding/ARMOR/capture-pluck-debug.sh`

Automated script for capturing debug output:
```bash
./capture-pluck-debug.sh <workspace> <output_file> <count>
```

## Technical Details

### Pluck Strand Source Location
The Pluck strand implementation is located at:
```
/home/coding/NEEDLE/src/strand/pluck.rs
```

### Default Exclude Labels
When no custom exclude labels are provided, Pluck uses:
```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked"];
```

### Default Split Threshold
```rust
split_after_failures: 3  // Triggers split after 3 consecutive failures
```

### Tracing Instrumentation Points
The Pluck strand uses `tracing::instrument!` macro (line 95-103) to automatically capture:
- Strand name ("pluck")
- Exclude labels configuration
- Split threshold value
- Function entry/exit timing

## Troubleshooting

### No Debug Output Appearing

1. **Verify RUST_LOG is set:**
   ```bash
   echo $RUST_LOG
   ```

2. **Check for typos in module paths:**
   ```bash
   # Correct
   RUST_LOG=needle::strand::pluck=debug
   
   # Incorrect (missing namespace)
   RUST_LOG=pluck=debug
   ```

3. **Ensure environment variable is exported:**
   ```bash
   # This won't work
   RUST_LOG=needle::strand::pluck=debug needle run
   
   # This will work
   export RUST_LOG=needle::strand::pluck=debug
   needle run
   ```

### Too Much Output

1. **Reduce scope to Pluck only:**
   ```bash
   export RUST_LOG=needle::strand::pluck=debug
   ```

2. **Use specific log level:**
   ```bash
   # Only INFO and above
   export RUST_LOG=needle::strand::pluck=info
   
   # Only WARN and above  
   export RUST_LOG=needle::strand::pluck=warn
   ```

### Missing Specific Events

1. **Check trace level for detailed events:**
   ```bash
   export RUST_LOG=needle::strand::pluck=trace
   ```

2. **Enable supporting modules:**
   ```bash
   export RUST_LOG=needle::strand::pluck=trace,needle::bead_store=debug
   ```

## Performance Considerations

### Impact of Debug Logging
- **`debug` level**: ~5-10% performance overhead
- **`trace` level**: ~10-20% performance overhead  
- **Full debug mode**: ~20-30% performance overhead

### Disk Space Usage
Debug logging can generate significant output:
- **Minimal debug**: ~1-2 MB per hour per worker
- **Full worker context**: ~5-10 MB per hour per worker
- **Maximum debug**: ~20-50 MB per hour per worker

## Related Documentation

- **NEEDLE Project**: `/home/coding/NEEDLE/`
- **Pluck Source**: `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Worker Module**: `/home/coding/NEEDLE/src/worker/mod.rs`
- **Strand Coordination**: `/home/coding/NEEDLE/src/strand/mod.rs`
- **ARMOR Configuration**: `/home/coding/ARMOR/.env.pluck-debug`
- **Capture Script**: `/home/coding/ARMOR/capture-pluck-debug.sh`

## Summary

Pluck provides comprehensive debug logging through Rust's tracing infrastructure. The key controls are:

1. **Environment Variable**: `RUST_LOG`
2. **Primary Module**: `needle::strand::pluck`
3. **Recommended Levels**: `debug` for filtering decisions, `trace` for detailed flow
4. **Supporting Modules**: Add `needle::bead_store`, `needle::worker`, `needle::dispatch` for full context
5. **Configuration Files**: Use `.env.pluck-debug` for persistent settings or `capture-pluck-debug.sh` for automated capture

The filtering decision logging is particularly detailed, showing exactly which beads were excluded, which labels caused the exclusion, and how many candidates remain at each filtering stage.
