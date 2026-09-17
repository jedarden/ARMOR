#!/usr/bin/env python3
"""
Tests for scripts/drift_check.py — the ARMOR fleet version-drift check.

Covers the three documented deployment states (current, stale, unavailable)
plus mismatched, the deduplicated alert fingerprint/emission, the live
/version probe, and the CLI exit-code contract from docs/drift-check.md.

Run: python3 -m pytest tests/test_drift_check.py -q
"""

import json
import socket
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

SCRIPTS_DIR = Path(__file__).resolve().parents[1] / "scripts"
sys.path.insert(0, str(SCRIPTS_DIR))

import drift_check  # noqa: E402

RELEASES = [
    {"tag": "v0.1.100", "published_at": "2026-08-20T00:00:00Z",
     "is_correctness": False, "url": ""},
    {"tag": "v0.1.99", "published_at": "2026-08-01T00:00:00Z",
     "is_correctness": True, "url": ""},   # security: fix auth bypass
    {"tag": "v0.1.98", "published_at": "2026-07-25T00:00:00Z",
     "is_correctness": False, "url": ""},
]


def deployment(cluster="iad-ci", image_type="armor", tag="v0.1.100",
               filepath=None):
    return {"cluster": cluster, "image_type": image_type, "image_tag": tag,
            "filepath": filepath or f"/dc/k8s/{cluster}/armor/armor-deployment.yml"}


# ---------------------------------------------------------------------------
# version parsing
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("tag,expected", [
    ("v0.1.42", 42),
    ("0.1.1966", 1966),
    ("fcbf6d3", None),      # git SHA -> not a version
    ("latest", None),
    ("", None),
    ("0.1.42-rc1", None),   # full match only
    ("0.2.1", None),
    ("0.1.1957@sha256:" + "a" * 64, 1957),  # digest pin reads as its version
    ("0.1.5@sha256:abc", None),             # truncated digest is not a pin
])
def test_parse_version(tag, expected):
    assert drift_check.parse_version(tag) == expected


# ---------------------------------------------------------------------------
# state: current
# ---------------------------------------------------------------------------

def test_classify_current_at_latest():
    report = drift_check.classify(deployment(tag="v0.1.100"), RELEASES, 50, 30)
    assert report["state"] == drift_check.STATE_CURRENT
    assert report["is_drift"] is False
    assert report["needs_update"] is False
    assert report["latest_tag"] == "v0.1.100"
    assert report["releases_behind"] == 0


def test_classify_current_digest_pinned_tag_reads_as_version():
    """`0.1.NNNN@sha256:…` is the fleet's manifest convention — the digest
    pin is provenance hardening, not a non-version tag."""
    tag = "0.1.100@sha256:" + "a" * 64
    report = drift_check.classify(deployment(tag=tag), RELEASES, 50, 30)
    assert report["state"] == drift_check.STATE_CURRENT
    assert report["releases_behind"] == 0
    assert report["using_non_version_tag"] is False


def test_classify_current_within_thresholds_is_not_drift():
    """2 releases behind, no missed correctness, 19 days: a routine bump."""
    releases = [
        {"tag": "v0.1.100", "published_at": "2026-08-20T00:00:00Z",
         "is_correctness": False, "url": ""},
        {"tag": "v0.1.99", "published_at": "2026-08-15T00:00:00Z",
         "is_correctness": False, "url": ""},
        {"tag": "v0.1.98", "published_at": "2026-08-01T00:00:00Z",
         "is_correctness": False, "url": ""},
    ]
    report = drift_check.classify(deployment(tag="v0.1.98"), releases, 50, 30)
    assert report["state"] == drift_check.STATE_CURRENT
    assert report["releases_behind"] == 2
    assert report["days_behind"] == 19


def test_classify_current_ignores_older_correctness_release():
    """A missed correctness release OLDER than the deployment is irrelevant."""
    report = drift_check.classify(deployment(tag="v0.1.99"), RELEASES, 50, 30)
    assert report["state"] == drift_check.STATE_CURRENT
    assert report["missed_correctness_releases"] == []


# ---------------------------------------------------------------------------
# state: stale
# ---------------------------------------------------------------------------

def test_classify_stale_by_release_threshold():
    releases = [{"tag": f"v0.1.{n}", "published_at": "2026-09-01T00:00:00Z",
                 "is_correctness": False, "url": ""} for n in range(60, 39, -1)]
    # 20 releases behind (v0.1.41..v0.1.60) against a threshold of 15
    report = drift_check.classify(deployment(tag="v0.1.40"), releases, 15, 30)
    assert report["state"] == drift_check.STATE_STALE
    assert report["is_drift"] is True
    assert report["needs_update"] is True
    assert report["releases_behind"] == 20


