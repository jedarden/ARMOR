# Pluck Debug Logging Requirements Summary

**Bead:** bf-2wsa  
**Date:** 2026-07-09  
**Status:** ✅ Requirements Identified and Documented

## Overview

This document summarizes the Pluck debug logging requirements for filtering decision logging in the ARMOR workspace.

## 1. Pluck Configuration File Location and Format

### Location
- **Path:** `/home/coding/ARMOR/.needle.yaml`
- **Format:** YAML configuration file

### Configuration Structure
```yaml
strands:
  pluck:
    exclude_labels: []  # Labels to exclude from bead selection
    split_after_failures: 0  # Auto-split threshold (0 = disabled)
```

### Default Settings
- **Default exclude_labels:** `["deferred", "human", "blocked"]` (when empty in config)
- **Default split threshold:** `3` failures (when 0 in config)

## 2. Available Debug Flags for Filtering Decisions

### Primary Debug Flags

| Module Path | Purpose | Log Level |
|-------------|---------|-----------|
| `needle::strand::pluck` | Core Pluck strand evaluation, filtering decisions, candidate selection | `debug` or `trace` |
| `needle::strand` | All strand coordination | `debug` |
| `needle::bead_store` | Bead store queries and operations | `debug` |
| `needle::worker` | Worker state machine and lifecycle | `debug` |
| `needle::dispatch` | Agent dispatch and execution | `debug` |

### Environment Variable Format
```bash
# Single module
export RUST_LOG=needle::strand::pluck=debug

# Multiple modules
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug

# Global trace
export RUST_LOG=trace
```

## 3. Required Flags for Filtering Decision Output

### Recommended Configuration
For filtering decision output, use:
```bash
export RUST_LOG=needle::strand::pluck=debug
```

### Comprehensive Configuration
For full system context including filtering decisions:
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

## 4. Available Configuration Presets

The workspace includes 6 pre-configured debug levels via `pluck-debug-config.sh`:

| Mode | RUST_LOG Setting | Use Case |
|------|------------------|----------|
| **minimal** | `needle::strand::pluck=info` | Quick health checks |
| **standard** | `needle::strand::pluck=debug` | Filtering decisions (recommended) |
| **detailed** | `needle::strand::pluck=trace` | Deep troubleshooting |
| **comprehensive** | Multi-module TRACE | Full system context |
| **full** | All NEEDLE modules DEBUG/TRACE | Complete debugging |
| **maximum** | `trace` | Everything (very verbose) |

## 5. Expected Debug Output

### Filtering Decision Events Captured

1. **Strand Evaluation Start**
   - Exclude labels configuration
   - Split threshold setting
   ```
   DEBUG Pluck strand evaluation starting
     exclude_labels=["deferred", "human", "blocked"]
     split_threshold=3
   ```

2. **Bead Store Query**
   - Filter parameters
   - Candidate count
   ```
   DEBUG Querying bead store for ready candidates
     filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }
   DEBUG Bead store returned N candidates count=5
   ```

3. **Label Filtering**
   - Excluded beads
   - Exclusion reasons
   ```
   DEBUG Label filtering excluded N beads
     excluded_count=2
     remaining=3
     excluded_labels=["deferred", "human"]
   DEBUG Excluded bead due to labels
     bead_id="bf-1234"
     labels=["deferred"]
     excluded_reasons=["deferred"]
   ```

4. **Status/Assignee Filtering**
   - Remaining candidates
   ```
   DEBUG Status/assignee filtering removed N beads
     filtered_count=1
     remaining=2
   ```

5. **Candidate Sorting**
   - Priority order
   - First candidate details
   ```
   DEBUG Sorting N candidates by (priority ASC, created_at ASC, id ASC)
     total=2
     first_bead_id="bf-abcd"
     first_priority=1
     first_created_at="2026-07-09T00:00:00Z"
   ```

6. **Split Trigger Evaluation**
   - Failure count analysis
   - Split decision
   ```
   DEBUG Checking split trigger for first candidate
     bead_id="bf-abcd"
     failure_count=2
     threshold=3
     split_triggered=false
   INFO Split threshold reached, returning Split instruction
     bead_id="bf-abcd"
     failure_count=3
     threshold=3
   ```

7. **Final Result**
   - NoWork / BeadFound / Split
   ```
   DEBUG No candidates remaining after filtering, returning NoWork
   INFO Returning N candidates for processing
     count=2
     candidates=["bf-abcd", "bf-1234"]
   ```

## 6. Usage Instructions

### Quick Start
```bash
# Using the configuration script
cd /home/coding/ARMOR
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log standard

# Manual configuration
export RUST_LOG=needle::strand::pluck=debug
needle run -w /home/coding/ARMOR -c 1
```

### Log Analysis
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
```

## 7. Configuration Files

### Primary Files
1. **Configuration:** `/home/coding/ARMOR/.needle.yaml`
2. **Debug Script:** `/home/coding/ARMOR/pluck-debug-config.sh`
3. **Environment:** `/home/coding/ARMOR/.env.pluck-debug`
4. **Documentation:** `/home/coding/ARMOR/pluck-debug-configuration.md`

### Source Code
- **Pluck Implementation:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Strand Trait:** `/home/coding/NEEDLE/src/strand/mod.rs`

## 8. Acceptance Criteria Status

✅ **Pluck config location documented** - Location: `/home/coding/ARMOR/.needle.yaml`  
✅ **List of required debug flags identified** - 6 preset levels available  
✅ **Configuration format understood** - YAML format with documented structure

## Summary

The Pluck debug logging requirements are fully identified and documented. The system uses Rust's standard `RUST_LOG` environment variable to control debug output, with 6 pre-configured preset levels ranging from minimal logging to full trace-level output. The recommended configuration for filtering decision output is `needle::strand::pluck=debug`, which provides comprehensive visibility into the bead selection and filtering process.

All configuration files, scripts, and documentation are in place and operational in the ARMOR workspace.
