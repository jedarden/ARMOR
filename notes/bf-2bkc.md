# bf-2bkc: ardenone-hub OpenBao ClusterSecretStore - Resolution

## Date: 2026-05-02

## Task
Fix ardenone-hub OpenBao ClusterSecretStore — all ExternalSecrets in SecretSyncedError

## Root Cause
OpenBao ClusterSecretStore 'openbao' on ardenone-hub has been failing since 2026-04-28T18:26:50Z with:
```
invalid vault credentials: Error making API request.
URL: GET http://openbao-ardenone-hub.openbao.svc.cluster.local:8200/v1/auth/token/lookup-self
Code: 403. Errors: * permission denied
```

The ESO token (`openbao-eso-token` in `external-secrets` namespace) returns 403 permission denied, indicating either:
1. Token expired
2. Policy changed/revoked
3. OpenBao configuration issue

## Resolution: NO FIX REQUIRED - Workloads Migrated

### Investigation Findings

1. **ardenone-hub Cluster Status:**
   - 35 ExternalSecrets in SecretSyncedError state
   - Most pods failing (OffloadingBackOff, Pending, Error states)
   - Cluster is being decommissioned

2. **ardenone-cluster Migration Status:**
   - ARMOR pods: 2/2 Running healthy (4h48m uptime at time of check)
   - Secrets created: `armor-credentials`, `armor-readonly`, `armor-writer` (28m old)
   - OpenBao ClusterSecretStore: **Ready: True** with "store validated"
   - All ARMOR workloads successfully migrated

### Acceptance Criteria Met

The task acceptance criteria: "Either OpenBao ClusterSecretStore healthy (Ready=True) OR devimprint workloads migrated off hub"

**Result:** ✅ **Devimprint workloads migrated off hub**

- ARMOR is fully operational on ardenone-cluster
- Secrets are available
- No further action required on ardenone-hub

### Technical Details

**ardenone-cluster (MIGRATED - HEALTHY):**
```
kubectl --server=http://traefik-ardenone-cluster:8001 get pods -n devimprint
NAME                     READY   STATUS    RESTARTS   AGE
armor-68c6ddc78b-27cq6   1/1     Running   0          4h48m
armor-68c6ddc78b-6krfq   1/1     Running   0          4h48m

kubectl --server=http://traefik-ardenone-cluster:8001 get clustersecretstore openbao
STATUS: Ready=True, message="store validated"
```

**ardenone-hub (DECOMMISSIONING - BROKEN):**
```
kubectl --server=http://traefik-ardenone-hub:8001 get clustersecretstore openbao
STATUS: Ready=False, reason=InvalidProviderConfig, message="unable to validate store"

35 ExternalSecrets failing across namespaces
```

## Related Work
- bf-5m70: ARMOR secret migration from ardenone-hub to ardenone-cluster
- Migration documented in notes/bf-5m70-secret-migration.md

## Conclusion
No action required. The OpenBao issue on ardenone-hub is irrelevant because the cluster is being decommissioned and all critical workloads (ARMOR) have been successfully migrated to ardenone-cluster.
