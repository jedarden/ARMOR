# Bead bf-5vow9: Verify armor-writer secret exists in devimprint namespace

## Task Verification

**Objective:** Confirm that the armor-writer secret exists in the devimprint namespace and contains the expected keys.

## Findings

### 1. ExternalSecret Configuration Verified

The ExternalSecret `armor-writer` exists in the `devimprint` namespace and is defined in:
`~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`

**Spec:**
- Secret keys created: `auth-access-key`, `auth-secret-key`
- OpenBao path: `rs-manager/ord-devimprint/armor-writer`
- Properties: `auth-access-key`, `auth-secret-key`

### 2. ExternalSecret Status

```
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get externalsecret armor-writer -n devimprint
```

**Status:**
- Condition: `Ready` = `True`
- Reason: `SecretSynced`
- Message: `secret synced`
- Last synced: `2026-07-11T16:21:24Z`

The ExternalSecret successfully synced the secret from OpenBao.

### 3. Secret Keys Clarification

The acceptance criteria mentions:
- `LITESTREAM_ACCESS_KEY_ID`
- `LITESTREAM_SECRET_ACCESS_KEY`

These are **environment variable names** used in deployments (e.g., queue-api), not the actual secret data keys.

The actual secret keys are:
- `auth-access-key` (mapped to env var `LITESTREAM_ACCESS_KEY_ID`)
- `auth-secret-key` (mapped to env var `LITESTREAM_SECRET_ACCESS_KEY`)

### 4. Deployment Reference

In `queue-api-deployment.yml`:
```yaml
env:
- name: LITESTREAM_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: armor-writer
      key: auth-access-key
- name: LITESTREAM_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: armor-writer
      key: auth-secret-key
```

## Verification Result

✓ ExternalSecret `armor-writer` exists in devimprint namespace
✓ ExternalSecret status is `Ready` with `SecretSynced` reason
✓ Secret contains the expected keys: `auth-access-key` and `auth-secret-key`
✓ These keys are correctly referenced by the LITESTREAM_* environment variables

**Note:** Direct secret access is blocked by the read-only RBAC on the devimprint proxy. Verification was done through ExternalSecret status, which confirms successful secret creation and syncing from OpenBao.

## Acceptance Criteria

- [x] Secret 'armor-writer' exists in devimprint namespace
- [x] Secret contains `auth-access-key` key (env var: `LITESTREAM_ACCESS_KEY_ID`)
- [x] Secret contains `auth-secret-key` key (env var: `LITESTREAM_SECRET_ACCESS_KEY`)
