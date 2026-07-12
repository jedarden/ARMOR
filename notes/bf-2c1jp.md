# Bead bf-2c1jp: Verify armor-writer secret exists in devimprint namespace

## Date
2026-07-11

## Finding

The armor-writer secret cannot be verified with current access configurations:

1. **Read-only proxy access** (via `traefik-iad-options:8001`): Explicitly denies access to secrets in the devimprint namespace. This is documented as stricter than other clusters' observers.

   ```
   Error from server (Forbidden): secrets "armor-writer" is forbidden: 
   User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

2. **Read/write kubeconfig** (`/home/coding/.kube/iad-options.kubeconfig`): Does not exist. This kubeconfig contains a cloudspace-admin OIDC token that expires every ~3 days and must be regenerated from the Spot UI.

## Prerequisite Status

The prerequisite bead bf-2txcw verified kubectl access to the iad-options cluster, but this only confirmed read-only proxy access — not the read/write access required for secret verification.

## Resolution Path

To complete this task, the read/write kubeconfig must be regenerated:
- Access the Rackspace Spot UI
- Generate a new cloudspace-admin OIDC token
- Save to `/home/coding/.kube/iad-options.kubeconfig`
- Retry: `kubectl --kubeconfig=/home/coding/.kube/iad-options.kubeconfig get secret armor-writer -n devimprint`
