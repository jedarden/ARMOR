#!/usr/bin/env python3
"""
Validate declarative-config against the ADR-004 restore-verifier scope
contract.

The contract (ADR-004, Decision 4 as amended by the 2026-09-25 Addendum and
the "Fleet topology" section): exactly one restore-verifier Deployment per
bucket scope (bucket + MEK + B2 credential set); a Deployment never straddles
scopes; each carries exactly one ARMOR_BUCKET and one ARMOR_MEK referenced
from that scope's own secret store; fleet coverage is the union of the
per-scope Deployments.

Checks, per verifier Deployment (counted pod-wide, init containers included):

- exactly one ARMOR_BUCKET and one ARMOR_MEK, each a valueFrom reference —
  missing entries and duplicates are both errors (a second bucket or MEK
  anywhere in the pod is a scope straddle);
- no `envFrom` anywhere in the pod — a wholesale env import makes the
  exactly-one contract unverifiable;
- every credential-shaped key present (MEK, MEK-ring keys, B2 keys — the
  current ARMOR_B2_* names and the legacy B2_KEY_ID/B2_KEY fallbacks) is a
  valueFrom reference, never a literal: a literal is a violation of the
  secrets-by-reference rule as well as the scope contract;
- ARMOR_BUCKET_ALIASES is allowed and never counts as a second bucket
  (aliases are alternate names for the same bucket); ARMOR_SECONDARY_B2_*
  replication keys on a verifier ARE a second scope and are rejected;
- no two verifier Deployments resolve to the same scope identity
  (cluster, namespace, bucket source, MEK source, B2 key sources) —
  namespace-qualified because manifests can only prove sameness of the same
  Secret or ConfigMap object;
- every proxy-declared scope (a non-verifier workload classified by
  find-armor-deployments.py that declares ARMOR_BUCKET) is covered by a
  verifier with the same bucket source in the same cluster and namespace.
  Proxy-declared scopes with no verifier are reported, not failed: ADR-004
  makes verifier rollout a per-scope decision, and the live fleet
  legitimately has proxies with no verifier. Pass --strict-coverage to fail
  on those too once the known gaps are closed.

Discovery is delegated to find-armor-deployments.py, so its classification
rules (argo-workflows exclusion, `.disabled` files, placeholder tags) apply
here too.

Exit codes: 0 valid (warnings allowed), 1 validation errors, 2 structural
(no k8s/ tree, or zero verifier Deployments discovered).

Run: python3 scripts/validate-restore-verifier-scopes.py [path] [--json]
"""

import argparse
import importlib.util
import json
import os
import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[1]
_SPEC = importlib.util.spec_from_file_location(
    "find_armor_deployments", REPO_ROOT / "scripts" / "find-armor-deployments.py")
find_armor_deployments = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(find_armor_deployments)

VERIFIER_IMAGE_TYPE = "armor-restore-verifier"

# The env keys whose count defines the exactly-one scope contract.
BUCKET_KEY = "ARMOR_BUCKET"
MEK_KEY = "ARMOR_MEK"

# Alternate names for the same bucket (ADR-004 Addendum) — never a second
# bucket, so never a straddle.
BUCKET_ALIAS_KEYS = ("ARMOR_BUCKET_ALIASES",)

# Retired ring keys where rotation has run (ADR-004 Addendum) — allowed
# alongside the one ARMOR_MEK, but still credential-shaped: by reference
# only.
RING_KEYS = ("VERIFIER_MEK_RING",)

# Replication config names a second bucket and second credential set; on a
# verifier that is a scope straddle by definition.
SECOND_BUCKET_KEYS = ("ARMOR_SECONDARY_B2_BUCKET",)

# Credential-shaped env keys: a literal is a violation of the
# secrets-by-reference rule as well as the scope contract. The B2 key pair
# has current (ARMOR_B2_*) and legacy (B2_KEY_ID/B2_KEY, read by
# internal/backend) spellings; the secondary pair is replication's.
CREDENTIAL_KEYS = (
    "ARMOR_B2_ACCESS_KEY_ID",
    "ARMOR_B2_SECRET_ACCESS_KEY",
    "B2_KEY_ID",
    "B2_KEY",
    "ARMOR_SECONDARY_B2_KEY_ID",
    "ARMOR_SECONDARY_B2_KEY",
) + RING_KEYS

