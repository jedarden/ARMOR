#!/usr/bin/env python3
"""
Inventory pin for the restore-verifier fleet — the deployment-topology
contract from ADR-004 (the Addendum and the "Fleet topology" section).

Two things are pinned here:

1. scripts/find-armor-deployments.py classifies per-workload manifests
   correctly (cluster from the k8s/<cluster>/ path segment, kind/name/
   namespace from the metadata block, image_type from the image line)
   against fixture manifests modeled on the live ones — including the files
   that must NOT classify: the ARMOR proxy Deployment itself, a `.disabled`
   monitoring manifest, an Argo WorkflowTemplate under k8s/*/argo-workflows/,
   an ExternalSecret carrying no image line, and an unexpanded $-placeholder
   tag.

2. The four prose surfaces that describe the fleet (ADR-004, the deployment
   guide, the alerting runbook, and plan.md's bump list) agree with
   GOLDEN_RESTORE_VERIFIER_INVENTORY. The mechanical enumeration —
   `python3 scripts/find-armor-deployments.py ~/declarative-config`, filtered
   to image_type == armor-restore-verifier — is authoritative; this test is
   the drift alarm that keeps the prose honest. When the fleet changes
   (scope added, retired, or re-homed in declarative-config), update
   GOLDEN_RESTORE_VERIFIER_INVENTORY and every prose count it checks in the
   same change.

Run: python3 -m pytest tests/test_restore_verifier_inventory.py -q
"""

import importlib.util
import re
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parents[1]
_FINDER_SCRIPT = REPO_ROOT / "scripts" / "find-armor-deployments.py"
_spec = importlib.util.spec_from_file_location(
    "find_armor_deployments", _FINDER_SCRIPT)
find_armor_deployments = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(find_armor_deployments)

# (cluster, namespace, workload_name) — a snapshot of
# `python3 scripts/find-armor-deployments.py ~/declarative-config` filtered
# to image_type == "armor-restore-verifier", taken 2026-09-25 (armor-79255e46).
# restore-verifier-acb verifies apexalgo-iad's armor-apexalgo bucket from
# rs-manager while that cluster's ArgoCD sync is broken; the plain
# rs-manager verifier was removed 2026-09-23 (declarative-config abe7dd0c).
GOLDEN_RESTORE_VERIFIER_INVENTORY = [
    ("iad-ci", "armor", "restore-verifier"),
    ("iad-kalshi", "armor", "restore-verifier"),
    ("ord-devimprint", "devimprint", "restore-verifier"),
    ("rs-manager", "armor", "restore-verifier-acb"),
]

# Every prose surface checked below states the fleet size next to
# "restore-verifier" ("four restore-verifier Deployments", "the **four**
# restore-verifier Deployments", ...). Derived from the golden length so a
# fleet-size change fails loudly until the prose catches up.
_SIZE_WORDS = {2: "two", 3: "three", 4: "four", 5: "five", 6: "six", 7: "seven"}
FLEET_SIZE_WORD = _SIZE_WORDS[len(GOLDEN_RESTORE_VERIFIER_INVENTORY)]

DOC_SURFACES = [
    REPO_ROOT / "docs" / "adr" / "004-continuous-restore-verification.md",
    REPO_ROOT / "docs" / "restore-verifier-deployment-guide.md",
    REPO_ROOT / "docs" / "runbooks" / "restore-verifier-alerting.md",
    REPO_ROOT / "docs" / "plan" / "plan.md",
]

_HOWTO = (
    "The mechanical enumeration is `python3 scripts/find-armor-deployments.py "
    "~/declarative-config` filtered to image_type == armor-restore-verifier. "
    "Update GOLDEN_RESTORE_VERIFIER_INVENTORY in "
    "tests/test_restore_verifier_inventory.py and this prose in the same "
    "change."
)


# ---------------------------------------------------------------------------
# fixture manifests, modeled on the live declarative-config ones
# ---------------------------------------------------------------------------

def _colocated_verifier(name, tag):
    """The co-located pattern: env by reference from the proxy's ConfigMap +
    Secret sources (iad-ci/armor, iad-kalshi/armor)."""
    return f"""\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {name}
  namespace: armor
  labels:
    app: {name}
    app.kubernetes.io/part-of: armor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {name}
  template:
    spec:
      containers:
        - name: restore-verifier
          image: ronaldraygun/armor-restore-verifier:{tag}
          env:
            - name: ARMOR_BUCKET
              valueFrom:
                configMapKeyRef:
                  name: armor-config
                  key: ARMOR_BUCKET
            - name: ARMOR_MEK
              valueFrom:
                secretKeyRef:
                  name: armor-secrets
                  key: mek
            - name: VERIFIER_HTTP_LISTEN
              value: "0.0.0.0:9002"
"""


