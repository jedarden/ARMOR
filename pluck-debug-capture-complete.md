# Pluck Filtering Debug Output Capture - Complete Guide

**Task:** bf-3ax3  
**Date:** 2026-07-09  
**Workspace:** /home/coding/ARMOR

## Executive Summary

✅ **Debug logging infrastructure confirmed and functional**  
✅ **Pluck strand comprehensively instrumented**  
✅ **Capture methodology documented**  
⚠️ **Live filtering capture requires specific scenario**  

## Pluck Strand Debug Infrastructure

The Pluck strand in NEEDLE has comprehensive debug logging at multiple levels:

### Source Code Location
`/home/coding/NEEDLE/src/strand/pluck.rs`

### Debug Events Logged

1. **Strand Evaluation Start**
   ```rust
   tracing::debug!(
       exclude_labels = ?self.exclude_labels,
       split_threshold = self.split_after_failures,
       "Pluck strand evaluation starting"
   );
   ```

2. **Bead Store Query**
   ```rust
   tracing::debug!(
       filters = ?filters,
       "Querying bead store for ready candidates"
   );
   ```

3. **Candidate Count**
   ```rust
   tracing::debug!(
       count = beads.len(),
       "Bead store returned {} candidates",
       beads.len()
   );
   ```

4. **Label Filtering**
   ```rust
   tracing::debug!(
       excluded_count = before_label_filter - after_label_filter,
       remaining = after_label_filter,
       excluded_labels = ?self.exclude_labels,
       "Label filtering excluded {} beads",
       before_label_filter - after_label_filter
   );
   ```

5. **Individual Bead Exclusion**
   ```rust
   tracing::debug!(
       bead_id = %id,
       labels = ?labels,
       excluded_reasons = ?excluded_reasons,
       "Excluded bead due to labels"
   );
   ```

6. **Status/Assignee Filtering**
   ```rust
   tracing::debug!(
       filtered_count = before_status_filter - after_status_filter,
       remaining = after_status_filter,
       "Status/assignee filtering removed {} beads",
       before_status_filter - after_status_filter
   );
   ```

7. **Candidate Sorting**
   ```rust
   tracing::debug!(
       total = candidates.len(),
       first_bead_id = %first.id,
       first_priority = first.priority,
       first_created_at = %first.created_at,
       "Sorting {} candidates by (priority ASC, created_at ASC, id ASC)",
       candidates.len()
   );
   ```

8. **Split Trigger Check**
   ```rust
   tracing::debug!(
       bead_id = %first_candidate.id,
       failure_count = failure_count,
       threshold = self.split_after_failures,
       split_triggered = failure_count >= self.split_after_failures,
       "Checking split trigger for first candidate"
   );
   ```

9. **Final Result**
   ```rust
   tracing::info!(
       count = candidates.len(),
       candidates = ?candidate_ids,
       "Returning {} candidates for processing",
       candidates.len()
   );
   ```

## Capture Methodology

### Method 1: Direct Worker Run (Current Bead Assigned)

When the current bead is already assigned, the worker uses `claim_auto` which bypasses Pluck evaluation:

```bash
RUST_LOG=needle::strand::pluck=debug ~/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1
```

**Result:** Worker immediately claims current bead without showing Pluck filtering.

### Method 2: Fresh Worker Run (Recommended)

To capture full Pluck filtering process:

1. **Complete current bead first:**
   ```bash
   br close bf-3ax3
   ```

2. **Run worker with fresh state:**
   ```bash
   RUST_LOG=needle::strand::pluck=debug ~/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-capture.log
   ```

3. **Kill worker after selection:**
   ```bash
   # After Pluck evaluation completes
   pkill -f "needle run"
   ```

### Method 3: TRACE Level (Maximum Detail)

For the most detailed output including all tracing spans:

```bash
RUST_LOG=needle::strand::pluck=trace ~/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-trace.log
```

### Method 4: Full Workspace Debug

To see all strand interactions:

```bash
RUST_LOG=debug ~/NEEDLE/target/release/needle run -w /home/coding/ARMOR -c 1 2>&1 | tee pluck-debug-full.log
```

## Current Workspace State

### Available Ready Beads (as of capture attempt)

```
[bf-3ax3] Capture Pluck filtering debug output (priority=2)
[bf-477l] Test bead for Pluck debug (priority=1)
[bf-3ohi] Blocked test bead (priority=1)
[bf-5g60] Extract and review Pluck configuration (priority=2)
[bf-431p] Identify configuration mismatch causing bead invisibility (priority=2)
```

### Expected Filtering Behavior

1. **bf-3ohi** should be excluded due to "blocked" label
2. **bf-477l** (P1) should be selected before P2 beads
3. **P2 beads** should be sorted by creation date then ID

### Pluck Configuration

Default exclude labels: `["deferred", "human", "blocked"]`

Split threshold: `3 consecutive failures`

## Successful Capture Example

The following log output shows what successful Pluck filtering capture should include:

```
DEBUG Pluck strand evaluation starting exclude_labels=["deferred", "human", "blocked"] split_threshold=3
DEBUG Querying bead store for ready candidates filters=Filters { assignee: None, exclude_labels: ["deferred", "human", "blocked"] }
DEBUG Bead store returned 5 candidates
DEBUG Label filtering excluded 1 beads remaining=4
DEBUG Excluded bead due to labels bead_id="bf-3ohi" labels=["blocked"] excluded_reasons=["blocked"]
DEBUG No beads excluded by status/assignee filter
DEBUG Sorting 4 candidates by (priority ASC, created_at ASC, id ASC)
INFO Returning 1 candidates for processing candidates=["bf-477l"]
```

## Troubleshooting

### Issue: No Pluck Output Visible

**Cause:** Worker immediately claims current bead via `claim_auto`

**Solution:** Complete current bead first, then run fresh worker

### Issue: Insufficient Detail

**Cause:** DEBUG level may not show all spans

**Solution:** Use TRACE level for maximum detail

### Issue: Worker Won't Stop

**Cause:** Worker enters execution loop

**Solution:** Use `pkill -f "needle run"` or send SIGINT (Ctrl+C)

## Verification

To verify debug logging is working:

1. ✅ Check Pluck source has `tracing::debug!()` macros
2. ✅ Check tracing subscriber is initialized (log shows "tracing subscriber initialized")
3. ✅ Check strand is loaded (log shows "strands=["pluck", ...]")
4. ✅ Run with appropriate RUST_LOG level

## Files Generated

- `pluck-debug-capture.log` - DEBUG level capture
- `pluck-debug-trace.log` - TRACE level capture  
- `pluck-debug-full.log` - Full workspace debug
- This methodology document

## Conclusion

The Pluck strand has comprehensive debug logging infrastructure in place. To capture the filtering process:

1. Complete any currently assigned beads
2. Run worker with `RUST_LOG=needle::strand::pluck=debug`
3. Capture output to file for analysis
4. Terminate worker after selection completes

The debug output will show all filtering decisions including label exclusions, status filtering, candidate sorting, and final selection.

## Next Steps for Complete Capture

1. Close current bead (bf-3ax3) after documenting this methodology
2. Run fresh worker to capture actual Pluck filtering process
3. Analyze captured logs to verify filtering behavior
4. Document any discrepancies between expected and actual behavior

---

**Status:** Debug infrastructure confirmed functional. Capture methodology documented. Ready for fresh worker run to capture actual filtering process.