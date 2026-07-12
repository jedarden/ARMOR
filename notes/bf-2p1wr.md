# ord-devimprint Kubeconfig Acquisition (Bead bf-2p1wr)

## Current Status
- **Cluster**: ord-devimprint (Rackspace Spot)
- **Server**: https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com
- **Current access**: Read-only via kubectl-proxy (ServiceAccount: devpod-observer)
- **Limitation**: Cannot read secret contents (needed for retrieving armor-writer secret)

## Verification of Current Access
```bash
# Can list secrets but not read contents
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
# Output: Lists secret names (armor-credentials, armor-writer, etc.)

# Cannot read secret data
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## What We Need
A kubeconfig file with write access to the ord-devimprint cluster, specifically with permissions to:
- Read secrets in the devimprint namespace
- Get/list secrets resource

## How to Obtain (Rackspace Spot Pattern)

Based on other Rackspace Spot clusters (iad-options, iad-ci), the credential pattern is:

### Option 1: Cloudspace-Admin OIDC Token (Preferred)
1. Log in to Rackspace Spot UI (https://spot.rackspace.com)
2. Navigate to the ord-devimprint cluster/cloudspace
3. Generate/retrieve the cloudspace-admin OIDC token
4. The token typically expires every ~3 days and needs regeneration

### Option 2: Direct ServiceAccount Token
1. Contact cluster administrator to create a ServiceAccount with appropriate RBAC
2. Request permissions: `get`, `list` secrets in `devimprint` namespace
3. Obtain long-lived token or configure OIDC authentication

## Kubeconfig Structure
Once credentials are obtained, the kubeconfig should follow this pattern:

```yaml
apiVersion: v1
kind: Config
clusters:
  - cluster:
      certificate-authority-data: <base64-encoded-ca>
      server: https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com
    name: ord-devimprint
contexts:
  - context:
      cluster: ord-devimprint
      namespace: devimprint
      user: cloudspace-admin  # or appropriate username
    name: ord-devimprint
current-context: ord-devimprint
preferences: {}
users:
  - name: cloudspace-admin
    user:
      token: <OIDC-token-or-serviceaccount-token>
```

## Verification Steps
Once kubeconfig is obtained at `~/.kube/ord-devimprint.kubeconfig`:

```bash
# Test basic connectivity
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# Test secrets access in devimprint namespace
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Test reading the specific secret we need
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o json
```

## Alternative Approaches
If OIDC credentials cannot be obtained:

1. **Request limited RBAC**: Ask cluster admin to create a ServiceAccount with minimal permissions:
   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: armor-secret-reader
     namespace: devimprint
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     name: armor-secret-reader
     namespace: devimprint
   rules:
   - apiGroups: [""]
     resources: ["secrets"]
     resourceNames: ["armor-writer"]
     verbs: ["get"]
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
     name: armor-secret-reader
     apiGroup: rbac.authorization.k8s.io
   ```

2. **Use cluster administrator access**: If you have cluster-admin access via another method (e.g., rs-manager), use that to create the above RBAC

## Next Steps
1. Contact cluster administrator or access Rackspace Spot UI
2. Obtain OIDC token or ServiceAccount credentials
3. Create kubeconfig file at `~/.kube/ord-devimprint.kubeconfig`
4. Verify access with test commands above
5. Proceed to next bead (retrieve armor-writer secret)

## Related Documentation
- Rackspace Spot cluster patterns in ~/declarative-config/k8s/CLAUDE.md
- Similar cluster setup: iad-options (uses cloudspace-admin OIDC token)
- Task acceptance criteria: Must be able to run `kubectl get secrets -n devimprint`

## Required Action
**USER ACTION REQUIRED**: This task cannot be completed by an automated agent because it requires:

### Option A: Access Rackspace Spot UI (Recommended)
1. Log in to https://spot.rackspace.com with your Rackspace credentials
2. Navigate to the ord-devimprint cluster (ID: hcp-5f30c973-cde7-42d9-8c7b-5d0573821330)
3. Download/Generate kubeconfig with cloudspace-admin permissions
4. Save to: `/home/coding/.kube/ord-devimprint.kubeconfig`
5. Run verification commands below

### Option B: Request from Cluster Administrator
If you don't have Spot UI access, request a kubeconfig or ServiceAccount token from the cluster administrator with:
- Cluster: ord-devimprint
- Required permissions: read secrets in devimprint namespace
- Specific secret needed: armor-writer

## Status
**LAST CHECKED**: 2026-07-12 12:11 UTC
**CURRENT STATE**: No kubeconfig exists at `~/.kube/ord-devimprint.kubeconfig`
**REQUIRED ACTION**: Agent cannot access Rackspace Spot UI (requires browser authentication). User must either:
- Access Spot UI and obtain kubeconfig
- Provide kubeconfig/credentials for agent to use
- Coordinate with cluster administrator

**CURRENT BARRIER**: This task requires manual intervention to obtain credentials from Rackspace Spot UI or cluster admin.

## After Obtaining Kubeconfig
Once you have the kubeconfig file, run these verification commands:

```bash
# Verify basic connectivity
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get nodes

# Verify secret access (acceptance criteria)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Test reading armor-writer secret specifically
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
```

If all commands succeed, the bead can be closed and you can proceed to retrieve the armor-writer secret.
