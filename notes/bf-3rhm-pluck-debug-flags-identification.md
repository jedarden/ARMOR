# Pluck Debug Flags and Configuration Structure

**Bead:** bf-3rhm  
**Date:** 2026-07-09  
**Status:** ✅ Complete

## Overview

This document identifies all supported Pluck debug flags, configuration file locations, and required settings for filtering decision logging.

---

## Supported Pluck Debug Flags

Pluck uses Rust's standard `RUST_LOG` environment variable for debug logging. The following levels and modules are supported:

### Log Levels for `needle::strand::pluck`

1. **`error`** - Critical failures only
2. **`warn`** - Warning conditions (not currently used in Pluck)
3. **`info`** - High-level operations and strand results
4. **`debug`** - Detailed filtering decisions and statistics
5. **`trace`** - Complete execution flow with all variables

### Recommended Debug Configurations

| Level | RUST_LOG Value | Use Case |
|-------|----------------|----------|
| **Minimal** | `needle::strand::pluck=info` | Quick health checks, basic verification |
| **Standard** | `needle::strand::pluck=debug` | Normal debugging, understanding filtering (recommended) |
| **Detailed** | `needle::strand::pluck=trace` | Deep troubleshooting, exact flow |
| **Comprehensive** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug` | Full system context |
| **Full** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug` | All NEEDLE modules |
| **Maximum** | `trace` | Everything (very verbose) |

---

## Configuration File Locations

### 1. Workspace Configuration (`.needle.yaml`)

**Location:** `/home/coding/ARMOR/.needle.yaml`

**Purpose:** Configures Pluck strand behavior for the ARMOR workspace

**Current Configuration:**
```yaml
strands:
  pluck:
    exclude_labels: []        # No label exclusions (empty array)
    split_after_failures: 0   # Auto-split disabled
```

**Configuration Parameters:**
- `exclude_labels`: Labels to exclude from bead selection (empty = none, defaults: `deferred`, `human`, `blocked`)
- `split_after_failures`: Auto-split threshold (0 = disabled, >0 = enable at N failures)

### 2. Debug Configuration Script

**Location:** `/home/coding/ARMOR/pluck-debug-config.sh`

**Purpose:** Provides preset configurations and automated log capture

**Usage:**
```bash
./pluck-debug-config.sh [workspace] [output_file] [mode] [count]
```

**Available Modes:**
- `minimal` - INFO level
- `standard` - DEBUG level (default, recommended)
- `detailed` - TRACE level
- `comprehensive` - Multi-module TRACE
- `full` - All NEEDLE modules
- `maximum` - Global TRACE

### 3. Manual Environment Configuration

**Method:** Set `RUST_LOG` environment variable before running NEEDLE

```bash
export RUST_LOG=needle::strand::pluck=debug
needle run -w /home/coding/ARMOR -c 1
```

---

## Required Flags for Filtering Decision Logging

For complete filtering decision visibility, use the **standard** debug level:

```bash
export RUST_LOG=needle::strand::pluck=debug
```

### What This Logs

With `RUST_LOG=needle::strand::pluck=debug`, you will see:

1. **Strand evaluation start:**
   ```
   DEBUG needle::strand::pluck: Pluck strand evaluation starting
     exclude_labels=["deferred", "human", "blocked"]
     split_threshold=3
   ```

2. **Bead store queries:**
   ```
   DEBUG needle::strand::pluck: Querying bead store for ready candidates
     filters=Filters { assignee: None, exclude_labels: [...] }
   ```

3. **Query results:**
   ```
   DEBUG needle::strand::pluck: Bead store returned N candidates
     count=5
   ```

4. **Label filtering:**
   ```
   DEBUG needle::strand::pluck: Label filtering excluded N beads
     excluded_count=2
     remaining=3
     excluded_labels=["deferred", "blocked"]
   ```

5. **Individual bead exclusions:**
   ```
   DEBUG needle::strand::pluck: Excluded bead due to labels
     bead_id="bf-1234"
     labels=["deferred"]
     excluded_reasons=["deferred"]
   ```

6. **Status/assignee filtering:**
   ```
   DEBUG needle::strand::pluck: Status/assignee filtering removed N beads
     filtered_count=1
     remaining=2
   ```

7. **Candidate sorting:**
   ```
   DEBUG needle::strand::pluck: Sorting N candidates by (priority ASC, created_at ASC, id ASC)
     total=5
     first_bead_id="bf-abcd"
     first_priority=1
     first_created_at=2026-01-01T00:00:00Z
   ```

