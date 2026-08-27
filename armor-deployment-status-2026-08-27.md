# ARMOR Deployment Validation Status

**Date:** 2026-08-27
**Task:** Deep-validate deployed ARMOR on iad-ci, iad-kalshi, rs-manager, iad-acb

## Summary

The validation task faces significant blockers - only one cluster (rs-manager) has a working ARMOR deployment, and access to it is currently blocked by expired kubeconfig credentials.

## Cluster Status

### ✅ rs-manager
- **Status:** ARMOR deployment **RUNNING** (version 0.1.1913)
- **Pod:** armor-684694d5bb-ttfvr (1/1 Running, 0 restarts, 3d14h age)
- **Service:** ClusterIP on 10.21.118.151 (ports 9000, 9001)
- **Access:** BLOCKED - kubeconfig credentials expired
- **Proxy:** Read-only proxy working (traefik-rs-manager:8001) but cannot access secrets
- **Validation:** READY TO RUN once credentials fixed
- **Notes:** Also has restore-verifier deployments for rs-manager and acb

### ❌ iad-ci
- **Status:** ARMOR deployment **CRASHLOOPBACKOFF**
- **Pod:** armor-6c56d6b875-8v8sp (0/1 CrashLoopBackOff, 44 restarts, 2d22h age)
- **Issue:** ExternalSecret `armor-secrets` sync failing - cannot find `PREFIX` property in `rs-manager/iad-ci/armor`
- **Error:** "error processing spec.data[5] (key: rs-manager/iad-ci/armor), err: cannot find secret data for key: \"PREFIX\""
- **Fix Required:** Update ExternalSecret to remove or fix the PREFIX property reference
- **Last Sync:** 7 days ago (2026-08-20)
- **Version:** 0.1.1913 (same as rs-manager)
- **Notes:** restore-verifier deployment also crashlooping

### ❓ iad-kalshi
- **Status:** **NO ARMOR DEPLOYMENT FOUND**
- **Proxy:** Connection refused (traefik-iad-kalshi:8001 not answering)
- **Search Results:** No `armor` namespace exists
- **Related:** kalshi-tape-query uses ARMOR credentials via ExternalSecret at `rs-manager/iad-kalshi/armor/kalshi-tape`
- **Notes:** This cluster may only have ARMOR clients, not an ARMOR deployment

### ❓ iad-acb
- **Status:** **NO ARMOR DEPLOYMENT FOUND**
- **Declarative Config:** Only has drawrace and restore-verifier-acb
- **Notes:** acb likely uses ARMOR from another cluster or has no ARMOR dependency

## Validation Script

Created `/home/coding/ARMOR/scripts/validate-deployment.sh` to perform:
1. ✅ Sequential 50MB multipart upload (ARMOR requires max_concurrent_requests=1)
2. ✅ Download and SHA-256 comparison
3. ✅ Object metadata verification
4. ✅ Cleanup of test objects
5. ✅ Detailed reporting with color-coded output

**Requirements:**
- AWS CLI installed
- ARMOR_AUTH_ACCESS_KEY and ARMOR_AUTH_SECRET_KEY environment variables
- Endpoint URL and bucket name

## Blockers

### Primary Blockers

1. **rs-manager kubeconfig expired**
   - File: `/home/coding/.kube/rs-manager.kubeconfig` (last updated Aug 27 12:08)
   - Error: "You must be logged in to the server (the server has asked the the client to provide credentials)"
   - Impact: Cannot run validation against the only working ARMOR deployment
   - Fix Required: Regenerate Spot UI admin kubeconfig for rs-manager

2. **iad-ci secret sync failure**
   - ExternalSecret `armor-secrets` failing for 7+ days
   - Missing `PREFIX` property in OpenBao secret path `rs-manager/iad-ci/armor`
   - Impact: ARMOR deployment cannot start
   - Fix Required: Either add PREFIX to OpenBao secret or remove from ExternalSecret spec

### Secondary Blockers

3. **iad-kalshi proxy unreachable**
   - traefik-iad-kalshi:8001 connection refused
   - Impact: Cannot verify if ARMOR deployment exists or its status
   - Fix Required: Check Tailscale connectivity and traefik ingress

4. **iad-acb access unknown**
   - No kubeconfig exists for iad-acb cluster
   - declarative-config only has drawrace and restore-verifier-acb
   - Impact: Cannot validate ARMOR on this cluster
   - Fix Required: Determine if iad-acb has ARMOR and how to access it

## Comparison with ord-devimprint Validation

The ord-devimprint validation (2026-07-18) was successful because:
- ARMOR deployment was running and healthy
- Access credentials were available
- All three validation steps could be executed:
  - Sequential 50MB multipart round-trip ✅
  - Spot-read of pre-bump object ✅
  - Canary + multipart-canary checks ✅

Current situation lacks these prerequisites on all target clusters.

## Recommendations

### Immediate Actions Required

1. **Fix rs-manager kubeconfig** (highest priority)
   - Regenerate from Spot UI admin console
   - Test access: `kubectl --kubeconfig=/home/coding/.kube/rs-manager.kubeconfig get pods -n armor`
   - Run validation once access restored

2. **Fix iad-ci secret sync**
   - Check OpenBao for missing `PREFIX` property at `rs-manager/iad-ci/armor`
   - Either add property to OpenBao or update ExternalSecret spec
   - Restart armor deployment after sync succeeds

3. **Investigate iad-kalshi**
   - Check if traefik-iad-kalshi Tailscale route is up
   - Verify if ARMOR deployment actually exists
   - If it doesn't exist, determine if validation is applicable

4. **Investigate iad-acb**
   - Determine if ARMOR deployment exists
   - If not, determine validation requirements

### Validation Execution Plan

Once rs-manager access is restored:

```bash
# 1. Set credentials from OpenBao (manual operator action)
export ARMOR_AUTH_ACCESS_KEY="..."
export ARMOR_AUTH_SECRET_KEY="..."

# 2. Run validation script
/home/coding/ARMOR/scripts/validate-deployment.sh rs-manager http://localhost:9000 rs-manager

# 3. Check canary objects
aws s3 ls s3://rs-manager/canary/ --endpoint-url=http://localhost:9000 --recursive
aws s3 ls s3://rs-manager/canary-multipart/ --endpoint-url=http://localhost:9000 --recursive

# 4. Spot-read pre-bump object (needs version information)
# Object written by old version needs to be identified first
```

## Notes

- **iad-native-ads**: DEPRECATED and removed from scope (per operator confirmation 2026-07-19)
- **ord-devimprint**: Not in validation scope for this task (already validated 2026-07-18)
- **Validation script**: Ready to use once access issues are resolved
- **Multipart requirement**: ARMOR requires sequential part uploads (max_concurrent_requests=1)

## Next Steps

This task cannot proceed without operator intervention to:
1. Regenerate rs-manager kubeconfig
2. Fix iad-ci OpenBao secret or ExternalSecret
3. Clarify iad-kalshi and iad-acb ARMOR deployment status

Task should be updated to reflect blockers and reopened once access issues are resolved.
