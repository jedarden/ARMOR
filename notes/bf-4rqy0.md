# Bead bf-4rqy0: Validate retrieved value is valid base64

## Task
Verify that the retrieved LITESTREAM_ACCESS_KEY_ID value is properly base64-encoded and non-empty.

## Infrastructure Blocker
**Cannot access secret for validation due to RBAC restrictions.**

### Access Attempts

1. **Attempted kubeconfig path** (bf-4743d):
   - Path: `/home/coding/.kube/ord-devimprint.kubeconfig`
   - Result: File does not exist
   - ord-devimprint uses kubectl-proxy over Tailscale, not kubeconfig

2. **Attempted proxy access** (bf-2pn4n, bf-2y15n):
   - Command: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint`
   - Result: **Forbidden**
   - Error: `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

### Root Cause
The `devpod-observer` ServiceAccount has read-only RBAC that explicitly denies access to secrets. This is a security restriction that prevents validation of the secret's base64 encoding.

### Validation Commands Blocked
The following validation commands cannot execute due to the RBAC blocker:

```bash
# Capture the value - BLOCKED by RBAC
VALUE=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}')

# Check non-empty - CANNOT TEST (value not retrieved)
# Validate base64 characters - CANNOT TEST (value not retrieved)
# Attempt decode - CANNOT TEST (value not retrieved)
```

### Access Pattern
According to CLAUDE.md:
- ord-devimprint uses kubectl-proxy over Tailscale
- Proxy runs in `devpod-observer` namespace with read-only RBAC
- Access is **read-only** and does NOT include secret access
- No direct kubeconfig exists for ord-devimprint (only iad-acb and iad-ci available)

### Resolution Path
To complete this validation, one of the following would be needed:
1. Direct kubeconfig with elevated permissions to ord-devimprint cluster
2. RBAC modification to grant devpod-observer SA secret read access in devimprint namespace
3. Alternative validation method that doesn't require direct secret access

### Re-verification History
- **2026-07-11 23:59 UTC**: RBAC blocker confirmed - kubectl-proxy returns Forbidden error for secret access. No kubeconfig available.
- **2026-07-11 23:57 UTC**: RBAC blocker confirmed - kubectl-proxy returns Forbidden error for secret access. No kubeconfig available.
- **2026-07-11 19:56 UTC**: RBAC blocker persists - no admin kubeconfig available (commit 9879d3d9)

### Status
- **Prerequisites**: All child beads (bf-4743d, bf-2pn4n, bf-2y15n) are closed
- **Blocker**: RBAC denies secret access
- **Validation**: Cannot proceed without secret access
- **Bead Status**: OPEN - awaiting infrastructure changes

### Related Documentation
- Git commit 9879d3d9: "docs(bf-4rqy0): re-verify RBAC blocker persists - no admin kubeconfig available"
- Git commit 03fb00e5: "docs(bf-4rqy0): document current state - RBAC blocker prevents validation completion"
- Git commit 3c50a542: "docs(bf-4rqy0): re-verify RBAC blocker persists - no kubeconfig available, validation impossible"
- Git commit 89eecb6f: "docs(bf-4rqy0): document RBAC blocker preventing base64 validation of LITESTREAM_ACCESS_KEY_ID"
- Git commit 8c9de496: "docs(bf-2y15n): document infrastructure blocker - ord-devimprint proxy denies secret access"

## Additional Finding: Property Name Mismatch (2026-07-12 00:13 UTC)

### Configuration Issue Discovered

Beyond the RBAC blocker, there's a **configuration mismatch** in the ExternalSecret:

**ExternalSecret armor-writer syncs:**
- `auth-access-key` (from OpenBao) → `auth-access-key` (in Kubernetes secret)
- `auth-secret-key` (from OpenBao) → `auth-secret-key` (in Kubernetes secret)

**Beads are attempting to retrieve:**
- `LITESTREAM_ACCESS_KEY_ID`
- `LITESTREAM_SECRET_ACCESS_KEY`

**The property names do not match!** The beads are looking for keys that don't exist in the ExternalSecret configuration.

### Verification

```bash
# Checked ExternalSecret properties
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get externalsecret armor-writer -n devimprint \
  -o jsonpath='{.spec.data[*].secretKey}'
# Output: auth-access-key auth-secret-key

# Searched for litestream properties - none found
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get externalsecrets -n devimprint -o json | grep -i litestream
# No output
```

### ExternalSecret Health

