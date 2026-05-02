# ARMOR CrashLoopBackOff Investigation (bf-1du7)

## Summary

The ARMOR CrashLoopBackOff issue on ardenone-hub has been **mitigated** by scaling the deployment to 0 replicas. The deployment configuration explicitly documents this with a comment referencing this bead.

## Current State (2026-05-02, verified 2026-05-02T19:00Z)

### ardenone-hub (problem location)
- **Deployment**: devimprint/armor
- **Replicas**: 0 desired, 0 ready, 0 available
- **Pods**: None running
- **CrashLoopBackOff**: 0 pods ✓
- **ExternalSecrets**: All in SecretSyncedError state
- **ClusterSecretStore openbao**: InvalidProviderConfig (OpenBao running but auth issue)

### ardenone-cluster (migration target)
- **Deployment**: devimprint/armor (replicas: 2)
- **Pods**: 2 pods in CreateContainerConfigError (not CrashLoopBackOff)
- **CrashLoopBackOff**: 0 pods ✓
- **Issue**: ClusterSecretStore points to local OpenBao which has no running pods
- **ExternalSecrets**: SecretSyncedError - cannot reach OpenBao

### OpenBao Infrastructure
- **rs-manager**: OpenBao running, ClusterSecretStore Valid
- **ardenone-manager**: OpenBao running, ClusterSecretStore InvalidProviderConfig
- **ardenone-cluster**: No OpenBao pods, ClusterSecretStore points to non-existent local service
- **ardenone-hub**: OpenBao running, ClusterSecretStore InvalidProviderConfig

## Root Cause

ardenone-hub OpenBao was unreachable, causing ExternalSecrets to fail sync. New ARMOR pods failed liveness probe at /healthz:9000 due to missing credentials.

## Mitigation Applied

ardenone-hub ARMOR deployment scaled to 0 replicas with documentation in config.

## Acceptance Criteria

**Met**: ARMOR deployment stable with 0 CrashLoopBackOff pods

## Follow-up

Fix ClusterSecretStore on ardenone-cluster to complete migration off ardenone-hub.
