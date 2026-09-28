#!/usr/bin/env python3
"""
Tests for scripts/cut-release.sh — the release-cut mutation workflow,
exercised in isolated throwaway git repositories.

cut-release.sh is the sole writer of VERSION, compose.yaml's
${ARMOR_VERSION:-...} defaults and CHANGELOG.md's newest entry, all in one
`release: armor <version>` commit. The other release-path suites pin what
happens around that commit — test_compose_version_parity.py the drift gate,
test_publish_release.py the CI publisher — while these tests pin the cut
itself, against a real git fixture seeded with a previous v* tag: the
release commit carries exactly VERSION, CHANGELOG.md and compose.yaml even
over a dirty shared-style checkout, the defaults land in parity, the
changelog entry is generated from the commit subjects since the previous
tag (bead-checkpoint and release commits filtered, maintenance fallback
when nothing is left), malformed and non-newer versions are refused, the
repository state gates (staged index, off-main) refuse without a commit,
--dry-run touches nothing, and re-cutting the same version is refused
rather than duplicated. Every invocation uses --no-push or --dry-run; the
push is armor-build's half of the workflow, not the script's.

Run: python3 -m pytest tests/test_cut_release.py -q
"""

import os
import re
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "cut-release.sh"
PARITY_GATE = REPO_ROOT / "scripts" / "compose-version-parity.sh"

OLD = "0.1.9998"
NEW = "0.1.9999"

FEAT = "feat(server): add range fallback (armor-aaa1111)"
FIX = "fix(crypto): unwrap ring DEKs (armor-bbb2221)"
# Newest first, like `git log`: the fix is committed after the feat.
EXPECTED_NOTES = f"- {FIX}\n- {FEAT}\n"

PREAMBLE = """# Changelog

All notable changes, newest first.
"""


# The fixtures carry their own identity config, so git runs hermetically:
# the host's global config (credential helpers, the bead merge driver) can
# otherwise add hundreds of milliseconds to every operation.
GIT_ENV = {
    **os.environ,
    "GIT_CONFIG_GLOBAL": "/dev/null",
    "GIT_CONFIG_SYSTEM": "/dev/null",
}


def git(root, *args):
    return subprocess.run(
        ["git", "-C", str(root), *args],
        capture_output=True, text=True, check=True, env=GIT_ENV,
    )


def git_out(root, *args):
    return git(root, *args).stdout.strip()


def commit_file(root, name, message, content="x\n"):
    (root / name).write_text(content)
    git(root, "add", name)
    git(root, "commit", "-m", message, "--", name)


def compose_yaml(version=OLD):
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


def seed_changelog():
    return f"""\
{PREAMBLE}
## {OLD} (2026-09-01)

- feat(x): seed entry
"""


def make_repo(tmp_path, version=OLD, compose="default", changelog="default",
              tag=True, history="mixed"):
    """Build an isolated git repository the script can cut a release in.

    The script resolves the repository from its own location, so it (and the
    parity gate used to verify the result) are copied into the fixture — the
    same layout they see in the real checkout. The seed commit carries
    VERSION/compose.yaml/CHANGELOG.md/README.md and is tagged v<version>, so
    post-seed commits exercise the since-previous-tag range the way the real
    previous release tag does.
    """
    root = tmp_path / "repo"
    (root / "scripts").mkdir(parents=True)
    shutil.copy(SCRIPT, root / "scripts" / "cut-release.sh")
    shutil.copy(PARITY_GATE, root / "scripts" / "compose-version-parity.sh")
    (root / "VERSION").write_text(version + "\n")
    if compose == "default":
        compose = compose_yaml()
    if compose is not None:
        (root / "compose.yaml").write_text(compose)
    if changelog == "default":
        changelog = seed_changelog()
    if changelog is not None:
        (root / "CHANGELOG.md").write_text(changelog)
    (root / "README.md").write_text("fixture readme\n")
    git(root, "init")
    git(root, "symbolic-ref", "HEAD", "refs/heads/main")
    git(root, "config", "user.email", "fixture@example.com")
    git(root, "config", "user.name", "Fixture")
    git(root, "config", "commit.gpgsign", "false")
    git(root, "config", "tag.gpgsign", "false")
    tracked = [name for name in ("VERSION", "compose.yaml", "CHANGELOG.md", "README.md", "scripts")
               if (root / name).exists()]
    git(root, "add", *tracked)
    git(root, "commit", "-m", "chore: seed fixture")
    if tag:
        git(root, "tag", "-a", f"v{version}", "-m", f"v{version}")
    if history == "mixed":
        commit_file(root, "feat.txt", FEAT)
        commit_file(root, "fix.txt", FIX)
        # Filtered bookkeeping: a bead checkpoint and a stray release commit
        # after the tag must not reach the changelog.
        commit_file(root, "beads.txt", "chore(beads): publish checkpoint (armor-aaa1111)")
        commit_file(root, "rel.txt", "release: armor 0.1.9997")
    elif history == "bookkeeping":
        commit_file(root, "beads.txt", "chore(beads): publish checkpoint (armor-aaa1111)")
    return root


