#!/usr/bin/env python3
"""Reject operator documentation that overstates release evidence.

The repository has useful in-tree tests for several capabilities whose
published-image or fleet proof is incomplete.  This gate keeps that distinction
explicit: the canonical status document owns the evidence boundary, every
operator-facing surface links to it, and pending capabilities may not acquire
strong release/production claims in those surfaces.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


def load_status(root: Path) -> dict:
    path = root / "config" / "documentation-status.json"
    try:
        return json.loads(path.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read {path}: {exc}") from exc


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

    for document in documents:
        if not isinstance(document, str):
            errors.append(f"operator document is not a path: {document!r}")
            continue
        path = root / document
        if not path.is_file():
            errors.append(f"operator document is missing: {document}")
            continue
        text = path.read_text()
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
        if not capability_id or capability_id == "<missing-id>":
            errors.append("capability has no id")
            continue
        if capability_id not in status_text:
            errors.append(f"{status_doc} does not name capability {capability_id}")
        if capability_status not in status_text:
            errors.append(
                f"{status_doc} does not record status {capability_status!r} "
                f"for {capability_id}"
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
