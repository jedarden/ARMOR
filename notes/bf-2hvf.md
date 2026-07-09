# Pluck Debug Logging Guide

**Bead:** bf-2hvf  
**Component:** NEEDLE Pluck Strand  
**Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`  
**Date:** 2026-07-09

## Overview

Pluck now includes comprehensive debug logging that traces the entire bead selection and filtering process. All logging is done via the `tracing` crate and can be enabled at runtime.

---

## Debug Events

Pluck emits the following debug events during execution:

### 1. Evaluation Start
```
DEBUG Pluck strand evaluation starting
  exclude_labels: ["deferred", "human", "blocked"]
  split_threshold: 3
```

### 2. Bead Store Query
```
DEBUG Querying bead store for ready candidates
  filters: Filters { assignee: None, exclude_labels: [...] }
```

### 3. Store Results
```
DEBUG Bead store returned {count} candidates
  count: 42
```

### 4. Label Filtering
When beads are excluded:
```
DEBUG Label filtering excluded {excluded_count} beads
  excluded_count: 5
  remaining: 37
  excluded_labels: ["deferred", "human", "blocked"]
```

For each excluded bead:
```
DEBUG Excluded bead due to labels
  bead_id: "bf-1234"
  labels: ["deferred", "priority:p2"]
  excluded_reasons: ["deferred"]
```

### 5. Status/Assignee Filtering
```
DEBUG Status/assignee filtering removed {filtered_count} beads
  filtered_count: 2
  remaining: 35
```

### 6. Sorting
```
DEBUG Sorting {total} candidates by (priority ASC, created_at ASC, id ASC)
  total: 35
  first_bead_id: "bf-0001"
  first_priority: 1
  first_created_at: "2026-07-09T00:00:00Z"
```

### 7. Split Trigger Check
```
DEBUG Checking split trigger for first candidate
  bead_id: "bf-0001"
  failure_count: 0
  threshold: 3
  split_triggered: false
```

When split is triggered:
```
INFO Split threshold reached, returning Split instruction
  bead_id: "bf-0001"
  failure_count: 4
  threshold: 3
```

### 8. Result Return
When candidates are found:
```
INFO Returning {count} candidates for processing
  count: 35
  candidates: ["bf-0001", "bf-0002", "bf-0003", ...]
```

When no candidates remain:
```
DEBUG No candidates remaining after filtering, returning NoWork
```

---

## Enabling Debug Logging

### Method 1: Environment Variable (Recommended)

Set the `RUST_LOG` environment variable to control tracing output:

```bash
# Enable Pluck DEBUG level only
RUST_LOG=needle::strand::pluck=debug needle-worker

# Enable Pluck TRACE level (most verbose)
RUST_LOG=needle::strand::pluck=trace needle-worker

# Enable DEBUG for all NEEDLE modules
RUST_LOG=needle=debug needle-worker

# Enable DEBUG for everything (very verbose)
RUST_LOG=debug needle-worker
```

### Method 2: Tracing Filter Directives

Use tracing filter syntax for fine-grained control:

```bash
# Pluck DEBUG + other crates INFO
RUST_LOG=needle::strand::pluck=debug,info needle-worker

# Pluck TRACE + NEEDLE INFO + other crates WARN
RUST_LOG=needle::strand::pluck=trace,needle=info,warn needle-worker
```

### Method 3: Log File Output

To capture debug output to a file:

```bash
# Enable DEBUG and redirect to file
RUST_LOG=needle::strand::pluck=debug needle-worker 2>&1 | tee pluck-debug.log

# Or use a specific tracing subscriber with file output
# (requires application support for file appenders)
```

---

## Log Levels

| Level | Usage | When to Use |
|-------|-------|-------------|
| **ERROR** | Failures that prevent operation | Store query failures |
| **WARN** | (None currently) | - |
| **INFO** | Significant events | Split trigger, candidate return |
| **DEBUG** | Detailed operation trace | Filter stages, counts, decisions |
| **TRACE** | Most granular details | (Reserved for future expansion) |

---

## Debug Output Examples

### Example 1: Normal Operation
```bash
$ RUST_LOG=needle::strand::pluck=debug needle-worker

DEBUG strand.pluck exclude_labels=["deferred", "human", "blocked"] split_threshold=3 Pluck strand evaluation starting
DEBUG strand.pluck filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] } Querying bead store for ready candidates
DEBUG strand.pluck count=42 Bead store returned 42 candidates
DEBUG strand.pluck excluded_count=5 remaining=37 excluded_labels=["deferred", "human", "blocked"] Label filtering excluded 5 beads
DEBUG strand.pluck count=37 No beads excluded by status/assignee filter
DEBUG strand.pluck total=37 first_bead_id="bf-0001" first_priority=1 first_created_at="2026-07-09T00:00:00Z" Sorting 37 candidates by (priority ASC, created_at ASC, id ASC)
DEBUG strand.pluck bead_id="bf-0001" failure_count=0 threshold=3 split_triggered=false Checking split trigger for first candidate
INFO strand.pluck count=37 candidates=["bf-0001", "bf-0002", ...] Returning 37 candidates for processing
```

### Example 2: All Beads Filtered
```bash
$ RUST_LOG=needle::strand::pluck=debug needle-worker

