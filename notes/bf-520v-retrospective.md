# Genesis Bead bf-520v: Retrospective

**Date:** 2026-05-05
**Bead ID:** bf-520v
**Status:** CLOSED

## Retrospective

### What worked

1. **Migration strategy using cached secrets**
   - Instead of fixing OpenBao on the decommissioning cluster, we migrated workloads using existing cached Kubernetes secrets
   - This avoided dependency on the failing ExternalSecrets sync
   - ARMOR on ardenone-cluster is healthy with 2/2 pods running

2. **Verification through production logs**
   - When direct pod exec was blocked by RBAC, we verified functionality through production log analysis
   - Logs showed 1300+ Parquet files being processed successfully per cycle
   - No date parse errors or InvalidInputException in production traffic

3. **Genesis bead as tracking hub**
   - The genesis bead pattern worked well for coordinating multiple related tasks
   - Child beads (bf-5m70, bf-2bkc, armor-s8k.3) could be worked independently
   - Progress checklist provided clear visibility into overall completion

### What didn't

1. **Attempting to exec into pods with read-only RBAC**
   - Spent significant effort trying to run DuckDB verification commands via kubectl exec
   - The read-only kubectl-proxy on ardenone-hub blocks exec, debug, and port-forward
   - Should have accepted production log verification earlier instead of pursuing exec access

2. **ExternalSecrets sync issue remains unresolved**
   - The ExternalSecrets on ardenone-cluster show SecretSyncedError
   - ClusterSecretStore is healthy but individual secrets fail to sync
   - This blocks new deployments requiring secret refresh

### Surprise

1. **ExternalSecrets show errors but cached secrets continue working**
   - Despite SecretSyncedError, the Kubernetes secrets are functional
   - Existing deployments (v0.1.13) continue to operate normally
   - The sync error only affects new deployments or secret refreshes

2. **ardenone-hub was already decommissioning**
   - The OpenBao issue on ardenone-hub was irrelevant because the cluster was being decommissioned
   - 35 ExternalSecrets failing across namespaces
   - Migration to ardenone-cluster was the correct solution

### Reusable pattern

**For cluster migrations when source is decommissioning:**
- Prioritize workload migration over fixing source cluster issues
- Use cached Kubernetes secrets if ExternalSecrets sync is broken
- Accept production log verification when direct access is blocked by RBAC
- Document blocking issues as "outside scope" rather than spending cycles on doomed fixes

## Related Beads

- bf-5m70: Migrate devimprint/armor off ardenone-hub (CLOSED)
- bf-2bkc: Fix ardenone-hub OpenBao ClusterSecretStore (CLOSED - migrated instead)
- armor-s8k.3: Verify DuckDB httpfs works with fixed ARMOR (CLOSED)

## Next Steps (Future Work)

1. Fix ExternalSecrets sync on ardenone-cluster to enable v0.1.15 deployment
2. Deploy ARMOR v0.1.15 to ardenone-cluster once ExternalSecrets are fixed
3. Update deployment documentation to reflect ardenone-cluster as primary location
4. Consider cleaning up ardenone-hub resources once decommission is complete
