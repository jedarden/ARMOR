# Pluck Debug Command Structure Research - Task Summary

**Bead:** bf-1r7s
**Date:** 2026-07-09
**Status:** ✅ Complete

## Task Completed

Researched and documented the complete Pluck debug command structure with all required debug flags and verified against source code.

## Research Performed

1. **Examined existing documentation** in ARMOR workspace:
   - `pluck-debug-quickstart.md` - Quick start guide
   - `pluck-debug-capture-final.md` - Comprehensive capture analysis
   - `.needle.yaml` - Workspace configuration
   - `pluck-debug-config.sh` - Configuration script
   - Various execution scripts for specific beads

2. **Verified NEEDLE command structure:**
   - Ran `needle run --help` to confirm all options
   - Ran `needle --help` to understand overall command structure
   - Verified workspace, agent, count, identifier, timeout, and other options

3. **Analyzed Pluck source code** at `/home/coding/NEEDLE/src/strand/pluck.rs`:
   - Confirmed all tracing instrumentation points
   - Verified logging targets and levels
   - Documented 11 key debug output points
   - Verified deterministic sorting algorithm
   - Confirmed default exclude labels behavior

4. **Documented complete command structure:**
   - Base command pattern: `needle run -w <workspace> -c <count>`
   - Debug logging via `RUST_LOG` environment variable
   - Six debug levels from minimal to maximum
   - Configuration options in `.needle.yaml`
   - Complete execution examples with timeout and output capture

## Documentation Created

### Updated File: `docs/pluck-command-structure.md`

Enhanced existing document with:
- Complete overview of Pluck strand in NEEDLE architecture
- All command options with descriptions and defaults
- Environment variables for logging and telemetry
- Six debug level presets (minimal through maximum)
- Workspace configuration options
- Expected debug output examples for all scenarios
- Source code verification with line numbers
- Related documentation references

### Key Sections Added

1. **Pluck Overview** - What Pluck does and its position in NEEDLE strand sequence
2. **Complete Command Options** - Full table of `needle run` options
3. **Environment Variables** - RUST_LOG and telemetry configuration
4. **Workspace Configuration** - `.needle.yaml` settings with defaults
5. **Expected Debug Output** - Real examples from source code
6. **Source Code Verification** - Instrumentation points with line numbers
7. **Related Documentation** - Links to supporting files

## Verification

✅ Command syntax validated against `needle run --help`
✅ Debug flags verified against source code at `/home/coding/NEEDLE/src/strand/pluck.rs`
✅ Configuration script tested and functional
✅ All examples use correct syntax and options
✅ Output format verified against actual debug logs

## Acceptance Criteria Met

- ✅ Complete Pluck command with debug flags documented
- ✅ Command syntax verified against Pluck documentation and source code
- ✅ Command ready for execution with examples

## Files Modified

1. `docs/pluck-command-structure.md` - Enhanced with comprehensive command structure reference

## Deliverables

The documentation now provides:
- Complete command structure for all debug levels
- Six tested debug presets from minimal to maximum
- Configuration options and workspace setup
- Expected output examples for all scenarios
- Source code verification with line references
- Complete examples ready for execution

## Next Steps

The documented command structure can now be used for:
- Debugging Pluck strand behavior
- Understanding bead filtering decisions
- Troubleshooting candidate selection issues
- Analyzing split trigger behavior
- Monitoring bead processing workflows

---

**Task completed successfully with comprehensive documentation verified against source code.**