EXIT_OK = 0
EXIT_ERRORS = 1
EXIT_STRUCTURAL = 2


def _ref_source(value_from):
    """Canonical scope-source string for a valueFrom block, or None when the
    reference is not a configmap/secret key reference (fieldRef, resourceFieldRef,
    ...): those cannot prove scope sameness, so they cannot carry a scope value."""
    if not isinstance(value_from, dict):
        return None
    secret_ref = value_from.get("secretKeyRef")
    if isinstance(secret_ref, dict) and secret_ref.get("name") and secret_ref.get("key"):
        return f"secret/{secret_ref['name']}/{secret_ref['key']}"
    cm_ref = value_from.get("configMapKeyRef")
    if isinstance(cm_ref, dict) and cm_ref.get("name") and cm_ref.get("key"):
        return f"configmap/{cm_ref['name']}/{cm_ref['key']}"
    return None


def _pod_containers(pod_spec):
    """(origin, container) for every init and main container, init first —
    the exactly-one counts are pod-wide."""
    for origin in ("initContainers", "containers"):
        for container in pod_spec.get(origin) or []:
            yield origin, container


def _env_entries(container):
    """(name, source) per env entry: source is the canonical ref string, the
    literal value prefixed `literal:`, or None for an unresolvable valueFrom."""
    entries = []
    for entry in container.get("env") or []:
        if not isinstance(entry, dict) or not entry.get("name"):
            continue
        value_from = entry.get("valueFrom")
        if value_from is not None:
            entries.append((entry["name"], _ref_source(value_from)))
        else:
            entries.append((entry["name"], f"literal:{entry.get('value')}"))
    return entries


def _pod_env(pod_spec):
    """The pod's env entries with their container origin, pod-wide."""
    entries = []
    for origin, container in _pod_containers(pod_spec):
        for name, source in _env_entries(container):
            entries.append((origin, container.get("name"), name, source))
    return entries


def _workload_docs(filepath):
    """(kind, metadata, pod_spec) per Deployment document in the file."""
    with open(filepath, "r") as handle:
        for doc in yaml.safe_load_all(handle):
            if not isinstance(doc, dict) or doc.get("kind") != "Deployment":
                continue
            metadata = doc.get("metadata") or {}
            pod_spec = (doc.get("spec") or {}).get("template", {}).get("spec") or {}
            yield doc, metadata, pod_spec


def _select_verifier_doc(record):
    """The Deployment document the finder's record points at. Records are
    matched on name when the finder extracted one (multi-document files —
    e.g. a Deployment plus its Service — must select the workload, not the
    Service's siblings), else the first Deployment document."""
    docs = list(_workload_docs(record["filepath"]))
    if not docs:
        return None
    name = record.get("workload_name")
    if name:
        named = [d for d in docs if (d[1] or {}).get("name") == name]
        if len(named) == 1:
            return named[0]
    return docs[0]


def _finding(severity, code, message, subject, filepath=None):
    return {
        "severity": severity,
        "code": code,
        "message": message,
        "subject": subject,
        "filepath": filepath,
    }


