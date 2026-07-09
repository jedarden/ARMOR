# Pluck Debug Flags and Logging Configuration - Complete Summary

**Bead:** bf-5p3g  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**NEEDLE Project:** /home/coding/NEEDLE

## Executive Summary

Pluck is a NEEDLE strand that selects beads for processing. Debug logging is controlled via Rust's standard `RUST_LOG` environment variable using the `tracing` crate. **No Pluck-specific CLI flags exist** - all debugging is controlled through environment variables.

## Primary Debug Configuration

### Environment Variable: `RUST_LOG`

**Purpose:** Controls log level for Rust modules in the NEEDLE binary

**Format:** `RUST_LOG=<module_path>=<log_level>[,<module_path>=<log_level>]`

**Available Log Levels:**
| Level | Usage | Verbosity |
|-------|-------|-----------|
| `error` | Only errors (default) | Minimal |
| `warn` | Warnings and errors | Low |
| `info` | Informational messages | Medium |
| `debug` | Detailed debugging | High |
| `trace` | Maximum detail | Maximum |

## Pluck-Specific Module Paths

### Primary Module
```
needle::strand::pluck
```
**Controls:** Core Pluck strand evaluation, filtering decisions, candidate selection

### Related Modules
```
needle::strand          # All strand implementations
needle::worker          # Worker state machine
needle::bead_store      # Bead storage operations
needle::dispatch        # Task dispatching
```

## Recommended Debug Configurations

### 1. Minimal Pluck Debug Output
```bash
export RUST_LOG=needle::strand::pluck=debug
```
**Shows:** Filtering decisions, candidate counts, exclusion reasons

### 2. Comprehensive Pluck Trace
```bash
export RUST_LOG=needle::strand::pluck=trace
```
**Shows:** Extremely detailed execution flow, function entry/exit, all variables

### 3. Full Strand Context
```bash
export RUST_LOG=needle::strand=debug,needle::strand::pluck=trace
```
**Shows:** All strand activity with detailed Pluck trace

### 4. Complete Worker Context
```bash
export RUST_LOG=needle::strand::pluck=trace,needle::worker=debug,needle::bead_store=debug,needle::dispatch=debug
```
**Shows:** Pluck details + worker coordination + storage operations

### 5. Maximum Debug Output (Not Recommended)
```bash
export RUST_LOG=debug
```
**Shows:** All modules at debug level (very verbose)

## Expected Pluck Debug Output Messages

When `RUST_LOG=needle::strand::pluck=debug` or higher is set, these messages appear:

### 1. Evaluation Start
```
[timestamp] DEBUG strand.pluck{...}: Pluck strand evaluation starting exclude_labels=[...] split_threshold=N
```
**Shows:** Configuration values for this evaluation cycle

### 2. Bead Store Query
```
[timestamp] DEBUG needle::strand::pluck: Querying bead store for ready candidates filters=...
```
**Shows:** What filters are being passed to the bead store

### 3. Candidate Count
```
[timestamp] DEBUG needle::strand::pluck: Bead store returned N candidates
```
**Shows:** How many beads passed the initial ready() filter

### 4. Label Filtering
```
[timestamp] DEBUG needle::strand::pluck: Filtering N candidates by labels
[timestamp] DEBUG needle::strand::pluck: Excluding bead_id=... reason=label:label_name
```
**Shows:** Which beads are excluded by label and why

### 5. Status/Assignee Filtering
```
[timestamp] DEBUG needle::strand::pluck: Filtering by status and assignee
[timestamp] DEBUG needle::strand::pluck: Excluding bead_id=... reason=in_progress
```
**Shows:** Beads excluded due to status or assignee conflicts

### 6. Sorting Results
```
[timestamp] DEBUG needle::strand::pluck: Sorting N candidates by priority, created_at, id
[timestamp] DEBUG needle::strand::pluck: First candidate: bead_id=... priority=N
```
**Shows:** How candidates are sorted and which is first

### 7. Split Decision
```
[timestamp] DEBUG needle::strand::pluck: Checking split trigger failures=N threshold=M
[timestamp] DEBUG needle::strand::pluck: Split triggered: bead_id=...
```
**Shows:** Whether bead splitting is triggered and why

### 8. Final Result
```
[timestamp] DEBUG needle::strand::pluck: Result: NoWork (no candidates)
[timestamp] DEBUG needle::strand::pluck: Result: BeadFound(bead_id=...)
[timestamp] DEBUG needle::strand::pluck: Result: Split(bead_id=...)
```
**Shows:** Final outcome of the evaluation

## Usage Examples

### Running NEEDLE with Pluck Debug Output
```bash
# Basic debug
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1

# With output capture
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log

# Comprehensive debug
RUST_LOG=needle::strand::pluck=trace,needle::worker=debug,needle::bead_store=debug needle run -w /home/coding/ARMOR -c 1
```

### Using the Capture Script
```bash
./capture-pluck-debug.sh /home/coding/ARMOR pluck-debug.log 1
```
**Runs:** NEEDLE with comprehensive Pluck debug settings and captures output

## Filtering-Related Debug Options

### Core Filtering Controls

1. **Label Exclusion** (`exclude_labels`)
   - Default: `["deferred", "human", "blocked"]`
   - Config location: `.needle.yaml` → `strands.pluck.exclude_labels`
   - Debug message: Shows which beads are excluded and why
   - Current setting: `[]` (empty, no exclusions)

2. **Status Filtering** (hardcoded in `pluck.rs`)
   - Excludes: `in_progress` status, Open beads with stale assignee
   - Cannot be configured (always applied)
   - Debug message: Shows status-based exclusions

