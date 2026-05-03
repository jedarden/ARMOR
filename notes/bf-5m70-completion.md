# bf-5m70: ARMOR Migration from ardenone-hub to ardenone-cluster

## Status: COMPLETE

## Migration Summary

The devimprint ARMOR (encrypted S3 proxy) has been successfully migrated from ardenone-hub to ardenone-cluster.

### Before Migration
- **Cluster**: ardenone-hub (being decommissioned)
- **ARMOR Status**: Deployment scaled to 0 replicas
- **Endpoint**: https://devimprint-armor-tailscale-ingress.tail1b1987.ts.net

### After Migration
- **Cluster**: ardenone-cluster
- **ARMOR Status**: 2x Running pods (armor-68c6ddc78b-27cq6, armor-68c6ddc78b-6krfq)
- **Endpoint**: https://devimprint-armor-tailscale-ingress.tail1b1987.ts.net (same Tailscale ingress)
- **Image**: ronaldraygun/armor:0.1.13

### Aggregator Status
- **Location**: Still on ardenone-hub (will be migrated separately)
- **S3 Endpoint**: Points to ARMOR via Tailscale ingress
- **Processing**: 76,361 rows/cycle (working correctly)

### Verification
- ARMOR health endpoint: OK
- Aggregator logs show successful uploads to ARMOR on ardenone-cluster
- DuckDB httpfs working through ARMOR

### Declarative Config
- ARMOR deployment exists in: /home/coding/declarative-config/k8s/ardenone-cluster/devimprint/armor-deployment.yml
- ARMOR secrets on ardenone-hub reference the migration (see armor-secrets.yml)

### Cleanup Required
The following resources remain on ardenone-hub but are not actively serving traffic:
- Deployment: armor (0 replicas)
- Services: armor, armor-svc
- ReplicaSets (old versions)

These resources are managed by ArgoCD and should be pruned when the devimprint-ns-ardenone-hub application syncs with the updated declarative-config.

### Notes
- The armor namespace on ardenone-hub contains a separate ARMOR deployment (1/1 Running) which is unrelated to devimprint and was not migrated.
- OpenBao ExternalSecrets on ardenone-hub are failing (SecretSyncedError) but this does not affect the migrated ARMOR on ardenone-cluster.

## Completion Date: 2026-05-02
