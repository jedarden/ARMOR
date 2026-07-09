# Pluck Command Structure Reference

**Bead:** bf-1r7s
**Date:** 2026-07-09
**Status:** ✅ Complete

## Overview

Pluck is a strand within the NEEDLE system that handles bead selection, filtering, and processing decisions. This reference provides the complete command structure for running NEEDLE with Pluck debug logging enabled.

## Complete Pluck Command Structure

### Basic Command Template

```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count> [additional_options]
```

### Full Command Template

```bash
RUST_LOG=<debug_level> needle run \
  --workspace <workspace> \
  --count <count> \
  --agent <agent> \
  --identifier <identifier> \
  --timeout <timeout> \
  --resume \
  --hot-reload <true|false>
```

## Command Components

### 1. Environment Variables

#### RUST_LOG (Primary Debug Control)

The `RUST_LOG` environment variable controls debug logging output at the crate/module level.

**Syntax:** `RUST_LOG=<module_path>=<level>[,<module_path>=<level>...]`

**Available Levels:**
- `error` - Only errors
- `warn` - Warnings and errors  
- `info` - Informational messages and above
- `debug` - Debug messages and above (recommended for most debugging)
- `trace` - Trace messages and above (most verbose)

**Module Paths for Pluck:**
- `needle::strand::pluck` - Pluck strand specifically
- `needle::strand` - All strand operations
- `needle::bead_store` - Bead database operations
- `needle::worker` - Worker lifecycle and state management
- `needle::dispatch` - Agent dispatch and execution
- `needle::claim` - Bead claiming operations
- `trace` - Global trace (everything at trace level)

### 2. NEEDLE Command Options

#### Required Options

| Option | Short | Parameter | Description | Example |
|--------|-------|------------|-------------|---------|
| `--workspace` | `-w` | `<WORKSPACE>` | Workspace directory containing beads | `-w /home/coding/ARMOR` |

#### Optional Options

| Option | Short | Parameter | Description | Default | Example |
|--------|-------|------------|-------------|---------|---------|
| `--count` | `-c` | `<COUNT>` | Number of workers to launch | `1` | `-c 3` |
| `--agent` | `-a` | `<AGENT>` | Agent adapter to use | Configured default | `-a claude-code-glm-4.7` |
| `--identifier` | `-i` | `<IDENTIFIER>` | Worker identifier (overrides NATO naming) | Auto-generated | `-i my-worker` |
| `--timeout` | `-t` | `<TIMEOUT>` | Agent execution timeout in seconds | Configured default | `-t 300` |
| `--resume` | - | - | Resume an existing worker session | `false` | `--resume` |
| `--hot-reload` | - | `<true\|false>` | Enable hot-reload for this worker | Configured default | `--hot-reload true` |

## Debug Configuration Presets

### Preset 1: Minimal (INFO level)
**Use case:** Quick health checks, basic operation verification

```bash
RUST_LOG=needle::strand::pluck=info needle run -w /home/coding/ARMOR -c 1
```

**Output:** High-level strand operations only

### Preset 2: Standard (DEBUG level) - Recommended
**Use case:** Normal debugging, understanding filtering behavior

```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1
```

**Output:** Filtering decisions and statistics

### Preset 3: Detailed (TRACE level)
**Use case:** Deep troubleshooting, understanding exact flow

```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1
```

**Output:** Complete execution details

### Preset 4: Comprehensive
**Use case:** Full context debugging, understanding system interactions

```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug needle run -w /home/coding/ARMOR -c 1
```

**Output:** Pluck TRACE + supporting modules DEBUG

### Preset 5: Full System
**Use case:** Complete system debugging

```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug needle run -w /home/coding/ARMOR -c 1
```

**Output:** All critical modules at DEBUG/TRACE level

### Preset 6: Maximum
**Use case:** Deep system-level debugging

```bash
RUST_LOG=trace needle run -w /home/coding/ARMOR -c 1
```

**Output:** Everything at TRACE level (very verbose)

## Configuration File

Pluck behavior can also be configured via `.needle.yaml` in the workspace:

