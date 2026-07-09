# Pluck Debug Command Structure - Complete Reference

**Bead:** bf-1r7s  
**Status:** ✅ Complete and Verified  
**Last Updated:** 2026-07-09

## Overview

Pluck is NEEDLE's bead selection strand, responsible for evaluating and selecting beads for agent execution. This document provides the complete command structure for debugging Pluck operations.

## Base Command Structure

### Minimal Command
```bash
needle run -w <workspace> -c <count>
```

**Parameters:**
- `-w <workspace>`: Path to the workspace directory (default: `/home/coding/ARMOR`)
- `-c <count>`: Number of operations to perform (default: `1`)

**Example:**
```bash
needle run -w /home/coding/ARMOR -c 1
```

## Debug Logging Configuration

### Environment Variable
All debug logging is controlled via the `RUST_LOG` environment variable.

### Debug Levels by Module

| Module | Description | Levels |
|--------|-------------|--------|
| `needle::strand::pluck` | Core pluck strand logic | `info`, `debug`, `trace` |
| `needle::strand` | Strand framework | `info`, `debug`, `trace` |
| `needle::bead_store` | Bead data store operations | `info`, `debug`, `trace` |
| `needle::worker` | Worker lifecycle management | `info`, `debug`, `trace` |
| `needle::dispatch` | Agent dispatch logic | `info`, `debug`, `trace` |
| `needle::claim` | Bead claiming logic | `info`, `debug`, `trace` |

## Preset Configurations

### Level 1: Minimal
```bash
export RUST_LOG=needle::strand::pluck=info
```
**Purpose:** High-level strand operations only  
**Output:** Basic strand start/completion messages

### Level 2: Standard (Recommended)
```bash
export RUST_LOG=needle::strand::pluck=debug
```
**Purpose:** Filtering decisions and statistics  
**Output:** 
- Strand evaluation start
- Bead store queries with filters
- Label filtering decisions
- Candidate sorting and selection
- Split threshold checks
- Final strand results

### Level 3: Detailed
```bash
export RUST_LOG=needle::strand::pluck=trace
```
**Purpose:** Complete execution details  
**Output:** Everything from Standard + detailed execution flow

### Level 4: Comprehensive
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
```
**Purpose:** Pluck TRACE + supporting modules  
**Output:** Detailed pluck execution + supporting module context

### Level 5: Full
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug
```
**Purpose:** All NEEDLE modules at DEBUG/TRACE level  
**Output:** Complete system-wide debugging

### Level 6: Maximum
```bash
export RUST_LOG=trace
```
**Purpose:** Everything at TRACE level  
**Output:** Extremely verbose all-module tracing  
**Warning:** Very large log files

## Complete Execution Commands

### Command Pattern 1: Direct Execution
```bash
export RUST_LOG=needle::strand::pluck=debug
needle run -w /home/coding/ARMOR -c 1
```

### Command Pattern 2: With Output Capture
```bash
export RUST_LOG=needle::strand::pluck=debug
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### Command Pattern 3: With Timeout
```bash
export RUST_LOG=needle::strand::pluck=debug
timeout 180s needle run -w /home/coding/ARMOR -c 1
```

### Command Pattern 4: Comprehensive Monitoring
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
timeout 180s needle run -w /home/coding/ARMOR -c 1 > >(tee -a stdout.log) 2> >(tee -a stderr.log >&2)
```

## Using the Configuration Script

### Quick Usage
```bash
cd /home/coding/ARMOR
bash pluck-debug-config.sh /home/coding/ARMOR output.log standard
```

### Available Modes
```bash
bash pluck-debug-config.sh /home/coding/ARMOR output.log minimal       # INFO level
bash pluck-debug-config.sh /home/coding/ARMOR output.log standard      # DEBUG level (recommended)
bash pluck-debug-config.sh /home/coding/ARMOR output.log detailed      # TRACE level
bash pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive # Multi-module DEBUG/TRACE
bash pluck-debug-config.sh /home/coding/ARMOR output.log full          # All NEEDLE modules
bash pluck-debug-config.sh /home/coding/ARMOR output.log maximum       # Everything at TRACE
```

## Expected Debug Output

### When Logging is Enabled
- ✅ **Strand evaluation start** - Shows exclude_labels configuration and split_threshold
- ✅ **Bead store queries** - Filter parameters and candidate count
- ✅ **Label filtering** - Excluded beads and reasons
- ✅ **Status/assignee filtering** - Remaining candidates
- ✅ **Candidate sorting** - Priority order and first candidate
- ✅ **Split threshold checks** - Failure count analysis
- ✅ **Final results** - NoWork / BeadFound / Split

## Log Analysis Commands

### View Pluck Events
```bash
grep -i "pluck" pluck-debug.log
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

## Command Verification

### Verify needle Command
```bash
command -v needle
needle run --help
```

### Validate Syntax
```bash
bash test-pluck-syntax.sh
```

### Test Configuration
```bash
bash pluck-debug-config.sh /home/coding/ARMOR test-debug.log standard
```

## Complete Command Examples

### Example 1: Quick Debug Check
```bash
export RUST_LOG=needle::strand::pluck=debug
cd /home/coding/ARMOR
needle run -w . -c 1
```

### Example 2: Comprehensive Analysis
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug
cd /home/coding/ARMOR
timeout 180s needle run -w . -c 1 > pluck-comprehensive.log 2>&1
```

### Example 3: Production Monitoring
```bash
cd /home/coding/ARMOR
bash pluck-debug-config.sh . logs/pluck-monitor-$(date +%Y%m%d-%H%M%S).log comprehensive
```

### Example 4: Deep Troubleshooting
```bash
export RUST_LOG=trace
cd /home/coding/ARMOR
timeout 300s needle run -w . -c 1 > pluck-maximum.log 2>&1
```

## File Structure Reference

### Key Files
- `pluck-debug-config.sh` - Main configuration script
- `test-pluck-syntax.sh` - Command syntax validation
- `execute-pluck-bf-*.sh` - Bead-specific execution scripts
- `capture-pluck-debug.sh` - Debug capture utility

### Documentation
- `pluck-debug-quickstart.md` - Quick start guide
- `pluck-debug-configuration.md` - Full configuration documentation
- `pluck-debug-summary.md` - Implementation summary
- `docs/pluck-command-structure.md` - This reference

## Best Practices

1. **Start with Standard level** (`needle::strand::pluck=debug`) for most debugging
2. **Use Detailed level** (`needle::strand::pluck=trace`) for deep troubleshooting
3. **Apply Comprehensive** only when system-wide context is needed
4. **Always use timeout** for long-running executions to prevent hangs
5. **Capture output** with `tee` for analysis and archival
6. **Use configuration script** for consistent, repeatable execution

## Troubleshooting

### Command Not Found
```bash
# Verify needle is in PATH
which needle
# Or use full path
~/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1
```

### No Debug Output
```bash
# Verify RUST_LOG is set
echo $RUST_LOG
# Ensure needle was built with debug symbols
cargo build --release
```

### Log File Too Large
```bash
# Use a lower debug level
export RUST_LOG=needle::strand::pluck=info
# Or filter output
needle run -w . -c 1 2>&1 | grep -i "pluck\|filter\|candidate" > filtered.log
```

## Status Verification

✅ **Command syntax:** Validated and tested  
✅ **Debug flags:** All levels documented and verified  
✅ **Configuration script:** Functional with 6 presets  
✅ **Documentation:** Complete with examples  
✅ **Ready for execution:** Yes

---

**This reference document consolidates all Pluck debug command knowledge into a single, authoritative source.**