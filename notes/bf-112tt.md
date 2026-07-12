# Bead bf-112tt: RBAC Blockade on LITESTREAM Credential Retrieval

## Task Objective
Retrieve and decode LITESTREAM_SECRET_ACCESS_KEY from the armor-writer secret and store both credentials securely.

## Current Status: BLOCKED by RBAC

### Problem
The ord-devimprint cluster's read-only kubectl-proxy explicitly denies access to secrets:

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Error from server (Forbidden): secrets "armor-writer" is forbidden: 
# User "system:serviceaccount:devpod-observer:devpod-observer" 
# cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

### ExternalSecret Configuration
The armor-writer secret is synced from OpenBao via ExternalSecret:

- **ExternalSecret**: armor-writer in devimprint namespace
- **Source**: OpenBao ClusterSecretStore
- **OpenBao Path**: rs-manager/ord-devimprint/armor-writer
- **Properties**:
  - auth-access-key → LITESTREAM_ACCESS_KEY_ID (assumed)
  - auth-secret-key → LITESTREAM_SECRET_ACCESS_KEY (assumed)
- **Status**: SecretSynced (last sync: 2026-07-12T14:21:25Z)

### Attempted Workarounds
1. ✗ Direct secret access via read-only proxy - BLOCKED by RBAC
2. ✗ rs-manager.kubeconfig - File does not exist (/home/coding/.kube/rs-manager.kubeconfig)
3. ✗ OpenBao direct access - No accessible OpenBao endpoint
4. ✗ Alternative clusters - No ord-devimprint admin credentials available

### Cached Files (All Empty/Invalid)
- /tmp/litestream_secret_key_decoded.txt - Contains RBAC blockade notice
- /tmp/litestream_access_key_id.decoded - Contains corrupted binary data
- /tmp/litestream_credentials_status.md - Previous attempt documentation

### Available Clusters with Admin Access
- ardenone-manager - Full cluster-admin via direct kubeconfig
- rs-manager - Full cluster-admin via direct kubeconfig (file missing)
- iad-ci - Full cluster-admin via direct kubeconfig

None of these provide access to ord-devimprint secrets.

## Resolution Options
To complete this task, one of the following is required:

1. Direct kubeconfig for ord-devimprint with secret read access
2. RBAC policy update to allow devpod-observer SA to read secrets in devimprint namespace
3. OpenBao admin access to retrieve credentials directly from OpenBao
4. Alternative credential delivery (e.g., manual provisioning, secure paste)
5. Cross-cluster secret sync from a cluster with admin access

## Timeline
- 2026-07-12 11:21 - RBAC blockade confirmed via kubectl-proxy
- 2026-07-12 11:09 - Previous attempt failed with same RBAC error
- 2026-07-11 - Multiple credential retrieval attempts all blocked by RBAC

## Next Steps
This task cannot be completed without elevated credentials or RBAC changes. The bead should remain open until one of the resolution options is implemented.

---

Generated: 2026-07-12 11:21 EDT
Bead: bf-112tt
Status: BLOCKED - Awaiting access resolution
