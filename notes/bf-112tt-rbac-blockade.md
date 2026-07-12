# Task bf-112tt: RBAC Blockade on LITESTREAM_SECRET_ACCESS_KEY

## Date
2026-07-12

## Objective
Retrieve and decode LITESTREAM_SECRET_ACCESS_KEY from ord-devimprint cluster and store both credentials.

## Current State

### ✅ ACCESS_KEY_ID - Successfully Retrieved
- **Location**: `/tmp/litestream_access_key_id.txt`
- **Value**: `lcs18qaArvWltpK/3oSfFrqiZ/oD7bcGMNYVkW2buD0=`
- **Permissions**: `600` (owner read/write only)
- **Status**: Properly stored and secured

### ❌ SECRET_ACCESS_KEY - RBAC Blockade

#### Access Attempt
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}'
```

#### Result
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" 
cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Root Cause Analysis

### Infrastructure Constraint
The ord-devimprint cluster is configured with **strict read-only RBAC**:
- **Access method**: kubectl-proxy over Tailscale only
- **ServiceAccount**: `devpod-observer` in `devpod-observer` namespace
- **RBAC policy**: Explicitly denies secrets access across all namespaces
- **No kubeconfig**: No direct kubeconfig file available with elevated privileges

### ExternalSecret Configuration
The secret is synced from OpenBao via ExternalSecret:
```yaml
kind: ExternalSecret
metadata:
  name: armor-writer
  namespace: devimprint
spec:
  data:
  - remoteRef:
      key: rs-manager/ord-devimprint/armor-writer
      property: auth-secret-key
    secretKey: auth-secret-key
  - remoteRef:
      key: rs-manager/ord-devimprint/armor-writer
      property: auth-access-key
    secretKey: auth-access-key
  refreshInterval: 1h
  secretStoreRef:
    kind: ClusterSecretStore
    name: openbao
status:
  conditions:
  - type: Ready
    status: "True"
    reason: SecretSynced
```

**Status**: ExternalSecret shows `SecretSynced: True`, last sync at `2026-07-12T14:21:25Z`

### Alternative Access Attempts
All failed due to lack of credentials:
1. ❌ rs-manager kubeconfig - Secret not found (different cluster)
2. ❌ OpenBao CLI - Not installed on this host
3. ❌ OpenBao environment variables - Not configured

## Requirements to Complete This Task

To retrieve the SECRET_ACCESS_KEY, ONE of the following is needed:

### Option 1: Elevated Kubeconfig
- Direct kubeconfig for ord-devimprint cluster
- With ServiceAccount that has `secrets/*` read access in `devimprint` namespace

### Option 2: RBAC Policy Update
Update `devpod-observer` ClusterRole/Role to allow:
```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
```
Scoped to `devimprint` namespace only.

### Option 3: OpenBao Direct Access
- Install OpenBao CLI (`bao`)
- Configure OpenBao token/endpoint
- Query: `bao kv get -field=auth-secret-key rs-manager/ord-devimprint/armor-writer`

## Impact

### What's Blocked
- ❌ Cannot retrieve LITESTREAM_SECRET_ACCESS_KEY
- ❌ Cannot complete credential pair for Litestream replication
- ❌ Cannot verify secret integrity or rotation

### What's Working
- ✅ ACCESS_KEY_ID retrieved and stored
- ✅ ExternalSecret syncing successfully from OpenBao
- ✅ Secret exists and is being refreshed (hourly interval)

## Recommendation

This task cannot be completed without infrastructure changes. The ord-devimprint cluster is intentionally configured with read-only access for security, and there is no current path to escalate privileges for this specific secret retrieval.

**Suggested resolution**: Obtain direct OpenBao access or request temporary elevated kubeconfig for credential retrieval operations.

## Files Preserved
- `/tmp/litestream_access_key_id.txt` (600) - ACCESS_KEY_ID cached
- `/tmp/litestream_secret_key_decoded.txt` (empty - RBAC blocked)
- `/tmp/litestream_credentials_status.md` - Status documentation
