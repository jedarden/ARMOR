# Bead bf-2txcw: Verify kubectl access to iad-options cluster

## Task
Confirm that kubectl can access the iad-options cluster.

## Findings

### Observer kubeconfig status
The file `/home/coding/.kube/iad-options-observer.kubeconfig` does **not exist**.

### Working access method
The correct read-only access method for iad-options is via the kubectl-proxy over Tailscale:

```bash
kubectl --server=http://traefik-iad-options:8001 get namespaces
kubectl --server=http://traefik-iad-options:8001 get pods -n options
```

This method works correctly and returns full namespace and pod listings.

### Access details (from CLAUDE.md)
- **Cluster:** iad-options (Rackspace Spot, us-east-iad-1)
- **Access:** Read-only proxy in `devpod-observer` namespace
- **RBAC:** Explicitly denies access to secrets (stricter than other clusters)
- **Ingress:** Single Tailscale ingress `traefik-iad-options` with kubectl-proxy routed via Traefik's `kubectl-tcp` entrypoint

## Verification completed
✅ kubectl access to iad-options cluster is functional via the proxy method
