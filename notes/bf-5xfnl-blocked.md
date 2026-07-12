# Task Blocked: bf-5xfnl - Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID

## Status: BLOCKED - Infrastructure Limitation

## Issue
The ord-devimprint cluster has **only a read-only kubectl-proxy** with no available read/write kubeconfig. The observer explicitly denies access to secrets.

## Cluster Access Reality (ord-devimprint)
- **Available access only:** `kubectl --server=http://kubectl-proxy-ord-devimprint:8001` (read-only proxy)
- **RBAC constraint:** Observer serviceaccount cannot read secrets
- **No read/write kubeconfig:** `/home/coding/.kube/ord-devimprint.kubeconfig` does not exist
- **No alternative kubeconfigs:** Only `iad-acb.kubeconfig` and `iad-ci.kubeconfig` exist on system

## What Was Attempted
```bash
# Attempt 1 (original field name from task description)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'

# Attempt 2 (corrected field name after checking deployment files)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}'
```

Error received (same for both attempts):
```
Error from server (Forbidden): secrets "armor-writer" is forbidden
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Acceptance Criteria Status
- ❌ Successfully retrieved the base64-encoded value - BLOCKED by RBAC
- ❌ Value is not empty - CANNOT VERIFY
- ❌ Value appears to be valid base64 - CANNOT VERIFY

## Why Prerequisite Beads "Completed"
The prerequisite chain (bf-58r06 → bf-2c1jp → bf-2txcw) apparently completed despite no functional secret access being available. Possible explanations:
1. Beads used a different access method that no longer exists (expired kubeconfig)
2. Beads were marked complete without actually verifying secret access
3. There was a temporary workaround that's no longer available

## Resolution Path
To complete this task, one of the following must be provided:
1. **A read/write kubeconfig** for ord-devimprint cluster (doesn't currently exist)
2. **RBAC modification** to allow the observer serviceaccount to read secrets (security risk)
3. **Alternative access method** to retrieve the secret value

## Context
- Task: Retrieve LITESTREAM_ACCESS_KEY_ID from armor-writer secret
- Namespace: devimprint
- Cluster: ord-devimprint (Rackspace Spot cluster)
- Secret: armor-writer
- Field: auth-access-key (mapped to LITESTREAM_ACCESS_KEY_ID env var in deployments)
- Source: ExternalSecret synced from OpenBao path rs-manager/ord-devimprint/armor-writer

## Field Mapping Discovery (2026-07-11)
The deployment files show that `LITESTREAM_ACCESS_KEY_ID` environment variable maps to the `auth-access-key` field in the `armor-writer` secret:
```yaml
- name: LITESTREAM_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: armor-writer
      key: auth-access-key
```

So the correct kubectl command would be:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}'
```

However, this still fails with the same RBAC error (cannot get secrets).

## Bead: bf-5xfnl
Status: BLOCKED - Cannot close without access to secret
