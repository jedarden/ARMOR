# Pluck Debug Command Quick Reference

**Bead:** bf-7423  
**Date:** 2026-07-09

## Six Essential Commands

### 1. Standard Debug (Recommended)
```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1
```

### 2. Detailed Trace
```bash
RUST_LOG=needle::strand::pluck=trace needle run -w /home/coding/ARMOR -c 1
```

### 3. Comprehensive Multi-Module
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug" needle run -w /home/coding/ARMOR -c 1
```

### 4. Full System Debug
```bash
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug" needle run -w /home/coding/ARMOR -c 1
```

### 5. Maximum Verbosity
```bash
RUST_LOG=trace needle run -w /home/coding/ARMOR -c 1
```

### 6. Minimal Logging
```bash
RUST_LOG=needle::strand::pluck=info needle run -w /home/coding/ARMOR -c 1
```

## Command with Output Capture

```bash
RUST_LOG=needle::strand::pluck=debug needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug.log
```

## Automated Script

```bash
# Standard debug
./pluck-debug-config.sh /home/coding/ARMOR output.log standard

# Comprehensive debug  
./pluck-debug-config.sh /home/coding/ARMOR output.log comprehensive

# Full system debug
./pluck-debug-config.sh /home/coding/ARMOR output.log full
```

## Analysis Commands

```bash
# All Pluck events
grep -i "pluck" output.log

# Filtering decisions
grep -i "filter" output.log

# Candidate processing
grep -i "candidate" output.log

# Split decisions
grep -i "split" output.log
```

## Debug Levels

| Level | Purpose | Output |
|-------|---------|--------|
| info | Health checks | High-level operations |
| debug | Normal debugging (recommended) | Filtering decisions |
| trace | Deep troubleshooting | Complete execution flow |
| comprehensive | System context | Pluck TRACE + supporting modules |
| full | Complete system | All NEEDLE modules |
| maximum | Deep analysis | Everything at TRACE |

## CLI Options

```
-w, --workspace <WORKSPACE>    # Workspace path (required)
-c, --count <COUNT>            # Worker count (required)
-a, --agent <AGENT>            # Agent adapter
-i, --identifier <IDENTIFIER>  # Worker identifier
-t, --timeout <TIMEOUT>        # Timeout in seconds
--resume                       # Resume existing session
--hot-reload <true|false>      # Enable hot-reload
```

## Environment Variables

```
RUST_LOG=<module>=<level>     # Primary debug control
RUST_BACKTRACE=1               # Enable error stack traces
```

## Module Paths

```
needle::strand::pluck        # Core Pluck logic
needle::strand              # General strand operations
needle::bead_store          # Bead database
needle::worker              # Worker lifecycle
needle::dispatch            # Dispatch logic
needle::claim               # Claiming logic
```

**Full documentation:** `/home/coding/ARMOR/notes/bf-7423-pluck-debug-command-reference.md`
