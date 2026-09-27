#!/usr/bin/env python3
"""
Scope-contract tests for scripts/validate-restore-verifier-scopes.py — the
manifest-level ADR-004 restore-verifier contract (one Deployment per bucket
+ MEK + B2 credential scope, never straddling, exactly one bucket and MEK
each by reference, every proxy-declared scope covered).

The fixtures are synthetic declarative-config trees modeled on the live
fleet's three verifier shapes (co-located ConfigMap+Secret references,
all-in-one credentials Secret, standalone ExternalSecret-backed Deployment
plus its Service) — same approach as tests/test_restore_verifier_inventory.py,
which pins the finder this validator delegates discovery to. Nothing here
reads ~/declarative-config: the live tree legitimately carries uncovered
proxy scopes, so validating it is an operator run, not a test assertion.

Run: python3 -m pytest tests/test_restore_verifier_scope_validation.py -q
"""

import importlib.util
import json
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[1]
_SPEC = importlib.util.spec_from_file_location(
    "validate_restore_verifier_scopes",
    REPO_ROOT / "scripts" / "validate-restore-verifier-scopes.py")
validator = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(validator)

VERIFIER_IMAGE = ("ronaldraygun/armor-restore-verifier:"
                  "0.1.1975@sha256:f15ca6ccccf08c29ab559d68cee00979ed8f991b8053f"
                  "59372a1cca853526cb3")
PROXY_IMAGE = ("ronaldraygun/armor:"
               "0.1.1975@sha256:85a73c8f0e3546ab37bb8979fbc14b1e7c146c638026699"
               "fd67a8aa7f10c4a139")


# ---------------------------------------------------------------------------
# fixture builders — dict-shaped manifests, rendered with safe_dump so the
# validator consumes real YAML
# ---------------------------------------------------------------------------

def _ref(name, configmap=None, secret=None, key=None):
    source = {"secretKeyRef": {"name": secret, "key": key}} if secret else \
        {"configMapKeyRef": {"name": configmap, "key": key}}
    return {"name": name, "valueFrom": source}


def _literal(name, value):
    return {"name": name, "value": value}


def _field_ref(name, field):
    return {"name": name, "valueFrom": {"fieldRef": {"fieldPath": field}}}


def _container(env=None, env_from=None, name="restore-verifier",
               image=VERIFIER_IMAGE):
    container = {"name": name, "image": image}
    if env is not None:
        container["env"] = env
    if env_from:
        container["envFrom"] = env_from
    return container


def _deployment(name, namespace, containers, init_containers=(),
                kind="Deployment"):
    pod = {"containers": containers}
    if init_containers:
        pod["initContainers"] = list(init_containers)
    return {
        "apiVersion": "apps/v1",
        "kind": kind,
        "metadata": {"name": name, "namespace": namespace},
        "spec": {"template": {"spec": pod}},
    }


def _service(name, namespace):
    return {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {"name": name, "namespace": namespace},
        "spec": {"ports": [{"name": "metrics", "port": 9002}]},
    }


# The co-located pattern (iad-ci/armor, iad-kalshi/armor): env by reference
# from the proxy's ConfigMap + Secret sources, retired MEK ring included
# where rotation has run.
COLOCATED_ENV = [
    _ref("ARMOR_BUCKET", configmap="armor-config", key="ARMOR_BUCKET"),
    _ref("ARMOR_MEK", secret="armor-secrets", key="master-encryption-key"),
    _ref("ARMOR_B2_ACCESS_KEY_ID", secret="armor-secrets",
         key="b2-access-key-id"),
    _ref("ARMOR_B2_SECRET_ACCESS_KEY", secret="armor-secrets",
         key="b2-secret-access-key"),
    _ref("VERIFIER_MEK_RING", secret="armor-secrets", key="mek-ring"),
]

# The credentials pattern (ord-devimprint/devimprint): everything in one
# scope-owned Secret.
CREDENTIALS_ENV = [
    _ref("ARMOR_BUCKET", secret="armor-credentials", key="bucket"),
    _ref("ARMOR_MEK", secret="armor-credentials", key="master-encryption-key"),
    _ref("ARMOR_B2_ACCESS_KEY_ID", secret="armor-credentials",
         key="b2-access-key-id"),
    _ref("ARMOR_B2_SECRET_ACCESS_KEY", secret="armor-credentials",
         key="b2-secret-access-key"),
]

