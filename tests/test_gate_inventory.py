#!/usr/bin/env python3
"""
Tests for the documented Python gate inventory — the parity pin between
every documented copy of the gate-suite list and the pytest invocation
scripts/definition-of-done.sh actually executes.

The inventory lives in five places: the repository map and the gate
section of AGENTS.md, the header comment of scripts/definition-of-done.sh,
the definition-of-done section of scripts/README.md, and the Suites table
of tests/README.md. They drifted independently until 2026-09-27 — the
AGENTS.md map row and scripts/README.md still predated
test_restore_verifier_inventory.py,
test_restore_verifier_scope_validation.py and
test_prohibited_constructs.py, and the tests/README.md table carried no
rows for five of the suites at all (armor-6d6ca9b1). The tests below parse
each documented copy and pin it to the script's own
`run "$PY" -m pytest ...` line, so a suite can no longer land
executed-but-undocumented, documented-but-not-executed, or
present-in-tests/-but-absent-from-the-table. This file is itself a gate
suite and lists itself in every copy it pins, which makes the
definition-of-done pytest leg self-enforcing.

Run: python3 -m pytest tests/test_gate_inventory.py -q
"""

import re
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
AGENTS = REPO_ROOT / "AGENTS.md"
DOD = REPO_ROOT / "scripts" / "definition-of-done.sh"
SCRIPTS_README = REPO_ROOT / "scripts" / "README.md"
TESTS_README = REPO_ROOT / "tests" / "README.md"

# The gate-section phrase separating "run by the definition of done" from
# suites that are documented but run another way (by hand). Rewording it
# must update this marker — these tests fail loudly rather than guess.
DOD_CLAIM_MARKER = "all run by the definition of done"

# A suite named in any of the documented lists, with or without the
# tests/ prefix (the AGENTS.md map row omits it, the gate bullet keeps it).
SUITE_TOKEN = re.compile(r"`(?:tests/)?(test_[a-z0-9_]+\.py)`")

# A suite named bare (no backticks) with the tests/ prefix — the form the
# definition-of-done script itself uses, in its invocation and header.
PREFIXED_SUITE = re.compile(r"\btests/(test_[a-z0-9_]+\.py)")

# A first-column Suites-table row in tests/README.md.
TABLE_ROW = re.compile(r"^\| `(tests/test_[a-z0-9_]+\.py)` \|", re.M)


def _suites(text):
    return set(SUITE_TOKEN.findall(text))


def _prefixed_suites(text):
    return set(PREFIXED_SUITE.findall(text))


# ---------------------------------------------------------------------------
# the anchor: what the gate actually executes
# ---------------------------------------------------------------------------

def dod_invocation_suites():
    """The suites enumerated on the script's `run "$PY" -m pytest` line."""
    lines = DOD.read_text().splitlines()
    invocations = [
        line for line in lines
        if line.startswith("run ") and " -m pytest " in line
    ]
    if len(invocations) != 1:
        pytest.fail(
            "scripts/definition-of-done.sh must carry exactly one "
            f"`run ... -m pytest ...` line, found {len(invocations)}"
        )
    suites = _prefixed_suites(invocations[0])
    if not suites:
        pytest.fail(
            "the definition-of-done pytest line no longer enumerates "
            "tests/test_*.py files; if the invocation shape changed, "
            "update the parsing in tests/test_gate_inventory.py"
        )
    return suites


# ---------------------------------------------------------------------------
# the documented copies
# ---------------------------------------------------------------------------

def dod_header_comment_suites():
    """The suites listed in the script's leading comment block."""
    comment = []
    for line in DOD.read_text().splitlines():
        if not line.startswith("#"):
            break
        comment.append(line)
    return _prefixed_suites("\n".join(comment))


def agents_map_suites():
    """The suites named in the AGENTS.md repository map's `tests/` row."""
    for line in AGENTS.read_text().splitlines():
        if line.startswith("| `tests/` |"):
            return _suites(line)
    pytest.fail("AGENTS.md repository map no longer carries a `tests/` row")


