# Bead bf-2c1jp: Secret Verification Blocked

## Issue

Cannot verify `armor-writer` secret in `devimprint` namespace due to missing kubeconfig.

## Root Cause

1. **Observer kubeconfig missing**: `/home/coding/.kube/iad-options-observer.kubeconfig` does not exist
2. **Read/write kubeconfig missing**: `/home/coding/.kube/iad-options.kubeconfig` does not exist
3. **Observer denies secret access**: Even with proxy access, the observer SA explicitly denies secret access

## Infrastructure Context

From CLAUDE.md:
- Observer proxy has stricter RBAC: "explicitly denies access to secrets"
- Read/write kubeconfig uses "cloudspace-admin OIDC token, expires every ~3 days — regenerate from Spot UI"

## Resolution Required

The read/write kubeconfig must be regenerated from the Rackspace Spot UI and saved to:
`/home/coding/.kube/iad-options.kubeconfig`

This is an OIDC token that expires approximately every 3 days.

## Verification Attempt

```bash
# Proxy access - denied as expected
kubectl --server=http://traefik-iad-options:8001 get secret armor-writer -n devimprint
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

The infrastructure is correctly configured - the observer cannot read secrets. Need read-write kubeconfig with cloudspace-admin OIDC token.

## Status

**BLOCKED** - Requires manual regeneration of kubeconfig from Rackspace Spot UI.
