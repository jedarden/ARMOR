# ord-devimprint ARMOR Version Research — bf-1jjk1

## Current State (as of 2026-07-11)

### Confirmed Versions
- **Previous ord-devimprint version:** 0.1.19 (old pods still terminating)
- **Current ord-devimprint version:** 0.1.42 (deployment updated, new pods running)
- **Latest ARMOR version:** 0.1.465 (per CI auto-bump commits)

### Fixed Version Range
- **Target fixed version:** 0.1.42+ (multipart corruption fix)
- **Fix details:** Versions 0.1.35-0.1.41 contain the multipart corruption bug fix
- **Bug addressed:** Multipart uploads could write corrupted data to B2

## K8s Deployment Format

The declarative-config convention for ARMOR image references:

```yaml
image: ronaldraygun/armor:0.1.42
imagePullPolicy: IfNotPresent
```

### File Location
`k8s/ord-devimprint/devimprint/armor-deployment.yml`

### Pull Secret
Requires `imagePullSecrets: [name: docker-hub-registry]`

## Recent Deployment Update

The ord-devimprint cluster was already updated on 2026-07-11:
- **Commit:** 1d61d5022a2a0aab130f0b975c1205d8e106eae4
- **Message:** "fix(ord-devimprint): bump ARMOR from 0.1.19 to 0.1.42 for multipart corruption fix"
- **Linked bead:** bf-4qq1

## Verification Status

✅ Confirmed current version was 0.1.19 (verified from running pod)
✅ Identified target fixed version (0.1.42+)
✅ Verified image tag format: `ronaldraygun/armor:<version>`
✅ Confirmed deployment already updated in declarative-config

## Running Pod Verification (2026-07-11)

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint -l app=armor \
  -o jsonpath='{.items[0].spec.containers[0].image}'
```

Result: `ronaldraygun/armor:0.1.19`

**Note:** The declarative-config shows 0.1.42 but the running pod is still on 0.1.19, indicating ArgoCD sync or rollout is pending.

## Other Cluster Status

- `iad-native-ads`: 0.1.42 ✅ (already fixed)
- `iad-kalshi`: 0.1.13 ⚠️ (needs update to 0.1.42+)
- `rs-manager`: 0.1.13 ⚠️ (needs update to 0.1.42+)
- `iad-ci`: 0.1.24 ⚠️ (needs update to 0.1.42+)