# The standalone pattern (rs-manager/armor restore-verifier-acb): dedicated
# ExternalSecret-backed Secret, Deployment plus Service in one file.
STANDALONE_ENV = [
    _ref("ARMOR_BUCKET", secret="restore-verifier-acb-b2-credentials",
         key="bucket"),
    _ref("ARMOR_MEK", secret="restore-verifier-acb-b2-credentials",
         key="master-encryption-key"),
    _ref("ARMOR_B2_ACCESS_KEY_ID", secret="restore-verifier-acb-b2-credentials",
         key="b2-access-key-id"),
    _ref("ARMOR_B2_SECRET_ACCESS_KEY",
         secret="restore-verifier-acb-b2-credentials",
         key="b2-secret-access-key"),
]


def _write_tree(tmp_path, files):
    """files: {relative path: [manifest dicts]} → path to the checkout."""
    dc = tmp_path / "declarative-config"
    for relpath, manifests in files.items():
        path = dc / relpath
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("---\n".join(yaml.safe_dump(m) for m in manifests))
    return str(dc)


def _standard_fleet(tmp_path, mutate=None):
    """The live fleet's shape: three proxy-covered scopes, the standalone acb
    scope with no co-located proxy, and one proxy-declared scope with no
    verifier (the legitimate per-scope decision the contract allows).
    `mutate(files)` may rewrite the dict before it lands."""
    files = {
        "k8s/iad-ci/armor/restore-verifier.yaml": [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV)]),
        ],
        "k8s/iad-kalshi/armor/restore-verifier.yaml": [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV)]),
        ],
        "k8s/ord-devimprint/devimprint/restore-verifier.yaml": [
            _deployment("restore-verifier", "devimprint",
                        [_container(env=CREDENTIALS_ENV)]),
        ],
        "k8s/rs-manager/armor/restore-verifier-acb-deployment.yml": [
            _deployment("restore-verifier-acb", "armor",
                        [_container(env=STANDALONE_ENV)]),
            _service("restore-verifier-acb", "armor"),
        ],
        "k8s/iad-ci/armor/armor-deployment.yaml": [
            _deployment("armor", "armor",
                        [_container(env=[_ref("ARMOR_BUCKET",
                                              configmap="armor-config",
                                              key="ARMOR_BUCKET")],
                                   image=PROXY_IMAGE)]),
        ],
        "k8s/iad-kalshi/armor/armor-deployment.yml": [
            _deployment("armor", "armor",
                        [_container(env=[_ref("ARMOR_BUCKET",
                                              configmap="armor-config",
                                              key="ARMOR_BUCKET")],
                                   image=PROXY_IMAGE)]),
        ],
        "k8s/ord-devimprint/devimprint/armor-deployment.yml": [
            _deployment("armor", "devimprint",
                        [_container(env=[_ref("ARMOR_BUCKET",
                                              secret="armor-credentials",
                                              key="bucket")],
                                   image=PROXY_IMAGE)]),
        ],
        # A scope declared but deliberately not verifier-covered.
        "k8s/ardenone-cluster/tradegraph-platform/armor-deployment.yaml": [
            _deployment("armor", "tradegraph-platform",
                        [_container(env=[_ref("ARMOR_BUCKET",
                                              secret="armor-secrets",
                                              key="bucket")],
                                   image=PROXY_IMAGE)]),
        ],
    }
    if mutate:
        mutate(files)
    return _write_tree(tmp_path, files)


def _codes(report, severity=None):
    return [f["code"] for f in report["findings"]
            if severity is None or f["severity"] == severity]


# ---------------------------------------------------------------------------
# the valid fleet
# ---------------------------------------------------------------------------

def test_valid_fleet_passes_with_uncovered_warnings(tmp_path):
    dc = _standard_fleet(tmp_path)
    report = validator.validate(dc)
    assert _codes(report, "error") == []
    assert _codes(report, "warning") == ["uncovered-scope"]
    assert validator.derive_exit(report) == validator.EXIT_OK
    assert [(v["cluster"], v["namespace"], v["workload"])
            for v in report["verifiers"]] == [
        ("iad-ci", "armor", "restore-verifier"),
        ("iad-kalshi", "armor", "restore-verifier"),
        ("ord-devimprint", "devimprint", "restore-verifier"),
        ("rs-manager", "armor", "restore-verifier-acb"),
    ]


