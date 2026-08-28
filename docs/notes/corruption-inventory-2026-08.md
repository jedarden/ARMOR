# ARMOR Multipart Corruption Inventory — August 2026

**Bead:** armor-7ba6cb35
**Date:** 2026-08-28
**Status:** 🔴 BLOCKED — Credential access prevents enumeration and verification

## Executive Summary

This inventory documents the multipart-era corruption audit across **four ARMOR buckets** (iad-ci, iad-kalshi, rs-manager, armor-apexalgo) that were deployed during the multipart bug window (2026-03-24 to 2026-07-16).

**Critical Findings:**
- ✅ **Enumerated:** iad-ci (593 objects), iad-kalshi (1,778 objects) — 2,371 total
- 🔴 **Not Enumerated:** rs-manager, armor-apexalgo — **blocked by B2 credential access**
- 🔴 **Not Verified:** ANY object — verification requires ARMOR MEK + B2 credentials from OpenBao
- 📊 **Total At-Risk Objects:** 2,371 enumerated (47.34 GB) + unknown for rs-manager/armor-apexalgo

**Blocker:** This task is **CREDENTIAL-GATED**. Verification of ALL objects requires ARMOR encryption keys and B2 credentials that must be fetched from OpenBao, but all access attempts return 403 permission denied.

### Statistics by Bucket

| Bucket | Objects | Size | ARMOR Versions | Risk Level | Audit Status |
|--------|---------|------|----------------|------------|--------------|
| iad-ci | 593 | 3.36 GB | 0.1.16, 0.1.24, 0.1.42 | MEDIUM | ✅ Enumerated, 🔴 NOT verified |
| iad-kalshi | 1,778 | 43.98 GB | 0.1.24, 0.1.38, 0.1.42 | HIGH | ✅ Enumerated, 🔴 NOT verified |
| rs-manager | **Unknown** | **Unknown** | 0.1.13 | MEDIUM | 🔴 NOT enumerated (B2 credentials) |
| armor-apexalgo | **Unknown** | **Unknown** | fcbf6d3 (legacy) | LOW | 🔴 NOT enumerated (B2 credentials) |
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

**Affected Deployment Windows:**
- 0.1.0: 2026-03-28 to 2026-04-23
- 0.1.3: 2026-04-23 to 2026-04-28
- 0.1.8: 2026-04-28 to 2026-05-01
- 0.1.13: 2026-05-01 to 2026-05-09
- 0.1.15-0.1.16: 2026-05-09 (brief)
- 0.1.20-0.1.24: 2026-05-13 to 2026-06-10
- 0.1.37-0.1.38: 2026-06-10 to 2026-06-11
- 0.1.42: 2026-06-11 to 2026-07-16

**Fixed by:** ARMOR 0.1.43+ (contains multipart fix)

## Scope Analysis

### iad-ci Bucket (593 objects, 3.36 GB) — ✅ ENUMERATED, 🔴 NOT VERIFIED

**Purpose:** CI/CD artifacts, build outputs, Forgejo backups
**Cluster:** iad-ci (Rackspace Spot)
**Risk:** MEDIUM — Build artifacts can be regenerated

**Objects by Type:**
- **Forgejo WAL backups** (`forgejo/wal/basebackups_005/*.tar.lz4`): ~200 objects
- **Forgejo CNPG backups** (`forgejo/cnpg/forgejo-postgres/base/*.tar.gz`): ~380 objects
- **Large build artifacts**: ~13 objects

**Remediation Strategy:** RE-GENERATE (once verification confirms corruption)
- Forgejo backups: Trigger new backups from live cluster
- Build artifacts: Rebuild from source/re-run CI pipelines
- No data loss expected — all artifacts are regenerable

### iad-kalshi Bucket (1,778 objects, 43.98 GB) — ✅ ENUMERATED, 🔴 NOT VERIFIED

**Purpose:** Weather pipeline data, tape processing outputs
**Cluster:** iad-kalshi (via iad-options proxy)
**Risk:** HIGH — Production data pipeline, may contain unique data

**Objects by Type:**
- **Weather data archives** (`raw/2026-05-*/**/ws.jsonl.gz`): ~1,650 objects
- **Processed tape data**: ~120 objects
- **Pipeline intermediate artifacts**: ~8 objects

**Remediation Strategy:** MIXED (once verification confirms corruption)
- Weather data: Query Kalshi API for historical data (if available)
- Tape data: May require reprocessing from tape archives
- Pipeline artifacts: Regenerate from raw inputs if available

