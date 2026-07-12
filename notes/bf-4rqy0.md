# bf-4rqy0: Base64 Validation Blocked by RBAC

## Task Objective
Validate that the retrieved LITESTREAM_ACCESS_KEY_ID value from the `armor-writer` secret in `ord-devimprint` cluster is properly base64-encoded and non-empty.

## Blocker Identified
The `ord-devimprint` cluster is only accessible via the read-only kubectl-proxy at `http://kubectl-proxy-ord-devimprint:8001`. The observer ServiceAccount (`devpod-observer:devpod-observer`) explicitly **denies access to secrets**.

### Evidence
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Access Configuration (from CLAUDE.md)
The ord-devimprint cluster uses Tailscale operator for kubectl-proxy exposure (no Traefik):
- Proxy runs in `devpod-observer` namespace with read-only RBAC
- Access is **read-only** — cannot create, delete, or modify resources
- **Secrets access is explicitly denied** by the observer's RBAC configuration

## Resolution Required
To complete the base64 validation task, one of the following is needed:

1. **Grant secret read access** to the `devpod-observer` ServiceAccount (least intrusive)
2. **Create a dedicated secret-reader ServiceAccount** with limited secret read permissions
3. **Use direct cluster admin access** via kubeconfig (if one exists for ord-devimprint)

## Prerequisite Status
The task requires completion of child beads:
- bf-4743d
- bf-2pn4n  
- bf-2y15n

These beads should have established secret access, but the current RBAC configuration prevents validation.

## Recommendation
Update the observer's ClusterRole/Role to include `get` on `secrets` resources for the `devimprint` namespace only, following the principle of least privilege.
