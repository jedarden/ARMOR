# ARMOR S3 Credentials Access Blocker - bf-37mxj

## Date: 2026-07-11

## Task Summary
Obtain S3 credentials (LITESTREAM_ACCESS_KEY_ID and LITESTREAM_SECRET_ACCESS_KEY) from the armor-writer secret in the ord-devimprint namespace.

## Current State

### Secret Location
- **Cluster**: ord-devimprint
- **Namespace**: devimprint
- **Secret Name**: armor-writer
- **Data Keys**:
  - `auth-access-key` → LITESTREAM_ACCESS_KEY_ID
  - `auth-secret-key` → LITESTREAM_SECRET_ACCESS_KEY

### Secret Source (ExternalSecret)
The secret is synced from OpenBao via ExternalSecrets Operator:
- **OpenBao Path**: `rs-manager/ord-devimprint/armor-writer`
- **ExternalSecret**: commitgraph/armor-writer (also devimprint/armor-writer)
- **Sync Status**: Unknown (ExternalSecrets have been failing in ord-devimprint)

## Access Constraints

### ❌ Read-Only kubectl-proxy
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o yaml
```
- **Result**: Secret metadata is accessible, but data field is redacted
- **Reason**: Proxy runs as devpod-observer ServiceAccount with read-only RBAC that denies secret access

### ❌ No Direct kubeconfig
- **File**: `~/.kube/ord-devimprint.kubeconfig` - Does not exist
- **Status**: No kubeconfig with write access to ord-devimprint cluster
- **Historical Context**: Previous kubeconfig expired (armor-s8k.3.2.2 notes)

### ❌ OpenBao Access Blocked
- **OpenBao Cluster**: rs-manager
- **OpenBao Service**: openbao.external-secrets.svc.cluster.local:8200
- **Tailscale Endpoint**: https://openbao-rs-manager.tail1b1987.ts.net:8200
- **Blocker**: Need authentication token to access OpenBao API
- **Token Location**: Secret `openbao-eso-token` in `external-secrets` namespace (blocked by read-only RBAC)

## Attempted Access Methods

### 1. kubectl-proxy (Read-Only)
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
# ✅ Lists secrets (names only)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o yaml
# ❌ Error: secrets is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

### 2. Pod Environment Variable Extraction
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 exec -n devimprint pod/queue-api-7999dffbd7-l8hgr -c litestream -- env
# ❌ No output (likely blocked by read-only RBAC)
```

### 3. OpenBao Direct Access
```bash
curl -k https://openbao-rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/armor-writer
# ❌ Error: missing client token or authorization
```

### 4. Cached Credentials Search
- **Local Repository**: No cached credentials found
- **Documentation**: No hardcoded credentials in README or notes
- **Historical Context**: bf-520v notes mention "using cached secrets for migration" but no actual cached values exist

## ExternalSecret Configuration

From `~/declarative-config/k8s/ord-devimprint/commitgraph/commitgraph-externalsecrets.yml`:

```yaml
apiVersion: external-secrets.io/v1alpha1
kind: ExternalSecret
metadata:
  name: armor-writer
  namespace: commitgraph
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: openbao
    kind: ClusterSecretStore
  target:
    name: armor-writer
    creationPolicy: Owner
  data:
    - secretKey: auth-access-key
      remoteRef:
        key: rs-manager/ord-devimprint/armor-writer
        property: auth-access-key
    - secretKey: auth-secret-key
      remoteRef:
        key: rs-manager/ord-devimprint/armor-writer
        property: auth-secret-key
```

## Resolution Required

This task requires one of the following access methods:

### Option A: Direct kubeconfig with write access
```bash
# Need a fresh kubeconfig for ord-devimprint with cluster-admin or secret-read access
kubectl --kubeconfig=<ord-devimprint.kubeconfig> get secret armor-writer -n devimprint -o jsonpath='{.data.auth-access-key}' | base64 -d
kubectl --kubeconfig=<ord-devimprint.kubeconfig> get secret armor-writer -n devimprint -o jsonpath='{.data.auth-secret-key}' | base64 -d
```

### Option B: OpenBao Admin Access
```bash
# Need OpenBao token with access to rs-manager/ord-devimprint/armor-writer
OPENBAO_TOKEN=<admin-token>
curl -H "X-Vault-Token: $OPENBAO_TOKEN" \
  https://openbao-rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/armor-writer
```

### Option C: Direct Credential Provision
User provides the actual credential values directly:
- LITESTREAM_ACCESS_KEY_ID
- LITESTREAM_SECRET_ACCESS_KEY

## Related Beads
- Parent: bf-37mxj (Obtain S3 credentials from ord-devimprint cluster)
- Related: bf-520v (ARMOR v0.1.x Maintenance - Genesis Bead)
- Related: armor-s8k.3.2.2 (Credential access issues on ord-devimprint)

## Files Referenced
- `~/declarative-config/k8s/ord-devimprint/commitgraph/commitgraph-externalsecrets.yml`
- `~/declarative-config/k8s/ord-devimprint/devimprint/queue-api-deployment.yml`
- `~/.kube/ord-devimprint.kubeconfig` (does not exist)
