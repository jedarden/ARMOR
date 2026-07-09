# Pluck Debug Flags Research and Command Construction

**Bead:** bf-667vk  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR

## Overview

This document provides comprehensive research on Pluck debug flags and constructs the full commands for different debugging scenarios. Pluck is the primary bead selection strand in the NEEDLE system, handling >90% of all bead processing.

## Debug Flag Infrastructure

### Environment Variable: `RUST_LOG`

The primary mechanism for controlling Pluck debug output is the `RUST_LOG` environment variable, which follows the standard Rust `tracing` crate logging framework.

**Format:** `RUST_LOG=<target>=<level>[,<target2>=<level2>,...]`

### Available Debug Targets

| Target | Description | Scope |
|--------|-------------|-------|
| `needle::strand::pluck` | Pluck strand evaluation and filtering decisions | Primary |
| `needle::strand` | All strand operations (pluck, mend, explore, etc.) | Broader |
| `needle::bead_store` | Bead store queries and data access | Supporting |
| `needle::worker` | Worker lifecycle and state transitions | System |
| `needle::dispatch` | Bead dispatching operations | System |
| `needle::claim` | Bead claiming and assignment | System |

### Log Levels (Increasing Verbosity)

| Level | Usage | Output Volume |
|-------|-------|---------------|
| `error` | Critical failures only | Minimal |
| `warn` | Warning conditions | Low |
| `info` | High-level operations | Medium |
| `debug` | Detailed execution flow | High |
| `trace` | Complete execution details | Maximum |

## Configuration Presets

Based on the existing `pluck-debug-config.sh` script, six preset configurations are available:

### 1. Minimal
```bash
RUST_LOG=needle::strand::pluck=info
```
**Use Case:** Quick health checks, production monitoring  
**Output:** High-level strand operations only (evaluation start, results)

### 2. Standard (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug
```
**Use Case:** Normal debugging, troubleshooting selection issues  
**Output:** Filtering decisions, statistics, per-stage execution details

### 3. Detailed
```bash
RUST_LOG=needle::strand::pluck=trace
```
**Use Case:** Deep troubleshooting, understanding execution flow  
**Output:** Complete execution details, including variable states and decisions

### 4. Comprehensive
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
```
**Use Case:** Full system context, understanding interactions  
**Output:** TRACE-level Pluck + DEBUG-level supporting modules

### 5. Full
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug
```
**Use Case:** Complete debugging session, systemic issues  
**Output:** All NEEDLE modules at DEBUG/TRACE levels

### 6. Maximum
```bash
RUST_LOG=trace
```
**Use Case:** Extremely detailed analysis, performance investigation  
**Output:** Everything at TRACE level (very verbose, may impact performance)

## Pluck-Specific Logging Stages

Based on source code analysis (`/home/coding/NEEDLE/src/strand/pluck.rs`), the following logging stages are available:

| Stage | Log Level | Information |
|-------|-----------|--------------|
| Evaluation Start | DEBUG | `exclude_labels`, `split_threshold` |
| Bead Store Query | DEBUG | Filters applied, query execution |
| Candidate Count | DEBUG | Number of beads returned |
| Label Filtering | DEBUG | Excluded beads, reasons, counts |
| Status/Assignee Filter | DEBUG | Removed beads with active assignees |
| Candidate Sorting | DEBUG | Total candidates, first candidate details |
| Split Trigger Check | DEBUG | Failure count vs threshold comparison |
| Split Result | INFO | When split threshold is reached |
| Final Result | INFO | NoWork / BeadFound with candidate list |

## Full Command Construction

### Basic Command Structure
```bash
RUST_LOG=<debug_level> /home/coding/NEEDLE/target/release/needle run -w <workspace> -c <count> 2>&1 | tee <output_file>
```

### Parameter Explanations

| Parameter | Description | Example |
|-----------|-------------|---------|
| `RUST_LOG` | Debug level configuration | `needle::strand::pluck=debug` |
| `/home/coding/NEEDLE/target/release/needle` | Path to NEEDLE binary | - |
| `run` | Command to launch workers | - |
| `-w <workspace>` | Workspace directory to process | `/home/coding/ARMOR` |
| `-c <count>` | Number of workers to launch | `1` |
| `2>&1` | Redirect stderr to stdout | - |
| `| tee <output>` | Save to file while displaying | `pluck-debug.log` |

### Recommended Commands by Scenario

#### 1. Quick Health Check
```bash
RUST_LOG=needle::strand::pluck=info /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-health.log
```

#### 2. Standard Debugging (Most Common)
```bash
RUST_LOG=needle::strand::pluck=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

