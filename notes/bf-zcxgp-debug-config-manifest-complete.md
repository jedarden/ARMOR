# Debug Configuration Files Manifest - ARMOR Codebase

**Generated:** 2026-07-09  
**Bead:** bf-zcxgp  
**Task:** Locate all debug configuration files in the codebase that require validation

## Summary

This manifest documents all debug configuration files found in the ARMOR codebase. The search included common configuration file patterns (`.yaml`, `.yml`, `.json`, `.toml`, `.conf`, `.config`, `.env*`, shell scripts) and debug-related naming patterns.

## Configuration Files Requiring Validation

### 1. Primary Debug Configuration Files

| File Path | Type | Purpose | Validation Required |
|-----------|------|---------|---------------------|
| `/home/coding/ARMOR/.env.pluck-debug` | Environment | RUST_LOG configuration for Pluck debug logging | ✅ Yes - Environment variable syntax |
| `/home/coding/ARMOR/pluck-config.yaml` | YAML | Pluck strand debug and filtering configuration | ✅ Yes - YAML syntax and schema |
| `/home/coding/ARMOR/.needle.yaml` | YAML | NEEDLE workspace configuration (references debug settings) | ✅ Yes - YAML syntax and schema |

### 2. Debug Configuration Shell Scripts

| File Path | Type | Purpose | Validation Required |
|-----------|------|---------|---------------------|
| `/home/coding/ARMOR/capture-pluck-debug.sh` | Shell script | Captures Pluck debug output to log files | ✅ Yes - Shell syntax |
| `/home/coding/ARMOR/pluck-debug-config.sh` | Shell script | Configures debug settings for Pluck | ✅ Yes - Shell syntax |
| `/home/coding/ARMOR/validate-debug-config.sh` | Shell script | Validates debug configuration files | ✅ Yes - Shell syntax (self-validation) |
| `/home/coding/ARMOR/analyze-pluck-debug.sh` | Shell script | Analyzes Pluck debug logs | ✅ Yes - Shell syntax |
| `/home/coding/ARMOR/notes/bf-kjvf-pluck-debug-commands.sh` | Shell script | Debug command reference | ✅ Yes - Shell syntax |

### 3. Log Rotation Configuration

| File Path | Type | Purpose | Validation Required |
|-----------|------|---------|---------------------|
| `/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh` | Shell script | Log rotation settings for debug logs | ✅ Yes - Shell syntax |

## Configuration File Details

### .env.pluck-debug
- **Format:** Environment variable assignments
- **Purpose:** Configures RUST_LOG levels for Pluck debugging
- **Key Settings:**
  - `RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug`
  - Multiple comment-out alternative configurations
- **Validation Needs:** Environment variable syntax, valid RUST_LOG module paths

### pluck-config.yaml
- **Format:** YAML
- **Purpose:** Main Pluck debug and filtering configuration
- **Key Sections:**
  - `debug` - Debug logging level and flags
  - `modules` - Complementary debug modules
  - `filtering` - Label exclusions and split settings
  - `output` - Log file configuration
- **Validation Needs:** YAML syntax, valid values for enum fields, file path validity

### .needle.yaml
- **Format:** YAML
- **Purpose:** NEEDLE workspace configuration
- **Key Sections:**
  - `strands.pluck.exclude_labels` - Bead filtering
  - `strands.pluck.split_after_failures` - Auto-split configuration
- **Validation Needs:** YAML syntax, valid strand configuration

## Documentation Files (Reference Only)

The following documentation files reference debug configuration but do not require validation themselves:

- `/home/coding/ARMOR/docs/debug-config-manifest.md`
- `/home/coding/ARMOR/docs/debug-config-files-manifest.md`
- `/home/coding/ARMOR/docs/pluck-debug-configuration.md`
- `/home/coding/ARMOR/docs/pluck-debug-command-reference.md`
- `/home/coding/ARMOR/docs/bf-1r7s-pluck-debug-command-reference.md`
- `/home/coding/ARMOR/docs/bf-5p3g-pluck-debug-logging.md`

Plus numerous execution summary and analysis markdown files in `/home/coding/ARMOR/notes/`.

## Validation Status

### Files Requiring Validation: 7
- ✅ `.env.pluck-debug` - Environment configuration
- ✅ `pluck-config.yaml` - YAML configuration
- ✅ `.needle.yaml` - YAML configuration
- ✅ `capture-pluck-debug.sh` - Shell script
- ✅ `pluck-debug-config.sh` - Shell script
- ✅ `validate-debug-config.sh` - Shell script
- ✅ `analyze-pluck-debug.sh` - Shell script

### Log Files (Excluded from Validation)
Over 200 log files in `/home/coding/ARMOR/logs/pluck-debug/` directory - these are runtime outputs, not configuration.

## Next Steps

1. Validate YAML syntax for `.needle.yaml` and `pluck-config.yaml`
2. Validate environment variable syntax for `.env.pluck-debug`
3. Validate shell script syntax for all debug shell scripts
4. Create automated validation tests for configuration file schemas
5. Implement pre-commit hooks for configuration file validation

## Search Methodology

This manifest was created using:
1. Direct pattern search: `find -name "debug.*" -type f`
2. Extension-based search: `find -name "*.yaml" -o -name "*.yml" -o -name "*.json" -o -name "*.toml"`
3. Case-insensitive search: `find -iname "*debug*"`
4. Shell script search: `find -name "*.sh" | grep debug`
5. Manual verification of configuration file content

**Total Configuration Files Identified:** 7  
**Total Documentation/Reference Files:** 100+ (not counted as configuration)
