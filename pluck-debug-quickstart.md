# Pluck Debug Logging - Quick Start

**Bead:** bf-3b63  
**Configuration Status:** ✅ Complete and Operational

## Quick Start Commands

### Standard Debug Level (Recommended)
```bash
cd /home/coding/ARMOR
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug-output.log standard
```

### Comprehensive Debug Level
```bash
cd /home/coding/ARMOR
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug-comprehensive.log comprehensive
```

### Manual Setup
```bash
# Set environment variable
export RUST_LOG=needle::strand::pluck=debug

# Run NEEDLE with debug output
cd /home/coding/NEEDLE
cargo run -- run -w /home/coding/ARMOR -c 1 2>&1 | tee /tmp/pluck-manual-debug.log
```

## Configuration Presets

| Mode | RUST_LOG Setting | Use Case |
|------|------------------|----------|
| **minimal** | `needle::strand::pluck=info` | Quick health checks |
| **standard** | `needle::strand::pluck=debug` | Normal debugging (recommended) |
| **detailed** | `needle::strand::pluck=trace` | Deep troubleshooting |
| **comprehensive** | `needle::strand::pluck=trace,...` | Full system context |
| **full** | All NEEDLE modules DEBUG/TRACE | Complete debugging |
| **maximum** | `trace` | Everything (very verbose) |

## What to Expect

When debug logging is enabled, you'll see:
- ✅ Pluck strand evaluation starting
- ✅ Bead store queries with filters
- ✅ Label filtering decisions
- ✅ Candidate sorting and selection
- ✅ Split threshold checks
- ✅ Final strand results

## Analysis Commands

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

## Files

- **Configuration Script:** `pluck-debug-config.sh`
- **Full Documentation:** `pluck-debug-configuration.md`
- **This Quick Start:** `pluck-debug-quickstart.md`

## Status

✅ **Debug logging configuration:** Complete  
✅ **Filtering decision flags:** Enabled  
✅ **Log output destination:** Configured  
✅ **Ready for execution:** Yes  

The Pluck debug logging system is configured and ready for use.