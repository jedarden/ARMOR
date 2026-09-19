"""Tests for scripts/publish_release.py.

Every HTTP call is intercepted by a fake urlopen so the tests never touch
Forgejo, GitHub, or a registry. The fake records mutations so idempotency
and refusal-to-move-a-tag can be asserted precisely.
"""

from __future__ import annotations

import io
import json
import sys
from email.message import Message
from pathlib import Path
from urllib.error import HTTPError

import pytest

SCRIPTS_DIR = Path(__file__).resolve().parents[1] / "scripts"
sys.path.insert(0, str(SCRIPTS_DIR))

import publish_release as pr  # noqa: E402

VERSION = "0.1.1969"
COMMIT = "96ba656f73dcb337fc71f02434315df03a623b13"
OTHER = "0" * 40
TAG = f"v{VERSION}"
FJ = pr.DEFAULT_FORGEJO_API
GH = pr.DEFAULT_GITHUB_API
REPO = pr.DEFAULT_REPO
GHCR_DIGEST = "sha256:" + "ab" * 32
GHCR_RV_DIGEST = "sha256:" + "ef" * 32
HUB_DIGEST = "sha256:" + "cd" * 32


class FakeResponse:
    def __init__(self, status: int, body, headers: dict | None = None):
        self.status = status
        self._raw = (json.dumps(body).encode() if body is not None else b"")
        self.headers = Message()
        for k, v in (headers or {}).items():
            self.headers[k] = v

    def read(self) -> bytes:
        return self._raw

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False


class FakeApi:
    """Route (METHOD, url-without-query) -> (status, body[, headers]).

    Unrouted requests raise HTTPError 404. Every request is recorded in
    ``calls`` as (method, url, payload, auth_present).
    """

    def __init__(self):
        self.routes: dict[tuple[str, str], tuple] = {}
        self.calls: list[tuple[str, str, dict | None, bool]] = []

    def route(self, method: str, url: str, status: int, body=None, headers: dict | None = None):
        self.routes[(method, url)] = (status, body, headers or {})

    def __call__(self, req, timeout=None):
        url = req.full_url.split("?", 1)[0]
        payload = json.loads(req.data) if req.data else None
        auth = bool(req.get_header("Authorization"))
        self.calls.append((req.get_method(), url, payload, auth))
        entry = self.routes.get((req.get_method(), url))
        if entry is None:
            raise HTTPError(req.full_url, 404, "not found", Message(), io.BytesIO(b'{"message":"Not Found"}'))
        status, body, headers = entry
        if status >= 400:
            raise HTTPError(req.full_url, status, "error", Message(), io.BytesIO(json.dumps(body or {}).encode()))
        return FakeResponse(status, body, headers)

    def mutations(self) -> list[tuple[str, str]]:
        return [(m, u) for m, u, _, _ in self.calls if m in ("POST", "PATCH", "PUT", "DELETE")]


@pytest.fixture
def api(monkeypatch):
    fake = FakeApi()
    monkeypatch.setattr(pr, "urlopen", fake)
    # Public GHCR digests are always resolvable in these tests.
    fake.route("GET", "https://ghcr.io/token", 200, {"token": "anon"})
    fake.route(
        "HEAD",
        f"https://ghcr.io/v2/jedarden/armor/manifests/{VERSION}",
        200,
        None,
        {"Docker-Content-Digest": GHCR_DIGEST},
    )
    fake.route(
        "HEAD",
        f"https://ghcr.io/v2/jedarden/armor-restore-verifier/manifests/{VERSION}",
        200,
        None,
        {"Docker-Content-Digest": GHCR_RV_DIGEST},
    )
    return fake


@pytest.fixture
def tokens(monkeypatch):
    monkeypatch.setenv("FORGEJO_TOKEN", "fj-test-token")
    monkeypatch.setenv("GITHUB_TOKEN", "gh-test-token")
    monkeypatch.delenv("DOCKER_CONFIG_JSON", raising=False)


def run(*extra, sleep=lambda s: None):
    return pr.main(["--version", VERSION, "--commit", COMMIT, "--github-wait-seconds", "0", *extra], sleep=sleep)


def outcome(capsys) -> dict:
    return json.loads(capsys.readouterr().out)


# ------------------------------------------------------------ validation


def test_rejects_non_semver_version(tokens, api):
    assert pr.main(["--version", "v0.1.1969", "--commit", COMMIT]) == pr.EXIT_USAGE
    assert api.calls == []


def test_rejects_abbreviated_commit(tokens, api):
    assert pr.main(["--version", VERSION, "--commit", COMMIT[:8]]) == pr.EXIT_USAGE
    assert api.calls == []


def test_requires_forgejo_token_unless_dry_run(monkeypatch, api):
    monkeypatch.delenv("FORGEJO_TOKEN", raising=False)
    monkeypatch.delenv("GITHUB_TOKEN", raising=False)
    assert pr.main(["--version", VERSION, "--commit", COMMIT]) == pr.EXIT_USAGE
    assert api.calls == []


