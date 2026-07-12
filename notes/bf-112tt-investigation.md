# BF-112TT Investigation Report

## Task
Retrieve and decode LITESTREAM_SECRET_ACCESS_KEY and store both credentials

## Investigation Summary (2026-07-12)

### Current Access Limitations
The ord-devimprint cluster has **only read-only kubectl-proxy access** which explicitly blocks secret retrieval:

```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer"
cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

### Infrastructure Investigation

#### ord-devimprint Cluster
- **Access method**: Read-only kubectl-proxy only (`kubectl-proxy-ord-devimprint:8001`)
- **Kubeconfig status**: No direct kubeconfig file exists with elevated permissions
- **Secret access**: BLOCKED by RBAC policy
- **Namespace**: devimprint exists and contains armor-writer secret

#### Other Clusters Checked
- **ardenone-manager**: Has direct kubeconfig but NO devimprint namespace
- **rs-manager**: Has direct kubeconfig but NO devimprint namespace  
- **iad-ci**: Has direct kubeconfig but NO devimprint namespace
- **iap-options**: Has read/write kubeconfig but NO devimprint namespace

### ExternalSecret Configuration
The `armor-writer` secret is synced from OpenBao:
- **ExternalSecret**: armor-writer in devimprint namespace
- **Store**: ClusterSecretStore/openbao
- **OpenBao path**: `rs-manager/ord-devimprint/armor-writer`
- **Properties**:
  - `auth-access-key` → secretKey: `auth-access-key`
  - `auth-secret-key` → secretKey: `auth-secret-key`

### OpenBao Access
- **CLI tools**: NOT installed on this server
- **Direct access**: No available method to query OpenBao directly

### Cached Files Analysis
Multiple credential files exist but contain RBAC error messages:
- `/tmp/litestream_secret_key_encoded.b64` - Contains RBAC error (not actual key)
- `/tmp/litestream_secret_key_decoded.txt` - Contains verification message (empty credential)
- `/tmp/litestream_access_key_id.txt` - Contains ACCESS_KEY_ID (retrieved previously)

### Previous Attempts
All attempts to retrieve SECRET_ACCESS_KEY have failed with identical RBAC errors:
- Jul 12, 2026 10:56 - RBAC blockade
- Jul 12, 2026 11:09 - Documentation update
- Multiple earlier attempts (documented in git commits)

## Root Cause
The ord-devimprint cluster's read-only proxy architecture **explicitly denies secret access** as a security measure. The devpod-observer ServiceAccount has restrictive RBAC that prevents secret reading, and no alternative access path exists from this server.

## Required for Task Completion
Task bf-112tt cannot be completed without ONE of the following:

1. **Direct kubeconfig** with secret read access to ord-devimprint cluster
2. **RBAC policy update** to allow devpod-observer SA to read secrets in devimprint namespace
3. **OpenBao CLI access** to retrieve credentials directly from the source
4. **Alternative secret access method** (different cluster, external service, etc.)

## Recommendation
**LEAVE BEAD OPEN** - This task requires infrastructure access beyond what is currently available on this server. The RBAC blockade is a security feature that cannot be bypassed without elevated privileges or policy changes.

**Bead-ID**: bf-112tt
**Investigation Date**: 2026-07-12
**Status**: BLOCKED - Requires escalation or infrastructure access
**Next Steps**: Request ord-devimprint kubeconfig with secret read access or OpenBao credentials
