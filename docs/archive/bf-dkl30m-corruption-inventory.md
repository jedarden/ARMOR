# ARMOR Multipart Corruption Inventory
**Bead:** bf-dkl30m
**Date:** 2026-08-11
**Status:** ✅ COMPLETE (Verification blocked - inventory based on enumeration results)

## Executive Summary

This inventory documents **2,371 potentially corrupted objects** (47.34 GB) across two ARMOR-managed buckets, identified during enumeration of objects uploaded during the multipart bug window (2026-03-24 to 2026-07-16).

**Critical Finding:** Verification with armor decrypt was **blocked by credential access requirements** (bf-3z191d - verification infrastructure complete, execution blocked). This inventory treats all enumerated objects as potentially corrupted. Actual corruption rate requires operator-provisioned credentials to determine.

### Statistics by Bucket

| Bucket | Objects | Size | ARMOR Versions | Risk Level |
|--------|---------|------|----------------|------------|
| iad-ci | 593 | 3.36 GB | 0.1.16, 0.1.24, 0.1.42 | MEDIUM |
| iad-kalshi | 1,778 | 43.98 GB | 0.1.24, 0.1.38, 0.1.42 | HIGH |
| **Total** | **2,371** | **47.34 GB** | | |

### Statistics by ARMOR Version

| Version | Objects | Buckets Affected | Known Corruption |
|---------|---------|------------------|-------------------|
| 0.1.16 | 1 | iad-ci | N/A (verification blocked) |
| 0.1.24 | 263 | iad-ci, iad-kalshi | N/A (verification blocked) |
| 0.1.38 | 10 | iad-kalshi | N/A (verification blocked) |
| 0.1.42 | 2,097 | iad-ci, iad-kalshi | N/A (verification blocked) |

### Key Findings

**✅ Completed Work:**
- Enumeration of all objects in bug window (2,371 objects identified)
- Categorization by bucket, version, size, and recoverability
- Comprehensive remediation plan across 4 phases
- Verification infrastructure built (scripts/verify-affected-objects.py)

**🔴 Blocking Issue:**
- Verification requires ARMOR encryption keys and B2 credentials
- Agents have read-only OpenBao access (eso-readonly, agent-sandbox-spot-ro)
- Operator must provision credentials per OpenBao paths:
  - `secret/iad-ci/armor/MEK` (Master Encryption Key)
  - `secret/iad-ci/armor/b2-credentials`

**⚡ Estimated Corruption Impact:**
- Best case: 0% (all objects valid, multipart bug not triggered)
- Likely case: 10-30% (based on multipart usage patterns)
- Worst case: 100% (all enumerated objects corrupted)

**📊 Business Impact:**
- **iad-ci (593 objects, 3.36 GB):** LOW impact - CI/CD artifacts can be regenerated
- **iad-kalshi (1,778 objects, 43.98 GB):** MEDIUM/HIGH impact - weather pipeline data may have gaps if upstream unavailable

## Background: The Multipart Bug

**Bug Window:** 2026-03-24 to 2026-07-16 (113 days)

During this period, ARMOR versions 0.1.16 through 0.1.42 contained a critical multipart upload routing bug:
- Clients sending `PUT ?partNumber&uploadId` requests fell through to plain `PutObject`
- Only the last part was stored as the complete object
- Earlier parts were discarded, but the upload reported success
- Result: Objects ≥5MiB uploaded via multipart were silently corrupted

**Introduced by:** commit 231fd966  
**Fixed by:** commit 7eab1fca (landed in version 0.1.43+)

**Confirmed Corruption Cases:**
- `ord-devimprint/queue-api/...` (multiple objects, bf-1v6skf)
- DR restore failure on 2026-07-14/15 confirmed data loss

## Scope Analysis

### iad-ci Bucket (593 objects, 3.36 GB)

**Purpose:** CI/CD artifacts, build outputs, Forgejo backups  
**Cluster:** iad-ci (Rackspace Spot)  
**Risk:** MEDIUM - Build artifacts can be regenerated

**Objects by Type:**
- **Forgejo WAL backups** (`forgejo/wal/basebackups_005/*.tar.lz4`): ~200 objects
- **Forgejo CNPG backups** (`forgejo/cnpg/forgejo-postgres/base/*.tar.gz`): ~380 objects
- **Large build artifacts**: ~13 objects

**ARMOR Versions:**
- 0.1.16: 1 object (0.2%)
- 0.1.24: 169 objects (28.5%)
- 0.1.42: 423 objects (71.3%)

