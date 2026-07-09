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