def test_coverage_maps_proxy_scopes_to_verifiers(tmp_path):
    dc = _standard_fleet(tmp_path)
    report = validator.validate(dc)
    covered = {(s["cluster"], s["namespace"]): s["covered_by"]
               for s in report["scopes"] if s["covered_by"]}
    assert covered == {
        ("iad-ci", "armor"): "restore-verifier",
        ("iad-kalshi", "armor"): "restore-verifier",
        ("ord-devimprint", "devimprint"): "restore-verifier",
    }
    uncovered = [(s["cluster"], s["namespace"], s["declared_by"])
                 for s in report["scopes"] if not s["covered_by"]]
    assert uncovered == [("ardenone-cluster", "tradegraph-platform", "armor")]


def test_scope_identity_is_namespace_qualified(tmp_path):
    """Same cluster, same ConfigMap source, different namespace: two distinct
    scopes — one verifier each, no duplicate finding."""
    def mutate(files):
        files["k8s/iad-ci/armor-two/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor-two",
                        [_container(env=COLOCATED_ENV)]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == []
    assert len(report["verifiers"]) == 5


# ---------------------------------------------------------------------------
# the exactly-one contract
# ---------------------------------------------------------------------------

def test_missing_bucket_env_is_an_error(tmp_path):
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=[e for e in COLOCATED_ENV
                                         if e["name"] != "ARMOR_BUCKET"])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-missing-armor-bucket"]


def test_missing_mek_env_is_an_error(tmp_path):
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=[e for e in COLOCATED_ENV
                                         if e["name"] != "ARMOR_MEK"])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-missing-armor-mek"]


def test_second_bucket_in_an_init_container_straddles(tmp_path):
    """The exactly-one counts are pod-wide: a second ARMOR_BUCKET in an init
    container is a scope straddle even though the main container is clean."""
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV)],
                        init_containers=[_container(
                            name="preflight",
                            env=[_ref("ARMOR_BUCKET",
                                      secret="other-credentials",
                                      key="bucket")])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-straddles-armor-buckets"]


def test_second_mek_straddles(tmp_path):
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV + [
                            _ref("ARMOR_MEK", secret="armor-secrets",
                                 key="retired-mek")])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-straddles-armor-meks"]


def test_bucket_aliases_are_not_a_second_bucket(tmp_path):
    """ARMOR_BUCKET_ALIASES are alternate names for the same bucket
    (ADR-004 Addendum) — allowed alongside the one ARMOR_BUCKET, literal
    spellings included: an alias list is not a credential."""
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV + [
                            _literal("ARMOR_BUCKET_ALIASES",
                                     "iad-ci-archive,iad-ci-old")])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    assert _codes(validator.validate(dc), "error") == []


def test_secondary_b2_replication_keys_straddle(tmp_path):
    """Replication's second bucket + second credential set are a second
    scope: rejected on a verifier even as references."""
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV + [
                            _ref("ARMOR_SECONDARY_B2_BUCKET",
                                 secret="armor-secrets",
                                 key="secondary-bucket"),
                            _ref("ARMOR_SECONDARY_B2_KEY_ID",
                                 secret="armor-secrets",
                                 key="secondary-key-id")])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-straddles-buckets"]


# ---------------------------------------------------------------------------
# secrets by reference
# ---------------------------------------------------------------------------

def test_literal_mek_is_rejected(tmp_path):
    def mutate(files):
        env = [e for e in COLOCATED_ENV if e["name"] != "ARMOR_MEK"]
        env.append(_literal("ARMOR_MEK", "a" * 64))
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor", [_container(env=env)]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-mek-not-reference"]


def test_fieldref_bucket_is_not_a_reference(tmp_path):
    """fieldRef and friends cannot prove scope sameness, so they cannot
    carry the bucket."""
    def mutate(files):
        env = [e for e in COLOCATED_ENV if e["name"] != "ARMOR_BUCKET"]
        env.append(_field_ref("ARMOR_BUCKET", "metadata.name"))
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor", [_container(env=env)]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-bucket-not-reference"]


def test_literal_b2_key_is_rejected(tmp_path):
    def mutate(files):
        env = [e for e in COLOCATED_ENV
               if e["name"] != "ARMOR_B2_ACCESS_KEY_ID"]
        env.append(_literal("ARMOR_B2_ACCESS_KEY_ID", "0011223344556677"))
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor", [_container(env=env)]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["literal-credential"]


def test_literal_mek_ring_is_rejected(tmp_path):
    """The retired ring is allowed where rotation has run — but only by
    reference; a ring key literal is a credential on the manifest."""
    def mutate(files):
        env = [e for e in COLOCATED_ENV if e["name"] != "VERIFIER_MEK_RING"]
        env.append(_literal("VERIFIER_MEK_RING", "b" * 64))
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor", [_container(env=env)]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["literal-credential"]


