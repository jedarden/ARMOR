# bf-2xkyl: BLOCKER Re-investigation - Still Missing Kubeconfig Access

## Investigation Date: 2026-07-11

## Task Status: BLOCKED

Task: Retrieve S3 credentials from armor-writer secret in devimprint namespace

## Confirmed Blockers

### 1. ord-devimprint Kubeconfig - MISSING
- **Expected location:** `~/.kube/ord-devimprint.kubeconfig`
- **Status:** Does NOT exist
- **Impact:** Cannot access ord-devimprint cluster secrets directly

### 2. rs-manager Kubeconfig - MISSING  
- **Expected location:** `~/.kube/rs-manager.kubeconfig`
- **Status:** Does NOT exist
- **Impact:** Cannot access OpenBao or rs-manager secrets

### 3. Read-only Proxy Access - INSUFFICIENT PERMISSIONS

#### ord-devimprint proxy
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001
```
- **Status:** Service accessible
- **Permissions:** READ-ONLY
- **Secret access:** BLOCKED
- **Error:** `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in namespace "devimprint"`

#### rs-manager proxy
```bash
kubectl --server=http://traefik-rs-manager:8001
```
- **Status:** Service accessible
- **Permissions:** READ-ONLY
- **Secret access:** BLOCKED for all namespaces (argocd, armor, openbao)
- **Error:** `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"`

### 4. ArgoCD rs-manager - INACCESSIBLE
- **Expected URL:** `https://argocd-rs-manager.tail1b1987.ts.net:8080`
- **Status:** Hostname does not resolve (NXDOMAIN)
- **Impact:** Cannot verify cluster status or access ArgoCD API

### 5. OpenBao Direct Access - INACCESSIBLE
- **Service:** `openbao-rs-manager.openbao.svc.cluster.local:8200`
- **Status:** ClusterIP only, not exposed externally
- **Impact:** Cannot retrieve credentials from OpenBao path `rs-manager/ord-devimprint/armor-writer`

## Secret Key Mapping (CONFIRMED)

The task requests `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY`, but the actual ExternalSecret defines:

```yaml
secretKey: auth-access-key
  remoteRef:
    key: rs-manager/ord-devimprint/armor-writer
    property: auth-access-key
    
secretKey: auth-secret-key  
  remoteRef:
    key: rs-manager/ord-devimprint/armor-writer
    property: auth-secret-key
```

**Actual keys in Kubernetes secret:**
- `auth-access-key` (NOT `LITESTREAM_ACCESS_KEY_ID`)
- `auth-secret-key` (NOT `LITESTREAM_SECRET_ACCESS_KEY`)

**Note:** This key name mismatch may indicate a configuration issue in litestream-restore-verification-job.yaml which references `access-key-id` and `secret-access-key`.

## Available Cluster Access Summary

| Cluster | Kubeconfig | Proxy Access | Secret Access | Status |
|---------|-----------|--------------|----------------|--------|
| ord-devimprint | ❌ None | ✅ kubectl-proxy-ord-devimprint:8001 | ❌ Forbidden | BLOCKED |
| rs-manager | ❌ None | ✅ traefik-rs-manager:8001 | ❌ Forbidden | BLOCKED |
| OpenBao | ❌ None | ❌ ClusterIP only | ❌ Not exposed | BLOCKED |

## Commands That FAIL

```bash
# No kubeconfig exists
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
# Error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory

kubectl --kubeconfig=~/.kube/rs-manager.kubeconfig get secret armor-writer -n devimprint  
# Error: stat /home/coding/.kube/rs-manager.kubeconfig: no such file or directory

# Read-only proxies block secret access
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"

kubectl --server=http://traefik-rs-manager:8001 get secret cluster-ord-devimprint -n argocd
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"

# ArgoCD hostname doesn't resolve
curl https://argocd-rs-manager.tail1b1987.ts.net:8080/
# Error: Host argocd-rs-manager.tail1b1987.ts.net not found: 3(NXDOMAIN)
```

## What Would Work (if credentials existed)

```bash
# Option 1: Direct ord-devimprint kubeconfig
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}' | base64 -d

kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-secret-key}' | base64 -d

# Option 2: rs-manager kubeconfig (for OpenBao access)
# Then access OpenBao at rs-manager/ord-devimprint/armor-writer path
```

## Root Cause

The prerequisite bead **bf-2p1wr** (Obtain ord-devimprint kubeconfig with write access) was marked **closed** but never actually obtained the required kubeconfig. The bead completion was incorrectly recorded.

## Acceptance Criteria - NOT MET

- ❌ Successfully retrieved `LITESTREAM_ACCESS_KEY_ID` (which is actually `auth-access-key`) value (base64-decoded)
- ❌ Successfully retrieved `LITESTREAM_SECRET_ACCESS_KEY` (which is actually `auth-secret-key`) value (base64-decoded)  
- ❌ Credentials are stored temporarily in a secure location

## Recommendation

**DO NOT CLOSE this bead.** The task cannot be completed without proper cluster access.

**Required actions:**
1. Re-open bead bf-2p1wr (obtain ord-devimprint kubeconfig)
2. Obtain actual kubeconfig with secret-read access to ord-devimprint OR
3. Obtain kubeconfig with secret-read access to rs-manager (for OpenBao access) OR
4. Receive credentials directly and store them securely

## References

- Prerequisite bead: bf-2p1wr (incorrectly marked closed)
- ExternalSecret config: `~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`
- Cluster documentation: CLAUDE.md (Kubernetes Access section)
- Prior blocker documentation: `notes/bf-2xkyl-blocker-confirmed.md`
