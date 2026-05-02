# armor-s8k.3.2.1: ARMOR v0.1.8 Verification on ord-devimprint - FAILED

## Task: Verify ARMOR v0.1.8 is running on ord-devimprint

**Date:** 2026-05-01
**Last attempted:** 2026-05-02 00:00 UTC

## Status: FAILED - Version Mismatch

## Findings

### Access Method
- ord-devimprint.kubeconfig: OIDC authentication failing (kubectl-oidc-login plugin not working)
- ord-devimprint.yaml: Static token expired
- Found via ArgoCD: `devimprint-ns-ardenone-hub` app (deployed on ardenone-hub cluster)
- Accessed via: `kubectl --server=http://traefik-ardenone-hub:8001`

### ARMOR Deployment Status (via ardenone-hub proxy)
```
Pod: armor-6c6f554d7d-8skcv
Status: Running
Image: localhost:7439/ronaldraygun/armor:0.1.11

Pod: armor-6cb55b69b-g468l
Status: Running
Image: localhost:7439/ronaldraygun/armor:0.1.13
```

### Aggregator Pod Status (via ardenone-hub proxy)
```
Pod: aggregator-68554db644-ng85f
Status: Running ✓
```

## Verification Result

**FAIL** - ARMOR is NOT running v0.1.8 as required.

- **Expected:** `ronaldraygun/armor:0.1.8`
- **Actual:** `localhost:7439/ronaldraygun/armor:0.1.11` and `localhost:7439/ronaldraygun/armor:0.1.13`
- **Aggregator:** Running ✓ (at least one pod is healthy)

## Notes

1. The deployment uses `localhost:7439` registry prefix which suggests images are pulled from a local registry
2. Two ARMOR replica pods are running different versions (0.1.11 and 0.1.13)
3. ArgoCD shows app as Degraded/OutOfSync
4. The ord-devimprint cluster appears to be deprecated or inaccessible; devimprint namespace now lives on ardenone-hub

## Acceptance Criteria Status

- [x] ARMOR pod is Running (2/2 pods Running)
- [ ] ARMOR image is ronaldraygun/armor:0.1.8 (FAIL - running 0.1.11 and 0.1.13)
- [x] Aggregator pod is Running (1/2 pods Running)

## References

- ArgoCD app: devimprint-ns-ardenone-hub
- Access via: kubectl --server=http://traefik-ardenone-hub:8001
- Cluster: ardenone-hub (devimprint namespace)