DEBUG strand.pluck Pluck strand evaluation starting
DEBUG strand.pluck Querying bead store for ready candidates
DEBUG strand.pluck count=3 Bead store returned 3 candidates
DEBUG strand.pluck excluded_count=3 remaining=0 Label filtering excluded 3 beads
  bead_id="bf-deferred" labels=["deferred"] excluded_reasons=["deferred"] Excluded bead due to labels
  bead_id="bf-human" labels=["human"] excluded_reasons=["human"] Excluded bead due to labels
  bead_id="bf-blocked" labels=["blocked"] excluded_reasons=["blocked"] Excluded bead due to labels
DEBUG strand.pluck No candidates remaining after filtering, returning NoWork
```

### Example 3: Split Triggered
```bash
$ RUST_LOG=needle::strand::pluck=debug needle-worker

DEBUG strand.pluck Pluck strand evaluation starting
DEBUG strand.pluck count=1 Bead store returned 1 candidates
DEBUG strand.pluck count=1 No beads excluded by label filter
DEBUG strand.pluck count=1 No beads excluded by status/assignee filter
DEBUG strand.pluck total=1 first_bead_id="bf-failing" first_priority=2 first_created_at="2026-07-08T12:00:00Z" Sorting 1 candidates by (priority ASC, created_at ASC, id ASC)
DEBUG strand.pluck bead_id="bf-failing" failure_count=4 threshold=3 split_triggered=true Checking split trigger for first candidate
INFO strand.pluck bead_id="bf-failing" failure_count=4 threshold=3 Split threshold reached, returning Split instruction
```

---

## Troubleshooting Pluck Filtering

### Debugging Missing Beads

If you expect a bead to be selected but it's not appearing:

1. **Enable debug logging:**
   ```bash
   RUST_LOG=needle::strand::pluck=debug needle-worker
   ```

2. **Check for exclusion events:** Look for "Excluded bead due to labels" entries

3. **Verify labels:** Check the `excluded_reasons` field to see which labels caused exclusion

4. **Check status/assignee:** Look for status filtering messages

### Debugging Empty Queue

When Pluck returns NoWork but you expect beads:

1. **Check initial count:** "Bead store returned N candidates"
   - If 0: No beads in ready state
   - If >0: Beads are being filtered out

2. **Track filtering stages:**
   - "Label filtering excluded N beads"
   - "Status/assignee filtering removed N beads"

3. **Verify exclude_labels:** Check if default excludes (`deferred`, `human`, `blocked`) apply

### Debugging Split Behavior

To understand why a bead is (or isn't) being split:

```bash
RUST_LOG=needle::strand::pluck=debug needle-worker
```

Look for the "Checking split trigger" event which shows:
- `failure_count`: Current failure count from labels
- `threshold`: Configured split threshold
- `split_triggered`: Whether split will occur

---

## Implementation Details

### Tracing Instrumentation

Pluck uses `tracing::instrument` for automatic span creation:

```rust
#[tracing::instrument(
    name = "strand.pluck",
    skip(self, store),
    fields(
        strand = "pluck",
        exclude_labels = ?self.exclude_labels,
        split_threshold = self.split_after_failures,
    )
)]
```

### Span Fields

All Pluck events include these fields:
- `strand`: "pluck" - identifies the strand type
- `exclude_labels`: List of labels being filtered
- `split_threshold`: Failure count threshold for auto-split

### Event Types

- **macro:** `tracing::debug!()` - Detailed diagnostic information
- **macro:** `tracing::info!()` - Significant operational events
- **macro:** `tracing::error!()` - Failure events

---

## Related Documentation

- **Pluck Filter Configurations:** See `bf-1jwl.md` for complete filter documentation
- **Pluck exclude_labels:** See `bf-ogec.md` for exclude_labels extraction
- **NEEDLE Logging:** Check NEEDLE project docs for global logging configuration

---

## Acceptance Criteria Verification

✅ **Debug logging mechanism identified and documented**
- Tracing-based logging implemented
- Environment variable configuration documented
- Multiple log levels available (DEBUG, INFO, ERROR)

✅ **Command or config to enable debug mode verified**
- `RUST_LOG=needle::strand::pluck=debug` enables Pluck DEBUG
- `RUST_LOG=needle::strand::pluck=trace` enables maximum verbosity
- Works with standard tracing subscriber

✅ **Debug output shows filtering logic execution**
- Store query results logged
- Each filtering stage (label, status/assignee) emits counts
- Individual excluded beads logged with reasons
- Sort operation logged with first candidate details
- Split trigger check logged with decision
- Final result (NoWork/BeadFound/Split) logged

---

## Future Enhancements

Possible improvements for future work:

1. **Histogram Metrics:** Track filtering statistics over time
2. **Structured JSON Output:** Emit logs in JSON format for parsing
3. **Per-Bead Timings:** Add timing information for each filtering stage
4. **Filter Decision Logs:** Emit specific reason for each bead's inclusion/exclusion

---

**Status:** ✅ Complete - Pluck debug logging implemented and documented
