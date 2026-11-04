#!/usr/bin/env python3
"""
Tests for scripts/compose-version-parity.sh and
scripts/record-green-release.sh — the consumer image-pin parity gate and
the one-edit script that moves the pins (armor-19792903).

Three surfaces must name one version, the release docs/release-status.md
records as the latest fully published, compat-green one: the register's
`release: latest-green` marker, the README Quick Start's
`export ARMOR_VERSION=` line, and every `${ARMOR_VERSION:-<version>}`
default in compose.yaml. A cut whose publication is incomplete (0.1.2006)
leaves every pin where it was, so a consumer never lands on a tag without
releases; scripts/record-green-release.sh moves all three once a
publication is verified.

The fixtures below prove the gate fails on drift in any surface — a
mismatched README export, a drifted compose default, a floating token, a
register naming a release newer than VERSION — and fails structurally on a
missing or duplicated marker, a missing export or default, or missing
files. The live-repository test pins the committed tree to parity, which
makes the definition-of-done pytest leg itself a parity gate. The
record-green-release tests prove the three surfaces move together, refuse
a version that was never cut or is already recorded, and leave a
structurally broken tree untouched.

Run: python3 -m pytest tests/test_compose_version_parity.py -q
"""

import shutil
import subprocess
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "compose-version-parity.sh"
RECORDER = REPO_ROOT / "scripts" / "record-green-release.sh"

VERSION = "0.1.9999"          # the checkout's VERSION (development counter)
GREEN = "0.1.9997"            # the register's latest-green release
NEXT_GREEN = "0.1.9998"       # a cut, not yet recorded


def status_md(version=GREEN):
    """The register's pin-bearing marker line in minimal prose."""
    return (
        "# Status\n\n"
        f"<!-- release: latest-green version: {version} evidence: armor-aaa1111 -->\n\n"
        "**v%s** is the latest fully published, compat-green release.\n" % version
    )


def readme_md(version=GREEN):
    """The README's pin-bearing export line in minimal prose."""
    return f"# ARMOR\n\n```bash\nexport ARMOR_VERSION={version}\n```\n"


def compose_yaml(version=GREEN):
    """The tracked compose.yaml's pin-bearing lines, nothing else."""
    return f"""\
services:
  armor-demo:
    image: ghcr.io/jedarden/armor:${{ARMOR_VERSION:-{version}}}
    profiles: [demo]

  armor-production:
    image: ghcr.io/jedarden/armor:${{ARMOR_VERSION:-{version}}}
    profiles: [production]
"""


def make_repo(tmp_path, compose="default", readme="default", status="default",
              version=VERSION, with_version=True, with_readme=True,
              with_status=True):
    """Build a minimal repo tree the gate can run against and return its
    root. The gate resolves the repo from its own location, so the script
    is copied into the fixture — the same layout it sees in the real
    checkout. Every consumer pin defaults to GREEN; VERSION defaults to a
    development counter ahead of it, the shape the real checkout carries
    between a cut and its green record. Pass a string to override a
    surface's content, None to omit the file entirely."""
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(SCRIPT, root / "scripts" / "compose-version-parity.sh")
    if with_version:
        (root / "VERSION").write_text(version + "\n")
    if compose == "default":
        compose = compose_yaml()
    if compose is not None:
        (root / "compose.yaml").write_text(compose)
    if readme == "default":
        readme = readme_md()
    if with_readme and readme is not None:
        (root / "README.md").write_text(readme)
    if status == "default":
        status = status_md()
    if with_status and status is not None:
        (root / "docs").mkdir()
        (root / "docs" / "release-status.md").write_text(status)
    return root


def run_gate(root):
    return subprocess.run(
        ["sh", str(root / "scripts" / "compose-version-parity.sh")],
        cwd=root, capture_output=True, text=True,
    )


def run_recorder(root, version):
    return subprocess.run(
        ["bash", str(root / "scripts" / "record-green-release.sh"), version],
        cwd=root, capture_output=True, text=True,
    )


# ---------------------------------------------------------------------------
# pass cases
# ---------------------------------------------------------------------------

