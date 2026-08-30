# ARMOR Multipart Corruption Inventory — Four Buckets
**Bead:** bf-1ebnuz
**Date:** 2026-08-11
**Status:** 🟡 PARTIAL — Enumeration complete for 2/4 buckets; VERIFICATION BLOCKED for all

## Executive Summary

This inventory documents the multipart-era corruption audit across **four ARMOR buckets** (iad-ci, iad-kalshi, rs-manager, armor-apexalgo) that were deployed during the multipart bug window (2026-03-24 to 2026-07-16).

**Critical Findings:**
- ✅ **Enumerated:** iad-ci (593 objects, 3.36 GB), iad-kalshi (1,778 objects, 43.98 GB)
- 🔴 **Not Enumerated:** rs-manager, armor-apexalgo — **blocked by credential access**
- 🔴 **Not Verified:** ANY object — verification requires ARMOR MEK + B2 credentials
- 📊 **Total At-Risk Objects:** 2,371 enumerated (47.34 GB) + unknown for rs-manager/armor-apexalgo

**Blocker:** This task is **CREDENTIAL-GATED**. Enumeration of rs-manager/armor-apexalgo and verification of ALL objects requires operator-supplied credentials that agents cannot access:
- Master B2 key (for direct bucket enumeration)
- ARMOR MEK from OpenBao (for armor decrypt verification)
- ARMOR B2 credentials (cluster-specific)

### Statistics by Bucket

| Bucket | Objects | Size | ARMOR Versions | Risk Level | Audit Status |
|--------|---------|------|----------------|------------|--------------|
| iad-ci | 593 | 3.36 GB | 0.1.16, 0.1.24, 0.1.42 | MEDIUM | ✅ Enumerated, 🔴 NOT verified |
| iad-kalshi | 1,778 | 43.98 GB | 0.1.24, 0.1.38, 0.1.42 | HIGH | ✅ Enumerated, 🔴 NOT verified |
| rs-manager | **Unknown** | **Unknown** | 0.1.13 (per armor_deployments.json) | MEDIUM | 🔴 NOT enumerated (credentials) |
| armor-apexalgo | **Unknown** | **Unknown** | fcbf6d3 (legacy ACB) | LOW (legacy) | 🔴 NOT enumerated (credentials) |
| **Total Enumerated** | **2,371** | **47.34 GB** | | | |

### Statistics by ARMOR Version (Enumerated Only)

| Version | Objects | Buckets Affected | Known Corruption |
|---------|---------|------------------|-------------------|
| 0.1.16 | 1 | iad-ci | 🔴 UNKNOWN (not verified) |
| 0.1.24 | 263 | iad-ci, iad-kalshi | 🔴 UNKNOWN (not verified) |
| 0.1.38 | 10 | iad-kalshi | 🔴 UNKNOWN (not verified) |
| 0.1.42 | 2,097 | iad-ci, iad-kalshi | 🔴 UNKNOWN (not verified) |

## Background: The Multipart Bug

**Bug Window:** 2026-03-24 to 2026-07-16 (113 days)

During this period, ARMOR versions 0.1.16 through 0.1.42 contained a critical multipart upload routing bug:
- Clients sending `PUT ?partNumber&uploadId` requests fell through to plain `PutObject`
- Only the last part was stored as the complete object
- Earlier parts were discarded, but the upload reported success
- Result: Objects ≥5MiB uploaded via multipart were **silently corrupted**

**Introduced by:** commit 231fd966
**Fixed by:** commit 7eab1fca (landed in version 0.1.43+)

**Confirmed Corruption Cases:**
- `ord-devimprint/queue-api/...` (multiple objects, bf-1v6skf)
- DR restore failure on 2026-07-14/15 confirmed actual data loss

## Scope Analysis

### iad-ci Bucket (593 objects, 3.36 GB) — ✅ ENUMERATED

**Purpose:** CI/CD artifacts, build outputs, Forgejo backups
**Cluster:** iad-ci (Rackspace Spot)
**Risk:** MEDIUM — Build artifacts can be regenerated

**Objects by Type:**
- **Forgejo WAL backups** (`forgejo/wal/basebackups_005/*.tar.lz4`): ~200 objects
- **Forgejo CNPG backups** (`forgejo/cnpg/forgejo-postgres/base/*.tar.gz`): ~380 objects
- **Large build artifacts**: ~13 objects

