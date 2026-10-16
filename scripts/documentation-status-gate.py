#!/usr/bin/env python3
"""Reject operator documentation that overstates release evidence.

The repository has useful in-tree tests for several capabilities whose
published-image or fleet proof is incomplete.  This gate keeps that distinction
explicit: the canonical status document owns the evidence boundary, every
operator-facing surface links to it, and pending capabilities may not acquire
strong release/production claims in those surfaces.  An active regression must
also remain visible as current evidence; citing its bead only as historical is
not an adequate status.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path


HISTORICAL_ONLY = re.compile(
    r"\bhistorical\s+(?:failure|incident|regression|bug)\b|"
    r"\b(?:formerly|previously)\s+(?:failed|broken|unreadable)\b",
    re.IGNORECASE,
)


def capability_tokens(capability: dict) -> list[str]:
    tokens = capability.get("tokens", [])
    return [str(token).lower() for token in tokens] if isinstance(tokens, list) else []


def active_evidence_terms(capability: dict) -> list[str]:
    terms = capability.get("active_evidence_terms", [])
    if not isinstance(terms, list) or not terms:
        return ["active regression", "not currently supported", "currently unverified"]
    return [str(term).lower() for term in terms]


def line_mentions_capability(line: str, capability: dict) -> bool:
    lowered = line.lower()
    capability_id = str(capability.get("id", "")).lower()
    return capability_id in lowered or any(
        token in lowered for token in capability_tokens(capability)
    )


def load_status(root: Path) -> dict:
    path = root / "config" / "documentation-status.json"
    try:
        return json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read {path}: {exc}") from exc


def load_bead_states(root: Path, bead_ids: set[str]) -> dict[str, str]:
    """Read the portable, committed bead-rs snapshot used by clean CI exports."""
    checkpoint = root / ".beads" / "checkpoint"
    try:
        current = json.loads((checkpoint / "current.json").read_text())
        relative = current["active_root"]["path"]
        if not re.fullmatch(r"objects/[0-9a-f]{64}\.jsonl", relative):
            raise ValueError(f"invalid checkpoint object path: {relative!r}")
        data = (checkpoint / relative).read_bytes()
        if hashlib.sha256(data).hexdigest() != Path(relative).stem:
            raise ValueError(f"checkpoint object has wrong digest: {relative}")
        states: dict[str, str] = {}
        for line in data.splitlines():
            record = json.loads(line)
            if record.get("record_type") != "issue":
                continue
            issue = record["issue"]
            bead_id = issue["id"]
            if bead_id in bead_ids:
                if bead_id in states:
                    raise ValueError(f"duplicate checkpoint issue: {bead_id}")
                states[bead_id] = issue["base_status"]
        return states
    except (OSError, KeyError, TypeError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read bead checkpoint: {exc}") from exc


def check(root: Path) -> list[str]:
    status = load_status(root)
    status_doc = status.get("status_document")
    documents = status.get("operator_documents")
    capabilities = status.get("capabilities")
    errors: list[str] = []

    if not isinstance(status_doc, str) or not status_doc:
        errors.append("status_document is missing")
        return errors
    status_path = root / status_doc
    if not status_path.is_file():
        errors.append(f"status document is missing: {status_doc}")
        return errors
    status_text = status_path.read_text()

    if not isinstance(documents, list) or not documents:
        errors.append("operator_documents must be a non-empty list")
        documents = []
    if not isinstance(capabilities, list) or not capabilities:
        errors.append("capabilities must be a non-empty list")
        capabilities = []

    cited_ids = {
        bead
        for capability in capabilities
        if isinstance(capability, dict)
        for bead in capability.get("evidence_beads", [])
        if isinstance(bead, str)
    }
    bead_states = load_bead_states(root, cited_ids)

    # The canonical register is itself an operator-facing surface.  Scan it
    # together with the configured documents so a status row cannot quietly
    # turn an active failure into a historical footnote.
    surfaces: list[tuple[str, str]] = [(status_doc, status_text)]

    for document in documents:
        if not isinstance(document, str):
            errors.append(f"operator document is not a path: {document!r}")
            continue
        path = root / document
        if not path.is_file():
            errors.append(f"operator document is missing: {document}")
            continue
        text = path.read_text()
        surfaces.append((document, text))
        if status_doc not in text and Path(status_doc).name not in text:
            errors.append(f"{document} does not link to {status_doc}")

        for line_number, line in enumerate(text.splitlines(), 1):
            lowered = line.lower()
            for capability in capabilities:
                if not isinstance(capability, dict):
                    errors.append(f"invalid capability entry: {capability!r}")
                    continue
                capability_id = capability.get("id", "<missing-id>")
                tokens = capability.get("tokens", [])
                forbidden = capability.get("forbidden_claims", [])
                if not isinstance(tokens, list) or not isinstance(forbidden, list):
                    errors.append(
                        f"{capability_id}: tokens and forbidden_claims must be lists"
                    )
                    continue
                if not any(str(token).lower() in lowered for token in tokens):
                    continue
                for expression in forbidden:
                    try:
                        matched = re.search(str(expression), line, re.IGNORECASE)
                    except re.error as exc:
                        errors.append(
                            f"{capability_id}: invalid claim regex {expression!r}: {exc}"
                        )
                        continue
                    if matched:
                        errors.append(
                            f"{document}:{line_number}: {capability_id} has "
                            f"unlicensed release claim {matched.group(0)!r}"
                        )

    for capability in capabilities:
        if not isinstance(capability, dict):
            continue
        capability_id = str(capability.get("id", "<missing-id>"))
        capability_status = str(capability.get("status", ""))
        evidence_beads = capability.get("evidence_beads", [])
        if not isinstance(evidence_beads, list):
            errors.append(f"{capability_id}: evidence_beads must be a list")
            continue
        if capability_status == "active-regression" and not evidence_beads:
            errors.append(
                f"{capability_id}: active-regression capabilities need evidence_beads"
            )
            continue
        evidence_beads = [str(bead) for bead in evidence_beads]
        for bead in evidence_beads:
            if not re.fullmatch(r"armor-[0-9a-f]+", bead):
                errors.append(
                    f"{capability_id}: invalid evidence bead reference {bead!r}"
                )
            elif bead not in bead_states:
                errors.append(f"{capability_id}: evidence bead {bead} is absent from checkpoint")

        known_states = [bead_states[bead] for bead in evidence_beads if bead in bead_states]
        if capability_status == "active-regression" and known_states and all(
            state == "closed" for state in known_states
        ) and len(known_states) == len(evidence_beads):
            errors.append(f"{capability_id}: active-regression has only closed evidence beads")
        if (capability_status == "repository-tested" or capability_status == "verified"
            or capability_status.endswith("-verified")):
            for bead in evidence_beads:
                if bead in bead_states and bead_states[bead] != "closed":
                    errors.append(
                        f"{capability_id}: {capability_status} cites non-closed bead "
                        f"{bead} ({bead_states[bead]})"
                    )

        if capability_status != "active-regression":
            continue

        tokens = capability_tokens(capability)
        terms = active_evidence_terms(capability)
        if not any(
            line_mentions_capability(line, capability)
            and any(term in line.lower() for term in terms)
            for line in status_text.splitlines()
        ):
            errors.append(
                f"{status_doc} does not record current active evidence for "
                f"{capability_id}"
            )

        for document, text in surfaces:
            for line_number, line in enumerate(text.splitlines(), 1):
                lowered = line.lower()
                if not any(token in lowered for token in tokens):
                    continue
                if not HISTORICAL_ONLY.search(line):
                    continue
                if any(term in lowered for term in terms):
                    continue
                errors.append(
                    f"{document}:{line_number}: {capability_id} describes an "
                    "active regression only as historical"
                )

    for capability in capabilities:
        if not isinstance(capability, dict):
            continue
        capability_id = str(capability.get("id", "<missing-id>"))
        capability_status = str(capability.get("status", ""))
        if not capability_id or capability_id == "<missing-id>":
            errors.append("capability has no id")
            continue
        if capability_id not in status_text:
            errors.append(f"{status_doc} does not name capability {capability_id}")
        evidence = " ".join(capability.get("evidence_beads", []))
        marker = f"<!-- capability: {capability_id} status: {capability_status}"
        if evidence:
            marker += f" evidence: {evidence}"
        marker += " -->"
        if marker not in status_text:
            errors.append(f"{status_doc} has no matching evidence marker for {capability_id}")
        table_label = capability.get("table_label")
        if not isinstance(table_label, str) or not table_label:
            errors.append(f"{capability_id}: table_label is missing")
            continue
        rows = [line for line in status_text.splitlines()
                if line.startswith(f"| {table_label} |")]
        if len(rows) != 1 or f"| `{capability_status}` |" not in rows[0]:
            errors.append(
                f"{status_doc} row for {capability_id} must record {capability_status}"
            )

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--root",
        type=Path,
        default=Path(__file__).resolve().parents[1],
        help="repository root (default: the parent of scripts/)",
    )
    args = parser.parse_args()
    try:
        errors = check(args.root.resolve())
    except ValueError as exc:
        print(f"documentation-status-gate: FAIL: {exc}", file=sys.stderr)
        return 2
    if errors:
        print("documentation-status-gate: FAIL", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1
    print("documentation-status-gate: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
