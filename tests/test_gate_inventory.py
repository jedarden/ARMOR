#!/usr/bin/env python3
"""
Tests for the Python gate inventory — the single-edit-point pin.

The inventory lives in ONE place: tests/gate-suites.txt, one suite per
line. scripts/definition-of-done.sh executes exactly those suites — it
reads the file for its pytest leg — so the gate that runs cannot drift
from the file. The same list was hand-copied into five documents and they
drifted independently until 2026-09-27 (armor-6d6ca9b1: the AGENTS.md map
row and scripts/README.md predated three suites, and the tests/README.md
table carried no rows for five of them); the copies were then pinned to
the invocation, and armor-2c918e99 collapsed them for good: the surfaces
that used to carry hand-copied lists — the AGENTS.md repository map and
gate section, the definition-of-done.sh header, the scripts/README.md
gate section — now point at the file instead of enumerating suites.

The tests below hold every piece to the file: the inventory lists only
existing tests/test_*.py suites, the script's pytest leg executes the
file's contents rather than an inline list, every gate suite has a row in
the tests/README.md Suites table (and every row has a file), and each
former copy surface references the file and does not enumerate suites.
This file is itself a gate suite and lists itself in the inventory, which
makes the definition-of-done pytest leg self-enforcing.

Adding a gate suite is one edit: a line in tests/gate-suites.txt. (A new
tests/test_*.py file also needs a Suites table row — that is the
per-suite documentation these tests force, not a second inventory.)

The suite also pins the --fast hermetic contract over the gate
invocations: every go test leg of scripts/cli-contract-gate.sh runs
-short, so the browser-mount smokes inside cmd/ skip and the fast lane
needs no browser and no network — the property the definition-of-done
header, AGENTS.md, and scripts/README.md all document.

Run: python3 -m pytest tests/test_gate_inventory.py -q
"""

import re
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
AGENTS = REPO_ROOT / "AGENTS.md"
CLI_CONTRACT_GATE = REPO_ROOT / "scripts" / "cli-contract-gate.sh"
DOD = REPO_ROOT / "scripts" / "definition-of-done.sh"
INVENTORY = REPO_ROOT / "tests" / "gate-suites.txt"
SCRIPTS_README = REPO_ROOT / "scripts" / "README.md"
TESTS_README = REPO_ROOT / "tests" / "README.md"

# The single edit point every surface must point at.
INVENTORY_REF = "tests/gate-suites.txt"

# A suite token in prose, with or without backticks and the tests/ prefix
# (the old copies used both shapes).
SUITE_TOKEN = re.compile(r"`?(?:tests/)?(test_[a-z0-9_]+\.py)`?")

# A first-column Suites-table row in tests/README.md.
TABLE_ROW = re.compile(r"^\| `(tests/test_[a-z0-9_]+\.py)` \|", re.M)

# This suite may name itself in prose — it IS the pin. No other suite may
# be enumerated on a former copy surface; that enumeration is what
# drifted five ways before the single edit point existed.
SELF_REFERENCE = "test_gate_inventory.py"


# ---------------------------------------------------------------------------
# the anchor: the single edit point and what the gate actually executes
# ---------------------------------------------------------------------------

def inventory_suites():
    """The suites enumerated by tests/gate-suites.txt, in file order."""
    if not INVENTORY.is_file():
        pytest.fail(
            f"{INVENTORY_REF} is missing — the Python gate inventory has "
            "no single edit point"
        )
    suites = [
        line.strip()
        for line in INVENTORY.read_text().splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]
    if not suites:
        pytest.fail(f"{INVENTORY_REF} lists no suites")
    return suites


def dod_pytest_line():
    """The script's single `run ... -m pytest ...` line."""
    lines = DOD.read_text().splitlines()
    invocations = [
        line for line in lines
        if line.lstrip().startswith("run ") and " -m pytest " in line
    ]
    if len(invocations) != 1:
        pytest.fail(
            "scripts/definition-of-done.sh must carry exactly one "
            f"`run ... -m pytest ...` line, found {len(invocations)}"
        )
    return invocations[0]


# ---------------------------------------------------------------------------
# the surfaces that used to carry hand-copied lists
# ---------------------------------------------------------------------------

def agents_map_row():
    """The AGENTS.md repository map's `tests/` row."""
    for line in AGENTS.read_text().splitlines():
        if line.startswith("| `tests/` |"):
            return line
    pytest.fail("AGENTS.md repository map no longer carries a `tests/` row")


def agents_python_bullet():
    """The AGENTS.md gate section's Python bullet (with continuations)."""
    lines = AGENTS.read_text().splitlines()
    starts = [i for i, line in enumerate(lines) if line.startswith("- Python:")]
    if not starts:
        pytest.fail("AGENTS.md gate section no longer carries a '- Python:' bullet")
    bullet = [lines[starts[0]]]
    for line in lines[starts[0] + 1:]:
        if line.startswith("  "):
            bullet.append(line)
        else:
            break
    return " ".join(part.strip() for part in bullet)