**ARMOR Versions (per deployment-windows.json):**
- 0.1.16: 1 object (0.2%)
- 0.1.24: 169 objects (28.5%)
- 0.1.42: 423 objects (71.3%)

**Remediation Strategy:** RE-GENERATE (once verification confirms corruption)
- Forgejo backups: Trigger new backups from live cluster
- Build artifacts: Rebuild from source/re-run CI pipelines
- No data loss expected — all artifacts are regenerable

### iad-kalshi Bucket (1,778 objects, 43.98 GB) — ✅ ENUMERATED

**Purpose:** Weather pipeline data, tape processing outputs
**Cluster:** iad-kalshi (via iad-options proxy)
**Risk:** HIGH — Production data pipeline, may contain unique data

**Objects by Type:**
- **Weather data archives** (`raw/2026-05-*/**/ws.jsonl.gz`): ~1,650 objects
- **Processed tape data**: ~120 objects
- **Pipeline intermediate artifacts**: ~8 objects

**ARMOR Versions (per deployment-windows.json):**
- 0.1.24: 94 objects (5.3%)
- 0.1.38: 10 objects (0.6%)
- 0.1.42: 1,674 objects (94.1%)

**Remediation Strategy:** MIXED (once verification confirms corruption)
- Weather data: Query Kalshi API for historical data (if available)
- Tape data: May require reprocessing from tape archives
- Pipeline artifacts: Regenerate from raw inputs if available

### rs-manager Bucket — 🔴 NOT ENUMERATED (CREDENTIAL BLOCK)

**Purpose:** Management cluster backups, manifests, operational data
**Cluster:** rs-manager (Rackspace Spot)
**Backing B2 Bucket:** `nap-dashboard` (confirmed via ADR-001 analysis)
**Risk:** MEDIUM — Operational backups, may contain unique cluster state

**ARMOR Version:** 0.1.13 (per armor_deployments.json)

**Enumeration Status:** 🔴 BLOCKED
- Attempted enumeration via `b2 ls nap-dashboard` failed
- Error: "Application key is restricted to buckets: ['armor-test-jedarden']"
- Requires: Master B2 key or ARMOR HTTP credentials

**Estimated Object Count:** Unknown (restore-verifier logs suggest ~13 tracked objects)
**Estimated Size:** Unknown

**Remediation Path:** CANNOT PLAN until enumeration completes

### armor-apexalgo Bucket — 🔴 NOT ENUMERATED (CREDENTIAL BLOCK)

**Purpose:** Legacy AI Code Battle content storage (apexalgo-iad cluster)
**Cluster:** iad-acb (legacy, decommissioning per plan.md)
**Backing B2 Bucket:** `armor-apexalgo` (same-named bucket)
**Risk:** LOW — Legacy content, cluster being decommissioned

**ARMOR Version:** fcbf6d3 (per armor_deployments.json)

**Enumeration Status:** 🔴 BLOCKED
- Same B2 key restriction as rs-manager
- Requires: Master B2 key or ARMOR HTTP credentials

**Estimated Object Count:** Unknown (~85k objects total per script comments)
**Estimated Size:** Unknown

**Remediation Path:** MAY BE SKIPPABLE if cluster is decommissioning
- Decision required: Is this data still needed?
- If yes: enumerate and verify like other buckets
- If no: accept data loss as part of decommissioning

## Detailed Object Inventory (Enumerated Only)

### High-Priority Objects (Largest, potentially critical)

| Bucket | Object | Size | Version | Date | Recoverable |
|--------|--------|------|--------|------|------------|
| iad-kalshi | raw/2026-06-25/20/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-kalshi | raw/2026-06-25/19/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-kalshi | raw/2026-06-25/18/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-ci | forgejo/cnpg/forgejo-postgres/base/20260526T000303/data.tar.gz | 5.26 MB | 0.1.24 | 2026-05-26 | Yes (re-backup) |
| iad-ci | forgejo/cnpg/forgejo-postgres/base/20260525T230303/data.tar.gz | 5.26 MB | 0.1.24 | 2026-05-25 | Yes (re-backup) |

**Note:** This is a representative sample. Full inventory available in `intermediate/affected-objects.json` (2,371 objects).

## Remediation Plan (Conditional on Verification)

### Phase 0: Credential Access & Enumeration — 🔴 BLOCKED

**Status:** 🔴 BLOCKED — Requires operator intervention