def _credentials_verifier(tag):
    """The ord-devimprint pattern: all B2/MEK values in armor-credentials,
    plus an explicit ARMOR_PREFIX on the verifier itself."""
    return f"""\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: restore-verifier
  namespace: devimprint
spec:
  template:
    spec:
      containers:
        - name: restore-verifier
          image: ronaldraygun/armor-restore-verifier:{tag}
          env:
            - name: ARMOR_BUCKET
              valueFrom:
                secretKeyRef:
                  name: armor-credentials
                  key: bucket
            - name: ARMOR_PREFIX
              valueFrom:
                configMapKeyRef:
                  name: armor-config
                  key: ARMOR_PREFIX
"""


def _standalone_verifier_with_service(tag):
    """The restore-verifier-acb pattern: a dedicated ExternalSecret-backed
    Secret, direct-to-B2, no co-located proxy. Multi-document: Deployment
    then Service — the finder must read the workload's metadata block, not
    the Service's."""
    return f"""\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: restore-verifier-acb
  namespace: armor
  labels:
    app: restore-verifier-acb
spec:
  template:
    spec:
      containers:
        - name: restore-verifier
          image: ronaldraygun/armor-restore-verifier:{tag}
---
apiVersion: v1
kind: Service
metadata:
  name: restore-verifier-acb
  namespace: armor
spec:
  ports:
    - name: metrics
      port: 9002
"""


_ARMOR_PROXY = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: armor
  namespace: armor
spec:
  template:
    spec:
      containers:
        - name: armor
          image: ronaldraygun/armor:0.1.1971@sha256:7c146c638026699fd67a8aa7f10c4a1398fe70045f3546ab37bb8979fbc14b1e
"""


# A `.disabled` monitoring manifest: the ServiceMonitor/PrometheusRule pair
# every cluster ships but no ARMOR cluster can apply (no prometheus-operator
# CRDs). Excluded by extension, not by content.
_DISABLED_MONITORING = """\
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: restore-verifier
  namespace: armor
"""


# An Argo WorkflowTemplate with a *pinned* image: only the finder's
# argo-workflows directory exclusion keeps this from classifying as a
# deployment.
_WORKFLOWTEMPLATE = """\
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: armor-restore-verifier-compat
  namespace: argo-workflows
spec:
  templates:
    - container:
        image: ronaldraygun/armor-restore-verifier:0.1.1971@sha256:15f4fd175a4041d595495801e207adbe183a5814ed9d7e37db0a2975604c2f
"""


# A manifest that mentions the image without an `image:` line (a comment or
# annotation): the finder warns and skips rather than classifying.
_EXTERNALSECRET = """\
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: armor-secrets
  namespace: armor
  annotations:
    # consumed by the ronaldraygun/armor deployment in this namespace
spec:
  target:
    name: armor-secrets
"""


# An unexpanded placeholder tag: never a pin, never an inventory entry.
_PLACEHOLDER_TAG = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: armor-test
  namespace: armor-test
spec:
  template:
    spec:
      containers:
        - name: armor
          image: ronaldraygun/armor:$IMAGE_TAG
"""


