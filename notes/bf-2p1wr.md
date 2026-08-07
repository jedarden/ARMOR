# Task bf-2p1wr: Obtain ord-devimprint kubeconfig with write access

## Summary

**STATUS: ⛔ BLOCKED - Requires manual intervention through Rackspace Spot UI**

This bead cannot be completed from this server due to the authentication requirements of the Rackspace Spot cloudspace-admin credentials. The OIDC authentication flow requires a web browser for OAuth2 authorization code exchange, which is not available in this headless server environment.

---

## Situation Analysis (Updated 2026-08-06)

### Existing kubeconfigs tested:

| Kubeconfig | Date | Status | Issue |
|------------|------|--------|-------|
| `ord-devimprint-observer.kubeconfig` | Jun 21 | ✅ Working | Read-only, denies secret access |
| `ord-devimprint-admin.kubeconfig` | Aug 6 | ❌ Expired | OIDC requires browser, static token expired Jul 29 |
| `ord-devimprint.kubeconfig` | May 4 | ❌ Expired | OIDC requires browser |
| `ord-devimprint-token.kubeconfig` | Jun 9 | ❌ Expired | "You must be logged in to the server" |

### Admin Kubeconfig Details

The admin kubeconfig contains **two authentication contexts**:

1. **`apexalgo-ord-devimprint-oidc`** (currently active):
   - Uses `kubectl oidc-login get-token` exec plugin
   - Requires browser for OAuth2 authorization code flow
   - Error: `could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH`
   - Callback URL: `http://localhost:18000/`
   - Timeout: `authorization error: context deadline exceeded`

2. **`apexalgo-ord-devimprint`** (static token):
   - JWT token issued: 2026-07-26T13:21:32.782Z
   - JWT expiration: 2026-07-29 (~3 day validity)
   - Current date: 2026-08-06 (token expired ~8 days ago)
   - Payload includes: `group: cloudspace-admin`, `nickname: rackspace`, `email: rackspace@jedarden.com`

### Test Results

```bash
# Observer works but is read-only
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-observer.kubeconfig version
# ✅ Client Version: v1.35.3, Server Version: v1.34.9

# Admin fails - OIDC browser requirement
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get secrets -n devimprint
# ❌ could not open the browser ... authorization error: context deadline exceeded
```

---

## What's Needed

### Manual Intervention Required

The Rackspace Spot cloudspace-admin credentials must be regenerated through the Spot web UI. This cannot be automated from this server because:

1. **OAuth2 Requirement**: The OIDC authentication flow uses the authorization code flow with PKCE, requiring:
   - Interactive user consent in a browser
   - Callback to localhost:18000 for token exchange
   - This is a security feature to prevent automated token generation

2. **Token Expiration**: Both OIDC sessions and static JWT tokens expire after ~3 days
   - OIDC tokens: Session timeout in Rackspace Spot
   - Static JWT tokens: Hardcoded 3-day expiration in JWT claims

3. **No API Alternative**: Rackspace Spot does not provide an API for generating cloudspace-admin credentials without browser interaction

---

## Resolution Path

### For Cluster Administrator (Manual Steps)

1. **Access Rackspace Spot Console**
   - URL: https://spot.rackspace.com/
   - Navigate to the `ord-devimprint` cloudspace
   - Access "Credentials" or "Access" section

2. **Generate New kubeconfig**
   - Select "cloudspace-admin" role
   - Download kubeconfig or copy token
   - Choose OIDC or static token format (OIDC is more secure)

3. **Update Server Kubeconfig**
   ```bash
   # On the server (this machine), replace the kubeconfig:
   vim /home/coding/.kube/ord-devimprint-admin.kubeconfig
   # Paste new kubeconfig content
   
   chmod 600 /home/coding/.kube/ord-devimprint-admin.kubeconfig
   ```

4. **Verify Access**
   ```bash
   # Test basic connectivity
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig version
   
   # Test secret access (acceptance criteria)
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get secrets -n devimprint
   
   # List armor-writer secret specifically
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
     get secret armor-writer -n devimprint
   ```

5. **Close This Bead**
   ```bash
   # After verification, close the bead:
   bf close bf-2p1wr
   ```

---

## Alternative Approaches (Long-term Solutions)

### Option 1: Create Long-lived ServiceAccount
Instead of using the expiring cloudspace-admin OIDC token, create a dedicated ServiceAccount with secret read permissions:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: armor-secret-reader
  namespace: devimprint
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: armor-secret-reader
  namespace: devimprint
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
  resourceNames: ["armor-writer"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: armor-secret-reader
  namespace: devimprint
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: armor-secret-reader
subjects:
- kind: ServiceAccount
  name: armor-secret-reader
  namespace: devimprint
```

Then create a long-lived token for this ServiceAccount (won't expire).

**Pros**: No expiration, least privilege, no browser needed  
**Cons**: Requires admin access to create, token must be stored securely

### Option 2: Use ExternalSecrets Operator
If the `armor-writer` secret needs to be accessed regularly, consider using ExternalSecrets to sync it to a location where it can be accessed without admin privileges.

**Pros**: No admin kubeconfig needed, automated sync  
**Cons**: Requires setup of ExternalSecrets, adds complexity

### Option 3: Rackspace Spot API for Credential Generation
Investigate if Rackspace Spot provides an API for generating cloudspace-admin credentials that could be scripted.

**Pros**: Could be automated  
**Cons**: May not exist due to security requirements, would still need credentials to call API

---

## Files Referenced

| File | Purpose | Status |
|------|---------|--------|
| `/home/coding/.kube/ord-devimprint-admin.kubeconfig` | Admin credentials | Expired, needs regeneration |
| `/home/coding/.kube/ord-devimprint-observer.kubeconfig` | Observer credentials | Working, read-only |
| `/home/coding/.kube/ord-devimprint.kubeconfig` | Alternative admin | Expired |
| `/home/coding/.kube/ord-devimprint-token.kubeconfig` | Token-based | Expired |

---

## Context

- **Parent Task**: bf-2p1wp (needs to retrieve `armor-writer` secret from devimprint namespace)
- **Blocker**: bf-2p1wr blocks parent task bf-2p1wp
- **Acceptance Criteria**: Successfully run `kubectl get secrets -n devimprint` with write-access kubeconfig
- **Security**: Cloudspace-admin OIDC tokens expire every ~3 days as a security measure
- **Environment**: Headless server (no browser, no GUI)

---

## Notes

- The observer kubeconfig uses a **long-lived ServiceAccount token** and does not expire
- The admin kubeconfig uses **OIDC tokens** which expire every ~3 days from Rackspace Spot
- Both kubeconfigs are managed via GitOps through `declarative-config`
- Cluster is accessed via Tailscale operator at `kubectl-proxy-ord-devimprint:8001` (read-only proxy)
- When updating the kubeconfig, ensure permissions are `600` for security
