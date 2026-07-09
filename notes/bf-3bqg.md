# Pluck Debug Configuration - Task bf-3bqg

## Overview
Complete debug configuration for Pluck strand filtering execution within NEEDLE system.

## Environment Status

### Pluck Executable
- **Location:** `/home/coding/.local/bin/needle`
- **Version:** needle 0.2.11 (rust, linux x86_64)
- **Type:** Standalone binary (12,307,208 bytes)
- **Status:** ✅ Accessible and functional

### Debug Scripts Available
1. **capture-pluck-debug.sh** - Basic capture script
2. **pluck-debug-config.sh** - Advanced configuration manager with presets

### Log Directory
- **Location:** `/home/coding/ARMOR/logs/pluck-debug/`
- **Status:** ✅ Created and writable
- **Permissions:** drwxr-xr-x (755)

## Debug Flag Configurations

### RUST_LOG Environment Variable Presets

| Mode | RUST_LOG Value | Use Case |
|------|---------------|----------|
| **minimal** | `needle::strand::pluck=info` | High-level strand operations only |
| **standard** | `needle::strand::pluck=debug` | Filtering decisions and statistics (default) |
| **detailed** | `needle::strand::pluck=trace` | Complete execution details |
| **comprehensive** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug` | TRACE + supporting modules |
| **full** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug` | All NEEDLE modules at DEBUG/TRACE level |
| **maximum** | `trace` | Everything at TRACE level (very verbose) |

## Command Configurations

### Quick Start (Standard Debug)
```bash
# Using the configuration manager
./pluck-debug-config.sh /home/coding/ARMOR logs/pluck-debug/pluck-debug.log standard 1

# Manual execution
export RUST_LOG="needle::strand::pluck=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee logs/pluck-debug/pluck-debug.log
```

### Detailed Execution Trace
```bash
# Maximum detail for Pluck filtering
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee logs/pluck-debug/pluck-detailed.log
```

### Using Configuration Manager
```bash
# Show available modes
./pluck-debug-config.sh --help

# Run with specific preset
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive 1

# All parameters
./pluck-debug-config.sh [workspace] [output_file] [mode] [count]
```

## Pluck Configuration (from needle config)
```yaml
strands:
  pluck:
    exclude_labels:
      - deferred
      - human
      - blocked
    split_after_failures: 3
```

## Analysis Commands

After capturing debug logs, analyze with:

```bash
# Pluck-specific operations
grep -i 'pluck' output.log

# Filtering decisions
grep -i 'filter' output.log

# Exclusion reasons
grep -i 'exclude' output.log

# Candidate evaluation
grep -i 'candidate' output.log

# Split operations
grep -i 'split' output.log

# Error conditions
grep -i 'error\|warn' output.log
```

## Verification Status

- ✅ **Pluck executable exists** - `/home/coding/.local/bin/needle`
- ✅ **Debug flags identified** - 6 preset configurations available
- ✅ **Log directory ready** - `logs/pluck-debug/` created and writable
- ✅ **Debug command configuration ready** - Multiple execution methods available

## Next Steps

The debug configuration is complete. To execute Pluck with debug logging:

1. Choose appropriate debug level (recommend: `standard` or `comprehensive`)
2. Run using configuration manager or manual RUST_LOG export
3. Analyze output using grep commands above

**Example:**
```bash
./pluck-debug-config.sh /home/coding/ARMOR logs/pluck-debug/pluck-session-$(date +%Y%m%d-%H%M%S).log comprehensive 1
```

---
*Configuration prepared for bead bf-3bqg on 2026-07-09*
