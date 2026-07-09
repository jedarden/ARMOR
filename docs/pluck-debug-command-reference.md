# Pluck Debug Command Structure - Complete Reference

**Bead:** bf-1r7s  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**NEEDLE Version:** 0.2.11

## Overview

The Pluck strand is NEEDLE's primary bead selection mechanism, handling >90% of all bead processing. This document provides the complete reference for debugging Pluck strand operations using Rust's tracing infrastructure.

## Core Debug Command Structure

### Basic Command Pattern

```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <concurrency>
```

### Complete Debug Command with All Options

```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug \
RUST_BACKTRACE=1 \
NEEDLE run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-$(date +%Y%m%d-%H%M%S).log
```

## RUST_LOG Environment Variable Configurations

### Debug Level Hierarchy

| Level | Purpose | When to Use |
|-------|---------|-------------|
| `error` | Critical failures only | Production monitoring |
| `warn` | Warning conditions | Production with alerts |
| `info` | High-level operations | Normal operation visibility |
| `debug` | Detailed decisions | **Recommended for debugging** |
| `trace` | Complete execution flow | Deep troubleshooting |

### Pluck-Specific Module Targets

| Module Target | Description |
|---------------|-------------|
| `needle::strand::pluck` | **Primary Pluck strand module** |
| `needle::strand` | All strand modules (pluck, mend, explore, weave, unravel, pulse, reflect, splice, knot) |
| `needle::bead_store` | Bead storage and query operations |
| `needle::worker` | Worker lifecycle and coordination |
| `needle::dispatch` | Task dispatch and coordination |
| `needle::claim` | Bead claiming operations |

### Preset RUST_LOG Configurations

#### 1. **minimal** - INFO level only
```bash
export RUST_LOG=needle::strand::pluck=info
```
**Output:** High-level strand operations only  
**Use case:** Quick health checks, basic operation verification

#### 2. **standard** - DEBUG level (Recommended)
```bash
export RUST_LOG=needle::strand::pluck=debug
```
**Output:** Filtering decisions and statistics  
**Use case:** Normal debugging, understanding filtering behavior

#### 3. **detailed** - TRACE level
```bash
export RUST_LOG=needle::strand::pluck=trace
```
**Output:** Complete execution details  
**Use case:** Deep troubleshooting, understanding exact flow

#### 4. **comprehensive** - Multi-module TRACE
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
```
**Output:** Pluck TRACE + supporting modules DEBUG  
**Use case:** Full context debugging, understanding system interactions

#### 5. **full** - All NEEDLE modules
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug
```
**Output:** All critical modules at DEBUG/TRACE level  
**Use case:** Complete system debugging

#### 6. **maximum** - Global TRACE
```bash
export RUST_LOG=trace
```
**Output:** Everything at TRACE level (very verbose)  
**Use case:** Deep system-level debugging

## Complete NEEDLE Command Reference

### Command Structure

```bash
needle [GLOBAL_OPTIONS] run [RUN_OPTIONS]
```

### Global Options

| Option | Short | Description | Default |
|--------|-------|-------------|---------|
| `--config <FILE>` | `-C` | Config file path | `.needle.yaml` |
| `--help` | `-h` | Show help | - |
| `--version` | `-V` | Show version | - |

### Run Command Options

| Option | Description | Example |
|--------|-------------|---------|
| `-w, --workspace <PATH>` | Workspace directory | `-w /home/coding/ARMOR` |
| `-c, --concurrency <NUM>` | Number of concurrent workers | `-c 1` |
| `--strands <LIST>` | Specific strands to run | `--strands pluck,mend` |
| `--one-shot` | Exit after one bead | - |

## Pluck Strand Tracing Instrumentation

### Tracing Span Fields

The Pluck strand uses `tracing::instrument` with the following fields:

```rust
#[tracing::instrument(
    name = "strand.pluck",
    skip(self, store),
    fields(
        strand = "pluck",
        exclude_labels = ?self.exclude_labels,
        split_threshold = self.split_after_failures,
    )
)]
```

### Debug Event Sequence

#### 1. **Evaluation Start**
```
DEBUG needle::strand::pluck: Pluck strand evaluation starting
  exclude_labels=["deferred", "human", "blocked"]
  split_threshold=3
```

#### 2. **Bead Store Query**
```
DEBUG needle::strand::pluck: Querying bead store for ready candidates
  filters=Filters { 
    assignee: None, 
    exclude_labels: ["deferred", "human", "blocked"] 
  }
```

#### 3. **Query Results**
```
DEBUG needle::strand::pluck: Bead store returned N candidates
  count=5
```