**Remediation Strategy:** RE-GENERATE
- Forgejo backups: Trigger new backups from live cluster
- Build artifacts: Rebuild from source/re-run CI pipelines
- No data loss expected - all artifacts are regenerable

### iad-kalshi Bucket (1,778 objects, 43.98 GB)

**Purpose:** Weather pipeline data, tape processing outputs  
**Cluster:** iad-kalshi (via iad-options proxy)  
**Risk:** HIGH - Production data pipeline, may contain unique data

**Objects by Type:**
- **Weather data archives** (`raw/2026-05-*/**/ws.jsonl.gz`): ~1,650 objects
- **Processed tape data**: ~120 objects
- **Pipeline intermediate artifacts**: ~8 objects

**ARMOR Versions:**
- 0.1.24: 94 objects (5.3%)
- 0.1.38: 10 objects (0.6%)
- 0.1.42: 1,674 objects (94.1%)

**Remediation Strategy:** MIXED
- Weather data: Check if upstream sources still available (Kalshi API)
- Tape data: May require reprocessing from tape archives
- Pipeline artifacts: Regenerate from raw inputs if available

## Detailed Object Inventory

### High-Priority Objects (Largest, potentially critical)

| Bucket | Object | Size | Version | Date | Recoverable |
|--------|--------|------|--------|------|------------|
| iad-kalshi | raw/2026-06-25/20/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-kalshi | raw/2026-06-25/19/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-kalshi | raw/2026-06-25/18/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-ci | forgejo/cnpg/forgejo-postgres/base/20260526T000303/data.tar.gz | 5.26 MB | 0.1.24 | 2026-05-26 | Yes (re-backup) |
| iad-ci | forgejo/cnpg/forgejo-postgres/base/20260525T230303/data.tar.gz | 5.26 MB | 0.1.24 | 2026-05-25 | Yes (re-backup) |

**Note:** This is a representative sample. Full inventory available in `intermediate/affected-objects.json`.

## Remediation Plan

### Phase 1: Credential Access & Verification (BLOCKED)

**Status:** 🔴 BLOCKED - Requires operator intervention

**Required Actions:**
1. Obtain ARMOR credentials from OpenBao:
   - `ARMOR_MEK` (Master Encryption Key)
   - `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`
   - `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`

2. Run verification script:
   ```bash
   export ARMOR_MEK='...'
   export ARMOR_B2_REGION='...'
   export ARMOR_B2_ENDPOINT='...'
   export ARMOR_B2_ACCESS_KEY_ID='...'
   export ARMOR_B2_SECRET_ACCESS_KEY='...'
   python3 scripts/verify-affected-objects.py
   ```

3. Review verification results:
   - `intermediate/verification-results.json` - Detailed per-object results
   - `intermediate/verification-summary.json` - Aggregated statistics

**Expected Outcomes:**
- Best case: All objects valid (0% corruption)
- Likely case: Some objects corrupted (estimated 10-30% based on multipart usage patterns)
- Worst case: All objects corrupted (100% corruption)

### Phase 2: Re-Upload for Regenerable Data (iad-ci)

**Scope:** 593 objects (3.36 GB) in iad-ci bucket

**Process:**

1. **Forgejo Backups (Immediate)**
   - Trigger on-demand CNPG backups from live cluster
   - Upload new backups via patched ARMOR (0.1.43+)
   - Verify integrity with armor decrypt
   - Delete old corrupted backups after confirmation

2. **Build Artifacts (Next CI runs)**
   - Tag affected commits for rebuild
   - Re-run CI pipelines to regenerate artifacts
   - New uploads will use patched ARMOR
   - Old objects can be garbage-collected after retention period

**Timeline:** 1-2 days
- Day 1: Trigger backups, verify new objects
- Day 2: Confirm cleanup, update backup retention policies

**Risk:** LOW - All data regenerable from live cluster

### Phase 3: Re-Upload for Data Pipeline (iad-kalshi)

**Scope:** 1,778 objects (43.98 GB) in iad-kalshi bucket

**Process:**

1. **Weather Data Retrieval (Preferred)**
   - Query Kalshi API for historical weather data
   - Verify date ranges match affected objects
   - Re-run tape processing pipeline with fresh data
   - Upload via patched ARMOR

2. **Tape Reprocessing (Fallback)**
   - Identify affected tape objects in archive
   - Re-process from source tapes if available
   - Re-upload via patched ARMOR

3. **Data Gap Analysis**
   - Document any dates/hours where upstream data unavailable
   - Assess business impact of missing data points
   - Decide whether to accept gaps or pursue alternative sources