Despite the configuration issue, the ExternalSecret reports:
- Status: `SecretSynced`
- Ready: `True`
- Last sync: ~37 minutes ago

The secret is syncing successfully, but it's not pulling the Litestream credentials.

### Root Cause Analysis

This appears to be one of:
1. **Wrong ExternalSecret** - The beads reference the wrong secret
2. **Missing properties** - The ExternalSecret spec needs to be updated to include Litestream keys
3. **OpenBao path mismatch** - Litestream credentials might be stored under a different OpenBao key path
4. **Secret naming confusion** - The credentials might be in a different secret entirely

### Combined Blockers

This task is blocked by **two independent issues**:
1. **RBAC** - Cannot directly access secrets to verify contents
2. **Configuration** - The ExternalSecret doesn't reference the correct properties

### Resolution Required

To unblock this task, both issues need resolution:
1. **Fix configuration**: Update ExternalSecret to include LITESTREAM_* properties, or determine correct secret/keys
2. **Fix RBAC**: Obtain kubeconfig with secret access, or modify RBAC to allow proxy SA to read secrets

## Re-verification (2026-07-11)

### Current State Verification

Re-verified both blockers persist:

1. **RBAC Blocker Confirmed**:
   - Cannot access secret: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint`
   - Error: `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"`
   - No kubeconfig file exists: `/home/coding/.kube/ord-devimprint.kubeconfig` (not found)

2. **Configuration Mismatch Confirmed**:
   - ExternalSecret properties: `auth-access-key`, `auth-secret-key`
   - Beads attempting to retrieve: `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`
   - Property names do not match
   - No litestream properties found in ExternalSecret configuration

### Validation Cannot Proceed

Without access to:
1. The secret itself (RBAC blocked)
2. The correct property names (configuration mismatch)

The base64 validation task cannot be completed as specified. The task requires retrieving `LITESTREAM_ACCESS_KEY_ID` but that property does not exist in the ExternalSecret.

### Recommendation

Before this bead can be completed:
1. Determine correct secret/property names for Litestream credentials
2. Obtain kubeconfig with secret access OR modify ExternalSecret to include correct properties
3. Re-validate the bead task specification matches the actual infrastructure

## Re-verification (2026-07-12 00:18 UTC)

### ExternalSecret Specification Analysis

Re-examined the `armor-writer` ExternalSecret specification in detail:

```yaml
spec:
  data:
  - remoteRef:
      key: rs-manager/ord-devimprint/armor-writer
      property: auth-access-key
    secretKey: auth-access-key
  - remoteRef:
      key: rs-manager/ord-devimprint/armor-writer
      property: auth-secret-key
    secretKey: auth-secret-key
```

**Critical Finding**: The ExternalSecret specification does NOT include any reference to `LITESTREAM_ACCESS_KEY_ID` or similar Litestream-related properties. Only two keys are being synced from OpenBao:
1. `auth-access-key` (OpenBao property) → `auth-access-key` (Kubernetes secret key)
2. `auth-secret-key` (OpenBao property) → `auth-secret-key` (Kubernetes secret key)

### Implications

The bead task specifies retrieving `LITESTREAM_ACCESS_KEY_ID` from the `armor-writer` secret, but this key:
1. Does not exist in the ExternalSecret specification
2. Cannot be synced from OpenBao (no remoteRef for it)
3. Will not exist in the resulting Kubernetes secret

### Conclusion

This is a **specification mismatch** between the bead task requirements and the actual ExternalSecret configuration. Before the validation can proceed, one of the following must occur:

1. **Update the ExternalSecret** in `declarative-config` to include the Litestream properties:
   ```yaml
   - remoteRef:
       key: rs-manager/ord-devimprint/armor-writer
       property: litestream-access-key-id  # or actual OpenBao property name
     secretKey: LITESTREAM_ACCESS_KEY_ID
   ```

2. **Update the bead task** to reference the correct property names that match the ExternalSecret spec

3. **Determine the correct secret** - Litestream credentials might be stored in a different ExternalSecret/secret entirely

### ExternalSecret Status (Confirmed)
- Name: `armor-writer`
- Namespace: `devimprint`
- Status: Ready (True)
- Reason: SecretSynced
- Last refresh: 2026-07-11T23:21:25Z
- Synced properties: `auth-access-key`, `auth-secret-key` only

## Timestamp
2026-07-12 00:18 UTC - Configuration mismatch confirmed; ExternalSecret does not include LITESTREAM_ACCESS_KEY_ID property