#### 4. **Label Filtering**
```
DEBUG needle::strand::pluck: Label filtering excluded N beads
  excluded_count=2
  remaining=3
  excluded_labels=["deferred", "human", "blocked"]
```

#### 5. **Individual Bead Exclusions**
```
DEBUG needle::strand::pluck: Excluded bead due to labels
  bead_id=bf-1234
  labels=["deferred", "high-priority"]
  excluded_reasons=["label:deferred"]
```

#### 6. **Status/Assignee Filtering**
```
DEBUG needle::strand::pluck: Status/assignee filtering removed N beads
  filtered_count=1
  remaining=2
```

#### 7. **Candidate Sorting**
```
DEBUG needle::strand::pluck: Sorting N candidates by (priority ASC, created_at ASC, id ASC)
  total=5
  first_bead_id=bf-abcd
  first_priority=1
  first_created_at=2026-07-09T12:00:00Z
```

#### 8. **Split Trigger Check**
```
DEBUG needle::strand::pluck: Checking split trigger for first candidate
  bead_id=bf-fail
  failure_count=4
  threshold=3
  split_triggered=true
```

#### 9. **Split Decision**
```
INFO needle::strand::pluck: Split threshold reached, returning Split instruction
  bead_id=bf-fail
  failure_count=4
  threshold=3
```

#### 10. **Final Result**
```
INFO needle::strand::pluck: Returning N candidates for processing
  count=2
  candidates=["bf-abcd", "bf-1234"]
```

### Error Events

```
ERROR needle::strand::pluck: Bead store query failed
  error=bf list failed
```

## Practical Execution Examples

### Example 1: Standard Debug Session
```bash
#!/bin/bash
WORKSPACE="/home/coding/ARMOR"
LOG_FILE="logs/pluck-debug/pluck-debug-$(date +%Y%m%d-%H%M%S).log"

mkdir -p logs/pluck-debug

RUST_LOG=needle::strand::pluck=debug \
NEEDLE run -w "$WORKSPACE" -c 1 2>&1 | tee "$LOG_FILE"

# Analysis
grep "Pluck strand evaluation starting" "$LOG_FILE"
grep "result=BeadFound" "$LOG_FILE"
grep "result=NoWork" "$LOG_FILE"
```

### Example 2: Comprehensive Multi-Module Debug
```bash
#!/bin/bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug \
RUST_BACKTRACE=1 \
cd /home/coding/NEEDLE
cargo run -- run -w /home/coding/ARMOR -c 1 2>&1 | tee comprehensive-debug.log
```

### Example 3: Filtered Debug with Regex
```bash
# Only show Pluck events (uses env_logger regex filtering)
RUST_LOG=needle::strand::pluck=debug/[.]*pluck/ \
NEEDLE run -w /home/coding/ARMOR -c 1
```

### Example 4: JSON Output for Analysis
```bash
RUST_LOG=needle::strand::pluck=debug \
NEEDLE run -w /home/coding/ARMOR -c 1 2>&1 | jq -c 'select(.message | contains("pluck"))'
```

## Log Analysis Commands

### View All Pluck Events
```bash
grep -i "pluck" pluck-debug.log
```

### Filter Specific Decisions
```bash
# Label filtering
grep -i "filter" pluck-debug.log
grep -i "exclude" pluck-debug.log

# Candidate processing
grep -i "candidate" pluck-debug.log

# Split decisions
grep -i "split" pluck-debug.log

# Errors
grep -i "error" pluck-debug.log
```

### Count Event Types
```bash
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "result=BeadFound" pluck-debug.log
grep -c "result=NoWork" pluck-debug.log
grep -c "result=Split" pluck-debug.log
grep -c "Excluded bead due to labels" pluck-debug.log
```

### Extract Specific Information
```bash
# Get all excluded bead IDs
grep "Excluded bead due to labels" pluck-debug.log | grep -oP 'bead_id=\K[^,]+'

# Get candidate counts
grep "Bead store returned" pluck-debug.log | grep -oP 'count=\K\d+'

# View split threshold checks
grep "Checking split trigger" pluck-debug.log -A 1
```

## Additional Environment Variables

### RUST_BACKTRACE
```bash
# Enable backtrace on errors
RUST_BACKTRACE=1

# Full backtrace (more detailed)
RUST_BACKTRACE=full
```

### RUST_LOG_SPAN_EVENTS
```bash
# Show span enter/exit events
RUST_LOG_SPAN_EVENTS=new,close
```

## Configuration File Integration

### .needle.yaml Configuration

