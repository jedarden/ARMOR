# bf-2p1wr: ord-devimprint Kubeconfig Investigation (2026-07-12)

## Investigation Summary

Investigated the requirements and options for obtaining a kubeconfig with write access to the ord-devimprint cluster.

## Current Status

**❌ TASK CANNOT BE COMPLETED BY AUTOMATED AGENTS**

### Existing Access
- **Read-only kubectl proxy:** `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **ServiceAccount:** `system:serviceaccount:devpod-observer:devpod-observer`
- **Permissions:** Can list resources, but **cannot read secret contents**

### Verification of Current Limitations
```bash
# Can list secret names
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
NAME                    TYPE                             DATA   AGE
armor-writer            Opaque                           2      81d
[... other secrets ...]

# But cannot read secret contents
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Cluster Identity

**ord-devimprint** is a **Rackspace Spot** cluster:
- **Cluster ID:** `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
- **Server URL:** `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Provider:** OpenStack infrastructure via Rackspace Spot
- **Management:** Managed by rs-manager cluster
- **GitOps:** Configured via declarative-config at `~/declarative-config/k8s/ord-devimprint/`

## Access Pattern Analysis

### Similar Clusters
| Cluster | Kubeconfig Path | Access Method | Status |
|---------|----------------|---------------|---------|
| iad-ci | ~/.kube/iad-ci.kubeconfig | ServiceAccount (argocd-manager) | ✅ EXISTS |
| iad-options | ~/.kube/iad-options.kubeconfig | Spot UI (cloudspace-admin OIDC) | ❌ MISSING |
| rs-manager | ~/.kube/rs-manager.kubeconfig | Direct kubeconfig | ❌ MISSING |
| **ord-devimprint** | **~/.kube/ord-devimprint.kubeconfig** | **Spot UI (cloudspace-admin OIDC)** | **❌ MISSING** |

### Rackspace Spot Kubeconfig Pattern
Based on the iad-options pattern (documented in CLAUDE.md):
- Kubeconfigs are obtained through the **Spot web console**
- Authentication method: **cloudspace-admin OIDC token**
- **Token validity:** ~3 days (requires periodic regeneration)
- **Access level:** Cluster-admin permissions

## Required Procedure (External Action)

To obtain the ord-devimprint kubeconfig, the following steps must be performed manually:

### Step 1: Access Rackspace Spot Console
1. Navigate to: https://spot.rackspace.com
2. Authenticate with valid Rackspace Spot credentials
3. Ensure account has access to the ord-devimprint cloudspace

### Step 2: Locate ord-devimprint Cloudspace
- Find cloudspace with ID: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
- Name: `ord-devimprint`
- Region: Likely us-west-002 (based on B2 bucket references)

### Step 3: Download Kubeconfig
- Use Spot UI to download kubeconfig
- Ensure it has **cloudspace-admin** permissions
- This provides cluster-admin level access including secret read permissions

### Step 4: Store Securely
```bash
# Save to standard location
mv ~/Downloads/kubeconfig-*.yaml ~/.kube/ord-devimprint.kubeconfig

# Set secure permissions
chmod 600 ~/.kube/ord-devimprint.kubeconfig
```

### Step 5: Verify Access
```bash
# Test secret access
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Should show full secret details, not just names
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o json
```

## Why This Cannot Be Automated

### Technical Barriers
1. **OIDC Authentication:** Rackspace Spot uses OpenID Connect tokens that require browser-based authentication
2. **Interactive Flow:** Token generation involves human authorization steps
3. **No API Alternative:** Unlike ServiceAccount-based clusters, Spot clusters require UI-generated tokens
4. **Security Model:** Follows zero-trust principles with short-lived, manually-authorized credentials

### Tool Availability
```bash
# spotctl tool (mentioned in docs) is not available
$ which spotctl
spotctl not found

# No direct API access from this system
$ kubectl get cloudspaces.spot.rackspace.com
error: the server doesn't have a resource type "cloudspaces"
```

## Acceptance Criteria Status

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Kubeconfig file obtained | ❌ | No file at `~/.kube/ord-devimprint.kubeconfig` |
| Permissions to read secrets | ❌ | Cannot obtain without kubeconfig |
| Can run `kubectl get secrets -n devimprint` | ⚠️ | Can list names only, not contents |

## Related Work Blocked

This task blocks:
- **bf-3d39n**: Child bead requiring ord-devimprint kubeconfig
- **armor-writer secret access**: Cannot retrieve credentials
- **ARMOR operations**: Cannot manage ARMOR deployment on ord-devimprint

## Recommendations

1. **DO NOT close this bead** - It should remain open until the kubeconfig is obtained
2. **Request manual action** - A human with Rackspace Spot console access needs to download the kubeconfig
3. **Document kubeconfig lifecycle** - Once obtained, document the token refresh process (~3 days)
4. **Consider automation** - Investigate if spotctl or similar tools could be safely configured for future access

## Investigation Conclusion

This task represents a **legitimate external dependency** that cannot be resolved through automated means. The Rackspace Spot authentication model requires browser-based OIDC token generation, which is by design a human-interactive process.

The bead should remain open until external action provides the kubeconfig, at which point verification and dependent tasks can proceed.

---

**Investigation Date:** 2026-07-12
**Investigator:** Automated agent investigation
**Result:** External action required - task cannot be completed programmatically