def cut(root, *args):
    return subprocess.run(
        ["bash", str(root / "scripts" / "cut-release.sh"), *args],
        cwd=root, capture_output=True, text=True, env=GIT_ENV,
    )


def run_parity_gate(root):
    return subprocess.run(
        ["sh", str(root / "scripts" / "compose-version-parity.sh")],
        cwd=root, capture_output=True, text=True,
    )


def entry_date(text, version=NEW):
    """The entry's UTC date, which must fall inside the test's own window."""
    match = re.search(rf"^## {re.escape(version)} \((\d{{4}}-\d{{2}}-\d{{2}})\)", text, re.M)
    assert match, f"no '## {version} (<date>)' heading in changelog:\n{text}"
    return match.group(1)


# ---------------------------------------------------------------------------
# the canonical cut: one commit, exactly the three release files
# ---------------------------------------------------------------------------

def test_cut_commits_exactly_the_three_release_files(tmp_path):
    """The release commit carries only VERSION, CHANGELOG.md and
    compose.yaml — a shared checkout's unstaged edits and untracked files
    stay out of it and survive in the worktree."""
    root = make_repo(tmp_path)
    (root / "README.md").write_text("fixture readme\nin-flight edit by another agent\n")
    (root / "stray.txt").write_text("someone's scratch file\n")
    head_before = git_out(root, "rev-parse", "HEAD")

    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert "release: armor 0.1.9999" in proc.stdout
    assert "not pushed (--no-push)" in proc.stdout

    committed = git_out(root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
    assert committed.splitlines() == ["CHANGELOG.md", "VERSION", "compose.yaml"]
    assert git_out(root, "log", "-1", "--format=%s") == f"release: armor {NEW}"
    assert git_out(root, "rev-parse", "HEAD^") == head_before
    assert (root / "VERSION").read_text() == NEW + "\n"

    # The dirty shared-checkout state neither rides along nor is clobbered.
    assert git_out(root, "diff", "--name-only") == "README.md"
    assert "stray.txt" not in git_out(root, "ls-files").splitlines()
    assert "in-flight edit" in (root / "README.md").read_text()
    assert (root / "stray.txt").exists()
    # Raw porcelain (not git_out, which strips the leading status column).
    assert git(root, "status", "--porcelain").stdout.splitlines() == [
        " M README.md", "?? stray.txt",
    ]


def test_cut_rewrites_compose_defaults_into_parity(tmp_path):
    """Cutting from defaults pinned at the old version leaves the tracked
    composition passing the parity gate that guards every other change."""
    root = make_repo(tmp_path, compose=compose_yaml(OLD))
    assert "0.1.9998" in (root / "compose.yaml").read_text()

    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 0, proc.stdout + proc.stderr

    gate = run_parity_gate(root)
    assert gate.returncode == 0, gate.stdout + gate.stderr
    assert NEW in gate.stdout
    compose = (root / "compose.yaml").read_text()
    assert compose.count(f"ARMOR_VERSION:-{NEW}") == 2
    assert f"ARMOR_VERSION:-{OLD}" not in compose


def test_cut_prepends_changelog_entry_from_commits_since_the_tag(tmp_path):
    """The entry is the non-bookkeeping commit subjects since the previous
    v* tag, newest first, inserted before the first existing heading."""
    root = make_repo(tmp_path)
    before = datetime.now(timezone.utc).date()

    proc = cut(root, NEW, "--no-push")
    after = datetime.now(timezone.utc).date()
    assert proc.returncode == 0, proc.stdout + proc.stderr

    changelog = (root / "CHANGELOG.md").read_text()
    date = entry_date(changelog)
    assert date in {before.isoformat(), after.isoformat()}
    assert changelog == (
        f"{PREAMBLE}\n"
        f"## {NEW} ({date})\n"
        f"\n"
        f"{EXPECTED_NOTES}"
        f"\n"
        f"## {OLD} (2026-09-01)\n"
        f"\n"
        f"- feat(x): seed entry\n"
    )
    # The seed commit predates the tag; bookkeeping never reaches the entry.
    assert "seed fixture" not in changelog
    assert "chore(beads)" not in changelog
    assert "release: armor 0.1.9997" not in changelog


def test_cut_with_only_bookkeeping_since_the_tag_writes_maintenance_entry(tmp_path):
    root = make_repo(tmp_path, history="bookkeeping")
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 0, proc.stdout + proc.stderr
    changelog = (root / "CHANGELOG.md").read_text()
    assert (
        f"- Maintenance release (no user-visible changes recorded since v{OLD})"
        in changelog
    )
    assert "chore(beads)" not in changelog


def test_cut_creates_a_missing_changelog(tmp_path):
    root = make_repo(tmp_path, changelog=None)
    assert not (root / "CHANGELOG.md").exists()
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 0, proc.stdout + proc.stderr
    changelog = (root / "CHANGELOG.md").read_text()
    assert changelog.startswith("# Changelog\n\n")
    assert changelog.count(f"## {NEW} ") == 1
    assert EXPECTED_NOTES in changelog


def test_cut_without_any_prior_tag_uses_the_whole_history(tmp_path):
    """No v* tag reachable from HEAD means the range is HEAD: every commit
    since the first one, the seed commit included."""
    root = make_repo(tmp_path, tag=False)
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 0, proc.stdout + proc.stderr
    changelog = (root / "CHANGELOG.md").read_text()
    assert EXPECTED_NOTES in changelog
    assert "- chore: seed fixture" in changelog


# ---------------------------------------------------------------------------
# idempotence: dry runs change nothing, a cut cannot happen twice
# ---------------------------------------------------------------------------

def test_dry_run_changes_nothing(tmp_path):
    root = make_repo(tmp_path)
    head_before = git_out(root, "rev-parse", "HEAD")
    files = {name: (root / name).read_text()
             for name in ("VERSION", "compose.yaml", "CHANGELOG.md")}

    proc = cut(root, NEW, "--dry-run")
    assert proc.returncode == 0, proc.stdout + proc.stderr
    assert "dry run" in proc.stdout
    assert f"## {NEW} (" in proc.stdout
    assert EXPECTED_NOTES in proc.stdout

    assert git_out(root, "rev-parse", "HEAD") == head_before
    assert git_out(root, "status", "--porcelain") == ""
    assert {name: (root / name).read_text() for name in files} == files


def test_recutting_the_same_version_is_refused_and_changes_nothing(tmp_path):
    root = make_repo(tmp_path)
    assert cut(root, NEW, "--no-push").returncode == 0
    head = git_out(root, "rev-parse", "HEAD")
    version = (root / "VERSION").read_text()
    changelog = (root / "CHANGELOG.md").read_text()

    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 2
    assert "not newer than the current VERSION" in proc.stderr

    assert git_out(root, "rev-parse", "HEAD") == head
    assert (root / "VERSION").read_text() == version
    assert (root / "CHANGELOG.md").read_text() == changelog
    assert changelog.count(f"## {NEW} (") == 1


# ---------------------------------------------------------------------------
# invalid input: malformed and non-newer versions
# ---------------------------------------------------------------------------

# Not pinned as malformed: a four-component "0.1.9998.5" matches the
# script's digit-glob version pattern (each `*` spans dots) and sorts newer,
# so the script accepts it; only genuinely wrong shapes are refused.
@pytest.mark.parametrize("bad", ["not-a-version", "0.1", "v0.1.9999"])
def test_malformed_versions_are_refused(tmp_path, bad):
    root = make_repo(tmp_path)
    head = git_out(root, "rev-parse", "HEAD")
    proc = cut(root, bad, "--no-push")
    assert proc.returncode == 2
    assert "is not MAJOR.MINOR.PATCH" in proc.stderr
    assert git_out(root, "rev-parse", "HEAD") == head
    assert git_out(root, "status", "--porcelain") == ""


@pytest.mark.parametrize("stale", [OLD, "0.1.9997", "0.0.1"])
def test_non_newer_versions_are_refused(tmp_path, stale):
    root = make_repo(tmp_path)
    head = git_out(root, "rev-parse", "HEAD")
    proc = cut(root, stale, "--no-push")
    assert proc.returncode == 2
    assert "not newer than the current VERSION" in proc.stderr
    assert git_out(root, "rev-parse", "HEAD") == head


def test_missing_version_argument_prints_usage(tmp_path):
    root = make_repo(tmp_path)
    proc = cut(root)
    assert proc.returncode == 2
    assert "Usage" in proc.stdout + proc.stderr


def test_unknown_flag_is_refused(tmp_path):
    root = make_repo(tmp_path)
    proc = cut(root, NEW, "--bogus")
    assert proc.returncode == 2
    assert "unknown argument: --bogus" in proc.stderr


# ---------------------------------------------------------------------------
# repository state gates: refuse without committing
# ---------------------------------------------------------------------------

def test_staged_changes_are_refused(tmp_path):
    root = make_repo(tmp_path)
    (root / "README.md").write_text("staged edit\n")
    git(root, "add", "README.md")
    head = git_out(root, "rev-parse", "HEAD")
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 2
    assert "staged changes" in proc.stderr
    assert git_out(root, "rev-parse", "HEAD") == head


def test_off_main_is_refused(tmp_path):
    root = make_repo(tmp_path)
    git(root, "checkout", "-b", "feature-x")
    head = git_out(root, "rev-parse", "HEAD")
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 2
    assert "releases are cut from main" in proc.stderr
    assert "feature-x" in proc.stderr
    assert git_out(root, "rev-parse", "HEAD") == head


def test_missing_compose_is_refused(tmp_path):
    root = make_repo(tmp_path, compose=None)
    head = git_out(root, "rev-parse", "HEAD")
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 2
    assert "compose.yaml not found" in proc.stderr
    assert git_out(root, "rev-parse", "HEAD") == head
    assert (root / "VERSION").read_text() == OLD + "\n"


def test_compose_without_a_default_is_refused(tmp_path):
    """A compose.yaml with no ${ARMOR_VERSION:-...} default is a structural
    break — nothing could ride in the release commit."""
    root = make_repo(
        tmp_path,
        compose=compose_yaml().replace(f"${{ARMOR_VERSION:-{OLD}}}", OLD),
    )
    assert "ARMOR_VERSION" not in (root / "compose.yaml").read_text()
    head = git_out(root, "rev-parse", "HEAD")
    proc = cut(root, NEW, "--no-push")
    assert proc.returncode == 2
    assert "carries no ${ARMOR_VERSION:-...} default" in proc.stderr
    assert "nothing to pin to" in proc.stderr
    assert git_out(root, "rev-parse", "HEAD") == head
