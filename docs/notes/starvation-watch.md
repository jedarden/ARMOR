# Workspace-local starvation watcher

*Added 2026-09-08 (bead armor-a2ffbafb). Companion to
[starvation-2026-08-28-resolution.md](starvation-2026-08-28-resolution.md),
which established that the 2026-08-28 starvation episode was resolved; this
watcher is the machine that catches the next one.*

## The gap it fills

NEEDLE deliberately removed the starvation alert emitter (865484e4,
2026-08-30, "make starvation a terminal verdict"): the alert class had
degenerated into filing blank-bodied `[Unravel] Starvation alert: beads
invisible in <empty workspace>` beads, including "Open beads: 0"
emergencies — the alert pipeline reacting to itself. What survived the
removal is the raw signal: **bead-rs rewrites
`.beads/diagnostics/pluck-diagnostics.json` on every ready query** with
`total_open_beads`, `exclusion_criteria`, `final_candidate_count`, and a
per-bead `excluded_beads` breakdown. Nothing consumed it — as of
2026-09-08, no script on this box or in any fleet repo reads that file
(verified by search across `~/.local/bin`, TUNNEL, tradegraph, and
irreversible-command-gate tooling; those projects' starvation units triage
and auto-close *alert beads*, they do not detect starvation).

## Design

`scripts/starvation-watch.py` runs one cycle per invocation:

1. Read the current diagnostics snapshot. Parse failures are retried
   briefly (bead-rs may be mid-rewrite) and otherwise end the cycle
   quietly — a missing file is never an alert.
2. Compare against the previous snapshot recorded in the watcher's state
   file. An identical timestamp means no ready query has run since the
   last cycle; nothing is re-evaluated.
3. **Declaration requires reproduction**: `total_open_beads > 0 AND
   final_candidate_count == 0` in *both* snapshots, and the pair must be
   temporally adjacent (`--max-gap-seconds`, default 3600) so
   "consecutive" means consecutive observations. A legitimately drained
   workspace (`open == 0`) and a healthy frontier (`candidates > 0`)
   write nothing.
4. On a genuine declaration, file **one normal task bead per episode** —
   plain title with the counts, no `[Unravel]` prefix, no blank Workspace
   field — whose body embeds both diagnostics snapshots verbatim plus a
   per-bead exclusion classification (assigned / manually blocked /
   dependency-blocked / resource conflict), so the follow-up work starts
   classified instead of blank.

Deduplication is mechanical, not stateful-only: the alert bead is created
with `--unique-ref starvation-watch:<episode-start-timestamp>`, so a
repeated create inside one episode returns `EXISTING` /
`EXISTING_CLOSED` instead of filing a duplicate even if the watcher's
state file is lost between runs. A new episode (starvation reproducing
after a healthy or drained snapshot) files a fresh bead. One episode can
therefore never produce two alert beads.

Steady-state cost is a single JSON file read per run; the bead CLI is
invoked only on a declaration. The watcher never mutates anything under
`.beads/` and never closes or modifies existing beads.

## What was deliberately NOT done

- **No NEEDLE worker-loop change.** Point 4 of the original proposal
  ("emit the existing PluckNoCandidate telemetry event") is already
  satisfied: NEEDLE's pluck emits `PluckNoCandidate` on the no-candidate
  condition, pinned by committed tests (NEEDLE 0f9cc75c, confirmed while
  closing armor-41d9474c). The event is in-process telemetry; a
  workspace-side script cannot and should not re-emit it. NEEDLE's
  source tree was also under active parallel edit by the emitter-fix
  lineage (armor-5be5f7bd) at the time this was written.
- **No k8s Job/CronJob.** Prohibited in this environment, and the bead
  said the same. The check is worker-machine-side: a systemd `--user`
  oneshot on a timer, the same mechanism every other workspace's
  starvation tooling uses on this box.

## Operations

```bash
# Install / re-point the timer (idempotent; rewrites paths for this checkout)
scripts/setup-starvation-watch-schedule.sh

# One cycle right now, against the live diagnostics
systemctl --user start armor-starvation-watch.service
journalctl --user -u armor-starvation-watch.service -n 20

# Where the schedule stands
systemctl --user list-timers armor-starvation-watch.timer

# Prove the declare path without filing anything into the workspace:
#   --dry-run prints the would-be alert bead instead of creating it
#   (note: --dry-run also skips the state write, so it always shows a
#   cold start; feed it fixture snapshots via --diagnostics/--state-file)
python3 scripts/starvation-watch.py --dry-run

# Built-in scenario tests (21 checks; never touches live state or beads)
python3 scripts/starvation-watch.py --self-test

# Remove the schedule
systemctl --user disable --now armor-starvation-watch.timer
rm ~/.config/systemd/user/armor-starvation-watch.{service,timer}
systemctl --user daemon-reload
```

State lives under `$XDG_STATE_HOME/starvation-watch/<sha-of-workspace-path>.json`
(two keys: the previous snapshot, and the current episode). Logs go to the
journal via `SyslogIdentifier=armor-starvation-watch`. The timer fires at
:07/:22/:37/:52 — off the :00/:30 marks the other timers cluster on.

## Tuning

| Flag | Default | Meaning |
|---|---|---|
| `--max-gap-seconds` | 3600 | Max age of the previous snapshot for the pair to count as consecutive. On a quiet box (no ready queries overnight) the first morning pair is skipped; persistent starvation still declares one cycle later. |
| `--workspace` | script's repo root | Workspace to watch. The script is workspace-agnostic; pointing it at another checkout watches that workspace. |
| `--bead-bin` | `bead` | Override for tests (a stub records the invocation instead of filing). |
