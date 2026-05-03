# DuckDB httpfs COUNT(*) Query - armor-s8k.3.2.2

## Date: 2026-05-03

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Investigation Results

### Aggregator Pods Found

#### ardenone-hub cluster (via read-only proxy)
- Namespace: devimprint
- Pod: aggregator-68554db644-ng85f (Running, 8d old)
- Image: ronaldraygun/devimprint-aggregator:latest
- S3 credentials from: armor-writer secret

#### ord-devimprint cluster
- Kubeconfig: /home/coding/.kube/ord-devimprint.kubeconfig
- Requires: OIDC browser authentication (token expired)
- Current token expired: 2026-04-27

### Access Constraints

1. **ardenone-hub proxy**: Read-only RBAC (devpod-observer serviceaccount)
   - Cannot exec into pods
   - Cannot read secrets (armor-writer, armor-readonly)

2. **ord-devimprint kubeconfig**: Requires OIDC flow
   - Plugin: kubectl-oidc_login
   - Issuer: https://login.spot.rackspace.com/
   - Browser auth required - no automated way to complete

### What Was Attempted

1. ✅ Located aggregator pod on ardenone-hub
2. ❌ Exec blocked by read-only RBAC
3. ❌ Port-forward to ARMOR works, but S3 credentials inaccessible
4. ❌ Local DuckDB query fails (no S3 credentials)
5. ❌ OIDC token refresh requires browser interaction

## Required to Complete Task

The task requires exec access to an aggregator pod with S3 credentials. Options:

### Option 1: Refresh ord-devimprint token
```bash
kubectl oidc-login get-token \
  --oidc-issuer-url=https://login.spot.rackspace.com/ \
  --oidc-client-id=mwG3lUMV8KyeMqHe4fJ5Bb3nM1vBvRNa \
  --oidc-extra-scope=openid \
  --oidc-extra-scope=profile \
  --oidc-extra-scope=email \
  --oidc-auth-request-extra-params=organization=org_KsELolwAOxl3Zxfm
```
Then update kubeconfig token and run the query.

### Option 2: Provide kubeconfig with exec access
A kubeconfig for ardenone-hub or ord-devimprint with exec permissions.

### Option 3: Manual execution
User manually runs the query and provides output.

## Status
**BLOCKED** - Cannot exec into aggregator pod due to RBAC/OIDC constraints.
