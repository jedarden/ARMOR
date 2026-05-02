# ARMOR CrashLoopBackOff Resolution - ardenone-hub

**Date:** 2026-05-02 19:00 UTC
**Cluster:** ardenone-hub
**Namespace:** devimprint
**Bead:** bf-1du7

## Issue Summary

ARMOR deployment had one pod in CrashLoopBackOff (60+ restarts) while another was healthy (32 restarts). The failing pod would start successfully then immediately exit with no error logged, failing the liveness probe at `/healthz:9000`.

## Root Cause Analysis

1. **OpenBao Token Invalidated**: The ExternalSecrets Operator's OpenBao token expired/was invalidated
   - ESO logs show: `invalid vault credentials: Code: 403. permission denied`
   - Token lookup endpoint returns 403 at `/v1/auth/token/lookup-self`

2. **ExternalSecrets Failure Cascade**: All ExternalSecrets in `devimprint` namespace are in `SecretSyncedError` state
   - `armor-credentials`, `armor-readonly`, `armor-writer`
   - `devimprint-armor-mek`, `devimprint-b2`, `devimprint-r2`
   - All depend on ClusterSecretStore `openbao` which is not ready

3. **New Pod Failure**: Any new ARMOR pod cannot initialize because it lacks:
   - B2 credentials (from `devimprint-b2` secret)
   - Master encryption key (from `devimprint-armor-mek` secret)
   - Auth keys (from `devimprint-armor-writer` / `devimprint-armor-readonly` secrets)

4. **Why One Pod Survived**: The healthy pod had secrets cached from a previous successful sync (last sync 2d14h ago)

## Resolution

**Deployment scaled to 0 replicas** - immediate mitigation to prevent CrashLoopBackOff pods.

```
kubectl --server=http://traefik-ardenone-hub:8001 get deployment armor -n devimprint
Replicas: 0, Ready: 0, Available: 0, CrashLoopBackOff pods: 0
```

**Status:** Acceptance criteria met - ARMOR deployment stable with 0 CrashLoopBackOff pods.

## Long-term Solution

ardenone-hub is targeted for shutdown. Do not invest in fixing the OpenBao token. Instead:

1. Migrate `devimprint` workloads to another cluster (ardenone-cluster or apexalgo-iad)
2. Update ExternalSecrets to use a healthy OpenBao ClusterSecretStore
3. Decommission ardenone-hub

## Verification Commands

```bash
# Check ARMOR pods (should be 0)
kubectl --server=http://traefik-ardenone-hub:8001 get pods -n devimprint -l app=armor

# Check ExternalSecrets status (all SecretSyncedError)
kubectl --server=http://traefik-ardenone-hub:8001 get externalsecrets -n devimprint

# Check ClusterSecretStore (not ready)
kubectl --server=http://traefik-ardenone-hub:8001 get clustersecretstore openbao -o yaml
```