8. **Split threshold checks:**
   ```
   DEBUG needle::strand::pluck: Checking split trigger for first candidate
     bead_id="bf-abcd"
     failure_count=2
     threshold=3
     split_triggered=false
   ```

9. **Final results:**
   ```
   DEBUG needle::strand::pluck: No candidates remaining after filtering, returning NoWork
   
   INFO needle::strand::pluck: Returning N candidates for processing
     count=3
     candidates=["bf-abcd", "bf-efgh", "bf-ijkl"]
   ```

---

## Additional Environment Variables

| Variable | Purpose | Value |
|----------|---------|-------|
| `RUST_LOG` | Controls debug output level | `needle::strand::pluck=debug` |
| `RUST_BACKTRACE` | Enables stack traces on errors | `1` (enable) or `0` (disable) |

---

## Pluck Strand Filtering Events

The Pluck strand logs the following specific filtering events at DEBUG level:

### 1. Label Filtering
- **Event:** Label filtering excluded beads
- **Trigger:** Beads contain excluded labels (deferred, human, blocked)
- **Fields:** `excluded_count`, `remaining`, `excluded_labels`

### 2. Individual Bead Exclusions
- **Event:** Excluded bead due to labels
- **Trigger:** Individual bead matches exclusion criteria
- **Fields:** `bead_id`, `labels`, `excluded_reasons`

### 3. Status/Assignee Filtering
- **Event:** Status/assignee filtering removed beads
- **Trigger:** Beads are InProgress or Open with stale assignee
- **Fields:** `filtered_count`, `remaining`

### 4. Split Trigger
- **Event:** Checking split trigger for first candidate
- **Trigger:** Evaluating first candidate for auto-split
- **Fields:** `bead_id`, `failure_count`, `threshold`, `split_triggered`

### 5. Candidate Selection
- **Event:** Returning N candidates for processing
- **Trigger:** Strand evaluation complete with candidates
- **Fields:** `count`, `candidates` (array of bead IDs)

---

## Verification Commands

After capturing logs, verify filtering decision visibility:

```bash
# Check Pluck strand events
grep "needle::strand::pluck" output.log

# Check label filtering
grep "Label filtering excluded" output.log

# Check individual exclusions
grep "Excluded bead due to labels" output.log

# Check status/assignee filtering
grep "Status/assignee filtering" output.log

# Check split decisions
grep "Checking split trigger" output.log

# Count total evaluations
grep -c "Pluck strand evaluation starting" output.log

# Count results by type
grep -c "Returning.*candidates for processing" output.log
grep -c "No candidates remaining after filtering" output.log
```

---

## Integration with NEEDLE

Pluck is configured as part of the standard NEEDLE strand set:

```
strands=["pluck", "mend", "explore", "weave", "unravel", "pulse", "reflect", "splice", "knot"]
```

### Pluck's Role in NEEDLE

1. **Primary bead selection** (~90% of all processing)
2. **Query bead store** for ready, unassigned beads
3. **Filter by excluded labels** (deferred, human, blocked)
4. **Filter by status/assignee** (remove InProgress and stale-assigned Open beads)
5. **Sort by priority** (priority ASC, created_at ASC, id ASC)
6. **Check split threshold** (failure-count label evaluation)
7. **Return result** (NoWork, BeadFound, or Split)

---

## Configuration Best Practices

1. **Use `standard` (debug) level for normal debugging** - Provides complete filtering visibility without excessive verbosity
2. **Set `split_after_failures` to 3-5 for production** - Enables auto-split while avoiding false positives
3. **Keep `exclude_labels` minimal** - Only exclude labels that truly prevent processing
4. **Use comprehensive mode for system-level debugging** - Adds supporting modules (bead_store, worker) for full context
5. **Capture logs to files for analysis** - Use `tee` or the debug script to preserve output

---

## Source Reference

**Pluck Implementation:** `/home/coding/NEEDLE/src/strand/pluck.rs`

The Pluck strand uses the `tracing` crate for structured logging with the following spans and events:

- **Span:** `strand.pluck` (with fields: `strand`, `exclude_labels`, `split_threshold`)
- **Events:** debug/info/error level logging at each filtering step
- **Fields:** Structured data for filters, candidates, counts, thresholds

---

## Status

✅ **Pluck debug flags identified and documented**  
✅ **Configuration file locations specified**  
✅ **Required flags for filtering decisions defined**  
✅ **All acceptance criteria met**

The Pluck debug logging system is fully configured and documented for use in filtering decision analysis and troubleshooting.
