# ord-devimprint Kubeconfig Request

## Current Situation

**Problem:** Need a kubeconfig with write access (specifically secret read permissions) to the ord-devimprint cluster to retrieve the `armor-writer` secret in the `devimprint` namespace.

**Current Access:** Read-only proxy at `kubectl-proxy-ord-devimprint:8001` via ServiceAccount `devpod-observer`
- Can LIST secrets: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint` ✓
- Cannot GET secret contents: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json` ✗

## Already Exists (But Inaccessible)

A `secret-reader` ServiceAccount **already exists** with the correct permissions:
- SA: `secret-reader` in `devpod-observer` namespace
- Token secret: `secret-reader-token` (created 25m ago)
- RoleBinding: Grants `get`/`list` on secrets in `devimprint` namespace

**Problem:** Cannot extract the token value because the current proxy SA (`devpod-observer`) cannot read secrets (only list them).

## Required Action

External action required: A cluster administrator needs to extract the `secret-reader-token` value and provide it.

### Command for Cluster Admin (with direct kubeconfig access):

```bash
kubectl get secret secret-reader-token -n devpod-observer -o jsonpath='{.data.token}' | base64 -d
```

### Once Token is Provided:

Create `~/.kube/ord-devimprint.kubeconfig`:

```yaml
apiVersion: v1
kind: Config
clusters:
  - cluster:
      certificate-authority-data: <CA_DATA_BASE64>
      server: https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com
    name: ord-devimprint
contexts:
  - context:
      cluster: ord-devimprint
      namespace: devimprint
      user: secret-reader
    name: ord-devimprint
current-context: ord-devimprint
users:
  - name: secret-reader
    user:
      token: <PROVIDED_TOKEN>
```

To get CA_DATA_BASE64:
```bash
kubectl get secret secret-reader-token -n devpod-observer -o jsonpath='{.data.ca\.crt}'
```

## Verification

Once kubeconfig is created:
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```

## Cluster Details

- **Cluster:** ord-devimprint (Rackspace Spot cluster in Chicago region)
- **API Server:** https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com
- **Target Secret:** armor-writer in devimprint namespace
- **Declarative-config:** k8s/ord-devimprint/devpod-observer/

## References

- RBAC definition: ~/declarative-config/k8s/ord-devimprint/devpod-observer/rbac.yml
- secret-reader SA: ~/declarative-config/k8s/ord-devimprint/devpod-observer/secret-reader-sa.yml
- Observer app: ~/declarative-config/k8s/ord-devimprint/devpod-observer-application.yml

Generated: 2026-07-12
