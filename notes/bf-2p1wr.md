# ord-devimprint Kubeconfig Request

## Current Status

**Access Method**: kubectl proxy (read-only)
- Endpoint: `http://kubectl-proxy-ord-devimprint:8001`
- ServiceAccount: `system:serviceaccount:devpod-observer:devpod-observer`
- Limitations: Cannot read secret contents (Forbidden)

**Verified Limitation**:
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

**What Works**:
- List namespaces, pods, deployments
- List secret names (but not contents)

## Required Access

Need kubeconfig with permissions to:
1. Read secrets in `devimprint` namespace
2. Specifically target: `armor-writer` secret

**Desired Location**: `/home/coding/.kube/ord-devimprint.kubeconfig`

## Verification Steps

Once kubeconfig is obtained, verify with:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
```

## Cluster Context

**Cluster**: ord-devimprint
**Purpose**: ARMOR deployments for devimprint project
**Relevant Secrets**:
- `armor-writer` (Opaque, 2 data keys, 81 days old)
- `armor-credentials` (Opaque, 7 data keys, 81 days old)
- `armor-readonly` (Opaque, 2 data keys, 81 days old)

## Action Required

❌ **External coordination needed** - This requires cluster administrator to:
1. Create or provide a ServiceAccount with secret read permissions in `devimprint` namespace
2. Generate kubeconfig with appropriate credentials
3. Securely deliver the kubeconfig to this server

## Related Documentation

- CLAUDE.md: ord-devimprint cluster section
- Plan references to ord-devimprint ARMOR deployments
- ADR-002: Multipart corruption detection gaps (ord-devimprint incident history)
