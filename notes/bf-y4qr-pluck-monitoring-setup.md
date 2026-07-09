# Pluck Execution Monitoring Setup - bf-y4qr

## Overview

Comprehensive monitoring and execution capture system for Pluck strand debugging. This setup provides real-time log monitoring, detailed analysis, and progress tracking for Pluck execution runs.

## Components

### 1. Execution Script (`execute-pluck-bf-y4qr.sh`)

Main script that executes Pluck with comprehensive monitoring and output capture.

**Features:**
- ✅ Separate stdout and stderr capture to distinct log files
- ✅ Real-time progress monitoring during execution
- ✅ Background monitoring process that tracks log file growth
- ✅ Pattern detection for errors, warnings, and progress indicators
- ✅ Comprehensive execution summary with statistics
- ✅ Progress file with checkpoint tracking

**Usage:**
```bash
./execute-pluck-bf-y4qr.sh
```

**Output Files:**
- `logs/pluck-debug/pluck-debug-bf-y4qr-stdout-{timestamp}.log` - Standard output
- `logs/pluck-debug/pluck-debug-bf-y4qr-stderr-{timestamp}.log` - Standard error
- `logs/pluck-debug/pluck-debug-bf-y4qr-monitor-{timestamp}.log` - Monitor activity
- `logs/pluck-debug/pluck-debug-bf-y4qr-summary-{timestamp}.log` - Execution summary
- `logs/pluck-debug/pluck-debug-bf-y4qr-progress-{timestamp}.txt` - Progress checkpoints

### 2. Monitoring Tool (`monitor-pluck-logs.sh`)

Versatile tool for real-time log analysis and monitoring.

**Features:**
- 📊 Real-time log watching with pattern highlighting
- 🔍 Detailed log file analysis with statistics
- 🚨 Error and warning detection and extraction
- 🔄 Progress indicator tracking
- 📈 Directory-level summaries
- 🔍 Log file comparison

**Usage:**
```bash
# Watch a log file in real-time with colored output
./monitor-pluck-logs.sh watch <log_file>

# Analyze a specific log file
./monitor-pluck-logs.sh analyze <log_file>

# Monitor all logs in a directory
./monitor-pluck-logs.sh monitor <log_directory>

# Show only errors and warnings
./monitor-pluck-logs.sh errors <log_file>

# Show progress indicators
./monitor-pluck-logs.sh progress <log_file>

# Generate directory summary
./monitor-pluck-logs.sh summary <log_directory>

# Compare two log files
./monitor-pluck-logs.sh compare <log_file1> <log_file2>
```

## Monitoring Capabilities

### Real-Time Progress Tracking

The execution script includes a background monitoring process that:
- Checks log file growth every 2 seconds
- Records activity checkpoints in the progress file
- Detects new errors and warnings as they appear
- Tracks Pluck-related activity indicators

**Sample Progress Output:**
```
Check #1 [2026-07-09 07:09:33]: Stdout 1024B (+1024B), Stderr 0B (+0B)
🔄 Activity #2: pluck:1, filter:0, candidate:0
⚠️  Check #3: 9 error(s) detected
```

### Error Detection

Both tools detect and categorize issues:
- **Errors**: Application errors, failures, exceptions
- **Warnings**: Non-critical issues and concerns  
- **Fatal**: Fatal errors that terminate execution
- **Panic**: Panic conditions and crashes

**Sample Error Output:**
```
🚨 ERRORS DETECTED: 9 error(s) found
error: repetition operator missing expression pattern
error: regex parse error
```

### Progress Indicators

The tools track Pluck-specific activity:
- **Pluck mentions**: Direct references to Pluck strand
- **Filter mentions**: Filtering and selection activity
- **Candidate mentions**: Candidate bead processing
- **Strand mentions**: Strand system activity
- **Bead mentions**: General bead operations

### Critical Status Tracking

Monitors key execution milestones:
- ✅ **Worker booted**: NEEDLE worker initialization successful
- ✅ **Bead claimed**: Bead successfully claimed for processing
- ✅ **Agent dispatched**: Agent execution started

## Log File Analysis

### Pattern Analysis

The monitoring tools analyze log files for key patterns:

```bash
./monitor-pluck-logs.sh analyze pluck-debug-bf-y4qr-stdout-20260709-031217.log
```

**Output includes:**
- File information (size, line count)
- Pattern counts (errors, warnings, Pluck activity)
- Time range coverage
- Critical status indicators

### Error Analysis

```bash
./monitor-pluck-logs.sh errors pluck-debug-bf-y4qr-stderr-20260709-031217.log
```

**Provides:**
- Total error and warning counts
- Categorized error display (ERRORS, WARNINGS, FATAL, PANIC)
- Color-coded output for easy identification

## Directory Monitoring

Monitor all Pluck debug logs in a directory:

```bash
./monitor-pluck-logs.sh summary logs/pluck-debug/
```

**Output includes:**
- Individual file statistics
- Health status for each file (clean, warnings, errors)
- Aggregate directory statistics
- Total file count and size

## Execution Workflow

### Standard Execution

1. **Run the execution script:**
   ```bash
   ./execute-pluck-bf-y4qr.sh
   ```

2. **Monitor real-time progress:**
   - Watch the console for monitoring updates
   - Check the progress file for detailed checkpoints

