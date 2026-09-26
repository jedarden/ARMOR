#!/usr/bin/env python3
"""
Tests for scripts/compose-version-parity.sh — the compose.yaml ↔ VERSION
image-pin parity gate.

compose.yaml pins its demo and production images through
${ARMOR_VERSION:-<version>} defaults, the README Quick Start points readers
at the repository's VERSION-file tag, and scripts/cut-release.sh bumps the
defaults in the release commit. The fixtures below prove the gate fails on
drift in either default — including a floating or non-semver token — and
fails structurally when VERSION or compose.yaml is missing or carries no
default to check. The live-repository test pins the committed tree to
parity, which makes the definition-of-done pytest leg itself a parity gate
and would have caught the 0.1.1973-vs-0.1.1975 lag that motivated the gate.

Run: python3 -m pytest tests/test_compose_version_parity.py -q
"""

import shutil
import subprocess
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "compose-version-parity.sh"

VERSION = "0.1.9999"


def make_repo(tmp_path, compose=None, version=VERSION, with_version=True):
    """Build a minimal repo tree the gate can run against and return its root.

    The gate resolves the repo from its own location, so the script is copied
    into the fixture — the same layout it sees in the real checkout.
    """
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(SCRIPT, root / "scripts" / "compose-version-parity.sh")
    if with_version:
        (root / "VERSION").write_text(version + "\n")
    if compose is not None:
        (root / "compose.yaml").write_text(compose)
    return root


def run_gate(root):
    return subprocess.run(
        ["sh", str(root / "scripts" / "compose-version-parity.sh")],
        cwd=root, capture_output=True, text=True,
    )


def compose_yaml(version=VERSION):
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


# ---------------------------------------------------------------------------
# pass cases
# ---------------------------------------------------------------------------

def test_live_repository_is_in_parity():
    """The committed tree passes its own gate — DoD fails here on any drift."""
    proc = subprocess.run(
        ["sh", str(SCRIPT)], cwd=REPO_ROOT, capture_output=True, text=True,
    )
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert (REPO_ROOT / "VERSION").read_text().strip() in proc.stdout


def test_both_defaults_in_parity_passes(tmp_path):
    root = make_repo(tmp_path, compose=compose_yaml())
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert VERSION in proc.stdout


def test_comment_mention_of_the_variable_is_not_a_default(tmp_path):
    """Prose naming ARMOR_VERSION without a ${...:-} interpolation carries no
    default and must neither satisfy nor fail the gate on its own."""
    compose = compose_yaml() + "# Set ARMOR_VERSION in the environment to override.\n"
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


# ---------------------------------------------------------------------------
# drift: exit 1
# ---------------------------------------------------------------------------

def test_first_default_drift_fails(tmp_path):
    compose = compose_yaml().replace(
        f"ARMOR_VERSION:-{VERSION}", "ARMOR_VERSION:-0.1.1973", 1
    )
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "0.1.1973" in proc.stderr and VERSION in proc.stderr
    assert "compose.yaml:3:" in proc.stderr


def test_second_default_drift_fails(tmp_path):
    """Both profiles are checked, not just the first default."""
    compose = compose_yaml().replace(
        f"ARMOR_VERSION:-{VERSION}", "ARMOR_VERSION:-0.1.1973"
    )
    # Repair only the demo default (line 3); production (line 7) stays stale.
    compose = compose.replace(
        "ARMOR_VERSION:-0.1.1973", f"ARMOR_VERSION:-{VERSION}", 1
    )
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "compose.yaml:7:" in proc.stderr


def test_floating_two_component_default_fails(tmp_path):
    """A default that is not MAJOR.MINOR.PATCH is drift, not parity."""
    compose = compose_yaml().replace(f"ARMOR_VERSION:-{VERSION}", "ARMOR_VERSION:-0.1")
    root = make_repo(tmp_path, compose=compose)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "0.1" in proc.stderr


# ---------------------------------------------------------------------------
# structural breaks: exit 2
# ---------------------------------------------------------------------------

def test_no_default_fails(tmp_path):
    """A hardcoded tag instead of the interpolation leaves nothing to check."""
    compose = compose_yaml().replace(
        f"ghcr.io/jedarden/armor:${{ARMOR_VERSION:-{VERSION}}}",
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
    root = make_repo(tmp_path, compose=compose_yaml(), with_version=False)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "VERSION not found" in proc.stderr


def test_non_semver_version_file_fails(tmp_path):
    root = make_repo(tmp_path, compose=compose_yaml(), version="not-a-version")
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "not MAJOR.MINOR.PATCH" in proc.stderr
