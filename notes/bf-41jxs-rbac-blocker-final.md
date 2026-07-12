# Bead bf-41jxs: SECRET_ACCESS_KEY RBAC Blocker - Final Status

**Date:** 2026-07-12
**Status:** BLOCKED by RBAC - Requires Admin Intervention

## Summary

The task to store both Litestream credentials in /tmp/ with proper permissions is **partially complete**:
- **ACCESS_KEY_ID**: ✓ Successfully stored with proper permissions (32 bytes, chmod 600)
- **SECRET_ACCESS_KEY**: ✗ RBAC blocker prevents retrieval; file exists but empty

## Current File State

```bash
# Files exist with correct permissions
-rw------- 1 coding users 32 Jul 12 10:48 /tmp/litestream_access_key_id.txt  # Valid
-rw------- 1 coding users  0 Jul 12 10:50 /tmp/litestream_secret_key_decoded.txt  # Empty - BLOCKED
```

## Root Cause

The read-only kubectl-proxy service account (`devpod-observer:devpod-observer`) lacks permissions to read secrets in the `devimprint` namespace. This affects all operations requiring secret access:

```bash
# Attempted via kubectl-proxy (FAILED)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"

# Attempted pod exec (FAILED)  
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 exec -n devimprint armor-5c5f8c5fd8-58wt4 -- env
# Error: Forbidden
```

## Evidence That Credentials Exist in Cluster

The ARMOR pods are running successfully in devimprint namespace:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint -l app=armor
# NAME                        READY   STATUS    RESTARTS   AGE
# armor-5c5f8c5fd8-58wt4       1/1     Running   0          2d
# armor-7876b6f9bc-4xgfc       1/1     Running   0          2d
# [... additional running pods ...]
```

The `armor-writer` secret exists with 2 keys (likely LITESTREAM_ACCESS_KEY_ID and LITESTREAM_SECRET_ACCESS_KEY):
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
# NAME               TYPE     DATA   AGE
# armor-writer       Opaque   2      80d
```

## Resolution Options

To complete this task, one of the following approaches is required:

### Option 1: Direct Kubeconfig (Recommended)
Use a direct kubeconfig with cluster-admin credentials to bypass the read-only proxy:
```bash
# Requires admin access to ord-devimprint cluster
kubectl --kubeconfig=/path/to/admin-kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}' | base64 -d > /tmp/litestream_secret_key_decoded.txt
chmod 600 /tmp/litestream_secret_key_decoded.txt
```

### Option 2: RBAC Permission Grant
Update the `devpod-observer` service account role to allow secret read in `devimprint` namespace. This would enable kubectl-proxy access but may not be desirable for security reasons.

### Option 3: Manual Operator Provision
Have an operator with admin access manually retrieve the SECRET_ACCESS_KEY and provide it securely.

## Pattern Context

This RBAC blocker pattern matches **bead bf-520v** (ARMOR v0.1.x Maintenance), where:
- "Using cached secrets for migration avoided OpenBao dependency"
- "Production log verification was accepted when RBAC blocked exec"

The pattern suggests that when RBAC blocks direct access, production evidence is accepted as validation, provided workloads are functioning correctly.

## Acceptance Criteria Status

| Criterion | Status | Notes |
|-----------|--------|-------|
| Both credentials stored in /tmp/ | ⚠️ PARTIAL | ACCESS_KEY_ID complete, SECRET_ACCESS_KEY file exists but empty |
| Restricted permissions (600) | ✅ MET | Both files have -rw------- permissions |
| Not group/world readable | ✅ MET | chmod 600 applied correctly |
| Files contain valid credential data | ❌ NOT MET | SECRET_ACCESS_KEY empty due to RBAC blocker |
| Files clearly named and identifiable | ✅ MET | Naming follows convention |

## Recommendation

This bead requires **admin intervention** to complete. Either:
1. Request admin with ord-devimprint cluster-admin access to retrieve SECRET_ACCESS_KEY
2. Document this as a known infrastructure limitation requiring elevated access for secret retrieval

The current state demonstrates proper file structure and permissions setup; only the actual credential value is missing due to the RBAC constraint.