```yaml
strands:
  pluck:
    # Labels to exclude when selecting beads
    exclude_labels: []
    
    # Auto-split beads after N consecutive failures (0 = disabled)
    split_after_failures: 0
```

## Practical Usage Examples

### Example 1: Standard Debug with Log Capture
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### Example 2: Comprehensive Debug with Multiple Workers
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug needle run -w /home/coding/ARMOR -c 3 2>&1 | tee pluck-comprehensive.log
```

### Example 3: Custom Agent with Timeout
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 --agent claude-code-glm-4.7 --timeout 600 2>&1 | tee pluck-custom-agent.log
```

### Example 4: Named Worker with Detailed Logging
```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1 --identifier debug-worker 2>&1 | tee pluck-detailed.log
```

### Example 5: Quick Test Run (Minimal Output)
```bash
RUST_LOG=needle::strand::pluck=info needle run -w /home/coding/ARMOR -c 1
```

## Expected Debug Output

When Pluck debug logging is enabled at `debug` level or higher, you should see:

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

## Log Analysis Commands

### View All Pluck Events
```bash
grep -i "pluck" pluck-debug.log
```

### Filter Specific Decisions
```bash
grep -i "filter" pluck-debug.log
grep -i "exclude" pluck-debug.log
grep -i "candidate" pluck-debug.log
grep -i "split" pluck-debug.log
```

### Count Events
```bash
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "result=BeadFound" pluck-debug.log
grep -c "result=NoWork" pluck-debug.log
grep -c "result=Split" pluck-debug.log
```

### View Filtering Details
```bash
grep -A 5 "Filtering by" pluck-debug.log
```

## Troubleshooting

### No Pluck Output Visible

1. **Check RUST_LOG is set correctly:**
   ```bash
   echo $RUST_LOG
   ```

2. **Verify Pluck strand is active:**
   ```bash
   grep "worker booted" pluck-debug.log | grep "pluck"
   ```

3. **Ensure beads are available:**
   ```bash
   br list --status=open
   ```

### Bead Store Query Failed

**Error:** `ERROR needle::strand::pluck: Bead store query failed error=bf list failed`

**Possible causes:**
- Bead store locked by another process
- Corrupted bead database  
- Permission issues

**Resolution:**
```bash
cd /home/coding/ARMOR
br doctor --repair
```

### Command Not Found

If `needle` command is not found, ensure NEEDLE is installed and in PATH:

```bash
# Check if needle is in PATH
which needle

# If not, add to PATH or use full path
export PATH="$PATH:/home/coding/NEEDLE/target/release"
```

## Integration with NEEDLE Ecosystem

Pluck is part of the standard NEEDLE strand set:
```
strands=["pluck", "mend", "explore", "weave", "unravel", "pulse", "reflect", "splice", "knot"]
```

Pluck is the **first strand evaluated** and is responsible for:
1. Querying the bead store for ready beads
2. Filtering by labels (deferred, human, blocked)
3. Filtering by assignee
4. Sorting candidates by priority
5. Checking for split conditions
6. Returning NoWork/BeadFound/Split result

## Related Documentation

- **Full Debug Configuration:** `/home/coding/ARMOR/pluck-debug-configuration.md`
- **Quick Start Guide:** `/home/coding/ARMOR/pluck-debug-quickstart.md`
- **Configuration Script:** `/home/coding/ARMOR/pluck-debug-config.sh`
- **Capture Script:** `/home/coding/ARMOR/capture-pluck-debug.sh`

## Command Reference Summary

### Minimal Viable Command
```bash
needle run -w /home/coding/ARMOR -c 1
```

### Standard Debug Command (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1
```

### Comprehensive Debug Command
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

## Status

✅ **Command structure documented:** Complete  
✅ **Debug flags identified:** All 6 presets documented  
✅ **Configuration options verified:** Comprehensive  
✅ **Usage examples provided:** Multiple scenarios covered  
✅ **Command syntax verified:** Verified against NEEDLE documentation  

The Pluck command structure reference is complete and ready for use.
