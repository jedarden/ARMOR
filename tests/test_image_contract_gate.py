#!/usr/bin/env python3
"""
Tests for scripts/image-contract-gate.sh — the Dockerfile default-image
contract gate.

An untargeted `docker build -f Dockerfile` publishes the LAST stage as
ronaldraygun/armor, so that stage must be the armor server:
ENTRYPOINT ["/armor"] in exec form (a shell-form entrypoint cannot execute
on FROM scratch) and no CMD. The companion images are published from the
same file via explicit --target stages — restore-verifier-runtime and
armor-fleet-runtime — which must stay addressable under those names with
their own entrypoints. Images 0.1.1833–0.1.1870 shipped /restore-verifier
as the default entrypoint because a multi-stage refactor left the
restore-verifier runtime stage last; deployed pods crash-looped. The
fixtures below replay that shape and the other refactors this gate exists
to stop, and the live-repository test pins the committed tree, which makes
the definition-of-done pytest leg itself an enforcement point.

Run: python3 -m pytest tests/test_image_contract_gate.py -q
"""

import shutil
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "image-contract-gate.sh"


def make_repo(tmp_path, dockerfile=None, dockerfile_test=None):
    """Build a minimal repo tree the gate can run against and return its root.

    The gate resolves the repo from its own location, so the script is copied
    into the fixture — the same layout it sees in the real checkout.
    """
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(SCRIPT, root / "scripts" / "image-contract-gate.sh")
    if dockerfile is not None:
        (root / "Dockerfile").write_text(dockerfile)
    if dockerfile_test is not None:
        (root / "Dockerfile.test").write_text(dockerfile_test)
    return root


def run_gate(root):
    return subprocess.run(
        ["sh", str(root / "scripts" / "image-contract-gate.sh")],
        cwd=root, capture_output=True, text=True,
    )


# The repo's real shape, minimised into composable stage blocks: builder,
# both companion runtime stages, armor runtime LAST (the published default
# image). Base-image pins are toolchain-parity.sh's concern; only stage
# shape matters here.
BUILDER_STAGE = """\
FROM golang:1.25.14-alpine AS builder
RUN CGO_ENABLED=0 go build -o /armor ./cmd/armor

"""

VERIFIER_STAGE = """\
FROM debian:bookworm-slim AS restore-verifier-runtime
COPY --from=builder /restore-verifier /restore-verifier
EXPOSE 9002
ENTRYPOINT ["/restore-verifier"]

"""

FLEET_STAGE = """\
FROM scratch AS armor-fleet-runtime
COPY --from=builder /armor-fleet /armor-fleet
EXPOSE 8080
ENTRYPOINT ["/armor-fleet"]

"""

ARMOR_STAGE = """\
# Runtime stage for armor — final stage = default build target
FROM scratch AS armor-runtime
COPY --from=builder /armor /armor
EXPOSE 9000 9001
ENTRYPOINT ["/armor"]
"""

GOOD_DOCKERFILE = BUILDER_STAGE + VERIFIER_STAGE + FLEET_STAGE + ARMOR_STAGE

GOOD_DOCKERFILE_TEST = """\
FROM golang:1.25.14-alpine AS builder
RUN go test ./... -short

FROM scratch
COPY --from=builder /armor /armor
ENTRYPOINT ["/armor"]
"""


# ---------------------------------------------------------------------------
# pass cases
# ---------------------------------------------------------------------------

def test_live_repository_honours_the_contract():
    """The committed tree passes its own gate — DoD fails here on any drift."""
    proc = subprocess.run(
        ["sh", str(SCRIPT)], cwd=REPO_ROOT, capture_output=True, text=True,
    )
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert "Image contract" in proc.stdout
    assert "/armor" in proc.stdout


