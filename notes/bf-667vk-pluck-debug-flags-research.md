# Pluck Debug Flags Research and Command Construction

**Bead:** bf-667vk  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**Task:** Research Pluck debug flags and construct command

## Overview

Pluck is a NEEDLE strand responsible for selecting work beads from the bead store. It filters beads by labels, assignee, and priority, then determines whether to return work, split, or indicate no work available. Comprehensive debug logging is available via the `RUST_LOG` environment variable.

## Available Debug Flags

### RUST_LOG Environment Variable

The `RUST_LOG` variable controls crate-level logging in Rust applications. For NEEDLE's Pluck strand, it supports module-specific logging levels:

**Logging Levels (in order of verbosity):**
- `error` - Critical errors only
- `warn` - Warning messages
- `info` - High-level operational information
- `debug` - Detailed execution flow (recommended for debugging)
- `trace` - Extremely detailed execution trace

### Module-Specific Flags

**NEEDLE modules that can be targeted:**
- `needle::strand::pluck` - The Pluck strand itself (primary target)
- `needle::strand` - All strand operations (general strand framework)
- `needle::bead_store` - Bead store queries and operations
- `needle::worker` - Worker lifecycle and state management
- `needle::dispatch` - Agent dispatch and execution
- `needle::claim` - Bead claiming operations

### Preset Configurations

Six pre-configured presets are available, ranging from minimal to maximum verbosity:

#### 1. Minimal (INFO level)
```bash
RUST_LOG=needle::strand::pluck=info
```
- **Output:** High-level strand operations only
- **Use Case:** Quick health checks, basic operation verification
- **Verbosity:** Lowest

#### 2. Standard (DEBUG level) - **RECOMMENDED**
```bash
RUST_LOG=needle::strand::pluck=debug
```
- **Output:** Filtering decisions and statistics
- **Use Case:** Normal debugging, understanding filtering behavior
- **Verbosity:** Low to moderate
- **When to use:** Default choice for most debugging scenarios

#### 3. Detailed (TRACE level)
```bash
RUST_LOG=needle::strand::pluck=trace
```
- **Output:** Complete execution details for Pluck strand
- **Use Case:** Deep troubleshooting, understanding exact flow
- **Verbosity:** Moderate to high
- **When to use:** When standard debug doesn't provide enough detail

#### 4. Comprehensive (Multi-module TRACE/DEBUG)
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
```
- **Output:** Pluck TRACE + supporting modules DEBUG
- **Use Case:** Full context debugging, understanding system interactions
- **Verbosity:** High
- **When to use:** When you need to see how Pluck interacts with other components

#### 5. Full (All NEEDLE modules)
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug
```
- **Output:** All critical modules at DEBUG/TRACE level
- **Use Case:** Complete system debugging
- **Verbosity:** Very high
- **When to use:** For comprehensive system-level debugging

#### 6. Maximum (Global TRACE)
```bash
RUST_LOG=trace
```
- **Output:** Everything at TRACE level
- **Use Case:** Deep system-level debugging
- **Verbosity:** Maximum (extremely verbose)
- **When to use:** Only when absolutely necessary; produces massive output

## Command Construction

### Basic Command Structure

```bash
export RUST_LOG="<selected_configuration>"
needle run -w /home/coding/ARMOR -c 1
```

### Full Command with Output Capture

```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
timeout 180s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-output.log
```

### Command Breakdown

**Component breakdown:**

1. **`export RUST_LOG="..."`**
   - Sets the environment variable for the current shell session
   - Controls what gets logged
   - Multiple modules are comma-separated
   - Syntax: `module_path=log_level`

2. **`timeout 180s`**
   - Limits execution to 180 seconds (3 minutes)
   - Prevents indefinite hangs
   - Exit code 124 indicates timeout was triggered
   - Can be adjusted based on expected execution time

3. **`needle run`**
   - The main NEEDLE command
   - Invokes the NEEDLE worker system

4. **`-w /home/coding/ARMOR`**
   - Specifies the workspace directory
   - NEEDLE reads beads from this location
   - Must be a valid ARMOR workspace with `.beads/` directory

5. **`-c 1`**
   - Specifies the cycle count (number of work cycles)
   - `1` = single execution cycle
   - Can be increased for multiple iterations

6. **`2>&1`**
   - Redirects stderr (2) to stdout (1)
   - Combines error and standard output
   - Ensures all logs are captured together

7. **`| tee pluck-debug-output.log`**
   - `tee` writes to both file and terminal
   - Allows real-time monitoring while logging
   - Output file can be specified with any path

## Recommended Configuration for Comprehensive Debugging

Based on the research and previous executions, the **comprehensive** configuration provides the best balance of detail and manageability:

```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

### Rationale for This Configuration

**Why these specific modules:**

1. **`needle::strand::pluck=trace`**
   - Pluck is the primary focus
   - TRACE level shows every decision point
   - Reveals filtering logic, candidate evaluation, and split decisions

2. **`needle::strand=debug`**
   - Shows strand framework operations
   - Provides context for strand loading and initialization
   - Helps understand strand lifecycle

3. **`needle::bead_store=debug`**
   - Shows bead store queries
   - Reveals what beads are being considered
   - Helps understand filtering at the data source level

4. **`needle::worker=debug`**
   - Shows worker lifecycle (BOOTING → SELECTING → BUILDING → DISPATCHING → EXECUTING)
   - Provides execution context
   - Helps understand when and why Pluck is invoked

5. **`needle::dispatch=debug`**
   - Shows agent dispatch operations
   - Reveals what happens after Pluck selects a bead
   - Provides complete execution flow visibility

## Expected Debug Output

When comprehensive logging is enabled, you should see:

### 1. Strand Evaluation Start
```
TRACE needle::strand::pluck: Pluck strand evaluation starting
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
DEBUG needle::strand::pluck: Filtering by excluded labels
  excluded_beads=["bf-1234", "bf-5678"]
  reasons=["label:deferred", "label:blocked"]
