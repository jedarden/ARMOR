# Phase 6: Multipart-Era Corruption Inventory

**Generated**: 2026-07-16
**Audit Scope**: Five unaudited ARMOR buckets
**Affected Version Window**: 0.1.35–0.1.41 (2026-06-10/11)
**Fixed Versions**: 0.1.42+

## Executive Summary

This document provides a comprehensive corruption inventory and remediation plan for ARMOR buckets that were not audited following the multipart upload bug discovery in June 2026.

### Risk Assessment

| Bucket | Cluster | Risk Level | Status | Objects >5MiB | Action Required |
|--------|---------|------------|--------|---------------|-----------------|
| armor-apexalgo | apexalgo-iad | CRITICAL | PENDING AUDIT | Unknown | FULL AUDIT REQUIRED |
| ord-devimprint | ord-devimprint | HIGH | KNOWN CORRUPTION | ~100 | REMEDIATION IN PROGRESS |
| iad-ci | iad-ci | MEDIUM | PENDING AUDIT | Unknown | FULL AUDIT REQUIRED |
| iad-kalshi | iad-kalshi | MEDIUM | PENDING AUDIT | Unknown | FULL AUDIT REQUIRED |
| rs-manager | rs-manager | MEDIUM | PENDING AUDIT | Unknown | FULL AUDIT REQUIRED |

## Detailed Inventory

### armor-apexalgo (CRITICAL - Live ACB Content)

**Description**: Confirmed LIVE ACB content. This bucket contains active application data that is currently in use.

**Risk Factors**:
- Contains live application data (ACB = AI Code Battle)
- Never rotate MEK without listing this bucket first
- Unknown exposure during multipart bug window

**Audit Priority**: CRITICAL

**Known Issues**:
- Unknown if this deployment was on affected versions during 2026-06-10/11 window
- No existing corruption reports

**Remediation Plan**:
1. **Immediate enumeration**: List all objects >5MiB with creation timestamps
2. **Version window analysis**: Cross-reference with deployment history
3. **Selective verification**: Verify objects written during affected window
4. **Live data protection**: If corruption found, implement graceful migration strategy

**Estimated Objects to Verify**: TBD (requires enumeration)

**Recovery Options**:
- Source data likely available in application databases
- Can re-upload from application backups
- Coordinate with application team before any remediation

---

### ord-devimprint (HIGH - Known Corruption)

**Description**: queue-api litestream backup chain already confirmed actively corrupted as of 2026-07-14/15.

**Risk Factors**:
- Disaster recovery capability is currently broken
- Multiple snapshot objects fail HMAC verification
- Live pod/PVC failure would result in data loss

**Audit Priority**: HIGH (partially complete)

**Known Issues**:
- Level-9 snapshot (44,908,497 bytes, created 2026-07-14) fails with "HMAC verification failed"
- Level-3 compacted object (836,559 bytes) reads successfully
- Multipart GET path bug (bf-1v6skf) was the root cause, now fixed in bf-24sxh7

**Confirmed Corrupted Objects**:
- `state/litestream/queue.db/0009/0000000000000001-0000000000066562.ltx` (44.9 MB, 2026-07-14 00:02 UTC)

**Remediation Plan**:
1. **✅ multipart GET path fix**: Deployed via bf-24sxh7 (closed 2026-07-15)
2. **ARMOR version upgrade**: ord-devimprint now on 0.1.42 (fixed version)
3. **Force fresh snapshot**: After ARMOR upgrade, create new litestream snapshot
4. **Test restore**: Verify new snapshot restores successfully
5. **Verify integrity**: Run `sqlite3 .verify` on restored database
6. **Promote new snapshot**: Make new snapshot the DR restore point

**Estimated Recovery Timeline**: 2-4 hours

**Pre-requisites**:
- ARMOR deployment on 0.1.42+ (✅ COMPLETE)
- Multipart GET path fix deployed (✅ COMPLETE)
- Litestream operational (✅ VERIFIED)

---

### iad-ci (MEDIUM - Never Audited)

**Description**: CI/CD cluster running Argo Workflows. Never audited since original 2026-06 multipart bug.

**Risk Factors**:
- Currently on version 0.1.24 (behind latest 0.1.1848)
- May have been exposed during multipart bug window
- Contains workflow artifacts and CI data

**Audit Priority**: MEDIUM

**Known Issues**:
- None known (never audited)
- Version drift detected but below threshold

