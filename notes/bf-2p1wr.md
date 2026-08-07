# Task bf-2p1wr: Obtain ord-devimprint kubeconfig with write access

## Situation Analysis

### Existing kubeconfigs found:
1. **`ord-devimprint-observer.kubeconfig`** - Read-only, long-lived SA token, explicitly denies access to secrets
2. **`ord-devimprint-admin.kubeconfig`** - Admin access, uses OIDC token that expires every ~3 days

### Current Status
The admin kubeconfig exists but its authentication has failed:

1. **OIDC Authentication** (current context: `apexalgo-ord-devimprint-oidc`):
   - Requires interactive browser-based authentication
   - Failed with: "could not open the browser" and "authorization error: context deadline exceeded"
   - This environment is a server without a graphical browser

2. **Static Token Authentication** (context: `apexalgo-ord-devimprint`):
   - Contains a JWT token that was issued on 2026-07-26
   - Token has expired (current date: 2026-08-06, ~11 days past issue date)
   - Returns "Unauthorized" error

## What's Needed

The Rackspace Spot cloudspace-admin OIDC token must be regenerated manually through the Spot UI. This cannot be done from this server because:

1. The OIDC authentication flow requires a web browser for the OAuth2 authorization code flow
2. The token expires every ~3 days and must be refreshed through the Spot control panel
3. There is no command-line API to generate new admin tokens without browser interaction

## Resolution Path

**Manual intervention required:**

1. Log into Rackspace Spot console (https://spot.rackspace.com/)
2. Navigate to the ord-devimprint cloudspace
3. Access the cloudspace-admin credentials section
4. Generate a new kubeconfig/OIDC token
5. Replace or update `/home/coding/.kube/ord-devimprint-admin.kubeconfig` with the new credentials
6. Verify access with: `kubectl get secrets -n devimprint`

## Alternative Approaches to Consider

1. **Long-lived service account**: Instead of using the expiring OIDC token, create a long-lived ServiceAccount in the devimprint namespace with secret read permissions
2. **API-based authentication**: If Rackspace Spot provides an API for generating admin credentials, that could be scripted
3. **Use the observer RBAC as base**: The observer SA already exists; we could extend its permissions to allow secret reading in specific namespaces

## Files Referenced
- `/home/coding/.kube/ord-devimprint-admin.kubeconfig` - Admin kubeconfig (expired)
- `/home/coding/.kube/ord-devimprint-observer.kubeconfig` - Observer kubeconfig (read-only, no secrets)
- `/home/coding/.kube/ord-devimprint.kubeconfig` - Another kubeconfig file (purpose unknown, dated May 4)
- `/home/coding/.kube/ord-devimprint-token.kubeconfig` - Token-based kubeconfig (dated Jun 9)

## Context
This bead (bf-2p1wr) is a dependency for parent task bf-2p1wp, which needs to retrieve the `armor-writer` secret from the devimprint namespace.