def test_live_repository_is_in_parity():
    """The committed tree passes its own gate — DoD fails here on any drift."""
    proc = subprocess.run(
        ["sh", str(SCRIPT)], cwd=REPO_ROOT, capture_output=True, text=True,
    )
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert "latest-green release" in proc.stdout
    # The pinned version really is what the register records.
    marker = next(
        line for line in (REPO_ROOT / "docs" / "release-status.md").read_text().splitlines()
        if "release: latest-green version:" in line
    )
    recorded = marker.split("version:")[1].strip().split()[0]
    assert recorded in proc.stdout
    assert (REPO_ROOT / "README.md").read_text().count(f"export ARMOR_VERSION={recorded}") == 1
    assert (REPO_ROOT / "compose.yaml").read_text().count(f"ARMOR_VERSION:-{recorded}") == 2


def test_all_pins_at_the_recorded_green_release_pass(tmp_path):
    root = make_repo(tmp_path)
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert GREEN in proc.stdout


def test_version_ahead_of_green_passes(tmp_path):
    """The checkout's VERSION may run ahead between a cut and its green
    record — that gap is the rule working, not drift."""
    root = make_repo(tmp_path, version="0.1.9999")
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


def test_version_equal_to_green_passes(tmp_path):
    """Steady state right after a green record: the counter and the pins
    name the same release."""
    root = make_repo(tmp_path, version=GREEN)
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


def test_comment_mention_of_the_variable_is_not_a_default(tmp_path):
    """Prose naming ARMOR_VERSION without a ${...:-} interpolation carries no
    default and must neither satisfy nor fail the gate on its own."""
    compose = compose_yaml() + "# Set ARMOR_VERSION in the environment to override.\n"
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


def test_readme_prose_mention_is_not_an_export(tmp_path):
    """README prose naming ARMOR_VERSION is not a pin; only the export line
    is checked."""
    readme = readme_md() + "Set `ARMOR_VERSION` in `.env` to override the default.\n"
    root = make_repo(tmp_path, readme=readme)
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


# ---------------------------------------------------------------------------
# drift: exit 1
# ---------------------------------------------------------------------------

def test_first_compose_default_drift_fails(tmp_path):
    compose = compose_yaml().replace(
        f"ARMOR_VERSION:-{GREEN}", "ARMOR_VERSION:-0.1.1973", 1
    )
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "0.1.1973" in proc.stderr and GREEN in proc.stderr
    assert "compose.yaml:3:" in proc.stderr


def test_second_compose_default_drift_fails(tmp_path):
    """Both profiles are checked, not just the first default."""
    compose = compose_yaml().replace(
        f"ARMOR_VERSION:-{GREEN}", "ARMOR_VERSION:-0.1.1973"
    )
    # Repair only the demo default (line 3); production (line 7) stays stale.
    compose = compose.replace(
        "ARMOR_VERSION:-0.1.1973", f"ARMOR_VERSION:-{GREEN}", 1
    )
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "compose.yaml:7:" in proc.stderr


def test_mismatched_readme_export_fails(tmp_path):
    """The acceptance case (armor-19792903): a README export left at an old
    release while the register and compose name the green one is drift."""
    readme = readme_md(version="0.1.1973")
    root = make_repo(tmp_path, readme=readme)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "0.1.1973" in proc.stderr and GREEN in proc.stderr
    assert "README.md:4:" in proc.stderr
    assert "README ARMOR_VERSION drift" in proc.stderr


def test_floating_two_component_default_fails(tmp_path):
    """A default that is not MAJOR.MINOR.PATCH is drift, not parity."""
    compose = compose_yaml().replace(f"ARMOR_VERSION:-{GREEN}", "ARMOR_VERSION:-0.1")
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "0.1" in proc.stderr


def test_green_newer_than_version_fails(tmp_path):
    """The register cannot record a green release that was never cut."""
    root = make_repo(tmp_path, version="0.1.9995")  # VERSION behind the register
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "newer than the checkout's VERSION" in proc.stderr


# ---------------------------------------------------------------------------
# structural breaks: exit 2
# ---------------------------------------------------------------------------

def test_no_marker_fails(tmp_path):
    root = make_repo(tmp_path, status="# Status\n\nNo marker at all.\n")
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "exactly one" in proc.stderr


def test_two_markers_fail(tmp_path):
    status = status_md() + f"\n<!-- release: latest-green version: {NEXT_GREEN} -->\n"
    root = make_repo(tmp_path, status=status)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "exactly one" in proc.stderr


def test_non_semver_marker_fails(tmp_path):
    root = make_repo(tmp_path, status=status_md(version="0.1x"))
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "not MAJOR.MINOR.PATCH" in proc.stderr


def test_missing_status_document_fails(tmp_path):
    root = make_repo(tmp_path, with_status=False)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "release-status.md not found" in proc.stderr


