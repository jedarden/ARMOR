# Pluck Debug Flags and Logging Configuration - Summary

**Task:** bf-5p3g  
**Date:** 2026-07-09  
**Status:** COMPLETE

## Overview

Comprehensive documentation of Pluck debug flags and logging configuration has been completed. The findings show that Pluck (within NEEDLE) uses Rust's standard `tracing` crate for debug output, with all logging controlled via the `RUST_LOG` environment variable.

## Key Findings

### Primary Debug Mechanism
- **Environment Variable:** `RUST_LOG`
- **Module Path:** `needle::strand::pluck`
- **Log Levels:** `error`, `warn`, `info`, `debug`, `trace`

### Available Debug Configurations

| Configuration | Use Case |
|---------------|----------|
| `RUST_LOG=needle::strand::pluck=debug` | Standard Pluck debugging - filtering decisions, counts |
| `RUST_LOG=needle::strand::pluck=trace` | Detailed execution trace - all variables, function calls |
| `RUST_LOG=needle::strand=debug,needle::strand::pluck=trace` | Full strand context |
| `RUST_LOG=needle::strand::pluck=trace,needle::worker=debug,needle::bead_store=debug` | Complete worker context |

### Filtering-Related Debug Messages

The following debug messages appear when Pluck debug logging is enabled:

1. **Evaluation Start** - Configuration values for this cycle
2. **Bead Store Query** - Filters being passed to bead store
3. **Candidate Count** - Number of beads passing ready() filter
4. **Label Filtering** - Which beads excluded by label and why
5. **Status/Assignee Filtering** - Beads excluded due to status/assignee
6. **Sorting Results** - How candidates are sorted
7. **Split Decision** - Whether bead splitting is triggered
8. **Final Result** - Outcome of the evaluation

### Usage Examples

```bash
# Basic debug
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1

# With output capture
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log

# Using provided capture script
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug.log 1
```

## Acceptance Criteria Status

- ✅ **List of available debug flags/variables found** - `RUST_LOG` is the primary mechanism; no CLI-specific flags exist
- ✅ **Documentation of which flags control filtering decision logging** - `needle::strand::pluck=debug` enables filtering decisions; `trace` provides maximum detail
- ✅ **Clear instructions on how to enable debug output** - Multiple examples with usage patterns provided

## Documentation Created

- **Primary Documentation:** `docs/pluck-debug-configuration.md` (270 lines)
- **Capture Script:** `capture-pluck-debug.sh` (verified and functional)
- **Configuration Reference:** `.needle.yaml` with Pluck settings

## Related Modules

- `needle::strand` - All strand implementations
- `needle::worker` - Worker state machine and coordination  
- `needle::bead_store` - Bead storage and retrieval operations
- `needle::dispatch` - Task dispatching and execution

## Conclusion

No Pluck-specific CLI flags exist - all debugging is controlled via the standard Rust `RUST_LOG` environment variable. The comprehensive documentation at `docs/pluck-debug-configuration.md` provides complete guidance on enabling and interpreting Pluck debug output, particularly for filtering decisions.
