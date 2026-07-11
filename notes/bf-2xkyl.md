# Bead bf-2xkyl - BLOCKER: Missing kubeconfig access

## Task
Retrieve S3 credentials from armor-writer secret in ord-devimprint cluster

## BLOCKER - Cannot Proceed
**Prerequisite bead bf-2p1wr was marked closed but work was not completed.**

## Current State (2026-07-11)
- **Prerequisite bead:** bf-2p1wr (status: closed)
- **Required kubeconfig:** `~/.kube/ord-devimprint.kubeconfig` (DOES NOT EXIST)
- **Read-only proxy:** `kubectl-proxy-ord-devimprint:8001` (blocks secret access)

## What Was Attempted
1. Checked for ord-devimprint kubeconfig files - **NOT FOUND**
2. Tried read-only proxy - **BLOCKED BY RBAC**:
   ```
   Error from server (Forbidden): secrets "armor-writer" is forbidden: 
   User "system:serviceaccount:devpod-observer:devpod-observer" 
   cannot get resource "secrets" in API group "" in the namespace "devimprint"
   ```
3. Checked kubectl contexts - **no ord-devimprint context found**

## What Is Needed
A kubeconfig file for ord-devimprint with secret-read permissions in the devimprint namespace. Following the pattern of other Rackspace Spot clusters:

**Option 1: ServiceAccount kubeconfig (similar to iad-ci)**
- Create ServiceAccount with secret-read RBAC in devimprint namespace
- Generate long-lived token kubeconfig
- Store at `~/.kube/ord-devimprint.kubeconfig`

**Option 2: OIDC cloudspace-admin token (similar to iad-options)**
- Obtain OIDC token from Rackspace Spot console
- Token expires every ~3 days, requires regeneration
- Less ideal for automation

## Next Steps
1. **Re-open bead bf-2p1wr** - The prerequisite bead should be re-opened and actually completed
2. **Coordinate with cluster administrator** to obtain appropriate kubeconfig
3. **Verify access** with: `kubectl get secret armor-writer -n devimprint`
4. **Then proceed** with retrieving the credentials

## Target Commands (once kubeconfig is available)
```bash
# Get access key
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d

# Get secret key
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}' | base64 -d
```

## Cluster Information
- **Name:** ord-devimprint
- **Type:** Rackspace Spot cluster
- **Location:** us-east-ord-1 (Chicago)
- **Namespace:** devimprint
- **Secret:** armor-writer
- **Required data:** LITESTREAM_ACCESS_KEY_ID, LITESTREAM_SECRET_ACCESS_KEY

## References
- Parent context: Litestream S3 replication configuration
- Similar pattern: iad-ci.kubeconfig (ServiceAccount with cluster-admin)
- RBAC requirement: secret-read in devimprint namespace
