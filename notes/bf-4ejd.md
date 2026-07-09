# Pluck Debug Flags Reference

**Bead:** bf-4ejd  
**Date:** 2026-07-09  
**Task:** Identify Pluck debug flags and command structure

## Overview

Pluck is a strand in the NEEDLE system that selects beads for processing by filtering and sorting candidates from the bead store. Debug output is controlled via the `RUST_LOG` environment variable, not CLI flags.

## Primary Debug Control: RUST_LOG

The `RUST_LOG` environment variable controls debug output at the Rust crate level. No CLI flags like `--debug` or `--verbose` exist for `needle run`.

### Available Debug Levels

| Mode | RUST_LOG Setting | Use Case |
|------|------------------|----------|
| **minimal** | `needle::strand::pluck=info` | Quick health checks |
| **standard** | `needle::strand::pluck=debug` | Normal debugging (recommended) |
| **detailed** | `needle::strand::pluck=trace` | Deep troubleshooting |
| **comprehensive** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug` | Full system context |
| **full** | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug` | Complete debugging |
| **maximum** | `trace` | Everything (very verbose) |

## Full Command Structure

### Method 1: Using Helper Script (Recommended)

```bash
cd /home/coding/ARMOR
bash pluck-debug-config.sh /home/coding/ARMOR pluck-debug-output.log standard
```

**Script usage:**
```bash
./pluck-debug-config.sh [workspace] [output_file] [mode] [count]
```

### Method 2: Manual Environment Variable

```bash
# Set the debug level
export RUST_LOG=needle::strand::pluck=debug

# Run NEEDLE with output capture
needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### Method 3: Inline Environment Variable

```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1
```

## Expected Debug Output

When Pluck debug logging is enabled at `debug` level, you should see:

```
DEBUG needle::strand::pluck: Pluck strand evaluation starting
  exclude_labels=["deferred", "human", "blocked"]
  split_threshold=3

DEBUG needle::strand::pluck: Querying bead store for ready candidates
  filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }

DEBUG needle::strand::pluck: Bead store returned N candidates
  count=5

DEBUG needle::strand::pluck: Filtering by excluded labels
  excluded_beads=["bf-1234", "bf-5678"]
  reasons=["label:deferred", "label:blocked"]

DEBUG needle::strand::pluck: Filtering by status and assignee
  remaining=3

DEBUG needle::strand::pluck: Sorting candidates by priority
  first_candidate="bf-abcd"

DEBUG needle::strand::pluck: Checking split threshold
  failure_count=2
  split_threshold=3
  should_split=false

DEBUG needle::strand::pluck: Strand evaluation complete
  result=BeadFound("bf-abcd")
```

## Additional Environment Variables

### RUST_BACKTRACE
For debugging errors with stack traces:
```bash
export RUST_BACKTRACE=1
```

### NEEDLE_INNER
Internal use - detects re-entrant tmux invocations. Not needed for normal debugging.

## Log Analysis Commands

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
grep -c "result=NoWork" pluck-debug.log
grep -c "result=Split" pluck-debug.log
```

## Troubleshooting

### No Pluck output visible

1. **Check RUST_LOG is set correctly:**
   ```bash
   echo $RUST_LOG
   ```

2. **Verify Pluck strand is active:**
   ```bash
   grep "worker booted" pluck-debug.log | grep "pluck"
   ```

3. **Ensure beads are available for processing:**
   ```bash
   br list --status=open
   ```

### Binary Location

The `needle` binary is installed at:
- **Path:** `/home/coding/.local/bin/needle`
- **Also available in:** `/home/coding/NEEDLE/target/release/needle`

## Pluck Strand Function

Pluck is the first strand evaluated in the NEEDLE processing pipeline:

1. Queries the bead store for ready beads
2. Filters by labels (`deferred`, `human`, `blocked`)
3. Filters by assignee
4. Sorts candidates by priority
5. Checks for split conditions
6. Returns `NoWork` / `BeadFound` / `Split` result

## Standard NEEDLE Strands

```
strands=["pluck", "mend", "explore", "weave", "unravel", "pulse", "reflect", "splice", "knot"]
```

## Files Referenced

- **Configuration Script:** `/home/coding/ARMOR/pluck-debug-config.sh`
- **Quick Start:** `/home/coding/ARMOR/pluck-debug-quickstart.md`
- **Full Config:** `/home/coding/ARMOR/pluck-debug-configuration.md`
- **Binary:** `/home/coding/.local/bin/needle`

## Summary

✅ **Debug flags identified:** `RUST_LOG` environment variable  
✅ **No CLI debug flags:** `needle run` has no `--debug` or `--verbose` flags  
✅ **Command structure:** `RUST_LOG=<level> needle run -w <workspace> -c <count>`  
✅ **Helper script available:** `pluck-debug-config.sh` with preset modes  
✅ **Ready for execution:** Yes  

The Pluck debug logging system is fully configured and documented. Use `RUST_LOG=needle::strand::pluck=debug` for standard debugging needs.
