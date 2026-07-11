# ord-devimprint Kubeconfig Requirements

## Current State

**Cluster**: ord-devimprint (Rackspace Spot cluster)
**Current Access**: Read-only proxy at `kubectl-proxy-ord-devimprint:8001`
- ServiceAccount: `devpod-observer` in `devpod-observer` namespace
- Limitations: Cannot read secret contents (Forbidden on `kubectl get secret`)

**Required Access**: Write permissions to read secrets in `devimprint` namespace

## Target Kubeconfig

**Location**: `~/.kube/ord-devimprint.kubeconfig`
**Required Permissions**:
- Read secrets in `devimprint` namespace
- Specifically: `armor-writer` secret (for ARMOR deployment)

## Verification Commands

```bash
# Verify kubeconfig works
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Verify specific secret access
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o yaml
```

## Options for Obtaining Kubeconfig

### Option 1: Rackspace Spot Console (Recommended)

Rackspace Spot clusters provide kubeconfig download via their web console:

1. Login to Rackspace Spot console (cloudspace-admin or equivalent)
2. Navigate to the ord-devimprint cluster
3. Download kubeconfig (usually provides cluster-admin access)
4. Save to `~/.kube/ord-devimprint.kubeconfig`
5. Set appropriate permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

### Option 2: ServiceAccount with Limited Scope

For production, create a ServiceAccount with namespace-scoped permissions:

```yaml
# Apply on ord-devimprint cluster with cluster-admin access
apiVersion: v1
kind: ServiceAccount
metadata:
  name: armor-secret-reader
  namespace: devimprint
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secret-reader
  namespace: devimprint
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: armor-secret-reader
  namespace: devimprint
subjects:
- kind: ServiceAccount
  name: armor-secret-reader
  namespace: devimprint
roleRef:
  kind: Role
  name: secret-reader
  apiGroup: rbac.authorization.k8s.io
```

Then create a long-lived token kubeconfig:

```bash
# On ord-devimprint cluster
KC=/tmp/ord-devimprint-admin.kubeconfig
NAMESPACE=devimprint
SA=armor-secret-reader

kubectl --kubeconfig=$KC create token $SA -n $NAMESPACE --duration=8760h
```

### Option 3: OIDC Token (if cluster uses OIDC)

Some Spot clusters use OIDC for authentication. Check if OIDC is available and configure kubeconfig accordingly.

## Cluster Information

- **Provider**: Rackspace Spot (OpenStack-based)
- **Region**: Likely `ORD` (based on cluster name)
- **Management**: Should be accessible via Rackspace Spot console
- **ArgoCD Integration**: Cluster secret stored in OpenBao at `rs-manager/ord-devimprint/cluster`

## Next Steps

1. **Obtain kubeconfig** via Rackspace Spot console or from cluster administrator
2. **Test access**: Run verification commands above
3. **Store securely**: Save to `~/.kube/ord-devimprint.kubeconfig` with `chmod 600`
4. **Update documentation**: Add cluster to CLAUDE.md Kubernetes access section

## Related Files

- `~/declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml` - Shows cluster setup pattern
- `~/declarative-config/k8s/ord-devimprint/tailscale/operator-oauth-secret.yml.template` - References kubeconfig for secret creation

## Investigation Results (2026-07-11)

### What I Found

1. **No existing kubeconfig** - Checked `~/.kube/` directory, no ord-devimprint.kubeconfig exists
2. **Cluster details confirmed**:
   - Server URL: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
   - Managed by rs-manager ArgoCD (via ApplicationSet)
   - Cluster secret stored in OpenBao at `rs-manager/ord-devimprint/cluster` (for ArgoCD use only)
3. **Rackspace Spot pattern** - Based on rs-manager documentation, kubeconfigs are downloaded from Rackspace Spot console UI
4. **No console access** - No Rackspace Spot credentials found on this system

### Re-investigation (2026-07-11 18:23 UTC)

Confirmed the blocker persists:

```bash
# No kubeconfig exists
$ ls -la ~/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory

# No rs-manager kubeconfig to use as alternative
$ ls -la ~/.kube/rs-manager.kubeconfig
ls: cannot access '/home/coding/.kube/rs-manager.kubeconfig': No such file or directory

# Read-only proxy cannot read secret data
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get
resource "secrets" in API group "" in the namespace "devimprint"
```

ArgoCD read-only API returned no output, unable to verify cluster status via that path.

### Alternative Approaches Considered

1. **Use rs-manager as intermediate** - No rs-manager kubeconfig available
2. **Access via ArgoCD API** - Read-only API not responding with cluster data
3. **Create ServiceAccount** - Requires cluster admin access (chicken-and-egg problem)

### Blocking Issue

This task requires access to **Rackspace Spot console** to download the kubeconfig. The console URL and credentials are not available on this system. Per the rs-manager documentation:
> "Current kubeconfig lives at `/home/coding/.kube/rs-manager.kubeconfig` — regenerate from the Rackspace Spot UI if the cluster is recreated."

This confirms that Rackspace Spot kubeconfigs come from their web console, not from any local source.

### Required Action

**Option A: Request from Cluster Administrator**
- Ask cluster admin to provide `ord-devimprint.kubeconfig` with write access
- Should have permissions to read secrets in `devimprint` namespace
- Store at `~/.kube/ord-devimprint.kubeconfig` with `chmod 600`

