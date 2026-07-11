# RBAC Blocker - LITESTREAM_ACCESS_KEY_ID Validation

## Date
2026-07-11

## Issue
Unable to retrieve the `armor-writer` secret from the devimprint namespace on ord-devimprint cluster due to RBAC permissions.

## Error Details
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## What Was Attempted
1. Tried to retrieve secret using kubectl-proxy (read-only access)
2. The devpod-observer service account does not have permission to read secrets
3. No read/write kubeconfig available for ord-devimprint cluster

## Expected Secret Structure
The `armor-writer` secret in the devimprint namespace should contain:
- `auth-access-key` (mapped to `LITESTREAM_ACCESS_KEY_ID` environment variable)
- `auth-secret-key` (mapped to `LITESTREAM_SECRET_ACCESS_KEY` environment variable)

This secret is synced from OpenBao via ExternalSecret from path:
`rs-manager/ord-devimprint/armor-writer`

## Resolution Options
1. Grant devpod-observer service account read access to secrets in devimprint namespace
2. Provide a read/write kubeconfig for ord-devimprint cluster
3. Manually retrieve the secret value from OpenBao or another cluster with admin access

## Prerequisite Status
**BLOCKED**: Cannot proceed with base64 decoding and validation without secret access.

## Related Files
- ExternalSecret: ~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml
- Deployment: ~/declarative-config/k8s/ord-devimprint/devimprint/queue-api-deployment.yml
