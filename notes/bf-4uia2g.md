# bf-4uia2g: Starvation alert — beads invisible to worker (recurrence)

## Task Description
NEEDLE starvation alert: "Open beads exist but Pluck found none — possible
configuration error." Workspace `/home/coding/ARMOR`, 1603 beads, 2 open,
0 in-progress, 0 claimed. Asked to check `exclude_labels`, workspace path,
and filter configuration.

**This is the second identical alert in two days** — same root cause as
bf-42c149 (diagnosed 2026-07-19). State is unchanged because every remaining
bead is operator-gated; unattended workers cannot move any of them.

## Finding
**Not a configuration error. The starvation is genuine, expected, and
operator-attention-required.** `br ready` correctly returns the empty set
because the only two open beads are dependency-blocked Genesis roots, and
every one of their remaining children is held in `blocked` status behind a
real gate (credentials, live cluster/data, or an explicit ops hold).

### Config check — all clean (the alert's checklist is a red herring here)
- `exclude_labels: []` in `.needle.yaml` → no label-based exclusions.
- Workspace path correct — `.beads/` in `/home/coding/ARMOR`.
- No filter misconfiguration: `br count` = 1603, `br list --status open` = 2,
  `br ready` = 0.

### Why no bead is ready
Both open beads are Genesis roots whose remaining children are themselves
blocked. `blocked` is a **manual operator status** independent of dependency
resolution — bead-forge does not auto-clear it when a dep chain resolves.
That is by design here: the flags represent deliberate gating.

**`bf-42dng8`** — *Genesis: ARMOR Phase 6 Backup Restore Verification* (open, P1)
- 4 of 5 blockers closed. 1 still blocked:
  - **`bf-1ebnuz`** — *Multipart-era corruption audit*. Zero dependencies.
    Description is explicitly **CREDENTIAL-GATED**: "do not dispatch to
    unattended workers (requires live B2/cluster credentials an operator must
    supply)." Manual operator hold, not an unmet dep.

**`bf-4l7q`** — *Genesis: ARMOR Integrity Detection Hardening (Phase 5)* (open, P1)
- 3 of 6 blockers closed. 3 still blocked:
  - **`bf-1v6skf`** (P0, large-multipart HMAC failure) — **all 4 deps closed**
    (`bf-28rb`, `bf-4595`, `bf-24sxh7`, `bf-2sq7gf`), yet held `blocked`.
    Reproducing/verifying any fix requires live S3 access to the corrupted
    object (the description's evidence is a `kubectl port-forward` + direct
    S3 GET). Cannot be verified by an unattended worker without creds.
  - **`bf-2t1f`** (P2, version-drift check) — both deps closed
    (`bf-1ia5y1`, `bf-3x13d8`). Re-scoped 2026-07-18 (comment [80]):
    code is **already built** (`scripts/version-drift-check.py` + pipeline,
    `k8s/armor-drift-check-{workflowtemplate,cronworkflow}.yml`,
    `docs/drift-check.md`). Remaining work is **OPS-GATED**: "do not dispatch
    to unattended workers."
  - **`bf-4qq1`** (P0, bump ord-devimprint ARMOR + verify restore) — blocked
    by `bf-5aqh0` (test-restore), which is itself blocked.

### New finding since bf-42c149 — `bf-2t1f` is one verified step from done
The prior note called `bf-2t1f` "manually held blocked" without the scoping
detail. Tracing it further:
- The drift-check manifests **are committed** in declarative-config
  (`f73030e feat(armor-drift-check): Add ARMOR version drift check workflows`),
  synced by ArgoCD app `argo-workflows-ns-iad-ci`.
- **But the cronworkflow is NOT yet deployed on iad-ci** — read-only check
  (`kubectl ... get cronworkflows -n argo-workflows` on iad-ci) shows no drift
  cronworkflow, and no drift workflow has ever run.

So `bf-2t1f`'s only remaining work is the ops-gated verification: confirm
ArgoCD synced the two manifests to iad-ci, watch the first scheduled run
(daily 09:00 UTC), and confirm reporting. This is the closest-to-actionable
item for the operator — no code or credentials required, just cluster eyes.

## Operator handoff (ranked by how close to unblockable)
1. **`bf-2t1f`** — verify ArgoCD sync of `argo-workflows-ns-iad-ci` landed the
   drift manifests on iad-ci; if not, investigate the sync gap (manifests are
   in git at f73030e but absent from the cluster). Watch first run + reporting.
   → Unblocks 1 of 3 children of `bf-4l7q`.
2. **`bf-1v6skf`** — with live S3 creds, reproduce the HMAC failure on the
   2026-07-14 level-9 LTX object and verify a decrypt fix. Deps already closed.
3. **`bf-1ebnuz`** — credential-gated corruption audit of unaudited buckets.
4. **`bf-4qq1` → `bf-5aqh0`** — bump ord-devimprint ARMOR in declarative-config,
   deploy, then test-restore queue-api to a scratch DB. Cluster + scratch access.

## Conclusion
No configuration fix is possible — the config is correct. No bead state was
mutated: every open/blocked bead is held behind a real gate, and unattended
workers should not `br reopen` operator holds (per bf-42c149 and each bead's
own scoping). The recurring alert is functioning correctly — it is surfacing
that all remaining ARMOR work needs an operator. Closest to resolution:
`bf-2t1f` (ArgoCD sync verification, no creds needed).

## Verification
```bash
br list --status open      # → bf-42dng8, bf-4l7q  (the two Genesis roots)
br ready                   # → (empty)              ← Pluck's empty result
cat .needle.yaml           # → exclude_labels: []   ← config clean
# bf-1ebnuz: 0 deps, blocked           ← credential gate (manual hold)
# bf-1v6skf: 4/4 deps closed, blocked  ← needs live S3 to verify
# bf-2t1f:   2/2 deps closed, blocked  ← ops-gated; manifests committed in
#                                         declarative-config f73030e but not
#                                         yet deployed to iad-ci
```
