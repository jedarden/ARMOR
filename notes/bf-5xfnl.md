# bf-5xfnl: Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID

## Attempt
Attempted to retrieve LITESTREAM_ACCESS_KEY_ID from armor-writer secret in devimprint namespace.

## Command Run
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

## Result
**Exit code 1 - Forbidden**

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Blocker
RBAC restriction on the ord-devimprint kubectl-proxy:
- The read-only proxy runs as `system:serviceaccount:devpod-observer:devpod-observer`
- This service account **explicitly denies access to secrets** (similar to iad-options cluster's stricter observer)
- Only pod/logs inspection is available through the proxy

## Infrastructure Status
This is a **persistent infrastructure blocker** that has been verified across multiple commits:
- 4186964e - re-verify infrastructure blocker - RBAC still denies secret access
- 1e69b07a - document persistent infrastructure blocker - RBAC prevents secret access

## Alternative Approaches Not Available
- No read/write kubeconfig exists for ord-devimprint cluster
- No cluster-admin access like ardenone-manager/iad-ci
- Proxy is the only documented access method

## Resolution Required
To complete this task, one of the following infrastructure changes is needed:
1. Add a read/write kubeconfig for ord-devimprint (similar to iad-ci)
2. Modify the devpod-observer RBAC to allow secret get/list in devimprint namespace
3. Provide the secret value through an alternative channel (e.g., manual copy, ExternalSecret sync)

## Next Steps
The bead cannot be completed without infrastructure access. Awaiting:
- Infrastructure resolution (RBAC update or kubeconfig provision)
- Or alternative task approach that doesn't require direct secret access