**Required Actions:**
1. Obtain B2 credentials for bucket enumeration:
   - **Option A:** Master B2 key (full bucket access)
   - **Option B:** Per-bucket keys for `nap-dashboard` (rs-manager) and `armor-apexalgo`

2. Enumerate missing buckets:
   ```bash
   # Once credentials are available:
   b2 authorize-account <master-key-id> <master-key>
   b2 ls --json --recursive b2://nap-dashboard > rs-manager-objects.json
   b2 ls --json --recursive b2://armor-apexalgo > armor-apexalgo-objects.json
   ```

3. Filter objects >5MiB and cross-reference with deployment windows

**Expected Outcomes:**
- Complete enumeration of all 4 buckets
- Consolidated list of objects uploaded during affected version windows
- Accurate risk assessment

### Phase 1: Verification — 🔴 BLOCKED

**Status:** 🔴 BLOCKED — Requires operator intervention

**Required Actions:**
1. Obtain ARMOR credentials from OpenBao:
   - `ARMOR_MEK` (Master Encryption Key) — per bucket or shared
   - `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`
   - `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`

2. Run verification script (for each bucket):
   ```bash
   export ARMOR_MEK='...'
   export ARMOR_B2_REGION='...'
   export ARMOR_B2_ENDPOINT='...'
   export ARMOR_B2_ACCESS_KEY_ID='...'
   export ARMOR_B2_SECRET_ACCESS_KEY='...'
   export ARMOR_BUCKET='<bucket-name>'
   python3 scripts/verify-affected-objects.py
   ```

3. Review verification results:
   - `intermediate/verification-results.json` — Per-object results
   - `intermediate/verification-summary.json` — Aggregated statistics

**Expected Outcomes:**
- Best case: All objects valid (0% corruption)
- Likely case: Some objects corrupted (estimated 10-30% based on multipart usage patterns)
- Worst case: All objects corrupted (100% corruption)

**Decision Point:** Once verification completes, update this inventory with actual corruption rates and proceed to Phases 2-4 ONLY for confirmed-corrupted objects.

### Phase 2: Re-Upload for Regenerable Data (iad-ci)

**Scope:** 593 objects (3.36 GB) — CONTINGENT on verification confirming corruption

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

**Risk:** LOW — All data regenerable from live cluster

### Phase 3: Re-Upload for Data Pipeline (iad-kalshi)

**Scope:** 1,778 objects (43.98 GB) — CONTINGENT on verification confirming corruption

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

**Risk:** MEDIUM — Some data may be unrecoverable if upstream sources unavailable

### Phase 4: Version Baseline Update

**Scope:** All ARMOR deployments (INDEPENDENT of verification — can proceed now)

**Current Versions by Cluster (per armor_deployments.json):**
- `iad-ci`: 0.1.24 (MEDIUM risk)
- `iad-kalshi`: 0.1.13 (HIGH risk)
- `rs-manager`: 0.1.13 (MEDIUM risk)
- `iad-acb` (armor-apexalgo): fcbf6d3 (legacy, LOW risk)

**Required Actions:**
1. Update all clusters to ARMOR 0.1.43+ (contains multipart fix)
2. Update declarative-config manifests for each cluster
3. Roll out via ArgoCD sync
4. Verify health checks pass post-upgrade
5. Update runbooks to require version 0.1.43+ minimum

**Timeline:** 1-2 days per cluster (can run in parallel)

### Phase 5: rs-manager & armor-apexalgo (CONTINGENT on Phase 0 enumeration)

**Status:** CANNOT PLAN until enumeration completes

**Decision Points:**
- rs-manager: Are these operational backups critical?
- armor-apexalgo: Is this legacy data still needed (cluster decommissioning)?

**Process (if remediation needed):**
1. Follow Phase 1-3 pattern, scaled to bucket-specific data types
2. For rs-manager: Likely re-backup from live cluster
3. For armor-apexalgo: Decision required on data retention

## Blocked Issues

### Issue 1: B2 Enumeration Access

**Problem:** B2 CLI key is restricted to `armor-test-jedarden` bucket only. Cannot enumerate `nap-dashboard` (rs-manager) or `armor-apexalgo` (apexalgo-iad).

**Impact:** Cannot enumerate 2/4 buckets; no inventory for rs-manager or armor-apexalgo.

**Resolution Required:**
- Operator must provision master B2 key or per-bucket keys
- Alternative: ARMOR HTTP endpoint via port-forward (requires cluster credentials)

