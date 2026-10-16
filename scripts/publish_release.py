#!/usr/bin/env python3
"""Publish an ARMOR release: annotated tag + Forgejo release + GitHub release.

Run by the armor-build workflow's ``publish-release`` step after the
image-existence gate, and by hand for backfills. Every operation is
idempotent so a re-run on the same (version, commit) repairs rather than
duplicates:

* Forgejo tag ``v<version>``: created (annotated, via the tags API) when
  absent; left alone when it already points at ``commit``; a tag that points
  somewhere else is a hard error (exit 3) -- never moved.
* Forgejo release for the tag: created when absent, otherwise its body is
  refreshed.
* GitHub release for the tag: the Forgejo push mirror carries the tag to
  GitHub; this script waits for it (``--github-wait-seconds``) and then
  creates or refreshes the release. If the tag never shows up, the release is
  still created with ``target_commitish`` so GitHub makes the tag itself.

The release body is the ``CHANGELOG.md`` entry for the version (normally
fetched from the released revision through the Forgejo raw API) followed by
the image-digest table. Batch backfill uses committed HEAD for corrected
historical notes while retaining each original release commit as the tag
target. A missing file or section degrades to the digest table alone; any
other changelog fetch failure exits 5 so the workflow's public-host retry
runs.

``--backfill-range FIRST..LAST`` is an operator repair mode run in a local
checkout. It finds VERSION-changing commits, checks the private server image
through authenticated Docker Hub access, and publishes only image-backed
versions. Authentication or registry errors are fatal rather than skips.

Credentials come ONLY from the environment and are never printed or passed
as arguments:

* ``FORGEJO_TOKEN``   -- Forgejo API token with write access to the repo
* ``GITHUB_TOKEN``    -- GitHub token with ``repo`` (or contents:write)
* ``DOCKER_CONFIG_JSON`` -- optional path to a Docker ``config.json`` whose
  inline ``auths`` entry for Docker Hub lets the script resolve the private
  ``ronaldraygun/*`` image digests for the release body. Without it those
  rows read "digest unavailable". The public GHCR digest needs no credential.

Exit codes: 0 ok; 2 usage/validation; 3 tag conflict; 4 GitHub publish
failed; 5 Forgejo API failed; 6 registry state unknown or no image in range.
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

DEFAULT_REPO = "jedarden/ARMOR"
DEFAULT_FORGEJO_API = "https://git.ardenone.com/api/v1"
DEFAULT_GITHUB_API = "https://api.github.com"
PUBLIC_IMAGE = "ghcr.io/jedarden/armor"
# The server image is the ONLY public mirror (operator decision 2026-09-19:
# restore-verifier and armor-fleet stay private). The digest lookup is
# deliberately anonymous: a row that reads "digest unavailable" means the
# package is missing or not public, which is what an outside consumer hits.
PUBLIC_IMAGES = (PUBLIC_IMAGE,)
PRIVATE_IMAGES = (
    "ronaldraygun/armor",
    "ronaldraygun/armor-restore-verifier",
    "ronaldraygun/armor-fleet",
)
MANIFEST_ACCEPT = ", ".join(
    [
        "application/vnd.oci.image.index.v1+json",
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
        "application/vnd.docker.distribution.manifest.v2+json",
    ]
)

VERSION_RE = re.compile(r"^\d+\.\d+\.\d+$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")

EXIT_OK = 0
EXIT_USAGE = 2
EXIT_TAG_CONFLICT = 3
EXIT_GITHUB = 4
EXIT_FORGEJO = 5
EXIT_REGISTRY = 6


class ApiError(Exception):
    def __init__(self, status: int, url: str, body: str):
        super().__init__(f"HTTP {status} from {url}: {body[:300]}")
        self.status = status
        self.url = url
        self.body = body


@dataclass
class Response:
    status: int
    headers: dict[str, str]
    body: Any


def http(
    method: str,
    url: str,
    *,
    token: str | None = None,
    token_scheme: str = "token",
    payload: dict | None = None,
    headers: dict[str, str] | None = None,
    timeout: int = 30,
) -> Response:
    """One HTTP call. Raises ApiError on any 4xx/5xx except where callers
    ask for the status by catching it."""
    hdrs = {"Accept": "application/json", "User-Agent": "armor-publish-release"}
    if headers:
        hdrs.update(headers)
    data = None
    if payload is not None:
        data = json.dumps(payload).encode()
        hdrs["Content-Type"] = "application/json"
    if token:
        hdrs["Authorization"] = f"{token_scheme} {token}"
    req = Request(url, data=data, method=method, headers=hdrs)
    try:
        with urlopen(req, timeout=timeout) as resp:  # noqa: S310 - fixed https hosts
            raw = resp.read()
            status = resp.status
            rh = {k.lower(): v for k, v in resp.headers.items()}
    except HTTPError as e:
        raw = e.read() if hasattr(e, "read") else b""
        raise ApiError(e.code, url, raw.decode(errors="replace")) from None
    except URLError as e:
        raise ApiError(0, url, str(e.reason)) from None
    body: Any = None
    if raw:
        try:
            body = json.loads(raw)
        except ValueError:
            body = raw.decode(errors="replace")
    return Response(status, rh, body)


def status_of(exc_or_resp: ApiError | Response) -> int:
    return exc_or_resp.status


# ---------------------------------------------------------------- digests


def registry_digest(
    registry: str, repo: str, tag: str, token_url: str, basic: str | None
) -> str | None:
    """Docker-Content-Digest for registry/repo:tag, or None if unresolvable.

    ``basic`` is an already base64-encoded ``user:password`` for the token
    endpoint (Docker Hub private repos); None means anonymous (GHCR public).
    """
    try:
        hdr = {"Authorization": f"Basic {basic}"} if basic else None
        tok = http("GET", token_url, headers=hdr).body
        bearer = (tok or {}).get("token") if isinstance(tok, dict) else None
        if not bearer:
            return None
        resp = http(
            "HEAD",
            f"https://{registry}/v2/{repo}/manifests/{tag}",
            token=bearer,
            token_scheme="Bearer",
            headers={"Accept": MANIFEST_ACCEPT},
        )
        return resp.headers.get("docker-content-digest")
    except ApiError:
        return None


def dockerhub_basic_from_config(path: str | None) -> str | None:
    """Return the base64 ``user:password`` for Docker Hub from a docker
    config.json, or None. Never logs it."""
    if not path or not os.path.exists(path):
        return None
    try:
        with open(path, encoding="utf-8") as fh:
            cfg = json.load(fh)
    except (OSError, ValueError):
        return None
    auths = cfg.get("auths") or {}
    for key in ("https://index.docker.io/v1/", "index.docker.io", "docker.io"):
        entry = auths.get(key) or {}
        if entry.get("auth"):
            return entry["auth"]
    return None


def collect_digests(version: str, docker_config: str | None) -> list[tuple[str, str, str | None]]:
    """[(image_ref, visibility, digest_or_None), ...] for the release body."""
    rows: list[tuple[str, str, str | None]] = []
    for image in PUBLIC_IMAGES:
        ghcr_repo = image.split("/", 1)[1]
        rows.append(
            (
                f"{image}:{version}",
                "public",
                registry_digest(
                    "ghcr.io",
                    ghcr_repo,
                    version,
                    f"https://ghcr.io/token?scope=repository:{ghcr_repo}:pull",
                    None,
                ),
            )
        )
    basic = dockerhub_basic_from_config(docker_config)
    for repo in PRIVATE_IMAGES:
        digest = None
        if basic:
            digest = registry_digest(
                "registry-1.docker.io",
                repo,
                version,
                f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{repo}:pull",
                basic,
            )
        rows.append((f"{repo}:{version}", "private", digest))
    return rows


def server_image_exists(version: str, docker_config: str | None) -> bool:
    """Check the private server image before backfilling a release.

    A missing tag is skippable. Missing credentials and registry failures are
    fatal: treating either as a missing image would silently lose releases.
    """
    basic = dockerhub_basic_from_config(docker_config)
    if not basic:
        raise ValueError("DOCKER_CONFIG_JSON must contain inline Docker Hub auth for backfill")
    repo = "ronaldraygun/armor"
    token_url = f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{repo}:pull"
    token = http("GET", token_url, headers={"Authorization": f"Basic {basic}"}).body
    bearer = (token or {}).get("token") if isinstance(token, dict) else None
    if not bearer:
        raise ValueError("Docker Hub returned no pull token for the server image")
    try:
        http(
            "HEAD",
            f"https://registry-1.docker.io/v2/{repo}/manifests/{version}",
            token=bearer,
            token_scheme="Bearer",
            headers={"Accept": MANIFEST_ACCEPT},
        )
    except ApiError as e:
        if e.status == 404:
            return False
        raise
    return True


def backfill_candidates(first: str, last: str) -> list[tuple[str, str]]:
    """Map versions in an inclusive range to their VERSION-changing commit.

    This mode runs in a local checkout, unlike the single-version publisher
    fetched into CI. The newest change to a version wins; old release commits
    cannot accidentally retag a newer VERSION revision.
    """
    def git(*args: str) -> str:
        result = subprocess.run(["git", *args], capture_output=True, text=True, check=True)
        return result.stdout.strip()

    low = tuple(map(int, first.split(".")))
    high = tuple(map(int, last.split(".")))
    found: dict[str, str] = {}
    for commit in git("log", "--format=%H", "HEAD", "--", "VERSION").splitlines():
        version = git("show", f"{commit}:VERSION")
        if not VERSION_RE.fullmatch(version):
            continue
        number = tuple(map(int, version.split(".")))
        if low <= number <= high:
            found.setdefault(version, commit)
        if number < low:
            break
    return sorted(found.items(), key=lambda item: tuple(map(int, item[0].split("."))))


# ------------------------------------------------------------------ body


def release_name(version: str) -> str:
    return f"ARMOR v{version}"


def extract_changelog_section(markdown: str, version: str) -> str | None:
    """Body of the ``## <version> (<date>)`` section of CHANGELOG.md, without
    the heading itself, or None when the file has no entry for ``version``.

    Same section boundaries as the awk recipe in docs/release-process.md:
    start after the ``## <version> (`` heading, stop at the next ``## ``
    heading. The prefix match is anchored by the opening parenthesis, so
    ``0.1.19`` never matches the ``0.1.1971`` entry.
    """
    wanted = f"## {version} ("
    lines = markdown.splitlines()
    start = next((i for i, line in enumerate(lines) if line.startswith(wanted)), None)
    if start is None:
        return None
    section: list[str] = []
    for line in lines[start + 1 :]:
        if line.startswith("## "):
            break
        section.append(line)
    return "\n".join(section).strip() or None


def fetch_changelog_entry(
    api: str, repo: str, version: str, commit: str, token: str, out: Outcome
) -> str | None:
    """CHANGELOG.md section for ``version`` as written in the release commit.

    A missing file or a missing section degrades to a digest-table-only body
    (logged, and recorded in the outcome); any other API failure is fatal
    (exit 5) so the workflow retries via the public Forgejo host.
    """
    url = f"{api}/repos/{repo}/raw/CHANGELOG.md?ref={quote(commit)}"
    try:
        resp = http("GET", url, token=token)
    except ApiError as e:
        if e.status == 404:
            _log(f"changelog: no CHANGELOG.md at {commit[:12]}; body carries only the digest table")
            return None
        raise SystemExit(_fail(EXIT_FORGEJO, f"CHANGELOG fetch failed: {e}"))
    markdown = resp.body if isinstance(resp.body, str) else ""
    entry = extract_changelog_section(markdown, version)
    if entry is None:
        _log(f"changelog: no '## {version} (' section at {commit[:12]}; body carries only the digest table")
    return entry


def release_body(
    version: str,
    commit: str,
    workflow: str | None,
    digests: list[tuple[str, str, str | None]],
    changelog: str | None = None,
) -> str:
    origin = (
        f"iad-ci Argo workflow `{workflow}`" if workflow else "a manual `scripts/publish_release.py` run"
    )
    lines = [f"## ARMOR v{version}", ""]
    if changelog:
        lines += [changelog, ""]
    lines += [
        f"Built from commit `{commit}` by {origin}.",
        "",
        "### Images",
        "",
        "| Image | Visibility | Digest |",
        "|---|---|---|",
    ]
    for ref, vis, digest in digests:
        lines.append(f"| `{ref}` | {vis} | `{digest}` |" if digest else f"| `{ref}` | {vis} | digest unavailable at publish time |")
    lines += [
        "",
        "Only semver tags are published; there is no floating tag.",
        "",
        "```bash",
        f"docker pull {PUBLIC_IMAGE}:{version}",
        f"go install github.com/jedarden/armor/cmd/armor@v{version}",
        "```",
        "",
        "The GHCR image is the public server image and the only public artifact. The `ronaldraygun/*` images (server, restore-verifier, fleet console) are private to the ardenone fleet.",
    ]
    return "\n".join(lines) + "\n"


# ---------------------------------------------------------------- forgejo


@dataclass
class Outcome:
    version: str
    tag: str
    commit: str
    forgejo_tag: str = "skipped"
    forgejo_release: str = "skipped"
    changelog: str = "skipped"
    github_tag_visible: bool | None = None
    github_release: str = "skipped"
    urls: dict[str, str] = field(default_factory=dict)
    mutations: list[str] = field(default_factory=list)

    def as_json(self) -> str:
        return json.dumps(self.__dict__, indent=2, sort_keys=True)


def forgejo_ensure_tag(api: str, repo: str, tag: str, commit: str, token: str, out: Outcome, dry: bool) -> None:
    url = f"{api}/repos/{repo}/tags/{quote(tag)}"
    try:
        resp = http("GET", url, token=token)
        existing = ((resp.body or {}).get("commit") or {}).get("sha", "")
        if existing != commit:
            raise SystemExit(
                _fail(
                    EXIT_TAG_CONFLICT,
                    f"tag {tag} already exists on Forgejo at {existing}, not {commit}; refusing to move a tag",
                )
            )
        out.forgejo_tag = "exists"
        return
    except ApiError as e:
        if e.status != 404:
            raise SystemExit(_fail(EXIT_FORGEJO, f"Forgejo tag lookup failed: {e}"))
    out.mutations.append(f"POST {api}/repos/{repo}/tags {tag} -> {commit}")
    if dry:
        out.forgejo_tag = "would-create"
        return
    try:
        http(
            "POST",
            f"{api}/repos/{repo}/tags",
            token=token,
            payload={"tag_name": tag, "target": commit, "message": release_name(tag[1:])},
        )
    except ApiError as e:
        raise SystemExit(_fail(EXIT_FORGEJO, f"Forgejo tag create failed: {e}"))
    out.forgejo_tag = "created"


def forgejo_ensure_release(
    api: str, repo: str, tag: str, commit: str, name: str, body: str, token: str, out: Outcome, dry: bool
) -> None:
    url = f"{api}/repos/{repo}/releases/tags/{quote(tag)}"
    existing = None
    try:
        existing = http("GET", url, token=token).body
    except ApiError as e:
        if e.status != 404:
            raise SystemExit(_fail(EXIT_FORGEJO, f"Forgejo release lookup failed: {e}"))
    if existing:
        rid = existing.get("id")
        out.mutations.append(f"PATCH {api}/repos/{repo}/releases/{rid}")
        if dry:
            out.forgejo_release = "would-update"
        else:
            try:
                r = http(
                    "PATCH",
                    f"{api}/repos/{repo}/releases/{rid}",
                    token=token,
                    payload={"name": name, "body": body, "draft": False, "prerelease": False},
                )
            except ApiError as e:
                raise SystemExit(_fail(EXIT_FORGEJO, f"Forgejo release update failed: {e}"))
            out.forgejo_release = "updated"
            out.urls["forgejo"] = (r.body or {}).get("html_url", "")
        return
    out.mutations.append(f"POST {api}/repos/{repo}/releases {tag}")
    if dry:
        out.forgejo_release = "would-create"
        return
    try:
        r = http(
            "POST",
            f"{api}/repos/{repo}/releases",
            token=token,
            payload={
                "tag_name": tag,
                "target_commitish": commit,
                "name": name,
                "body": body,
                "draft": False,
                "prerelease": False,
            },
        )
    except ApiError as e:
        raise SystemExit(_fail(EXIT_FORGEJO, f"Forgejo release create failed: {e}"))
    out.forgejo_release = "created"
    out.urls["forgejo"] = (r.body or {}).get("html_url", "")


# ----------------------------------------------------------------- github


def github_wait_for_tag(api: str, repo: str, tag: str, token: str, wait: int, interval: int, sleep=time.sleep) -> bool:
    url = f"{api}/repos/{repo}/git/ref/tags/{quote(tag)}"
    deadline = time.monotonic() + wait
    while True:
        try:
            http("GET", url, token=token, token_scheme="Bearer")
            return True
        except ApiError as e:
            if e.status not in (404, 0):
                _log(f"GitHub ref lookup returned {e.status}; continuing to wait")
        if time.monotonic() >= deadline:
            return False
        sleep(interval)


def github_ensure_release(
    api: str, repo: str, tag: str, commit: str, name: str, body: str, token: str, out: Outcome, dry: bool
) -> None:
    url = f"{api}/repos/{repo}/releases/tags/{quote(tag)}"
    existing = None
    try:
        existing = http("GET", url, token=token, token_scheme="Bearer").body
    except ApiError as e:
        if e.status != 404:
            raise SystemExit(_fail(EXIT_GITHUB, f"GitHub release lookup failed: {e}"))
    if existing:
        rid = existing.get("id")
        out.mutations.append(f"PATCH {api}/repos/{repo}/releases/{rid}")
        if dry:
            out.github_release = "would-update"
        else:
            try:
                r = http(
                    "PATCH",
                    f"{api}/repos/{repo}/releases/{rid}",
                    token=token,
                    token_scheme="Bearer",
                    payload={"name": name, "body": body, "draft": False, "prerelease": False},
                )
            except ApiError as e:
                raise SystemExit(_fail(EXIT_GITHUB, f"GitHub release update failed: {e}"))
            out.github_release = "updated"
            out.urls["github"] = (r.body or {}).get("html_url", "")
        return
    out.mutations.append(f"POST {api}/repos/{repo}/releases {tag}")
    if dry:
        out.github_release = "would-create"
        return
    try:
        r = http(
            "POST",
            f"{api}/repos/{repo}/releases",
            token=token,
            token_scheme="Bearer",
            payload={
                "tag_name": tag,
                "target_commitish": commit,
                "name": name,
                "body": body,
                "draft": False,
                "prerelease": False,
                "generate_release_notes": True,
            },
        )
    except ApiError as e:
        hint = ""
        if e.status == 422:
            hint = " (422 usually means the commit has not reached the GitHub mirror yet; re-run once it has)"
        raise SystemExit(_fail(EXIT_GITHUB, f"GitHub release create failed: {e}{hint}"))
    out.github_release = "created"
    out.urls["github"] = (r.body or {}).get("html_url", "")


# ------------------------------------------------------------------- main


def _log(msg: str) -> None:
    print(msg, file=sys.stderr)


def _fail(code: int, msg: str) -> int:
    _log(f"error: {msg}")
    return code


def parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    mode = p.add_mutually_exclusive_group(required=True)
    mode.add_argument("--version", help="release version, e.g. 0.1.1969 (no leading v)")
    mode.add_argument("--backfill-range", metavar="FIRST..LAST", help="publish every version in this inclusive range with a Docker Hub server image")
    p.add_argument("--commit", help="full 40-hex commit SHA for --version")
    p.add_argument("--changelog-commit", help="revision containing corrected historical notes; defaults to --commit")
    p.add_argument("--repo", default=DEFAULT_REPO, help="owner/name on both Forgejo and GitHub")
    p.add_argument("--forgejo-api", default=DEFAULT_FORGEJO_API)
    p.add_argument("--github-api", default=DEFAULT_GITHUB_API)
    p.add_argument("--workflow", default=None, help="Argo workflow name to credit in the body")
    p.add_argument("--tag-only", action="store_true", help="create the Forgejo tag, publish no releases")
    p.add_argument("--no-github", action="store_true", help="skip the GitHub release")
    p.add_argument("--github-wait-seconds", type=int, default=300)
    p.add_argument("--github-poll-interval", type=int, default=10)
    p.add_argument("--dry-run", action="store_true", help="look up state, print planned mutations, change nothing")
    return p.parse_args(argv)


def main(argv: list[str] | None = None, *, sleep=time.sleep) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    if "/" not in args.repo:
        return _fail(EXIT_USAGE, "--repo must be owner/name")

    if args.backfill_range:
        if args.tag_only:
            return _fail(EXIT_USAGE, "--tag-only cannot be used with --backfill-range")
        if args.commit or args.changelog_commit:
            return _fail(EXIT_USAGE, "--commit and --changelog-commit cannot be used with --backfill-range")
        parts = args.backfill_range.split("..")
        if len(parts) != 2 or any(not VERSION_RE.fullmatch(part) for part in parts):
            return _fail(EXIT_USAGE, "--backfill-range must be FIRST..LAST with two MAJOR.MINOR.PATCH versions")
        if tuple(map(int, parts[0].split("."))) > tuple(map(int, parts[1].split("."))):
            return _fail(EXIT_USAGE, "--backfill-range FIRST must not exceed LAST")
        if not args.dry_run and (not os.environ.get("FORGEJO_TOKEN") or
                                 (not args.no_github and not os.environ.get("GITHUB_TOKEN"))):
            return _fail(EXIT_USAGE, "FORGEJO_TOKEN and GITHUB_TOKEN are required for backfill")
        try:
            candidates = backfill_candidates(*parts)
        except (OSError, subprocess.CalledProcessError) as e:
            return _fail(EXIT_USAGE, f"cannot read VERSION history for backfill: {e}")
        if not candidates:
            return _fail(EXIT_USAGE, "no VERSION-changing commits found in backfill range")
        try:
            changelog_commit = subprocess.run(
                ["git", "rev-parse", "HEAD"], capture_output=True, text=True, check=True
            ).stdout.strip()
        except (OSError, subprocess.CalledProcessError) as e:
            return _fail(EXIT_USAGE, f"cannot resolve backfill changelog revision: {e}")
        published: list[str] = []
        skipped: list[str] = []
        for version, commit in candidates:
            try:
                exists = server_image_exists(version, os.environ.get("DOCKER_CONFIG_JSON"))
            except (ApiError, ValueError) as e:
                return _fail(EXIT_REGISTRY, f"cannot establish server image state for {version}: {e}")
            if not exists:
                _log(f"backfill: {version} has no authenticated Docker Hub server image; skipping")
                skipped.append(version)
                continue
            one = ["--version", version, "--commit", commit,
                   "--changelog-commit", changelog_commit, "--repo", args.repo,
                   "--forgejo-api", args.forgejo_api, "--github-api", args.github_api,
                   "--github-wait-seconds", str(args.github_wait_seconds),
                   "--github-poll-interval", str(args.github_poll_interval)]
            if args.workflow:
                one += ["--workflow", args.workflow]
            if args.no_github:
                one.append("--no-github")
            if args.dry_run:
                one.append("--dry-run")
            result = main(one, sleep=sleep)
            if result != EXIT_OK:
                return result
            published.append(version)
        print(json.dumps({"backfill_published": published, "backfill_skipped_no_image": skipped}, sort_keys=True))
        return EXIT_OK if published else _fail(EXIT_REGISTRY, "no versions in the range have a server image")

    if not VERSION_RE.fullmatch(args.version):
        return _fail(EXIT_USAGE, f"--version must be MAJOR.MINOR.PATCH, got {args.version!r}")
    if not args.commit or not COMMIT_RE.fullmatch(args.commit):
        return _fail(EXIT_USAGE, "--commit must be a full 40-character lowercase hex SHA")
    if args.changelog_commit and not COMMIT_RE.fullmatch(args.changelog_commit):
        return _fail(EXIT_USAGE, "--changelog-commit must be a full 40-character lowercase hex SHA")

    forgejo_token = os.environ.get("FORGEJO_TOKEN", "")
    github_token = os.environ.get("GITHUB_TOKEN", "")
    if not args.dry_run and not forgejo_token:
        return _fail(EXIT_USAGE, "FORGEJO_TOKEN is not set")
    if not args.dry_run and not args.tag_only and not args.no_github and not github_token:
        return _fail(EXIT_USAGE, "GITHUB_TOKEN is not set (pass --no-github to skip)")

    tag = f"v{args.version}"
    out = Outcome(version=args.version, tag=tag, commit=args.commit)

    forgejo_ensure_tag(args.forgejo_api, args.repo, tag, args.commit, forgejo_token, out, args.dry_run)

    if args.tag_only:
        print(out.as_json())
        return EXIT_OK

    digests = collect_digests(args.version, os.environ.get("DOCKER_CONFIG_JSON"))
    name = release_name(args.version)
    changelog = fetch_changelog_entry(
        args.forgejo_api, args.repo, args.version,
        args.changelog_commit or args.commit, forgejo_token, out
    )
    out.changelog = "included" if changelog else "unavailable"
    body = release_body(args.version, args.commit, args.workflow, digests, changelog)

    forgejo_ensure_release(args.forgejo_api, args.repo, tag, args.commit, name, body, forgejo_token, out, args.dry_run)

    if not args.no_github:
        if args.dry_run and not github_token:
            out.github_tag_visible = None
            out.github_release = "would-create-or-update"
            out.mutations.append(f"POST/PATCH {args.github_api}/repos/{args.repo}/releases {tag}")
        else:
            out.github_tag_visible = github_wait_for_tag(
                args.github_api,
                args.repo,
                tag,
                github_token,
                args.github_wait_seconds,
                args.github_poll_interval,
                sleep=sleep,
            )
            if not out.github_tag_visible:
                _log(f"tag {tag} not visible on GitHub after {args.github_wait_seconds}s; creating the release with target_commitish")
            github_ensure_release(args.github_api, args.repo, tag, args.commit, name, body, github_token, out, args.dry_run)

    print(out.as_json())
    return EXIT_OK


if __name__ == "__main__":
    sys.exit(main())
