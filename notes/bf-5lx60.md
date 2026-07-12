# Task bf-5lx60: RBAC Blocker

## Task Description
Extract base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in ord-devimprint cluster.

## Execution Attempt
Command attempted:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

## Blocker
**RBAC Forbidden Error**: The `devpod-observer` service account does not have permission to read `secrets` resources in the `devimprint` namespace.

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Cluster Access Details
- **Cluster**: ord-devimprint
- **Access method**: kubectl-proxy over Tailscale (read-only)
- **ServiceAccount**: devpod-observer
- **Namespace**: devimprint
- **Direct kubeconfig**: Not available (checked ~/.kube/*.kubeconfig)

## Resolution Required
This task requires one of the following:
1. RBAC update to grant secrets read access to devpod-observer SA in devimprint namespace
2. Direct kubeconfig with appropriate permissions (similar to iad-ci pattern)
3. Alternative access method to retrieve the secret value

## Acceptance Criteria Status
❌ Successfully retrieved the base64-encoded value - BLOCKED BY RBAC
❌ Value is not empty - CANNOT VERIFY
❌ kubectl command completed without error - RBAC FORBIDDEN
