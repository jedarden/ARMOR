# ARMOR Multipart HMAC Fix - Deployment Guide

## Current State

- **Code Fix**: ✅ Complete (commit 3edbb9b4, version 0.1.1858+)
- **Tests**: ✅ All passing
- **Production Deployment**: ❌ NOT DEPLOYED (still on armor:0.1.42)
- **Snapshot Status**: ❌ 2026-07-14 snapshot is corrupted (created with buggy version)

## Deployment Requirements

### 1. Build New Docker Image

The fixed code needs to be built and pushed to `ronaldraygun/armor`:

```bash
# Current version should be tagged
VERSION=$(cat VERSION)  # Should be 0.1.1859+
docker build -t ronaldraygun/armor:${VERSION} .
docker push ronaldraygun/armor:${VERSION}
```

### 2. Update declarative-config

Update the ARMOR deployment in `jedarden/declarative-config`:

```yaml
# File: k8s/ord-devimprint/armor/deployment.yaml (or similar)
spec:
  template:
    spec:
      containers:
      - name: armor
        image: ronaldraygun/armor:0.1.1859  # Update from 0.1.42
```

### 3. Deploy via ArgoCD

After pushing to declarative-config, ArgoCD will automatically sync the change to ord-devimprint.

### 4. Verify Deployment

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint -l app=armor
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get deployment armor -n devimprint
```

### 5. Wait for New Snapshot

Litestream will automatically create a new snapshot with the fixed ARMOR version. The old snapshot will remain corrupted but new ones will work.

### 6. Test Restore

```bash
# Test restore with new snapshot
litestream restore -config /path/to/litestream.yml
```

## Verification Steps

After deployment, verify:

1. **Deployment Health**: Pods are running and not crashing
2. **Version Check**: New version is deployed (`kubectl describe pod`)
3. **Canary Tests**: Multipart upload canary passes
4. **New Snapshot**: Litestream creates a new snapshot
5. **Restore Test**: Restore succeeds with HMAC verification

## Estimated Timeline

- **CI/CD Build**: 5-10 minutes (armor-build workflow)
- **ArgoCD Sync**: 1-2 minutes
- **Litestream Snapshot**: Depends on schedule (typically daily)
- **Verification**: 15-30 minutes

## Risk Assessment

**LOW RISK**:
- The fix is well-tested with comprehensive regression tests
- Only affects NEW multipart uploads and snapshots
- Existing objects are not impacted (except they can't be restored)
- Rolling deployment with 3 replicas ensures availability

**MITIGATION**:
- Monitor deployment closely after sync
- Check logs for HMAC verification errors
- Have rollback plan ready (revert to 0.1.42)

## Success Criteria

- ✅ ARMOR version 0.1.1859+ deployed to ord-devimprint
- ✅ All pods healthy and passing health checks
- ✅ New litestream snapshot created
- ✅ Restore test succeeds with HMAC verification
- ✅ No HMAC verification errors in logs

## Post-Deployment Tasks

1. Monitor litestream logs for successful snapshot creation
2. Verify restore works with new snapshot
3. Document the fix in runbook
4. Consider cleanup of old corrupted snapshot (optional)

## Notes

- The old snapshot (2026-07-14) cannot be repaired - it was created with buggy code
- After deployment, litestream will create new clean snapshots automatically
- The fix ensures all future multipart uploads will have correct HMAC data
- queue-api DR restore will be functional once new snapshot is created
