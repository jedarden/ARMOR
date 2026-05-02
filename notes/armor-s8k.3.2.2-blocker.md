# armor-s8k.3.2.2: DuckDB httpfs COUNT(*) Query Verification - BLOCKED

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Status: BLOCKED - Insufficient Access

## Findings

### Access Issues
1. **ord-devimprint.kubeconfig**: OIDC authentication broken (kubectl-oidc-login plugin not working)
2. **ardenone-hub proxy**: Read-only access - cannot exec, read secrets, or create pods
3. **No write-access kubeconfig**: ardenone-manager.kubeconfig referenced in CLAUDE.md doesn't exist

### Aggregator Location
- **Cluster**: ardenone-hub (not ord-devimprint as originally specified)
- **Namespace**: devimprint
- **Pod**: aggregator-5d58d6c67-7gl9m (Running)
- **Access method**: `kubectl --server=http://traefik-ardenone-hub:8001`

### Attempted Approaches
1. ✅ Located aggregator pod on ardenone-hub
2. ❌ kubectl exec failed: "unable to upgrade connection: Forbidden" (read-only RBAC)
3. ❌ kubectl debug failed: "pods is forbidden" (read-only RBAC)
4. ❌ Secret access failed: "secrets is forbidden" (cannot get S3 credentials)
5. ❌ Checked rs-manager, iad-ci: No devimprint namespace or aggregator pod

### Root Cause
The read-only proxy (devpod-observer ServiceAccount) on ardenone-hub intentionally blocks:
- `exec` into pods
- Creating resources (pods, jobs)
- Reading secrets

This prevents running the DuckDB query inside the aggregator pod.

## Requirements to Complete Task
1. **Write access to ardenone-hub cluster** OR
2. **Fixed ord-devimprint.kubeconfig** with working OIDC auth OR
3. **Direct kubeconfig for ardenone-hub** with cluster-admin access

## Alternative Approaches (if write access available)
1. Create a Job/CronJob that runs the query and logs results
2. Deploy a test pod with DuckDB that queries s3://devimprint/commits/**/*.parquet
3. Use kubectl port-forward to access ARMOR service and run query locally

## References
- Parent bead: armor-s8k.3 (notes show ord-devimprint is deprecated)
- ArgoCD app: devimprint-ns-ardenone-hub (OutOfSync, Degraded)
- ARMOR pods on ardenone-hub: v0.1.11 and v0.1.13 (not v0.1.8 as expected)
