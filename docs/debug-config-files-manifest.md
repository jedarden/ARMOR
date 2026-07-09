# ARMOR Debug Configuration Files Manifest

**Generated:** 2026-07-09  
**Purpose:** Complete inventory of all debug configuration files in the ARMOR codebase  
**Task:** bf-zcxgp - Locate debug configuration files

## Summary

This document provides a comprehensive manifest of all debug configuration files discovered in the ARMOR codebase. These files control various aspects of debug logging, filtering behavior, and system diagnostics.

## Primary Configuration Files

### 1. `pluck-config.yaml`
**Location:** `/home/coding/ARMOR/pluck-config.yaml`  
**Type:** YAML Configuration  
**Purpose:** Main Pluck strand debug logging and filtering behavior control  

**Key Configuration Sections:**
- `debug.level`: Controls logging verbosity (info/debug/trace/off)
- `debug.log_filtering_decisions`: Enables detailed filtering operation logging
- `debug.log_bead_store_queries`: Logs bead store interactions
- `debug.log_split_evaluation`: Logs split decision logic
- `modules`: Controls complementary debug modules (strand, worker, bead_store, dispatch, claim)
- `filtering`: Configures bead selection behavior
- `output`: Controls log output destination and formatting

**Current Settings:**
- Level: `debug`
- File output: `logs/pluck-debug.log`
- Log rotation: 100MB max size, 5 backups

---

### 2. `.env.pluck-debug`
**Location:** `/home/coding/ARMOR/.env.pluck-debug`  
**Type:** Environment Configuration  
**Purpose:** RUST_LOG environment variable configuration for debug logging  

**Configuration Levels:**
- Minimal: `needle::strand::pluck=debug`
- Comprehensive: `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug`
- Maximum: `debug`

**Current Active Configuration:**
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug
```

**Usage:**
```bash
source .env.pluck-debug
needle run -w /home/coding/ARMOR -c 1
```

---

### 3. `.needle.yaml`
**Location:** `/home/coding/ARMOR/.needle.yaml`  
**Type:** YAML Configuration  
**Purpose:** NEEDLE workspace configuration for ARMOR  

**Debug-Related Configuration:**
- References RUST_LOG environment variable for debug logging control
- Contains pluck strand configuration for bead selection and filtering
- Documents that debug logging is controlled via RUST_LOG environment variable

---

## Debug Management Scripts

### 4. `pluck-debug-config.sh`
**Location:** `/home/coding/ARMOR/pluck-debug-config.sh`  
**Type:** Bash Script (Executable)  
**Purpose:** Debug configuration manager with preset configurations  

**Available Modes:**
- `minimal` - INFO level: High-level strand operations only
- `standard` - DEBUG level: Filtering decisions and statistics
- `detailed` - TRACE level: Complete execution details
- `comprehensive` - TRACE + supporting modules
- `full` - All NEEDLE modules at DEBUG/TRACE level
- `maximum` - Everything at TRACE level (very verbose)

**Usage:**
```bash
./pluck-debug-config.sh /home/coding/ARMOR output.log standard 1
```

---

### 5. `validate-debug-config.sh`
**Location:** `/home/coding/ARMOR/validate-debug-config.sh`  
**Type:** Bash Script (Executable)  
**Purpose:** Validates syntax and structure of all debug configuration files  

**Validation Checks:**
- Primary configuration files structure validation
- YAML structure validation
- Shell script syntax validation
- RUST_LOG format validation
- Expected configuration keys validation

**Usage:**
```bash
./validate-debug-config.sh
```

---

### 6. `capture-pluck-debug.sh`
**Location:** `/home/coding/ARMOR/capture-pluck-debug.sh`  
**Type:** Bash Script (Executable)  
**Purpose:** Captures pluck debug output to log files  

**Usage:**
```bash
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug.log 1
```

---

### 7. `analyze-pluck-debug.sh`
**Location:** `/home/coding/ARMOR/analyze-pluck-debug.sh`  
**Type:** Bash Script (Executable)  
**Purpose:** Analyzes captured debug logs and provides summaries  

---

## Configuration File Interactions

### Debug Configuration Hierarchy

1. **Base Layer:** `.env.pluck-debug` - Environment variable configuration
2. **Configuration Layer:** `pluck-config.yaml` - Detailed behavior control
3. **Management Layer:** Shell scripts for validation and management
4. **Workspace Layer:** `.needle.yaml` - Workspace-level configuration

### Debug Activation Flow

```
1. Source environment: source .env.pluck-debug
2. Configure behavior: pluck-config.yaml (auto-loaded)
3. Execute: needle run -w /home/coding/ARMOR -c 1
4. Optional: Use management scripts for presets/capture
```

---

## File Locations Summary

| File | Location | Type | Purpose |
|------|-----------|------|---------|
| `pluck-config.yaml` | `/home/coding/ARMOR/` | YAML | Main debug configuration |
| `.env.pluck-debug` | `/home/coding/ARMOR/` | Environment | RUST_LOG configuration |
| `.needle.yaml` | `/home/coding/ARMOR/` | YAML | Workspace configuration |
| `pluck-debug-config.sh` | `/home/coding/ARMOR/` | Script | Debug preset manager |
| `validate-debug-config.sh` | `/home/coding/ARMOR/` | Script | Configuration validator |
| `capture-pluck-debug.sh` | `/home/coding/ARMOR/` | Script | Debug capture utility |
| `analyze-pluck-debug.sh` | `/home/coding/ARMOR/` | Script | Log analysis utility |

---

## Related Documentation

- `/home/coding/ARMOR/docs/debug-config-manifest.md` - Debug configuration manifest
- `/home/coding/ARMOR/docs/pluck-debug-configuration.md` - Detailed pluck debug documentation
- `/home/coding/ARMOR/docs/pluck-debug-command-reference.md` - Command reference

---

## Validation Status

All debug configuration files have been located and documented in this manifest. The configuration files are:

- ✓ Structurally valid
- ✓ Properly formatted
- ✓ Executable (for scripts)
- ✓ Documented with inline comments
- ✓ Integrated with the NEEDLE system

---

## Notes

- No standard `debug.yaml`, `debug.yml`, `debug.json`, or `debug.toml` files were found in the codebase
- Debug configuration is primarily handled through `pluck-config.yaml` and environment variables
- All debug-related files are located in the root `/home/coding/ARMOR/` directory
- Debug logs are written to `logs/pluck-debug.log` with rotation enabled

---

## Maintenance

When adding new debug configuration files:
1. Update this manifest
2. Run `./validate-debug-config.sh` to ensure validity
3. Document configuration options in the file itself
4. Ensure integration with existing debug configuration hierarchy

---

**End of Manifest**