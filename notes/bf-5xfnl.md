# Task bf-5xfnl: Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from secret

## Status: BLOCKED - Missing read/write kubeconfig

## Cluster location clarification

The `devimprint` namespace and `armor-writer` secret exist on the **ord-devimprint** cluster (not iad-options). Verified 2026-07-11:
- ord-devimprint: `devimprint` namespace exists, `armor-writer` secret exists
- iad-options: `devimprint` namespace does NOT exist

## What was attempted

1. Checked for ord-devimprint read/write kubeconfig - no kubeconfig documented in CLAUDE.md for this cluster
2. Checked for iad-options read/write kubeconfig at `/home/coding/.kube/iad-options.kubeconfig` - does not exist
3. Attempted ord-devimprint proxy access via `http://kubectl-proxy-ord-devimprint:8001` - blocked by RBAC (observer SA denies secret access)
4. Verified secret exists on ord-devimprint: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint` → `secret/armor-writer` confirmed
5. Attempted iad-options proxy access - devimprint namespace doesn't exist on that cluster
6. Checked all available kubeconfigs: only `iad-acb.kubeconfig` and `iad-ci.kubeconfig` exist (neither relevant)

## Root cause

The ord-devimprint cluster has only read-only proxy access documented in CLAUDE.md. According to project documentation:

> Read/write (cloudspace-admin OIDC token, expires every ~3 days — regenerate from Spot UI)

The credential needs to be regenerated from the Spot UI and saved to `/home/coding/.kube/iad-options.kubeconfig`.

## Prerequisite status

- Child bead bf-58r06: BLOCKED (same issue - missing kubeconfig, could not retrieve value)
- Child bead bf-2c1jp: BLOCKED (same issue - read/write kubeconfig required for secret access)
- Namespace devimprint: Cannot verify without secret read access
- Secret armor-writer: Cannot verify without secret read access

## Git evidence

Recent commits confirm the blocker persists:
- `97e3cc7d docs(bf-48qtv): document blocked state - prerequisite bf-58r06 did not retrieve value`
- `5238ba3f docs(bf-58r06): re-verify blocked state - kubeconfig still missing`
- `8e26e14b docs(bf-58r06): task blocked - missing iad-options read/write kubeconfig`

## Command to run once kubeconfig is available

```bash
# For ord-devimprint (if kubeconfig path becomes available):
kubectl --kubeconfig=<ord-devimprint-kubeconfig-path> get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'

# OR if secret is replicated to iad-options with appropriate kubeconfig:
kubectl --kubeconfig=/home/coding/.kube/iad-options.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

## Next steps

User needs to provide read/write access to the armor-writer secret. Options:
1. **For ord-devimprint cluster**: Obtain or create a read/write kubeconfig for ord-devimprint (not currently documented in CLAUDE.md)
2. **For iad-options cluster**: Regenerate the cloudspace-admin OIDC token from Spot UI for iad-options cluster and save to `/home/coding/.kube/iad-options.kubeconfig`, then verify if the devimprint namespace/secret exists there
3. **Alternative**: Provide the secret value directly or create an alternative access method

## Verification (2026-07-11)

Confirmed the following:
- ✅ `devimprint` namespace exists on **ord-devimprint** cluster
- ✅ `armor-writer` secret exists in `devimprint` namespace on ord-devimprint
- ✅ Proxy access to ord-devimprint works for namespace/pod queries
- ❌ Proxy access blocked for secret reads (RBAC: observer SA cannot get secrets)
- ❌ No read/write kubeconfig exists for ord-devimprint cluster
- ❌ `devimprint` namespace does NOT exist on iad-options cluster

## Acceptance criteria (not met)

- ❌ Successfully retrieved the base64-encoded value
- ❌ Value is not empty
- ❌ Value appears to be valid base64

**Status**: BLOCKED - awaiting read/write credentials or kubeconfig

Date: 2026-07-11
Last verified: 2026-07-12 01:01 UTC
Bead ID: bf-5xfnl

## Re-verification (2026-07-12 01:01 UTC)

Re-verified all access paths:
- ❌ ord-devimprint proxy still blocks secret access (RBAC Forbidden)
- ❌ No kubeconfigs exist for ord-devimprint or rs-manager
- ❌ Only available kubeconfigs: `iad-acb.kubeconfig`, `iad-ci.kubeconfig`
- ✅ Secret source confirmed: ExternalSecret from OpenBao at `rs-manager/ord-devimprint/armor-writer`
- ✅ Field mapping confirmed: `auth-access-key` → `LITESTREAM_ACCESS_KEY_ID`

### Available kubeconfigs
```bash
$ ls -la ~/.kube/*.kubeconfig
-rw-r--r-- 1 coding users  282 Jun 25 07:20 /home/coding/.kube/iad-acb.kubeconfig
-rw-r--r--  coding users 2809 Jun  7 08:31 /home/coding/.kube/iad-ci.kubeconfig
```

### Secret structure (from ExternalSecret manifest)
```yaml
# armor-writer secret in devimprint namespace
secretKey: auth-access-key
remoteRef:
  key: rs-manager/ord-devimprint/armor-writer
  property: auth-access-key
```

Infrastructure blocker remains unresolved. Task cannot complete without external access.
