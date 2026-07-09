# Pluck Debug Configuration Guide - Bead bf-3bqg

**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**Task:** Prepare debug configuration for Pluck execution

## Overview

This document provides complete configuration for executing Pluck (NEEDLE's strand filtering component) with comprehensive debug logging enabled. The configuration captures detailed trace output for debugging Pluck's filtering behavior and decision-making process.

## 1. Pluck Executable Location

### ✅ Verified Location
- **Binary:** `/home/coding/.local/bin/needle`
- **Version:** `0.2.11`
- **Type:** Executable binary
- **Accessibility:** Readable and executable
- **Description:** NEEDLE is the delivery system that processes beads through various strands including "pluck"

**Verification Command:**
```bash
which needle
# Output: /home/coding/.local/bin/needle

needle --version
# Output: needle 0.2.11
```

## 2. Debug Flags Configuration

### Required Environment Variables

**Primary Debug Configuration:**
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

### Flag Descriptions

| Flag | Purpose | Level |
|------|---------|-------|
| `needle::strand::pluck=trace` | Maximum detail for Pluck strand filtering operations | TRACE |
| `needle::strand=debug` | General strand operations and interactions | DEBUG |
| `needle::bead_store=debug` | Bead discovery and claiming operations | DEBUG |
| `needle::worker=debug` | Worker lifecycle and state transitions | DEBUG |
| `needle::dispatch=debug` | Agent dispatch and rate limiting | DEBUG |

### Alternative Debug Levels

**Maximum Detail (All Trace):**
```bash
export RUST_LOG="trace"
```

**Strand Interactions Focus:**
```bash
export RUST_LOG="debug"
```

**Pluck Only (Minimal):**
```bash
export RUST_LOG="needle::strand::pluck=debug"
```

## 3. Log Directory Configuration

### ✅ Verified Log Directory
- **Location:** `/home/coding/ARMOR/logs/pluck-debug/`
- **Status:** Created and writable
- **Permissions:** `drwxr-xr-x 2 coding users 4096 Jul 9 02:06`

**Verification:**
```bash
mkdir -p /home/coding/ARMOR/logs/pluck-debug
ls -ld /home/coding/ARMOR/logs/pluck-debug
test -w /home/coding/ARMOR/logs/pluck-debug && echo "Writable" || echo "Not writable"
```

## 4. Debug Command Configuration

### Option A: Using the Capture Script (Recommended)

**Script Location:** `/home/coding/ARMOR/capture-pluck-debug.sh`

**Usage:**
```bash
bash capture-pluck-debug.sh /home/coding/ARMOR <output_file> <count>
```

**Example:**
```bash
# Single run with timestamped output
bash capture-pluck-debug.sh /home/coding/ARMOR pluck-debug-capture-$(date +%Y%m%d-%H%M%S).log 1

# Multiple runs
bash capture-pluck-debug.sh /home/coding/ARMOR pluck-debug-batch.log 5
```

### Option B: Direct Execution with Timeout

**Single Run (60 second timeout):**
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
timeout 60s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee /home/coding/ARMOR/logs/pluck-debug/pluck-debug-$(date +%Y%m%d-%H%M%S).log
```

**Short Test Run:**
```bash
export RUST_LOG="needle::strand::pluck=trace"
timeout 30s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee /home/coding/ARMOR/logs/pluck-debug/pluck-test.log
```

### Option C: Background Execution

**Long-running capture:**
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
nohup needle run -w /home/coding/ARMOR -c 1 > /home/coding/ARMOR/logs/pluck-debug/pluck-background-$(date +%Y%m%d-%H%M%S).log 2>&1 &
```

## 5. Command Analysis

Once debug logs are captured, analyze them with these commands:

**Filter for Pluck operations:**
```bash
grep -i 'pluck' /home/coding/ARMOR/logs/pluck-debug/*.log
```

**Filter for filtering decisions:**
```bash
grep -i 'filter' /home/coding/ARMOR/logs/pluck-debug/*.log
```

**Filter for exclusions:**
```bash
grep -i 'exclude' /home/coding/ARMOR/logs/pluck-debug/*.log
```

**Filter for candidate evaluations:**
```bash
grep -i 'candidate' /home/coding/ARMOR/logs/pluck-debug/*.log
```

**Check for errors:**
```bash
grep -i 'error\|warn' /home/coding/ARMOR/logs/pluck-debug/*.log
```

## 6. Expected Debug Output

When Pluck debug logging is enabled, you should see:

```
✅ Worker boot sequence with strand initialization
✅ Telemetry system startup and trace sanitizer loading
✅ Pluck strand loading and registration
✅ Bead store queries and candidate discovery
✅ Filtering decisions for each bead
✅ State transitions: BOOTING → SELECTING → BUILDING → DISPATCHING → EXECUTING
✅ Agent dispatch operations
✅ Rate limiting and throttling information
```

## 7. Quick Reference

**Minimum working debug command:**
```bash
export RUST_LOG="needle::strand::pluck=trace"
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

**Comprehensive debug command:**
```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee /home/coding/ARMOR/logs/pluck-debug/pluck-$(date +%Y%m%d-%H%M%S).log
```

## 8. Acceptance Criteria Status

- ✅ **Pluck executable location confirmed:** `/home/coding/.local/bin/needle` v0.2.11
- ✅ **Debug flags identified and documented:** Complete RUST_LOG configuration with alternatives
- ✅ **Log directory exists and writable:** `/home/coding/ARMOR/logs/pluck-debug/` created and verified
- ✅ **Debug command configuration ready:** Multiple execution options documented

## 9. Next Steps

With this debug configuration prepared, you can now:

1. Execute Pluck with debug logging using any of the documented methods
2. Capture comprehensive trace output for analysis
3. Debug Pluck filtering behavior and decision-making
4. Identify and resolve any issues with strand operations
5. Analyze bead discovery and claiming processes

## 10. Related Files

- **Capture Script:** `/home/coding/ARMOR/capture-pluck-debug.sh`
- **Log Directory:** `/home/coding/ARMOR/logs/pluck-debug/`
- **Previous Debug Runs:** `/home/coding/ARMOR/bf-6a7c-pluck-debug-summary.md`
- **NEEDLE Binary:** `/home/coding/.local/bin/needle`

**Status:** ✅ Complete - All debug configuration prepared and documented
