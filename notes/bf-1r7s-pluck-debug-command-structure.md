# Pluck Debug Command Structure - Complete Reference

**Bead ID:** bf-1r7s  
**Date:** 2026-07-09  
**Task:** Research and document Pluck debug command structure with all required debug flags  
**Status:** ✅ COMPLETED

## Executive Summary

Pluck is NEEDLE's primary bead selection strand, handling >90% of all bead processing. All debug logging is controlled **exclusively via the `RUST_LOG` environment variable** - there are NO CLI debug flags. The complete command structure combines `RUST_LOG` configuration with standard `needle run` CLI options.

## Complete Command Template

### Basic Structure
```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count>
```

### Full Command with All Options
```bash
RUST_LOG=<debug_level> needle run -w <workspace> -c <count> -a <agent> -i <identifier> -t <timeout> [--resume] [--hot-reload <true|false>]
```

## Primary Debug Control: RUST_LOG Environment Variable

### Syntax Format
```bash
RUST_LOG=<module_path>=<level>[,<module_path>=<level>]
```

### Available Log Levels
| Level | Purpose | Output Detail |
|-------|---------|--------------|
| `error` | Critical failures | Errors only |
| `warn` | Warnings | Warnings and errors |
| `info` | High-level operations | Strand lifecycle events |
| `debug` | Detailed operations (recommended) | Filtering decisions and statistics |
| `trace` | Complete execution flow | All operations including per-item details |

### Pluck-Specific Module Paths

#### Core Module
```
needle::strand::pluck        # Primary Pluck strand logic
```

#### Supporting Modules
```
needle::strand              # General strand operations
needle::bead_store          # Bead database operations  
needle::worker              # Worker lifecycle management
needle::dispatch            # Bead dispatch logic
needle::claim               # Bead claiming logic
needle::telemetry           # Telemetry event logging
needle::sanitize            # Secret scanning and sanitization
```

## Six Complete Command Configurations

### 1. Minimal Logging (INFO level)
```bash
RUST_LOG=needle::strand::pluck=info needle run -w /home/coding/ARMOR -c 1
```
**Purpose:** Quick health checks, basic operation verification  
**Output:** High-level strand operations only

### 2. Standard Debug (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1
```
**Purpose:** Normal debugging, understanding filtering behavior  
**Output:** Filtering decisions and statistics

### 3. Detailed Trace
```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1
```
**Purpose:** Deep troubleshooting, understanding exact flow  
**Output:** Complete execution details including per-bead decisions

### 4. Comprehensive Multi-Module
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug" needle run -w /home/coding/ARMOR -c 1
```
**Purpose:** Full context debugging, understanding system interactions  
**Output:** Pluck TRACE + supporting modules DEBUG

### 5. Full System Debug
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug" needle run -w /home/coding/ARMOR -c 1
```
**Purpose:** Complete system debugging  
**Output:** All critical NEEDLE modules at DEBUG/TRACE

### 6. Maximum Verbosity (Global TRACE)
```bash
RUST_LOG=trace needle run -w /home/coding/ARMOR -c 1
```
**Purpose:** Deep system-level debugging  
**Output:** Everything at TRACE level (very verbose)

## Commands with Output Capture

### Standard Debug with Log File
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

### Detailed Trace with Timestamped Log
```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-$(date +%Y%m%d-%H%M%S).log
```

### Comprehensive Multi-Module with Backtrace Support
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug" RUST_BACKTRACE=1 needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-comprehensive.log
```

### Production Execution with Timeout
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" timeout 180s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-$(date +%Y%m%d-%H%M%S).log
```

## needle run CLI Options

### Required Options
```bash
-w, --workspace <WORKSPACE>    # Path to workspace directory
-c, --count <COUNT>            # Number of workers to launch [default: 1]
```

### Optional Options
```bash
-a, --agent <AGENT>            # Agent adapter to use
-i, --identifier <IDENTIFIER>  # Worker identifier (overrides NATO naming)
-t, --timeout <TIMEOUT>        # Agent execution timeout in seconds
--resume                       # Resume an existing worker session
--hot-reload <true|false>      # Enable hot-reload for this worker
```

## Secondary Debug Environment Variables

### RUST_BACKTRACE
```bash
RUST_BACKTRACE=1    # Enable full backtraces on errors
RUST_BACKTRACE=0    # Disable backtraces (default)
```

**Usage Example:**
```bash
RUST_LOG=needle::strand::pluck=debug RUST_BACKTRACE=1 needle run -w /home/coding/ARMOR -c 1
```

## Expected Debug Output Events

When Pluck debug logging is enabled at `debug` or `trace` level, you should see:

### 1. Strand Evaluation Start
```
DEBUG needle::strand::pluck: Pluck strand evaluation starting
  exclude_labels=["deferred", "human", "blocked"]
  split_threshold=3
```

### 2. Bead Store Query
```
DEBUG needle::strand::pluck: Querying bead store for ready candidates
  filters=Filters { 
    assignee: None, 
    exclude_labels: ["deferred", "human", "blocked"] 
  }
```

### 3. Query Results
```
DEBUG needle::strand::pluck: Bead store returned N candidates
  count=5
```

### 4. Label Filtering
```
DEBUG needle::strand::pluck: Filtering by excluded labels
  excluded_beads=["bf-1234", "bf-5678"]
  reasons=["label:deferred", "label:blocked"]
```

### 5. Status/Assignee Filtering
```
DEBUG needle::strand::pluck: Filtering by status and assignee
  remaining=3
```

### 6. Sorting
```
DEBUG needle::strand::pluck: Sorting candidates by priority
  first_candidate="bf-abcd"
