#!/usr/bin/env python3
"""Tests for the release-status documentation gate."""

import json
import shutil
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "documentation-status-gate.py"
STATUS = REPO_ROOT / "config" / "documentation-status.json"


def make_tree(tmp_path: Path) -> Path:
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    (root / "config").mkdir()
    (root / "docs").mkdir()
    shutil.copy(SCRIPT, root / "scripts" / SCRIPT.name)
    shutil.copy(STATUS, root / "config" / STATUS.name)
    config = json.loads(STATUS.read_text())
    status_doc = config["status_document"]
    status_lines = ["# Release status"]
    for cap in config["capabilities"]:
        line = f"{cap['id']}: {cap['status']}"
        if cap.get("status") == "active-regression":
            line += " active regression not currently supported"
            line += " " + " ".join(cap.get("evidence_beads", []))
        status_lines.append(line)
    (root / status_doc).write_text("\n".join(status_lines) + "\n")
    for document in config["operator_documents"]:
        path = root / document
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(f"See {status_doc}.\n")
    return root


def run_gate(root: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["python3", str(root / "scripts" / SCRIPT.name), "--root", str(root)],
        cwd=root,
        capture_output=True,
        text=True,
    )


def test_live_tree_passes():
    result = subprocess.run(
        ["python3", str(SCRIPT), "--root", str(REPO_ROOT)],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr


def test_pending_capability_cannot_be_called_release_verified(tmp_path):
    root = make_tree(tmp_path)
    readme = root / "README.md"
    readme.write_text(
        f"Named-key reads are release-verified. See {STATUS.name}.\n"
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "named-key-read" in result.stderr
    assert "release-verified" in result.stderr


def test_pending_capability_cannot_be_called_verified(tmp_path):
    root = make_tree(tmp_path)
    readme = root / "README.md"
    readme.write_text(
        f"Named-key reads are verified. See docs/release-status.md.\n"
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "named-key-read" in result.stderr


def test_pending_capability_can_state_the_limitation(tmp_path):
    root = make_tree(tmp_path)
    readme = root / "README.md"
    status_doc = json.loads(STATUS.read_text())["status_document"]
    readme.write_text(
        f"Named-key reads remain release-pending; see {status_doc}.\n"
    )
    result = run_gate(root)
    assert result.returncode == 0, result.stderr


def test_active_regression_cannot_be_described_only_as_historical(tmp_path):
    root = make_tree(tmp_path)
    readme = root / "README.md"
    readme.write_text(
        f"Named-key reads have a historical failure: armor-8317f313. "
        f"See {STATUS.name}.\n"
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "describes an active regression only as historical" in result.stderr


def test_active_status_requires_current_evidence(tmp_path):
    root = make_tree(tmp_path)
    status_doc = root / json.loads(STATUS.read_text())["status_document"]
    status_doc.write_text(
        status_doc.read_text().replace(
            "named-key-read: active-regression active regression not currently supported",
            "named-key-read: active-regression historical failure",
        )
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "describes an active regression only as historical" in result.stderr


@pytest.mark.parametrize("document", ["README.md", "docs/cli-reference.md"])
def test_operator_surface_must_link_status_document(tmp_path, document):
    root = make_tree(tmp_path)
    (root / document).write_text("No status link here.\n")
    result = run_gate(root)
    assert result.returncode == 1
    assert f"{document} does not link" in result.stderr