# --------------------------------------------------------- fresh publish


def test_fresh_publish_creates_tag_and_both_releases(tokens, api, capsys):
    api.route("POST", f"{FJ}/repos/{REPO}/tags", 201, {"name": TAG})
    api.route("POST", f"{FJ}/repos/{REPO}/releases", 201, {"id": 7, "html_url": "https://git/x/releases/tag/v"})
    api.route("GET", f"{GH}/repos/{REPO}/git/ref/tags/{TAG}", 200, {"ref": f"refs/tags/{TAG}"})
    api.route("POST", f"{GH}/repos/{REPO}/releases", 201, {"id": 9, "html_url": "https://github.com/x/releases/tag/v"})

    assert run("--workflow", "armor-build-abc12") == pr.EXIT_OK
    out = outcome(capsys)
    assert out["forgejo_tag"] == "created"
    assert out["forgejo_release"] == "created"
    assert out["github_tag_visible"] is True
    assert out["github_release"] == "created"
    assert out["urls"]["forgejo"].startswith("https://git/")
    assert out["urls"]["github"].startswith("https://github.com/")

    posts = {u: p for m, u, p, _ in api.calls if m == "POST"}
    tag_payload = posts[f"{FJ}/repos/{REPO}/tags"]
    assert tag_payload == {"tag_name": TAG, "target": COMMIT, "message": f"ARMOR v{VERSION}"}
    fj_rel = posts[f"{FJ}/repos/{REPO}/releases"]
    assert fj_rel["tag_name"] == TAG and fj_rel["target_commitish"] == COMMIT
    assert fj_rel["prerelease"] is False and fj_rel["draft"] is False
    gh_rel = posts[f"{GH}/repos/{REPO}/releases"]
    assert gh_rel["generate_release_notes"] is True
    assert gh_rel["tag_name"] == TAG and gh_rel["target_commitish"] == COMMIT
    body = gh_rel["body"]
    assert f"`ghcr.io/jedarden/armor:{VERSION}` | public | `{GHCR_DIGEST}`" in body
    assert f"`ghcr.io/jedarden/armor-restore-verifier:{VERSION}` | public | `{GHCR_RV_DIGEST}`" in body
    assert f"`ronaldraygun/armor:{VERSION}` | private | digest unavailable" in body
    assert "armor-fleet" in body and "ghcr.io/jedarden/armor-fleet" not in body
    assert "armor-build-abc12" in body
    assert f"go install github.com/jedarden/armor/cmd/armor@v{VERSION}" in body
    # Every Forgejo/GitHub call carried a credential; none was printed.
    assert all(auth for m, u, _, auth in api.calls if "/repos/" in u)


# ------------------------------------------------------------ idempotent


def test_rerun_updates_instead_of_duplicating(tokens, api, capsys):
    api.route("GET", f"{FJ}/repos/{REPO}/tags/{TAG}", 200, {"commit": {"sha": COMMIT}})
    api.route("GET", f"{FJ}/repos/{REPO}/releases/tags/{TAG}", 200, {"id": 7})
    api.route("PATCH", f"{FJ}/repos/{REPO}/releases/7", 200, {"id": 7, "html_url": "https://git/r"})
    api.route("GET", f"{GH}/repos/{REPO}/git/ref/tags/{TAG}", 200, {"ref": "x"})
    api.route("GET", f"{GH}/repos/{REPO}/releases/tags/{TAG}", 200, {"id": 9})
    api.route("PATCH", f"{GH}/repos/{REPO}/releases/9", 200, {"id": 9, "html_url": "https://github.com/r"})

    assert run() == pr.EXIT_OK
    out = outcome(capsys)
    assert (out["forgejo_tag"], out["forgejo_release"], out["github_release"]) == ("exists", "updated", "updated")
    assert api.mutations() == [
        ("PATCH", f"{FJ}/repos/{REPO}/releases/7"),
        ("PATCH", f"{GH}/repos/{REPO}/releases/9"),
    ]


def test_tag_at_other_commit_is_refused_before_any_mutation(tokens, api, capsys):
    api.route("GET", f"{FJ}/repos/{REPO}/tags/{TAG}", 200, {"commit": {"sha": OTHER}})
    with pytest.raises(SystemExit) as ex:
        run()
    assert ex.value.code == pr.EXIT_TAG_CONFLICT
    assert api.mutations() == []
    assert "refusing to move a tag" in capsys.readouterr().err


def test_tag_only_creates_tag_and_nothing_else(tokens, api, capsys):
    api.route("POST", f"{FJ}/repos/{REPO}/tags", 201, {})
    assert run("--tag-only") == pr.EXIT_OK
    out = outcome(capsys)
    assert out["forgejo_tag"] == "created"
    assert out["forgejo_release"] == "skipped" and out["github_release"] == "skipped"
    assert api.mutations() == [("POST", f"{FJ}/repos/{REPO}/tags")]


