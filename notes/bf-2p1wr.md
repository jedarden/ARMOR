# Task bf-2p1wr: Obtain ord-devimprint kubeconfig with write access

## Current Situation

The ord-devimprint cluster is currently accessed via a read-only kubectl-proxy:
- Server: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- Access: Read-only via ServiceAccount `system:serviceaccount:devpod-observer:devpod-observer`
- Limitation: Cannot read secrets (Forbidden error when attempting to access `armor-writer` secret)

## Cluster Details

**ord-devimprint is a Rackspace Spot cluster:**
- Server URL: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- Exposed via Tailscale operator (hostname: `kubectl-proxy-ord-devimprint`)
- Similar to: `rs-manager`, `iad-options`, `iad-ci`

## Required Action

To obtain write access to ord-devimprint:

1. **Access Rackspace Spot UI** - Login to the Spot console at https://spot.rackspace.com
2. **Navigate to the ord-devimprint cloudspace** - Find the cluster with ID `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
3. **Download kubeconfig** - Use the Spot UI to download a kubeconfig with cloudspace-admin OIDC token
4. **Store securely** - Save to `~/.kube/ord-devimprint.kubeconfig`
5. **Verify access** - Run `kubectl get secrets -n devimprint` to confirm write access

## Pattern Reference

This follows the same pattern as `iad-options` (another Spot cluster):
- Kubeconfig path: `~/.kube/iad-options.kubeconfig`
- Token type: cloudspace-admin OIDC token
- Expiration: ~3 days (requires regeneration from Spot UI)

## Next Steps

Once kubeconfig is obtained:
1. Verify secret access: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`
2. Retrieve armor-writer secret for ARMOR operations
3. Proceed with dependent tasks that require cluster write access

## Notes

- The cluster name suggests it may be in us-west-002 (based on B2 bucket references in deployment configs)
- ExternalSecrets for this cluster reference OpenBao paths under `rs-manager/ord-devimprint/*`