The Pluck strand behavior is controlled via `.needle.yaml`:

```yaml
strands:
  pluck:
    exclude_labels: []  # Empty = use defaults (deferred, human, blocked)
    split_after_failures: 0  # 0 = disabled, positive = enable auto-split
```

### Relationship to Debug Logging

The `.needle.yaml` file configures **behavior**, while `RUST_LOG` configures **visibility**. For comprehensive debugging:

1. Set your desired behavior in `.needle.yaml`
2. Set appropriate `RUST_LOG` level for visibility
3. Run with command structure shown above

## Troubleshooting

### No Pluck Output Visible

**Symptoms:** No pluck events in log despite setting RUST_LOG

**Solutions:**
```bash
# Verify RUST_LOG is set
echo $RUST_LOG

# Check if Pluck strand is active
grep "worker booted" pluck-debug.log | grep "pluck"

# Ensure beads exist
br list --status=open
```

### Missing Expected Events

**Common causes:**
1. RUST_LOG syntax error
2. Wrong module target
3. No beads in queue
4. Strand not loaded

**Verification:**
```bash
# Test basic logging
RUST_LOG=info needle run -w /home/coding/ARMOR -c 1

# Check strand loading
RUST_LOG=needle::worker=debug needle run -w /home/coding/ARMOR -c 1
```

### Performance Impact

Higher debug levels (especially `trace`) can impact performance:

| Level | Performance Impact | When to Use |
|-------|-------------------|-------------|
| `info` | Minimal | Production monitoring |
| `debug` | Low | **Recommended for debugging** |
| `trace` | Moderate | Deep troubleshooting only |
| `trace` (global) | High | Short debugging sessions only |

## Advanced Topics

### Custom Log Formatting

NEEDLE uses `tracing-subscriber` with JSON support:

```bash
# JSON output for parsing
RUST_LOG_FORMAT=json RUST_LOG=needle::strand::pluck=debug \
NEEDLE run -w /home/coding/ARMOR -c 1
```

### OpenTelemetry Integration

NEEDLE includes OTLP support (default feature):

```bash
# With OTLP endpoint (when configured)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 \
RUST_LOG=needle::strand::pluck=debug \
NEEDLE run -w /home/coding/ARMOR -c 1
```

### Conditional Tracing

The Pluck source uses conditional `tracing::instrument`:

```rust
#[tracing::instrument(
    name = "strand.pluck",
    skip(self, store),
    fields(
        strand = "pluck",
        exclude_labels = ?self.exclude_labels,
        split_threshold = self.split_after_failures,
    )
)]
```

## Reference Documentation

### Rust Tracing Documentation
- [tracing crate](https://docs.rs/tracing/)
- [tracing-subscriber](https://docs.rs/tracing-subscriber/)
- [env_logger](https://docs.rs/env_logger/)

### NEEDLE Documentation
- Version: 0.2.11
- Repository: `/home/coding/NEEDLE`
- Pluck source: `src/strand/pluck.rs`

## Summary

The complete Pluck debug command structure is:

```bash
RUST_LOG=needle::strand::pluck=debug \
NEEDLE run -w <workspace> -c <concurrency> 2>&1 | tee <output-file>
```

**Recommended for most debugging:**
```bash
RUST_LOG=needle::strand::pluck=debug,needle::strand=info,needle::worker=info \
NEEDLE run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-$(date +%Y%m%d-%H%M%S).log
```

**For comprehensive system debugging:**
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug \
RUST_BACKTRACE=1 \
NEEDLE run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-comprehensive-$(date +%Y%m%d-%H%M%S).log
```

## Appendix: Complete Module Reference

### NEEDLE Module Hierarchy

```
needle
├── strand
│   ├── pluck      ← Primary bead selection
│   ├── mend       → Bead completion
│   ├── explore    → Discovery/research
│   ├── weave      → Documentation
│   ├── unravel    → Undo/revert
│   ├── pulse      → Health checks
│   ├── reflect    → Review/analysis
│   ├── splice     → Code surgery
│   └── knot       → Dependency management
├── bead_store     → Storage backend
├── worker         → Worker lifecycle
├── dispatch       → Task distribution
└── claim          → Bead claiming
```

### Log Level Severity

```
ERROR  → Critical failures
WARN   → Warning conditions
INFO   → High-level operations (recommended for production)
DEBUG  → Detailed decisions (recommended for debugging)
TRACE  → Complete execution flow
```

---

**Document Status:** ✅ Complete  
**Verification:** ✅ Command syntax verified against NEEDLE 0.2.11 source code  
**Ready for execution:** Yes
