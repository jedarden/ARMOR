# Pluck Output Redirection Configuration - bf-2wb4

**Date:** 2026-07-09  
**Bead:** bf-2wb4  
**Workspace:** /home/coding/ARMOR  
**Parent Bead:** bf-kjvf (Construct Pluck debug command)

## Overview

This document describes the output redirection strategy for Pluck debug execution, including log file locations, redirection syntax, and write permissions validation.

## Log File Location

**Primary Log Directory:** `/home/coding/ARMOR/logs/pluck-debug/`

**Status:** ✅ Confirmed and accessible  
**Permissions:** `drwxr-xr-x` (owner: coding, group: users)  
**Write Access:** ✅ Verified writable

### Log File Types

The log directory supports multiple types of log files:

| File Type | Pattern | Purpose |
|-----------|---------|---------|
| **Combined Log** | `pluck-combined-{bead_id}-{timestamp}.log` | Stdout + stderr merged |
| **Stdout Log** | `pluck-stdout-{bead_id}-{timestamp}.log` | Standard output only |
| **Stderr Log** | `pluck-stderr-{bead_id}-{timestamp}.log` | Standard error only |
| **Summary Log** | `pluck-summary-{bead_id}-{timestamp}.log` | Execution summary |
| **Capture Log** | `pluck-debug-{bead_id}-capture-{timestamp}.log` | Full debug capture |

## Output Redirection Syntax

### Method 1: Simple Combined Capture (Recommended for most cases)

```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" \
needle run -w /home/coding/ARMOR -c 1 \
2>&1 | tee logs/pluck-debug/pluck-combined-bf-2wb4-$(date +%Y%m%d-%H%M%S).log
```

**Key components:**
- `2>&1` - Redirect stderr to stdout
- `tee` - Write to file AND display to terminal
- `$(date +%Y%m%d-%H%M%S)` - Timestamp for unique filenames

### Method 2: Separated stdout/stderr Capture

```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" \
needle run -w /home/coding/ARMOR -c 1 \
> >(tee logs/pluck-debug/pluck-stdout-bf-2wb4-$(date +%Y%m%d-%H%M%S).log) \
2> >(tee logs/pluck-debug/pluck-stderr-bf-2wb4-$(date +%Y%m%d-%H%M%S).log >&2)
```

**Key components:**
- `> >(...)` - Process substitution for stdout
- `2> >(...)` - Process substitution for stderr
- `>&2` - Ensure stderr goes to terminal error stream

### Method 3: Using the Configuration Script (Recommended)

The `pluck-log-redirection.sh` script provides automated setup:

```bash
# Standard usage
./pluck-log-redirection.sh -b bf-2wb4 -p comprehensive

# Test-only mode (validates without full setup)
./pluck-log-redirection.sh --test-only

# Custom bead ID with specific preset
./pluck-log-redirection.sh -b bf-2wb4 -p detailed
```

**Script features:**
- Automatic log directory creation
- Multiple RUST_LOG presets (minimal, standard, detailed, comprehensive, full, maximum)
- Output redirection validation
- Sample test execution
- Summary report generation

## Write Permissions Verification

### Directory Permissions
```bash
ls -ld /home/coding/ARMOR/logs/pluck-debug/
# drwxr-xr-x 6 coding users 12288 Jul  9 05:04 .
```

**Result:** ✅ Owner has full permissions (rwx)

### Write Access Test
```bash
test -w /home/coding/ARMOR/logs/pluck-debug/ && echo "Writable" || echo "Not writable"
# Writable
```

**Result:** ✅ Directory is writable

### Sample File Creation Test
```bash
touch /home/coding/ARMOR/logs/pluck-debug/test-write-$(date +%s).tmp && rm -f /home/coding/ARMOR/logs/pluck-debug/test-write-*.tmp && echo "Write test successful"
# Write test successful
```

**Result:** ✅ File creation and deletion successful

## Redirection Strategy

### Strategy Overview

The output redirection strategy uses a **multi-tiered approach**:

1. **Real-time Terminal Output** - See progress as it happens
2. **Persistent Log Files** - Complete capture for later analysis
3. **Log Rotation** - Prevent disk space issues
4. **Summarization** - Quick-reference execution summaries