```

### 7. Split Decision
```
DEBUG needle::strand::pluck: Checking split threshold
  failure_count=2
  split_threshold=3
  should_split=false
```

### 8. Final Result
```
DEBUG needle::strand::pluck: Strand evaluation complete
  result=BeadFound("bf-abcd")
```

## Automated Configuration Scripts

### Script Location
```bash
/home/coding/ARMOR/pluck-debug-config.sh
```

### Usage Examples
```bash
# Standard debug (recommended)
./pluck-debug-config.sh /home/coding/ARMOR output.log standard

# Detailed trace
./pluck-debug-config.sh /home/coding/ARMOR output.log detailed

# Comprehensive multi-module
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive

# Full system debug with 3 workers
./pluck-debug-config.sh /home/coding/ARMOR output.log full 3
```

### Available Presets
- `minimal` - INFO level
- `standard` - DEBUG level (recommended)
- `detailed` - TRACE level
- `comprehensive` - TRACE + supporting modules
- `full` - All NEEDLE modules DEBUG/TRACE
- `maximum` - Global TRACE

## Log Analysis Commands

### Filter Specific Events
```bash
# All Pluck events
grep -i "pluck" output.log

# Filtering decisions
grep -i "filter" output.log

# Excluded beads
grep -i "exclude" output.log

# Candidate processing
grep -i "candidate" output.log

# Split decisions
grep -i "split" output.log

# Strand evaluation lifecycle
grep -i "evaluation" output.log
```

### Count Event Types
```bash
grep -c "Pluck strand evaluation starting" output.log
grep -c "result=BeadFound" output.log
grep -c "result=NoWork" output.log
grep -c "result=Split" output.log
```

### Multi-line Context
```bash
# Show 5 lines after each filter event
grep -A 5 "Filtering by" output.log

# Show context around split decisions
grep -B 3 -A 3 "split threshold" output.log
```

## Quick Reference Table

| Debug Level | RUST_LOG Value | Verbosity | Use Case |
|------------|----------------|-----------|----------|
| Minimal | `needle::strand::pluck=info` | Low | Health checks |
| Standard | `needle::strand::pluck=debug` | Medium | Normal debugging (recommended) |
| Detailed | `needle::strand::pluck=trace` | High | Deep troubleshooting |
| Comprehensive | `needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug` | High | System context |
| Full | All NEEDLE modules at DEBUG/TRACE | Very High | Complete system debug |
| Maximum | `trace` | Extreme | Deep system analysis |

## Complete Example Session

### Session Setup
```bash
# 1. Navigate to workspace
cd /home/coding/ARMOR

# 2. Create log directory
mkdir -p logs/pluck-debug

# 3. Run with standard debug level
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee logs/pluck-debug/pluck_debug_$(date +%Y%m%d_%H%M%S).log

# 4. Analyze the output
grep -i "pluck" logs/pluck-debug/pluck_debug_*.log
grep -A 5 "Filtering by" logs/pluck-debug/pluck_debug_*.log
```

### Using the Automated Script
```bash
# Quick start with recommended settings
cd /home/coding/ARMOR
./pluck-debug-config.sh /home/coding/ARMOR logs/pluck-debug/output.log standard

# Comprehensive debugging
./pluck-debug-config.sh /home/coding/ARMOR logs/pluck-debug/comprehensive.log comprehensive
```

## Troubleshooting

### No Pluck Output Visible
1. **Check RUST_LOG is set correctly:**
   ```bash
   echo $RUST_LOG
   ```

2. **Verify Pluck strand is active:**
   ```bash
   grep "worker booted" output.log | grep "pluck"
   ```

3. **Ensure beads are available for processing:**
   ```bash
   br list --status=open
   ```

### Bead Store Query Failed
**Error:** `ERROR needle::strand::pluck: Bead store query failed`

**Possible causes:**
- Bead store locked by another process
- Corrupted bead database  
- Permission issues

**Resolution:**
```bash
cd /home/coding/ARMOR
br doctor --repair
```

## Acceptance Criteria Verification

✅ **Complete Pluck command documented with all debug flags**
- All 6 debug configurations documented
- Environment variables explained
- CLI options detailed

✅ **Command syntax verified against Pluck documentation**
- Verified against existing validation reports
- Cross-referenced with multiple source documents
- Tested against actual Pluck execution logs

✅ **Command ready for execution**
- Production-ready command templates provided
- Automated scripts available
- Expected output documented

## Version Information

- **NEEDLE Version:** 0.2.11 (as of 2026-07-09)
- **Tracing Crate:** Standard Rust `tracing` ecosystem
- **Log Levels:** error, warn, info, debug, trace

## Related Documentation

- **Complete Command Reference:** `/home/coding/ARMOR/notes/bf-7423-pluck-debug-command-reference.md`
- **Quick Reference:** `/home/coding/ARMOR/notes/bf-7423-pluck-debug-quick-reference.md`
- **Complete Guide:** `/home/coding/ARMOR/notes/bf-5p3g-pluck-debug-complete-guide.md`
- **Debug Flags:** `/home/coding/ARMOR/notes/bf-5p3g-pluck-debug-flags.md`
- **Validation Report:** `/home/coding/ARMOR/notes/bf-t5my-pluck-syntax-validation-summary.md`

## Status

**COMPLETE** - All Pluck debug commands constructed and documented
- 6 complete command configurations provided
- Automated script usage documented
- Expected output and analysis commands included
- Ready for immediate execution

---

**Research Completed:** 2026-07-09  
**Bead ID:** bf-1r7s  
**Status:** ✅ COMPLETED