```

### 5. Status/Assignee Filtering
```
DEBUG needle::strand::pluck: Filtering by status and assignee
  remaining=3
```

### 6. Sorting
```
DEBUG needle::strand::pluck: Sorting candidates by priority
  first_candidate="bf-abcd"
```

### 7. Split Decision
```
DEBUG needle::strand::pluck: Checking split threshold
  failure_count=2
  split_threshold=3
  should_split=false
```

### 8. Final Result
```
DEBUG needle::strand::pluck: Strand evaluation complete
  result=BeadFound("bf-abcd")
```

## Alternative Configurations

### Quick Health Check
```bash
export RUST_LOG="needle::strand::pluck=info"
timeout 30s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-health-check.log
```

### Standard Debugging
```bash
export RUST_LOG="needle::strand::pluck=debug"
timeout 60s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-standard-debug.log
```

### Maximum Verbosity (Use with Caution)
```bash
export RUST_LOG="trace"
timeout 180s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-maximum-verbosity.log
```

## Log Analysis Commands

After capturing logs, use these commands to analyze specific aspects:

### View All Pluck Events
```bash
grep -i "pluck" pluck-debug-output.log
```

### Filter Specific Decisions
```bash
grep -i "filter" pluck-debug-output.log
grep -i "exclude" pluck-debug-output.log
grep -i "candidate" pluck-debug-output.log
grep -i "split" pluck-debug-output.log
```

### Count Events by Type
```bash
grep -c "Pluck strand evaluation starting" pluck-debug-output.log
grep -c "result=BeadFound" pluck-debug-output.log
grep -c "result=NoWork" pluck-debug-output.log
grep -c "result=Split" pluck-debug-output.log
```

### View Worker Lifecycle
```bash
grep -i "worker" pluck-debug-output.log | grep -i "state\|booting\|executing"
```

## Configuration Tools

### Automated Configuration Script

A configuration script is available at:
```
/home/coding/ARMOR/pluck-debug-config.sh
```

**Usage:**
```bash
# Standard debugging
./pluck-debug-config.sh /home/coding/ARMOR output.log standard

# Comprehensive debugging
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive

# Show help
./pluck-debug-config.sh --help
```

### Manual Execution Scripts

Several execution scripts are available for specific scenarios:
```
execute-pluck-capture.sh           - Simple capture with comprehensive logging
execute-pluck-bf-<bead-id>.sh     - Bead-specific execution with detailed logging
```

## Execution Best Practices

### 1. Start with Standard Debug
Begin with `needle::strand::pluck=debug` and only increase to TRACE if needed.

### 2. Use Timeout
Always use `timeout` to prevent indefinite execution:
```bash
timeout 180s needle run ...
```

### 3. Capture Output
Always use `tee` to capture output while monitoring in real-time:
```bash
... 2>&1 | tee output.log
```

### 4. Separate Logs if Needed
For detailed analysis, separate stdout and stderr:
```bash
... > >(tee stdout.log) 2> >(tee stderr.log >&2)
```

### 5. Use Appropriate Cycle Count
- `-c 1` for single execution (most debugging)
- `-c N` for multiple iterations (load testing)

## Troubleshooting

### No Pluck Output Visible

**Check 1: Verify RUST_LOG is set**
```bash
echo $RUST_LOG
```

**Check 2: Verify Pluck strand is active**
```bash
grep "worker booted" pluck-debug-output.log | grep -i "pluck"
```

**Check 3: Ensure beads are available**
```bash
br list --status=open
```

### Bead Store Query Failed

**Error:**
```
ERROR needle::strand::pluck: Bead store query failed error=bf list failed
```

**Resolution:**
```bash
# Check bead store integrity
cd /home/coding/ARMOR
br doctor --repair
```

## Summary

### Recommended Full Command for Comprehensive Debugging

```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
timeout 180s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee logs/pluck-debug/pluck-debug-comprehensive.log
```

### Command Rationale

1. **Comprehensive but manageable verbosity** - TRACE for Pluck, DEBUG for supporting modules
2. **180 second timeout** - Sufficient for full execution cycle
3. **Single cycle** - Focus on one execution for clarity
4. **Output capture** - Enables post-execution analysis
5. **Organized log location** - Logs stored in `logs/pluck-debug/` directory

### Next Steps

1. Execute the recommended command
2. Analyze the captured logs using the provided grep commands
3. Adjust verbosity level based on findings
4. Document any anomalies or unexpected behavior

## Status

✅ **Debug flags research:** Complete  
✅ **Available configurations documented:** Yes  
✅ **Command construction documented:** Yes  
✅ **Rationale provided:** Yes  
✅ **Ready for execution:** Yes

The Pluck debug flags have been researched, documented, and the full command with appropriate logging flags has been constructed and is ready for execution in the next step.
