# Blocked: Missing ord-devimprint Kubeconfig

## Status
**BLOCKED** - Cannot complete without prerequisite kubeconfig

## Issue
Bead bf-2xkyl requires retrieving S3 credentials from the `armor-writer` secret in the `devimprint` namespace. The ord-devimprint cluster is a Rackspace Spot cluster that requires write access to read secrets.

## Prerequisite Not Met
- **Bead bf-2p1wr** (obtain ord-devimprint kubeconfig with write access) is not complete
- Required kubeconfig file: `~/.kube/ord-devimprint.kubeconfig`
- **File does not exist**

## Why Read-Only Proxy Fails
```
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
Error from server (Forbidden): User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

The read-only proxy ServiceAccount explicitly denies secrets access.

## What's Needed
The user must manually obtain the kubeconfig from Rackspace Spot console:
1. Log in to Rackspace Spot console
2. Navigate to ord-devimprint cloudspace (hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com)
3. Download/generate cloudspace-admin kubeconfig
4. Save to `~/.kube/ord-devimprint.kubeconfig`
5. Set permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

## Commands to Run Once Kubeconfig is Available
```bash
# Verify connectivity
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# Retrieve ACCESS_KEY_ID
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d

# Retrieve SECRET_ACCESS_KEY
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}' | base64 -d
```

## Pattern Reference
This follows the same pattern as iad-options (another Rackspace Spot cluster), which uses `~/.kube/iad-options.kubeconfig` with cloudspace-admin OIDC token from Spot UI.

## Next Steps
1. User obtains kubeconfig from Rackspace Spot UI
2. Save to `~/.kube/ord-devimprint.kubeconfig`
3. Retry this bead (bf-2xkyl)
