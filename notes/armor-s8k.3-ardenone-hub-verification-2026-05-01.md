# ARMOR v0.1.13 DuckDB httpfs Verification - ardenone-hub Follow-up

## Date: 2026-05-01

## Task Context
Bead armor-s8k.3: Verify DuckDB httpfs works with fixed ARMOR after date fix.

## Summary
The main verification task was **completed on ord-devimprint cluster** (see `notes/armor-s8k.3-verification-summary-2026-05-02.md`). This document captures follow-up observations from ardenone-hub cluster.

## ardenone-hub Status

### v0.1.11 (Running - Has URL Encoding Bug)
**Pod**: `armor-6c6f554d7d-8skcv` (devimprint namespace)
**Image**: `ronaldraygun/armor:0.1.11`
**Status**: 1/1 Running (29 restarts)

**Bug Evidence from Logs**:
```
{"time":"2026-05-02T02:33:53.756017714Z"...,"path":"/devimprint/commits/year=2026/month=04/day=02/clone-worker-6b94b786b8-5np4b-1777636125.parquet","status":400}
```

The `year=2026` in the path causes HTTP 400 because v0.1.11 doesn't URL-decode the key parameter.

### v0.1.13 (Failing - Has Fix)
**Pod**: `armor-6cb55b69b-g468l` (devimprint namespace)
**Image**: `localhost:7439/ronaldraygun/armor:0.1.13`
**Status**: 0/1 CrashLoopBackOff (49 restarts)
**Exit Code**: 2

**Logs** (truncated):
```
{"time":"2026-05-02T02:44:26.872651405Z","level":"INFO","service":"armor","msg":"ARMOR starting"...}
{"time":"2026-05-02T02:44:26.872909096Z","level":"INFO","service":"armor","msg":"B2 key management disabled (application key is bucket-scoped)"}
```

**Issue**: Container starts successfully but exits immediately with code 2. No error messages logged before exit.

**Events**:
- Container fails liveness probe
- ExternalSecret update failures: `ClusterSecretStore "openbao" is not ready`
- Image pulled from local registry (`localhost:7439`)

## Deployment Issue Analysis

The v0.1.13 deployment failure on ardenone-hub is a **deployment/runtime issue**, not a code issue:
1. Code fix verified working on ord-devimprint cluster
2. Container starts but exits silently (exit code 2)
3. ExternalSecret issues suggest secret management problems

## Verification Status

| Criteria | Status | Evidence |
|----------|--------|----------|
| Deploy fixed ARMOR | ✅ | Deployed on ord-devimprint |
| DuckDB httpfs glob expansion | ✅ | Verified on ord-devimprint |
| No InvalidInputException | ✅ | Verified on ord-devimprint |
| Timestamps reasonable | ✅ | Verified on ord-devimprint |
| Matches boto3 approach | ✅ | Verified on ord-devimprint |

## Conclusion

**Bead armor-s8k.3 is COMPLETE** based on ord-devimprint verification. The v0.1.13 URL decode fix works correctly for DuckDB httpfs.

The ardenone-hub deployment issue requires separate investigation:
1. Check ExternalSecret/ClusterSecretStore configuration
2. Verify environment variables match v0.1.11
3. Consider pulling image from Docker Hub instead of local registry

## Related Files
- `notes/armor-s8k.3-verification-summary-2026-05-02.md` - Primary verification summary
- `notes/armor-s8k.3-duckdb-httpfs-final-verification-2026-05-02.md` - Full verification details
- Issue: https://github.com/jedarden/ARMOR/issues/8
