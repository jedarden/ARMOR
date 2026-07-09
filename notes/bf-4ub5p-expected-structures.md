# Debug Configuration File Structure Definitions

**Bead:** bf-4ub5p  
**Date:** 2026-07-09  
**Task:** Define expected structure for debug configuration files

## Overview

This document defines the expected structure and schema for all debug configuration files in the ARMOR workspace.

---

## 1. pluck-config.yaml - Primary Debug Configuration

### File Type
YAML configuration file for Pluck strand debug logging and filtering behavior

### Expected Top-Level Sections

```yaml
debug:           # Required - Debug logging configuration
modules:         # Required - Complementary debug modules
filtering:       # Required - Filtering configuration  
output:          # Required - Log output configuration
```

### Section: `debug`
**Required:** Yes  
**Description:** Controls debug logging level and detailed feature toggles

**Expected Keys:**
- `level` (string, required): Debug logging level
  - Allowed values: `info`, `debug`, `trace`, `off`
  - Default: `info`
  
- `log_filtering_decisions` (boolean, required): Enable detailed filtering decision logging
  - Default: `false`
  
- `log_bead_store_queries` (boolean, required): Enable bead store query logging
  - Default: `false`
  
- `log_split_evaluation` (boolean, required): Enable split threshold evaluation logging
  - Default: `false`

**Structure:**
```yaml
debug:
  level: <string>
  log_filtering_decisions: <boolean>
  log_bead_store_queries: <boolean>
  log_split_evaluation: <boolean>
```

### Section: `modules`
**Required:** Yes  
**Description:** Complementary debug modules that provide additional context

**Expected Keys:**
- `strand` (boolean, required): Enable strand-level debug logging
- `worker` (boolean, required): Enable worker coordination debug logging
- `bead_store` (boolean, required): Enable bead store access debug logging
- `dispatch` (boolean, required): Enable dispatch coordination debug logging
- `claim` (boolean, required): Enable claim process debug logging

**Structure:**
```yaml
modules:
  strand: <boolean>
  worker: <boolean>
  bead_store: <boolean>
  dispatch: <boolean>
  claim: <boolean>
```

### Section: `filtering`
**Required:** Yes  
**Description:** Controls bead filtering and selection behavior

**Expected Keys:**
- `exclude_labels` (array of strings, required): Labels to exclude when selecting beads
  - Default: `[]` (empty array)
  
- `split_after_failures` (integer, required): Auto-split beads after N consecutive failures
  - Default: `0` (disabled)
  - Must be >= 0
  
- `sort_order` (string, required): Priority order for candidate selection
  - Allowed values: `created`, `updated`, `priority`, `random`
  - Default: `priority`

**Structure:**
```yaml
filtering:
  exclude_labels: <array of strings>
  split_after_failures: <integer >= 0>
  sort_order: <string>
```

### Section: `output`
**Required:** Yes  
**Description:** Log output configuration

**Expected Keys:**
- `file` (string, required): Log file location (relative to workspace root)
  - Default: `""` (stdout only)
  
- `timestamps` (boolean, required): Include timestamp in log output
  - Default: `true`
  
- `source_location` (boolean, required): Include module/function in log output
  - Default: `true`
  
- `colorize` (boolean, required): Colorize console output
  - Default: `true`
  
- `max_size_mb` (integer, required): Maximum log file size before rotation (in MB)
  - Default: `0` (no rotation)
  - Must be >= 0
  
- `max_backups` (integer, required): Maximum number of rotated log files to keep
  - Default: `5`
  - Must be >= 0

**Structure:**
```yaml
output:
  file: <string>
  timestamps: <boolean>
  source_location: <boolean>
  colorize: <boolean>
  max_size_mb: <integer >= 0>
  max_backups: <integer >= 0>
```

---

## 2. .needle.yaml - NEEDLE Strand Configuration

### File Type
YAML configuration file for NEEDLE strand behavior

### Expected Top-Level Sections

```yaml
strands:         # Required - Strand configurations
```

### Section: `strands`
**Required:** Yes  
**Description:** Top-level container for all strand configurations

**Expected Sub-Sections:**
- `pluck` (object, required): Pluck strand configuration

**Structure:**
```yaml
strands:
  pluck:
    exclude_labels: <array of strings>
    split_after_failures: <integer >= 0>
```

### Sub-Section: `strands.pluck`
**Required:** Yes  
**Description:** Controls bead selection and filtering for Pluck strand

**Expected Keys:**
- `exclude_labels` (array of strings, required): Labels to exclude when selecting beads
  - Default: `[]` (empty array)
  
- `split_after_failures` (integer, required): Auto-split beads after N consecutive failures
  - Default: `0` (disabled)
  - Must be >= 0

---

## 3. .env.pluck-debug - Environment Configuration

### File Type
Shell environment variable configuration file

### Expected Structure

**Required Components:**
- Shell comment header (lines starting with `#`)
- `export` statements for `RUST_LOG` environment variable
- Usage documentation in comments

**Expected Export Statements:**
At least one active `export RUST_LOG=...` statement (not commented out)

**Typical RUST_LOG Patterns:**
```bash
export RUST_LOG=needle::strand::pluck=<level>
export RUST_LOG=needle::strand::pluck=<level>,needle::strand=<level>,...
```

**Allowed Log Levels:**
- `error`
- `warn`
- `info`
- `debug`
- `trace`
- `off`

