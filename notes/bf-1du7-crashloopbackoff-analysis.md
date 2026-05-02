# ARMOR CrashLoopBackOff on ardenone-hub - Analysis

## Date
2026-05-02

## Issue Summary
ARMOR deployment on ardenone-hub (devimprint namespace) has one pod in CrashLoopBackOff state due to ExternalSecret sync failure. The ClusterSecretStore `openbao` cannot validate, preventing new pods from loading required secrets (B2 credentials, MEK, auth keys).

## Current State

### ARMOR Deployment
```
NAME                      READY   STATUS             RESTARTS        AGE
armor-755d878c84-l8grt   0/1     CrashLoopBackOff   64 (2m ago)     5h
armor-7c79d57db6-k2j6j   1/1     Running            32 (153m ago)   5h
```

- **Healthy pod**: armor-7c79d57db6-k2j6j (using cached secrets from 19 days ago)
- **Failing pod**: armor-755d878c84-l8grt (cannot get secrets, exits immediately)
- **Service endpoints**: Active (10.42.0.70:9000, 10.42.0.70:9001) - serving via healthy pod

### ClusterSecretStore Status
```
NAME      STATUS   MESSAGE
openbao   False    unable to validate store
```

- OpenBao pod is Running (1/1, 0 restarts)
- ClusterSecretStore cannot validate connection
- Last successful sync: 2d14h ago (secrets are stale)

### ArgoCD Application Status
```
Application: devimprint-ns-ardenone-hub
Sync Status: OutOfSync
Health: Degraded
```

Multiple resources OutOfSync including:
- Deployment/armor
- ExternalSecrets (all devimprint secrets)

## Root Cause

1. **OpenBao ClusterSecretStore failure**: The ClusterSecretStore cannot validate, causing all ExternalSecret syncs to fail
2. **Rolling update stuck**: A deployment update created a new ReplicaSet, but the new pod cannot start without fresh secrets
3. **No cached secrets for new pod**: The healthy pod uses secrets cached from 19 days ago; new pods need fresh secrets that can't be synced

## Access Constraints

| Method | Status | Issue |
|--------|--------|-------|
| ardenone-hub proxy (traefik-ardenone-hub:8001) | Read-only | RBAC blocks all write operations including pod deletion |
| ArgoCD read-only API | Read-only | Cannot modify applications or resources |
| ord-devimprint.kubeconfig | Expired | OIDC token expired, requires browser re-auth |
| ardenone-hub write kubeconfig | Missing | No write-access kubeconfig exists |

## Service Impact

- **Current status**: Service is functional via the healthy pod with cached secrets
- **Risk**: If the healthy pod fails, no new pods can start (no secret access)
- **Data freshness**: Secrets are 19+ days old (last successful sync)

## Resolution Options

### Option 1: Immediate Mitigation (Blocked - No Write Access)
Delete the failing ReplicaSet to stabilize the deployment:
```bash
kubectl --server=ardenone-hub --write-access delete replicaset armor-755d878c84 -n devimprint
```
**Status**: BLOCKED - requires write access to ardenone-hub

### Option 2: Fix OpenBao ClusterSecretStore (Not Recommended - Decommissioning)
Investigate and fix OpenBao connectivity:
- Check OpenBao logs for errors
- Verify ESO token validity
- Test ClusterSecretStore connection
**Status**: NOT RECOMMENDED - ardenone-hub is targeted for shutdown

### Option 3: Migrate to ord-devimprint (Recommended - Long Term)
Move devimprint workloads off ardenone-hub:
- ord-devimprint cluster exists (Rackspace Spot)
- Requires refreshed kubeconfig (OIDC token expired)
- Migration would resolve this and future issues
**Status**: RECOMMENDED - aligns with decommissioning plan

### Option 4: Manual Secret Replication (Workaround)
Manually copy secrets from another cluster:
- Export secrets from a healthy cluster
- Apply directly to ardenone-hub (requires write access)
**Status**: BLOCKED - requires write access

## Recommended Path Forward

1. **Short term**: Service remains functional via healthy pod with cached secrets
2. **Medium term**: Refresh ord-devimprint.kubeconfig OIDC credentials
3. **Long term**: Complete migration of devimprint workloads to ord-devimprint

## ArgoCD Application Details

- **Repository**: jedarden/declarative-config
- **Path**: k8s/ardenone-hub/devimprint
- **Target**: https://ardenone-hub.ardenone.com:6443
- **Revision**: 9cc7598e365fd52a21e9671234cc2846b784f113

## Required Actions to Unblock

1. **Create write-access kubeconfig for ardenone-hub** (for immediate mitigation)
2. **Refresh ord-devimprint.kubeconfig OIDC token** (for migration)
3. **Execute migration plan** (long-term resolution)

## Acceptance Criteria

The bead requires "ARMOR deployment stable with 0 CrashLoopBackOff pods."

**Current status**: NOT MET - 1 CrashLoopBackOff pod exists
**Blocker**: Cannot modify resources on ardenone-hub (read-only access only)

## Conclusion

This is an infrastructure access issue, not a code issue. The ARMOR service is functional despite the CrashLoopBackOff pod, but the deployment cannot be fully stabilized without write access to ardenone-hub or migration to another cluster.

Given that ardenone-hub is targeted for shutdown, the recommended resolution is migration rather than investing in fixing the OpenBao/ExternalSecret issue on this cluster.
