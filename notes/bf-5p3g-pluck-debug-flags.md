# Pluck Debug Flags and Logging Configuration

## Overview
Pluck is the primary bead selection strand in NEEDLE (Navigates Every Enqueued Deliverable, Logs Effort). It handles >90% of all bead processing by querying the bead store for unassigned, ready beads, filtering them by excluded labels, and sorting them in deterministic priority order.

## Debug Logging Control

### Environment Variable: `RUST_LOG`
The primary way to enable debug logging for Pluck (and all other NEEDLE components) is through the standard Rust `RUST_LOG` environment variable.

#### Enable All Debug Logs
```bash
export RUST_LOG=debug
needle run
```

#### Enable Pluck-Specific Debug Logs
```bash
export RUST_LOG=needle::strand::pluck=debug
needle run
```

#### Enable Multiple Components
```bash
export RUST_LOG=needle::strand::pluck=debug,needle::bead_store=debug
needle run
```

#### Enable Trace Level (Most Verbose)
```bash
export RUST_LOG=trace
needle run
```

## Pluck Debug Logging Points

Based on analysis of `/home/coding/NEEDLE/src/strand/pluck.rs`, the following debug events are logged:

### 1. Strand Initialization
```rust
tracing::debug!(
    exclude_labels = ?self.exclude_labels,
    split_threshold = self.split_after_failures,
    "Pluck strand evaluation starting"
);
```

### 2. Bead Store Query
```rust
tracing::debug!(
    filters = ?filters,
    "Querying bead store for ready candidates"
);
```

### 3. Candidate Count
```rust
tracing::debug!(
    count = beads.len(),
    "Bead store returned {} candidates",
    beads.len()
);
```

### 4. Label Filtering
```rust
tracing::debug!(
    excluded_count = before_label_filter - after_label_filter,
    remaining = after_label_filter,
    excluded_labels = ?self.exclude_labels,
    "Label filtering excluded {} beads",
    before_label_filter - after_label_filter
);
```

### 5. Individual Excluded Beads
```rust
tracing::debug!(
    bead_id = %id,
    labels = ?labels,
    excluded_reasons = ?excluded_reasons,
    "Excluded bead due to labels"
);
```

### 6. Status Filtering
```rust
tracing::debug!(
    filtered_count = before_status_filter - after_status_filter,
    remaining = after_status_filter,
    "Status/assignee filtering removed {} beads",
    before_status_filter - after_status_filter
);
```

### 7. Candidate Sorting
```rust
tracing::debug!(
    total = candidates.len(),
    first_bead_id = %first.id,
    first_priority = first.priority,
    first_created_at = %first.created_at,
    "Sorting {} candidates by (priority ASC, created_at ASC, id ASC)",
    candidates.len()
);
```

### 8. Split Trigger Checking
```rust
tracing::debug!(
    bead_id = %first_candidate.id,
    failure_count = failure_count,
    threshold = self.split_after_failures,
    split_triggered = failure_count >= self.split_after_failures,
    "Checking split trigger for first candidate"
);
```

### 9. Split Disabled
```rust
tracing::debug!("Split trigger disabled (threshold = 0)");
```

### 10. No Candidates Remaining
```rust
tracing::debug!("No candidates remaining after filtering, returning NoWork");
```

## Default Excluded Labels

Pluck excludes the following labels by default (when `exclude_labels` is empty):
- `deferred` - Beads marked for later processing
- `human` - Beads requiring human intervention
- `blocked` - Beads blocked by dependencies

## Configuration

### Pluck Configuration in `~/.config/needle/config.yaml`
```yaml
strands:
  pluck:
    exclude_labels: []           # Empty = use defaults ["deferred", "human", "blocked"]
    split_after_failures: 3       # Auto-split after N consecutive failures (0 = disabled)
```

### Custom Exclude Labels Example
```yaml
strands:
  pluck:
    exclude_labels: ["deferred", "wip", "blocked"]
    split_after_failures: 5
```

## Tracing Fields Captured

The Pluck strand uses structured logging with the following fields:

- `exclude_labels`: Labels configured for exclusion
- `split_threshold`: Failure count threshold for auto-split
- `filters`: Filters applied to bead store query
- `count`: Number of candidates returned/processed
- `excluded_count`: Number of beads filtered by labels
- `remaining`: Number of candidates after filtering
- `filtered_count`: Number filtered by status/assignee
- `bead_id`: ID of specific bead being logged
- `labels`: Full set of labels on a bead
- `excluded_reasons`: Specific labels causing exclusion
- `first_bead_id`, `first_priority`, `first_created_at`: First candidate details
- `failure_count`: Consecutive failure count for split trigger
- `split_triggered`: Whether split threshold was reached

## Log Output

Logs are written to stderr and can be found in:
- **Interactive terminal**: Direct to stderr
- **Tmux sessions**: `~/.needle/logs/<session_name>.stderr.log`
- **Worker logs**: `~/.needle/logs/needle-<agent>-<worker_id>.stderr.log`

## Examples

### Enable Pluck Debug Logs for a Single Worker Run
```bash
RUST_LOG=needle::strand::pluck=debug needle run --workspace /path/to/workspace
```

### Monitor Pluck Behavior in Real-Time
```bash
# In one terminal
RUST_LOG=needle::strand::pluck=debug needle run

# In another terminal, tail the log
tail -f ~/.needle/logs/needle-*.stderr.log | grep "strand.pluck"
```

### Debug Filtering Issues
```bash
# Enable debug logs for both Pluck and the bead store
RUST_LOG=needle::strand::pluck=debug,needle::bead_store=debug needle run
```

## Summary

- **Primary control**: `RUST_LOG` environment variable
- **Pluck component path**: `needle::strand::pluck`
- **Default debug level**: `info` (use `debug` for filtering details)
- **Most verbose**: `trace` level
- **Key filtering debug events**: Label exclusion, status filtering, candidate sorting
- **Structured fields**: All relevant context (counts, labels, bead IDs) captured in log fields
- **Log location**: `~/.needle/logs/*.stderr.log` for worker sessions

The debug logging is comprehensive for understanding Pluck's filtering decisions, making it possible to diagnose why specific beads are or aren't being selected for processing.
