# Pluck Debug Flags and Logging Configuration - Task Summary

**Task:** bf-5p3g  
**Date:** 2026-07-09  
**Status:** ✅ Complete

## Findings

### Primary Debug Control
- **Environment Variable:** `RUST_LOG`
- **No CLI flags** - all debug configuration is via environment variables

### Available Module Paths
1. `needle::strand::pluck` - Core Pluck filtering decisions
2. `needle::strand` - All strand implementations  
3. `needle::worker` - Worker coordination
4. `needle::bead_store` - Bead storage operations
5. `needle::dispatch` - Task dispatching

### Recommended Configurations

| Level | Configuration | Use Case |
|-------|--------------|----------|
| debug | `RUST_LOG=needle::strand::pluck=debug` | Filtering decisions, candidate counts |
| trace | `RUST_LOG=needle::strand::pluck=trace` | Detailed execution flow, all variables |
| comprehensive | `RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug` | Full worker context |

### Usage Methods

1. **Direct export:**
   ```bash
   export RUST_LOG=needle::strand::pluck=debug
   needle run -w /home/coding/ARMOR -c 1
   ```

2. **Source .env file:**
   ```bash
   source .env.pluck-debug
   needle run -w /home/coding/ARMOR -c 1
   ```

3. **Capture script:**
   ```bash
   ./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug.log 1
   ```

### Key Files
- **Configuration:** `.needle.yaml` → `strands.pluck`
- **Debug .env:** `.env.pluck-debug` 
- **Capture script:** `capture-pluck-debug.sh`
- **Documentation:** `docs/pluck-debug-configuration.md`

## Filtering-Related Debug Output

When debug is enabled, Pluck logs:
1. Evaluation start with configuration values
2. Bead store queries and filters
3. Candidate counts
4. Label-based exclusions
5. Status/assignee filtering
6. Sorting decisions
7. Split trigger evaluation
8. Final selection result

## Acceptance Criteria Met

✅ List of available debug flags/variables found  
✅ Documentation of which flags control filtering decision logging  
✅ Clear instructions on how to enable debug output  

## Documentation Status

Comprehensive documentation already exists at:
- `/home/coding/ARMOR/docs/pluck-debug-configuration.md`

This document provides complete coverage of:
- Environment variable configuration
- Module paths and log levels
- Recommended configurations for different scenarios
- Expected debug output messages
- Usage examples and scripts
- Troubleshooting guidance

No additional documentation is needed - the existing guide is thorough and complete.
