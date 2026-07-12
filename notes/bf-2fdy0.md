# RBAC Blocker: Cannot Retrieve Secret from ord-devimprint

## Task Attempted
Retrieve LITESTREAM_SECRET_ACCESS_KEY from the `armor-writer` secret in the `devimprint` namespace.

## Access Methods Tried

### 1. Direct kubeconfig (file doesn't exist)
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig
```
Result: `stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory`

### 2. Kubectl-proxy over Tailscale
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
```
Result: Forbidden by RBAC

## RBAC Error Details
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Root Cause
The `devpod-observer` ServiceAccount used by the kubectl-proxy has **read-only access** and **explicitly denies access to secrets**. This is by design for security - the observer proxy intentionally cannot access sensitive resources.

## Resolution Options
To complete this task, one of the following would be needed:

1. **Create a dedicated ServiceAccount** with secret access in the devimprint namespace
2. **Use direct cluster access** with cluster-admin credentials (if available)
3. **Coordinate with cluster admin** to grant the observer SA limited secret access for this specific secret

## Status
**BLOCKED** - Cannot proceed without elevated credentials or RBAC changes.
