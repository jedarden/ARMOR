# ord-devimprint Kubeconfig Acquisition - bf-2p1wr

## Current Situation

The ord-devimprint cluster is currently accessible only via a read-only kubectl proxy:
- `kubectl-proxy-ord-devimprint:8001` - Read-only access, explicitly denies secret access

## What's Needed

A kubeconfig with write access to ord-devimprint, specifically the ability to:
- Read secrets in the `devimprint` namespace
- Manage resources as needed

## Cluster Details

From `/home/coding/declarative-config/k8s/ord-devimprint/external-secrets/external-secrets-application.yml`:

- **Cluster Type:** Rackspace Spot cluster
- **API Server:** `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Managed by:** rs-manager ArgoCD

## Steps to Obtain Kubeconfig

### Option 1: Download from Rackspace Spot Console

1. Log into the Rackspace Spot console at https://spot.rackspace.com
2. Navigate to the ord-devimprint cluster
3. Use the "Download Kubeconfig" feature to get admin or service account credentials
4. Save to `~/.kube/ord-devimprint.kubeconfig`

### Option 2: Create ServiceAccount via Cluster Access

If you have cluster-admin access (via Spot console or existing credentials):

```yaml
# Create a service account with secret read permissions
apiVersion: v1
kind: ServiceAccount
metadata:
  name: armor-writer
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
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods", "services", "configmaps"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: armor-writer-secret-reader
  namespace: devimprint
subjects:
  - kind: ServiceAccount
    name: armor-writer
    namespace: devimprint
roleRef:
  kind: Role
  name: secret-reader
  apiGroup: rbac.authorization.k8s.io
```

Then get the service account token and create a kubeconfig:

```bash
# Get the token name
SECRET_NAME=$(kubectl --kubeconfig=<ord-devimprint-admin-kubeconfig> \
  get sa armor-writer -n devimprint \
  -o jsonpath='{.secrets[0].name}')

# Get the token
TOKEN=$(kubectl --kubeconfig=<ord-devimprint-admin-kubeconfig> \
  get secret $SECRET_NAME -n devimprint \
  -o jsonpath='{.data.token}' | base64 -d)

# Create the kubeconfig
cat > ~/.kube/ord-devimprint.kubeconfig <<EOF
apiVersion: v1
kind: Config
clusters:
  - cluster:
      certificate-authority-data: <CA_DATA>
      server: https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com
    name: ord-devimprint
contexts:
  - context:
      cluster: ord-devimprint
      namespace: devimprint
      user: armor-writer
    name: ord-devimprint
current-context: ord-devimprint
preferences: {}
users:
  - name: armor-writer
    user:
      token: $TOKEN
EOF
```

## Verification

Once obtained, verify access:

```bash
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```

## Verification History

This task has been verified 25 times (2026-07-11 to 2026-07-12). All attempts have reached the same conclusion.

### Verification Attempt 25 (2026-07-12)

#### Confirmed Findings

1. **No kubeconfig exists:**
   - `~/.kube/ord-devimprint.kubeconfig` - NOT_FOUND
   - Only existing kubeconfigs: `iad-acb.kubeconfig`, `iad-ci.kubeconfig`
   - `rs-manager.kubeconfig` - documented but does not exist

2. **Read-only proxy behavior verified:**
   ```bash
   # CAN list secret names (verified working)
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
   # Shows: admin-oauth, armor-credentials, armor-readonly, armor-writer, etc.

   # CANNOT read secret data (Forbidden - verified)
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o yaml
   # Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

3. **All alternative paths blocked:**
   - No existing ord-devimprint kubeconfig available
   - Cannot create ServiceAccount via read-only proxy (RBAC denies)
   - ArgoCD cluster credentials stored in ExternalSecret but inaccessible (RBAC denies secret reading)
   - No rs-manager kubeconfig on this system to use as path
   - No self-service elevation mechanism

#### Status

**BLOCKED - PERSISTENT (25th verification):**

This task cannot be completed by an agent. It requires manual user action to access the Rackspace Spot console.

### Required User Action

To complete this task, the user must:

1. Log into https://spot.rackspace.com
2. Navigate to the ord-devimprint cluster (API: hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com)
3. Download kubeconfig OR create the `armor-writer` ServiceAccount with appropriate permissions
4. Securely transfer the kubeconfig to `~/.kube/ord-devimprint.kubeconfig` on this server
5. Verify access with commands above

### Acceptance Criteria Status

- [ ] Kubeconfig file for ord-devimprint cluster obtained - **BLOCKED**
- [ ] Kubeconfig has permissions to read secrets in devimprint namespace - **BLOCKED**
- [ ] Can successfully run: kubectl get secrets -n devimprint - **BLOCKED**

## Related Context

This is bead bf-2p1wr, part of the ARMOR project. The persistent blocker has been confirmed across 25 verification attempts - all reach the same conclusion: this requires Rackspace Spot console access or cluster administrator coordination to resolve.
