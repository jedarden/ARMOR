# Bead bf-2778z: Secret Key Name Correction

## Important Discovery

The bead description contains an inaccuracy about the secret key name.

## Bead Description vs. Reality

### What the bead says:
```yaml
Commands:
kubectl --kubeconfig=<path> get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d
```

### What's actually in the secret:
From `/home/coding/declarative-config/k8s/ord-devimprint/devimprint/queue-api-deployment.yml`:
```yaml
env:
  - name: LITESTREAM_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: armor-writer
        key: auth-access-key  # ← The ACTUAL secret key name
```

The environment variable `LITESTREAM_ACCESS_KEY_ID` is populated from the `auth-access-key` field of the `armor-writer` secret.

## Corrected Command

If/when access becomes available, the correct command should be:
```bash
kubectl --kubeconfig=<path> get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}' | base64 -d
```

## Status Remains BLOCKED

This correction doesn't change the fundamental blocker:
- ❌ No write-access kubeconfig for ord-devimprint exists
- ❌ Read-only proxy (`kubectl-proxy-ord-devimprint:8001`) explicitly forbids secret access
- ❌ Prerequisite bead `bf-2p1wr` remains OPEN

The task cannot proceed until proper cluster credentials are obtained.

## Secret Contents Reference

From `devimprint-externalsecrets.yml`, the `armor-writer` secret contains:
- `auth-access-key` (maps to `LITESTREAM_ACCESS_KEY_ID` env var)
- `auth-secret-key` (maps to `LITESTREAM_SECRET_ACCESS_KEY` env var)

These are synced from OpenBao path: `rs-manager/ord-devimprint/armor-writer`

## Verification Date
2026-07-11

## Latest Verification Attempt (2026-07-11)

Attempted to retrieve the secret using the corrected key name:

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}' | base64 -d
```

**Result:** Forbidden - same RBAC blocker
```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

## Conclusion

The task remains **blocked by RBAC**. The read-only proxy on ord-devimprint explicitly denies secret access, and no write-access kubeconfig is available for this cluster.
