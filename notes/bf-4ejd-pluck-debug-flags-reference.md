# Pluck Debug Flags Reference

**Bead:** bf-4ejd
**Date:** 2026-07-09
**Task:** Identify Pluck debug flags and command structure

## Overview

Pluck (part of the NEEDLE system) uses the Rust `RUST_LOG` environment variable for debug logging. There are no CLI debug flags - all debug control is via environment variables.

## Primary Debug Flags

### RUST_LOG (Primary Debug Control)

**Purpose:** Controls Rust crate-level logging verbosity

**Format:** `RUST_LOG=<module_path>=<level>`

**Available Levels:**
- `error` - Only errors
- `warn` - Warnings and errors
- `info` - High-level operations
- `debug` - Detailed debugging information
- `trace` - Complete execution trace (most verbose)

### RUST_BACKTRACE (Error Debugging)

**Purpose:** Enables stack traces on errors

**Values:**
- `0` or unset - No backtraces (default)
- `1` - Full backtraces on errors

## Pluck-Specific Module Paths

### Core Pluck Module
```
needle::strand::pluck
```

### Supporting Modules
```
needle::strand          - General strand operations
needle::bead_store      - Bead database operations
needle::worker          - Worker lifecycle
needle::dispatch        - Bead dispatch logic
needle::claim           - Bead claiming logic
```

## Recommended Configurations

### 1. Standard Debugging (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug
```
**Output:** Filtering decisions, statistics, candidate processing

### 2. Detailed Trace
```bash
RUST_LOG=needle::strand::pluck=trace
```
**Output:** Complete execution flow, all decision points

### 3. Comprehensive (Multi-Module)
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
```
**Output:** Pluck trace + supporting modules at debug level

### 4. Full System Debug
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug
```
**Output:** All critical NEEDLE modules at DEBUG/TRACE

### 5. Maximum Verbosity
```bash
RUST_LOG=trace
```
**Output:** Everything at TRACE level (very verbose, all Rust crates)

### 6. Minimal Logging
```bash
RUST_LOG=needle::strand::pluck=info
```
**Output:** High-level operations only

## Command Structure

### Basic Command Pattern
```bash
RUST_LOG=<level> needle run -w <workspace> -c <count>
```

### With Output Capture
```bash
RUST_LOG=<level> needle run -w <workspace> -c <count> 2>&1 | tee <output.log>
```

### With Multiple Modules
```bash
RUST_LOG="module1=level1,module2=level2" needle run -w <workspace> -c <count>
```

## needle run CLI Options

### Required Options
- `-w, --workspace <WORKSPACE>` - Path to workspace directory
- `-c, --count <COUNT>` - Number of workers to launch [default: 1]

### Optional Options
- `-a, --agent <AGENT>` - Agent adapter to use
- `-i, --identifier <IDENTIFIER>` - Worker identifier (overrides NATO naming)
- `-t, --timeout <TIMEOUT>` - Agent execution timeout in seconds
- `--resume` - Resume an existing worker session
- `--hot-reload <true|false>` - Enable hot-reload for this worker

## Complete Examples

### Example 1: Standard Debug (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1
```

### Example 2: Detailed Trace with Output Capture
```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-trace.log
```

### Example 3: Comprehensive Multi-Module Debug
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug" needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-comprehensive.log
```

### Example 4: With Backtrace on Errors
```bash
RUST_LOG=needle::strand::pluck=debug RUST_BACKTRACE=1 needle run -w /home/coding/ARMOR -c 1
```

### Example 5: Full System Debug
```bash
RUST_LOG="trace" needle run -w /home/coding/ARMOR -c 1 2>&1 | tee full-trace.log
```

## Automated Configuration Script

A helper script is available at `/home/coding/ARMOR/pluck-debug-config.sh` with preset configurations:

```bash
# Standard debug (recommended)
./pluck-debug-config.sh /home/coding/ARMOR output.log standard

# Detailed trace
./pluck-debug-config.sh /home/coding/ARMOR output.log detailed

# Comprehensive multi-module
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive

# Full system debug
./pluck-debug-config.sh /home/coding/ARMOR output.log full
```

Available presets:
- `minimal` - INFO level
- `standard` - DEBUG level (recommended)
- `detailed` - TRACE level
- `comprehensive` - TRACE + supporting modules
- `full` - All NEEDLE modules DEBUG/TRACE
- `maximum` - Global TRACE

## Expected Debug Output Events

When Pluck debug logging is enabled at `debug` or `trace` level, you should see:

1. **Strand evaluation start**
   - Exclude labels
   - Split threshold

2. **Bead store query**
   - Filter parameters
   - Candidate count

3. **Label filtering**
   - Excluded beads
   - Exclusion reasons

4. **Status/assignee filtering**
   - Remaining candidates

5. **Candidate sorting**
   - Priority order

6. **Split decision**
   - Failure count vs threshold
   - Should split decision

7. **Final result**
   - BeadFound / NoWork / Split

## Log Analysis Commands

After capturing logs, analyze them with:

```bash
# All Pluck events
grep -i "pluck" output.log

# Filtering decisions
grep -i "filter" output.log

# Excluded beads
grep -i "exclude" output.log

# Candidate processing
grep -i "candidate" output.log

# Split decisions
grep -i "split" output.log

# Count event types
grep -c "Pluck strand evaluation starting" output.log
grep -c "result=BeadFound" output.log
grep -c "result=NoWork" output.log
grep -c "result=Split" output.log
```

## Key Findings Summary

✅ **No CLI debug flags** - All debug control via environment variables
✅ **Primary flag:** `RUST_LOG` - Controls module-level logging
✅ **Secondary flag:** `RUST_BACKTRACE` - Enables error stack traces
✅ **Command structure:** `RUST_LOG=<level> needle run -w <workspace> -c <count>`
✅ **6 preset configurations** available via helper script
✅ **Ready to construct execution commands**

## Status

**COMPLETE** - All debug flags identified and documented
- RUST_LOG syntax and levels understood
- Command structure documented
- Preset configurations available
- Ready to construct execution commands