def test_classify_stale_by_days_threshold():
    releases = [
        {"tag": "v0.1.100", "published_at": "2026-09-01T00:00:00Z",
         "is_correctness": False, "url": ""},
        {"tag": "v0.1.90", "published_at": "2026-01-01T00:00:00Z",
         "is_correctness": False, "url": ""},
    ]
    report = drift_check.classify(deployment(tag="v0.1.90"), releases, 50, 30)
    assert report["state"] == drift_check.STATE_STALE
    assert report["days_behind"] >= 30


def test_classify_stale_missed_correctness_beats_thresholds():
    """Any missed correctness release flags stale even below both thresholds."""
    report = drift_check.classify(deployment(tag="v0.1.99"), [
        {"tag": "v0.1.100", "published_at": "2026-09-01T00:00:00Z",
         "is_correctness": True, "url": ""},
        {"tag": "v0.1.99", "published_at": "2026-08-25T00:00:00Z",
         "is_correctness": False, "url": ""},
    ], 50, 30)
    assert report["state"] == drift_check.STATE_STALE
    assert report["missed_correctness_releases"] == ["v0.1.100"]


# ---------------------------------------------------------------------------
# state: mismatched
# ---------------------------------------------------------------------------

def test_classify_mismatched_non_version_tag():
    report = drift_check.classify(deployment(tag="fcbf6d3"), RELEASES, 50, 30)
    assert report["state"] == drift_check.STATE_MISMATCHED
    assert report["using_non_version_tag"] is True
    assert report["needs_update"] is True


def test_classify_mismatched_running_differs_from_manifest():
    report = drift_check.classify(
        deployment(tag="v0.1.100"), RELEASES, 50, 30, running_tag="v0.1.98")
    assert report["state"] == drift_check.STATE_MISMATCHED
    assert report["running_tag"] == "v0.1.98"
    assert "v0.1.98" in report["error"] and "v0.1.100" in report["error"]


def test_classify_running_matches_manifest_is_not_mismatch():
    report = drift_check.classify(
        deployment(tag="v0.1.100"), RELEASES, 50, 30, running_tag="v0.1.100")
    assert report["state"] == drift_check.STATE_CURRENT


def test_classify_probe_plain_version_matches_digest_pinned_manifest():
    """A probe reporting `0.1.100` against a `0.1.100@sha256:…` manifest is
    the same release — compared by parsed version, not raw string."""
    tag = "0.1.100@sha256:" + "a" * 64
    report = drift_check.classify(
        deployment(tag=tag), RELEASES, 50, 30, running_tag="0.1.100")
    assert report["state"] == drift_check.STATE_CURRENT
    assert report["running_tag"] == "0.1.100"


def test_classify_probe_different_version_vs_digest_pinned_is_mismatched():
    tag = "0.1.100@sha256:" + "a" * 64
    report = drift_check.classify(
        deployment(tag=tag), RELEASES, 50, 30, running_tag="0.1.98")
    assert report["state"] == drift_check.STATE_MISMATCHED


# ---------------------------------------------------------------------------
# state: unavailable
# ---------------------------------------------------------------------------

def test_classify_unavailable_probe_error():
    report = drift_check.classify(
        deployment(tag="v0.1.100"), RELEASES, 50, 30,
        probe_error="probe https://x/version failed: connection refused")
    assert report["state"] == drift_check.STATE_UNAVAILABLE
    assert "connection refused" in report["error"]
    assert report["needs_update"] is False


def test_classify_unavailable_no_releases():
    report = drift_check.classify(deployment(tag="v0.1.100"), [], 50, 30)
    assert report["state"] == drift_check.STATE_UNAVAILABLE
    assert "no releases" in report["error"]


def test_classify_fleet_reports_unconfigured_cluster_as_unavailable():
    reports = drift_check.classify_fleet(
        [deployment(cluster="iad-ci", tag="v0.1.100")],
        RELEASES, 50, 30, expected_clusters=("iad-ci", "iad-acb"))
    by_cluster = {r["cluster"]: r for r in reports}
    assert by_cluster["iad-ci"]["state"] == drift_check.STATE_CURRENT
    assert by_cluster["iad-acb"]["state"] == drift_check.STATE_UNAVAILABLE
    assert "no ARMOR deployment manifest" in by_cluster["iad-acb"]["error"]


def test_classify_fleet_applies_cluster_probe():
    probes = {"iad-ci": (None, "probe failed: timeout")}
    reports = drift_check.classify_fleet(
        [deployment(cluster="iad-ci", tag="v0.1.100")],
        RELEASES, 50, 30, probes=probes, expected_clusters=("iad-ci",))
    assert reports[0]["state"] == drift_check.STATE_UNAVAILABLE


# ---------------------------------------------------------------------------
# dedup fingerprint + alert emission
# ---------------------------------------------------------------------------

