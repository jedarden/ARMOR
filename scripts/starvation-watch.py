#!/usr/bin/env python3
"""Workspace-local starvation watcher for the ARMOR bead workspace.

Consumes ``.beads/diagnostics/pluck-diagnostics.json`` — rewritten by
bead-rs on every ready query — and files ONE plain task bead per
starvation episode, only when the condition reproduces across two
consecutive snapshots:

    total_open_beads > 0 AND final_candidate_count == 0

A drained workspace (open == 0) and a healthy frontier (candidates > 0)
write nothing. A single snapshot is never enough; the pair must also be
temporally adjacent (``--max-gap-seconds``, default 1h) so "consecutive"
means consecutive observations, not two reads from different eras.

This fills the detection gap left by NEEDLE 865484e4 (2026-08-30, "make
starvation a terminal verdict" removed the alert emitter) without
resurrecting it. The old emitter could file "Open beads: 0" emergencies
with a blank Workspace field; this watcher files a normally-formed task
bead carrying the diagnostics JSON verbatim plus a per-bead exclusion
classification, guarded by an idempotent ``--unique-ref`` so one episode
can never produce two alert beads even if the watcher's state file is
lost between runs.

Steady-state cost is one JSON file read per invocation; the bead CLI is
invoked only on a genuine declaration.

Usage:
    starvation-watch.py [--workspace DIR] [--dry-run] [--self-test]

Designed to run as a systemd --user oneshot on a timer (see
scripts/armor-starvation-watch.{service,timer} and
scripts/setup-starvation-watch-schedule.sh). It deliberately is not a
k8s Job/CronJob — those are prohibited in this environment — and it does
not modify NEEDLE: the surviving PluckNoCandidate telemetry event is
already emitted by NEEDLE's own pluck with committed tests pinning it.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path

STATE_VERSION = 1
DEFAULT_MAX_GAP_SECONDS = 3600
DEFAULT_BEAD_BIN = "bead"
ALERT_LABEL = "starvation-watch"
UNIQUE_REF_NAMESPACE = "starvation-watch"
ALERT_PRIORITY = "1"  # critical: an episode means zero dispatchable work


class SnapshotError(Exception):
    """The diagnostics file is missing, unreadable, or has the wrong shape."""


def parse_ts(value: str) -> datetime:
    """Parse a bead-rs timestamp (nanosecond precision, 'Z' suffix).

    datetime.fromisoformat only takes microsecond precision before 3.11,
    so trim the fraction ourselves instead of depending on the version.
    """
    text = value.strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    if "." in text:
        head, tail = text.split(".", 1)
        for i, ch in enumerate(tail):
            if not ch.isdigit():
                frac, rest = tail[:i], tail[i:]
                break
        else:
            frac, rest = tail, ""
        frac = (frac + "000000")[:6]
        text = f"{head}.{frac}{rest}"
    return datetime.fromisoformat(text)


def gap_seconds(prev_ts: str, cur_ts: str) -> float:
    return (parse_ts(cur_ts) - parse_ts(prev_ts)).total_seconds()


def load_snapshot(path: Path, retries: int = 3, sleep: float = 0.5) -> dict:
    """Read and validate one diagnostics snapshot.

    bead-rs may be mid-rewrite when we read, so a parse failure is
    retried briefly before giving up on this cycle.
    """
    import time

    last_err: Exception | None = None
    for attempt in range(retries):
        try:
            raw = path.read_text(encoding="utf-8")
            snap = json.loads(raw)
        except (OSError, json.JSONDecodeError) as exc:
            last_err = exc
            if attempt + 1 < retries:
                time.sleep(sleep)
            continue
        validate_snapshot(snap)
        return snap
    raise SnapshotError(f"{path}: {last_err}")


def validate_snapshot(snap) -> None:
    if not isinstance(snap, dict):
        raise SnapshotError("snapshot is not a JSON object")
    for key in ("timestamp", "total_open_beads", "final_candidate_count"):
        if key not in snap:
            raise SnapshotError(f"snapshot missing required key {key!r}")
    if not isinstance(snap["excluded_beads"], list):
        raise SnapshotError("excluded_beads is not a list")


def is_starved(snap: dict) -> bool:
    """The starvation predicate: work exists but nothing is dispatchable."""
    return snap["total_open_beads"] > 0 and snap["final_candidate_count"] == 0


def is_drained(snap: dict) -> bool:
    return snap["total_open_beads"] == 0


def classify_exclusions(snap: dict) -> list[str]:
    """Render one line per excluded bead with the reason(s) it was excluded."""
    lines = []
    for entry in snap.get("excluded_beads", []):
        reasons = []
        if entry.get("assignee"):
            reasons.append(f"assigned to {entry['assignee']}")
        if entry.get("manual_blocked"):
            reasons.append("manually blocked (policy park)")
        if entry.get("has_blockers"):
            n = entry.get("blocker_count", "?")
            reasons.append(f"dependency-blocked ({n} blocker(s))")
        if entry.get("has_resource_conflicts"):
            n = entry.get("conflict_count", "?")
            reasons.append(f"resource conflict ({n})")
        if not reasons:
            reasons.append("unclassified")
        lines.append(
            f"- {entry.get('bead_id', '?')} (P{entry.get('priority', '?')}) "
            f"{entry.get('title', '')} — {'; '.join(reasons)}"
        )
    return lines


def episode_key(ts: str) -> str:
    """Compact, colon-free key for --unique-ref from an episode start ts."""
    return parse_ts(ts).astimezone(timezone.utc).strftime("%Y%m%dT%H%M%SZ")


def render_body(workspace: str, cur: dict, prev: dict | None) -> str:
    parts = [
        f"Starvation reproduced in the {workspace} bead workspace: "
        f"{cur['total_open_beads']} open beads and "
        f"{cur['final_candidate_count']} ready candidates in the pluck "
        f"diagnostics snapshot {cur['timestamp']},"
    ]
    if prev is not None:
        parts.append(
            f"reproduced across two consecutive snapshots (previous snapshot "
            f"{prev['timestamp']}: {prev['total_open_beads']} open, "
            f"{prev['final_candidate_count']} candidates)."
        )
    else:
        parts.append("with no previous snapshot on record for this watcher.")
    parts.append(
        "A ready query returned no candidates while open beads remained, on "
        "more than one consecutive snapshot: workers cannot claim work in "
        "this workspace. Investigate the exclusions below."
    )
    parts.append("")
    parts.append(f"## Exclusion classification ({len(cur.get('excluded_beads', []))} beads)")
    parts.append("")
    classification = classify_exclusions(cur)
    parts.extend(classification if classification else ["(none recorded)"])
    parts.append("")
    parts.append("## Current diagnostics snapshot (verbatim)")
    parts.append("")
    parts.append("```json")
    parts.append(json.dumps(cur, indent=2, sort_keys=True))
    parts.append("```")
    if prev is not None:
        parts.append("")
        parts.append("## Previous diagnostics snapshot (verbatim)")
        parts.append("")
        parts.append("```json")
        parts.append(json.dumps(prev, indent=2, sort_keys=True))
        parts.append("```")
    return "\n".join(parts)


def render_title(cur: dict, prev: dict | None) -> str:
    since = prev["timestamp"] if prev is not None else cur["timestamp"]
    return (
        f"Starvation reproduced: {cur['total_open_beads']} open beads, "
        f"{cur['final_candidate_count']} ready candidates across consecutive "
        f"pluck snapshots (since {since})"
    )


def file_alert(bead_bin: str, title: str, body: str, ref: str, dry_run: bool) -> tuple[str, str]:
    """Create the alert bead. Returns (status, bead_id_or_message).

    status is one of: created, existing, existing_closed, dry_run, error.
    Idempotent via --unique-ref: a repeated create for the same episode
    returns existing/existing_closed instead of filing a duplicate.
    """
    cmd = [
        bead_bin, "create",
        "--title", title,
        "--priority", ALERT_PRIORITY,
        "--issue-type", "task",
        "--label", ALERT_LABEL,
        "--description", body,
        "--unique-ref", f"{UNIQUE_REF_NAMESPACE}:{ref}",
    ]
    if dry_run:
        print(f"[dry-run] would run: {' '.join(cmd[:2])} --title {title!r} ...", file=sys.stderr)
        print(body)
        return "dry_run", ""
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return "error", str(exc)
    out = (proc.stdout or "").strip()
    if proc.returncode != 0:
        return "error", (proc.stderr or out or f"exit {proc.returncode}")
    if out.startswith("EXISTING_CLOSED"):
        return "existing_closed", out[len("EXISTING_CLOSED"):].strip()
    if out.startswith("EXISTING"):
        return "existing", out[len("EXISTING"):].strip()
    return "created", out


def default_state_path(workspace: Path) -> Path:
    """Per-workspace state file under XDG_STATE_HOME, keyed by path hash."""
    xdg = os.environ.get("XDG_STATE_HOME") or str(Path.home() / ".local" / "state")
    slug = hashlib.sha256(str(workspace.resolve()).encode()).hexdigest()[:16]
    return Path(xdg) / "starvation-watch" / f"{slug}.json"


def load_state(path: Path) -> dict:
    try:
        state = json.loads(path.read_text(encoding="utf-8"))
        if isinstance(state, dict) and state.get("version") == STATE_VERSION:
            return state
    except (OSError, json.JSONDecodeError):
        pass
    return {"version": STATE_VERSION, "prev": None, "episode": None}


def save_state(path: Path, state: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=str(path.parent), prefix=path.name, suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(state, handle, indent=2, sort_keys=True)
            handle.write("\n")
        os.replace(tmp, path)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise


def run_cycle(workspace: str, diagnostics: Path, state_path: Path,
              bead_bin: str, max_gap_seconds: float, dry_run: bool,
              log=None, filer=None) -> str:
    """One watcher cycle. Returns a short outcome string.

    ``filer`` overrides the bead-filing callable (used by --self-test);
    it takes (bead_bin, title, body, ref, dry_run) and returns
    (status, detail) exactly like ``file_alert``.
    """
    if log is None:
        log = lambda msg: print(msg, file=sys.stderr)  # noqa: E731

    cur = load_snapshot(diagnostics)
    state = load_state(state_path)
    prev = state.get("prev")
    episode = state.get("episode")

    if prev is not None and prev.get("timestamp") == cur.get("timestamp"):
        # No ready query has rewritten the diagnostics file since our last
        # cycle: nothing new to observe, and re-evaluating the same pair
        # would re-litigate an already-adjudicated episode.
        log(f"starvation-watch: unchanged (snapshot {cur['timestamp']} already evaluated)")
        return "unchanged"

    starved_cur = is_starved(cur)
    prev_starved = prev is not None and is_starved(prev)
    outcome = "healthy"

    if prev is None:
        # Cold start: record the snapshot, never declare on the first one.
        episode = {"started": cur["timestamp"], "alerted": False, "bead": None} if starved_cur else None
        outcome = "cold-start" + (" (starved, awaiting reproduction)" if starved_cur else "")
    elif starved_cur and prev_starved:
        gap = gap_seconds(prev["timestamp"], cur["timestamp"])
        if episode is None:
            episode = {"started": prev["timestamp"], "alerted": False, "bead": None}
        if gap > max_gap_seconds:
            # Observations too far apart to call consecutive; keep the
            # episode but wait for a fresh pair.
            outcome = f"starved-but-gap({gap:.0f}s>{max_gap_seconds:.0f}s)"
        elif not episode.get("alerted"):
            ref = episode_key(episode["started"])
            body = render_body(workspace, cur, prev)
            filer = filer if filer is not None else file_alert
            status, detail = filer(bead_bin, render_title(cur, prev), body, ref, dry_run)
            episode["alerted"] = True
            episode["bead"] = detail if status in ("created", "existing") else None
            episode["file_status"] = status
            outcome = f"declared({status})" if status != "error" else f"error:{detail}"
        else:
            outcome = "starved (already alerted this episode)"
    elif starved_cur:
        episode = {"started": cur["timestamp"], "alerted": False, "bead": None}
        outcome = "starved (first snapshot of episode)"
    else:
        episode = None
        if is_drained(cur):
            outcome = "drained"
        elif prev is not None and prev_starved:
            outcome = "recovered"

    state["prev"] = cur
    state["episode"] = episode
    if not dry_run:
        save_state(state_path, state)
    log(f"starvation-watch: {outcome} "
        f"(open={cur['total_open_beads']} candidates={cur['final_candidate_count']} "
        f"excluded={len(cur.get('excluded_beads', []))} ts={cur['timestamp']})")
    return outcome


def run_self_test() -> int:
    """Exercise the state machine and rendering against synthetic snapshots.

    Never touches the real diagnostics file, state file, or bead CLI.
    """
    failures = []

    def snap(ts, open_, candidates, excluded=None):
        return {
            "timestamp": ts,
            "total_open_beads": open_,
            "final_candidate_count": candidates,
            "exclusion_criteria": {"has_assignee": 0, "manually_blocked": 0,
                                   "has_dependencies": 0, "resource_conflicts": 0},
            "excluded_beads": excluded or [],
        }

    excluded = [
        {"bead_id": "armor-aaa", "title": "blocked work", "priority": 1,
         "assignee": None, "manual_blocked": False, "has_blockers": True,
         "blocker_count": 2, "has_resource_conflicts": False, "conflict_count": 0},
        {"bead_id": "armor-bbb", "title": "parked work", "priority": 0,
         "assignee": "someone", "manual_blocked": True, "has_blockers": False,
         "blocker_count": 0, "has_resource_conflicts": True, "conflict_count": 1},
    ]

    def cycle(snapshots, bead_bin="bead", max_gap=3600):
        """Feed snapshots as successive cycles; return (outcomes, filings)."""
        filings = []
        with tempfile.TemporaryDirectory() as tmp:
            diag = Path(tmp) / "pluck-diagnostics.json"
            state_path = Path(tmp) / "state.json"
            outcomes = []
            for s in snapshots:
                diag.write_text(json.dumps(s), encoding="utf-8")

                def filer(bead_bin=bead_bin, title="", body="", ref="",
                          dry_run=False, filings=filings):
                    filings.append({"title": title, "body": body, "ref": ref})
                    return "created", "armor-fake0001"
                outcomes.append(run_cycle(
                    "/tmp/ws", diag, state_path, bead_bin, max_gap,
                    dry_run=False, log=lambda *_: None, filer=filer))
        return outcomes, filings

    def check(name, cond):
        print(f"{'PASS' if cond else 'FAIL'}: {name}")
        if not cond:
            failures.append(name)

    t = "2026-09-08T10:00:00Z"
    seq = lambda i: f"2026-09-08T10:{i:02d}:00Z"

    # 1-2: healthy pairs and a single starved snapshot never declare.
    out, f = cycle([snap(t, 5, 2), snap(seq(1), 5, 2)])
    check("healthy->healthy files nothing", not f)
    out, f = cycle([snap(t, 5, 2), snap(seq(1), 5, 0)])
    check("healthy->starved files nothing (needs consecutive)", not f)

    # 3-4: two consecutive starved snapshots declare exactly once.
    out, f = cycle([snap(t, 5, 2), snap(seq(1), 5, 0), snap(seq(2), 5, 0), snap(seq(3), 5, 0)])
    check("starved pair declares once", len(f) == 1)
    check("declared outcome reported", any(o.startswith("declared") for o in out))

    # 5: recovery re-arms; a later episode files again.
    out, f = cycle([snap(t, 5, 0), snap(seq(1), 5, 0), snap(seq(2), 5, 3),
                    snap(seq(3), 5, 0), snap(seq(4), 5, 0)])
    check("recovery re-arms, second episode files again", len(f) == 2)

    # 6: a drained workspace is never starvation.
    out, f = cycle([snap(t, 0, 0), snap(seq(1), 0, 0), snap(seq(2), 0, 0)])
    check("drained workspace files nothing", not f)
    check("drained outcome reported", "drained" in out)

    # 7: an unchanged snapshot is not re-evaluated.
    out, f = cycle([snap(t, 5, 0), snap(t, 5, 0), snap(t, 5, 0)])
    check("unchanged snapshots file nothing", not f)
    check("unchanged outcome reported", "unchanged" in out)

    # 8: a pair spread wider than max_gap is not consecutive evidence.
    out, f = cycle([snap(t, 5, 0), snap("2026-09-08T23:00:00Z", 5, 0)], max_gap=3600)
    check("stale pair does not declare", not f)
    out, f = cycle([snap(t, 5, 0), snap("2026-09-08T23:00:00Z", 5, 0),
                    snap("2026-09-08T23:15:00Z", 5, 0)], max_gap=3600)
    check("next fresh pair after stale gap declares", len(f) == 1)

    # 9: the body embeds both snapshots verbatim and the classification.
    out, f = cycle([snap(t, 2, 1, excluded), snap(seq(1), 2, 0, excluded),
                    snap(seq(2), 2, 0, excluded)])
    body = f[0]["body"] if f else ""
    check("alert filed for body inspection", len(f) == 1)
    check("body embeds current snapshot verbatim",
          json.dumps(snap(seq(2), 2, 0, excluded), indent=2, sort_keys=True) in body)
    check("body embeds previous snapshot verbatim",
          json.dumps(snap(seq(1), 2, 0, excluded), indent=2, sort_keys=True) in body)
    check("body classifies dependency-blocked bead",
          "armor-aaa (P1) blocked work — dependency-blocked (2 blocker(s))" in body)
    check("body classifies parked bead with all reasons",
          "armor-bbb (P0) parked work — assigned to someone; "
          "manually blocked (policy park); resource conflict (1)" in body)
    check("title carries counts, never a bare 'Open beads: 0' style alert",
          "2 open beads, 0 ready candidates" in f[0]["title"])

    # 10: the ref handed to the filer is the episode-scoped compact
    # timestamp; file_alert composes the namespace onto it.
    check("unique-ref is episode-scoped and colon-free",
          f[0]["ref"] == "starvation-watch:20260908T100100Z"
          or f[0]["ref"] == "20260908T100100Z")

    # 11: the real filer shells out to bead create with --unique-ref.
    import subprocess as sp
    orig_run = sp.run

    recorded = {}

    def fake_run(cmd, **kwargs):
        recorded["cmd"] = cmd
        return sp.CompletedProcess(cmd, 0, stdout="armor-1234\n", stderr="")

    sp.run = fake_run
    try:
        status, detail = file_alert("bead", "T", "B", "20260908T100000Z", dry_run=False)
    finally:
        sp.run = orig_run
    check("filer invokes bead create", recorded["cmd"][:2] == ["bead", "create"])
    check("filer passes namespaced unique-ref",
          f"{UNIQUE_REF_NAMESPACE}:20260908T100000Z" in recorded["cmd"])
    check("filer parses fresh-create id", status == "created" and detail == "armor-1234")

    print()
    if failures:
        print(f"{len(failures)} self-test(s) FAILED: {failures}")
        return 1
    print("all self-tests passed")
    return 0


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    default_workspace = Path(__file__).resolve().parent.parent
    parser.add_argument("--workspace", default=str(default_workspace),
                        help=f"bead workspace to watch (default: {default_workspace})")
    parser.add_argument("--diagnostics",
                        help="pluck-diagnostics.json path "
                             "(default: <workspace>/.beads/diagnostics/pluck-diagnostics.json)")
    parser.add_argument("--state-file", help="watcher state path (default: XDG state dir)")
    parser.add_argument("--bead-bin", default=DEFAULT_BEAD_BIN)
    parser.add_argument("--max-gap-seconds", type=float, default=DEFAULT_MAX_GAP_SECONDS,
                        help="max age of the previous snapshot for the pair to count "
                             "as consecutive (default: 3600)")
    parser.add_argument("--dry-run", action="store_true",
                        help="print the would-be alert instead of filing; no state write")
    parser.add_argument("--self-test", action="store_true",
                        help="run the built-in scenario tests and exit")
    args = parser.parse_args(argv)

    if args.self_test:
        return run_self_test()

    workspace = str(Path(args.workspace).resolve())
    diagnostics = Path(args.diagnostics) if args.diagnostics else \
        Path(workspace) / ".beads" / "diagnostics" / "pluck-diagnostics.json"
    state_path = Path(args.state_file) if args.state_file else default_state_path(Path(workspace))

    try:
        outcome = run_cycle(workspace, diagnostics, state_path,
                            args.bead_bin, args.max_gap_seconds, args.dry_run)
    except SnapshotError as exc:
        print(f"starvation-watch: no cycle this run: {exc}", file=sys.stderr)
        return 0  # transient (file mid-rewrite or absent) is not a failure
    return 0 if not outcome.startswith("error") else 1


if __name__ == "__main__":
    sys.exit(main())