def test_dry_run_reports_plan_without_mutating(monkeypatch, api, capsys):
    monkeypatch.delenv("FORGEJO_TOKEN", raising=False)
    monkeypatch.delenv("GITHUB_TOKEN", raising=False)
    monkeypatch.delenv("DOCKER_CONFIG_JSON", raising=False)
    assert run("--dry-run") == pr.EXIT_OK
    out = outcome(capsys)
    assert out["forgejo_tag"] == "would-create"
    assert out["forgejo_release"] == "would-create"
    assert out["github_release"] == "would-create-or-update"
    assert api.mutations() == []
    assert any("POST" in m and "/tags" in m for m in out["mutations"])


# ---------------------------------------------------------------- github


def test_github_release_created_even_when_mirror_lags(tokens, api, capsys):
    api.route("POST", f"{FJ}/repos/{REPO}/tags", 201, {})
    api.route("POST", f"{FJ}/repos/{REPO}/releases", 201, {"id": 1, "html_url": "f"})
    # No route for the GitHub ref -> 404 forever; wait is 0 so we fall through.
    api.route("POST", f"{GH}/repos/{REPO}/releases", 201, {"id": 2, "html_url": "g"})
    slept: list[int] = []
    assert run(sleep=slept.append) == pr.EXIT_OK
    out = outcome(capsys)
    assert out["github_tag_visible"] is False
    assert out["github_release"] == "created"
    posted = [p for m, u, p, _ in api.calls if m == "POST" and u == f"{GH}/repos/{REPO}/releases"][0]
    assert posted["target_commitish"] == COMMIT


def test_github_422_exits_4_after_forgejo_side_succeeded(tokens, api, capsys):
    api.route("POST", f"{FJ}/repos/{REPO}/tags", 201, {})
    api.route("POST", f"{FJ}/repos/{REPO}/releases", 201, {"id": 1, "html_url": "f"})
    api.route("GET", f"{GH}/repos/{REPO}/git/ref/tags/{TAG}", 200, {})
    api.route("POST", f"{GH}/repos/{REPO}/releases", 422, {"message": "Validation Failed"})
    with pytest.raises(SystemExit) as ex:
        run()
    assert ex.value.code == pr.EXIT_GITHUB
    assert "mirror" in capsys.readouterr().err
    assert ("POST", f"{FJ}/repos/{REPO}/releases") in api.mutations()


def test_no_github_flag_skips_github_entirely(tokens, api, capsys):
    api.route("POST", f"{FJ}/repos/{REPO}/tags", 201, {})
    api.route("POST", f"{FJ}/repos/{REPO}/releases", 201, {"id": 1, "html_url": "f"})
    assert run("--no-github") == pr.EXIT_OK
    assert outcome(capsys)["github_release"] == "skipped"
    assert not any(GH in u for _, u, _, _ in api.calls)


# --------------------------------------------------------------- digests


def test_private_digests_resolved_from_docker_config(tokens, api, tmp_path, monkeypatch, capsys):
    cfg = tmp_path / "config.json"
    cfg.write_text(json.dumps({"auths": {"https://index.docker.io/v1/": {"auth": "dXNlcjpwYXNz"}}}))
    monkeypatch.setenv("DOCKER_CONFIG_JSON", str(cfg))
    for repo in pr.PRIVATE_IMAGES:
        api.route("GET", "https://auth.docker.io/token", 200, {"token": "hub-bearer"})
        api.route(
            "HEAD",
            f"https://registry-1.docker.io/v2/{repo}/manifests/{VERSION}",
            200,
            None,
            {"Docker-Content-Digest": HUB_DIGEST},
        )
    api.route("POST", f"{FJ}/repos/{REPO}/tags", 201, {})
    api.route("POST", f"{FJ}/repos/{REPO}/releases", 201, {"id": 1, "html_url": "f"})
    assert run("--no-github") == pr.EXIT_OK
    posted = [p for m, u, p, _ in api.calls if m == "POST" and u == f"{FJ}/repos/{REPO}/releases"][0]
    for repo in pr.PRIVATE_IMAGES:
        assert f"`{repo}:{VERSION}` | private | `{HUB_DIGEST}`" in posted["body"]
    token_calls = [c for c in api.calls if c[1] == "https://auth.docker.io/token"]
    assert token_calls and all(auth for _, _, _, auth in token_calls)


def test_release_body_never_mentions_a_floating_tag():
    body = pr.release_body(VERSION, COMMIT, None, [(f"ghcr.io/jedarden/armor:{VERSION}", "public", GHCR_DIGEST)])
    assert "Only semver tags are published" in body
    assert ":latest" not in body
