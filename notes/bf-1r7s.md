# Bead bf-1r7s: Pluck Debug Command Documentation

**Date:** 2026-07-09  
**Status:** ✅ Complete

## Summary

Researched and documented the complete Pluck debug command structure for NEEDLE 0.2.11, including all RUST_LOG configurations, tracing instrumentation points, and practical execution examples.

## Work Completed

### 1. Source Code Analysis
- Analyzed `/home/coding/NEEDLE/src/strand/pluck.rs` (917 lines)
- Extracted all `tracing::debug!()`, `tracing::info!()`, and `tracing::error!()` calls
- Documented `tracing::instrument` span fields

### 2. Documentation Created
Created comprehensive reference: `/home/coding/ARMOR/docs/pluck-debug-command-reference.md`

Includes:
- Complete command structure with all debug flags
- 6 preset RUST_LOG configurations (minimal through maximum)
- All NEEDLE module targets and log levels
- Complete tracing event sequence (10+ event types)
- Practical execution examples with scripts
- Log analysis commands
- Troubleshooting guide
- Performance impact analysis

### 3. Key Findings

#### Recommended Debug Command (Standard)
```bash
RUST_LOG=needle::strand::pluck=debug \
NEEDLE run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-$(date +%Y%m%d-%H%M%S).log
```

#### Comprehensive Debug Command
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug \
RUST_BACKTRACE=1 \
NEEDLE run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-comprehensive-$(date +%Y%m%d-%H%M%S).log
```

#### RUST_LOG Module Targets
- `needle::strand::pluck` - Primary Pluck strand
- `needle::strand` - All strand modules  
- `needle::bead_store` - Bead storage operations
- `needle::worker` - Worker lifecycle
- `needle::dispatch` - Task dispatch
- `needle::claim` - Bead claiming

### 4. Verification
- Command syntax verified against NEEDLE 0.2.11 source code
- Cross-referenced with existing ARMOR workspace documentation
- All debug levels validated (error, warn, info, debug, trace)
- Confirmed integration with .needle.yaml configuration

## Deliverables

✅ Complete command reference document  
✅ 6 preset RUST_LOG configurations documented  
✅ 10+ tracing event types documented  
✅ Practical execution examples with scripts  
✅ Log analysis commands  
✅ Troubleshooting guide  
✅ Performance impact analysis  

## Integration

Document integrates with existing ARMOR workspace:
- References `.needle.yaml` configuration
- Compatible with existing `pluck-debug-config.sh` script
- Aligns with existing documentation in workspace

## Acceptance Criteria Met

✅ Complete Pluck command with debug flags documented  
✅ Command syntax verified against Pluck source code  
✅ Command ready for execution  
✅ Comprehensive reference created for future use

---

# Complete Pluck Debug Command Structure

## Core Command Structure

### Base Command
```bash
needle run -w <workspace> -c <count>
```

### With Debug Logging
```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count> 2>&1 | tee <output_file>
```

## Command Options (needle run)

| Option | Short | Parameter | Description | Default |
|--------|-------|-----------|-------------|---------|
| `--workspace` | `-w` | `<PATH>` | Workspace directory to process beads from | Required |
| `--agent` | `-a` | `<NAME>` | Agent adapter to use | Config default |
| `--count` | `-c` | `<NUMBER>` | Number of workers to launch | 1 |
| `--identifier` | `-i` | `<ID>` | Worker identifier (overrides NATO naming) | Auto-generated |
| `--timeout` | `-t` | `<SECONDS>` | Agent execution timeout | Config default |
| `--resume` | | | Resume existing worker session | false |
| `--hot-reload` | | `true\|false` | Enable hot-reload for this worker | Config default |

## Environment Variables

| Variable | Purpose | Examples |
|----------|---------|----------|
| `RUST_LOG` | Controls Rust crate-level logging | `needle::strand::pluck=debug` |
| `RUST_BACKTRACE` | Enable backtraces on errors | `1` |

## Complete Command Examples

### Basic Example (Standard Debug)
```bash
RUST_LOG=needle::strand::pluck=debug \
  needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### With Comprehensive Logging
```bash
RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug \
  needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-comprehensive.log
```

### With Multiple Workers
```bash
RUST_LOG=needle::strand::pluck=debug \
  needle run -w /home/coding/ARMOR -c 3 2>&1 | tee pluck-multi-worker.log
```

### With Agent Specification
```bash
RUST_LOG=needle::strand::pluck=debug \
  needle run -w /home/coding/ARMOR -a claude -c 1 2>&1 | tee pluck-claude.log
```

### With Timeout
```bash
RUST_LOG=needle::strand::pluck=debug \
  needle run -w /home/coding/ARMOR -t 300 -c 1 2>&1 | tee pluck-timeout.log
```

### With Custom Identifier
```bash
RUST_LOG=needle::strand::pluck=debug \
  needle run -w /home/coding/ARMOR -i "debug-worker-1" -c 1 2>&1 | tee pluck-custom.log
```

## Configuration Script Usage

The `pluck-debug-config.sh` script provides convenient preset configurations:

```bash
# Standard debug level
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log standard

# Comprehensive debug level
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log comprehensive

# Detailed trace level
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log detailed

# Full system debugging
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log full

# Maximum verbosity
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log maximum
```

## Log Analysis Commands

```bash
# View all Pluck-related events
grep -i "pluck" pluck-debug.log

# Filter specific decisions
grep -i "filter" pluck-debug.log
grep -i "exclude" pluck-debug.log
grep -i "candidate" pluck-debug.log
grep -i "split" pluck-debug.log

# Count events by type
grep -c "Pluck strand evaluation starting" pluck-debug.log
grep -c "result=BeadFound" pluck-debug.log
grep -c "result=NoWork" pluck-debug.log
grep -c "result=Split" pluck-debug.log
```

## Summary

The complete Pluck debug command structure provides:

✅ **Base command:** `needle run -w <workspace> -c <count>`  
✅ **Debug control:** Via `RUST_LOG` environment variable  
✅ **Six preset levels:** From minimal to maximum verbosity  
✅ **Flexible options:** Agent selection, timeout, custom identifiers  
✅ **Output capture:** Standard shell redirection with `tee`  
✅ **Analysis tools:** Grep-based log filtering and counting  
✅ **Script automation:** `pluck-debug-config.sh` for convenience

All commands have been verified against NEEDLE 0.2.11 documentation and source code.
