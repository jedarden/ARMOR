# bf-5rkb68: Starvation alert — beads invisible to worker (recurrence #3)

## Task Description
NEEDLE starvation alert: "Open beads exist but Pluck found none — possible
configuration error." Workspace `/home/coding/ARMOR`, 1604 beads, 2 open,
0 in-progress, 0 claimed. Asked to check `exclude_labels`, workspace path,
and filter configuration.

**Third identical alert in as many days.** Same root cause as bf-42c149
(2026-07-19) and bf-4uia2g (2026-07-20 00:34). Bead state is **unchanged**
since bf-4uia2g was written ~64 minutes ago — no operator has moved any of
the gated beads, so the alert correctly refired.

## Finding
**Not a configuration error. The starvation is genuine, expected, and
operator-attention-required.** Diagnosis is identical to bf-4uia2g; this
note exists only to mark the recurrence and confirm no state drift. See
[`notes/bf-4uia2g.md`](bf-4uia2g.md) and [`notes/bf-42c149.md`](bf-42c149.md)
for the full root-cause, dependency trace, and ranked operator handoff.

### Config check — all clean (alert checklist is a red herring, again)
- `.needle.yaml` → `strands.pluck.exclude_labels: []` → no label exclusions.
- `.needle.yaml` → `split_after_failures: 0` → no auto-split interference.
- Workspace path correct — `.beads/` in `/home/coding/ARMOR`.
- Counts consistent: `br count --status open` = 2, `br list --status open`
  = `bf-42dng8` + `bf-4l7q`, `br ready` = 0 (Pluck's empty result).

### Why no bead is ready (unchanged from bf-4uia2g)
Both open beads are Genesis roots; every remaining child is held in
`blocked` — a **manual operator status** bead-forge does not auto-clear
when a dependency chain resolves. That is by design: the blocks represent
deliberate gating, not stale state I should mutate.

- **`bf-42dng8`** — *Genesis: Phase 6 Backup Restore Verification* (open, P1).
  4/5 children closed; the lone hold is **`bf-1ebnuz`** (0 deps, explicitly
  **CREDENTIAL-GATED** — "do not dispatch to unattended workers").
- **`bf-4l7q`** — *Genesis: Phase 5 Integrity Detection Hardening* (open, P1).
  3/6 children closed; 3 held:
  - **`bf-1v6skf`** — 4/4 deps closed, yet `blocked`. Needs **live S3 creds**
    to reproduce/verify the HMAC failure on the corrupted object.
  - **`bf-2t1f`** — 2/2 deps closed, yet `blocked`. Code already shipped
    (`scripts/version-drift-check.py`, manifests at `f73030e`); remaining
    work is **OPS-GATED** — verify ArgoCD landed the drift cronworkflow on
    iad-ci and watch the first scheduled run.
  - **`bf-4qq1`** — blocked by **`bf-5aqh0`** (test-restore queue-api),
    itself blocked. Needs cluster + scratch access.

## Conclusion
No config fix exists — config is correct. No bead state mutated: every
open/blocked bead sits behind a real operator gate, and unattended workers
must not `br reopen` these holds (per bf-42c149/bf-4uia2g and each bead's
own scoping). The recurring alert is working as intended — it is surfacing
that all remaining ARMOR work needs an operator. Closest to resolution
remains **`bf-2t1f`** (ArgoCD sync verification on iad-ci, no creds needed).

## Recommendation for the recurring-alert noise
Three identical alerts in three days add no signal once the gate is
documented. Consider one of:
- Suppress the starvation alert when all open beads are `blocked` (not just
  `open`), or
- Add a `gated` / `operator-hold` label and add it to `exclude_labels`, or
- Add a one-line `notes/` pointer in the alert body so each recurrence
  auto-references the standing diagnosis instead of spawning a fresh bead.

## Verification
```bash
br list --status open      # → bf-42dng8, bf-4l7q  (the two Genesis roots)
br ready                   # → (empty)              ← Pluck's empty result
cat .needle.yaml           # → exclude_labels: []   ← config clean
# bf-1ebnuz: 0 deps, blocked           ← credential gate (manual hold)
# bf-1v6skf: 4/4 deps closed, blocked  ← needs live S3 to verify
# bf-2t1f:   2/2 deps closed, blocked  ← ops-gated; manifests at f73030e
# bf-4qq1:   blocked by bf-5aqh0       ← itself blocked (test-restore)
```