def validate(declarative_config_path, strict_coverage=False):
    """Validate one declarative-config checkout. Returns the report dict; the
    exit code is derived by `derive_exit`."""
    findings = []
    records = find_armor_deployments.find_armor_deployments(declarative_config_path)
    verifier_records = [r for r in records if r["image_type"] == VERIFIER_IMAGE_TYPE]
    other_records = [r for r in records if r["image_type"] != VERIFIER_IMAGE_TYPE]

    verifiers = []
    for record in sorted(verifier_records,
                         key=lambda r: (r["cluster"], r["namespace"], r["workload_name"])):
        subject = (f"{record['cluster']}/{record['namespace']} "
                   f"{record['workload_name']}")
        selected = _select_verifier_doc(record)
        if selected is None:
            findings.append(_finding(
                "error", "verifier-unreadable",
                f"{subject}: no Deployment document found in {record['filepath']}",
                subject, record["filepath"]))
            continue
        _doc, _metadata, pod_spec = selected
        env = _pod_env(pod_spec)

        # envFrom imports env wholesale, so the exactly-one contract could
        # not be verified from the manifest.
        for origin, container in _pod_containers(pod_spec):
            if container.get("envFrom"):
                findings.append(_finding(
                    "error", "verifier-envfrom",
                    f"{subject}: container {container.get('name')} ({origin}) "
                    f"uses envFrom — a wholesale env import makes the "
                    f"exactly-one scope contract unverifiable", subject,
                    record["filepath"]))

        bucket_source = _scope_value(
            env, BUCKET_KEY, subject, record["filepath"], findings,
            "verifier-bucket-not-reference")
        mek_source = _scope_value(
            env, MEK_KEY, subject, record["filepath"], findings,
            "verifier-mek-not-reference")

        # Credential-shaped keys: by reference only. RING_KEYS are allowed
        # additions (retired ring keys), the rest are required to be refs
        # whenever present.
        credential_sources = []
        for _origin, _container, name, source in env:
            if name not in CREDENTIAL_KEYS:
                continue
            if not source or source.startswith("literal:") or not source.split("/", 1)[0] in ("secret", "configmap"):
                findings.append(_finding(
                    "error", "literal-credential",
                    f"{subject}: {name} is not a secretKeyRef/configMapKeyRef "
                    f"reference ({source}) — credential values travel by "
                    f"reference (secrets-by-reference rule, ADR-004)",
                    subject, record["filepath"]))
            elif source not in credential_sources:
                credential_sources.append(source)

        for key in SECOND_BUCKET_KEYS:
            if any(name == key for _o, _c, name, _s in env):
                findings.append(_finding(
                    "error", "verifier-straddles-buckets",
                    f"{subject}: {key} present — replication's second bucket "
                    f"is a second scope, and a Deployment never straddles "
                    f"scopes", subject, record["filepath"]))

        if bucket_source and mek_source:
            verifiers.append({
                "cluster": record["cluster"],
                "namespace": record["namespace"],
                "workload": record["workload_name"],
                "filepath": record["filepath"],
                "bucket_source": bucket_source,
                "mek_source": mek_source,
                "credential_sources": credential_sources,
                "identity": [record["cluster"], record["namespace"],
                             bucket_source, mek_source,
                             sorted(credential_sources)],
            })

    # No two verifier Deployments may resolve to the same scope identity.
    seen = {}
    for verifier in verifiers:
        key = json.dumps(verifier["identity"])
        if key in seen:
            findings.append(_finding(
                "error", "duplicate-scope-identity",
                f"{verifier['cluster']}/{verifier['namespace']} "
                f"{verifier['workload']} resolves to the same scope identity "
                f"as {seen[key]['workload']} (bucket {verifier['bucket_source']}, "
                f"MEK {verifier['mek_source']}) — exactly one verifier "
                f"Deployment per scope", verifier["workload"],
                verifier["filepath"]))
        else:
            seen[key] = verifier

    # Coverage: every proxy-declared scope needs a verifier with the same
    # bucket source in the same cluster and namespace.
    verifier_buckets = {(v["cluster"], v["namespace"], v["bucket_source"]): v["workload"]
                        for v in verifiers}
    scopes = []
    for record in sorted(other_records,
                         key=lambda r: (r["cluster"], r["namespace"], r["workload_name"])):
        selected = _select_verifier_doc(record)
        if selected is None:
            continue
        _doc, _metadata, pod_spec = selected
        for _origin, _container, name, source in _pod_env(pod_spec):
            if name != BUCKET_KEY or not source:
                continue
            scope = (record["cluster"], record["namespace"], source)
            covered_by = verifier_buckets.get(scope)
            scopes.append({
                "cluster": record["cluster"],
                "namespace": record["namespace"],
                "bucket_source": source,
                "declared_by": record["workload_name"],
                "covered_by": covered_by,
                "filepath": record["filepath"],
            })
            if covered_by is None:
                findings.append(_finding(
                    "error" if strict_coverage else "warning",
                    "uncovered-scope",
                    f"{record['cluster']}/{record['namespace']}: scope "
                    f"declared by {record['workload_name']} (bucket source "
                    f"{source}) has no restore-verifier Deployment — ADR-004 "
                    f"coverage is the union of the per-scope Deployments",
                    record["workload_name"], record["filepath"]))
            break  # one scope per workload: the exactly-one contract's proxy side

    return {
        "declarative_config": str(declarative_config_path),
        "strict_coverage": strict_coverage,
        "verifiers": verifiers,
        "scopes": scopes,
        "findings": findings,
    }