### rs-manager Bucket — 🔴 NOT ENUMERATED (B2 CREDENTIAL BLOCK)

**Purpose:** Management cluster backups, manifests, operational data
**Cluster:** rs-manager (Rackspace Spot)
**Risk:** MEDIUM — Operational backups, may contain unique cluster state

**ARMOR Version:** 0.1.13

**Enumeration Status:** 🔴 BLOCKED
- Requires B2 credentials for `nap-dashboard` bucket
- Error: "Application key is restricted to buckets: ['armor-test-jedarden']"
- Requires: Master B2 key or ARMOR HTTP credentials

**Estimated Object Count:** Unknown
**Estimated Size:** Unknown

**Remediation Path:** CANNOT PLAN until enumeration completes

### armor-apexalgo Bucket — 🔴 NOT ENUMERATED (B2 CREDENTIAL BLOCK)

**Purpose:** Legacy AI Code Battle content storage (apexalgo-iad cluster)
**Cluster:** iad-acb (legacy, decommissioning per plan.md)
**Risk:** LOW — Legacy content, cluster being decommissioned

**ARMOR Version:** fcbf6d3 (legacy)

**Enumeration Status:** 🔴 BLOCKED
- Same B2 key restriction as rs-manager
- Requires: Master B2 key or ARMOR HTTP credentials

**Estimated Object Count:** Unknown
**Estimated Size:** Unknown

**Remediation Path:** MAY BE SKIPPABLE if cluster is decommissioning
- Decision required: Is this data still needed?

## Verification Blocker

### OpenBao Credential Access

**Problem:** All OpenBao access attempts return 403 permission denied.

**Attempted Access:**
- Endpoints tried: `http://traefik-rs-manager:8200`, `http://traefik-ardenone-cluster:8200`
- Tokens tried: `~/.vault-token`, `~/.vault-token-openbao-v2`, `~/.vault-token-ardenone-cluster`
- All return: `403 permission denied`

**Required Paths for Verification:**
- `secret/rs-manager/iad-ci/armor/MASTER_ENCRYPTION_KEY` — ARMOR MEK
- `secret/rs-manager/iad-ci/b2/iad-ci` — B2 credentials (key_id, application_key)
- Similar paths for iad-kalshi, rs-manager, armor-apexalgo buckets

**Impact:** Cannot complete verification; actual corruption rate unknown for ALL buckets.

**Reference:** Memory file `openbao-no-agent-write-path.md` documents agent read-only access limitation (agents have `eso-readonly` and `agent-sandbox-spot-ro` policies only).

### Tools Available

**✅ Available:**
- `cmd/armor-decrypt/armor-decrypt` — functional, can decrypt with MEK + B2 credentials
- `cmd/verify-objects/main.go` — verification tool source (needs compilation, requires credentials)
- `intermediate/filtered-objects.json` — 2,371 enumerated objects ready for verification
- `intermediate/deployment-windows.json` — affected version windows

**❌ Blocked:**
- No OpenBao access to retrieve MEK or B2 credentials
- No escrow files found on system (`find /home/coding -name "*.escrow"` returns empty)
- Environment variables not set (`ARMOR_MEK`, `ARMOR_B2_*` all empty)

## Detailed Object Inventory (Enumerated Only)

### Sample High-Value Objects

| Bucket | Object | Size | Version | Date | Recoverable |
|--------|--------|------|--------|------|------------|
| iad-kalshi | raw/2026-06-25/20/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-kalshi | raw/2026-06-25/19/ws.jsonl.gz | 10.2 MB | 0.1.42 | 2026-06-25 | Maybe (API) |
| iad-ci | forgejo/cnpg/forgejo-postgres/base/20260526T000303/data.tar.gz | 5.26 MB | 0.1.24 | 2026-05-26 | Yes (re-backup) |
| iad-ci | forgejo/cnpg/forgejo-postgres/base/20260525T230303/data.tar.gz | 5.26 MB | 0.1.24 | 2026-05-25 | Yes (re-backup) |

**Note:** This is a representative sample. Full inventory available in `intermediate/filtered-objects.json` (2,371 objects).

## Remediation Plan (Conditional on Verification)

### Phase 0: Credential Access — 🔴 BLOCKED

**Status:** 🔴 BLOCKED — Requires operator intervention

**Required Actions:**
1. Operator retrieves ARMOR credentials from OpenBao:
   - `ARMOR_MEK` (Master Encryption Key) — per bucket or shared
   - `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`
   - `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`

2. Operator provides credentials via one of:
   - Escrow file: JSON with MEK + B2 config (loaded by verify-objects)
   - Environment variables: Set for verification process only
   - Time-scoped write token: Agent can fetch directly