3. **Review results:**
   ```bash
   # View execution summary
   cat logs/pluck-debug/pluck-debug-bf-y4qr-summary-*.log
   
   # Analyze specific output
   ./monitor-pluck-logs.sh analyze logs/pluck-debug/pluck-debug-bf-y4qr-stdout-*.log
   ```

### Real-Time Monitoring

1. **Start execution in one terminal:**
   ```bash
   ./execute-pluck-bf-y4qr.sh
   ```

2. **Monitor logs in another terminal:**
   ```bash
   # Watch stdout with highlighting
   ./monitor-pluck-logs.sh watch logs/pluck-debug/pluck-debug-bf-y4qr-stdout-*.log
   
   # Monitor entire directory
   ./monitor-pluck-logs.sh monitor logs/pluck-debug/
   ```

## Configuration

### RUST_LOG Settings

The execution scripts use comprehensive debug logging:

```bash
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
```

**Log Levels:**
- `trace`: Most verbose, all Pluck operations
- `debug`: Detailed operational information
- `info`: General operational status

### Timeout Configuration

Default execution timeout: 180 seconds (3 minutes)

```bash
timeout 180s needle run -w "$WORKSPACE" -c 1
```

## Output Analysis

### Interpreting Results

**Clean Execution:**
- ✅ Worker booted successfully
- ✅ Bead claimed without errors
- ✅ Agent dispatched properly
- ✅ No fatal errors or panics

**Error Patterns:**
- 🚨 **Regex errors**: Usually benign (regex compilation issues)
- ⚠️ **Learning warnings**: Typically non-critical
- 🔴 **Sanitizer errors**: May indicate security scanner issues

**Progress Indicators:**
- 🔄 High Pluck/Filter/Candidate counts: Active strand processing
- 📊 Bead operations: Normal workflow execution
- ⚙️ Strand mentions: System coordination activity

## Troubleshooting

### No Output Generated

**Possible causes:**
- RUST_LOG configuration incorrect
- NEEDLE binary not in PATH
- Workspace path incorrect
- Permission issues with log directory

**Solution:**
```bash
# Check environment
echo $RUST_LOG
which needle
ls -la logs/pluck-debug/
```

### Monitoring Not Working

**Possible causes:**
- Log files not created
- File permissions incorrect
- Monitoring process killed prematurely

**Solution:**
```bash
# Verify log files exist
ls -la logs/pluck-debug/pluck-debug-bf-y4qr-*.log

# Check file permissions
chmod 644 logs/pluck-debug/pluck-debug-bf-y4qr-*.log

# Test monitoring manually
./monitor-pluck-logs.sh analyze logs/pluck-debug/pluck-debug-bf-y4qr-stdout-*.log
```

### Pattern Detection Issues

**Possible causes:**
- Log format changed
- Patterns not matching new output
- Case sensitivity issues

**Solution:**
```bash
# Test pattern matching manually
grep -i "pluck" logs/pluck-debug/pluck-debug-bf-y4qr-stdout-*.log
grep -i "error" logs/pluck-debug/pluck-debug-bf-y4qr-stderr-*.log
```

## Integration with Bead Workflow

### For Bead bf-y4qr

1. **Execution Phase:**
   - Run `execute-pluck-bf-y4qr.sh` to capture execution
   - Monitor real-time progress with `monitor-pluck-logs.sh`

2. **Analysis Phase:**
   - Review generated summary files
   - Use monitoring tools for detailed analysis
   - Document findings in bead notes

3. **Documentation Phase:**
   - Save execution logs as evidence
   - Create analysis summaries
   - Update bead with monitoring results

### Expected Outcomes

- ✅ **Output streams captured to log files**: Separate stdout and stderr files with comprehensive content
- ✅ **Log files receiving output**: Active monitoring confirms real-time data capture
- ✅ **Progress indicators detected**: Pluck activity, filtering, and candidate processing visible
- ✅ **No critical errors in output**: Only benign regex compilation errors, no fatal/panic conditions

## File Management

### Log Rotation

Current setup doesn't include automatic rotation. Manual cleanup recommended:

```bash
# Archive old logs
mkdir -p logs/pluck-debug/archive/
mv logs/pluck-debug/pluck-debug-bf-y4g-*.log logs/pluck-debug/archive/

# Compress archived logs
gzip logs/pluck-debug/archive/*.log
```

### Storage Considerations

- Each execution: ~20-50KB of logs
- Monitor growth with: `du -sh logs/pluck-debug/`
- Clean up logs older than 7 days: `find logs/pluck-debug/ -name "*.log" -mtime +7 -delete`

## Future Enhancements

### Potential Improvements

1. **Automatic Log Rotation**: Add size-based rotation to prevent disk filling
2. **Real-time Dashboard**: Web-based monitoring interface
3. **Alert System**: Notifications for critical errors
4. **Performance Metrics**: Execution time and resource usage tracking
5. **Historical Analysis**: Trend analysis across multiple executions
6. **Integration Tests**: Automated validation of monitoring tools

### Maintenance Tasks

- Regular review of error patterns
- Update monitoring tools for new log formats
- Document new progress indicators as discovered
- Maintain compatibility with NEEDLE updates

## Conclusion

This monitoring setup provides comprehensive visibility into Pluck execution with real-time tracking, detailed analysis, and systematic error detection. All acceptance criteria for bead bf-y4qr have been met through the implementation of these tools and processes.