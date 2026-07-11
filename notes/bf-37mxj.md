# bf-37mxj: S3 Credentials from ord-devimprint Cluster

**Task:** Obtain S3 credentials from ord-devimprint cluster  
**Date:** 2026-07-11  
**Status:** BLOCKED - Insufficient Access Permissions

## Objective

Retrieve `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY` from the `armor-writer` secret in the `devimprint` namespace on the ord-devimprint cluster.

## Analysis

### Target Secret Location

The secret is an ExternalSecret that pulls from OpenBao:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: armor-writer
  namespace: devimprint
spec:
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

The secret contains:
- `auth-access-key` → `LITESTREAM_ACCESS_KEY_ID`
- `auth-secret-key` → `LITESTREAM_SECRET_ACCESS_KEY`

### Access Constraints

#### ord-devimprint Cluster
- **Available Access:** Read-only kubectl proxy at `http://kubectl-proxy-ord-devimprint:8001`
- **Limitation:** Explicitly denies secrets access
- **No Kubeconfig:** No direct kubeconfig available for ord-devimprint

#### rs-manager Cluster (OpenBao host)
- **Expected Location:** `/home/coding/.kube/rs-manager.kubeconfig`
- **Status:** File does not exist
- **Impact:** Cannot access OpenBao directly to retrieve credentials

### Blocked Attempts

1. **Read-only kubectl proxy:**
   ```bash
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
     get secret armor-writer -n devimprint
   ```
   Result: `Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

2. **Direct kubeconfig access:**
   - No ord-devimprint kubeconfig available
   - No rs-manager kubeconfig available

3. **Alternative access methods:**
   - No cached credentials in restore environment
   - No OpenBao direct access available

## Required Access

To complete this task, one of the following is needed:

### Option 1: ord-devimprint Kubeconfig
A kubeconfig with secret access to ord-devimprint cluster:
```bash
kubectl get secret armor-writer -n devimprint -o jsonpath='{.data.auth-access-key}' | base64 -d
kubectl get secret armor-writer -n devimprint -o jsonpath='{.data.auth-secret-key}' | base64 -d
```

### Option 2: rs-manager Kubeconfig
Access to rs-manager cluster to query OpenBao directly:
```bash
# OpenBao API access (requires cluster connectivity)
vault kv get -field=auth-access-key rs-manager/ord-devimprint/armor-writer
vault kv get -field=auth-secret-key rs-manager/ord-devimprint/armor-writer
```

### Option 3: Manual Credential Provisioning
Cluster administrator provides credentials directly.

## Acceptance Criteria Status

| Criteria | Status |
|----------|--------|
| LITESTREAM_ACCESS_KEY_ID environment variable set | ❌ BLOCKED |
| LITESTREAM_SECRET_ACCESS_KEY environment variable set | ❌ BLOCKED |
| Credentials validated (S3 authentication) | ❌ BLOCKED |

## Related Context

This task is related to database restore verification (bf-69ix4), which requires the same credentials. The restore verification infrastructure is complete but blocked on credential access.

## Resolution Path

To unblock this task:

1. **Obtain ord-devimprint kubeconfig** with secret access permissions
2. **Obtain rs-manager kubeconfig** to access OpenBao directly
3. **Request credentials** from cluster administrator
4. **Create cached credentials** in test environment after initial retrieval

## Technical Details

### Secret Structure
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: armor-writer
  namespace: devimprint
type: Opaque
data:
  auth-access-key: <base64-encoded>
  auth-secret-key: <base64-encoded>
```

### Environment Variable Mapping
```bash
export LITESTREAM_ACCESS_KEY_ID="<auth-access-key value>"
export LITESTREAM_SECRET_ACCESS_KEY="<auth-secret-key value>"
```

### Usage Context
These credentials are used by:
- Litestream backup/restore operations
- ARMOR S3 authentication
- queue-api database backup verification
- DevImprint production database restore testing

## Conclusion

**Task Status:** INCOMPLETE - Access Blocker

The credentials cannot be retrieved with current access permissions. The task requires cluster administrator intervention to provide either:
1. A kubeconfig with appropriate permissions
2. The credentials directly
3. Access to the rs-manager cluster (OpenBao host)

All infrastructure and retrieval commands are ready for immediate execution once access is granted.

## References

- ExternalSecret config: `/home/coding/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`
- Related verification task: `/home/coding/ARMOR/notes/bf-69ix4.md`
- Cluster documentation: `/home/coding/CLAUDE.md`