def test_envfrom_hides_the_contract(tmp_path):
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier.yaml"] = [
            _deployment("restore-verifier", "armor",
                        [_container(env=COLOCATED_ENV,
                                    env_from=[{"configMapRef":
                                               {"name": "armor-config"}}])]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["verifier-envfrom"]


# ---------------------------------------------------------------------------
# one Deployment per scope
# ---------------------------------------------------------------------------

def test_duplicate_scope_identity_is_an_error(tmp_path):
    """Two verifier Deployments in the same cluster and namespace resolving
    to the same scope identity double-cover one scope."""
    def mutate(files):
        files["k8s/iad-ci/armor/restore-verifier-2.yaml"] = [
            _deployment("restore-verifier-2", "armor",
                        [_container(env=COLOCATED_ENV)]),
        ]
    dc = _standard_fleet(tmp_path, mutate)
    report = validator.validate(dc)
    assert _codes(report, "error") == ["duplicate-scope-identity"]


def test_deployment_and_service_document_pair_selects_the_workload(tmp_path):
    """The standalone shape is a Deployment plus its Service in one file,
    same name: the Service must not be read as the workload (and its
    absence of env must not zero the scope out)."""
    dc = _standard_fleet(tmp_path)
    report = validator.validate(dc)
    acb = [v for v in report["verifiers"]
           if v["workload"] == "restore-verifier-acb"]
    assert len(acb) == 1
    assert acb[0]["bucket_source"] == \
        "secret/restore-verifier-acb-b2-credentials/bucket"
    assert acb[0]["mek_source"] == \
        "secret/restore-verifier-acb-b2-credentials/master-encryption-key"


# ---------------------------------------------------------------------------
# coverage
# ---------------------------------------------------------------------------

def test_uncovered_scope_warns_by_default_and_errors_strict(tmp_path):
    dc = _standard_fleet(tmp_path)
    report = validator.validate(dc)
    assert _codes(report) == ["uncovered-scope"]
    assert validator.derive_exit(report) == validator.EXIT_OK

    strict = validator.validate(dc, strict_coverage=True)
    assert _codes(strict) == ["uncovered-scope"]
    assert strict["findings"][0]["severity"] == "error"
    assert validator.derive_exit(strict) == validator.EXIT_ERRORS


def test_strict_flag_drives_the_exit_code(tmp_path):
    dc = _standard_fleet(tmp_path)
    assert validator.main([dc]) == validator.EXIT_OK
    assert validator.main([dc, "--strict-coverage"]) == validator.EXIT_ERRORS


# ---------------------------------------------------------------------------
# structural break
# ---------------------------------------------------------------------------

def test_no_k8s_tree_is_structural(tmp_path):
    dc = tmp_path / "declarative-config"
    dc.mkdir()
    assert validator.main([str(dc)]) == validator.EXIT_STRUCTURAL


def test_no_verifier_deployments_is_structural(tmp_path):
    """A tree with proxy Deployments but no restore-verifier cannot be
    validated against the scope contract at all."""
    dc = _write_tree(tmp_path, {
        "k8s/iad-ci/armor/armor-deployment.yaml": [
            _deployment("armor", "armor",
                        [_container(env=[_ref("ARMOR_BUCKET",
                                              configmap="armor-config",
                                              key="ARMOR_BUCKET")],
                                   image=PROXY_IMAGE)]),
        ],
    })
    assert validator.main([dc]) == validator.EXIT_STRUCTURAL


# ---------------------------------------------------------------------------
# the machine-readable report
# ---------------------------------------------------------------------------

def test_json_report_shape(tmp_path, capsys):
    dc = _standard_fleet(tmp_path)
    assert validator.main([dc, "--json"]) == validator.EXIT_OK
    report = json.loads(capsys.readouterr().out)
    assert set(report) == {"declarative_config", "strict_coverage",
                           "verifiers", "scopes", "findings"}
    assert report["strict_coverage"] is False
    for verifier in report["verifiers"]:
        assert verifier["bucket_source"].split("/", 1)[0] in ("secret",
                                                              "configmap")
        assert verifier["mek_source"].split("/", 1)[0] in ("secret",
                                                           "configmap")
        assert len(verifier["identity"]) == 5
    for finding in report["findings"]:
        assert finding["severity"] in ("error", "warning")
        assert finding["code"] and finding["message"]
