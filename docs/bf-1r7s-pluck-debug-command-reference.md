# Pluck Debug Command Structure - Complete Reference

**Bead:** bf-1r7s  
**Date:** 2026-07-09  
**Status:** ✅ Complete and Verified

## Executive Summary

This document provides the complete Pluck debug command structure with all required debug flags for comprehensive logging. The commands are verified against existing Pluck documentation and ready for immediate execution.

## Complete Pluck Command Structure

### Basic Command Template

```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count>
```

### Full Command with Output Capture

```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count> 2>&1 | tee <output_file>
```

### Command Components

| Component | Description | Required | Example |
|-----------|-------------|----------|---------|
| `RUST_LOG` | Environment variable controlling debug verbosity | Yes | `needle::strand::pluck=debug` |
| `needle` | The NEEDLE CLI binary | Yes | - |
| `run` | Command to execute NEEDLE worker | Yes | - |
| `-w` | Workspace path flag | Yes | `/home/coding/ARMOR` |
| `<workspace>` | Path to ARMOR workspace | Yes | `/home/coding/ARMOR` |
| `-c` | Run count flag | Yes | `1` |
| `<count>` | Number of cycles to run | Yes | `1` |
| `2>&1` | Redirect stderr to stdout | Optional* | - |
| `| tee` | Capture output to file while displaying | Optional* | - |
| `<output_file>` | Log output destination | Optional* | `pluck-debug.log` |

*Optional but recommended for debugging sessions

## Debug Levels and RUST_LOG Settings

### Level 1: minimal
```bash
RUST_LOG=needle::strand::pluck=info
```
- **Purpose:** Quick health checks, basic operation verification
- **Output:** High-level strand operations only
- **Verbosity:** Low
- **Use when:** Verifying Pluck strand is operational

### Level 2: standard (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug
```
- **Purpose:** Normal debugging, understanding filtering behavior
- **Output:** Filtering decisions and statistics
- **Verbosity:** Medium
- **Use when:** Investigating bead selection issues, filter behavior

### Level 3: detailed
```bash
RUST_LOG=needle::strand::pluck=trace
```
- **Purpose:** Deep troubleshooting, understanding exact flow
- **Output:** Complete execution details
- **Verbosity:** High
- **Use when:** Standard debug doesn't reveal the issue

### Level 4: comprehensive
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
```
- **Purpose:** Full context debugging, understanding system interactions
- **Output:** Pluck TRACE + supporting modules DEBUG
- **Verbosity:** Very High
- **Use when:** Need context from supporting modules (bead store queries, worker lifecycle)

### Level 5: full
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug
```
- **Purpose:** Complete system debugging
- **Output:** All critical NEEDLE modules at DEBUG/TRACE level
- **Verbosity:** Extreme
- **Use when:** Complex issues involving multiple modules

### Level 6: maximum
```bash
RUST_LOG=trace
```
- **Purpose:** Deep system-level debugging
- **Output:** Everything at TRACE level
- **Verbosity:** Maximum (very verbose)
- **Use when:** All else fails, need complete system visibility

## Ready-to-Execute Commands

### Quick Health Check
```bash
RUST_LOG=needle::strand::pluck=info needle run -w /home/coding/ARMOR -c 1
```

### Standard Debug Session (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-standard.log
```

### Deep Troubleshooting Session
```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-detailed.log
```

### Comprehensive Debug Session
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-comprehensive.log
```

### Full System Debug Session
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-full.log
```

### Maximum Verbosity Session
```bash
RUST_LOG=trace needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-maximum.log
```

## Using the Configuration Script

The `pluck-debug-config.sh` script provides a simplified interface:

### Script Location
```bash
/home/coding/ARMOR/pluck-debug-config.sh
```

### Script Usage
```bash
bash pluck-debug-config.sh [workspace] [output_file] [mode] [count]
```

### Script Examples
```bash
# Standard debug (recommended)
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log standard

# Comprehensive debug
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug-comprehensive.log comprehensive

# Detailed trace with multiple runs
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug-detailed.log detailed 3

# Quick health check
bash pluck-debug-config.sh /home/coding/ARMOR pluck-minimal.log minimal
```

### Available Script Modes
- `minimal` - INFO level
- `standard` - DEBUG level (default)
- `detailed` - TRACE level
- `comprehensive` - Multi-module TRACE
- `full` - All NEEDLE modules
- `maximum` - Global TRACE

## Expected Debug Output Events

When Pluck debug logging is enabled, you should see these events in order:

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

### Quick Analysis
```bash
# View all Pluck events
grep -i "pluck" pluck-debug.log

# Filter specific decisions
grep -i "filter" pluck-debug.log
grep -i "exclude" pluck-debug.log
grep -i "candidate" pluck-debug.log
grep -i "split" pluck-debug.log

# Count events
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "result=BeadFound" pluck-debug.log
grep -c "result=NoWork" pluck-debug.log
grep -c "result=Split" pluck-debug.log
```

### Detailed Analysis
```bash
# View complete filtering flow
grep -A 10 "Pluck strand evaluation starting" pluck-debug.log

# Check excluded beads with reasons
grep -A 5 "Filtering by excluded labels" pluck-debug.log

# Verify split decisions
grep -B 5 -A 5 "Checking split threshold" pluck-debug.log

# Count beads by result type
grep "result=" pluck-debug.log | sort | uniq -c
```

## Verification Checklist

✅ **Command Structure Verified**
- Command syntax matches NEEDLE CLI specification
- All required flags documented (`-w`, `-c`)
- Environment variable syntax correct (`RUST_LOG`)
- Output redirection syntax verified (`2>&1 | tee`)

✅ **Debug Flags Documented**
- All 6 debug levels with exact RUST_LOG settings
- Each level includes use case description
- Verbosity levels clearly defined

✅ **Ready for Execution**
- Commands are copy-paste ready
- Script interface documented
- Examples cover common use cases

✅ **Output Analysis**
- Expected events documented
- Analysis commands provided
- Troubleshooting guidance included

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

**Error message:**
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

### Command not found

**Error message:**
```
bash: needle: command not found
```

**Resolution:**
```bash
# Use full path to needle binary
/home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1

# Or add to PATH
export PATH="/home/coding/NEEDLE/target/release:$PATH"
```

## References

- **Configuration Script:** `/home/coding/ARMOR/pluck-debug-config.sh`
- **Quick Start Guide:** `/home/coding/ARMOR/pluck-debug-quickstart.md`
- **Full Configuration:** `/home/coding/ARMOR/pluck-debug-configuration.md`
- **NEEDLE Project:** `/home/coding/NEEDLE/`

## Status

✅ **Pluck command structure:** Complete  
✅ **Debug flags documented:** All 6 levels  
✅ **Command syntax verified:** Against NEEDLE documentation  
✅ **Ready for execution:** Yes  

**Acceptance Criteria Status:**
- ✅ Complete Pluck command with debug flags documented
- ✅ Command syntax verified against Pluck documentation
- ✅ Command ready for execution

The Pluck debug command structure is fully documented and ready for use.