def test_readme_without_an_export_fails(tmp_path):
    root = make_repo(tmp_path, readme="# ARMOR\n\nNo pin here.\n")
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "no export ARMOR_VERSION= line" in proc.stderr


def test_two_export_lines_fail(tmp_path):
    """Exactly one README line may carry the pin, even when they agree."""
    readme = readme_md() + f"export ARMOR_VERSION={GREEN}\n"
    root = make_repo(tmp_path, readme=readme)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "exactly one" in proc.stderr


def test_missing_readme_fails(tmp_path):
    root = make_repo(tmp_path, with_readme=False)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "README.md not found" in proc.stderr


def test_no_compose_default_fails(tmp_path):
    """A hardcoded tag instead of the interpolation leaves nothing to check."""
    compose = compose_yaml().replace(
        f"ghcr.io/jedarden/armor:${{ARMOR_VERSION:-{GREEN}}}",
        "ghcr.io/jedarden/armor:0.1.9998",
    )
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "no ${ARMOR_VERSION" in proc.stderr


def test_missing_compose_fails(tmp_path):
    root = make_repo(tmp_path, compose=None)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "compose.yaml not found" in proc.stderr


def test_missing_version_file_fails(tmp_path):
    root = make_repo(tmp_path, with_version=False)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "VERSION not found" in proc.stderr


def test_non_semver_version_file_fails(tmp_path):
    root = make_repo(tmp_path, version="not-a-version")
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "not MAJOR.MINOR.PATCH" in proc.stderr


# ---------------------------------------------------------------------------
# record-green-release.sh: the one-edit pin move
# ---------------------------------------------------------------------------

def make_recordable_repo(tmp_path, green=GREEN, version=VERSION):
    root = make_repo(tmp_path, version=version)
    shutil.copy(RECORDER, root / "scripts" / "record-green-release.sh")
    return root


def test_recorder_moves_all_three_pins_together(tmp_path):
    root = make_recordable_repo(tmp_path)
    assert run_gate(root).returncode == 0

    proc = run_recorder(root, NEXT_GREEN)
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert f"recorded latest-green {NEXT_GREEN}" in proc.stdout
    assert f"(was {GREEN})" in proc.stdout

    status = (root / "docs" / "release-status.md").read_text()
    assert f"release: latest-green version: {NEXT_GREEN}" in status
    assert f"release: latest-green version: {GREEN}" not in status
    readme = (root / "README.md").read_text()
    assert f"export ARMOR_VERSION={NEXT_GREEN}" in readme
    assert f"export ARMOR_VERSION={GREEN}" not in readme
    compose = (root / "compose.yaml").read_text()
    assert compose.count(f"ARMOR_VERSION:-{NEXT_GREEN}") == 2
    assert f"ARMOR_VERSION:-{GREEN}" not in compose

    gate = run_gate(root)
    assert gate.returncode == 0, gate.stdout + gate.stderr
    assert NEXT_GREEN in gate.stdout


def test_recorder_refuses_a_version_that_was_never_cut(tmp_path):
    root = make_recordable_repo(tmp_path)
    proc = run_recorder(root, "0.1.10000")
    assert proc.returncode == 2
    assert "newer than the checkout's VERSION" in proc.stderr
    # Nothing moved.
    assert run_gate(root).returncode == 0
    assert GREEN in (root / "README.md").read_text()


def test_recorder_refuses_the_already_recorded_version(tmp_path):
    root = make_recordable_repo(tmp_path)
    proc = run_recorder(root, GREEN)
    assert proc.returncode == 2
    assert "already" in proc.stderr


def test_recorder_refuses_a_malformed_version(tmp_path):
    root = make_recordable_repo(tmp_path)
    proc = run_recorder(root, "not-a-version")
    assert proc.returncode == 2
    assert "is not MAJOR.MINOR.PATCH" in proc.stderr


def test_recorder_needs_the_marker_and_leaves_a_broken_tree_alone(tmp_path):
    """A register without exactly one marker is refused before any pin is
    rewritten."""
    root = make_recordable_repo(tmp_path)
    (root / "docs" / "release-status.md").write_text("# Status\n\nno marker\n")
    pins = (root / "README.md").read_text()
    proc = run_recorder(root, NEXT_GREEN)
    assert proc.returncode == 2
    assert "exactly one" in proc.stderr
    assert (root / "README.md").read_text() == pins