### Log Rotation Configuration

**Location:** `/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh`

**Settings:**
- **Max Size:** 10MB per file before rotation
- **Max Age:** 7 days before cleanup
- **Max Files:** 50 log files total

**Usage:**
```bash
# Run rotation with defaults
./logs/pluck-debug/log-rotation-config.sh

# Dry run to see what would be done
./logs/pluck-debug/log-rotation-config.sh --dry-run

# Custom settings
MAX_SIZE_MB=5 MAX_AGE_DAYS=3 ./logs/pluck-debug/log-rotation-config.sh
```

### RUST_LOG Presets

The redirection strategy integrates with RUST_LOG configuration:

| Preset | RUST_LOG Value | Use Case | Log Volume |
|--------|----------------|----------|-------------|
| **minimal** | `needle::strand::pluck=info` | Quick health checks | Low |
| **standard** | `needle::strand::pluck=debug` | Normal debugging | Medium |
| **detailed** | `needle::strand::pluck=trace` | Deep troubleshooting | High |
| **comprehensive** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug` | Full system context | Very High |
| **full** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug` | Complete system debugging | Maximum |
| **maximum** | `trace` | Everything (very verbose) | Extreme |

## Complete Execution Example

### Step 1: Setup Log Environment
```bash
cd /home/coding/ARMOR
./pluck-log-redirection.sh -b bf-2wb4 -p comprehensive
```

### Step 2: Run Pluck with Output Redirection
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" \
needle run -w /home/coding/ARMOR -c 1 \
2>&1 | tee logs/pluck-debug/pluck-combined-bf-2wb4-$(date +%Y%m%d-%H%M%S).log
```

### Step 3: Analyze Results
```bash
# View captured log
tail -100 logs/pluck-debug/pluck-combined-bf-2wb4-*.log

# Search for specific events
grep -i "pluck" logs/pluck-debug/pluck-combined-bf-2wb4-*.log
grep -i "filter" logs/pluck-debug/pluck-combined-bf-2wb4-*.log
grep -i "candidate" logs/pluck-debug/pluck-combined-bf-2wb4-*.log

# Count events
grep -c "Pluck strand evaluation starting" logs/pluck-debug/pluck-combined-bf-2wb4-*.log
grep -c "result=BeadFound" logs/pluck-debug/pluck-combined-bf-2wb4-*.log
```

### Step 4: Run Log Rotation (Optional)
```bash
./logs/pluck-debug/log-rotation-config.sh
```

## Integration with Parent Bead (bf-kjvf)

This output redirection configuration integrates directly with the Pluck debug command constructed in bead bf-kjvf:

**From bf-kjvf:**
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" needle run -w /home/coding/ARMOR -c 1
```

**With bf-2wb4 output redirection:**
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" \
needle run -w /home/coding/ARMOR -c 1 \
2>&1 | tee logs/pluck-debug/pluck-combined-bf-2wb4-$(date +%Y%m%d-%H%M%S).log
```

## Acceptance Criteria Status

- ✅ **Log file path confirmed and accessible** - `/home/coding/ARMOR/logs/pluck-debug/` exists and is writable
- ✅ **Output redirection syntax constructed** - Multiple methods documented (tee, process substitution, script)
- ✅ **Write permissions verified** - Directory permissions tested and confirmed
- ✅ **Redirection strategy documented** - Comprehensive documentation with examples

## Files and Components

| Component | Location | Purpose |
|-----------|----------|---------|
| **Log Directory** | `/home/coding/ARMOR/logs/pluck-debug/` | Primary log storage |
| **Configuration Script** | `/home/coding/ARMOR/pluck-log-redirection.sh` | Automated setup and validation |
| **Rotation Script** | `/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh` | Log rotation management |
| **This Documentation** | `/home/coding/ARMOR/notes/bf-2wb4-pluck-output-redirection.md` | Complete strategy reference |

## Status

✅ **Output redirection configuration complete**  
✅ **All acceptance criteria met**  
✅ **Integration with parent bead (bf-kjvf) verified**  
✅ **Ready for execution chain continuation**

The Pluck output redirection is fully configured and ready for use with the debug command from bead bf-kjvf.
