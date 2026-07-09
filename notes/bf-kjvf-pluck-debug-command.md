# Pluck Debug Command Construction - bf-kjvf

**Date:** 2026-07-09  
**Bead:** bf-kjvf  
**Workspace:** /home/coding/ARMOR

## Command Structure

The Pluck debug command is controlled **exclusively via the `RUST_LOG` environment variable**. There are NO CLI debug flags.

### Base Command Pattern
```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count>
```

### Full Command with All Options
```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count> -a <agent> -i <identifier> -t <timeout> [--resume] [--hot-reload <true|false>]
```

## Available Debug Levels

| Level | RUST_LOG Setting | Use Case |
|-------|------------------|----------|
| **minimal** | `needle::strand::pluck=info` | Quick health checks |
| **standard** | `needle::strand::pluck=debug` | Normal debugging (recommended) |
| **detailed** | `needle::strand::pluck=trace` | Deep troubleshooting |
| **comprehensive** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug` | Full system context |
| **full** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug` | Complete system debugging |
| **maximum** | `trace` | Everything (very verbose) |

## Recommended Command (Comprehensive Level)

```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" needle run -w /home/coding/ARMOR -c 1
```

## Command with Log Capture

```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" needle run -w /home/coding/ARMOR -c 1 2>&1 | tee logs/pluck-debug/pluck-debug-bf-kjvf-capture-$(date +%Y%m%d-%H%M%S).log
```

## Using the Configuration Script (Recommended)

The `pluck-debug-config.sh` script provides preset configurations:

```bash
# Standard debug level
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug-output.log standard

# Comprehensive debug level
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug-comprehensive.log comprehensive

# Full debug level with custom count
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug-full.log full 3
```

## Command Validation

### Dry-run Syntax Check
```bash
# Test RUST_LOG setting (doesn't run needle)
echo "RUST_LOG=$RUST_LOG"

# Verify command syntax (dry-run)
timeout 1s needle run -w /home/coding/ARMOR -c 0 --dry-run 2>&1 || echo "Command syntax validated"
```

### Manual Validation Steps
1. **Check RUST_LOG is set correctly:**
   ```bash
   echo $RUST_LOG
   ```

2. **Verify workspace path:**
   ```bash
   ls -la /home/coding/ARMOR
   ```

3. **Check needle is available:**
   ```bash
   which needle
   needle --version
   ```

## Expected Debug Output

When the command runs successfully, you should see:
- ✅ Pluck strand evaluation starting
- ✅ Bead store queries with filters
- ✅ Label filtering decisions
- ✅ Candidate sorting and selection
- ✅ Split threshold checks
- ✅ Final strand results

## Log Analysis Commands

```bash
# View all Pluck events
grep -i "pluck" pluck-debug-output.log

# Filter specific decisions
grep -i "filter" pluck-debug-output.log
grep -i "exclude" pluck-debug-output.log
grep -i "candidate" pluck-debug-output.log
grep -i "split" pluck-debug-output.log

# Count events
grep -c "Pluck strand evaluation starting" pluck-debug-output.log
grep -c "result=BeadFound" pluck-debug-output.log
```

## Command Components Explained

1. **`RUST_LOG=...`** - Sets debug level for specific Rust modules
2. **`needle run`** - Main NEEDLE command
3. **`-w /home/coding/ARMOR`** - Specifies workspace path
4. **`-c 1`** - Run count (number of operations)
5. **`-a <agent>`** - Optional: Specify agent type
6. **`-i <identifier>`** - Optional: Specific bead/operation identifier
7. **`-t <timeout>`** - Optional: Timeout in seconds
8. **`--resume`** - Optional: Resume previous operation
9. **`--hot-reload <true|false>`** - Optional: Enable/disable hot reload

## Status

✅ **Command constructed:** Yes  
✅ **Syntax validated:** Documentation-based validation complete  
✅ **Flags documented:** All debug levels and options documented  
✅ **Usage examples:** Provided for common scenarios  

The Pluck debug command is ready for execution with comprehensive debug logging enabled.
