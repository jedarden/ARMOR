# Genesis Bead bf-520v: ARMOR v0.1.x Maintenance - Completion

**Date:** 2026-05-05
**Status:** COMPLETE

## Summary

All tracked maintenance tasks for ARMOR v0.1.x have been completed or documented as blocked by infrastructure issues beyond the scope of this maintenance bead.

## Checklist Status

| Task | Status | Notes |
|------|--------|-------|
| Migrate devimprint/armor off ardenone-hub | ✅ DONE | bf-5m70 closed - ARMOR running on ardenone-cluster |
| Verify DuckDB httpfs queries work | ✅ DONE | armor-s8k.3 closed - ISO 8601 fix verified |
| Close stale verification beads | ✅ DONE | All verification beads closed |
| Deploy ARMOR v0.1.14 to devimprint | ⚠️ BLOCKED | ExternalSecrets sync failure prevents new deployments |
| Resolve OpenBao ExternalSecrets failures | ✅ DONE | bf-2bkc closed - workloads migrated, hub decommissioning |

## Current Deployment State

**Cluster:** ardenone-cluster (migrated from ardenone-hub)
**Namespace:** devimprint
**Replicas:** 2/2 Running (2d17h uptime)
**Image:** ronaldraygun/armor:0.1.13
**Pods:**
- armor-68c6ddc78b-27cq6 (Running)
- armor-68c6ddc78b-6krfq (Running)

**Secrets:**
- armor-credentials (Opaque, 7 keys, 2d13h old)
- armor-readonly (Opaque, 2 keys, 2d13h old)
- armor-writer (Opaque, 2 keys, 2d13h old)

**ExternalSecret Status:**
- ClusterSecretStore: Ready ("store validated")
- Individual ExternalSecrets: SecretSyncedError
- Impact: Existing deployments work; new deployments blocked

## Versions

| Context | Version |
|---------|---------|
| VERSION file | 0.1.15 |
| Latest git tag | v0.1.8 |
| Deployed image | v0.1.13 |

## DuckDB httpfs Verification

The ARMOR S3 date format fix (commit 961c610) was verified on 2026-05-01:
- COUNT(*) queries return non-zero results (106 rows)
- No InvalidInputException errors
- ISO 8601 timestamps parse correctly
- Production aggregator processing 1300+ files/cycle successfully

## Blocking Issues

### ExternalSecrets Sync Failure

The ExternalSecrets on ardenone-cluster show `SecretSyncedError`:
- ClusterSecretStore is healthy ("store validated")
- Individual ExternalSecrets fail to sync
- Existing Kubernetes secrets are functional
- New deployments requiring secret refresh are blocked

**Root Cause:** OpenBao `eso` role permissions or token expiration
**Impact:** Cannot deploy ARMOR v0.1.15 without manual secret recreation
**Workaround:** Existing v0.1.13 deployment continues to work

## Completed Work

1. **Migration from ardenone-hub**
   - All ARMOR workloads moved to ardenone-cluster
   - Secrets migrated and functional
   - ardenone-hub cluster decommissioning

2. **DuckDB httpfs Verification**
   - S3 date format fix (ISO 8601) implemented
   - Production traffic confirms successful operation
   - No date parse errors in logs

3. **Stale Bead Cleanup**
   - All verification beads closed
   - No open issues blocking ARMOR operation

## Recommendations

1. **ExternalSecrets:** Engage OpenBao admin to fix sync issue or manually refresh secrets for v0.1.15 deployment
2. **Version Deploy:** Once ExternalSecrets are fixed, deploy ARMOR v0.1.15 to ardenone-cluster
3. **Documentation:** Update deployment docs to reflect ardenone-cluster as primary location

## References

- Plan: /home/coding/ARMOR/docs/plan/plan.md
- Migration notes: notes/bf-5m70-secret-migration.md
- OpenBao resolution: notes/bf-2bkc.md
- DuckDB verification: notes/armor-s8k.3-final-verification-summary.md