**Common Module Paths:**
- `needle::strand::pluck`
- `needle::strand`
- `needle::bead_store`
- `needle::worker`
- `needle::dispatch`
- `needle::claim`

---

## 4. pluck-debug-config.sh - Debug Configuration Script

### File Type
Executable Bash shell script

### Expected Components

**Required:**
1. **Shebang line** (first line): `#!/bin/bash`
2. **Error handling**: `set -e` or equivalent
3. **Color code definitions** (optional but recommended):
   - `RED`, `GREEN`, `YELLOW`, `BLUE`, `NC` variables
4. **Parameter handling**:
   - `WORKSPACE` variable (default: `/home/coding/ARMOR`)
   - `OUTPUT` variable (default: timestamped log file)
   - `MODE` variable (default: `standard`)
   - `COUNT` variable (default: `1`)
5. **Configuration presets**:
   - Associative array named `PRESETS` with at least these keys:
     - `minimal`
     - `standard`
     - `detailed`
     - `comprehensive`
     - `full`
6. **Functions**:
   - `show_usage()`: Display usage information
   - `show_configuration()`: Display current configuration
   - `run_debug_capture()`: Execute the debug capture
7. **Help flag handling**: Check for `-h` or `--help`
8. **Validation**: Mode validation, workspace existence check
9. **Execution**: Call to `run_debug_capture()`

**Structure:**
```bash
#!/bin/bash
# Comments describing purpose

set -e

# Color definitions
RED='...'
GREEN='...'
...

# Parameter variables
WORKSPACE="${1:-/home/coding/ARMOR}"
OUTPUT="${2:-...}"
MODE="${3:-standard}"
COUNT="${4:-1}"

# Configuration presets
declare -A PRESETS=(
    ["minimal"]="..."
    ["standard"]="..."
    ...
)

# Functions
show_usage() { ... }
show_configuration() { ... }
run_debug_capture() { ... }

# Help flag check
if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    show_usage
    exit 0
fi

# Validation
if [[ -z "${PRESETS[$MODE]}" ]]; then
    echo "Error: Invalid mode"
    exit 1
fi

# Run
run_debug_capture "$MODE"
```

---

## 5. capture-pluck-debug.sh - Debug Capture Script

### File Type
Executable Bash shell script

### Expected Components

**Required:**
1. **Shebang line** (first line): `#!/bin/bash`
2. **Error handling**: `set -e`
3. **Parameter variables**:
   - `WORKSPACE` (default: `/home/coding/ARMOR`)
   - `OUTPUT_FILE` (default: timestamped log file)
   - `COUNT` (default: `1`)
4. **RUST_LOG configuration**: Hardcoded comprehensive logging setting
5. **Execution**: Call to `needle run` with output capture via `tee`
6. **Summary output**: Display completion message and analysis suggestions

**Structure:**
```bash
#!/bin/bash
# Comments describing purpose

set -e

WORKSPACE="${1:-/home/coding/ARMOR}"
OUTPUT_FILE="${2:-...}"
COUNT="${3:-1}"

# Display configuration
echo "=== Pluck Filtering Debug Capture ==="

# Set RUST_LOG
export RUST_LOG="...comprehensive setting..."

# Run NEEDLE with output capture
RUST_LOG="$RUST_LOG" needle run -w "$WORKSPACE" -c "$COUNT" 2>&1 | tee "$OUTPUT_FILE"

# Completion message
echo "Capture complete..."
```

---

## 6. analyze-pluck-debug.sh - Debug Analysis Script

### File Type
Executable Bash shell script

### Expected Components

**Required:**
1. **Shebang line** (first line): `#!/bin/bash`
2. **Parameter handling**: Accept log file path as argument
3. **Analysis functions**: Functions to parse and analyze debug output
4. **Summary output**: Display analysis results

**Common Analysis Patterns:**
- Filter decision counting
- Candidate evaluation statistics
- Bead store interaction tracking
- Split evaluation analysis

---

## 7. .beads/config.yaml - Bead Forge Configuration

### File Type
Bead Forge CLI configuration file

### Expected Structure

This is a standard Bead Forge configuration file. Expected sections include:
- Project/workspace settings
- Bead store configuration
- Output settings

**Note:** Structure depends on Bead Forge version and configuration options.

---

## Validation Criteria Summary

### Critical Structure Requirements

1. **YAML files must have:**
   - All required top-level sections
   - All required keys within each section
   - Proper data types (string, boolean, integer, array)
   - Valid enum values where specified

2. **Shell scripts must have:**
   - Proper shebang line
   - Required variables defined
   - Required functions present
   - Proper validation logic

3. **Environment files must have:**
   - At least one active `export RUST_LOG=...` statement
   - Valid module paths and log levels

### Data Type Requirements

- **Strings**: Non-empty string values
- **Booleans**: `true` or `false` (YAML boolean values)
- **Integers**: Numeric values, >= 0 for counts/sizes
- **Arrays**: YAML array format, even if empty

### Enum Value Requirements

- **debug.level**: `info`, `debug`, `trace`, `off`
- **filtering.sort_order**: `created`, `updated`, `priority`, `random`
- **RUST_LOG levels**: `error`, `warn`, `info`, `debug`, `trace`, `off`

---

**Structure Definitions Completed:** 2026-07-09  
**Status:** ✅ COMPLETE  
**Next:** Validate each configuration file against these structure definitions
