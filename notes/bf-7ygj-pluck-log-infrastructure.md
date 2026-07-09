# Pluck Log Capture Infrastructure

**Bead:** bf-7ygj
**Date:** 2026-07-09
**Status:** Complete

## Overview

Infrastructure for capturing Pluck's stdout and stderr output during debugging and development.

## Log File Location

**Primary log file:** `/home/coding/ARMOR/pluck-debug.log`

**Timestamped logs:** `/home/coding/ARMOR/pluck-debug-<timestamp>.log`

**Notes directory:** `/home/coding/ARMOR/notes/`

## Output Redirection Command

### Basic redirection (stdout + stderr to same file)
```bash
pluck > pluck-debug.log 2>&1
```

### Timestamped log capture
```bash
pluck > pluck-debug-$(date +%Y%m%d-%H%M%S).log 2>&1
```

### Appending to existing log
```bash
pluck >> pluck-debug.log 2>&1
```

### With tee (view output while capturing)
```bash
pluck 2>&1 | tee pluck-debug.log
```

## Write Permissions

✅ **Verified:** Write permissions confirmed for `/home/coding/ARMOR/`
- User: `coding`
- Group: `users`
- Permissions: `drwxr-xr-x` (755)

## Usage Patterns

### Development debugging
```bash
# Run with debug flags and capture output
RUST_LOG=debug pluck > pluck-debug.log 2>&1

# View in real-time while capturing
RUST_LOG=debug pluck 2>&1 | tee pluck-debug.log
```

### Production troubleshooting
```bash
# Capture with timestamp
pluck > pluck-debug-$(date +%Y%m%d-%H%M%S).log 2>&1

# Check log size
ls -lh pluck-debug*.log
```

### Session-based logging
```bash
# Create session-specific log
SESSION_ID="bf-$(date +%y%m%d)-pluck"
pluck > ${SESSION_ID}.log 2>&1
```

## Log Management

### Rotate large logs
```bash
# Archive old log
mv pluck-debug.log pluck-debug-archived-$(date +%Y%m%d).log

# Compress archived logs
gzip pluck-debug-archived-*.log
```

### Clean up old logs
```bash
# Remove logs older than 7 days
find /home/coding/ARMOR -name "pluck-debug-*.log" -mtime +7 -delete

# List all Pluck logs
ls -lh /home/coding/ARMOR/pluck-debug*.log
```

## Integration with Bead Workflows

When working on beads related to Pluck debugging:
1. Use timestamped logs: `pluck-debug-<bead-id>-<timestamp>.log`
2. Reference log files in bead notes
3. Commit important log captures as artifacts (small logs only)
4. Use `.gitignore` for transient debug logs

## Related Documentation

- **bf-2zo5:** Pluck installation verification and debug configuration
- **bf-6a7c:** Pluck debug execution and capture

## Acceptance Criteria Met

✅ Log file path determined: `/home/coding/ARMOR/pluck-debug.log`
✅ Output redirection command prepared: `> pluck-debug.log 2>&1`
✅ Write permissions verified: Confirmed for `/home/coding/ARMOR/`
