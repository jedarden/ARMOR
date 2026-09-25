#!/usr/bin/env python3
"""
Tests for scripts/toolchain-parity.sh — the go.mod ↔ Dockerfile toolchain
parity gate.

go.mod's `toolchain` directive is the single source of the Go version and
every golang:<tag> base image in Dockerfile and Dockerfile.test must pin it
exactly (AGENTS.md). The fixtures below prove the gate fails on drift in
either Dockerfile — including a wrong second stage, untagged / digest-pinned
golang bases, and floating minor tags — and fails structurally when the
directive or a Dockerfile is missing. The live-repository test pins the
committed tree to parity, which makes the definition-of-done pytest leg
itself a parity gate.

Run: python3 -m pytest tests/test_toolchain_parity.py -q
"""

import shutil
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "toolchain-parity.sh"

TOOLCHAIN = "go1.25.14"

GOMOD = f"""module github.com/jedarden/armor

go 1.25.0

toolchain {TOOLCHAIN}
"""

VERSION = TOOLCHAIN[2:]  # 1.25.14


def make_repo(tmp_path, dockerfile=None, dockerfile_test=None, gomod=GOMOD):
    """Build a minimal repo tree the gate can run against and return its root.

    The gate resolves the repo from its own location, so the script is copied
    into the fixture — the same layout it sees in the real checkout.
    """
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(SCRIPT, root / "scripts" / "toolchain-parity.sh")
    (root / "go.mod").write_text(gomod)
    if dockerfile is not None:
        (root / "Dockerfile").write_text(dockerfile)
    if dockerfile_test is not None:
        (root / "Dockerfile.test").write_text(dockerfile_test)
    return root


def run_gate(root):
    return subprocess.run(
        ["sh", str(root / "scripts" / "toolchain-parity.sh")],
        cwd=root, capture_output=True, text=True,
    )


DEFAULT_DOCKERFILE = f"""\
FROM golang:{VERSION}-alpine AS builder
RUN CGO_ENABLED=0 ./scripts/release-gate.sh

FROM golang:{VERSION}-alpine AS bead-cli
RUN true

FROM debian:bookworm-slim AS restore-verifier-runtime
FROM scratch AS armor-runtime
"""

DEFAULT_DOCKERFILE_TEST = f"""\
FROM golang:{VERSION}-alpine AS builder
RUN go test ./... -short
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
    assert TOOLCHAIN in proc.stdout


def test_parity_with_non_golang_runtime_stages_passes(tmp_path):
    """debian/scratch runtime stages carry no toolchain and are out of scope;
    both golang stages in the main Dockerfile must match."""
    root = make_repo(
        tmp_path, dockerfile=DEFAULT_DOCKERFILE,
        dockerfile_test=DEFAULT_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


def test_platform_flag_is_skipped(tmp_path):
    dockerfile = f"""\
FROM --platform=$BUILDPLATFORM golang:{VERSION}-alpine AS builder
RUN true
"""
    root = make_repo(
        tmp_path, dockerfile=dockerfile, dockerfile_test=DEFAULT_DOCKERFILE_TEST,
    )
    proc = run_gate(root)
    assert proc.returncode == 0, proc.stdout + proc.stderr


# ---------------------------------------------------------------------------
# drift: exit 1
# ---------------------------------------------------------------------------

def test_dockerfile_builder_drift_fails(tmp_path):
    dockerfile = f"""\
FROM golang:1.25.13-alpine AS builder
RUN true

FROM golang:{VERSION}-alpine AS bead-cli
RUN true
"""
    root = make_repo(tmp_path, dockerfile=dockerfile,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "1.25.13" in proc.stderr and VERSION in proc.stderr
    assert "Dockerfile:" in proc.stderr


def test_second_golang_stage_drift_fails(tmp_path):
    """Both Dockerfile golang stages are checked, not just the first FROM."""
    dockerfile = f"""\
FROM golang:{VERSION}-alpine AS builder
RUN true

FROM golang:1.24.9-alpine AS bead-cli
RUN true
"""
    root = make_repo(tmp_path, dockerfile=dockerfile,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "bead-cli" in proc.stderr


def test_test_dockerfile_drift_fails(tmp_path):
    root = make_repo(
        tmp_path, dockerfile=DEFAULT_DOCKERFILE,
        dockerfile_test="FROM golang:1.26.0-alpine AS builder\nRUN true\n",
    )
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "Dockerfile.test:" in proc.stderr


def test_untagged_golang_base_fails(tmp_path):
    dockerfile = "FROM golang AS builder\nRUN true\n"
    root = make_repo(tmp_path, dockerfile=dockerfile,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "no version tag" in proc.stderr


def test_digest_pinned_golang_base_fails(tmp_path):
    dockerfile = f"FROM golang@sha256:{'a' * 64} AS builder\nRUN true\n"
    root = make_repo(tmp_path, dockerfile=dockerfile,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "digest-pinned" in proc.stderr


def test_floating_minor_tag_fails(tmp_path):
    """`golang:1.25-alpine` does not pin the declared toolchain version."""
    dockerfile = "FROM golang:1.25-alpine AS builder\nRUN true\n"
    root = make_repo(tmp_path, dockerfile=dockerfile,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 1
    assert "pins Go 1.25;" in proc.stderr


# ---------------------------------------------------------------------------
# structural breaks: exit 2
# ---------------------------------------------------------------------------

def test_missing_toolchain_directive_fails(tmp_path):
    gomod = "module github.com/jedarden/armor\n\ngo 1.25.0\n"
    root = make_repo(tmp_path, dockerfile=DEFAULT_DOCKERFILE,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST, gomod=gomod)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "no toolchain directive" in proc.stderr


def test_missing_dockerfile_fails(tmp_path):
    root = make_repo(tmp_path, dockerfile=None,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "Dockerfile not found" in proc.stderr


def test_dockerfile_without_golang_base_fails(tmp_path):
    dockerfile = "FROM debian:bookworm-slim AS builder\nRUN true\n"
    root = make_repo(tmp_path, dockerfile=dockerfile,
                     dockerfile_test=DEFAULT_DOCKERFILE_TEST)
    proc = run_gate(root)
    assert proc.returncode == 2
    assert "no golang base image" in proc.stderr