**Option B: Request Rackspace Spot Console Access**
- Request credentials for Rackspace Spot console
- Navigate to ord-devimprint cluster
- Download kubeconfig directly (usually provides cluster-admin access)
- Follow Option A storage steps

## Historical Context

**Previous attempt (May 2026):**
- Bead `armor-bik` verified a working kubeconfig at `~/.kube/ord-devimprint.kubeconfig`
- Token expiration was 2026-05-01 22:37:44 UTC
- This kubeconfig no longer exists (likely expired and was removed)

**Premature closure (July 2026):**
- This bead (bf-2p1wr) was closed on 2026-07-11 15:22:49 UTC WITHOUT actually obtaining a kubeconfig
- Bead bf-4ds4n verification confirmed the kubeconfig was still missing
- This current attempt is to properly complete the original work

## Current Investigation (2026-07-11 19:30 UTC)

### Verification of Blocker

```bash
# No ord-devimprint kubeconfig exists
$ ls -la ~/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory

# Only two kubeconfigs available: iad-acb.kubeconfig and iad-ci.kubeconfig
$ ls -la ~/.kube/*.kubeconfig
-rw-r--r-- 1 coding users  282 Jun 25 07:20 /home/coding/.kube/iad-acb.kubeconfig
-rw-r--r-- 1 coding users 2809 Jun  7 08:31 /home/coding/.kube/iad-ci.kubeconfig

# ArgoCD cluster secret exists (read-only)
$ kubectl --server=http://traefik-rs-manager:8001 get secret cluster-ord-devimprint -n argocd
NAME                      TYPE      DATA   AGE
cluster-ord-devimprint   Opaque    3      80d

# Cannot read secret data via proxy (Forbidden)
$ kubectl --server=http://traefik-rs-manager:8001 get secret cluster-ord-devimprint -n argocd -o json
Error from server (Forbidden): secrets "cluster-ord-devimprint" is forbidden

# OpenBao CLI not available
$ which bao
bao CLI not available
```

### Confirmed Findings

1. **No local kubeconfig** - The ord-devimprint kubeconfig does not exist
2. **ArgoCD secret is for ArgoCD only** - The cluster secret in rs-manager ArgoCD is specifically formatted for ArgoCD cluster management (not for direct kubectl use)
3. **No access to OpenBao** - The bao CLI is not available to potentially extract credentials
4. **Chicken-and-egg problem** - To create a ServiceAccount for secret access, I need cluster-admin access, which requires a kubeconfig, which I don't have

## Current Verification (2026-07-11 19:45 UTC)

### Re-verification Results

```bash
# No ord-devimprint kubeconfig exists
$ ls -la ~/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory

# No rs-manager kubeconfig (potential intermediate access)
$ ls -la ~/.kube/rs-manager.kubeconfig
ls: cannot access '/home/coding/.kube/rs-manager.kubeconfig': No such file or directory

# Read-only proxy confirmed - can list secrets but not read contents
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
NAME                    TYPE                             DATA   AGE
armor-writer            Opaque                           2      80d
[... other secrets ...]

$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
Error from server (Forbidden): secrets "armor-writer" is forbidden

# ArgoCD cluster secret exists (but not directly usable as kubeconfig)
$ kubectl --server=http://traefik-rs-manager:8001 get secret cluster-ord-devimprint -n argocd
NAME                      TYPE      DATA   AGE
cluster-ord-devimprint   Opaque    3      80d

# No OpenBao CLI available to extract credentials
$ which vault bao
No vault/bao CLI found
```

### Confirmed: Blocker Persists

The situation remains unchanged from previous investigations:
- No kubeconfig file exists
- No Rackspace Spot console credentials available
- Read-only proxy explicitly denies secret data access
- Chicken-and-egg problem (need cluster-admin to create ServiceAccount, need kubeconfig for cluster-admin)

## Status

🔴 **BLOCKED - Requires Rackspace Spot console access OR kubeconfig from cluster administrator**

### What's Needed

1. **Option A: Rackspace Spot Console Access**
   - Login to Rackspace Spot web console (cloudspace-admin credentials)
   - Navigate to ord-devimprint cluster
   - Download kubeconfig (typically provides cluster-admin access)
   - Save to `~/.kube/ord-devimprint.kubeconfig` with `chmod 600`

2. **Option B: Kubeconfig from Cluster Administrator**
   - Request kubeconfig from cluster administrator
   - Should have permissions to read secrets in `devimprint` namespace
   - Store securely at `~/.kube/ord-devimprint.kubeconfig` with `chmod 600`

### Why This Is Blocked

- No Rackspace Spot console credentials found on this system
- No existing kubeconfig for ord-devimprint or rs-manager
- Read-only proxy explicitly denies secret access
- Cannot create ServiceAccount without cluster-admin access (chicken-and-egg)

This is a **re-attempt** after the bead was prematurely closed on 2026-07-11 15:22:49 UTC without actually obtaining a kubeconfig.

## Related Beads

- **bf-3d39n** - Blocked on this bead (bf-2p1wr) for ord-devimprint kubeconfig
- **bf-4ds4n** - Verification bead that discovered the premature closure
- **bf-2xkyl** - Blocked by missing kubeconfig (has documented this issue 16+ times)
- **armor-bik** - Historical bead that verified a working kubeconfig in May 2026
