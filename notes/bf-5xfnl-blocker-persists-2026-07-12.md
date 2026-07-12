# Bead bf-5xfnl: Infrastructure Blocker Persists

## Task
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in ord-devimprint cluster.

## Current Status: BLOCKED

### Infrastructure Blocker Details
1. **Read-only proxy RBAC restriction**: The kubectl-proxy for ord-devimprint (http://kubectl-proxy-ord-devimprint:8001) runs with ServiceAccount `devpod-observer` which explicitly lacks secrets read permissions.

2. **No write-access kubeconfig**: No kubeconfig file exists at ~/.kube/ord-devimprint.kubeconfig (verified 2026-07-12 00:27 UTC).

3. **Previous dependency chain incorrectly closed**: Beads bf-2p1wr (obtain kubeconfig), bf-3d39n (verify kubeconfig access), bf-5xrym (verify secret exists), and bf-2pn4n (test kubectl access) were all marked as "closed" despite the kubeconfig never being obtained.

### Secret Mapping
From ExternalSecret analysis (/home/coding/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml):
- Kubernetes secret: `armor-writer` in namespace `devimprint`
- Secret key: `auth-access-key` (maps to environment variable `LITESTREAM_ACCESS_KEY_ID`)
- OpenBao source: `rs-manager/ord-devimprint/armor-writer` property `auth-access-key`

### Verification Attempts
All verification attempts fail with RBAC Forbidden error:
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

### Acceptance Criteria Status
- ❌ Successfully retrieved the base64-encoded value - BLOCKED (RBAC)
- ❌ Value is not empty - CANNOT VERIFY (cannot access secret)
- ❌ Value appears to be valid base64 - CANNOT VERIFY (cannot access secret)

## Resolution Required
This bead requires one of the following infrastructure changes:
1. Create ~/.kube/ord-devimprint.kubeconfig with secret read permissions from Rackspace Spot console
2. Update devpod-observer RBAC to allow secret read in devimprint namespace
3. Access the value directly from OpenBao (rs-manager/ord-devimprint/armor-writer)
4. Retrieve from another cluster with appropriate access permissions

## Bead Status
**OPEN and BLOCKED** - Cannot proceed without external intervention to resolve the infrastructure blocker.

Generated: 2026-07-12 00:27 UTC
