# ord-devimprint Kubeconfig Access Blocker Analysis

**Date:** 2026-07-11  
**Bead:** bf-2p1wr  
**Status:** Persistent Blocker

## Current State

### Access Available
- **Read-only proxy:** `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **Namespace:** devpod-observer (ServiceAccount: devpod-observer)
- **Capabilities:** Can list pods, list secret names, but cannot read secret contents

### Access Denied
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Cluster Analysis

### Management Chain
- ord-devimprint is managed via ArgoCD on **rs-manager** cluster
- Application: `apps-ord-devimprint` in rs-manager's ArgoCD
- Source: `jedarden/declarative-config/k8s/ord-devimprint/`
- Destination: rs-manager deploys Helm charts TO ord-devimprint

### Cluster Type
- Likely a **Rackspace Spot cluster** in Chicago (ORD = O'Hare)
- Follows naming pattern of other Spot clusters (iad-options, iad-ci, iad-kalshi)

## What's Needed

### Acceptance Criteria (NOT MET)
- [ ] Kubeconfig file for ord-devimprint cluster obtained
- [ ] Kubeconfig has permissions to read secrets in devimprint namespace
- [ ] Can successfully run: `kubectl get secrets -n devimprint`

### Required Actions
This task requires **coordination with the cluster administrator** or **access to Rackspace Spot console**:

1. **Generate kubeconfig from Rackspace Spot console:**
   - Login to Rackspace Spot UI (https://console.rackspace.com)
   - Navigate to the ord-devimprint cloudspace
   - Generate kubeconfig/OIDC token (similar to iad-options pattern)
   - Store at: `~/.kube/ord-devimprint.kubeconfig`

2. **Alternative: Get credentials from cluster administrator**
   - Request kubeconfig from whoever manages the ord-devimprint cluster
   - Ensure it has secret-read permissions in devimprint namespace

## Why This Can't Be Automated

- No existing kubeconfig found in `~/.kube/`
- rs-manager does not expose ord-devimprint credentials as accessible secrets
- Read-only proxy explicitly blocks secret access (RBAC design)
- Rackspace Spot kubeconfigs require UI-based authentication/OIDC token generation
- No programmatic access to Spot console available

## Next Steps

**BLOCKED:** Requires user action to provide kubeconfig or grant Spot console access.

Once kubeconfig is obtained:
1. Save to `~/.kube/ord-devimprint.kubeconfig`
2. Test with: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`
3. Verify armor-writer secret is readable
4. Proceed to child bead that needs the secret

## Related Documentation

- CLAUDE.md: Kubernetes Access section (no ord-devimprint write access documented)
- iad-options pattern: Similar setup with expiring OIDC token requiring Spot UI refresh