@pytest.fixture()
def declarative_config(tmp_path):
    dc = tmp_path / "declarative-config"
    files = {
        "k8s/iad-ci/armor/restore-verifier.yaml": _colocated_verifier(
            "restore-verifier",
            "0.1.1971@sha256:15f4fd175a4041d595495801e207adbe183a5814ed9d7e37db0a2975604c2f"),
        "k8s/iad-kalshi/armor/restore-verifier.yaml": _colocated_verifier(
            "restore-verifier",
            "0.1.1970@sha256:a6ffe666b95fcd51a5781e4548786bde23437685c5eac1630dfe9ed407dc1e71"),
        "k8s/ord-devimprint/devimprint/restore-verifier.yaml": _credentials_verifier(
            "0.1.1975@sha256:f15ca6ccccf08c29ab559d68cee00979ed8f991b8053f59372a1cca853526cb3"),
        "k8s/rs-manager/armor/restore-verifier-acb-deployment.yml":
            _standalone_verifier_with_service(
                "0.1.1970@sha256:a6ffe666b95fcd51a5781e4548786bde23437685c5eac1630dfe9ed407dc1e71"),
        "k8s/iad-ci/armor/armor-deployment.yaml": _ARMOR_PROXY,
        "k8s/iad-ci/armor/restore-verifier-monitoring.yaml.disabled": _DISABLED_MONITORING,
        "k8s/iad-ci/argo-workflows/armor-workflowtemplate.yml": _WORKFLOWTEMPLATE,
        "k8s/iad-ci/armor/armor-externalsecret.yaml": _EXTERNALSECRET,
        "k8s/iad-ci/armor-test/armor-test-deployment.yml": _PLACEHOLDER_TAG,
    }
    for relpath, content in files.items():
        path = dc / relpath
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
    return str(dc)


# ---------------------------------------------------------------------------
# the finder, against the fixtures
# ---------------------------------------------------------------------------

def _restore_verifiers(declarations):
    return sorted(
        (d for d in declarations if d["image_type"] == "armor-restore-verifier"),
        key=lambda d: (d["cluster"], d["namespace"], d["workload_name"]),
    )


def test_finder_enumerates_the_golden_inventory(declarative_config):
    found = _restore_verifiers(
        find_armor_deployments.find_armor_deployments(declarative_config))
    assert [(d["cluster"], d["namespace"], d["workload_name"]) for d in found] \
        == sorted(GOLDEN_RESTORE_VERIFIER_INVENTORY)
    assert all(d["kind"] == "Deployment" for d in found)


def test_finder_reads_the_workload_metadata_not_the_service(declarative_config):
    """The acb fixture is a Deployment+Service document pair with the same
    name; the record must carry the Deployment's kind and namespace."""
    found = _restore_verifiers(
        find_armor_deployments.find_armor_deployments(declarative_config))
    acb = [d for d in found if d["workload_name"] == "restore-verifier-acb"]
    assert len(acb) == 1
    assert acb[0]["kind"] == "Deployment"
    assert acb[0]["namespace"] == "armor"
    assert acb[0]["cluster"] == "rs-manager"


def test_finder_does_not_classify_non_deployments(declarative_config):
    found = find_armor_deployments.find_armor_deployments(declarative_config)
    for d in found:
        assert ".disabled" not in d["filepath"], d["filepath"]
        assert "argo-workflows" not in d["filepath"], d["filepath"]
        assert "armor-externalsecret" not in d["filepath"], d["filepath"]
        assert "armor-test" not in d["filepath"], d["filepath"]


def test_finder_still_sees_the_proxy_so_the_filter_is_meaningful(declarative_config):
    found = find_armor_deployments.find_armor_deployments(declarative_config)
    proxies = [d for d in found if d["image_type"] == "armor"]
    assert [(d["cluster"], d["namespace"], d["workload_name"]) for d in proxies] \
        == [("iad-ci", "armor", "armor")]


# ---------------------------------------------------------------------------
# the prose, against the golden inventory
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("surface", DOC_SURFACES,
                         ids=lambda p: str(p.relative_to(REPO_ROOT)))
def test_fleet_prose_lists_every_deployment(surface):
    text = surface.read_text()
    for cluster, namespace, workload in GOLDEN_RESTORE_VERIFIER_INVENTORY:
        assert f"{cluster}/{namespace}" in text, \
            f"{surface} does not mention {cluster}/{namespace}; {workload} " \
            f"is pinned in the golden inventory. {_HOWTO}"
    assert "restore-verifier-acb" in text, \
        f"{surface} does not name the standalone acb deployment. {_HOWTO}"


@pytest.mark.parametrize("surface", DOC_SURFACES,
                         ids=lambda p: str(p.relative_to(REPO_ROOT)))
def test_fleet_prose_states_the_fleet_size(surface):
    text = surface.read_text()
    pattern = rf"\b{FLEET_SIZE_WORD}\b[\s*]*restore-verifier"
    assert re.search(pattern, text, re.IGNORECASE), \
        f"{surface} states no '{FLEET_SIZE_WORD} ... restore-verifier' fleet " \
        f"size matching the golden inventory of " \
        f"{len(GOLDEN_RESTORE_VERIFIER_INVENTORY)}. {_HOWTO}"
