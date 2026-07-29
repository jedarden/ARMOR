# ARMOR Admin Token Provisioning for bf-4ngud5

## Generated Tokens

### rs-manager Deployment
- **Token**: `4373a55cf023b8d95d1089d43c9ced9443dd317e485e39a185a00e7d558821a3`
- **OpenBao Path**: `rs-manager/backblaze/armor`
- **Property**: `admin-token`
- **K8s Secret**: `armor-secrets` (namespace: `armor`)

### ord-devimprint Deployment  
- **Token**: `2b9369a06ec8bd20279055a498f88025cd03a26e9d23063c3e1ab06520ddeb24`
- **OpenBao Path**: `rs-manager/ord-devimprint/armor-writer`
- **Property**: `admin-token`
- **K8s Secret**: `armor-credentials` (namespace: `devimprint`)

## Provisioning Steps

### Option 1: Via OpenBao CLI (if available)
```bash
# rs-manager
vault kv patch secret/rs-manager/backblaze/armor admin-token=4373a55cf023b8d95d1089d43c9ced9443dd317e485e39a185a00e7d558821a3

# ord-devimprint  
vault kv patch secret/rs-manager/ord-devimprint/armor-writer admin-token=2b9369a06ec8bd20279055a498f88025cd03a26e9d23063c3e1ab06520ddeb24
```

### Option 2: Via OpenBao API
```bash
# rs-manager
curl -X PATCH \
  -H "X-Vault-Token: <token>" \
  -d '{"data":{"admin-token":"4373a55cf023b8d95d1089d43c9ced9443dd317e485e39a185a00e7d558821a3"}}' \
  https://openbao.rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/backblaze/armor

# ord-devimprint
curl -X PATCH \
  -H "X-Vault-Token: <token>" \
  -d '{"data":{"admin-token":"2b9369a06ec8bd20279055a498f88025cd03a26e9d23063c3e1ab06520ddeb24"}}' \
  https://openbao.rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/armor-writer
```

## Verification Steps

After provisioning, the ExternalSecrets should automatically sync (refreshInterval: 1h). To force immediate sync:

```bash
# For rs-manager
kubectl --kubeconfig=/home/coding/.kube/rs-manager.kubeconfig \
  -n armor get externalsecret armor-secrets

# For ord-devimprint  
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  -n devimprint get externalsecret armor-credentials
```

Check that the secret contains the admin-token:

```bash
# rs-manager
kubectl --kubeconfig=/home/coding/.kube/rs-manager.kubeconfig \
  -n armor get secret armor-secrets -o jsonpath='{.data.admin-token}' | base64 -d

# ord-devimprint
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  -n devimprint get secret armor-credentials -o jsonpath='{.data.admin-token}' | base64 -d
```

## Deployment Rollout

Once tokens are provisioned and synced, the deployments need to be rolled out:

```bash
# rs-manager
kubectl --kubeconfig=/home/coding/.kube/rs-manager.kubeconfig \
  -n armor rollout restart deployment armor

# ord-devimprint
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  -n devimprint rollout restart deployment armor
```

## Security Notes

- Tokens are 64-character hex strings (256 bits) generated with `openssl rand -hex 32`
- These tokens should be treated as secrets - do not commit to git
- After provisioning and rollout, remove this file or move to secure storage
- Consider rotating these tokens on a regular basis
