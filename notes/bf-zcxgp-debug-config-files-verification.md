# Debug Configuration Files Location - ARMOR Codebase

**Task:** bf-zcxgp - Locate debug configuration files  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR

## Summary

Successfully located all debug configuration files in the ARMOR codebase. The search covered common debug configuration file patterns (debug.yaml, debug.yml, debug.json, debug.toml) and identified the actual debug configuration infrastructure used by the project.

## Search Results

### Primary Configuration Files Found

1. **`pluck-config.yaml`** 
   - **Path:** `/home/coding/ARMOR/pluck-config.yaml`
   - **Type:** YAML configuration file
   - **Purpose:** Main Pluck strand debug logging and filtering configuration
   - **Contains:** debug level, logging flags, module settings, filtering rules, output configuration

2. **`.env.pluck-debug`**
   - **Path:** `/home/coding/ARMOR/.env.pluck-debug`
   - **Type:** Environment configuration file
   - **Purpose:** RUST_LOG environment variable settings for debug logging
   - **Contains:** Multiple debug level presets for comprehensive logging

3. **`.needle.yaml`**
   - **Path:** `/home/coding/ARMOR/.needle.yaml`
   - **Type:** YAML configuration file
   - **Purpose:** NEEDLE workspace configuration with debug references
   - **Contains:** Pluck strand configuration, references debug logging via RUST_LOG

### Standard Debug Configuration Patterns NOT Found

The following common debug configuration file patterns were searched but **not found** in the ARMOR codebase:
- ❌ `debug.yaml`
- ❌ `debug.yml`
- ❌ `debug.json`
- ❌ `debug.toml`

**Note:** ARMOR uses a custom debug configuration approach centered around `pluck-config.yaml` and environment variables rather than standard debug.* file patterns.

### Supporting Debug Scripts Found

- **`pluck-debug-config.sh`** - Debug configuration manager with preset modes
- **`capture-pluck-debug.sh`** - Automated debug log capture
- **`analyze-pluck-debug.sh`** - Debug log analysis tool
- **`validate-debug-config.sh`** - Configuration validation script

## Verification

All discovered files were verified for:
- ✅ File existence and accessibility
- ✅ Proper YAML syntax (for .yaml files)
- ✅ Comprehensive debug configuration coverage
- ✅ Active usage in the codebase

## Configuration Coverage

The ARMOR debug configuration provides:
- **Debug Levels:** info, debug, trace, off
- **Module Coverage:** strand::pluck, strand, bead_store, worker, dispatch, claim
- **Output Management:** File logging, timestamps, source location, colorization, log rotation
- **Filtering Options:** Label exclusions, auto-split thresholds, sort order

## Conclusion

✅ **All debug configuration files in the ARMOR codebase have been located and catalogued.**

The project uses a well-structured debug configuration system centered around `pluck-config.yaml` with supporting environment configuration in `.env.pluck-debug` and workspace settings in `.needle.yaml`. No standard debug.* file patterns exist in this codebase, as ARMOR uses a custom configuration approach.

---

**Next Steps:** See `/home/coding/ARMOR/notes/bf-zcxgp-debug-config-manifest.md` for comprehensive details on all discovered files.