def test_good_shape_passes(tmp_path):
    root = make_repo(
        tmp_path, dockerfile=GOOD_DOCKERFILE,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stderr


def test_final_stage_name_is_not_pinned(tmp_path):
    """Renaming the final stage changes nothing an untargeted build
    publishes; only the entrypoint/command contract and the companion
    --target names are pinned (see the gate header)."""
    dockerfile = GOOD_DOCKERFILE.replace(
        "AS armor-runtime", "AS armor-server-runtime")
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stderr


def test_whitespace_and_unnamed_final_stage_pass(tmp_path):
    """ENTRYPOINT spacing is normalised, and the final stage needs no name
    (Dockerfile.test ships exactly that shape)."""
    dockerfile = GOOD_DOCKERFILE.replace(
        'FROM scratch AS armor-runtime\nCOPY --from=builder /armor /armor\n'
        'EXPOSE 9000 9001\nENTRYPOINT ["/armor"]',
        'FROM scratch\nCOPY --from=builder /armor /armor\n'
        'ENTRYPOINT [ "/armor" ]')
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stderr


# ---------------------------------------------------------------------------
# contract violations: exit 1
# ---------------------------------------------------------------------------

def test_companion_stage_last_fails(tmp_path):
    """THE incident (images 0.1.1833–0.1.1870): restore-verifier-runtime
    reordered to the end publishes /restore-verifier as ronaldraygun/armor."""
    dockerfile = BUILDER_STAGE + FLEET_STAGE + ARMOR_STAGE + "\n" + VERIFIER_STAGE
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "final stage restore-verifier-runtime" in proc.stderr
    assert 'expected ENTRYPOINT ["/armor"]' in proc.stderr
    assert "would publish /restore-verifier" in proc.stderr


def test_wrong_final_entrypoint_fails(tmp_path):
    dockerfile = GOOD_DOCKERFILE.replace(
        'FROM scratch AS armor-runtime\nCOPY --from=builder /armor /armor\n'
        'EXPOSE 9000 9001\nENTRYPOINT ["/armor"]',
        'FROM scratch AS armor-runtime\n'
        'COPY --from=builder /armor-fleet /armor-fleet\n'
        'ENTRYPOINT ["/armor-fleet"]')
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert 'expected ENTRYPOINT ["/armor"]' in proc.stderr


def test_shell_form_final_entrypoint_fails(tmp_path):
    """Shell form wraps the entrypoint in /bin/sh -c, which cannot execute
    on a FROM scratch image — the contract is the exec form."""
    dockerfile = GOOD_DOCKERFILE.replace(
        'ENTRYPOINT ["/armor"]\n', 'ENTRYPOINT /armor\n')
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "exec form" in proc.stderr


def test_final_stage_cmd_fails(tmp_path):
    dockerfile = GOOD_DOCKERFILE.replace(
        'ENTRYPOINT ["/armor"]\n', 'ENTRYPOINT ["/armor"]\nCMD ["serve"]\n')
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "declares a CMD" in proc.stderr


def test_final_stage_without_entrypoint_fails(tmp_path):
    dockerfile = GOOD_DOCKERFILE.replace('ENTRYPOINT ["/armor"]\n', "")
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "declares no ENTRYPOINT" in proc.stderr


def test_missing_companion_target_fails(tmp_path):
    """CI publishes ronaldraygun/armor-restore-verifier via
    docker build --target restore-verifier-runtime; deleting the named
    stage takes that addressability away."""
    dockerfile = BUILDER_STAGE + FLEET_STAGE + ARMOR_STAGE
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "--target stage restore-verifier-runtime is gone" in proc.stderr


def test_renamed_companion_target_fails(tmp_path):
    """A renamed companion stage is still a stage — but no longer the one
    the release pipeline targets, so the name is pinned."""
    dockerfile = GOOD_DOCKERFILE.replace(
        "AS restore-verifier-runtime", "AS verifier-runtime")
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "--target stage restore-verifier-runtime is gone" in proc.stderr


def test_companion_entrypoint_drift_fails(tmp_path):
    dockerfile = GOOD_DOCKERFILE.replace(
        'ENTRYPOINT ["/armor-fleet"]', 'ENTRYPOINT ["/armor"]')
    root = make_repo(
        tmp_path, dockerfile=dockerfile,
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert 'companion stage armor-fleet-runtime ENTRYPOINT' in proc.stderr
    assert 'expected ["/armor-fleet"]' in proc.stderr


def test_test_dockerfile_default_image_fails(tmp_path):
    """Dockerfile.test is held to the same default-image rule: its last
    stage builds ronaldraygun/armor-test."""
    dockerfile_test = GOOD_DOCKERFILE_TEST.replace(
        'ENTRYPOINT ["/armor"]', 'ENTRYPOINT ["/restore-verifier"]')
    root = make_repo(
        tmp_path, dockerfile=GOOD_DOCKERFILE,
        dockerfile_test=dockerfile_test,
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "Dockerfile.test: final stage" in proc.stderr


# ---------------------------------------------------------------------------
# structural breaks: exit 2
# ---------------------------------------------------------------------------

def test_missing_dockerfile_fails(tmp_path):
    root = make_repo(tmp_path, dockerfile=None,
                     dockerfile_test=GOOD_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "Dockerfile not found" in proc.stderr


def test_dockerfile_without_stages_fails(tmp_path):
    root = make_repo(
        tmp_path, dockerfile="# no stages\nRUN true\n",
        dockerfile_test=GOOD_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "no FROM stages" in proc.stderr
