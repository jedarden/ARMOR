# Bead bf-112tt: LITESTREAM_SECRET_ACCESS_KEY Retrieval Blocker

## Status: BLOCKED by RBAC

### Issue
Cannot retrieve `LITESTREAM_SECRET_ACCESS_KEY` from `armor-writer` secret in `devimprint` namespace due to insufficient RBAC permissions.

### Evidence
1. **Read-only proxy blocks secret access:**
   ```
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
   Error: secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

2. **No admin kubeconfig available:**
   - Only kubeconfigs found: `iad-acb.kubeconfig`, `iad-ci.kubeconfig`
   - No `ord-devimprint.kubeconfig` exists for admin access

3. **OpenBao access failed:**
   - ExternalSecret syncs from OpenBao (`rs-manager/ord-devimprint/armor-writer`)
   - Direct OpenBao access via `https://openbao.ardenone.com:8200` times out
   - Cannot retrieve credentials at source

### ExternalSecret Details
```yaml
apiVersion: external-secrets.io/v1
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

### Resolution Required
To complete this bead, one of the following is needed:
1. **Create ord-devimprint.kubeconfig** with cluster-admin or secret-read permissions
2. **OpenBao VPN access** - OpenBao server may require VPN/TS routing
3. **Alternative access method** - Direct OpenBao token or API access with proper credentials

### Note on Key Names
The bead references `LITESTREAM_SECRET_ACCESS_KEY`, but the ExternalSecret shows `auth-secret-key` as the source property. The Kubernetes secret key mapping may differ from OpenBao property names.

### References
- CLAUDE.md: ord-devimprint cluster uses read-only proxy only
- ExternalSecret: `armor-writer` in `devimprint` namespace
- OpenBao path: `rs-manager/ord-devimprint/armor-writer`