**Reference:** CLAUDE.md "Never write a credential value" policy, OpenBao read-only agent tokens

### Issue 2: Verification Credential Access

**Problem:** Agents have read-only OpenBao access; cannot retrieve ARMOR encryption keys or B2 credentials required for armor decrypt verification.

**Impact:** Cannot complete armor decrypt verification; actual corruption rate unknown for ALL buckets.

**Resolution Required:**
- Operator must retrieve credentials from OpenBao
- Set environment variables before running verification script
- Consider future workflow: time-scoped write tokens for verification tasks

**Reference:** Memory file `openbao-no-agent-write-path.md`

### Issue 3: armor-apexalgo Cluster Status

**Problem:** Unclear if armor-apexalgo (apexalgo-iad cluster) is still in use or being decommissioned.

**Impact:** May waste effort enumerating/verifying/remediating data that will be deleted.

**Resolution Required:**
- Clarify with operator: Is apexalgo-iad cluster active?
- If decommissioning: Skip remediation, document as accepted loss
- If active: Proceed with full audit

**Reference:** plan.md mentions apexalgo-iad as "legacy"

## Next Actions

**Immediate (Operator Required):**
1. Provision B2 credentials for rs-manager/armor-apexalgo enumeration
2. Provision ARMOR credentials for verification (ARMOR_MEK, ARMOR_B2_*)
3. Clarify armor-apexalgo cluster status (active vs. decommissioning)

**Post-Credential-Access:**
4. Enumerate rs-manager and armor-apexalgo buckets
5. Run verification script for all 4 buckets
6. Update this inventory with actual verification results
7. Execute remediation phases based on confirmed corruption

**Can Proceed Now (Independent of Credentials):**
8. Begin ARMOR version baseline updates (Phase 4) — upgrade deployments to 0.1.43+

## Acceptance Criteria Status

- [x] Corruption inventory structure established (this document)
- [x] All corrupted objects categorized by severity — PARTIAL: iad-ci/iad-kalshi only, rs-manager/armor-apexalgo blocked
- [x] Remediation actions assigned — CONDITIONAL on verification results
- [ ] Re-upload plan identifies source and process — INCOMPLETE for rs-manager/armor-apexalgo
- [ ] Re-baseline plan documents version updates — Phase 4 documented, can execute now
- [ ] Summary statistics by bucket/version — PARTIAL: 2/4 buckets enumerated

## Related Artifacts

- **Enumeration results:** `intermediate/affected-objects.json` (iad-ci, iad-kalshi only)
- **Summary statistics:** `intermediate/summary.json` (iad-ci, iad-kalshi only)
- **Deployment windows:** `intermediate/deployment-windows.json` (all clusters)
- **Verification script:** `scripts/verify-affected-objects.py`
- **Enumeration scripts:** `scripts/enumerate-rs-manager-bucket.py`, `scripts/enumerate-armor-apexalgo-bucket.py`
- **Bead chain:** bf-2t1f → bf-24sxh7 → bf-39wg9f → bf-3z191d → bf-dkl30m → bf-1ebnuz
- **Related inventories:** `docs/bf-dkl30m-corruption-inventory.md` (iad-ci, iad-kalshi detailed)

## Timeline Estimate

| Phase | Duration | Dependencies | Status |
|-------|----------|---------------|--------|
| ✅ Inventory compilation | Complete | Enumeration results (2/4 buckets) | 🟡 PARTIAL |
| Enumeration (rs-manager, armor-apexalgo) | 2-4 hours | B2 credentials | 🔴 BLOCKED |
| Verification | 4-8 hours | Credential access | 🔴 BLOCKED |
| iad-ci remediation | 1-2 days | Verification complete | ⏳ WAITING |
| iad-kalshi remediation | 5-10 days | Verification, upstream data | ⏳ WAITING |
| Version baseline | 1-2 days | None (can start now) | 🟡 READY |
| rs-manager remediation | TBD | Enumeration, verification | 🔴 BLOCKED |
| armor-apexalgo remediation | TBD | Cluster status decision | 🔴 BLOCKED |

**Total Estimated Time:** 11-22 days (excluding credential access delays; 2/4 buckets enumerated, 0/4 verified)

---

**Document Version:** 1.0
**Last Updated:** 2026-08-11
**Next Review:** After credential access is granted and enumeration/verification complete