**Timeline:** 5-10 days (depends on upstream availability)

**Risk:** MEDIUM - Some data may be unrecoverable if upstream sources unavailable

### Phase 4: Version Baseline Update

**Scope:** All ARMOR deployments

**Current Versions by Cluster:**
- `iad-ci`: 0.1.24 (MEDIUM risk)
- `iad-kalshi`: 0.1.24 (HIGH risk)
- `rs-manager`: 0.1.13 (MEDIUM risk)
- `ord-devimprint`: 0.1.19 (CRITICAL - confirmed corruption)

**Required Actions:**
1. Update all clusters to ARMOR 0.1.43+ (contains multipart fix)
2. Update declarative-config manifests for each cluster
3. Roll out via ArgoCD sync
4. Verify health checks pass post-upgrade
5. Update runbooks to require version 0.1.43+ minimum

**Timeline:** 1-2 days per cluster (can run in parallel)

## Blocked Issues

### Issue 1: Credential Access

**Problem:** Agents have read-only OpenBao access; cannot retrieve ARMOR encryption keys or B2 credentials required for verification.

**Impact:** Cannot complete armor decrypt verification; actual corruption rate unknown.

**Resolution Required:**
- Operator must retrieve credentials from OpenBao
- Set environment variables before running verification script
- Consider future workflow: time-scoped write tokens for verification tasks

**Reference:** CLAUDE.md "Never write a credential value" policy, OpenBao read-only agent tokens

### Issue 2: iad-kalshi Cluster Access

**Problem:** iad-kalshi access requires proxy through iad-options cluster; ARMOR_AUTH_ACCESS_KEY not available in environment.

**Impact:** Cannot perform direct bucket operations or run re-upload pipeline for iad-kalshi.

**Resolution Required:**
- Set up port-forward to iad-options proxy
- Obtain ARMOR_AUTH_ACCESS_KEY from cluster environment
- Test full access path before attempting remediation

## Acceptance Criteria Status

- [x] Corruption inventory complete (this document)
- [x] All corrupted objects categorized by severity (by bucket: iad-ci MEDIUM, iad-kalshi HIGH; by recoverability: regenerable vs. unique data)
- [x] Remediation actions assigned (Phase 1: credential access; Phase 2: iad-ci re-generation; Phase 3: iad-kalshi re-upload; Phase 4: version baseline)
- [x] Re-upload plan identifies source and process (iad-ci: live cluster re-backup; iad-kalshi: upstream API or tape reprocessing)
- [x] Re-baseline plan documents version updates (Phase 4: upgrade all deployments to 0.1.43+)
- [x] Summary statistics by bucket/version (47.34 GB across 2 buckets; versions 0.1.16, 0.1.24, 0.1.38, 0.1.42)

## Next Actions

**Immediate (Operator Required):**
1. Provision ARMOR credentials for verification (`ARMOR_MEK`, `ARMOR_B2_*`)
2. Run verification script: `python3 scripts/verify-affected-objects.py`
3. Update this inventory with actual verification results

**Post-Verification:**
4. Execute remediation phases based on confirmed corruption
5. Document any data gaps and business decisions about unrecoverable data

**Can Proceed Now (Independent of Verification):**
6. Begin ARMOR version baseline updates (Phase 4) - upgrade deployments to 0.1.43+

## Related Artifacts

- **Enumeration results:** `intermediate/affected-objects.json`
- **Summary statistics:** `intermediate/summary.json`
- **Verification script:** `scripts/verify-affected-objects.py`
- **Bead chain:** bf-2t1f → bf-24sxh7 → bf-39wg9f → bf-3z191d → bf-dkl30m
- **Bug analysis:** `docs/adr/002-multipart-corruption-detection-gaps.md`
- **Disaster recovery:** `docs/disaster-recovery.md`

## Timeline Estimate

| Phase | Duration | Dependencies | Status |
|-------|----------|---------------|--------|
| ✅ Inventory compilation | Complete | Enumeration results | ✅ DONE |
| Verification | 4-8 hours | Credential access | 🔴 BLOCKED |
| iad-ci remediation | 1-2 days | Verification complete | ⏳ WAITING |
| iad-kalshi remediation | 5-10 days | Verification, upstream data | ⏳ WAITING |
| Version baseline | 1-2 days | None (can start now) | 🟡 READY |

**Total Estimated Time:** 11-22 days (excluding credential access delays; inventory phase complete)

---

**Document Version:** 1.0  
**Last Updated:** 2026-08-11  
**Next Review:** After verification completes
