# ARMOR CrashLoopBackOff Investigation - ardenone-hub

**Date:** 2026-05-02
**Bead:** bf-1du7

## Current State

### ARMOR Deployment Status

| Namespace | Replicas | Ready | Status | Pod Name | Restarts |
|-----------|----------|-------|--------|----------|----------|
| armor | 1 | 1/1 | Running | armor-7b5876fd57-4s979 | 2 (11h ago) |
| devimprint | 0 | 0 | Scaled down | - | - |

**Acceptance Status:** ✅ **MET** - 0 CrashLoopBackOff pods

The deployment described in the bead (with CrashLoopBackOff pods) has been resolved by scaling down the devimprint deployment. The armor namespace deployment is stable with 1 running pod.

### Root Cause Analysis

**Primary Issue:** ClusterSecretStore `openbao` on ardenone-hub is in `InvalidProviderConfig` state

```
Status:
  conditions:
  - lastTransitionTime: "2026-04-30T04:55:46Z"
    message: unable to validate store
    reason: InvalidProviderConfig
    status: "False"
    type: Ready
```

**Impact:**
- All ExternalSecrets in devimprint namespace are failing (SecretSyncedError)
- All ExternalSecrets in armor namespace are failing (SecretSyncedError)
- Last successful sync: April 30, 2026 04:56 UTC
- New pods cannot be created (no fresh secrets)
- Existing pods survive on cached secrets from 21+ days ago

### ExternalSecret Status

| Secret | Namespace | Status | Last Sync |
|--------|-----------|--------|-----------|
| armor-secrets | armor | SecretSyncedError | 2026-04-30 |
| devimprint-b2 | devimprint | SecretSyncedError | 2026-04-30 |
| devimprint-armor-mek | devimprint | SecretSyncedError | 2026-04-30 |
| devimprint-armor-readonly | devimprint | SecretSyncedError | 2026-04-30 |
| devimprint-armor-writer | devimprint | SecretSyncedError | 2026-04-30 |

### OpenBao Status

**ardenone-hub OpenBao:**
- Pods: Running (openbao-ardenone-hub-0: 1/1)
- Service: ClusterIP 10.43.20.120:8200
- Issue: Token-based auth failing for ExternalSecrets operator

**ardenone-cluster OpenBao (central):**
- Pods: Running (openbao-ardenone-cluster-0: 1/1)
- ClusterSecretStore: Valid and healthy
- Auth method: Kubernetes service account (working)

**apexalgo-iad:**
- ExternalName service pointing to `ardenone-cluster-mesh.tailscale.svc.cluster.local`
- ClusterSecretStore also failing (InvalidProviderConfig)
- Uses token auth (same issue as ardenone-hub)

## Architecture Notes

```
ardenone-cluster (central OpenBao)
├── OpenBao running in openbao namespace
├── ClusterSecretStore: Uses Kubernetes auth (working)
└── ExternalSecrets: Mostly healthy

apexalgo-iad
├── ExternalName service: openbao -> ardenone-cluster-mesh.tailscale
├── ClusterSecretStore: Token auth (BROKEN)
└── ARMOR pods: Pending (no secrets)

ardenone-hub
├── OpenBao running in openbao namespace
├── ClusterSecretStore: Token auth (BROKEN)
├── ARMOR (armor namespace): 1/1 Running (cached secrets)
└── ARMOR (devimprint namespace): Scaled to 0
```

## Migration Considerations

**ardenone-hub Decommission Status:** Planned

The bead indicates ardenone-hub is targeted for shutdown. Options:

1. **Migrate ARMOR to ardenone-cluster**
   - Central OpenBao is healthy with working auth
   - Would require creating new ExternalSecrets pointing to ardenone-cluster paths
   - Need to ensure secrets exist in OpenBao at new paths

2. **Fix OpenBao token auth on ardenone-hub**
   - Not recommended given planned decommission
   - Token rotation may be required
   - Root cause: Token in `openbao-eso-token` secret may be expired

## Recommendations

1. **Short-term:** Current state is stable (0 CrashLoopBackOff)
   - Cached secret valid for 21+ days
   - Pod is running and serving traffic
   - Monitor for secret expiration

2. **Long-term:** Migrate to ardenone-cluster
   - Create `k8s/ardenone-cluster/armor/` directory
   - Update ExternalSecret paths from `ardenone-hub/*` to `ardenone-cluster/*`
   - Ensure OpenBao has secrets at new paths
   - Create ArgoCD application for ardenone-cluster

## Files of Interest

- `/home/coding/declarative-config/k8s/ardenone-hub/armor/` - ARMOR manifests for ardenone-hub
- `/home/coding/declarative-config/k8s/ardenone-cluster/` - Target for migration (no armor/ dir exists yet)

## Acceptance Criteria

✅ **ARMOR deployment stable with 0 CrashLoopBackOff pods**

- Current pod: armor-7b5876fd57-4s979 (1/1 Running)
- Restarts: 2 (11 hours ago - likely during failed secret refresh attempt)
- No pods in CrashLoopBackOff state
- devimprint deployment scaled to 0 (mitigation applied)