3. **Dependency Filtering** (via `store.ready()`)
   - Excludes: Beads with unclosed dependencies
   - Cannot be configured (always applied)
   - Debug message: Visible in bead store query phase

4. **Auto-Split Decision** (`split_after_failures`)
   - Config location: `.needle.yaml` → `strands.pluck.split_after_failures`
   - Current: `0` (disabled)
   - Debug message: Shows split trigger evaluation

### Current ARMOR Workspace Configuration
From `/home/coding/ARMOR/.needle.yaml`:
```yaml
strands:
  pluck:
    exclude_labels: []           # No label-based exclusions
    split_after_failures: 0      # Auto-split disabled
```

## Configuration Debugging

To debug why specific beads are or aren't being selected:

```bash
# Enable comprehensive debug
RUST_LOG=needle::strand::pluck=trace,needle::bead_store=debug

# Run NEEDLE and capture output
needle run -w /path/to/workspace -c 1 2>&1 | tee pluck-filter-debug.log

# Search for specific bead IDs
grep "bead_id=bf-XXXX" pluck-filter-debug.log

# Search for filtering decisions
grep -i "exclude\|filter" pluck-filter-debug.log
```

## Troubleshooting

### Debug Output Not Appearing

**Symptom:** RUST_LOG is set but no Pluck debug messages appear

**Possible Causes:**

1. **Tracing Subscriber Not Initialized**
   - Check for: `NEEDLE worker boot: tracing subscriber initialized`
   - If missing, NEEDLE may have failed to boot properly

2. **Incorrect Module Path**
   - Verify: `needle::strand::pluck` (exact spelling and colons)
   - Common mistake: `needle::strands::pluck` (extra 's')

3. **Worker Claims Immediately**
   - If worker has a claimed bead, it won't run Pluck
   - Release claimed beads: `br release <bead_id>`

4. **No Open Beads Available**
   - Pluck only runs when there are unclaimed, ready beads
   - Check: `br list --status open`

5. **Log Level Too Low**
   - Use `trace` for maximum detail
   - Or use `RUST_LOG=trace` to see everything

### Verifying RUST_LOG is Set
```bash
# Before running NEEDLE
echo "RUST_LOG=$RUST_LOG"

# Within NEEDLE output (should appear early)
grep "RUST_LOG" pluck-debug.log
```

## Available Files and Tools

### Documentation
- `/home/coding/ARMOR/docs/pluck-debug-configuration.md` - Comprehensive debug guide
- `/home/coding/ARMOR/.needle.yaml` - Pluck strand configuration

### Scripts
- `/home/coding/ARMOR/capture-pluck-debug.sh` - Debug capture automation

### Source Code (NEEDLE project)
- `/home/coding/NEEDLE/src/strand/pluck.rs` - Pluck strand implementation
- `/home/coding/NEEDLE/.needle.yaml` - Default NEEDLE configuration

## Summary Table

| Configuration | Use Case | Output Level |
|---------------|----------|-------------|
| `RUST_LOG=needle::strand::pluck=debug` | Standard Pluck debugging | Filtering decisions, counts |
| `RUST_LOG=needle::strand::pluck=trace` | Detailed execution trace | All variables, function calls |
| `RUST_LOG=needle::strand=debug,needle::strand::pluck=trace` | Full strand context | All strands + detailed Pluck |
| `RUST_LOG=needle::strand::pluck=trace,needle::worker=debug,needle::bead_store=debug` | Complete worker context | Pluck + coordination + storage |
| `RUST_LOG=debug` | Maximum output (not recommended) | All modules at debug level |

## Key Points

1. **No Pluck-specific CLI flags** - All debugging controlled via `RUST_LOG` environment variable
2. **Module path is `needle::strand::pluck`** - Exact path required for filtering decisions
3. **Current config: No label exclusions** - `exclude_labels: []` in `.needle.yaml`
4. **Auto-split disabled** - `split_after_failures: 0`
5. **Use `trace` for maximum detail** - If `debug` doesn't show enough, upgrade to `trace`
6. **Capture output for analysis** - Use `tee` or the capture script

## ARMOR Environment Variables (Not Pluck-Specific)

From `/home/coding/ARMOR/internal/config/config.go`:

```bash
# Server configuration
ARMOR_LISTEN, ARMOR_ADMIN_LISTEN

# B2 backend configuration  
ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID
ARMOR_B2_SECRET_ACCESS_KEY, ARMOR_BUCKET, ARMOR_PREFIX, ARMOR_CF_DOMAIN

# Encryption configuration
ARMOR_MEK, ARMOR_BLOCK_SIZE

# Authentication
ARMOR_AUTH_ACCESS_KEY, ARMOR_AUTH_SECRET_KEY

# Debugging/Features
ARMOR_CANARY_DISABLED  # Set to "true" to disable canary checks

# Multi-key support
ARMOR_MEK_<NAME>       # Pattern for named keys
ARMOR_KEY_ROUTES       # Prefix to key name mappings
```

**Note:** These ARMOR environment variables are separate from NEEDLE/Pluck's `RUST_LOG` configuration.

## Related Beads and Documentation

- **Pluck Configuration Review:** `/home/coding/ARMOR/notes/bf-1hm4.md`
- **Complete Documentation:** `/home/coding/ARMOR/docs/pluck-debug-configuration.md`
- **NEEDLE Project:** `/home/coding/NEEDLE/`
- **Bead-forge CLI:** `br --help`
