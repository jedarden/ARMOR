# Bead bf-2xkyl: Retrieve S3 credentials from armor-writer secret - BLOCKED

## Status
**BLOCKED** - Prerequisite bead bf-2p1wr did not provide required kubeconfig

## Issue
Bead bf-2p1wr ("Obtain ord-devimprint kubeconfig with write access") shows as **closed**, but the required kubeconfig file does not exist:

```bash
# No kubeconfig exists:
$ ls /home/coding/.kube/*ord*devimprint*
# (no files found)

# Read-only proxy explicitly denies secret access:
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i get secrets -n devimprint
no

$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## What's Needed
1. A kubeconfig file with write access to ord-devimprint cluster (able to read secrets)
2. The kubeconfig should be stored at `~/.kube/ord-devimprint.kubeconfig` or similar
3. Must be able to run: `kubectl get secret armor-writer -n devimprint`

## Next Steps
- Re-open bead bf-2p1wr to actually obtain the kubeconfig
- OR coordinate with cluster administrator to obtain the credentials
- Once kubeconfig is available, retrieve the S3 credentials from armor-writer secret

## Commands to Run Once Kubeconfig is Available
```bash
kubectl --kubeconfig=/path/to/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d

kubectl --kubeconfig=/path/to/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}' | base64 -d
```
