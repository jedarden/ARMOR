# Pluck Debug Configuration Files Location Summary

**Bead:** bf-4g8se  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**Task:** Locate debug configuration files for Pluck execution

## Overview

This document summarizes the location and existence of all debug configuration files for Pluck execution in the ARMOR workspace. All expected files have been verified and exist.

## Primary Configuration Files

### 1. Main Pluck Configuration
- **Location:** `/home/coding/ARMOR/pluck-config.yaml`
- **Status:** ✅ EXISTS
- **Purpose:** Controls Pluck strand debug logging levels, filtering decisions, bead store queries, and output configuration
- **Key Settings:**
  - Debug level configuration (info/debug/trace/off)
  - Filtering decision logging
  - Bead store query logging
  - Split threshold evaluation logging
  - Log file output configuration

### 2. NEEDLE Configuration
- **Location:** `/home/coding/ARMOR/.needle.yaml`
- **Status:** ✅ EXISTS
- **Purpose:** Configures NEEDLE strand behavior, including Pluck filtering settings
- **Key Settings:**
  - `strands.pluck.exclude_labels` - Labels to exclude when selecting beads
  - `strands.pluck.split_after_failures` - Auto-split beads after N consecutive failures

### 3. Environment Configuration
- **Location:** `/home/coding/ARMOR/.env.pluck-debug`
- **Status:** ✅ EXISTS
- **Purpose:** RUST_LOG environment variable presets for different debug levels
- **Configuration:** Complete worker context preset (recommended):
  ```bash
  export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
  ```

## Documentation Files

### 1. Main Debug Documentation
- **Location:** `/home/coding/ARMOR/docs/pluck-debug-configuration.md`
- **Status:** ✅ EXISTS
- **Purpose:** Comprehensive guide to Pluck debug logging, including:
  - RUST_LOG environment variable usage
  - Available log levels and module paths
  - Recommended debug configurations
  - Expected debug output messages
  - Usage examples and troubleshooting

### 2. Command Reference Documentation
- **Location:** `/home/coding/ARMOR/docs/pluck-debug-command-reference.md`
- **Status:** ✅ EXISTS
- **Purpose:** Reference documentation for Pluck debug commands

### 3. Research Documentation
- **Location:** `/home/coding/ARMOR/notes/bf-667vk-pluck-debug-flags-research.md`
- **Status:** ✅ EXISTS
- **Purpose:** Research findings on Pluck debug flags and command construction

## Script Files

### 1. Debug Capture Script
- **Location:** `/home/coding/ARMOR/capture-pluck-debug.sh`
- **Status:** ✅ EXISTS (executable)
- **Purpose:** Captures complete Pluck filtering debug output with comprehensive logging
- **Usage:** `./capture-pluck-debug.sh <workspace> <output_file> <count>`

### 2. Debug Configuration Manager
- **Location:** `/home/coding/ARMOR/pluck-debug-config.sh`
- **Status:** ✅ EXISTS (executable)
- **Purpose:** Provides preset configurations for different debug levels
- **Modes:** minimal, standard, detailed, comprehensive, full, maximum
- **Usage:** `./pluck-debug-config.sh <workspace> <output_file> <mode> <count>`

### 3. Debug Log Analyzer
- **Location:** `/home/coding/ARMOR/analyze-pluck-debug.sh`
- **Status:** ✅ EXISTS (executable)
- **Purpose:** Analyzes captured debug logs and provides structured output
- **Usage:** `./analyze-pluck-debug.sh <log_file>`

## Log Directory Structure

### Debug Logs Directory
- **Location:** `/home/coding/ARMOR/logs/pluck-debug/`
- **Status:** ✅ EXISTS
- **Purpose:** Storage directory for captured Pluck debug logs
- **Contains:** Multiple timestamped debug capture log files from previous executions

## Debug Configuration Levels

### Available RUST_LOG Presets

1. **Minimal**
   - `RUST_LOG=needle::strand::pluck=info`
   - High-level strand operations only

2. **Standard** (RECOMMENDED)
   - `RUST_LOG=needle::strand::pluck=debug`
   - Filtering decisions and statistics

3. **Detailed**
   - `RUST_LOG=needle::strand::pluck=trace`
   - Complete execution details

4. **Comprehensive**
   - `RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug`
   - TRACE + supporting modules

5. **Full**
   - All NEEDLE modules at DEBUG/TRACE level

6. **Maximum**
   - `RUST_LOG=trace`
   - Everything at TRACE level (very verbose)

## Related Module Paths

**Primary Pluck Module:**
- `needle::strand::pluck` - Core Pluck strand evaluation

**Related Modules:**
- `needle::strand` - All strand implementations
- `needle::worker` - Worker coordination
- `needle::bead_store` - Bead storage operations
- `needle::dispatch` - Task dispatching
- `needle::claim` - Claim process

## Configuration File Relationships

```
pluck-config.yaml          → Main debug configuration
↓
.needle.yaml               → NEEDLE framework configuration
↓
.env.pluck-debug           → Environment variable presets
↓
capture-pluck-debug.sh     → Execution with debug enabled
↓
analyze-pluck-debug.sh     → Post-execution analysis
```

## Summary

**All expected debug configuration files are present and accounted for:**

✅ **Primary Configuration:** 3 files (pluck-config.yaml, .needle.yaml, .env.pluck-debug)
✅ **Documentation:** 3 files (main docs, command reference, research notes)
✅ **Scripts:** 3 executable files (capture, config manager, analyzer)
✅ **Log Directory:** 1 directory (logs/pluck-debug/)

**No missing configuration files detected.**

## Usage Quick Reference

```bash
# Enable debug logging
source .env.pluck-debug

# Run with debug capture
./capture-pluck-debug.sh /home/coding/ARMOR output.log 1

# Analyze captured logs
./analyze-pluck-debug.sh output.log

# Use specific debug level
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive 1
```

## Notes

- All configuration files are properly formatted and valid
- All scripts have executable permissions
- Log directory structure is properly organized
- Documentation is comprehensive and up-to-date
- No missing or corrupted configuration files detected