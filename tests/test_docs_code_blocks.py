#!/usr/bin/env python3
"""
Regression test for scrubber-corrupted copy-paste commands in docs/.

A prior sanitize pass replaced quoted shell fragments (secret-reference
substitutions like `$(bao kv get ...)` or `$ARMOR_ADMIN_TOKEN`) with the
literal placeholder `REMOVED-NOT-A-SECRET-VALUE`, breaking the resulting
command's syntax rather than leaving a usable secret-by-reference example
(armor-3a98d0b0). The placeholder is harmless prose outside a fenced code
block, but inside one it means the exact command an operator would copy
during an incident does not run. This test fails on any occurrence of the
placeholder inside a fenced code block under docs/, excluding docs/archive/
(superseded material that is not meant to be copy-pasted).

Run: python3 -m pytest tests/test_docs_code_blocks.py -q
"""

import re
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
DOCS_ROOT = REPO_ROOT / "docs"

PLACEHOLDER = "REMOVED-NOT-A-SECRET-VALUE"

# A fenced code block: a line starting with three-or-more backticks or
# tildes (optionally indented, optionally followed by a language tag),
# up to the matching closing fence of the same character and length.
FENCE_RE = re.compile(r"^([ \t]*)(`{3,}|~{3,})")


def _doc_files():
    return sorted(
        path for path in DOCS_ROOT.rglob("*.md")
        if "archive" not in path.relative_to(DOCS_ROOT).parts
    )


def _fenced_blocks(text):
    """Yield (start_line, block_text) for every fenced code block in text."""
    lines = text.splitlines()
    i = 0
    while i < len(lines):
        m = FENCE_RE.match(lines[i])
        if m:
            fence_char = m.group(2)[0]
            fence_len = len(m.group(2))
            closing = re.compile(
                r"^[ \t]*" + re.escape(fence_char) + "{" + str(fence_len) + ",}[ \t]*$"
            )
            start = i
            i += 1
            block = []
            while i < len(lines) and not closing.match(lines[i]):
                block.append(lines[i])
                i += 1
            yield start + 2, "\n".join(block)
        i += 1


def _placeholder_hits(path):
    text = path.read_text()
    hits = []
    for start_line, block in _fenced_blocks(text):
        for offset, line in enumerate(block.splitlines()):
            if PLACEHOLDER in line:
                hits.append(start_line + offset)
    return hits


@pytest.mark.parametrize("path", _doc_files(), ids=lambda p: str(p.relative_to(DOCS_ROOT)))
def test_no_scrubber_placeholder_in_fenced_code_blocks(path):
    hits = _placeholder_hits(path)
    assert not hits, (
        f"{path.relative_to(REPO_ROOT)} has {PLACEHOLDER!r} inside a fenced "
        f"code block at line(s) {hits} — restore the real secret-by-reference "
        f"command instead of the scrubber placeholder"
    )


def test_fixture_sanity_fence_parser_finds_a_hit():
    """The fence parser itself is exercised: a synthetic hit must be caught.

    Guards against the parametrized test above passing vacuously if the
    fence regex ever stops matching real fences.
    """
    sample = (
        "prose with REMOVED-NOT-A-SECRET-VALUE outside a fence is not a hit\n"
        "\n"
        "```bash\n"
        f'curl -H "Authorization: Bearer {PLACEHOLDER}"\n'
        "```\n"
    )
    hits = []
    for start_line, block in _fenced_blocks(sample):
        for offset, line in enumerate(block.splitlines()):
            if PLACEHOLDER in line:
                hits.append(start_line + offset)
    assert hits == [4]
