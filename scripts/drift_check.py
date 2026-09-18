#!/usr/bin/env python3
"""
ARMOR Drift Check — running/deployed version vs approved release, with
deduplicated alert emission.

Implements the live half of docs/drift-check.md ("Version Probe"): enumerate
ARMOR deployments, compare each deployed image tag against the approved
latest release, classify every deployment, and optionally file ONE alert
bead per distinct drift fingerprint via `bead create --unique-ref`.

Every deployment is classified into exactly one state:

- ``current``     deployed tag == latest approved version tag, or close
                  enough that no documented threshold is exceeded
                  (routine version bumps are not drift)
- ``stale``       parseable version behind latest beyond a threshold
                  (releases_behind >= --releases-threshold,
                  days_behind >= --days-threshold) or missing a
                  correctness-labelled release
- ``mismatched``  the deployed image is not the approved release in a way
                  version math cannot express: a non-version tag (git SHA,
                  ``latest``), or a live probe disagreeing with the
                  declared manifest tag
- ``unavailable`` the deployment could not be verified at all: the probe
                  failed, or a cluster listed in the configuration has no
                  ARMOR manifest in declarative-config

Alerting is deduplicated: the fingerprint of the non-current subset is a
stable hash of (cluster, image_type, state, deployed_tag, latest_tag), so
unchanged drift filed with ``--emit-bead`` returns ``EXISTING`` from the
bead CLI instead of minting a second alert bead. A changed fleet picture
hashes differently and files fresh.

Exit codes (docs/drift-check.md):
    0  all deployments current
    1  drift detected (stale / mismatched / unavailable present)
    2  error (bad arguments, missing input, fetcher failure)

Usage:
    python3 scripts/drift_check.py --json
    python3 scripts/drift_check.py --releases-file /tmp/releases.json
    python3 scripts/drift_check.py --probe-url iad-kalshi=https://armor.example.com/version
    python3 scripts/drift_check.py --emit-bead --dry-run
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

STATE_CURRENT = "current"
STATE_STALE = "stale"
STATE_MISMATCHED = "mismatched"
STATE_UNAVAILABLE = "unavailable"

DRIFT_STATES = (STATE_STALE, STATE_MISMATCHED, STATE_UNAVAILABLE)

UNIQUE_REF_NAMESPACE = "drift-check"
ALERT_LABEL = "armor-drift-check"
ALERT_PRIORITY = "2"

# Full match on the documented auto-version shape, with an optional
# digest pin (`0.1.1957@sha256:…` — the manifest convention, read as its
# version). Bare git SHAs and `latest` deliberately do not parse (they
# become `mismatched`, not `stale`).
VERSION_RE = re.compile(r"^v?0\.1\.(\d+)(?:@sha256:[0-9a-fA-F]{64})?$")
SERVER_HEADER_RE = re.compile(r"ARMOR/(v?0\.1\.\d+\S*)")


class ProbeError(RuntimeError):
    """A live /version probe could not produce a running tag."""


def parse_version(tag: str) -> Optional[int]:
    """Return the auto-version number for a tag, or None for non-versions."""
    if not tag:
        return None
    match = VERSION_RE.match(tag.strip())
    if match:
        return int(match.group(1))
    return None


def parse_ts(value: str) -> Optional[datetime]:
    """Parse an ISO timestamp, tolerating 'Z' and naive (assumed UTC)."""
    if not value:
        return None
    try:
        dt = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def latest_release(releases: List[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    """Pick the approved latest release: highest version, newest date as fallback."""
    versioned = [(parse_version(r.get("tag", "")), r) for r in releases]
    versioned = [(v, r) for v, r in versioned if v is not None]
    if versioned:
        return max(versioned, key=lambda vr: vr[0])[1]
    dated = [(parse_ts(r.get("published_at", "")), r) for r in releases]
    dated = [(t, r) for t, r in dated if t is not None]
    if dated:
        return max(dated, key=lambda tr: tr[0])[1]
    return None


def releases_behind(deployed_version: Optional[int],
                    releases: List[Dict[str, Any]]) -> Optional[int]:
    """Count approved releases newer than the deployed version."""
    if deployed_version is None:
        return None
    behind = sum(
        1 for r in releases
        if (v := parse_version(r.get("tag", ""))) is not None and v > deployed_version
    )
    return behind


def days_behind(deployed_version: Optional[int],
                releases: List[Dict[str, Any]]) -> Optional[int]:
    """Whole days between the deployed release's date and the latest release's."""
    if deployed_version is None:
        return None
    deployed_ts = None
    latest_ts = None
    for r in releases:
        ts = parse_ts(r.get("published_at", ""))
        v = parse_version(r.get("tag", ""))
        if ts is None:
            continue
        if latest_ts is None or ts > latest_ts:
            latest_ts = ts
        if v == deployed_version and deployed_ts is None:
            deployed_ts = ts
    if deployed_ts is None or latest_ts is None:
        return None
    delta = (latest_ts - deployed_ts).days
    return max(delta, 0)


def missed_correctness(deployed_version: Optional[int],
                       releases: List[Dict[str, Any]]) -> List[str]:
    """Correctness-labelled release tags newer than the deployed version."""
    if deployed_version is None:
        return []
    return sorted(
        r.get("tag", "") for r in releases
        if r.get("is_correctness")
        and (v := parse_version(r.get("tag", ""))) is not None
        and v > deployed_version
    )


def classify(deployment: Dict[str, Any],
             releases: List[Dict[str, Any]],
             releases_threshold: int,
             days_threshold: int,
             running_tag: Optional[str] = None,
             probe_error: Optional[str] = None) -> Dict[str, Any]:
    """Classify one deployment into current / stale / mismatched / unavailable."""
    cluster = deployment.get("cluster", "unknown")
    image_type = deployment.get("image_type", "armor")
    deployed_tag = deployment.get("image_tag") or ""
    report = {
        "cluster": cluster,
        "image_type": image_type,
        "deployed_tag": deployed_tag,
        "running_tag": running_tag,
        "state": STATE_UNAVAILABLE,
        "releases_behind": None,
        "days_behind": None,
        "missed_correctness_releases": [],
        "latest_tag": None,
        "is_drift": False,
        "needs_update": False,
        "using_non_version_tag": False,
        "error": None,
        "filepath": deployment.get("filepath"),
    }

    # 1. Verification itself failed — the deployment state is unknown.
    if probe_error:
        report["state"] = STATE_UNAVAILABLE
        report["error"] = probe_error
        return report

    # 2. Live running tag disagrees with the declared manifest tag.
    # Compared by parsed version so a probe reporting `0.1.1957` against a
    # `0.1.1957@sha256:…` manifest is not a false mismatch; textual
    # inequality of two non-versions (e.g. a SHA vs the manifest) is.
    if running_tag is not None and running_tag != deployed_tag:
        running_version = parse_version(running_tag)
        declared_version = parse_version(deployed_tag)
        if (running_version is None or declared_version is None
                or running_version != declared_version):
            report["state"] = STATE_MISMATCHED
            report["error"] = (
                f"running image tag {running_tag} != declared manifest tag {deployed_tag}"
            )
            report["needs_update"] = True
            return report

    # 3. A tag version math cannot evaluate is never "the approved release".
    deployed_version = parse_version(deployed_tag)
    if deployed_version is None:
        report["state"] = STATE_MISMATCHED
        report["using_non_version_tag"] = True
        report["needs_update"] = True
        report["error"] = f"non-version image tag: {deployed_tag!r}"
        return report

    latest = latest_release(releases or [])
    if latest is None:
        report["state"] = STATE_UNAVAILABLE
        report["error"] = "no releases available for comparison"
        return report
    report["latest_tag"] = latest.get("tag", "")

    behind = releases_behind(deployed_version, releases)
    lag_days = days_behind(deployed_version, releases)
    missed = missed_correctness(deployed_version, releases)
    report["releases_behind"] = behind
    report["days_behind"] = lag_days
    report["missed_correctness_releases"] = missed

    exceeds = (
        (behind is not None and behind >= releases_threshold)
        or (lag_days is not None and lag_days >= days_threshold)
        or bool(missed)
    )
    if exceeds:
        report["state"] = STATE_STALE
        report["is_drift"] = True
        report["needs_update"] = True
    else:
        report["state"] = STATE_CURRENT
    return report


def classify_fleet(deployments: List[Dict[str, Any]],
                   releases: List[Dict[str, Any]],
                   releases_threshold: int,
                   days_threshold: int,
                   probes: Optional[Dict[str, Tuple[Optional[str], Optional[str]]]] = None,
                   expected_clusters: Tuple[str, ...] = ()) -> List[Dict[str, Any]]:
    """Classify every deployment, plus `unavailable` entries for configured
    clusters that have no ARMOR manifest at all."""
    probes = probes or {}
    reports = [
        classify(d, releases, releases_threshold, days_threshold,
                 running_tag=probes.get(d.get("cluster", ""), (None, None))[0],
                 probe_error=probes.get(d.get("cluster", ""), (None, None))[1])
        for d in deployments
    ]
    seen_clusters = {r["cluster"] for r in reports}
    for cluster in expected_clusters:
        if cluster not in seen_clusters:
            reports.append({
                "cluster": cluster,
                "image_type": None,
                "deployed_tag": None,
                "running_tag": None,
                "state": STATE_UNAVAILABLE,
                "releases_behind": None,
                "days_behind": None,
                "missed_correctness_releases": [],
                "latest_tag": None,
                "is_drift": False,
                "needs_update": False,
                "using_non_version_tag": False,
                "error": "cluster configured for monitoring but no ARMOR deployment manifest found",
                "filepath": None,
            })
    reports.sort(key=lambda r: (r["cluster"], r["image_type"] or ""))
    return reports


def drift_fingerprint(reports: List[Dict[str, Any]]) -> Optional[str]:
    """Stable short hash of the non-current subset; None when all current.

    Dedup identity for alerting: the same drift twice hashes identically so
    `--unique-ref` replays to EXISTING, while any change to the drifting set
    (a deployment fixed, a new stale one, a new release) yields a new key.
    Tags enter the hash version-normalized, so a digest-only rebuild of the
    same stale version refiles nothing.
    """
    drifting = [r for r in reports if r["state"] in DRIFT_STATES]
    if not drifting:
        return None

    def tag_key(tag: Optional[str]) -> str:
        version = parse_version(tag or "")
        return f"v0.1.{version}" if version is not None else (tag or "")

    lines = sorted(
        "|".join([
            r["cluster"],
            r["image_type"] or "",
            r["state"],
            tag_key(r["deployed_tag"]),
            tag_key(r["latest_tag"]),
        ])
        for r in drifting
    )
    digest = hashlib.sha256("\n".join(lines).encode()).hexdigest()
    return digest[:16]


def render_alert_body(reports: List[Dict[str, Any]], fingerprint: str) -> str:
    """Human-readable alert bead body for the non-current subset."""
    counts: Dict[str, int] = {}
    for r in reports:
        if r["state"] in DRIFT_STATES:
            counts[r["state"]] = counts.get(r["state"], 0) + 1
    lines = [
        "ARMOR fleet version drift detected.",
        "",
        f"Fingerprint: {fingerprint}",
        f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%SZ')}",
        "States: " + ", ".join(f"{k}={v}" for k, v in sorted(counts.items())),
        "",
    ]
    for r in reports:
        if r["state"] not in DRIFT_STATES:
            continue
        lines.append(f"- {r['cluster']} ({r['image_type'] or 'unknown'}): {r['state']}"
                     f" deployed={r['deployed_tag']} latest={r['latest_tag']}")
        if r.get("error"):
            lines.append(f"  error: {r['error']}")
        if r.get("releases_behind") is not None:
            lines.append(f"  releases_behind: {r['releases_behind']}")
        if r.get("days_behind") is not None:
            lines.append(f"  days_behind: {r['days_behind']}")
        if r.get("missed_correctness_releases"):
            lines.append(f"  missed correctness releases: {', '.join(r['missed_correctness_releases'])}")
        if r.get("filepath"):
            lines.append(f"  manifest: {r['filepath']}")
    lines += [
        "",
        "Reproduce: python3 scripts/drift_check.py --json",
        "This alert deduplicates on the fingerprint; it re-fires only when the"
        " drifting set changes.",
    ]
    return "\n".join(lines)


def resolve_bead_bin(bead_bin: str) -> str:
    """Absolute path for the bead CLI, or the input unchanged.

    systemd --user units on this box run with a PATH that may not include
    the bead binary's directory, so resolve through PATH once here (the
    same reason starvation-watch.py resolves before invoking).
    """
    if os.path.sep in bead_bin:
        return bead_bin
    return shutil.which(bead_bin) or bead_bin


def emit_alert(reports: List[Dict[str, Any]],
               fingerprint: str,
               bead_bin: str = "bead",
               dry_run: bool = False,
               runner=None) -> Tuple[str, str]:
    """File the deduplicated alert bead. Returns (status, detail).

    Status is one of created / existing / existing_closed / dry_run / error,
    mirroring starvation-watch.py's alert filing.
    """
    if runner is None:
        runner = subprocess.run
    title = f"ARMOR fleet version drift detected ({fingerprint})"
    body = render_alert_body(reports, fingerprint)
    cmd = [
        resolve_bead_bin(bead_bin), "create",
        "--title", title,
        "--priority", ALERT_PRIORITY,
        "--issue-type", "task",
        "--label", ALERT_LABEL,
        "--description", body,
        "--unique-ref", f"{UNIQUE_REF_NAMESPACE}:{fingerprint}",
    ]
    if dry_run:
        return "dry_run", " ".join(cmd[:2]) + f" --title {title!r} --unique-ref {UNIQUE_REF_NAMESPACE}:{fingerprint}"
    try:
        proc = runner(cmd, capture_output=True, text=True, timeout=120)
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


def probe_version(url: str, timeout: float = 5.0,
                  urlopen=None) -> str:
    """Fetch the running ARMOR version from a /version endpoint.

    Prefers the JSON body's `version` field; falls back to the
    `Server: ARMOR/<version>` header present on every ARMOR response.
    Raises ProbeError when neither yields a tag.
    """
    if urlopen is None:
        urlopen = urllib.request.urlopen
    try:
        with urlopen(url, timeout=timeout) as resp:
            status = getattr(resp, "status", None) or resp.getcode()
            headers = resp.headers or {}
            body = resp.read(64 * 1024).decode("utf-8", "replace")
    except (urllib.error.URLError, OSError, ValueError) as exc:
        raise ProbeError(f"probe {url} failed: {exc}") from exc
    if status != 200:
        raise ProbeError(f"probe {url} returned HTTP {status}")
    try:
        version = json.loads(body).get("version")
        if isinstance(version, str) and version.strip():
            return version.strip()
    except (json.JSONDecodeError, AttributeError):
        pass
    header = headers.get("Server", "")
    match = SERVER_HEADER_RE.search(header)
    if match:
        return match.group(1)
    raise ProbeError(f"probe {url} returned no version (no JSON version field, no Server header)")


def load_finder_module():
    """Import the hyphenated find-armor-deployments.py for enumeration."""
    path = Path(__file__).resolve().parent / "find-armor-deployments.py"
    spec = importlib.util.spec_from_file_location("find_armor_deployments", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def fetch_releases(fetcher_path: Path) -> List[Dict[str, Any]]:
    """Run github-release-fetcher.py and parse its JSON list."""
    proc = subprocess.run(
        [sys.executable, str(fetcher_path)],
        capture_output=True, text=True, timeout=120,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"github-release-fetcher.py failed: {proc.stderr.strip()}")
    return json.loads(proc.stdout)


def load_config(path: Optional[Path]) -> Dict[str, Any]:
    """Load drift-config.json; missing file means defaults only."""
    if path is None or not path.exists():
        return {}
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)


def summarize(reports: List[Dict[str, Any]]) -> Dict[str, int]:
    counts = {state: 0 for state in (STATE_CURRENT, STATE_STALE,
                                     STATE_MISMATCHED, STATE_UNAVAILABLE)}
    for r in reports:
        counts[r["state"]] += 1
    return counts


def format_report(reports: List[Dict[str, Any]], thresholds: Dict[str, int]) -> str:
    counts = summarize(reports)
    icon_for = {
        STATE_CURRENT: "✅",
        STATE_STALE: "🟡",
        STATE_MISMATCHED: "🟠",
        STATE_UNAVAILABLE: "⚪",
    }
    lines = [
        "=" * 80,
        "ARMOR Drift Check",
        "=" * 80,
        f"Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')}",
        f"Thresholds: >= {thresholds['releases']} releases, >= {thresholds['days']} days",
        "",
        f"Total deployments: {len(reports)}",
        f"current: {counts[STATE_CURRENT]}  stale: {counts[STATE_STALE]}"
        f"  mismatched: {counts[STATE_MISMATCHED]}"
        f"  unavailable: {counts[STATE_UNAVAILABLE]}",
        "",
    ]
    for r in reports:
        lines.append(f"{icon_for[r['state']]} {r['cluster']}"
                     f" ({r['image_type'] or 'unknown'}) [{r['state']}]")
        lines.append(f"   Deployed: {r['deployed_tag']}")
        if r["running_tag"]:
            lines.append(f"   Running:  {r['running_tag']}")
        if r["latest_tag"]:
            lines.append(f"   Latest:   {r['latest_tag']}")
        if r["releases_behind"] is not None:
            lines.append(f"   Releases behind: {r['releases_behind']}")
        if r["days_behind"] is not None:
            lines.append(f"   Days behind: {r['days_behind']}")
        if r["missed_correctness_releases"]:
            lines.append("   🚨 MISSED CORRECTNESS RELEASES: "
                         + ", ".join(r["missed_correctness_releases"]))
        if r["error"]:
            lines.append(f"   error: {r['error']}")
        lines.append("")
    return "\n".join(lines)


def main(argv: Optional[List[str]] = None) -> int:
    repo_root = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(
        description="ARMOR drift check: classify deployments as current/stale/"
                    "mismatched/unavailable and optionally file one deduplicated alert bead."
    )
    parser.add_argument("--config", type=Path,
                        default=repo_root / "config" / "drift-config.json",
                        help="drift-config.json path (default: config/drift-config.json)")
    parser.add_argument("--manifests", type=Path, default=None,
                        help="declarative-config checkout (default from config, else ~/declarative-config)")
    parser.add_argument("--releases-file", type=Path, default=None,
                        help="releases JSON file (default: run github-release-fetcher.py)")
    parser.add_argument("--latest-tag", default=None, metavar="TAG",
                        help="treat TAG (e.g. v0.1.1970) as the latest approved release even if"
                             " the release source does not list it yet; use when tags lag VERSION")
    parser.add_argument("--probe-url", action="append", default=[], metavar="CLUSTER=URL",
                        help="live /version endpoint for a cluster; running tag is compared"
                             " against the declared manifest tag (repeatable)")
    parser.add_argument("--expected-cluster", action="append", default=[],
                        help="cluster that must have an ARMOR manifest (adds to config clusters)")
    parser.add_argument("--releases-threshold", type=int, default=None)
    parser.add_argument("--days-threshold", type=int, default=None)
    parser.add_argument("--json", action="store_true", help="machine-readable JSON output")
    parser.add_argument("--output", type=Path, default=None,
                        help="also write the report to this file")
    parser.add_argument("--emit-bead", action="store_true",
                        help="file ONE deduplicated alert bead when anything is non-current")
    parser.add_argument("--dry-run", action="store_true",
                        help="with --emit-bead, print the bead command instead of running it")
    parser.add_argument("--bead-bin", default="bead", help="bead CLI binary (default: bead)")
    args = parser.parse_args(argv)

    try:
        config = load_config(args.config)
    except (OSError, json.JSONDecodeError) as exc:
        print(f"Error: cannot read config {args.config}: {exc}", file=sys.stderr)
        return 2

    releases_threshold = args.releases_threshold or config.get("releases_threshold", 50)
    days_threshold = args.days_threshold or config.get("days_threshold", 30)

    manifests_path = args.manifests
    if manifests_path is None:
        configured = config.get("declarative_config_path", "~/declarative-config")
        manifests_path = Path(os.path.expanduser(str(configured)))
    if not manifests_path.exists():
        print(f"Error: declarative-config not found at {manifests_path}", file=sys.stderr)
        return 2

    try:
        if args.releases_file is not None:
            with open(args.releases_file, "r", encoding="utf-8") as fh:
                releases = json.load(fh)
        else:
            releases = fetch_releases(repo_root / "scripts" / "github-release-fetcher.py")
    except (OSError, json.JSONDecodeError, RuntimeError, subprocess.TimeoutExpired) as exc:
        print(f"Error: cannot load releases: {exc}", file=sys.stderr)
        return 2

    if args.latest_tag:
        # An operator-asserted latest: appended so latest_release() (highest
        # version wins) picks it up, without discarding the fetched history
        # that missed_correctness_releases still needs.
        if parse_version(args.latest_tag) is None:
            print(f"Error: --latest-tag expects a version tag like v0.1.1970, got {args.latest_tag!r}",
                  file=sys.stderr)
            return 2
        if not any(r.get("tag") == args.latest_tag for r in releases):
            releases = list(releases) + [{
                "tag": args.latest_tag,
                "published_at": datetime.now(timezone.utc).isoformat(),
                "is_correctness": False,
                "url": "",
            }]

    try:
        finder = load_finder_module()
        deployments = finder.find_armor_deployments(str(manifests_path))
    except Exception as exc:  # enumeration must never half-report
        print(f"Error: deployment enumeration failed: {exc}", file=sys.stderr)
        return 2

    probes: Dict[str, Tuple[Optional[str], Optional[str]]] = {}
    for spec in args.probe_url:
        cluster, sep, url = spec.partition("=")
        if not sep or not cluster or not url:
            print(f"Error: --probe-url expects CLUSTER=URL, got {spec!r}", file=sys.stderr)
            return 2
        try:
            probes[cluster] = (probe_version(url), None)
        except ProbeError as exc:
            probes[cluster] = (None, str(exc))

    expected_clusters = tuple(dict.fromkeys(
        list(config.get("clusters", [])) + list(args.expected_cluster)))

    reports = classify_fleet(
        deployments, releases, releases_threshold, days_threshold,
        probes=probes, expected_clusters=expected_clusters,
    )

    result = {
        "thresholds": {"releases": releases_threshold, "days": days_threshold},
        "summary": {
            "total_deployments": len(reports),
            **{f"{state}_count": count for state, count in summarize(reports).items()},
            "needs_update": sum(1 for r in reports if r["needs_update"]),
        },
        "deployments": reports,
        "fingerprint": drift_fingerprint(reports),
        "generated_at": datetime.now(timezone.utc).isoformat(),
    }

    if args.emit_bead:
        fingerprint = result["fingerprint"]
        if fingerprint is None:
            print("All deployments current; no alert to file.", file=sys.stderr)
        else:
            status, detail = emit_alert(reports, fingerprint,
                                        bead_bin=args.bead_bin, dry_run=args.dry_run)
            result["alert"] = {"status": status, "detail": detail}
            print(f"alert: {status} {detail}".rstrip(), file=sys.stderr)

    if args.json:
        output = json.dumps(result, indent=2)
    else:
        output = format_report(reports, result["thresholds"])
    print(output)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(output, encoding="utf-8")
        print(f"Report written to {args.output}", file=sys.stderr)

    return 1 if result["fingerprint"] is not None else 0


if __name__ == "__main__":
    sys.exit(main())
