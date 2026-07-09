# Pluck Debug Configuration Summary

**Bead:** bf-3b63  
**Task:** Configure Pluck for debug logging output  
**Date:** 2026-07-09  
**Status:** ✅ Complete

## Configuration Created

### 1. Primary Configuration Script
**File:** `pluck-debug-config.sh`

A comprehensive debug configuration manager with preset modes for different logging levels:

- **minimal** - INFO level: High-level strand operations only
- **standard** - DEBUG level: Filtering decisions and statistics (default)
- **detailed** - TRACE level: Complete execution details
- **comprehensive** - TRACE + supporting modules (bead_store, worker)
- **full** - All NEEDLE modules at DEBUG/TRACE level
- **maximum** - Everything at TRACE level (very verbose)

**Usage:**
```bash
./pluck-debug-config.sh [workspace] [output_file] [mode] [count]
```

**Examples:**
```bash
# Standard debug output
./pluck-debug-config.sh /home/coding/ARMOR pluck-debug.log standard 1

# Detailed trace with comprehensive context
./pluck-debug-config.sh /home/coding/ARMOR pluck-detailed.log comprehensive 1

# Minimal high-level output
./pluck-debug-config.sh /home/coding/ARMOR pluck-minimal.log minimal 1
```

### 2. Log Analysis Script
**File:** `analyze-pluck-debug.sh`

Automated analysis tool for captured debug logs that provides:
- Overall statistics (pline counts, filtering operations)
- Pluck strand evaluation details
- Filtering decisions and exclusions
- Candidate information and sorting
- Split decision analysis
- Bead store query details
- Error and warning summaries
- Quick diagnosis of logging effectiveness

**Usage:**
```bash
./analyze-pluck-debug.sh <log_file>
```

**Example:**
```bash
./analyze-pluck-debug.sh pluck-debug-capture-20260709-004000.log
```

### 3. Existing Scripts (Previously Created)

**File:** `capture-pluck-debug.sh`
Original capture script with comprehensive logging preset.

**File:** `execute-pluck-capture.sh`
Enhanced capture script with timeout and analysis.

## Debug Flags Enabled

Based on the comprehensive research from bead bf-5p3g, the following debug flags are now properly configured:

### Primary Module: `needle::strand::pluck`

Controls core Pluck strand operations:
- Evaluation start with configuration parameters
- Label filtering decisions with per-bead details
- Status/assignee filtering operations
- Candidate sorting and priority selection
- Split trigger evaluation and decisions
- Final result reporting

### Supporting Modules

**`needle::strand`** - General strand operations  
**`needle::bead_store`** - Bead store queries and operations  
**`needle::worker`** - Worker state machine and processing  
**`needle::dispatch`** - Agent dispatch and execution  
**`needle::claim`** - Bead claiming operations

## Expected Debug Output

When using standard or higher debug modes, the following output is captured:

### 1. Strand Evaluation
```
DEBUG Pluck strand evaluation starting
  exclude_labels=[...] split_threshold=N
```

### 2. Bead Store Queries
```
DEBUG Querying bead store for ready candidates
  filters=...
DEBUG Bead store returned N candidates
```

### 3. Label Filtering
```
DEBUG Filtering N candidates by labels
DEBUG Excluding bead_id=... reason=label:label_name
DEBUG excluded_count=M remaining=N
```

### 4. Status/Assignee Filtering
```
DEBUG Filtering by status and assignee
DEBUG Excluding bead_id=... reason=in_progress
```

### 5. Candidate Sorting
```
DEBUG Sorting N candidates by priority, created_at, id
DEBUG First candidate: bead_id=... priority=N
```

### 6. Split Decisions
```
DEBUG Checking split trigger failures=N threshold=M
DEBUG Split triggered: bead_id=...
```

### 7. Final Results
```
DEBUG Result: NoWork (no candidates)
DEBUG Result: BeadFound(bead_id=...)
DEBUG Result: Split(bead_id=...)
```

## Configuration Options

### Quick Start Commands

**Standard filtering debug:**
```bash
RUST_LOG="needle::strand::pluck=debug" needle run -w /home/coding/ARMOR -c 1
```

**Comprehensive debug with context:**
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug" needle run -w /home/coding/ARMOR -c 1
```

**Using the configuration script (recommended):**
```bash
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive 1
```

## Log Output Destination

Debug logs are captured to timestamped files in the workspace root:
- `pluck-debug-capture-YYYYMMDD-HHMMSS.log` - Full comprehensive capture
- `pluck-debug-YYYYMMDD-HHMMSS.log` - Standard debug output
- Custom filenames via script parameters

## Verification

To verify debug logging is working:

1. **Check RUST_LOG is set:**
   ```bash
   echo $RUST_LOG
   ```

2. **Run with verbose mode:**
   ```bash
   ./pluck-debug-config.sh /home/coding/ARMOR test.log standard 1
   ```

3. **Analyze output:**
   ```bash
   ./analyze-pluck-debug.sh test.log
   ```

4. **Look for key indicators:**
   - "Pluck strand evaluation starting"
   - "Filtering...candidates"
   - "Excluding bead_id..."
   - "Result:..."

## Integration with NEEDLE Configuration

The `.needle.yaml` file references this debug configuration:

```yaml
strands:
  pluck:
    exclude_labels: []
    split_after_failures: 0

# Note: Debug logging is controlled via RUST_LOG environment variable
# Use pluck-debug-config.sh for easy configuration
# See docs/pluck-debug-configuration.md for detailed options
```

## Files Created/Modified

1. ✅ `pluck-debug-config.sh` - New comprehensive configuration script
2. ✅ `analyze-pluck-debug.sh` - New log analysis script
3. ✅ `notes/bf-3b63-pluck-debug-configuration-summary.md` - This summary
4. ℹ️ `docs/pluck-debug-configuration.md` - Comprehensive documentation (existing)
5. ℹ️ `.needle.yaml` - Configuration file with debug reference (existing)

## Acceptance Criteria Status

- ✅ **Debug logging configuration created** - Comprehensive script with 6 preset modes
- ✅ **Flags for filtering decision logging are enabled** - All RUST_LOG configurations properly set
- ✅ **Configuration ready for execution** - Scripts tested and documented

## Next Steps

To use the debug configuration:

1. Run capture with desired mode:
   ```bash
   ./pluck-debug-config.sh /home/coding/ARMOR debug-output.log comprehensive 1
   ```

2. Analyze the captured output:
   ```bash
   ./analyze-pluck-debug.sh debug-output.log
   ```

3. Examine specific filtering decisions:
   ```bash
   grep -i "filter\|exclude" debug-output.log
   ```

## Related Documentation

- **Comprehensive Guide:** `docs/pluck-debug-configuration.md`
- **Previous Research:** `docs/bf-5p3g-pluck-debug-logging.md`
- **Analysis Script:** `analyze-pluck-debug.sh`
- **Configuration Script:** `pluck-debug-config.sh`

---

**Status:** ✅ Complete - All acceptance criteria met, configuration ready for execution