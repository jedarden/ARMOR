#!/usr/bin/env python3
"""Workspace-local starvation watcher for the ARMOR bead workspace.

Consumes ``.beads/diagnostics/pluck-diagnostics.json`` — rewritten by
bead-rs on every ready query — and files ONE plain task bead per
starvation episode, only when the condition reproduces across two
consecutive snapshots:

    total_open_beads > 0 AND final_candidate_count == 0
    AND (an excluded bead carries none of the four park reasons
         OR open beads the snapshot does not account for exist)

A workspace whose every open bead is parked — assigned, manually
blocked, dependency-blocked or resource-conflicted — is the frontier
predicate working as intended and declares nothing (armor-90060d9b,
2026-09-13): the gate alone fired on exactly that healthy state. A
drained workspace (open == 0) and a healthy frontier (candidates > 0)
also write nothing. A single snapshot is never enough; the pair must
also be temporally adjacent (``--max-gap-seconds``, default 1h) so
"consecutive" means consecutive observations, not two reads from
different eras.

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

## Second, independent check: empty-database integrity

The starvation predicate reads a diagnostics file that bead-rs only
rewrites on a ready query, so a *wiped or empty* beads.db — where
``total_open_beads`` reads 0 too, as in the 2026-08-28 incident where the
human remedy was ``bead init`` + checkpoint restore (2134 issues) — is
invisible to it. Each cycle therefore also runs one independent
integrity check, entirely separate from the state machine above:

1. Confirm the workspace is bead-rs (``.needle.yaml`` ``bead_cli.backend``
   and the on-disk shape). Anything else files one plain task bead naming
   what was found and stops — bead-rs-shaped recovery against a bf store
   silently reinitializes it with the wrong schema.
2. Detect: ``bead list --json`` (JSONL; ``[]`` means zero records) must
   exit 0 AND yield zero issue records while
   ``.beads/checkpoint/forensic.jsonl`` still holds records. A non-zero
   exit, a lock error, or unparseable output means "skip this cycle",
   never "empty" — and a healthy workspace is never empty, because the
   list spans all statuses (closed beads included).
3. Capture ``bead doctor`` (read-only) plus the forensic record count
   into an episode record under the state dir *before* touching anything.
4. Repair, only when every guard passed: ``bead init`` then
   ``bead sync import-only --input .beads/checkpoint/forensic.jsonl
   --restore-into-empty --actor starvation-watch``. Any failure stops the
   repair, latches the generation (no retry until it changes), and files
   one task bead with the captured output.
5. Verify by property: post-restore record count > 0 and a clean
   ``bead doctor``; before/after counts go into the episode record.
6. File ONE alert per checkpoint generation, deduped through
   ``--unique-ref starvation-watch:integrity:<generation>``.

``--no-repair`` detects and alerts without repairing; ``--skip-integrity``
runs the starvation half only. The bead binary is resolved to an absolute
path because the systemd --user manager PATH on this box does not include
``~/.local/bin``.

Usage:
    starvation-watch.py [--workspace DIR] [--dry-run] [--self-test]
                        [--no-repair] [--skip-integrity]

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
import re
import shutil
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

# Integrity check (second, independent; see module docstring).
INTEGRITY_STATE_VERSION = 1
DEFAULT_ACTOR = "starvation-watch"
LIST_TIMEOUT_SECONDS = 120
DOCTOR_TIMEOUT_SECONDS = 300
INIT_TIMEOUT_SECONDS = 300
IMPORT_TIMEOUT_SECONDS = 900
# Wording only: a repair failure matching these reads as a schema/column
# problem — the wrong-CLI corruption precedent — and the filed bead says so.
SCHEMA_ERROR_MARKERS = (
    "no such column", "missing field", "unknown field", "malformed",
    "schema", "unrecognized", "invalid record", "hash mismatch",
)


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


# The per-bead fields mirror the snapshot's exclusion_criteria keys
# (has_assignee / manually_blocked / has_dependencies /
# resource_conflicts): a bead excluded for any of them is parked on
# purpose. The alert's job is the bead invisible for none of them.
PARK_REASON_FIELDS = ("assignee", "manual_blocked", "has_blockers",
                      "has_resource_conflicts")


def exclusion_is_park(entry: dict) -> bool:
    """True when this excluded bead carries at least one park reason.

    Manually-blocked beads are intentional parks, not starvation — so
    are assigned, dependency-blocked and resource-conflicted ones.
    """
    return any(entry.get(field) for field in PARK_REASON_FIELDS)


def unexplained_invisible_beads(snap: dict) -> list[dict]:
    """Open beads excluded from the frontier with no park reason at all.

    The true-positive signal: the frontier predicate skipped these beads
    without any of its four documented reasons, so no worker can reach
    them and nothing in the snapshot explains why.
    """
    return [entry for entry in snap.get("excluded_beads", [])
            if not exclusion_is_park(entry)]


def unaccounted_open_beads(snap: dict) -> int:
    """Open beads the snapshot neither reports as candidates nor lists.

    A truncated or unrecorded exclusion list must not read as "all
    parked": open beads absent from both counts are invisible for an
    unexplained reason by construction.
    """
    return max(0, snap["total_open_beads"] - snap["final_candidate_count"]
               - len(snap.get("excluded_beads", [])))


def is_starved(snap: dict) -> bool:
    """The starvation predicate: work exists but nothing is dispatchable,
    and at least one open bead is invisible for an unexplained reason.

    The gate ``total_open_beads > 0 and final_candidate_count == 0``
    stays — zero dispatchable work is the alert's charter — but a
    workspace whose every open bead is parked no longer declares: those
    exclusions are the ready-frontier predicate working as intended.
    Beyond the gate, a declaration requires an unexplained exclusion (an
    ``excluded_beads`` entry carrying none of the four park reasons) or
    open beads the snapshot does not account for at all.
    """
    if not (snap["total_open_beads"] > 0 and snap["final_candidate_count"] == 0):
        return False
    if unexplained_invisible_beads(snap):
        return True
    return unaccounted_open_beads(snap) > 0


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
    parked = sum(1 for e in cur.get("excluded_beads", []) if exclusion_is_park(e))
    parts.append(
        f"Trigger: {len(unexplained_invisible_beads(cur))} unexplained "
        f"exclusion(s) plus {unaccounted_open_beads(cur)} open bead(s) the "
        f"snapshot does not account for; the remaining {parked} excluded "
        f"bead(s) are legitimate parks (assigned / manually blocked / "
        f"dependency-blocked / resource-conflicted) and did not trigger this."
    )
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


def resolve_bead_bin(bead_bin: str) -> str:
    """Absolute path to the bead CLI.

    The systemd --user manager PATH on this box does not include
    ~/.local/bin (verified 2026-09-08), so a bare "bead" resolves in an
    interactive shell but not under the timer. Fall back to the known
    install location instead of failing every cycle with FileNotFoundError.
    """
    if os.path.sep in bead_bin:
        return bead_bin
    found = shutil.which(bead_bin)
    if found:
        return found
    fallback = Path.home() / ".local" / "bin" / bead_bin
    if fallback.is_file():
        return str(fallback)
    return bead_bin  # let subprocess report it; callers treat that as "skip"


def file_alert(bead_bin: str, title: str, body: str, ref: str, dry_run: bool) -> tuple[str, str]:
    """Create the alert bead. Returns (status, bead_id_or_message).

    status is one of: created, existing, existing_closed, dry_run, error.
    Idempotent via --unique-ref: a repeated create for the same episode
    returns existing/existing_closed instead of filing a duplicate.
    """
    cmd = [
        resolve_bead_bin(bead_bin), "create",
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


def run_bead(workspace: Path, bead_bin: str, args: list[str],
             timeout: float) -> tuple[int | None, str, str, str]:
    """Run one bead subcommand in the workspace.

    Returns (exit, stdout, stderr, error). error is non-empty when the
    binary could not be run at all or timed out — a condition distinct
    from a non-zero exit, and always a "skip this cycle", never "empty".
    """
    cmd = [resolve_bead_bin(bead_bin)] + args
    try:
        proc = subprocess.run(cmd, cwd=str(workspace), capture_output=True,
                              text=True, timeout=timeout)
    except OSError as exc:
        return None, "", "", f"cannot run {cmd[0]}: {exc}"
    except subprocess.TimeoutExpired:
        return None, "", "", f"timed out after {timeout:.0f}s"
    return proc.returncode, proc.stdout or "", proc.stderr or "", ""


def default_lister(workspace: Path, bead_bin: str) -> tuple[int | None, str, str, str]:
    # --no-auto-flush keeps detection strictly read-only: a list must never
    # publish a checkpoint as a side effect of being run by a timer.
    return run_bead(workspace, bead_bin,
                    ["list", "--json", "--limit", "999999", "--no-auto-flush"],
                    LIST_TIMEOUT_SECONDS)


def default_doctorer(workspace: Path, bead_bin: str) -> tuple[int | None, str, str, str]:
    # doctor is read-only by design unless --repair is passed (never passed here).
    return run_bead(workspace, bead_bin, ["doctor"], DOCTOR_TIMEOUT_SECONDS)


def default_repairer(workspace: Path, bead_bin: str, forensic_path: Path,
                     actor: str = DEFAULT_ACTOR) -> tuple[list[dict], bool]:
    """The documented human remedy from 2026-08-28, executed in order.

    init rebuilds the (missing/empty/wrong-schema) database; import-only
    stages, validates and atomically activates the checkpoint into it.
    Stops at the first failing step. Auto-flush stays on for these: they
    are real mutations, and the checkpoint publication after them is the
    R026 contract.
    """
    steps: list[dict] = []
    commands = (
        (["init"], INIT_TIMEOUT_SECONDS),
        (["sync", "import-only", "--input", str(forensic_path),
          "--restore-into-empty", "--actor", actor], IMPORT_TIMEOUT_SECONDS),
    )
    for args, timeout in commands:
        rc, out, err, run_err = run_bead(workspace, bead_bin, args, timeout)
        steps.append({
            "command": "bead " + " ".join(args),
            "exit": rc,
            "error": run_err,
            "stdout_tail": out[-2000:],
            "stderr_tail": err[-2000:],
        })
        if run_err or rc != 0:
            return steps, False
    return steps, True


def count_bead_list_records(stdout: str) -> tuple[int | None, str]:
    """Count issue records in `bead list --json` output.

    The output is compact JSONL (one object per line) — except for an
    empty database, which prints a literal ``[]``. Returns (None, reason)
    for anything unparseable: the caller must treat that as "skip this
    cycle", never as zero records.
    """
    count = 0
    for line in stdout.splitlines():
        line = line.strip()
        if not line or line == "[]":
            continue
        try:
            parsed = json.loads(line)
        except json.JSONDecodeError as exc:
            return None, f"{exc} at {line[:60]!r}"
        if isinstance(parsed, list):
            count += sum(1 for item in parsed if isinstance(item, dict))
        elif isinstance(parsed, dict):
            count += 1
        else:
            return None, f"unexpected JSON {type(parsed).__name__} in list output"
    return count, ""


def backend_guard(workspace: Path) -> tuple[bool, str]:
    """Confirm the workspace is bead-rs before any bead-rs-shaped repair.

    The backend is declared in .needle.yaml (``bead_cli.backend``; a bare
    top-level ``backend:`` is the older bf spelling) and echoed by the
    on-disk shape: ``.beads/config.json`` + ``.beads/checkpoint/`` is
    bead-rs, ``.beads/config.yaml`` + a flat ``issues.jsonl`` is bf.
    Running bead-rs recovery against a bf store does not fail cleanly —
    it silently reinitializes the store with the wrong schema (SEAM,
    2026-08-14) — so an unconfirmable backend counts as not-bead-rs.
    """
    beads = workspace / ".beads"
    try:
        needle = workspace / ".needle.yaml"
        if needle.is_file():
            declared = None
            in_bead_cli = False
            for raw in needle.read_text(encoding="utf-8", errors="replace").splitlines():
                if re.match(r"^bead_cli:\s*$", raw):
                    in_bead_cli = True
                    continue
                if re.match(r"^backend:\s*(\S+)\s*$", raw):
                    declared = re.match(r"^backend:\s*(\S+)\s*$", raw).group(1).strip("'\"")
                elif in_bead_cli and re.match(r"^\s", raw):
                    m = re.match(r"^\s*backend:\s*(\S+)\s*$", raw)
                    if m:
                        declared = m.group(1).strip("'\"")
                elif in_bead_cli:
                    in_bead_cli = False
            if declared is not None and declared != "bead-rs":
                return False, f".needle.yaml declares bead backend {declared!r}, not bead-rs"

        config_json = beads / "config.json"
        bf_config = beads / "config.yaml"
        bf_issues = beads / "issues.jsonl"
        if bf_config.is_file() or bf_issues.is_file():
            return False, ("bf-shaped store found (.beads/config.yaml and/or "
                           ".beads/issues.jsonl); bead-rs recovery must not run here")
        if config_json.is_file():
            json.loads(config_json.read_text(encoding="utf-8"))  # shape tell, must parse
            return True, "bead-rs confirmed (.beads/config.json parses; no bf-shaped files)"
        if (workspace / ".needle.yaml").is_file():
            return False, ".needle.yaml present but declares no bead backend"
        return False, "no .beads/config.json and no .needle.yaml backend declaration"
    except (OSError, json.JSONDecodeError) as exc:
        return False, f"backend could not be confirmed: {exc}"


def forensic_record_count(path: Path) -> int:
    try:
        return sum(1 for line in path.read_text(encoding="utf-8").splitlines() if line.strip())
    except OSError:
        return 0


def checkpoint_generation(checkpoint_dir: Path, forensic: Path) -> tuple[str, str]:
    """(generation, source). Falls back to a content hash when the
    checkpoint manifest is missing or unreadable, so alert dedup still
    keys on one stable value per checkpoint state."""
    current = checkpoint_dir / "current.json"
    try:
        manifest = json.loads(current.read_text(encoding="utf-8"))
        gen = manifest.get("generation_id")
        if isinstance(gen, str) and gen:
            return gen, "checkpoint/current.json:generation_id"
    except (OSError, json.JSONDecodeError):
        pass
    try:
        digest = hashlib.sha256(forensic.read_bytes()).hexdigest()[:16]
        return f"sha256-{digest}", "sha256(forensic.jsonl)"
    except OSError:
        return "unknown", "no checkpoint readable"


def doctor_is_clean(rc: int | None, stdout: str) -> bool:
    """doctor exits 0 on healthy scopes; WARN lines exist on the live
    healthy workspace (e.g. manually-blocked beads, advisory secret
    findings), so only hard failure lines disqualify."""
    if rc != 0:
        return False
    return not any(line.startswith(("FAIL", "ERROR")) for line in stdout.splitlines())


def schema_shaped_failure(steps: list[dict]) -> str | None:
    for step in steps:
        text = (step.get("stderr_tail", "") + step.get("stdout_tail", "")).lower()
        if step.get("error"):
            text += step["error"].lower()
        for marker in SCHEMA_ERROR_MARKERS:
            if marker in text:
                return marker
    return None


def load_integrity_state(path: Path) -> dict:
    try:
        state = json.loads(path.read_text(encoding="utf-8"))
        if isinstance(state, dict) and state.get("version") == INTEGRITY_STATE_VERSION:
            return state
    except (OSError, json.JSONDecodeError):
        pass
    return {"version": INTEGRITY_STATE_VERSION, "last_gen": None,
            "last_outcome": None, "repair_failed": None}


def episode_record_path(state_path: Path, generation: str) -> Path:
    safe = re.sub(r"[^A-Za-z0-9._-]", "_", generation)
    return state_path.with_name(f"{state_path.stem}-episode-{safe}.json")


def write_episode_record(path: Path, record: dict) -> None:
    record["updated_at"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    try:
        save_state(path, record)
    except OSError as exc:
        print(f"integrity-watch: could not write episode record {path}: {exc}",
              file=sys.stderr)


def one_line(text: str, limit: int = 160) -> str:
    return " ".join(text.split())[:limit]


def run_integrity_check(workspace: str, bead_bin: str, state_path: Path,
                        dry_run: bool = False, no_repair: bool = False,
                        log=None, filer=None, lister=None, doctorer=None,
                        repairer=None, guard=None) -> str:
    """One integrity cycle: is the database empty while the checkpoint
    holds work — and if so, restore it. Fully independent of the
    starvation state machine (separate state file, separate predicate);
    every failure mode decays to "skip this cycle", never to "empty".

    ``lister``/``doctorer``/``repairer``/``guard``/``filer`` override the
    real bead invocations (--self-test); they match the shapes of the
    ``default_*`` functions and ``file_alert``.
    """
    if log is None:
        log = lambda msg: print(msg, file=sys.stderr)  # noqa: E731
    ws = Path(workspace)
    beads_dir = ws / ".beads"
    checkpoint_dir = beads_dir / "checkpoint"
    forensic = checkpoint_dir / "forensic.jsonl"
    filer = filer if filer is not None else file_alert
    lister = lister if lister is not None else default_lister
    doctorer = doctorer if doctorer is not None else default_doctorer
    repairer = repairer if repairer is not None else default_repairer
    guard = guard if guard is not None else backend_guard

    def file_integrity_alert(title: str, body: str, ref: str) -> tuple[str, str]:
        return filer(bead_bin, title, body, ref, dry_run)

    if not beads_dir.is_dir():
        log("integrity-watch: no .beads/ directory; nothing to check")
        return "integrity:no-workspace"

    ok, backend = guard(ws)
    if not ok:
        # Wrong-CLI recovery corrupts the other tool's store, so the repair
        # is withheld and a human is handed the finding instead.
        file_integrity_alert(
            f"Bead integrity: backend unconfirmed at {ws.name}, bead-rs recovery withheld",
            f"The integrity watcher declined to run bead-rs-shaped recovery on\n"
            f"`{ws}` because the backend could not be confirmed as bead-rs.\n\n"
            f"What was found: {backend}\n\n"
            f"Nothing was modified. Confirm the backend per the bead-rs rollout\n"
            f"notes (.needle.yaml bead_cli.backend plus the on-disk shape) before\n"
            f"any recovery is attempted here.",
            f"integrity-backend:{hashlib.sha256(str(ws).encode()).hexdigest()[:16]}")
        log(f"integrity-watch: backend unconfirmed ({backend}); repair withheld")
        return "integrity:backend-unconfirmed"

    if not checkpoint_dir.is_dir():
        log("integrity-watch: no .beads/checkpoint/; emptiness would be unrestorable, skipping")
        return "integrity:no-checkpoint"

    records = forensic_record_count(forensic)
    generation, gen_source = checkpoint_generation(checkpoint_dir, forensic)
    state = load_integrity_state(state_path)

    rc, out, err, run_err = lister(ws, bead_bin)
    if run_err:
        log(f"integrity-watch: skipped this cycle (bead list unavailable: {run_err})")
        return "integrity:skipped(list-unavailable)"
    if rc != 0:
        log(f"integrity-watch: skipped this cycle "
            f"(bead list exited {rc}: {one_line(err)})")
        return f"integrity:skipped(list-exit-{rc})"
    count, parse_err = count_bead_list_records(out)
    if count is None:
        log(f"integrity-watch: skipped this cycle (unparseable list output: {parse_err})")
        return "integrity:skipped(unparseable)"

    if count > 0:
        state["last_gen"] = generation
        state["last_outcome"] = f"healthy({count})"
        if not dry_run:
            save_state(state_path, state)
        log(f"integrity-watch: healthy ({count} issues; checkpoint {generation} "
            f"holds {records} records)")
        return f"integrity:healthy({count})"

    # The database reads zero records. If the checkpoint holds nothing
    # either, there is nothing to restore and no episode.
    if records == 0:
        log("integrity-watch: database empty but the checkpoint holds no records; "
            "nothing to restore")
        return "integrity:empty-but-nothing-to-restore"

    # ---- Episode: empty database over a populated checkpoint ----
    episode_path = episode_record_path(state_path, generation)
    record = {
        "workspace": str(ws),
        "generation": generation,
        "generation_source": gen_source,
        "detected_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "checkpoint_records": records,
        "backend": backend,
        "before_issue_count": 0,
        "bead_list_exit": rc,
        "bead_list_stderr_tail": err[-2000:],
    }
    if state.get("repair_failed") == generation:
        log(f"integrity-watch: database still empty, but repair already failed for "
            f"{generation}; not retrying (episode record: {episode_path.name})")
        return "integrity:repair-failed(latched)"

    # Diagnostics BEFORE any repair attempt (bead doctor is read-only).
    doctor_rc, doctor_out, doctor_err, doctor_run_err = doctorer(ws, bead_bin)
    record["doctor_before"] = {
        "exit": doctor_rc, "error": doctor_run_err,
        "stdout": doctor_out, "stderr_tail": doctor_err[-2000:],
    }
    if not dry_run:  # --dry-run contract: no state writes at all
        write_episode_record(episode_path, record)
    log(f"integrity-watch: database EMPTY ({records} checkpoint records, "
        f"{generation}); doctor exit {doctor_rc}; episode record {episode_path.name}")

    if dry_run:
        log("integrity-watch: [dry-run] would run: bead init; bead sync import-only "
            "--input .beads/checkpoint/forensic.jsonl --restore-into-empty "
            f"--actor {DEFAULT_ACTOR}")
        return "integrity:dry-run(would-repair)"

    if no_repair:
        status, detail = file_integrity_alert(
            f"Bead database empty in {ws.name}: {records} checkpoint records, "
            f"auto-repair disabled",
            f"`bead list --json` in `{ws}` exited 0 and returned zero issue records\n"
            f"while `.beads/checkpoint/forensic.jsonl` still holds {records} records.\n\n"
            f"Checkpoint generation: {generation} (from {gen_source})\n"
            f"Backend: {backend}\n"
            f"`bead doctor` (read-only) exited {doctor_rc}; full output is in the\n"
            f"episode record {episode_path}\n\n"
            f"The watcher runs with --no-repair, so it did not attempt the restore.\n"
            f"Human remedy, from the 2026-08-28 incident:\n"
            f"    bead init\n"
            f"    bead sync import-only --input .beads/checkpoint/forensic.jsonl \\\n"
            f"      --restore-into-empty --actor <you>",
            f"integrity:{generation}")
        log(f"integrity-watch: --no-repair set; alert filing status {status} {detail}")
        return f"integrity:alerted-no-repair({status})"

    steps, repair_ok = repairer(ws, bead_bin, forensic)
    record["repair_steps"] = steps
    if not repair_ok:
        state["repair_failed"] = generation
        state["last_gen"], state["last_outcome"] = generation, "repair-failed"
        save_state(state_path, state)
        write_episode_record(episode_path, record)
        marker = schema_shaped_failure(steps)
        flavor = (f"reads as a schema/column problem ({marker}) — do NOT apply the "
                  f"other CLI's recovery recipes to it" if marker
                  else "does not read as a schema/column error; diagnose before retrying")
        file_integrity_alert(
            f"Bead restore FAILED in {ws.name} (checkpoint {generation})",
            f"The empty-database restore in `{ws}` failed and has been latched off\n"
            f"(no automatic retry until the checkpoint generation changes).\n\n"
            f"Checkpoint generation: {generation} (from {gen_source})\n"
            f"Checkpoint records: {records}\n"
            f"Failure {flavor}\n\n"
            f"## Captured repair output\n\n"
            f"```json\n{json.dumps(steps, indent=2)}\n```\n\n"
            f"Episode record: {episode_path}",
            f"integrity-repair-failed:{generation}")
        log("integrity-watch: repair FAILED; latched and beaded")
        return "integrity:repair-failed"

    # Verify by property, never by reading data back for its own sake.
    rc2, out2, err2, run_err2 = lister(ws, bead_bin)
    after, after_parse = count_bead_list_records(out2) if not run_err2 and rc2 == 0 \
        else (None, f"list unavailable (exit {rc2}): {run_err2 or one_line(err2)}")
    doctor2_rc, doctor2_out, doctor2_err, doctor2_run_err = doctorer(ws, bead_bin)
    record["after_issue_count"] = after
    record["doctor_after"] = {
        "exit": doctor2_rc, "error": doctor2_run_err, "stdout": doctor2_out,
        "stderr_tail": doctor2_err[-2000:],
    }
    if after is None or after == 0 or not doctor_is_clean(doctor2_rc, doctor2_out):
        state["repair_failed"] = generation
        state["last_gen"], state["last_outcome"] = generation, "unverified"
        save_state(state_path, state)
        write_episode_record(episode_path, record)
        file_integrity_alert(
            f"Bead restore finished but UNVERIFIED in {ws.name} "
            f"(checkpoint {generation})",
            f"The restore in `{ws}` completed without a bead-level error, but the\n"
            f"property check did not pass:\n"
            f"- post-restore `bead list --json` records: {after!r}\n"
            f"- post-restore `bead doctor`: exit {doctor2_rc}, clean = "
            f"{doctor_is_clean(doctor2_rc, doctor2_out)}\n\n"
            f"Before/after counts and both doctor outputs are in the episode\n"
            f"record: {episode_path}\n\n"
            f"Repair has been latched off until the checkpoint generation changes.",
            f"integrity-repair-failed:{generation}")
        log(f"integrity-watch: repair ran but UNVERIFIED (after={after}, "
            f"doctor exit {doctor2_rc}); latched and beaded")
        return "integrity:unverified"

    state["last_gen"], state["last_outcome"] = generation, f"restored({after})"
    state.pop("repair_failed", None)
    save_state(state_path, state)
    write_episode_record(episode_path, record)
    file_integrity_alert(
        f"Bead database was empty: restored {after} issues from checkpoint "
        f"{generation} in {ws.name}",
        f"`bead list --json` in `{ws}` exited 0 and returned zero issue records\n"
        f"while `.beads/checkpoint/forensic.jsonl` still held {records} records —\n"
        f"the same shape as the 2026-08-28 incident (empty beads.db, populated\n"
        f"checkpoint, invisible to the starvation predicate because open==0 too).\n\n"
        f"The watcher restored the workspace from the checkpoint:\n"
        f"- backend: {backend}\n"
        f"- checkpoint generation: {generation} (from {gen_source})\n"
        f"- issues before: 0\n- issues after: {after}\n"
        f"- pre-repair `bead doctor` exit: {doctor_rc} (full output in the episode\n"
        f"  record)\n"
        f"- post-restore `bead doctor`: exit {doctor2_rc}, clean = True\n\n"
        f"Episode record with both doctor outputs and each repair step:\n"
        f"{episode_path}",
        f"integrity:{generation}")
    log(f"integrity-watch: restored {after} issues from {generation} "
        f"(was 0); alert filed")
    return f"integrity:restored({after})"


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
        {"bead_id": "armor-ccc", "title": "mystery work", "priority": 2,
         "assignee": None, "manual_blocked": False, "has_blockers": False,
         "blocker_count": 0, "has_resource_conflicts": False, "conflict_count": 0},
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
    # The fixture mixes parked beads (aaa, bbb) with one unexplained bead
    # (ccc): the declaration must fire on ccc, not on the parks.
    out, f = cycle([snap(t, 3, 1, excluded), snap(seq(1), 3, 0, excluded),
                    snap(seq(2), 3, 0, excluded)])
    body = f[0]["body"] if f else ""
    check("alert filed for body inspection", len(f) == 1)
    check("body embeds current snapshot verbatim",
          json.dumps(snap(seq(2), 3, 0, excluded), indent=2, sort_keys=True) in body)
    check("body embeds previous snapshot verbatim",
          json.dumps(snap(seq(1), 3, 0, excluded), indent=2, sort_keys=True) in body)
    check("body classifies dependency-blocked bead",
          "armor-aaa (P1) blocked work — dependency-blocked (2 blocker(s))" in body)
    check("body classifies parked bead with all reasons",
          "armor-bbb (P0) parked work — assigned to someone; "
          "manually blocked (policy park); resource conflict (1)" in body)
    check("body classifies unexplained bead as unclassified",
          "armor-ccc (P2) mystery work — unclassified" in body)
    check("body states the parked-vs-unexplained trigger split",
          "1 unexplained exclusion(s) plus 0 open bead(s) the snapshot does "
          "not account for; the remaining 2 excluded bead(s) are legitimate "
          "parks" in body)
    check("title carries counts, never a bare 'Open beads: 0' style alert",
          "3 open beads, 0 ready candidates" in f[0]["title"])

    # 10: the ref handed to the filer is the episode-scoped compact
    # timestamp; file_alert composes the namespace onto it.
    check("unique-ref is episode-scoped and colon-free",
          f[0]["ref"] == "starvation-watch:20260908T100100Z"
          or f[0]["ref"] == "20260908T100100Z")

    # 10b-10e: parked beads are not starvation (armor-90060d9b). The four
    # park reasons are intentional exclusions; only an unexplained
    # invisible bead — or open beads the snapshot fails to account for —
    # turns an empty frontier into an alert.
    def entry(bead_id, title, **reasons):
        base = {"assignee": None, "manual_blocked": False, "has_blockers": False,
                "blocker_count": 0, "has_resource_conflicts": False,
                "conflict_count": 0}
        base.update(reasons)
        return {"bead_id": bead_id, "title": title, "priority": 1, **base}

    # 10b: the exact false positive from the 2026-09-13 live state — every
    # open bead parked, one per reason, across consecutive snapshots.
    all_parked = [
        entry("armor-pp1", "assigned work", assignee="w1"),
        entry("armor-pp2", "manual park", manual_blocked=True),
        entry("armor-pp3", "dep-blocked work", has_blockers=True, blocker_count=4),
        entry("armor-pp4", "conflicted work", has_resource_conflicts=True,
              conflict_count=2),
    ]
    check("predicate: every park reason classifies as park",
          all(exclusion_is_park(e) for e in all_parked))
    out, f = cycle([snap(t, 4, 0, all_parked), snap(seq(1), 4, 0, all_parked),
                    snap(seq(2), 4, 0, all_parked)])
    check("all-parked workspace files nothing", not f)
    check("all-parked workspace never reports starvation",
          not any("starved" in o for o in out))

    # 10c: the true positive — one bead invisible for none of the four
    # reasons, among otherwise legitimate parks.
    mystery = entry("armor-uu1", "unexplained work")
    mixed = all_parked + [mystery]
    check("predicate: reason-less exclusion is not a park",
          not exclusion_is_park(mystery))
    out, f = cycle([snap(t, 5, 0, mixed), snap(seq(1), 5, 0, mixed),
                    snap(seq(2), 5, 0, mixed)])
    check("one unexplained invisible bead declares", len(f) == 1)
    check("declared body names the unexplained bead",
          bool(f) and "armor-uu1" in f[0]["body"])

    # 10d: open beads the snapshot neither lists as excluded nor counts as
    # candidates are unexplained by construction (truncated exclusion list
    # must not read as "all parked").
    check("predicate: unaccounted open beads are counted",
          unaccounted_open_beads(snap(t, 9, 0, all_parked)) == 5)
    out, f = cycle([snap(t, 9, 0, all_parked), snap(seq(1), 9, 0, all_parked)])
    check("unaccounted open beads declare", len(f) == 1)

    # 10e: a healthy frontier is never starvation, even with an unexplained
    # exclusion present — the alert's charter is zero dispatchable work,
    # and paging while workers hold candidates would recreate the false
    # positives this predicate change removes (e.g. a deferred-label park
    # the snapshot records without a reason).
    out, f = cycle([snap(t, 5, 2, mixed), snap(seq(1), 5, 2, mixed)])
    check("healthy frontier with an unexplained exclusion files nothing", not f)

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
    check("filer invokes bead create",
          recorded["cmd"][1] == "create"
          and os.path.basename(recorded["cmd"][0]) == "bead")
    check("filer passes namespaced unique-ref",
          f"{UNIQUE_REF_NAMESPACE}:20260908T100000Z" in recorded["cmd"])
    check("filer parses fresh-create id", status == "created" and detail == "armor-1234")

    # --- integrity check: the second, independent half ---

    # 12: `bead list --json` output counting. An empty database prints a
    # literal `[]` — naive non-empty-line counting would read that as one
    # record and never fire; unparseable output must read as "skip", not 0.
    check("empty-db list output `[]` counts as zero records",
          count_bead_list_records("[]\n") == (0, ""))
    check("JSONL lines count as records",
          count_bead_list_records('{"id":"a"}\n\n{"id":"b"}\n{"id":"c"}\n') == (3, ""))
    check("a JSON-array line counts its objects",
          count_bead_list_records("[{},{},{}]") == (3, ""))
    out = count_bead_list_records("bead: database is locked\n")
    check("unparseable list output is never a count", out[0] is None)
    # Verified live 2026-09-13 against the installed bead-rs binary: a
    # freshly `bead init`-ed store answers `bead list --json` with exit 0
    # and the two bytes `[]` — no trailing newline. An empty stdout is the
    # same signal (exit 0 plus zero records), while a JSON `null` line is
    # not a record count and must read as "skip this cycle".
    check("live empty-store shape `[]` (no newline) counts as zero",
          count_bead_list_records("[]") == (0, ""))
    check("empty stdout with exit 0 counts as zero records",
          count_bead_list_records("") == (0, ""))
    check("JSON null line is unparseable, never zero",
          count_bead_list_records("null\n")[0] is None)

    # 13: doctor cleanliness — WARN lines exist on the healthy live
    # workspace; only hard failures disqualify.
    check("doctor with warnings is clean", doctor_is_clean(0, "OK a: b\nWARN c: d\n"))
    check("doctor FAIL line is not clean", not doctor_is_clean(0, "FAIL a: b\n"))
    check("doctor non-zero exit is not clean", not doctor_is_clean(1, "OK a: b\n"))

    # 14: backend guard against synthetic stores (never the live workspace).
    with tempfile.TemporaryDirectory() as tmp:
        ws = Path(tmp) / "ws"
        (ws / ".beads").mkdir(parents=True)
        ok, _ = backend_guard(ws)
        check("bare .beads dir is not a confirmable backend", ok is False)
        (ws / ".beads" / "config.json").write_text('{"prefix":"x"}', encoding="utf-8")
        ok, _ = backend_guard(ws)
        check("bead-rs shape (config.json) confirms", ok is True)
        (ws / ".beads" / "config.yaml").write_text("backend: bf\n", encoding="utf-8")
        ok, msg = backend_guard(ws)
        check("bf-shaped file alongside config.json is refused", ok is False)
        (ws / ".beads" / "config.yaml").unlink()
        (ws / ".needle.yaml").write_text("bead_cli:\n  backend: bf\n", encoding="utf-8")
        ok, msg = backend_guard(ws)
        check("needle.yaml bf declaration is refused", ok is False and "bf" in msg)
        (ws / ".needle.yaml").write_text("bead_cli:\n  backend: bead-rs\n", encoding="utf-8")
        ok, _ = backend_guard(ws)
        check("needle.yaml bead-rs declaration confirms", ok is True)

    # 15+: full cycles against fakes. The harness holds one workspace with
    # a populated checkpoint and a mutable in-memory "database".
    def integrity_cycles(configs, generation="gen-test0001", records=12219):
        results = []
        with tempfile.TemporaryDirectory() as tmp:
            state_path = Path(tmp) / "integrity.json"
            ws = Path(tmp) / "ws"
            forensic = ws / ".beads" / "checkpoint" / "forensic.jsonl"
            forensic.parent.mkdir(parents=True)
            forensic.write_text('{"r":1}\n' * records, encoding="utf-8")
            # The manifest is what makes the generation readable as
            # "gen-test0001"; without it checkpoint_generation falls back to
            # hashing forensic.jsonl and every dedup assertion below misses.
            (ws / ".beads" / "checkpoint" / "current.json").write_text(
                json.dumps({"generation_id": generation}), encoding="utf-8")
            holder = {"issues": 0, "list_rc": 0, "list_out": None}
            filings: list[dict] = []
            repair_calls = [0]

            cfg = dict(configs[0])

            def filer(bead_bin="", title="", body="", ref="", dry_run=False,
                      filings=filings):
                filings.append({"title": title, "body": body, "ref": ref})
                return "created", "armor-fake0002"

            def guard(ws_arg, holder=holder):
                return (holder.get("backend_ok", True),
                        holder.get("backend_msg", "bead-rs confirmed"))

            def lister(ws_arg, bead_bin, holder=holder, cfg=cfg):
                if holder["list_rc"] != 0:
                    return holder["list_rc"], "", "boom", ""
                if holder["list_out"] is not None:
                    return 0, holder["list_out"], "", ""
                if holder["issues"] > 0:
                    return 0, "\n".join('{"id":"x"}' for _ in range(holder["issues"])), "", ""
                return 0, "[]\n", "", ""

            def doctorer(ws_arg, bead_bin, cfg=cfg):
                return cfg.get("doctor_rc", 0), "OK schema_validity: fine\n", "", ""

            def repairer(ws_arg, bead_bin, forensic_arg, holder=holder, cfg=cfg,
                         repair_calls=repair_calls):
                repair_calls[0] += 1
                if not cfg.get("repair_ok", True):
                    return [{"command": "bead init", "exit": 1, "error": "",
                             "stdout_tail": "",
                             "stderr_tail": "bead: Internal error: no such column: bogus"}], False
                holder["issues"] = cfg.get("repaired_issues", 2134)
                return [{"command": "bead init", "exit": 0, "error": "",
                         "stdout_tail": "", "stderr_tail": ""},
                        {"command": "bead sync import-only ...", "exit": 0, "error": "",
                         "stdout_tail": "", "stderr_tail": ""}], True

            for raw in configs:
                cfg.clear()
                cfg.update(raw)
                holder.update({k: v for k, v in raw.items()
                               if k in ("list_rc", "list_out", "backend_ok", "backend_msg")})
                if "issues" in raw:
                    holder["issues"] = raw["issues"]
                outcome = run_integrity_check(
                    str(ws), "bead", state_path,
                    dry_run=raw.get("dry_run", False),
                    no_repair=raw.get("no_repair", False),
                    log=lambda *_: None, filer=filer, lister=lister,
                    doctorer=doctorer, repairer=repairer, guard=guard)
                results.append({"outcome": outcome, "filings": list(filings),
                                "repair_calls": repair_calls[0]})
        return results

    seq = integrity_cycles([
        {"issues": 5},
        {"issues": 0},
        {},  # post-restore: the repair flipped the in-memory database
    ])
    check("integrity: healthy workspace files nothing", not seq[0]["filings"])
    check("integrity: healthy outcome carries the count",
          seq[0]["outcome"] == "integrity:healthy(5)")
    check("integrity: empty db is restored from checkpoint",
          seq[1]["outcome"] == "integrity:restored(2134)")
    check("integrity: exactly one alert per generation, keyed on it",
          len(seq[1]["filings"]) == 1
          and seq[1]["filings"][0]["ref"] == "integrity:gen-test0001")
    check("integrity: alert carries before/after counts",
          "issues before: 0" in seq[1]["filings"][0]["body"]
          and "issues after: 2134" in seq[1]["filings"][0]["body"])
    check("integrity: post-restore cycle is healthy and quiet",
          seq[2]["outcome"] == "integrity:healthy(2134)"
          and len(seq[2]["filings"]) == 1 and seq[2]["repair_calls"] == 1)

    seq = integrity_cycles([{"issues": 0, "list_rc": 1}, {"list_rc": 0, "list_out": "junk\n"}])
    check("integrity: non-zero list exit skips, never declares empty",
          seq[0]["outcome"] == "integrity:skipped(list-exit-1)"
          and not seq[0]["filings"] and seq[0]["repair_calls"] == 0)
    check("integrity: unparseable list output skips",
          seq[1]["outcome"] == "integrity:skipped(unparseable)" and not seq[1]["filings"])

    seq = integrity_cycles([{"issues": 0, "backend_ok": False,
                             "backend_msg": "bf-shaped store found"}])
    check("integrity: unconfirmed backend files one bead naming it and never repairs",
          seq[0]["outcome"] == "integrity:backend-unconfirmed"
          and seq[0]["repair_calls"] == 0
          and seq[0]["filings"][0]["ref"].startswith("integrity-backend:")
          and "bf-shaped" in seq[0]["filings"][0]["body"])

    with tempfile.TemporaryDirectory() as tmp:  # empty checkpoint = nothing to restore
        state_path = Path(tmp) / "i.json"
        ws = Path(tmp) / "ws"
        (ws / ".beads" / "checkpoint").mkdir(parents=True)
        (ws / ".beads" / "checkpoint" / "forensic.jsonl").write_text("", encoding="utf-8")
        outcome = run_integrity_check(str(ws), "bead", state_path,
                                      log=lambda *_: None, lister=lambda *_: (0, "[]\n", "", ""),
                                      doctorer=lambda *_: (0, "OK\n", "", ""),
                                      repairer=lambda *_: ([], True),
                                      guard=lambda *_: (True, "bead-rs confirmed"),
                                      filer=lambda *_: ("created", ""))
    check("integrity: empty db over an empty checkpoint is not an episode",
          outcome == "integrity:empty-but-nothing-to-restore")

    seq = integrity_cycles([{"issues": 0, "repair_ok": False}, {"issues": 0}])
    check("integrity: failed repair files one bead with captured output",
          seq[0]["outcome"] == "integrity:repair-failed"
          and seq[0]["filings"][0]["ref"] == "integrity-repair-failed:gen-test0001"
          and "no such column" in seq[0]["filings"][0]["body"])
    check("integrity: failed repair is latched, not retried",
          seq[1]["outcome"] == "integrity:repair-failed(latched)"
          and seq[1]["repair_calls"] == 1 and len(seq[1]["filings"]) == 1)

    seq = integrity_cycles([{"issues": 0, "repaired_issues": 0}, {"issues": 0}])
    check("integrity: restore that fails verification is latched and beaded",
          seq[0]["outcome"] == "integrity:unverified"
          and seq[0]["filings"][0]["ref"] == "integrity-repair-failed:gen-test0001"
          and seq[1]["outcome"] == "integrity:repair-failed(latched)"
          and seq[1]["repair_calls"] == 1)

    seq = integrity_cycles([{"issues": 0, "dry_run": True}])
    check("integrity: dry-run repairs nothing and files nothing",
          seq[0]["outcome"] == "integrity:dry-run(would-repair)"
          and seq[0]["repair_calls"] == 0 and not seq[0]["filings"])

    seq = integrity_cycles([{"issues": 0, "no_repair": True}])
    check("integrity: --no-repair alerts without repairing",
          seq[0]["outcome"].startswith("integrity:alerted-no-repair")
          and seq[0]["repair_calls"] == 0
          and seq[0]["filings"][0]["ref"] == "integrity:gen-test0001")

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
    parser.add_argument("--integrity-state-file",
                        help="integrity-check state path (default: XDG state dir, "
                             "sibling of the starvation state file)")
    parser.add_argument("--no-repair", action="store_true",
                        help="integrity check: detect and alert, never auto-repair")
    parser.add_argument("--skip-integrity", action="store_true",
                        help="run only the starvation half (pre-integrity behavior)")
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

    # Integrity runs first so a same-cycle restore gives the starvation
    # half a live database to evaluate. Its failures never take the
    # starvation half down: any crash here is logged and swallowed.
    integrity_outcome = "skipped"
    if not args.skip_integrity:
        integrity_state = Path(args.integrity_state_file) if args.integrity_state_file \
            else state_path.with_name(state_path.stem + "-integrity.json")
        try:
            integrity_outcome = run_integrity_check(
                workspace, args.bead_bin, integrity_state,
                dry_run=args.dry_run, no_repair=args.no_repair)
        except Exception as exc:  # noqa: BLE001 - the timer must stay quiet
            print(f"starvation-watch: integrity check crashed: {exc!r}", file=sys.stderr)
            integrity_outcome = "crashed"

    try:
        outcome = run_cycle(workspace, diagnostics, state_path,
                            args.bead_bin, args.max_gap_seconds, args.dry_run)
    except SnapshotError as exc:
        print(f"starvation-watch: no cycle this run: {exc}", file=sys.stderr)
        return 0  # transient (file mid-rewrite or absent) is not a failure
    # Integrity outcomes report through the journal and deduped beads; only
    # a starvation filing error flips the unit's exit status.
    return 0 if not outcome.startswith("error") else 1


if __name__ == "__main__":
    sys.exit(main())
