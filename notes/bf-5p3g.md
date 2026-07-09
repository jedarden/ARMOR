# Pluck Debug Flags and Logging Configuration

## Overview

Pluck is the primary bead selection strand in NEEDLE, handling >90% of all bead processing. It uses Rust's `tracing` framework for comprehensive debug logging that can be enabled at runtime via environment variables.

## Primary Debug Environment Variable: `RUST_LOG`

The `RUST_LOG` environment variable controls logging verbosity and scope. It uses the format:
```
RUST_LOG=module_path=level
```

### Available Log Levels

- `error` - Critical failures only
- `warn` - Warning messages
- `info` - General informational messages (default)
- `debug` - Detailed execution flow
- `trace` - Maximum verbosity, every operation

## Pluck-Specific Debug Targets

### Core Pluck Module

**Target:** `needle::strand::pluck`

**Levels:**
- `RUST_LOG=needle::strand::pluck=debug` - Filtering decisions and candidate counts
- `RUST_LOG=needle::strand::pluck=trace` - Detailed execution flow

### Related Module Targets

- `needle::strand` - All strand operations (Pluck, Mend, Explore, etc.)
- `needle::bead_store` - Bead store queries and operations
- `needle::worker` - Worker coordination logic
- `needle::dispatch` - Work dispatch operations

## Recommended Debug Configurations

### 1. Minimal Pluck Debug (Filtering Focus)
```bash
export RUST_LOG=needle::strand::pluck=debug
```
**Use case:** Quick debugging of filtering decisions without excessive output

### 2. Comprehensive Pluck Trace
```bash
export RUST_LOG=needle::strand::pluck=trace
```
**Use case:** Deep dive into Pluck execution flow

### 3. Full Strand Context
```bash
export RUST_LOG=needle::strand=debug,needle::strand::pluck=trace
```
**Use case:** Understanding Pluck within the broader strand waterfall

### 4. Complete Worker Context (RECOMMENDED)
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```
**Use case:** Full visibility into bead selection, claiming, and dispatch

### 5. Maximum Debug Output (Not Recommended)
```bash
export RUST_LOG=debug
```
**Use case:** Last resort for elusive bugs; produces massive output

## What Gets Logged at Each Level

### DEBUG Level (`needle::strand::pluck=debug`)

- Evaluation start with configuration parameters
- Bead store query filters
- Candidate counts after each filtering stage
- Label filtering exclusions with per-bead details
- Status/assignee filtering results
- Sorting operations with first candidate details
- Split trigger checks with decision logic
- Final candidate list returned for processing

**Example output:**
```
DEBUG strand.pluck: Pluck strand evaluation starting exclude_labels=["deferred", "human", "blocked"] split_threshold=3
DEBUG strand.pluck: Querying bead store for ready candidates
DEBUG strand.pluck: Bead store returned 42 candidates
DEBUG strand.pluck: Label filtering excluded 3 beads remaining=39
DEBUG strand.pluck: Excluded bead due to labels bead_id="bf-123" labels=["deferred"] excluded_reasons=["deferred"]
```

### TRACE Level (`needle::strand::pluck=trace`)

Everything from DEBUG plus:
- Detailed execution flow between filtering stages
- Individual bead examination during filtering
- Split trigger evaluation details
- Granular sorting operation details

## Filtering Decision Logging

Pluck logs three specific filtering operations that determine which beads are claimable:

### 1. Label Filtering
```
DEBUG strand.pluck: Label filtering excluded {count} beads remaining={after_count}
```
Logs each excluded bead with:
- `bead_id` - The excluded bead's ID
- `labels` - All labels on the bead
- `excluded_reasons` - Which labels triggered exclusion

### 2. Status/Assignee Filtering
```
DEBUG strand.pluck: Status/assignee filtering removed {count} beads remaining={after_count}
```
Removes beads that are:
- In `InProgress` status (claimed by another worker)
- `Open` status but have a stale assignee

### 3. Split Trigger Check
```
DEBUG strand.pluck: Checking split trigger for first candidate bead_id={id} failure_count={count} threshold={threshold} split_triggered={bool}
```

## Configuration Files

### Workspace-Level Override
Create `.needle.yaml` in your workspace root:
```yaml
strands:
  pluck:
    exclude_labels: []          # Empty = use defaults ["deferred", "human", "blocked"]
    split_after_failures: 3     # Auto-split beads after N consecutive failures
```

### Global Configuration
`~/.config/needle/config.yaml` - Workspace configs override these settings

## Usage Examples

### Direct NEEDLE Execution
```bash
# Enable Pluck debug logging
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1

# Full worker context
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug needle run -w /home/coding/ARMOR -c 1
```

### Using the ARMOR Capture Script
```bash
# The ARMOR workspace includes a pre-configured capture script
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug.log 1
```

This script:
1. Sets comprehensive debug logging
2. Runs NEEDLE with the specified workspace
3. Captures all output to a timestamped log file
4. Provides grep commands for analyzing specific aspects

## Log Analysis Commands

After capturing debug output, analyze it with:

```bash
# Filter for Pluck-specific messages
grep -i 'pluck' pluck-debug.log

# Find filtering decisions
grep -i 'filter' pluck-debug.log

# Find excluded beads
grep -i 'exclude' pluck-debug.log

# Find candidate counts
grep -i 'candidate' pluck-debug.log

# Find split triggers
grep -i 'split' pluck-debug.log
```

## Key Implementation Details

### Default Exclude Labels
When `exclude_labels` is empty (default), Pluck uses:
```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked"];
```

### Deterministic Sorting
Candidates are sorted by:
```rust
// (priority ASC, created_at ASC, id ASC)
candidates.sort_by(|a, b| {
    a.priority.cmp(&b.priority)
        .then_with(|| a.created_at.cmp(&b.created_at))
        .then_with(|| a.id.as_ref().cmp(b.id.as_ref()))
});
```

### Split Trigger Logic
```rust
if failure_count >= self.split_after_failures {
    return StrandResult::Split(Box::new(first_candidate.clone()), failure_count);
}
```

## Related Files

- **Core Implementation:** `/home/coding/NEEDLE/src/strand/pluck.rs` (917 lines)
- **Strand Waterfall:** `/home/coding/NEEDLE/src/strand/mod.rs` (901 lines)
- **Configuration:** `/home/coding/NEEDLE/src/config/mod.rs`
- **Bead Store:** `/home/coding/NEEDLE/src/bead_store/mod.rs`
- **ARMOR Workspace Config:** `/home/coding/ARMOR/.env.pluck-debug`
- **ARMOR Capture Script:** `/home/coding/ARMOR/capture-pluck-debug.sh`

## Recent Enhancements

### Commit `5002562` (2026-07-09)
"feat(pluck): add comprehensive debug logging"

Added 115 lines of detailed tracing events throughout the Pluck filtering pipeline:
- Evaluation start with configuration
- Bead store query results
- Label filtering decisions with per-bead details
- Status/assignee filtering results
- Sorting operation details
- Split trigger check with decision
- Final result with candidate list

This enhancement enables runtime debugging via `RUST_LOG=needle::strand::pluck=debug` without code changes.

## Summary

Pluck's debug logging is controlled via the `RUST_LOG` environment variable with module-specific targets. The recommended configuration for comprehensive debugging is:

```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

This provides full visibility into:
- Filtering decisions (label, status, assignee)
- Candidate counts at each stage
- Split trigger evaluation
- Strand waterfall execution
- Bead store operations
- Worker coordination