def _scope_value(env, key, subject, filepath, findings, not_reference_code):
    """Validate exactly-one + by-reference for one scope key (see
    _exactly_one_scope_value above for the contract); returns the canonical
    source string or None."""
    entries = [(origin, container, source) for origin, container, name, source
               in env if name == key]
    code_prefix = f"verifier-{key.lower().replace('_', '-')}"
    if not entries:
        findings.append(_finding(
            "error", f"verifier-missing-{key.lower().replace('_', '-')}",
            f"{subject}: no {key} env anywhere in the pod — the scope "
            f"contract requires exactly one", subject, filepath))
        return None
    if len(entries) > 1:
        where = ", ".join(sorted({f"{origin}/{container}" for origin, container, _ in entries}))
        findings.append(_finding(
            "error", f"verifier-straddles-{key.lower().replace('_', '-')}s",
            f"{subject}: {len(entries)} {key} env entries ({where}) — a "
            f"Deployment never straddles scopes", subject, filepath))
        return None
    source = entries[0][2]
    if not source or not source.split("/", 1)[0] in ("secret", "configmap"):
        findings.append(_finding(
            "error", not_reference_code,
            f"{subject}: {key} is not a secretKeyRef/configMapKeyRef reference "
            f"({source}) — ADR-004 requires the scope values referenced from "
            f"the scope's own secret store", subject, filepath))
        return None
    return source


def derive_exit(report):
    """Structural break (checked by the caller) is 2; any error finding is 1;
    warnings alone are 0."""
    if any(f["severity"] == "error" for f in report["findings"]):
        return EXIT_ERRORS
    return EXIT_OK


def _print_human(report, stream):
    print("restore-verifier Deployments and their scopes:", file=stream)
    for v in report["verifiers"]:
        creds = ", ".join(v["credential_sources"]) or "-"
        print(f"  {v['cluster']}/{v['namespace']} {v['workload']}: "
              f"bucket={v['bucket_source']} mek={v['mek_source']} creds=[{creds}]",
              file=stream)
    print("\nProxy-declared scopes and their coverage:", file=stream)
    if not report["scopes"]:
        print("  (no proxy-declared scopes discovered)", file=stream)
    for s in report["scopes"]:
        state = s["covered_by"] or "UNCOVERED"
        print(f"  {s['cluster']}/{s['namespace']} {s['bucket_source']} "
              f"(declared by {s['declared_by']}): {state}", file=stream)
    print("\nFindings:", file=stream)
    if not report["findings"]:
        print("  (none)", file=stream)
    for f in report["findings"]:
        print(f"  {f['severity'].upper()} {f['code']}: {f['message']}", file=stream)
    errors = sum(1 for f in report["findings"] if f["severity"] == "error")
    warnings = sum(1 for f in report["findings"] if f["severity"] == "warning")
    print(f"\n{len(report['verifiers'])} verifier Deployment(s), "
          f"{len(report['scopes'])} proxy-declared scope(s), "
          f"{errors} error(s), {warnings} warning(s)", file=stream)


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Validate declarative-config against the ADR-004 "
                    "restore-verifier scope contract.")
    parser.add_argument("path", nargs="?", default=None,
                        help="path to a declarative-config checkout "
                             "(default ~/declarative-config)")
    parser.add_argument("--json", action="store_true",
                        help="emit the machine-readable report on stdout")
    parser.add_argument("--strict-coverage", action="store_true",
                        help="fail on proxy-declared scopes with no verifier "
                             "instead of warning")
    args = parser.parse_args(argv)

    path = args.path or os.path.expanduser("~/declarative-config")
    if not os.path.isdir(os.path.join(path, "k8s")):
        print(f"Error: no k8s/ tree under {path}", file=sys.stderr)
        return EXIT_STRUCTURAL

    report = validate(path, strict_coverage=args.strict_coverage)
    if not report["verifiers"]:
        print("Error: no restore-verifier Deployments discovered — the "
              "scope contract cannot be validated against an empty fleet",
              file=sys.stderr)
        return EXIT_STRUCTURAL

    if args.json:
        print(json.dumps(report, indent=2, sort_keys=True))
    else:
        _print_human(report, sys.stdout)

    errors = sum(1 for f in report["findings"] if f["severity"] == "error")
    warnings = sum(1 for f in report["findings"] if f["severity"] == "warning")
    print(f"validate-restore-verifier-scopes: {errors} error(s), "
          f"{warnings} warning(s)", file=sys.stderr)
    return derive_exit(report)


if __name__ == "__main__":
    sys.exit(main())