**Expected Outcomes:**
- Verification can proceed for enumerated objects
- Enumeration can complete for rs-manager/armor-apexalgo

### Phase 1: Verification — 🔴 BLOCKED (waiting for Phase 0)

**Status:** 🔴 BLOCKED — Waiting for credential access

**Process (once credentials available):**
```bash
# Option A: Using escrow file (recommended)
./cmd/verify-objects/verify-objects \
  intermediate/filtered-objects.json \
  /path/to/escrow.json \
  docs/notes/verification-results-2026-08.json

# Option B: Using environment variables
export ARMOR_MEK="<64-char-hex>"
export ARMOR_B2_REGION="us-west-002"
export ARMOR_B2_ENDPOINT="..."
export ARMOR_B2_ACCESS_KEY_ID="..."
export ARMOR_B2_SECRET_ACCESS_KEY="..."
# verify-objects reads from env or can be modified to use env
```

**Expected Outcomes:**
- Best case: All objects valid (0% corruption)
- Likely case: Some objects corrupted (estimated 10-30%)
- Worst case: All objects corrupted (100% corruption)

**Decision Point:** Once verification completes, proceed to remediation ONLY for confirmed-corrupted objects.

### Phase 2: Re-Upload for Regenerable Data (iad-ci) — ⏳ CONDITIONAL

**Scope:** 593 objects (3.36 GB) — CONTINGENT on verification confirming corruption

**Timeline:** 1-2 days
- Day 1: Trigger backups, verify new objects
- Day 2: Confirm cleanup, update backup retention policies

**Risk:** LOW — All data regenerable from live cluster

### Phase 3: Re-Upload for Data Pipeline (iad-kalshi) — ⏳ CONDITIONAL

**Scope:** 1,778 objects (43.98 GB) — CONTINGENT on verification confirming corruption

**Timeline:** 5-10 days (depends on upstream availability)

**Risk:** MEDIUM — Some data may be unrecoverable if upstream sources unavailable

### Phase 4: rs-manager & armor-apexalgo — 🔴 BLOCKED (waiting for Phase 0)

**Status:** CANNOT PLAN until enumeration completes

**Decision Points:**
- rs-manager: Are these operational backups critical?
- armor-apexalgo: Is this legacy data still needed (cluster decommissioning)?

## Next Actions

**Immediate (Operator Required):**
1. 🔴 **CRITICAL:** Retrieve ARMOR credentials from OpenBao for verification
   - Paths: `secret/rs-manager/iad-ci/armor/MASTER_ENCRYPTION_KEY`, `secret/rs-manager/iad-ci/b2/iad-ci`
   - Similar for iad-kalshi, rs-manager, armor-apexalgo
   - Provide via escrow file or scoped write token

2. 🔴 **CRITICAL:** Provide B2 credentials for rs-manager/armor-apexalgo enumeration
   - Master B2 key OR per-bucket keys for `nap-dashboard`, `armor-apexalgo`

**Post-Credential-Access:**
3. Enumerate rs-manager and armor-apexalgo buckets
4. Run verification script for all 4 buckets
5. Update this inventory with actual verification results
6. Execute remediation phases based on confirmed corruption
7. Create follow-up beads for each confirmed corrupt object

**Can Proceed Now (Independent of Credentials):**
8. Begin ARMOR version baseline updates — upgrade deployments to 0.1.43+

## Acceptance Criteria Status

- [x] Corruption inventory structure established (this document)
- [ ] All corrupted objects categorized by severity — INCOMPLETE: verification blocked
- [ ] Remediation actions assigned — CONDITIONAL on verification results
- [ ] Re-upload plan identifies source and process — INCOMPLETE for rs-manager/armor-apexalgo
- [ ] Re-baseline plan documents version updates — Phase 4 documented
- [ ] Summary statistics by bucket/version — PARTIAL: 2/4 buckets enumerated

## Related Artifacts

- **Enumeration results:** `intermediate/filtered-objects.json` (2,371 objects)
- **Deployment windows:** `intermediate/deployment-windows.json`
- **Verification tool:** `cmd/verify-objects/main.go`
- **Decryption tool:** `cmd/armor-decrypt/armor-decrypt`
- **Previous inventory:** `docs/bf-1ebnuz-corruption-inventory-four-buckets.md` (2026-08-11)
- **Memory:** `openbao-no-agent-write-path.md` (agent credential access limitations)

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
**Last Updated:** 2026-08-28
**Next Review:** After credential access is granted and enumeration/verification complete
