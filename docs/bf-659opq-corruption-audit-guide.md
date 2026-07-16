# Phase 6: Multipart-Era Corruption Audit Execution Guide

## Overview

This document provides step-by-step instructions for executing the multipart-era corruption audit across ARMOR buckets. The audit detects objects corrupted during the multipart upload bug period (versions 0.1.35-0.1.41, fixed in 0.1.42+).

## Prerequisites

1. **ARMOR version-drift check complete** (bf-2t1f): Required to identify which deployment windows were affected
2. **Multipart GET path fix deployed** (bf-24sxh7): Required for accurate verification (otherwise all large objects appear corrupt)
3. **B2 credentials OR ARMOR HTTP access**: At least one access method must be available

## Affected Buckets

| Bucket | Cluster | Risk Level | Description |
|--------|---------|------------|-------------|
| armor-apexalgo | apexalgo-iad | CRITICAL | Confirmed LIVE ACB content - never rotate MEK without listing this bucket first |
| ord-devimprint | ord-devimprint | HIGH | queue-api already confirmed actively corrupted as of 2026-07-14/15 |
| iad-ci | iad-ci | MEDIUM | Never audited since original 2026-06 multipart bug |
| iad-kalshi | iad-kalshi | MEDIUM | Never audited since original 2026-06 multipart bug |
| rs-manager | rs-manager | MEDIUM | Never audited since original 2026-06 multipart bug |

## Access Methods

### Method 1: Direct B2 Access (Preferred)

Set up B2 credentials:

```bash
export ARMOR_B2_REGION="us-east-005"  # or your region
export ARMOR_B2_ENDPOINT="https://s3.us-east-005.backblazeb2.com"  # or your endpoint
export ARMOR_B2_ACCESS_KEY_ID="your-key-id"
export ARMOR_B2_SECRET_ACCESS_KEY="your-secret-key"
```

### Method 2: ARMOR HTTP API via Port-Forward

Set up ARMOR auth credentials:

```bash
export ARMOR_AUTH_ACCESS_KEY="your-access-key"
export ARMOR_AUTH_SECRET_KEY="your-secret-key"
```

Then set up port-forwards for each cluster:

```bash
# Terminal 1: iad-ci
kubectl --kubeconfig=~/.kube/iad-ci.kubeconfig port-forward -n armor svc/armor 9000:9000

# Terminal 2: iad-kalshi (if accessible)
kubectl --kubeconfig=~/.kube/iad-kalshi.kubeconfig port-forward -n armor svc/armor 9001:9000

# Terminal 3: rs-manager (if accessible)
kubectl --kubeconfig=~/.kube/rs-manager.kubeconfig port-forward -n armor svc/armor 9002:9000

# Terminal 4: ord-devimprint (if accessible)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig port-forward -n armor svc/armor 9003:9000

# Terminal 5: armor-apexalgo (if accessible)
kubectl --kubeconfig=~/.kube/apexalgo-iad.kubeconfig port-forward -n armor svc/armor 9004:9000
```

## Running the Audit

### Step 1: Run the Comprehensive Framework

```bash
cd /home/coding/ARMOR

# Audit all buckets
python3 scripts/corruption-audit-framework.py --work-dir ./audit_work --output ./corruption_audit_results.json

# Or audit a specific bucket
python3 scripts/corruption-audit-framework.py --bucket ord-devimprint --output ./ord_devimprint_audit.json
```

### Step 2: Interpret the Results

The audit produces a JSON report with the following structure:

```json
{
  "audit_timestamp": "2026-07-16T18:00:00",
  "summary": {
    "total_buckets": 5,
    "buckets_audited": 5,
    "total_objects_enumerated": 150,
    "candidates_for_verification": 50,
    "verified_clean": 45,
    "corrupted": 3,
    "unable_to_verify": 2
  },
  "buckets": {
    "ord-devimprint": {
      "verified_clean": 40,
      "corrupted": 2,
      "unable_to_verify": 1,
      "verification_results": [...]
    }
  },
  "corruption_inventory": {
    "remediation_plan": [...]
  }
}
```

