# Bead bf-2y15n: Blocked - Missing kubeconfig for ord-devimprint

## Issue
Unable to retrieve LITESTREAM_ACCESS_KEY_ID from armor-writer secret because the required kubeconfig file does not exist.

## Findings

### 1. Kubeconfig file missing
```bash
$ ls -la /home/coding/.kube/*.kubeconfig
-rw-r--r-- 1 coding users  282 Jun 25 07:20 /home/coding/.kube/iad-acb.kubeconfig
-rw-r--r-- 1 coding users 2809 Jun  7 08:31 /home/coding/.kube/iad-ci.kubeconfig
```

The file `/home/coding/.kube/ord-devimprint.kubeconfig` does not exist.

### 2. Read-only proxy cannot access secrets
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

The proxy runs with read-only RBAC and explicitly denies access to secrets.

### 3. Cluster access pattern
According to CLAUDE.md, ord-devimprint cluster access is documented as:
- Proxy runs in `devpod-observer` namespace with **read-only RBAC**
- Access is **read-only** — cannot create, delete, or modify resources
- Exposed via Tailscale operator — hostname `kubectl-proxy-ord-devimprint`
- No direct kubeconfig is documented for this cluster (unlike ardenone-manager, rs-manager, or iad-ci)

## Prerequisite verification
The bead specifies prerequisites:
- bf-4743d: "Verify kubeconfig path exists and is accessible" — marked **closed**
- bf-2pn4n: "Test kubectl access to devimprint namespace" — marked **closed**

However, the kubeconfig file does not exist at the expected path, suggesting either:
1. The prerequisite beads were marked complete incorrectly
2. The kubeconfig was expected to be created but wasn't
3. There's been a regression since those beads completed

## Resolution required
To complete this bead, one of the following is needed:
1. Create `/home/coding/.kube/ord-devimprint.kubeconfig` with appropriate secret-read permissions
2. Update the task to use an alternative method for accessing the secret (e.g., ExternalSecret caching, or access through a cluster with credentials like ardenone-manager)
3. Revoke the prerequisite beads' completion status if they did not actually verify the kubeconfig exists

## Status
**BLOCKED** — Cannot proceed without valid secret access.
