# bf-2zzcld: Starvation alert — beads invisible to worker (recurrence #6)

## Task Description
NEEDLE starvation alert: "Open beads exist but Pluck found none — possible
configuration error." Workspace `/home/coding/ARMOR`, 1605 beads, 2 open,
0 in-progress, 0 claimed. Asked to check `exclude_labels`, workspace path,
and filter configuration.

**Sixth identical alert.** Same root cause as bf-42c149 (2026-07-19),
bf-4uia2g (2026-07-20 00:34), bf-5rkb68 (recurrence #3), bf-9bo5na
(recurrence #4), and bf-2s35wz (recurrence #5). Bead state re-verified
this run and is **unchanged** — no operator has moved any gated bead
since recurrence #5, so the alert correctly refired. This note exists
only to mark the recurrence and confirm no state drift.

## Finding
**Not a configuration error. The starvation is genuine, expected, and
operator-attention-required.** Diagnosis is identical to the five prior
notes (see `notes/bf-2s35wz.md` for #5 and `notes/bf-9bo5na.md` for the
full root-cause and dependency trace). Config re-checked below.

### Config check — all clean (alert checklist is a red herring, again)
- `.needle.yaml` → `pluck.exclude_labels: []` → no label exclusions.
- `.needle.yaml` → `pluck.split_after_failures: 0` → no auto-split interference.
- Workspace path correct — `.beads/` in `/home/coding/ARMOR`.
- Counts consistent: `br count --status open` = 2, `br list --status open`
  = `bf-42dng8` + `bf-4l7q`, `br ready` = 0 (Pluck's empty result).
- Total beads 1604→1605 (+1 since recurrence #5 — this very alert bead).

### Why no bead is ready (unchanged, re-verified this run)
Both open beads are Genesis roots; every remaining child is held in
`blocked` — a **manual operator status** bead-forge does not auto-clear
when a dependency chain resolves. That is by design: the blocks represent
deliberate gating, not stale state an unattended worker should mutate.

Dependency chains re-verified this run — all still as recurrence #5 recorded:

- **`bf-42dng8`** — *Genesis: Phase 6 Backup Restore Verification* (open, P1).
  4/5 children closed; the lone hold is **`bf-1ebnuz`** (0 deps, explicitly
  **CREDENTIAL-GATED** — "do not dispatch to unattended workers"). Still `blocked`.
- **`bf-4l7q`** — *Genesis: Phase 5 Integrity Detection Hardening* (open, P1).
  3/6 children closed; 3 held:
  - **`bf-1v6skf`** (P0 bug) — **4/4 deps closed** (bf-28rb, bf-4595,
    bf-24sxh7, bf-2sq7gf all `closed`), yet still `blocked`. Needs **live S3
    creds** to reproduce/verify the HMAC failure on the corrupted multipart object.
  - **`bf-2t1f`** (P2) — **2/2 deps closed** (bf-1ia5y1, bf-3x13d8 both
    `closed`), yet still `blocked`. Code already shipped; remaining work is
    **OPS-GATED** — verify ArgoCD landed the drift cronworkflow on iad-ci
    and watch the first scheduled run. **Closest to resolution; no creds required.**
  - **`bf-4qq1`** (P0) — blocked by **`bf-5aqh0`** (test-restore queue-api),
    itself `blocked`. Both still `blocked`. Needs cluster + scratch access.

## Conclusion
No config fix exists — config is correct. No bead state mutated: every
open/blocked bead sits behind a real operator gate, and unattended workers
must not `br reopen` these holds (per the prior notes and each bead's own
scoping). The recurring alert is working as intended — it is surfacing that
all remaining ARMOR work needs an operator. Closest to resolution remains
**`bf-2t1f`** (ArgoCD sync verification on iad-ci, no creds needed).

## Recommendation for the recurring-alert noise
Six identical alerts now. The alert adds no new signal after the first
diagnosis for this gate-set. Reiterating the prior recommendation:
suppress/snooze the starvation alert once all open beads are `blocked`
(rather than merely `open`), or auto-acknowledge it after the first
diagnosis note for a given gate-set so operators aren't re-pinged every
poll interval until they act.
