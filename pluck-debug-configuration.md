# Pluck Debug Logging Configuration

**Bead:** bf-3b63  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR

## Overview

This configuration enables comprehensive debug logging for the Pluck strand filtering decision process. The setup provides multiple preset configurations ranging from minimal logging to full trace-level output.

## Configuration Script

The main configuration script is located at:
```
/home/coding/ARMOR/pluck-debug-config.sh
```

This script provides preset configurations for different debug levels and handles log capture automatically.

## Available Debug Presets

### 1. **minimal** - INFO level
- **RUST_LOG:** `needle::strand::pluck=info`
- **Output:** High-level strand operations only
- **Use case:** Quick health checks, basic operation verification

### 2. **standard** - DEBUG level (Recommended)
- **RUST_LOG:** `needle::strand::pluck=debug`
- **Output:** Filtering decisions and statistics
- **Use case:** Normal debugging, understanding filtering behavior

### 3. **detailed** - TRACE level
- **RUST_LOG:** `needle::strand::pluck=trace`
- **Output:** Complete execution details
- **Use case:** Deep troubleshooting, understanding exact flow

### 4. **comprehensive** - Multi-module TRACE
- **RUST_LOG:** `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug`
- **Output:** Pluck TRACE + supporting modules DEBUG
- **Use case:** Full context debugging, understanding system interactions

### 5. **full** - All NEEDLE modules
- **RUST_LOG:** `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug`
- **Output:** All critical modules at DEBUG/TRACE level
- **Use case:** Complete system debugging

### 6. **maximum** - Global TRACE
- **RUST_LOG:** `trace`
- **Output:** Everything at TRACE level (very verbose)
- **Use case:** Deep system-level debugging

## Usage

### Basic Usage

```bash
# Run with standard debug level (recommended)
./pluck-debug-config.sh /home/coding/ARMOR output.log standard

# Run with comprehensive debug level
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive

# Run with detailed trace level
./crawl-debug-config.sh /home/coding/ARMOR output.log detailed
```

### Advanced Usage

```bash
# Specify workspace, output file, mode, and run count
./pluck-debug-config.sh /home/coding/ARMOR custom-output.log full 3

# Show help
./pluck-debug-config.sh --help
```

## Expected Debug Output

When Pluck debug logging is enabled, you should see the following events:

### 1. Strand Evaluation Start
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

## Manual Configuration

If you prefer to set up the environment manually:

```bash
# Set the desired debug level
export RUST_LOG=needle::strand::pluck=debug

# Run NEEDLE
cd /home/coding/NEEDLE
cargo run -- run -w /home/coding/ARMOR -c 1

# Or with output capture
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

## Log Analysis

After capturing logs, analyze them with these commands:

```bash
# View all Pluck-related events
grep -i "pluck" pluck-debug.log

# Filter specific decisions
grep -i "filter" pluck-debug.log

# Check excluded beads
grep -i "exclude" pluck-debug.log

# View candidate processing
grep -i "candidate" pluck-debug.log

# Check split decisions
grep -i "split" pluck-debug.log

# Count events by type
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "result=BeadFound" pluck-debug.log
grep -c "result=NoWork" pluck-debug.log
grep -c "result=Split" pluck-debug.log
```

## Troubleshooting

### No Pluck output visible

1. **Check RUST_LOG is set correctly:**
   ```bash
   echo $RUST_LOG
   ```

2. **Verify Pluck strand is active:**
   ```bash
   grep "worker booted" pluck-debug.log | grep "pluck"
   ```

3. **Ensure beads are available for processing:**
   ```bash
   br list --status=open
   ```

### Bead store query failed

This error indicates the Pluck strand cannot access the bead store:
```
ERROR needle::strand::pluck: Bead store query failed error=bf list failed
```

**Possible causes:**
- Bead store locked by another process
- Corrupted bead database
- Permission issues

**Resolution:**
```bash
# Check bead store integrity
cd /home/coding/ARMOR
br doctor --repair
```

## Configuration Files

### Script Configuration
The main configuration is in the script itself:
- **Location:** `/home/coding/ARMOR/pluck-debug-config.sh`
- **Configuration:** Lines 20-27 contain the preset definitions

### Environment Variables
- **RUST_LOG:** Controls Rust crate-level logging
- **RUST_BACKTRACE:** Set to `1` for backtraces on errors

## Integration with NEEDLE

The Pluck strand is part of the standard NEEDLE strand set:
```
strands=["pluck", "mend", "explore", "weave", "unravel", "pulse", "reflect", "splice", "knot"]
```

Pluck is the first strand evaluated and is responsible for:
1. Querying the bead store for ready beads
2. Filtering by labels (deferred, human, blocked)
3. Filtering by assignee
4. Sorting candidates by priority
5. Checking for split conditions
6. Returning NoWork/BeadFound/Split result

## Next Steps

Once debug logging is configured:

1. **Run a test capture:**
   ```bash
   ./pluck-debug-config.sh /home/coding/ARMOR test-output.log standard
   ```

2. **Analyze the output:**
   ```bash
   grep "Pluck strand evaluation starting" test-output.log
   ```

3. **Filter for specific decisions:**
   ```bash
   grep -A 5 "Filtering by" test-output.log
   ```

4. **Verify expected behavior:**
   - Check that excluded labels are properly filtered
   - Verify split threshold logic
   - Confirm candidate selection order

## Status

✅ **Debug logging infrastructure:** Configured and operational  
✅ **Configuration script:** Created and executable  
✅ **Preset configurations:** 6 levels available  
✅ **Documentation:** Complete  
✅ **Ready for execution:** Yes  

The Pluck debug logging configuration is complete and ready for use.