def agents_gate_suites():
    """(DoD-claimed, other) suites from the gate section's Python bullet."""
    lines = AGENTS.read_text().splitlines()
    starts = [i for i, line in enumerate(lines) if line.startswith("- Python:")]
    if not starts:
        pytest.fail(
            "AGENTS.md gate section no longer carries a '- Python:' bullet"
        )
    bullet = [lines[starts[0]]]
    for line in lines[starts[0] + 1:]:
        if line.startswith("  "):
            bullet.append(line)
        else:
            break
    text = " ".join(part.strip() for part in bullet)
    if DOD_CLAIM_MARKER not in text:
        pytest.fail(
            f"AGENTS.md's Python bullet no longer contains the marker "
            f"'{DOD_CLAIM_MARKER}'. Either restore the claim or update "
            f"DOD_CLAIM_MARKER in tests/test_gate_inventory.py to the new "
            f"wording"
        )
    claimed, _, rest = text.partition(DOD_CLAIM_MARKER)
    return _suites(claimed), _suites(rest)


def scripts_readme_dod_suites():
    """The suites listed in scripts/README.md's DoD section (only)."""
    section = []
    in_section = False
    for line in SCRIPTS_README.read_text().splitlines():
        if line.startswith("## "):
            if in_section:
                break
            in_section = line == "## definition-of-done.sh"
            continue
        if in_section:
            section.append(line)
    if not in_section:
        pytest.fail(
            "scripts/README.md no longer carries a "
            "'## definition-of-done.sh' section"
        )
    return _suites("\n".join(section))


def table_row_suites():
    """The `tests/test_*.py` paths that have a Suites-table row."""
    return set(TABLE_ROW.findall(TESTS_README.read_text()))


def pytest_files_in_tests_dir():
    """Every pytest-shaped file at the top of tests/."""
    return {f"tests/{path.name}" for path in (REPO_ROOT / "tests").glob("test_*.py")}


# ---------------------------------------------------------------------------
# the pins
# ---------------------------------------------------------------------------

def test_gate_section_claim_matches_invocation():
    """The suites AGENTS.md says the DoD runs are exactly the ones it runs."""
    claimed, documented_other = agents_gate_suites()
    executed = dod_invocation_suites()
    assert claimed == executed, (
        "AGENTS.md's gate section and scripts/definition-of-done.sh "
        "disagree on the DoD-run suites — only documented: "
        f"{sorted(claimed - executed)}, only executed: "
        f"{sorted(executed - claimed)}"
    )
    assert not documented_other & executed, (
        "suites named in AGENTS.md after the DoD claim must not also be in "
        f"the invocation: {sorted(documented_other & executed)}"
    )


def test_repository_map_matches_gate_section():
    """The map row's 'Python gate suites' list is the same inventory."""
    mapped = agents_map_suites()
    claimed, documented_other = agents_gate_suites()
    assert mapped == claimed | documented_other, (
        "the AGENTS.md repository map and the gate section disagree on the "
        f"Python gate suites — only in the map: "
        f"{sorted(mapped - claimed - documented_other)}, only in the gate "
        f"section: {sorted((claimed | documented_other) - mapped)}"
    )


def test_dod_header_comment_matches_invocation():
    """The script's own header lists the suites it executes."""
    assert dod_header_comment_suites() == dod_invocation_suites(), (
        "the comment block of scripts/definition-of-done.sh no longer "
        "matches its own pytest invocation"
    )


def test_scripts_readme_dod_section_matches_invocation():
    """The scripts/README.md gate section lists the suites DoD executes."""
    assert scripts_readme_dod_suites() == dod_invocation_suites(), (
        "the definition-of-done section of scripts/README.md no longer "
        "matches the pytest invocation in scripts/definition-of-done.sh"
    )


def test_tests_readme_table_covers_every_pytest_file():
    """Every tests/test_*.py file has a Suites row; every row has a file."""
    rows = table_row_suites()
    files = pytest_files_in_tests_dir()
    assert rows == files, (
        "tests/README.md's Suites table and the tests/ directory disagree "
        f"— missing rows: {sorted(files - rows)}, rows without a file: "
        f"{sorted(rows - files)}"
    )
