# bf-1du7: ARMOR CrashLoopBackOff on ardenone-hub

## Issue Summary
ARMOR deployment on ardenone-hub (devimprint namespace) has 1 pod stuck in CrashLoopBackOff due to expired OpenBao ESO token. New pods cannot load secrets from OpenBao.

## Current State (2026-05-02)
- **Healthy pod:** armor-7c79d57db6-k2j6j (1/1 Running, 32 restarts) - uses cached secrets
- **Crashing pod:** armor-755d878c84-l8grt (0/1 CrashLoopBackOff, 61 restarts) - cannot load secrets
- **Deployment spec:** replicas: 1
- **Actual pods:** 2 (old ReplicaSet not scaled down)

## Root Cause
1. OpenBao ESO token expired (ClusterSecretStore `openbao` in `InvalidProviderConfig` state)
2. All ExternalSecrets in devimprint are in `SecretSyncedError` state
3. New pod cannot initialize without secrets (B2 credentials, MEK, auth keys)
4. Old ReplicaSet (armor-755d878c84) stuck with 1 crashing replica

## Immediate Fix Required (Manual)
Scale down the old ReplicaSet to 0:
```bash
# Requires write access to ardenone-hub (does not currently exist)
kubectl scale replicaset armor-755d878c84 -n devimprint --replicas=0
```

This will:
- Terminate the crashing pod
- Leave 1 healthy pod with cached secrets
- Meet acceptance criteria (0 CrashLoopBackOff pods)

## Access Constraints
- **Read-only proxy:** `traefik-ardenone-hub:8001` - cannot modify resources
- **Write-access kubeconfig:** Does not exist for ardenone-hub
- **ArgoCD:** Degraded due to PVC immutability issues, cannot sync

## Long-term Solution
Migrate devimprint namespace off ardenone-hub (cluster is targeted for shutdown). Options:
1. Migrate to ord-devimprint cluster (requires refreshed OIDC token)
2. Migrate to ardenone-manager cluster
3. Consolidate onto existing clusters

## Acceptance Criteria
ARMOR deployment stable with 0 CrashLoopBackOff pods.

**Status:** BLOCKED - requires write-access kubeconfig for ardenone-hub or manual intervention via cluster admin.