def test_fingerprint_none_when_all_current():
    reports = drift_check.classify_fleet(
        [deployment(tag="v0.1.100")], RELEASES, 50, 30)
    assert drift_check.drift_fingerprint(reports) is None


def test_fingerprint_stable_for_identical_drift():
    args = ([deployment(cluster="iad-kalshi", tag="v0.1.10")], RELEASES, 50, 30)
    assert (drift_check.drift_fingerprint(drift_check.classify_fleet(*args))
            == drift_check.drift_fingerprint(drift_check.classify_fleet(*args)))


def test_fingerprint_changes_when_drift_changes():
    def fp(tag):
        reports = drift_check.classify_fleet(
            [deployment(cluster="iad-kalshi", tag=tag)], RELEASES, 50, 30)
        return drift_check.drift_fingerprint(reports)

    assert fp("v0.1.10") != fp("v0.1.11")
    assert fp("v0.1.10") is not None


def test_fingerprint_ignores_digest_only_rebuild():
    """A digest-only rebuild of the same stale version is the same drift
    picture and must keep the dedup key (no duplicate alert)."""
    def fp(tag):
        reports = drift_check.classify_fleet(
            [deployment(cluster="iad-kalshi", tag=tag)], RELEASES, 50, 30)
        return drift_check.drift_fingerprint(reports)

    pinned = "0.1.10@sha256:" + "b" * 64
    assert fp(pinned) == fp("v0.1.10")


class FakeProc:
    def __init__(self, stdout="", returncode=0, stderr=""):
        self.stdout, self.returncode, self.stderr = stdout, returncode, stderr


def test_emit_alert_created_and_dedup():
    """First create returns created; a replay through --unique-ref returns
    EXISTING and must not file a second bead."""
    reports = drift_check.classify_fleet(
        [deployment(cluster="iad-kalshi", tag="v0.1.10")], RELEASES, 50, 30)
    fp = drift_check.drift_fingerprint(reports)
    calls = []

    def runner(cmd, **kwargs):
        calls.append(cmd)
        if len(calls) == 1:
            return FakeProc(stdout="armor-abc123\n")
        return FakeProc(stdout=f"EXISTING armor-abc123\n")

    status, detail = drift_check.emit_alert(reports, fp, runner=runner)
    assert status == "created"
    status2, detail2 = drift_check.emit_alert(reports, fp, runner=runner)
    assert status2 == "existing"
    assert len(calls) == 2
    assert calls[0][-1] == f"drift-check:{fp}"
    assert "--unique-ref" in calls[0]


def test_emit_alert_existing_closed_status():
    reports = drift_check.classify_fleet(
        [deployment(cluster="iad-kalshi", tag="v0.1.10")], RELEASES, 50, 30)
    fp = drift_check.drift_fingerprint(reports)
    runner = lambda cmd, **kw: FakeProc(stdout="EXISTING_CLOSED armor-old\n")
    status, detail = drift_check.emit_alert(reports, fp, runner=runner)
    assert status == "existing_closed"
    assert detail == "armor-old"


def test_emit_alert_dry_run_runs_nothing():
    reports = drift_check.classify_fleet(
        [deployment(tag="v0.1.10")], RELEASES, 50, 30)
    fp = drift_check.drift_fingerprint(reports)
    calls = []
    status, detail = drift_check.emit_alert(
        reports, fp, dry_run=True, runner=lambda cmd, **kw: calls.append(cmd))
    assert status == "dry_run"
    assert calls == []
    assert "drift-check:" in detail


# ---------------------------------------------------------------------------
# bead binary resolution (systemd --user PATH safety)
# ---------------------------------------------------------------------------

def test_resolve_bead_bin_passthrough_absolute(tmp_path):
    assert drift_check.resolve_bead_bin(str(tmp_path / "bead")) == str(tmp_path / "bead")


def test_resolve_bead_bin_resolves_via_path(tmp_path, monkeypatch):
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    fake = bin_dir / "bead"
    fake.write_text("#!/bin/sh\n", encoding="utf-8")
    fake.chmod(0o755)
    monkeypatch.setenv("PATH", str(bin_dir))
    assert drift_check.resolve_bead_bin("bead") == str(fake)


def test_resolve_bead_bin_unresolvable_returns_input(monkeypatch):
    monkeypatch.setenv("PATH", "")
    assert drift_check.resolve_bead_bin("drift-check-no-such-bin") == "drift-check-no-such-bin"


def test_render_alert_body_lists_drifting_deployments():
    reports = drift_check.classify_fleet(
        [deployment(cluster="iad-kalshi", tag="v0.1.10"),
         deployment(cluster="iad-ci", tag="v0.1.100")],
        RELEASES, 50, 30)
    body = drift_check.render_alert_body(reports, "fp01")
    assert "iad-kalshi" in body and "stale" in body
    assert "Fingerprint: fp01" in body
    assert "iad-ci" not in body.split("Reproduce")[0].split("States:")[1]


