# Pluck Debug Flags and Logging Configuration

## Overview

Pluck is the primary strand in NEEDLE (the bead processing system) that handles bead selection and filtering. It uses Rust's `tracing` framework for structured logging, controlled via the `RUST_LOG` environment variable.

## Environment Variable: RUST_LOG

The `RUST_LOG` environment variable controls logging verbosity and scope. It uses the format:

```bash
RUST_LOG=module_path=log_level,module_path2=log_level2
```

### Log Levels (in order of verbosity)

- `error` - Errors only
- `warn` - Warnings and errors  
- `info` - Informational messages (default)
- `debug` - Detailed debugging information
- `trace` - Most detailed level (includes function entry/exit)

### Module Paths for Pluck

| Module Path | Description |
|-------------|-------------|
| `needle::strand::pluck` | **Primary Pluck strand** - filtering decisions, candidate selection |
| `needle::strand` | General strand operations |
| `needle::bead_store` | Bead store queries and operations |
| `needle::worker` | Worker state machine and processing |
| `needle::dispatch` | Agent dispatch and execution |

## Recommended Debug Configurations

### For Pluck Filtering Decisions (Recommended)

```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

This provides:
- **Trace** level for Pluck strand (most detailed filtering decisions)
- **Debug** level for related components
- Full visibility into which beads are filtered and why

### For Maximum Verbosity

```bash
export RUST_LOG="trace"
```

Enables trace-level logging across all modules.

### For Pluck-Specific Debugging

```bash
export RUST_LOG="needle::strand::pluck=debug"
```

Only enables debug logging for the Pluck strand itself.

## Key Debug Events Logged

The Pluck strand logs the following filtering decisions at **debug** level:

1. **Evaluation Start**
   - Configuration (exclude_labels, split_threshold)
   - Initial candidate query

2. **Label Filtering**
   - Number of beads excluded by label
   - Individual excluded bead IDs and reasons
   - Remaining candidate count

3. **Status/Assignee Filtering**
   - Removal of in-progress beads
   - Removal of stale assignee beads

4. **Candidate Sorting**
   - Total count being sorted
   - First candidate details (priority, created_at)

5. **Split Trigger Evaluation**
   - Failure count for first candidate
   - Split threshold comparison
   - Split trigger decision

6. **Final Results**
   - Number of candidates returned
   - List of candidate IDs
   - NoWork reasons

## Example Usage

### Manual Debug Session

```bash
# Set environment variable
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

# Run NEEDLE
needle run -w /path/to/workspace -c 1
```

### Using the Capture Script

ARMOR includes a helper script for capturing debug output:

```bash
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1
```

This automatically sets `RUST_LOG` and captures output to a file.

### Analyzing Debug Output

```bash
# View Pluck filtering decisions
grep -i 'pluck' output.log

# View filtering exclusions
grep -i 'exclude' output.log

# View candidate selection
grep -i 'candidate' output.log

# View split trigger events
grep -i 'split' output.log
```

## Filtering Decision Details

Pluck performs the following filtering operations (each logged separately):

1. **Label Exclusion** - Removes beads with excluded labels (default: `deferred`, `human`, `blocked`)
2. **Status Filter** - Removes beads in `InProgress` status
3. **Assignee Filter** - Removes `Open` beads with stale assignees
4. **Sorting** - Orders by `(priority ASC, created_at ASC, id ASC)`
5. **Split Check** - Triggers split if failure-count exceeds threshold

## Configuration Files

The `.needle.yaml` file in each workspace can configure Pluck behavior:

```yaml
exclude_labels:
  - deferred
  - human  
  - blocked
```

Custom exclude labels override the defaults and are logged at debug level.

## Tracing Instrumentation

Pluck uses structured tracing with the following fields:

- `strand` - Always "pluck"
- `exclude_labels` - Configured exclusion labels
- `split_threshold` - Failure count threshold for split trigger
- `bead_id` - Specific bead being evaluated
- `count` - Number of candidates
- `candidates` - List of candidate IDs

## Output Format

Debug output uses structured logging (JSON when OTLP is enabled, text format otherwise):

```
2026-07-09T00:32:15.123Z DEBUG needle::strand::pluck:pluck_strand: Pluck strand evaluation starting
  exclude_labels=["deferred", "human", "blocked"] split_threshold=3
```

## Related Documentation

- **Pluck Source**: `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Strand Architecture**: NEEDLE strand system documentation
- **Capture Script**: `capture-pluck-debug.sh` in ARMOR workspace

## Troubleshooting

### No debug output appearing

1. Verify `RUST_LOG` is set: `echo $RUST_LOG`
2. Check stderr is being captured: `needle run ... 2> output.log`
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