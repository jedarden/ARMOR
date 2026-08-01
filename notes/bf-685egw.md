# armor-build 0.1.1897 Investigation

## Task Context
Investigate armor-build workflow for version 0.1.1897 (commit f32c9234, 2026-07-20 13:08:49 UTC).

## Findings

### Workflow Logs Purged
The workflow logs for 0.1.1897 build attempt have been **purged** by Argo Workflows TTL retention policy:

**TTL Configuration** (from `argo-workflows-iad-ci-workflow-controller-configmap`):
- Successful workflows: **30 minutes** after completion (1800 seconds)
- Failed workflows: **2 hours** after failure (7200 seconds)  
- Default fallback: **1 hour** after completion (3600 seconds)

**Current State**: Only 4 workflows remain in the cluster, all from 3-5 days ago. The July 20th workflow (12+ days ago) is long gone.

### Evidence of Build Failure
The 0.1.1897 image **does not exist** on Docker Hub (HTTP 404), confirming the workflow did not complete successfully. This is documented in bead **bf-3u9pv0** ("0.1.1897 ghost tag"):

> **CI did not build 0.1.1897 image (ghost tag) — VERSION bumped, no pullable image**
> 
> HEAD's VERSION is 0.1.1897 (auto-bump commit f32c9234) but ronaldraygun/armor:0.1.1897 does not exist on Docker Hub (registry manifest API returns HTTP 404).

**Likely Cause** (from bead analysis):
> The armor-build WorkflowTemplate runs go-test as a GATE before docker-build, and 3decc976 added 623 lines of new key_rotation tests — if any fail in CI, no image is published.

The bead notes that commit 3decc976 (key_rotation.go multipart-rotation-resume, 156 LOC) was added in 0.1.1897, and new key_rotation tests likely caused the go-test step to fail, preventing docker-build from ever running.

## Conclusion

**Workflow Name/UID**: Unknown — logs purged by TTL retention policy  
**Phase**: Likely **Failed** (go-test step)  
**Status Message**: Unknown — logs purged  
**Error Output**: Unknown — logs purged  

**Root Cause**: The workflow likely failed at the go-test gate due to new key_rotation tests added in 3decc976, preventing the docker-build step from executing. This resulted in a "ghost tag" where VERSION was bumped but no corresponding Docker image was published.

**Impact**: Fleet deployment was retargeted to 0.1.1896 instead to avoid ImagePullBackOff failures.

## Next Steps (from bead bf-3u9pv0)
- Re-run armor-build to build 0.1.1898 (will compute patch+1 from current HEAD)
- Add post-build registry-existence assertion to WorkflowTemplate to prevent future ghost tags
