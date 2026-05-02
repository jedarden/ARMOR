# armor-s8k.3.2.1: ARMOR v0.1.8 Verification on ord-devimprint - BLOCKED

## Task: Verify ARMOR v0.1.8 is running on ord-devimprint

**Date:** 2026-05-01

## Status: BLOCKED - Expired Credentials - Cannot Complete Task

## Issue

The kubeconfig at `/home/coding/.kube/ord-devimprint.kubeconfig` exists but returns "Unauthorized" errors when attempting to access the cluster.

```bash
$ kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint
error: You must be logged in to the server (Unauthorized)
```

## Investigation

1. **Kubeconfig exists**: File last modified 2026-04-28 (3 days ago)
2. **Cluster type**: Rackspace Spot cluster (server: hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com)
3. **Auth method**: Token-based (token appears to have expired)
4. **No kubectl-proxy**: Unlike other clusters (ardenone-hub, apexalgo-iad), ord-devimprint has no Tailscale kubectl-proxy
5. **Not in ArgoCD**: ord-devimprint cluster is not registered in ArgoCD (only devimprint namespace on ardenone-hub)

## Comparison to Other Clusters

| Cluster | Kubeconfig | Proxy | ArgoCD |
|---------|-----------|-------|--------|
| ardenone-hub | ✅ | ✅ (traefik-ardenone-hub:8001) | ✅ |
| apexalgo-iad | ✅ | ✅ (traefik-apexalgo-iad:8001) | ✅ |
| ardenone-manager | ✅ | ✅ (traefik-ardenone-manager:8001) | ✅ |
| rs-manager | ✅ | ✅ (traefik-rs-manager:8001) | ✅ |
| iad-ci | ✅ | ❌ | ❌ |
| **ord-devimprint** | ✅ (expired) | ❌ | ❌ |

## Required Action

Need to refresh the ord-devimprint.kubeconfig credentials. The ord-devimprint cluster access pattern is not documented in CLAUDE.md.

## Acceptance Criteria (Cannot Verify Without Access)

- [ ] ARMOR pod is Running on ord-devimprint
- [ ] ARMOR image is ronaldraygun/armor:0.1.8
- [ ] Aggregator pod is Running

## References

- Previous successful verification: notes/armor-s8k.3.2.md (v0.1.8 on ord-devimprint)
- Kubeconfig: /home/coding/.kube/ord-devimprint.kubeconfig
- Cluster: apexalgo-ord-devimprint (Rackspace Spot)