**Remediation Plan**:
1. **Full enumeration**: List all objects >5MiB
2. **Version window analysis**: Determine deployment history
3. **Targeted verification**: Verify objects from affected window
4. **Version upgrade consideration**: Plan upgrade to latest version

**Estimated Objects to Verify**: TBD (requires enumeration)

**Recovery Options**:
- CI workflows can be re-run if needed
- Artifact data may be reconstructible
- Coordinate with CI/CD team

---

### iad-kalshi (MEDIUM - Never Audited)

**Description**: Kalshi weather workloads cluster. Never audited since original 2026-06 multipart bug.

**Risk Factors**:
- Currently on version 0.1.13 (behind latest 0.1.1848)
- Likely exposed during multipart bug window
- Contains weather pipeline data

**Audit Priority**: MEDIUM

**Known Issues**:
- None known (never audited)
- Significant version drift detected

**Remediation Plan**:
1. **Full enumeration**: List all objects >5MiB
2. **Version window analysis**: Determine deployment history
3. **Comprehensive verification**: High likelihood of exposure
4. **Version upgrade planning**: Significant version gap requires planning

**Estimated Objects to Verify**: TBD (requires enumeration)

**Recovery Options**:
- Weather data may be reconstructible from source APIs
- Pipeline can be re-run if source data available
- Coordinate with data engineering team

---

### rs-manager (MEDIUM - Never Audited)

**Description**: Rackspace Spot manager cluster. Never audited since original 2026-06 multipart bug.

**Risk Factors**:
- Currently on version 0.1.13 (behind latest 0.1.1848)
- Likely exposed during multipart bug window
- Contains cluster management data

**Audit Priority**: MEDIUM

**Known Issues**:
- None known (never audited)
- Significant version drift detected

**Remediation Plan**:
1. **Full enumeration**: List all objects >5MiB
2. **Version window analysis**: Determine deployment history
3. **Comprehensive verification**: High likelihood of exposure
4. **Version upgrade planning**: Significant version gap requires planning

**Estimated Objects to Verify**: TBD (requires enumeration)

**Recovery Options**:
- Manager state can be rebuilt from cluster state
- Configuration data likely version-controlled
- Coordinate with infrastructure team

---

## Remediation Matrix

| Priority | Bucket | Action | Timeline | Dependencies |
|----------|--------|--------|----------|--------------|
| P0 | ord-devimprint | Complete litestream restore test | Immediate | ARMOR upgrade ✅, GET fix ✅ |
| P1 | armor-apexalgo | Full corruption audit | This week | B2/HTTP access |
| P2 | iad-kalshi | Full corruption audit | This week | B2/HTTP access |
| P3 | iad-ci | Full corruption audit | This week | B2/HTTP access |
| P4 | rs-manager | Full corruption audit | This week | B2/HTTP access |

## Success Criteria

### ord-devimprint (Known Corruption)
- [ ] Fresh snapshot created post-upgrade
- [ ] Test restore completes successfully
- [ ] Database integrity check passes
- [ ] New snapshot promoted as DR restore point
- [ ] Old corrupted snapshots archived/replaced

### armor-apexalgo (Live Data)
- [ ] All objects >5MiB enumerated
- [ ] Objects in affected window identified
- [ ] All candidates verified successfully
- [ ] No corruption detected in live data
- [ ] MEK rotation safety confirmed

### Other Buckets (Never Audited)
- [ ] All objects >5MiB enumerated
- [ ] Deployment windows analyzed
- [ ] High-risk candidates verified
- [ ] Corruption (if any) documented
- [ ] Recovery plans (if needed) created

## Long-term Actions

1. **Continuous monitoring**: Extend canary monitor to cover multipart path (bf-4595)
2. **Automated audits**: Schedule regular corruption audits as part of DR drills
3. **Version drift alerts**: Implement automated version drift monitoring (bf-2t1f)
4. **Documentation updates**: Keep DR documentation current with audit findings

## References

- **Beads**: bf-659opq (this audit), bf-2t1f (version drift), bf-24sxh7 (GET fix)
- **ADRs**: docs/adr/002-multipart-corruption-detection-gaps.md
- **DR Procedures**: docs/disaster-recovery.md
- **Litestream**: docs/litestream-restore-procedure-and-verification.md

---

**Document Status**: DRAFT
**Next Review**: After ord-devimprint restore test completion
**Maintainer**: ARMOR team