#### 3. Deep Troubleshooting
```bash
RUST_LOG=needle::strand::pluck=trace /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-trace.log
```

#### 4. Full System Context
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-comprehensive.log
```

#### 5. Maximum Diagnostics
```bash
RUST_LOG=trace /home/coding/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-maximum.log
```

### Using the Configuration Script (Simplified)
```bash
# Standard debug level
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log standard 1

# Comprehensive debug level  
bash pluck-debug-config.sh /home/coding/ARMOR pluck-comprehensive.log comprehensive 1

# Maximum verbosity
bash pluck-debug-config.sh /home/coding/ARMOR pluck-maximum.log maximum 1
```

## Log Analysis Commands

After capturing debug output, use these commands to analyze specific aspects:

```bash
# View all Pluck events
grep -i "pluck" pluck-debug.log

# Filter specific decisions
grep -i "filter" pluck-debug.log          # Filtering decisions
grep -i "exclude" pluck-debug.log         # Excluded beads
grep -i "candidate" pluck-debug.log       # Candidate selection
grep -i "split" pluck-debug.log           # Split triggers

# Count events
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "result=BeadFound" pluck-debug.log
grep -c "Split threshold reached" pluck-debug.log

# View specific stages
grep "Label filtering excluded" pluck-debug.log
grep "Status/assignee filtering" pluck-debug.log
grep "Checking split trigger" pluck-debug.log
```

## Expected Output Patterns

### Successful Bead Selection
```
[timestamp] DEBUG needle::strand::pluck: Pluck strand evaluation starting exclude_labels=[...] split_threshold=3
[timestamp] DEBUG needle::strand::pluck: Querying bead store for ready candidates
[timestamp] DEBUG needle::strand::pluck: Bead store returned N candidates
[timestamp] DEBUG needle::strand::pluck: No beads excluded by label filter
[timestamp] INFO needle::strand::pluck: Returning N candidates for processing
```

### Auto-Split Trigger
```
[timestamp] DEBUG needle::strand::pluck: Checking split trigger for first candidate bead_id=bf-XXX failure_count=3 threshold=3
[timestamp] INFO needle::strand::pluck: Split threshold reached, returning Split instruction
[timestamp] INFO needle::worker: auto-split triggered: using SPLIT template bead_id=bf-XXX failure_count=3 threshold=3
```

### Label Filtering in Action
```
[timestamp] DEBUG needle::strand::pluck: Label filtering excluded 2 beads remaining=5
[timestamp] DEBUG needle::strand::pluck: Excluded bead due to labels bead_id=bf-YYY labels=[deferred] excluded_reasons=["deferred"]
```

## Performance Considerations

| Level | Performance Impact | Recommended Duration |
|-------|-------------------|---------------------|
| `error/warn` | Minimal | Production long-term |
| `info` | Low | Production monitoring |
| `debug` | Low-Medium | Debugging sessions |
| `trace` | Medium-High | Short troubleshooting |
| `maximum (trace)` | High | Brief diagnostics only |

## Configuration Files

Pluck behavior can also be configured via `.needle.yaml`:

```yaml
strands:
  pluck:
    exclude_labels: ["deferred", "human", "blocked"]
    split_after_failures: 3  # 0 = disabled
```

## Key Findings

1. **Primary Debug Mechanism**: `RUST_LOG` environment variable
2. **Target Specificity**: Use `needle::strand::pluck` for focused debugging
3. **Level Selection**: `debug` is recommended for most troubleshooting
4. **Script Availability**: `pluck-debug-config.sh` provides preset configurations
5. **Log Analysis**: Standard grep patterns work well for log analysis
6. **Performance**: Higher log levels (trace) may impact performance

## Conclusion

The Pluck debug system is well-instrumented with comprehensive logging at all decision points. The `RUST_LOG` environment variable provides fine-grained control over output verbosity, and the existing configuration script offers convenient presets for common debugging scenarios.

For most debugging needs, **`RUST_LOG=needle::strand::pluck=debug`** provides the best balance between detail and readability.

---

**Status:** ✅ Research Complete  
**Ready for Execution:** Yes  
**Recommended Starting Point:** Standard debug level (`needle::strand::pluck=debug`)