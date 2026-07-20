# bf-3xmr5h: Starvation alert — beads invisible to worker (recurrence #7)

## Task Description
NEEDLE starvation alert: "Open beads exist but Pluck found none — possible
configuration error." Workspace `/home/coding/ARMOR`, 1606 beads (alert-time),
2 open, 0 in-progress, 0 claimed. Asked to check `exclude_labels`, workspace
path, and filter configuration.

**Seventh identical alert.** Same root cause as the six prior recurrences
(bf-42c149, bf-4uia2g, bf-5rkb68, bf-9bo5na, bf-2s35wz, bf-2zzcld). Bead
state re-verified this run and is **unchanged** — no operator has moved any
gated bead since recurrence #6, so the alert correctly refired. This note
exists only to mark the recurrence and confirm no state drift. Full root-cause
and dependency trace live in `notes/bf-9bo5na.md` (#4); `notes/bf-2zzcld.md`
is the most recent prior note.

## Finding
**Not a configuration error. The starvation is genuine, expected, and
operator-attention-required.** Config re-checked below; diagnosis identical to
the six prior notes.

### Config check — all clean (alert checklist is a red herring, again)
- `.needle.yaml` → `strands.pluck.exclude_labels: []` → no label exclusions.
- `.needle.yaml` → `strands.pluck.split_after_failures: 0` → no auto-split interference.
- Workspace path correct — `.beads/` in `/home/coding/ARMOR`.
- Counts consistent: `br list --status open` = `bf-42dng8` + `bf-4l7q`,
  `br count --status open` = 2, `br ready` = 0 (Pluck's empty result).
- Total beads 1605→1608 (+3 since recurrence #6 — intervening alert beads and
  churn; alert fired reporting 1606 mid-sequence).

### Why no bead is ready (unchanged, re-verified this run)
Both open beads are Genesis roots; every remaining child is held in
`blocked` — a **manual operator status** bead-forge does not auto-clear
when a dependency chain resolves. By design: the blocks are deliberate
gating, not stale state an unattended worker should mutate. All five gated
children re-checked this run and still `blocked`:

- **`bf-42dng8`** — *Genesis: Phase 6 Backup Restore Verification* (open, P1).
  Lone hold **`bf-1ebnuz`** (0 deps) — explicitly **CREDENTIAL-GATED**, still
  `blocked`.
- **`bf-4l7q`** — *Genesis: Phase 5 Integrity Detection Hardening* (open, P1).
  Three held:
  - **`bf-1v6skf`** (P0 bug) — 4/4 deps closed, still `blocked`; needs live S3
    creds to reproduce/verify the HMAC failure.
  - **`bf-2t1f`** (P2) — 2/2 deps closed, still `blocked`; code shipped,
    **OPS-GATED** — verify ArgoCD landed the drift cronworkflow on iad-ci and
    watch the first scheduled run. **Closest to resolution; no creds required.**
  - **`bf-4qq1`** (P0) — blocked by **`bf-5aqh0`** (test-restore queue-api),
    both still `blocked`; needs cluster + scratch access.

## Conclusion
No config fix exists — config is correct. No bead state mutated: every
open/blocked bead sits behind a real operator gate, and unattended workers
must not `br reopen` these holds. The recurring alert is working as intended
— it is surfacing that all remaining ARMOR work needs an operator. Closest to
resolution remains **`bf-2t1f`** (ArgoCD sync verification on iad-ci, no creds
needed).

## Recommendation for the recurring-alert noise
Seven identical alerts now. The alert adds no new signal after the first
diagnosis for this gate-set. Reiterating the prior recommendation:
suppress/snooze the starvation alert once all open beads are `blocked`
(rather than merely `open`), or auto-acknowledge it after the first diagnosis
note for a given gate-set so operators aren't re-pinged every poll interval
until they act.