# ---------------------------------------------------------------------------
# live /version probe
# ---------------------------------------------------------------------------

class _Handler(BaseHTTPRequestHandler):
    body = b'{"version":"0.1.99","format_write_version":2,"go":"1.23.1"}'
    armor_server_header = None

    def version_string(self):
        """Simulate ARMOR's own Server header (BaseHTTPRequestHandler would
        otherwise prepend BaseHTTP/..., which would win headers.get())."""
        return self.armor_server_header or super().version_string()

    def do_GET(self):
        if self.armor_server_header:
            # non-JSON body -> exercises the Server-header fallback
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"ok")
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(self.body)

    def log_message(self, *args):
        pass


@pytest.fixture
def probe_server():
    server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    yield f"http://127.0.0.1:{server.server_address[1]}/version"
    server.shutdown()


def test_probe_version_parses_json_body(probe_server):
    assert drift_check.probe_version(probe_server) == "0.1.99"


def test_probe_version_falls_back_to_server_header(probe_server):
    _Handler.armor_server_header = "ARMOR/0.1.5"
    try:
        assert drift_check.probe_version(probe_server) == "0.1.5"
    finally:
        _Handler.armor_server_header = None


def test_probe_version_connection_refused_is_probe_error():
    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    dead_port = sock.getsockname()[1]
    sock.close()  # nothing listens here now
    with pytest.raises(drift_check.ProbeError):
        drift_check.probe_version(f"http://127.0.0.1:{dead_port}/version", timeout=2)


# ---------------------------------------------------------------------------
# enumeration through the real finder + CLI contract
# ---------------------------------------------------------------------------

def write_dc(root: Path, cluster: str, tag: str):
    d = root / "k8s" / cluster / "armor"
    d.mkdir(parents=True)
    (d / "armor-deployment.yml").write_text(
        f"image: ronaldraygun/armor:{tag}\n", encoding="utf-8")


def test_enumerate_and_classify_end_to_end(tmp_path):
    write_dc(tmp_path, "iad-ci", "v0.1.100")
    write_dc(tmp_path, "iad-kalshi", "v0.1.10")
    finder = drift_check.load_finder_module()
    deployments = finder.find_armor_deployments(str(tmp_path))
    assert len(deployments) == 2
    reports = drift_check.classify_fleet(
        deployments, RELEASES, 50, 30, expected_clusters=("iad-ci", "iad-kalshi"))
    by_cluster = {r["cluster"]: r for r in reports}
    assert by_cluster["iad-ci"]["state"] == drift_check.STATE_CURRENT
    assert by_cluster["iad-kalshi"]["state"] == drift_check.STATE_STALE


@pytest.fixture
def dc_and_releases(tmp_path):
    write_dc(tmp_path / "dc", "iad-ci", "v0.1.100")
    releases = tmp_path / "releases.json"
    releases.write_text(json.dumps(RELEASES), encoding="utf-8")
    empty_config = tmp_path / "empty-config.json"
    empty_config.write_text("{}", encoding="utf-8")
    return tmp_path / "dc", releases, empty_config


def _run_cli(args):
    return subprocess.run(
        [sys.executable, str(SCRIPTS_DIR / "drift_check.py"), *args],
        capture_output=True, text=True, timeout=60)


def test_cli_exit_zero_when_all_current(dc_and_releases):
    dc, releases, config = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config)])
    assert proc.returncode == 0, proc.stderr
    data = json.loads(proc.stdout)
    assert data["summary"]["current_count"] == 1
    assert data["summary"]["total_deployments"] == 1
    assert data["fingerprint"] is None


def test_cli_exit_one_on_stale_and_dry_run_alert(dc_and_releases, tmp_path):
    write_dc(tmp_path / "dc", "iad-kalshi", "v0.1.10")
    dc, releases, config = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--emit-bead", "--dry-run"])
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    assert data["summary"]["stale_count"] == 1
    assert data["alert"]["status"] == "dry_run"
    assert data["fingerprint"]


def test_cli_exit_two_on_missing_manifests_dir(tmp_path):
    releases = tmp_path / "releases.json"
    releases.write_text(json.dumps(RELEASES), encoding="utf-8")
    proc = _run_cli(["--json", "--manifests", str(tmp_path / "nope"),
                     "--releases-file", str(releases),
                     "--config", str(tmp_path / "empty.json")])
    assert proc.returncode == 2


def test_cli_unavailable_cluster_in_expected_list(dc_and_releases):
    dc, releases, config = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--expected-cluster", "iad-acb"])
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    states = {d["cluster"]: d["state"] for d in data["deployments"]}
    assert states["iad-ci"] == drift_check.STATE_CURRENT
    assert states["iad-acb"] == drift_check.STATE_UNAVAILABLE
