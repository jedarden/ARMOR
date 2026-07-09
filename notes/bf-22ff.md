# Pluck Log File Output Redirection Configuration

**Bead ID:** bf-22ff
**Completed:** 2026-07-09
**Task:** Configure log file output redirection for Pluck execution

## Overview

Successfully configured comprehensive log file output redirection for Pluck execution with automated log rotation. The configuration provides multiple output streams (stdout, stderr, combined) and supports flexible RUST_LOG presets.

## Configuration Details

### Log Directory Location
- **Path:** `/home/coding/ARMOR/logs/pluck-debug`
- **Status:** ✓ Created and verified
- **Permissions:** drwxrwxrwx (writable)

### Output Redirection Script
- **File:** `/home/coding/ARMOR/pluck-log-redirection.sh`
- **Purpose:** Main configuration and validation tool
- **Status:** ✓ Executable and tested

### Log Rotation Script
- **File:** `/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh`
- **Purpose:** Automated log file rotation and cleanup
- **Status:** ✓ Configured and tested

## Features Implemented

### 1. Multi-Stream Output Capture
- **Stdout Log:** `pluck-stdout-{BEAD_ID}-{TIMESTAMP}.log`
- **Stderr Log:** `pluck-stderr-{BEAD_ID}-{TIMESTAMP}.log`
- **Combined Log:** `pluck-combined-{BEAD_ID}-{TIMESTAMP}.log`
- **Summary Log:** `pluck-summary-{BEAD_ID}-{TIMESTAMP}.log`

### 2. RUST_LOG Presets
Configurable logging presets for different verbosity levels:
- `minimal` - INFO level (high-level operations only)
- `standard` - DEBUG level (filtering decisions, statistics)
- `detailed` - TRACE level (complete execution details)
- `comprehensive` - TRACE + supporting modules
- `full` - All NEEDLE modules at DEBUG/TRACE level
- `maximum` - Everything at TRACE level

### 3. Log Rotation Configuration
- **Maximum file size:** 10MB (configurable via MAX_SIZE_MB)
- **Maximum age:** 7 days (configurable via MAX_AGE_DAYS)
- **Maximum file count:** 50 files (configurable via MAX_LOG_FILES)
- **Rotation strategy:** File renaming with incremental numbers (.1, .2, etc.)

## Usage Examples

### Basic Setup
```bash
# Run with default settings
./pluck-log-redirection.sh -b bf-22ff

# Run with comprehensive logging preset
./pluck-log-redirection.sh -b bf-22ff -p comprehensive

# Run in test-only mode
./pluck-log-redirection.sh --test-only
```

### Log Rotation
```bash
# Run log rotation
/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh

# Dry run to see what would be done
/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh --dry-run

# Custom rotation settings
MAX_SIZE_MB=5 /home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh
```

### Capturing Pluck Execution
```bash
# Example: Run pluck with full logging
RUST_LOG=needle::strand::pluck=trace \
  BEAD_ID=bf-22ff \
  ./pluck-log-redirection.sh -b bf-22ff -p full

# Or capture existing pluck output manually:
br pluck <args> > logs/pluck-debug/pluck-stdout-bf-22ff-$(date +%Y%m%d-%H%M%S).log 2> logs/pluck-debug/pluck-stderr-bf-22ff-$(date +%Y%m%d-%H%M%S).log
```

## Validation Results

All acceptance criteria met:
- ✓ Log file location created and verified (`/home/coding/ARMOR/logs/pluck-debug`)
- ✓ Output redirection syntax validated (stdout/stderr/combined streams)
- ✓ Sample command successfully wrote to log file (test executed and validated)
- ✓ Log rotation configured for long-running processes

## Test Output

The test run on 2026-07-09 04:48:25 produced the following log files:
```
pluck-stdout-manual-20260709-044825.log: 95 bytes, 3 lines
pluck-stderr-manual-20260709-044825.log: 80 bytes, 2 lines
pluck-combined-manual-20260709-044825.log: 175 bytes
pluck-summary-manual-20260709-044825.log: 929 bytes
```

## Architecture Notes

The configuration follows these principles:
1. **Modular design:** Separation of setup, rotation, and validation concerns
2. **Automation-ready:** Environment variable configuration for CI/CD integration
3. **Non-intrusive:** No changes to core Pluck code required
4. **Flexible:** Easy to adapt for different debugging scenarios
5. **Maintainable:** Clear documentation and usage examples

## Files Created/Modified

1. `/home/coding/ARMOR/pluck-log-redirection.sh` - Main configuration script
2. `/home/coding/ARMOR/logs/pluck-debug/log-rotation-config.sh` - Rotation utility
3. `/home/coding/ARMOR/notes/bf-22ff.md` - This documentation

## Conclusion

The Pluck log file output redirection is fully configured and tested. The system is ready for production use and provides comprehensive logging capabilities with automated maintenance.
