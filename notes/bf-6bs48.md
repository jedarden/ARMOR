# bf-6bs48: RBAC Blocker - Secret Access Forbidden on ord-devimprint

## Task Attempted
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in devimprint namespace.

## Result
**RBAC BLOCKER - Access Forbidden**

### Verification (2026-07-11)
Attempted retrieval via kubectl-proxy:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

**Error:**
```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

### Secret Existence Verified
The `armor-writer` secret does exist (confirmed via `get secrets`):
```
armor-writer            Opaque                           2     80d
```

However, the ServiceAccount `devpod-observer` lacks permissions to read secret data.

### Root Cause
The kubectl-proxy for ord-devimprint runs with read-only RBAC that **explicitly blocks secret access**. This matches the pattern on `iad-options` where the observer also denies secret access.

### Access Constraints
- No direct kubeconfig exists for ord-devimprint (unlike iad-options which has a read/write kubeconfig)
- kubectl-proxy only provides list/get on resource metadata, not data access
- The devpod-observer ServiceAccount cannot read secrets in devimprint namespace

### Next Steps Required
To retrieve secret values from ord-devimprint, need one of:
1. Direct kubeconfig with elevated privileges (similar to iad-options pattern)
2. Updated RBAC rules to grant secret read access to devpod-observer SA
3. Alternative retrieval method (ExternalSecrets dump, direct OpenBao access, cached values from prior access)

### Acceptance Criteria Status
- ❌ Base64-encoded value retrieved: BLOCKED by RBAC
- ✅ armor-writer secret exists: CONFIRMED
- ❓ Contains LITESTREAM_ACCESS_KEY_ID field: Cannot verify without data access

## Documentation
This finding documents the RBAC blocker preventing secret access on ord-devimprint via the standard kubectl-proxy pattern.
