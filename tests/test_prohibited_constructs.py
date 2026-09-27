#!/usr/bin/env python3
"""
Tests for scripts/prohibited-constructs-gate.sh — the prohibited
deployment-constructs gate.

The org's hard rules forbid `.github/workflows/*` files, `kind: Job` /
`kind: CronJob` manifests, and `:latest` or unpinned `ronaldraygun/*` image
references — "including examples and notes". The fixtures under
tests/fixtures/prohibited-constructs/ hold one deliberately-prohibited
document per rule; each test replays a fixture into a scratch tree at its
realistic path (real extension, not the storage `.txt`) and proves the gate
fails on it. The complement is pinned too: pinned-tag / digest / variable-tag
`image:` lines pass, comment lines describing the prohibitions pass, the
fixtures directory itself is exempt, and the live repository is clean — which
makes the definition-of-done pytest leg itself an enforcement point.

Run: python3 -m pytest tests/test_prohibited_constructs.py -q
"""

import shutil
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "prohibited-constructs-gate.sh"
FIXTURES = REPO_ROOT / "tests" / "fixtures" / "prohibited-constructs"

# (fixture file, path it represents in a real tree, gate rule label that
# must appear in the gate's output)
FAILING_FIXTURES = [
    ("github-workflows-file.txt", ".github/workflows/ci.yml", "[workflow-file]"),
    ("kind-job.txt", "manifests/job.yaml", "[kind-job-cronjob]"),
    ("kind-cronjob.txt", "manifests/cronjob.yaml", "[kind-job-cronjob]"),
    ("image-latest-tag.txt", "manifests/deploy.yaml", "[image-latest]"),
    ("ronaldraygun-latest-note.txt", "docs/runbook-note.md", "[ronaldraygun-latest]"),
    ("ronaldraygun-unpinned.txt", "manifests/deploy.yaml", "[ronaldraygun-unpinned]"),
]

CLEAN_DEPLOYMENT = """apiVersion: apps/v1
kind: Deployment
metadata:
  name: armor
spec:
  template:
    spec:
      containers:
      - name: armor
        image: ronaldraygun/armor:0.1.1977
      - name: verifier
        image: ronaldraygun/armor-restore-verifier:$IMAGE_TAG
      - name: quoted
        image: "ronaldraygun/armor:<version>"
      - name: digested
        image: ronaldraygun/armor@sha256:{}
      - name: mirror
        image: ghcr.io/jedarden/armor:${{ARMOR_VERSION:-0.1.1977}}
""".format("a" * 64)


def make_repo(tmp_path, files=None):
    """Build a scratch tree the gate can run against and return its root.

    The gate resolves the repo from its own location, so the script is copied
    into the fixture — the same layout it sees in the real checkout.
    """
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(SCRIPT, root / "scripts" / "prohibited-constructs-gate.sh")
    for rel, content in (files or {}).items():
        target = root / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)
    return root


def run_gate(root):
    return subprocess.run(
        ["sh", str(root / "scripts" / "prohibited-constructs-gate.sh")],
        cwd=root, capture_output=True, text=True,
    )


@pytest.mark.parametrize(
    "fixture_name,tree_path,label", FAILING_FIXTURES,
    ids=[name.rsplit(".", 1)[0] for name, _, _ in FAILING_FIXTURES],
)
def test_gate_fails_on_prohibited_fixture(tmp_path, fixture_name, tree_path, label):
    """Every prohibited construct fails the gate, at its real path.

    The fixture is stored flat with a neutral extension (the constructs are
    prohibited in real manifests, so this tree must not carry them under a
    manifest name); the scratch copy gets the realistic path.
    """
    content = (FIXTURES / fixture_name).read_text()
    root = make_repo(tmp_path, {tree_path: content})
    result = run_gate(root)
    assert result.returncode == 1, result.stderr
    assert label in result.stderr
    assert tree_path in result.stderr


def test_gate_passes_on_pinned_and_variable_tags(tmp_path):
    root = make_repo(tmp_path, {"manifests/deploy.yaml": CLEAN_DEPLOYMENT})
    result = run_gate(root)
    assert result.returncode == 0, result.stderr


# The gate scans this file too, so the probes below are assembled rather
# than stated: a literal ronaldraygun/armor tag-along ref here would be a
# real hit, not a documentation line.
RRG = "ronaldraygun/armor"
LATEST = ":" + "latest"


def test_gate_passes_when_rules_are_only_described(tmp_path):
    """Comment lines describing the prohibitions never trip the gate.

    The rules must stay documentable — the org guard draws the same line
    (real manifest lines, never comments).
    """
    notes = (
        f"# image: {RRG}{LATEST} is prohibited\n"
        "# kind: Job and kind: CronJob are prohibited\n"
        f"// {RRG}{LATEST}\n"
        "The rules hold for notes too: no kind: Job anywhere.\n"
    )
    root = make_repo(tmp_path, {"docs/rules.md": notes})
    result = run_gate(root)
    assert result.returncode == 0, result.stderr


def test_gate_exempts_only_its_own_fixtures_directory(tmp_path):
    """The fixtures directory is exempt by name; the same content next door
    still fails (the parametrized cases above prove that direction too).
    """
    content = (FIXTURES / "kind-job.txt").read_text()
    root = make_repo(tmp_path, {
        "tests/fixtures/prohibited-constructs/kind-job.txt": content,
    })
    result = run_gate(root)
    assert result.returncode == 0, result.stderr


def test_gate_passes_on_this_repository():
    """The live tree stays clean — this test is itself an enforcement point.

    Runs the committed gate against the checkout, so any prohibited
    construct introduced anywhere in the repository fails the definition of
    done.
    """
    result = subprocess.run(
        ["sh", str(SCRIPT)], cwd=REPO_ROOT, capture_output=True, text=True,
    )
    assert result.returncode == 0, result.stderr
