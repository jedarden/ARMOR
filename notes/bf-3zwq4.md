# bf-3zwq4: Cannot Store Value - Extraction Blocked by RBAC

## Task Objective
Store the validated base64-encoded LITESTREAM_ACCESS_KEY_ID value temporarily for use in the decoding step.

## Prerequisite Status
The bead prerequisites state "Previous child bead complete (value validated as proper base64)" but the validation step **did not complete successfully**.

### Chain of Failures
1. **bf-5lx60** (Extraction) - RBAC blocker: devpod-observer SA cannot read secrets
2. **bf-4rqy0** (Validation) - RBAC blocker: same issue, no value to validate
3. **bf-3zwq4** (Storage) - No validated value exists to store

## Root Cause
The `ord-devimprint` cluster is only accessible via read-only kubectl-proxy at `http://kubectl-proxy-ord-devimprint:8001`. The observer ServiceAccount (`devpod-observer:devpod-observer`) explicitly denies secret access.

### Error Evidence
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Acceptance Criteria Status
- ❌ Value is stored in a temporary location (no value exists)
- ❌ Value is accessible for the next step (no value to access)
- ✅ Storage method is documented (this note file)

## Intended Storage Method
The bead description suggests two approaches:
```bash
# Shell variable
export LITESTREAM_ACCESS_KEY_ID_B64=<extracted_value>

# Temporary file
echo <extracted_value> > /tmp/litestream_access_key_id.b64
```

Neither can be executed since `<extracted_value>` does not exist.

## Resolution Required
To complete the litestream restore workflow, one of the following is needed:

1. **Grant secret read access** to the `devpod-observer` ServiceAccount for the `devimprint` namespace
2. **Create a dedicated secret-reader ServiceAccount** with limited permissions
3. **Use direct cluster admin access** via kubeconfig (if available)
4. **Alternative cluster access** with elevated privileges

## Recommendation
Update the RBAC configuration for ord-devimprint to allow secret read access for the observer SA, or create a workflow that uses a cluster with appropriate credentials (e.g., ardenone-manager or iad-ci which have direct kubeconfigs with cluster-admin access).

---
*Date: 2026-07-11*
*Cluster: ord-devimprint*
*Proxy: kubectl-proxy-ord-devimprint:8001 (read-only, no secret access)*
*Secret: armor-writer in devimprint namespace*
