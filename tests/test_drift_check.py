#!/usr/bin/env python3
"""
Tests for scripts/drift_check.py — the ARMOR fleet version-drift check.

Covers the three documented deployment states (current, stale, unavailable)
plus mismatched, the deduplicated alert fingerprint/emission, the live
/version probe, the VERSION floor (max(newest tag, VERSION) as the approved
latest, with the tags_behind_version warning), and the CLI exit-code
contract from docs/drift-check.md.

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
    # Same version as the newest fixture tag: no VERSION floor, no
    # tags_behind_version warning, so these CLI tests stay independent of
    # the real checkout's VERSION.
    version_file = tmp_path / "VERSION"
    version_file.write_text("0.1.100\n", encoding="utf-8")
    return tmp_path / "dc", releases, empty_config, version_file


def _run_cli(args):
    return subprocess.run(
        [sys.executable, str(SCRIPTS_DIR / "drift_check.py"), *args],
        capture_output=True, text=True, timeout=60)


def test_cli_exit_zero_when_all_current(dc_and_releases):
    dc, releases, config, version_file = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--version-file", str(version_file)])
    assert proc.returncode == 0, proc.stderr
    data = json.loads(proc.stdout)
    assert data["summary"]["current_count"] == 1
    assert data["summary"]["total_deployments"] == 1
    assert data["fingerprint"] is None


def test_cli_exit_one_on_stale_and_dry_run_alert(dc_and_releases, tmp_path):
    write_dc(tmp_path / "dc", "iad-kalshi", "v0.1.10")
    dc, releases, config, version_file = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--version-file", str(version_file),
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
    dc, releases, config, version_file = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--version-file", str(version_file),
                     "--expected-cluster", "iad-acb"])
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    states = {d["cluster"]: d["state"] for d in data["deployments"]}
    assert states["iad-ci"] == drift_check.STATE_CURRENT
    assert states["iad-acb"] == drift_check.STATE_UNAVAILABLE


def test_cli_latest_tag_override_makes_lagging_source_visible(dc_and_releases):
    # The release source stops at v0.1.100 (tags lagged VERSION); the operator
    # asserts v0.1.200 is the real latest. The deployment at v0.1.100 is then
    # one approved release behind: current under the releases threshold, stale
    # as soon as that threshold is one release. The days threshold is pinned
    # high because the --latest-tag entry is stamped "now" while the fixture's
    # v0.1.100 date is fixed — an unpinned default would trip as the fixture
    # ages past it (this test failed exactly that way on 2026-09-19).
    dc, releases, config, version_file = dc_and_releases
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--version-file", str(version_file),
                     "--latest-tag", "v0.1.200", "--days-threshold", "3650"])
    assert proc.returncode == 0, proc.stderr
    report = json.loads(proc.stdout)["deployments"][0]
    assert report["latest_tag"] == "v0.1.200"
    assert report["releases_behind"] == 1
    assert report["state"] == drift_check.STATE_CURRENT

    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--version-file", str(version_file),
                     "--latest-tag", "v0.1.200", "--releases-threshold", "1",
                     "--days-threshold", "3650"])
    assert proc.returncode == 1, proc.stderr
    report = json.loads(proc.stdout)["deployments"][0]
    assert report["state"] == drift_check.STATE_STALE


def test_cli_latest_tag_rejects_non_version():
    proc = _run_cli(["--latest-tag", "latest"])
    assert proc.returncode == 2
    assert "--latest-tag" in proc.stderr


# ---------------------------------------------------------------------------
# VERSION floor: max(newest tag, VERSION) is the approved latest
# ---------------------------------------------------------------------------

def write_version_file(tmp_path, text="0.1.1969"):
    p = tmp_path / "VERSION"
    p.write_text(text + "\n", encoding="utf-8")
    return p


def version_floor_cli_env(tmp_path, tag):
    """Manifests + releases ending at v0.1.1957 + a VERSION of 0.1.1969 —
    the 2026-09-17 incident shape (tags stopped 12 releases behind prod)."""
    dc = tmp_path / "dc"
    write_dc(dc, "iad-ci", tag)
    releases = tmp_path / "releases.json"
    releases.write_text(json.dumps([
        {"tag": "v0.1.1957", "published_at": "2026-08-01T00:00:00Z",
         "is_correctness": False, "url": ""},
        {"tag": "v0.1.1956", "published_at": "2026-07-25T00:00:00Z",
         "is_correctness": False, "url": ""},
    ]), encoding="utf-8")
    config = tmp_path / "empty-config.json"
    config.write_text("{}", encoding="utf-8")
    version_file = write_version_file(tmp_path)
    return ["--json", "--manifests", str(dc), "--releases-file", str(releases),
            "--config", str(config), "--version-file", str(version_file)]


def test_resolve_version_source_default_is_repo_version(tmp_path):
    (tmp_path / "VERSION").write_text("0.1.1970\n", encoding="utf-8")
    assert drift_check.resolve_version_source(None, tmp_path) == (1970, "0.1.1970")


def test_resolve_version_source_reads_given_file(tmp_path):
    assert (drift_check.resolve_version_source(str(write_version_file(tmp_path)), tmp_path)
            == (1969, "0.1.1969"))


def test_resolve_version_source_keeps_v_prefix(tmp_path):
    assert (drift_check.resolve_version_source(str(write_version_file(tmp_path, "v0.1.1969")), tmp_path)
            == (1969, "v0.1.1969"))


def test_resolve_version_source_missing_is_none(tmp_path):
    assert drift_check.resolve_version_source(str(tmp_path / "nope"), tmp_path) == (None, "")


def test_resolve_version_source_malformed_is_none(tmp_path):
    assert (drift_check.resolve_version_source(str(write_version_file(tmp_path, "not-a-version")), tmp_path)
            == (None, ""))


def test_resolve_version_source_url():
    class FakeResp:
        status = 200

        def read(self, n=-1):
            return b"0.1.1969\n"

        def __enter__(self):
            return self

        def __exit__(self, *exc):
            return False

    version = drift_check.resolve_version_source(
        "https://raw.githubusercontent.com/jedarden/ARMOR/main/VERSION",
        Path("/unused"), urlopen=lambda url, timeout=None: FakeResp())
    assert version == (1969, "0.1.1969")


def test_resolve_version_source_url_failure_degrades_to_none():
    import urllib.error

    def boom(url, timeout=None):
        raise urllib.error.URLError("no network")

    version = drift_check.resolve_version_source(
        "https://example.invalid/VERSION", Path("/unused"), urlopen=boom)
    assert version == (None, "")


def test_resolve_version_source_url_http_exception_degrades_to_none():
    # http.client.HTTPException (IncompleteRead, BadStatusLine, …) is not an
    # OSError; the fetch must degrade to tag-only, not crash the run.
    import http.client

    def truncated(url, timeout=None):
        raise http.client.IncompleteRead(b"0.1.19")

    version = drift_check.resolve_version_source(
        "https://example.invalid/VERSION", Path("/unused"), urlopen=truncated)
    assert version == (None, "")


def test_approved_latest_appends_version_floor():
    releases = [{"tag": "v0.1.1957", "published_at": "2026-08-01T00:00:00Z",
                 "is_correctness": False, "url": ""}]
    merged, warning = drift_check.approved_latest_from_version(releases, 1969, "0.1.1969")
    assert drift_check.latest_release(merged)["tag"] == "0.1.1969"
    assert warning is not None and "tags_behind_version" in warning
    assert "v0.1.1957" in warning


def test_approved_latest_noop_when_tags_cover_version():
    releases = [{"tag": "v0.1.1970", "published_at": "2026-09-01T00:00:00Z",
                 "is_correctness": False, "url": ""}]
    merged, warning = drift_check.approved_latest_from_version(releases, 1969, "0.1.1969")
    assert merged == releases
    assert warning is None


def test_approved_latest_noop_without_parseable_tags():
    """An empty/broken release source stays broken: no floor, deployments
    stay unavailable rather than 'current' against zero history."""
    merged, warning = drift_check.approved_latest_from_version([], 1969, "0.1.1969")
    assert merged == []
    assert warning is None


def test_classify_at_newest_tag_with_version_floor_is_stale():
    """The acceptance case: tags end at 1957, VERSION is 1969 — a 0.1.1957
    deployment is stale against the VERSION-raised latest."""
    releases = [{"tag": "v0.1.1957", "published_at": "2026-08-01T00:00:00Z",
                 "is_correctness": False, "url": ""}]
    merged, _ = drift_check.approved_latest_from_version(releases, 1969, "0.1.1969")
    report = drift_check.classify(deployment(tag="0.1.1957"), merged, 50, 30)
    assert report["state"] == drift_check.STATE_STALE
    assert report["latest_tag"] == "0.1.1969"


def test_classify_between_newest_tag_and_version_is_current():
    """A deployment newer than every tag but <= VERSION is current against
    the VERSION floor, never 'newer than latest'."""
    releases = [{"tag": "v0.1.1957", "published_at": "2026-09-01T00:00:00Z",
                 "is_correctness": False, "url": ""}]
    merged, _ = drift_check.approved_latest_from_version(releases, 1969, "0.1.1969")
    report = drift_check.classify(deployment(tag="0.1.1963"), merged, 50, 30)
    assert report["state"] == drift_check.STATE_CURRENT
    assert report["latest_tag"] == "0.1.1969"
    assert report["is_drift"] is False


def test_classify_fleet_at_newest_tag_with_version_floor_is_stale():
    """Fleet-level acceptance: with the floor applied, a deployment at the
    newest tag itself (v0.1.1957) goes stale once VERSION (0.1.1969) outruns
    it — classify_fleet measures against the approved latest, not the tag."""
    releases = [{"tag": "v0.1.1957", "published_at": "2026-08-01T00:00:00Z",
                 "is_correctness": False, "url": ""}]
    merged, warning = drift_check.approved_latest_from_version(releases, 1969, "0.1.1969")
    assert warning, "the floor must fire for this fleet picture"
    reports = drift_check.classify_fleet([deployment(tag="0.1.1957")], merged, 50, 30)
    assert len(reports) == 1
    report = reports[0]
    assert report["state"] == drift_check.STATE_STALE
    assert report["latest_tag"] == "0.1.1969"
    assert report["is_drift"] is True
    assert report["needs_update"] is True


def test_classify_fleet_between_newest_tag_and_version_is_current():
    """Fleet-level acceptance: deployments newer than every fetched tag but
    <= VERSION classify current, never unavailable — the 0.1.1963-0.1.1969
    fleet the parent observed misclassifying."""
    releases = [{"tag": "v0.1.1957", "published_at": "2026-08-01T00:00:00Z",
                 "is_correctness": False, "url": ""}]
    merged, _ = drift_check.approved_latest_from_version(releases, 1969, "0.1.1969")
    reports = drift_check.classify_fleet(
        [deployment(cluster="ardenone-cluster", tag="0.1.1963"),
         deployment(cluster="iad-kalshi", tag="v0.1.1969")],
        merged, 50, 30)
    assert len(reports) == 2
    assert all(r["state"] == drift_check.STATE_CURRENT for r in reports)
    assert all(r["latest_tag"] == "0.1.1969" for r in reports)
    assert all(not r["is_drift"] for r in reports)
    assert all(not r["needs_update"] for r in reports)


def test_fingerprint_non_none_on_warning_alone():
    reports = drift_check.classify_fleet(
        [deployment(tag="v0.1.100")], RELEASES, 50, 30)
    assert drift_check.drift_fingerprint(reports) is None
    fp = drift_check.drift_fingerprint(
        reports, warnings=["tags_behind_version: VERSION 0.1.1969 newer than v0.1.1957"])
    assert fp is not None


def test_fingerprint_changes_when_warning_appears_or_text_changes():
    reports = drift_check.classify_fleet(
        [deployment(cluster="iad-kalshi", tag="v0.1.10")], RELEASES, 50, 30)
    fp_plain = drift_check.drift_fingerprint(reports)
    warning = "tags_behind_version: VERSION 0.1.1969 newer than v0.1.1957"
    fp_warned = drift_check.drift_fingerprint(reports, warnings=[warning])
    assert fp_plain != fp_warned
    # Same warning twice -> same key (dedup still holds); changed text -> new key.
    assert (drift_check.drift_fingerprint(reports, warnings=[warning]) == fp_warned)
    assert (drift_check.drift_fingerprint(reports, warnings=[warning + " (updated)"])
            != fp_warned)


def test_render_alert_body_includes_tags_behind_version():
    reports = drift_check.classify_fleet(
        [deployment(tag="v0.1.100")], RELEASES, 50, 30)
    body = drift_check.render_alert_body(
        reports, "fp02",
        warnings=["tags_behind_version: tagging lapsed; using VERSION"])
    assert "tags_behind_version" in body
    assert "tagging lapsed" in body


def test_cli_version_floor_reports_latest_warning_and_stale(tmp_path):
    # Acceptance: with tags ending at 1957 and VERSION at 1969, --json
    # reports latest=0.1.1969, flags tags_behind_version, and classifies a
    # 0.1.1957 deployment as stale.
    args = version_floor_cli_env(tmp_path, "0.1.1957")
    proc = _run_cli(args)
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    assert data["warnings"], "tags_behind_version must reach the JSON output"
    assert "tags_behind_version" in data["warnings"][0]
    report = data["deployments"][0]
    assert report["latest_tag"] == "0.1.1969"
    assert report["state"] == drift_check.STATE_STALE
    assert data["fingerprint"]


def test_cli_version_floor_deployment_between_tag_and_version_is_current(tmp_path):
    # 0.1.1963 is newer than every tag but <= VERSION: current, not unknown.
    # Exit is still 1 because the tags_behind_version warning fired.
    args = version_floor_cli_env(tmp_path, "0.1.1963")
    proc = _run_cli(args)
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    report = data["deployments"][0]
    assert report["latest_tag"] == "0.1.1969"
    assert report["state"] == drift_check.STATE_CURRENT
    assert data["summary"]["current_count"] == 1
    assert data["warnings"]


def test_cli_version_floor_mixed_fleet_classifies_both_ways(tmp_path):
    # The parent's observed fleet in one run: with tags ending at 1957 and
    # VERSION at 0.1.1969, a 0.1.1957 deployment is stale (twelve releases
    # behind the approved latest) while a 0.1.1969 deployment stays current.
    args = version_floor_cli_env(tmp_path, "0.1.1969")
    write_dc(tmp_path / "dc", "iad-kalshi", "v0.1.1957")
    proc = _run_cli(args)
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    by_cluster = {r["cluster"]: r for r in data["deployments"]}
    assert by_cluster["iad-ci"]["state"] == drift_check.STATE_CURRENT
    assert by_cluster["iad-kalshi"]["state"] == drift_check.STATE_STALE
    assert by_cluster["iad-kalshi"]["latest_tag"] == "0.1.1969"
    assert data["summary"]["current_count"] == 1
    assert data["summary"]["stale_count"] == 1
    assert data["warnings"]


def test_cli_version_floor_acceptance_1957_stale_1963_current(tmp_path):
    # The parent's acceptance end to end in a single run: tags end at
    # v0.1.1957 and VERSION is 0.1.1969, so --json reports latest 0.1.1969,
    # flags tags_behind_version, a 0.1.1957 deployment classifies stale, and
    # a 0.1.1963 deployment (newer than every tag, <= VERSION) classifies
    # current — in the same --json output.
    args = version_floor_cli_env(tmp_path, "0.1.1963")
    write_dc(tmp_path / "dc", "iad-kalshi", "v0.1.1957")
    proc = _run_cli(args)
    assert proc.returncode == 1, proc.stderr
    data = json.loads(proc.stdout)
    assert data["warnings"], "tags_behind_version must reach the JSON output"
    assert "tags_behind_version" in data["warnings"][0]
    by_cluster = {r["cluster"]: r for r in data["deployments"]}
    assert by_cluster["iad-ci"]["state"] == drift_check.STATE_CURRENT
    assert by_cluster["iad-ci"]["latest_tag"] == "0.1.1969"
    assert by_cluster["iad-kalshi"]["state"] == drift_check.STATE_STALE
    assert by_cluster["iad-kalshi"]["latest_tag"] == "0.1.1969"
    assert data["summary"]["current_count"] == 1
    assert data["summary"]["stale_count"] == 1
    assert data["fingerprint"]


def test_cli_version_matching_newest_tag_stays_quiet(tmp_path):
    # VERSION == newest tag: no floor entry, no warning, exit 0.
    dc = tmp_path / "dc"
    write_dc(dc, "iad-ci", "v0.1.100")
    releases = tmp_path / "releases.json"
    releases.write_text(json.dumps(RELEASES), encoding="utf-8")
    config = tmp_path / "empty-config.json"
    config.write_text("{}", encoding="utf-8")
    version_file = write_version_file(tmp_path, "v0.1.100")
    proc = _run_cli(["--json", "--manifests", str(dc),
                     "--releases-file", str(releases), "--config", str(config),
                     "--version-file", str(version_file)])
    assert proc.returncode == 0, proc.stderr
    data = json.loads(proc.stdout)
    assert data["warnings"] == []
    assert data["deployments"][0]["latest_tag"] == "v0.1.100"
    assert data["fingerprint"] is None