def dod_header_comment():
    """The definition-of-done script's leading comment block."""
    comment = []
    for line in DOD.read_text().splitlines():
        if not line.startswith("#"):
            break
        comment.append(line)
    return "\n".join(comment)


def scripts_readme_dod_section():
    """The scripts/README.md definition-of-done section (only)."""
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
    return "\n".join(section)


def former_copy_surfaces():
    return {
        "the AGENTS.md repository map `tests/` row": agents_map_row(),
        "the AGENTS.md gate-section Python bullet": agents_python_bullet(),
        "the scripts/definition-of-done.sh header comment": dod_header_comment(),
        "the scripts/README.md definition-of-done section": scripts_readme_dod_section(),
    }


# ---------------------------------------------------------------------------
# the pins
# ---------------------------------------------------------------------------

def test_inventory_lists_existing_pytest_suites():
    """Every inventory line is an existing, unique tests/test_*.py suite."""
    suites = inventory_suites()
    assert len(set(suites)) == len(suites), (
        f"duplicate suite in {INVENTORY_REF}: "
        f"{sorted({s for s in suites if suites.count(s) > 1})}"
    )
    for suite in suites:
        assert re.fullmatch(r"tests/test_[a-z0-9_]+\.py", suite), (
            f"inventory line {suite!r} is not a tests/test_*.py path"
        )
        assert (REPO_ROOT / suite).is_file(), (
            f"{suite} is listed in {INVENTORY_REF} but does not exist"
        )


def test_dod_pytest_leg_executes_the_inventory():
    """The gate executes the file's suites, not an inline list."""
    line = dod_pytest_line()
    assert '"${gate_suites[@]}"' in line, (
        "the definition-of-done pytest leg must execute the gate_suites "
        f"array read from {INVENTORY_REF}, not an inline suite list"
    )
    assert not re.search(r"\btests/test_", line), (
        "the definition-of-done pytest leg enumerates suites inline — "
        f"add them to {INVENTORY_REF} instead"
    )
    assert f"gate_suites_file={INVENTORY_REF}" in DOD.read_text(), (
        "the definition-of-done script must load its pytest suites from "
        f"{INVENTORY_REF}"
    )


def test_former_copy_surfaces_point_at_the_inventory():
    """Each surface that used to carry a hand-copied list points at the file."""
    for name, text in former_copy_surfaces().items():
        assert INVENTORY_REF in text, (
            f"{name} must reference {INVENTORY_REF} — the single edit point "
            "for the gate inventory"
        )
        enumerated = [
            token for token in SUITE_TOKEN.findall(text)
            if token != SELF_REFERENCE
        ]
        assert not enumerated, (
            f"{name} enumerates gate suites ({sorted(set(enumerated))}) — "
            f"the inventory lives only in {INVENTORY_REF}; hand-copied "
            "lists are what drifted before armor-2c918e99"
        )


def test_every_gate_suite_has_a_suites_row():
    """No suite can be executed by the gate but undocumented in the table."""
    rows = set(TABLE_ROW.findall(TESTS_README.read_text()))
    missing = set(inventory_suites()) - rows
    assert not missing, (
        "suites executed by the definition of done but missing a "
        f"tests/README.md Suites row: {sorted(missing)}"
    )


def test_suites_table_rows_all_exist():
    """Every tests/test_*.py file has a Suites row; every row has a file."""
    rows = set(TABLE_ROW.findall(TESTS_README.read_text()))
    files = {f"tests/{path.name}" for path in (REPO_ROOT / "tests").glob("test_*.py")}
    assert rows == files, (
        "tests/README.md's Suites table and the tests/ directory disagree "
        f"— missing rows: {sorted(files - rows)}, rows without a file: "
        f"{sorted(rows - files)}"
    )


# ---------------------------------------------------------------------------
# the --fast hermetic contract on the gate invocations
# ---------------------------------------------------------------------------

def test_cli_contract_gate_test_legs_run_short():
    """Every go test leg of the contract gate runs -short.

    The contract gate is a definition-of-done --fast leg, and --fast is
    documented (script header, AGENTS.md, scripts/README.md) as needing
    no network and no browser. The browser-mount smokes inside cmd/armor
    and cmd/armor-fleet skip under -short; the full gate owns them via
    scripts/verify-agentation-mount.sh. The legs and the docs drifted
    apart once — the script shipped without the -short the docs claimed
    — so this holds the invocations to the documented contract.
    """
    legs = [
        line.strip()
        for line in CLI_CONTRACT_GATE.read_text().splitlines()
        if '"$GO" test' in line
    ]
    assert legs, "cli-contract-gate.sh no longer runs any go test leg"
    missing = [line for line in legs if "-short" not in line]
    assert not missing, (
        "cli-contract-gate.sh go test leg(s) without -short make --fast "
        "non-hermetic (the cmd browser-mount smokes would run and reach "
        "for a browser and esm.sh): " + "; ".join(missing)
    )