Exit codes:
- `0`: All verified objects are clean
- `1`: Corruption detected
- `2`: Unable to verify some objects

### Step 3: Review Verification Details

For each object, the verification results include:

```json
{
  "bucket": "ord-devimprint",
  "key": "state/litestream/queue.db/0009/snapshot.ltx",
  "size": 44908497,
  "risk_level": "HIGH",
  "in_affected_window": true,
  "verification_status": "CORRUPTED",
  "decrypt_result": {
    "success": false,
    "error": "HMAC verification failed"
  }
}
```

Verification status values:
- `VERIFIED`: Object is clean and restorable
- `CORRUPTED`: Object cannot be decrypted/verified
- `UNABLE_TO_VERIFY`: Access error or timeout

## Remediation

### For Corrupted Objects

The remediation plan in the output will specify actions:

```json
{
  "bucket": "ord-devimprint",
  "key": "state/litestream/queue.db/0009/snapshot.ltx",
  "action": "RE_UPLOAD",
  "priority": "HIGH",
  "reason": "Corrupted - needs re-upload from source"
}
```

**Steps:**
1. **Identify the source**: Locate the original data source (if available)
2. **Re-upload**: Upload the original data through the current ARMOR instance (fixed version)
3. **Verify**: Re-run the audit to confirm the new object is clean
4. **Update references**: Update any applications/backups that reference the old object

### For ord-devimprint queue-api Specific Case

The queue-api litestream backup chain is known to be corrupted. Follow the litestream restore procedure with these steps:

1. **Force fresh snapshot**: After ARMOR version upgrade, force litestream to create a new snapshot
2. **Test restore**: Perform a test restore to a scratch location
3. **Verify integrity**: Check database integrity with `sqlite3 .verify`
4. **Promote**: Once verified, promote the new snapshot as the restore point

See `docs/litestream-restore-procedure-and-verification.md` for detailed steps.

### For Unable-to-Verify Objects

Objects that couldn't be verified need manual investigation:

1. **Check access permissions**: Verify credentials have read access
2. **Check network connectivity**: Ensure port-forwards are active
3. **Check ARMOR logs**: Look for errors in the ARMOR pod logs
4. **Manual verification**: Try manual download with `armor-decrypt`

## Post-Audit Actions

1. **Update project memory**: Document findings in memory files
2. **Update DR documentation**: Update disaster-recovery.md with any changes to restore procedures
3. **Schedule follow-up**: If unable to verify some objects, schedule a follow-up audit
4. **Close bead**: Once audit is complete and documented, close bf-659opq

## Known Limitations

1. **Deployment window accuracy**: The cross-reference against deployment windows is approximate without exact deployment timestamps
2. **B2 credential scope**: Some credentials may not have access to all buckets
3. **Port-forward requirements**: HTTP method requires active port-forwards to each cluster
4. **Verification time**: Large objects may take significant time to verify

## Emergency Procedures

If critical corruption is found in live buckets:

1. **Immediate assessment**: Determine if the corruption affects live data
2. **Source verification**: Check if source data is available for re-upload
3. **Graceful degradation**: If source is unavailable, document the corruption and plan migration
4. **Stakeholder notification**: Alert relevant teams if their data is affected

## Contact and Support

For issues with:
- **Audit execution**: Check this guide and script help text
- **ARMOR access**: Check disaster-recovery.md for credential procedures
- **Remediation**: Check specific service documentation (e.g., litestream)

## Related Documentation

- `docs/adr/002-multipart-corruption-detection-gaps.md`: Full incident analysis
- `docs/disaster-recovery.md`: ARMOR disaster recovery procedures
- `docs/litestream-restore-procedure-and-verification.md`: queue-api specific restore procedures
- `.beads/issues.jsonl`: bf-659opq (this audit), bf-2t1f (version drift), bf-24sxh7 (GET fix)