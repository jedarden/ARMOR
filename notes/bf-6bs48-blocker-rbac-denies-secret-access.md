# Bead bf-6bs48: BLOCKER - RBAC Prevents Secret Data Access

## Date: 2026-07-11

## Task
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in devimprint namespace.

## Blocker
**RBAC does not permit secret data retrieval via kubectl-proxy**

## Investigation

### Parent Bead Verification Status
The parent bead (bf-enpyd) claimed:
> "Secret read permissions: ✅ Working - Successfully listed 9+ secrets including armor-credentials, armor-readonly, and armor-writer"

However, this claim only verified **LIST** permission, not **GET** permission.

### Actual Permissions
- `kubectl get secrets` - ✅ WORKING (can list secret names)
- `kubectl get secret <name>` - ❌ FORBIDDEN (cannot read secret data)

### Error Message
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot 
get resource "secrets" in API group "" in the namespace "devimprint"
```

### Authorization Check
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i get secrets -n devimprint
# Output: no
```

## Root Cause
The kubectl-proxy-ord-devimprint uses ServiceAccount `devpod-observer:devpod-observer` which has:
- **LIST** permission on secrets (to enumerate secret names)
- **NO GET** permission on secrets (to read secret data)

This is consistent across multiple clusters' observer proxies for security reasons.

## Available Secrets (LIST only)
```
NAME                    TYPE                             DATA   AGE
admin-oauth             Opaque                           3      62d
armor-credentials       Opaque                           7      80d
armor-readonly          Opaque                           2      80d
armor-writer            Opaque                           2      80d  ← TARGET SECRET
devimprint-b2-workers   Opaque                           5      65d
devimprint-cloudflare   Opaque                           8      80d
docker-hub-registry     kubernetes.io/dockerconfigjson   1      80d
github-oauth            Opaque                           2      31d
github-pat              Opaque                           1      80d
queue-api-auth          Opaque                           2      2d13h
```

The `armor-writer` secret exists and contains 2 data fields (likely `auth-access-key` and `auth-secret-key` which map to `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY`), but the actual values cannot be retrieved.

## Resolution Required
To retrieve the LITESTREAM_ACCESS_KEY_ID, one of the following is needed:

1. **Direct kubeconfig for ord-devimprint** - A kubeconfig with secret read access to the devimprint namespace
2. **rs-manager kubeconfig** - To access OpenBao and retrieve the secret from the source path `rs-manager/ord-devimprint/armor-writer`
3. **Cluster administrator assistance** - Direct provision of the credential value

## Related Blockers
- Bead bf-112tt: LITESTREAM_SECRET_ACCESS_KEY Retrieval - BLOCKED (same issue)
- Bead bf-2778z: Documents that kubeconfigs with secret access do not exist

## Status
**BLOCKED** - Cannot retrieve LITESTREAM_ACCESS_KEY_ID without secret access permissions.
