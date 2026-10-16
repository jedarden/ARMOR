#!/usr/bin/env python3
"""Tests for the release-status documentation gate."""

import hashlib
import json
import shutil
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "documentation-status-gate.py"
STATUS = REPO_ROOT / "config" / "documentation-status.json"


def write_checkpoint(root: Path, states: dict[str, str]) -> None:
    checkpoint = root / ".beads" / "checkpoint"
    (checkpoint / "objects").mkdir(parents=True, exist_ok=True)
    data = "".join(
        json.dumps({"record_type": "issue", "issue": {"id": bead, "base_status": state}})
        + "\n"
        for bead, state in sorted(states.items())
    ).encode()
    digest = hashlib.sha256(data).hexdigest()
    relative = f"objects/{digest}.jsonl"
    (checkpoint / relative).write_bytes(data)
    (checkpoint / "current.json").write_text(
        json.dumps({"active_root": {"path": relative, "sha256": digest}})
    )


def make_tree(tmp_path: Path) -> Path:
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    (root / "config").mkdir()
    (root / "docs").mkdir()
    shutil.copy(SCRIPT, root / "scripts" / SCRIPT.name)
    shutil.copy(STATUS, root / "config" / STATUS.name)
    config = json.loads(STATUS.read_text())
    status_doc = config["status_document"]
    status_lines = ["# Release status", "| Capability | Status |", "|---|---|"]
    states = {}
    for cap in config["capabilities"]:
        beads = cap.get("evidence_beads", [])
        marker = f"<!-- capability: {cap['id']} status: {cap['status']}"
        if beads:
            marker += " evidence: " + " ".join(beads)
        status_lines.append(marker + " -->")
        line = f"| {cap['table_label']} | `{cap['status']}` | {cap['id']}"
        if cap.get("status") == "active-regression":
            line += " active regression not currently supported"
            line += " " + " ".join(beads)
        line += " |"
        status_lines.append(line)
        for bead in beads:
            states[bead] = "closed"
    states["armor-8317f313"] = "deferred"
    states["armor-73564f97"] = "in_progress"
    write_checkpoint(root, states)
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
            "named-key-read active regression not currently supported",
            "named-key-read historical failure",
        )
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "describes an active regression only as historical" in result.stderr


def test_all_cited_beads_closed_cannot_remain_active_regression(tmp_path):
    root = make_tree(tmp_path)
    config = json.loads(STATUS.read_text())
    write_checkpoint(
        root,
        {bead: "closed" for cap in config["capabilities"]
         for bead in cap.get("evidence_beads", [])},
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "named-key-read: active-regression has only closed evidence beads" in result.stderr


def test_repository_tested_cannot_cite_open_bead(tmp_path):
    root = make_tree(tmp_path)
    config_path = root / "config" / STATUS.name
    config = json.loads(config_path.read_text())
    named = next(cap for cap in config["capabilities"] if cap["id"] == "named-key-read")
    named["status"] = "repository-tested"
    config_path.write_text(json.dumps(config))
    status_doc = root / config["status_document"]
    status_doc.write_text(
        status_doc.read_text().replace(
            "capability: named-key-read status: active-regression",
            "capability: named-key-read status: repository-tested",
        ).replace(
            "| Named-key and ring-key reads | `active-regression` |",
            "| Named-key and ring-key reads | `repository-tested` |",
        )
    )
    result = run_gate(root)
    assert result.returncode == 1
    assert "repository-tested cites non-closed bead armor-8317f313" in result.stderr


@pytest.mark.parametrize("document", ["README.md", "docs/cli-reference.md"])
def test_operator_surface_must_link_status_document(tmp_path, document):
    root = make_tree(tmp_path)
    (root / document).write_text("No status link here.\n")
    result = run_gate(root)
    assert result.returncode == 1
    assert f"{document} does not link" in result.stderr
