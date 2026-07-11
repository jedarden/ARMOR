# Task: Obtain ord-devimprint kubeconfig with write access

## Current Situation

The ord-devimprint cluster is currently accessible only via a read-only kubectl proxy:

- **Proxy endpoint**: `kubectl-proxy-ord-devimprint:8001`
- **Access level**: Read-only (via devpod-observer service account)
- **Secrets access**: DENIED (`kubectl auth can-i get secrets -n devimprint` returns `no`)

## What's Needed

To retrieve the `armor-writer` secret, we need a kubeconfig file with:
- Permissions to read secrets in the `devimprint` namespace
- Stored securely at `~/.kube/ord-devimprint.kubeconfig`

## Existing Kubeconfig Pattern

Other clusters have direct kubeconfigs:
- `~/.kube/iad-ci.kubeconfig` - Full cluster-admin access
- `~/.kube/iad-acb.kubeconfig` - Another cluster config

## Cluster Information

From ExternalSecret configuration (`~/declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml`):
- **Cluster Server**: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Type**: Rackspace Spot cluster
- **Management Cluster**: rs-manager

## Acquisition Process

### Method 1: Rackspace Spot Portal (Recommended)

1. **Access Rackspace Spot Portal**:
   ```bash
   # Navigate to: https://spot.rackspace.com
   # Authenticate with Rackspace Spot credentials
   ```

2. **Download Admin Kubeconfig**:
   - Find cluster: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330` or "ord-devimprint"
   - Download kubeconfig to `/tmp/ord-devimprint-admin.kubeconfig`

3. **Create ServiceAccount with Write Access**:
   ```bash
   KC=/tmp/ord-devimprint.kubeconfig

   # Create serviceaccount
   kubectl --kubeconfig=$KC create serviceaccount argocd-manager -n kube-system

   # Grant cluster-admin permissions
   kubectl --kubeconfig=$KC create clusterrolebinding argocd-manager \
     --clusterrole=cluster-admin --serviceaccount=kube-system:argocd-manager

   # Generate long-lived token (1 year)
   TOKEN=$(kubectl --kubeconfig=$KC create token argocd-manager \
     -n kube-system --duration=8760h)

   echo "Token: $TOKEN"
   ```

4. **Create Kubeconfig File**:
   ```bash
   # Extract CA data from admin kubeconfig
   CA_DATA=$(kubectl --kubeconfig=$KC config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

   # Create final kubeconfig
   cat > ~/.kube/ord-devimprint.kubeconfig << EOF
   apiVersion: v1
   kind: Config
   clusters:
     - cluster:
         certificate-authority-data: $CA_DATA
         server: https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com
       name: ord-devimprint
   contexts:
     - context:
         cluster: ord-devimprint
         user: argocd-manager
       name: ord-devimprint
   current-context: ord-devimprint
   preferences: {}
   users:
     - name: argocd-manager
       user:
         token: $TOKEN
   EOF

   # Secure permissions
   chmod 600 ~/.kube/ord-devimprint.kubeconfig
   ```

### Method 2: OpenBao Integration (Optional)

For GitOps integration, store credentials in OpenBao:

```bash
bao kv put secret/rs-manager/ord-devimprint/cluster \
  server="https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com" \
  token="$TOKEN"
```

This enables the ExternalSecret to sync automatically to rs-manager.

## Next Steps

**This requires Rackspace Spot portal access or coordination with the cluster administrator.**

## Verification Steps (once kubeconfig is obtained)

```bash
# Test basic connectivity
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# Test secret access (acceptance criteria)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Test the specific secret we need
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```

## Cluster Notes

From CLAUDE.md:
- ord-devimprint uses Tailscale operator (not Traefik like other clusters)
- Proxy hostname: `kubectl-proxy-ord-devimprint`
- No existing write-access kubeconfig on file

## Status

**DOCUMENTATION COMPLETE** - Detailed acquisition process documented above.

This task requires:
1. **Rackspace Spot portal access** with admin permissions on the ord-devimprint cluster
2. **Or coordination with the cluster administrator** who can provide the kubeconfig

The documentation above provides the exact steps needed to:
- Access the Rackspace Spot portal and download the admin kubeconfig
- Create a ServiceAccount with cluster-admin permissions
- Generate a long-lived token (1 year validity)
- Store the kubeconfig securely at `~/.kube/ord-devimprint.kubeconfig`
- Optionally integrate with OpenBao for GitOps automation

Once the kubeconfig is obtained, run the verification steps below to confirm access.
