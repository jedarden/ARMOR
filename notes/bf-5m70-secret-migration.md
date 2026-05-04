# ARMOR Secret Migration - bf-5m70

## Date: 2026-05-02

## Context
Migrating ARMOR from ardenone-hub to ardenone-cluster before hub decommission. The ARMOR secrets exist on ardenone-hub as Kubernetes secrets (synced before OpenBao failure), but the read-only kubectl-proxy prevents secret data extraction.

## Current State

### ardenone-hub (Source)
- Namespace: `devimprint`
- Secrets exist:
  - `devimprint-armor-mek` (master-encryption-key)
  - `devimprint-armor-writer` (auth-access-key, auth-secret-key)
  - `devimprint-armor-readonly` (auth-access-key, auth-secret-key)
  - `devimprint-b2` (id_key, application_key, b2-region, bucket)
- ARMOR deployment: 0/0 replicas (scaled down due to OpenBao failure)

### ardenone-cluster (Target)
- Namespace: `devimprint` (exists, created 69m ago)
- ARMOR deployment: 0/2 replicas (CreateContainerConfigError - secrets not found)
- ExternalSecrets: All in SecretSyncedError state (trying to pull from rs-manager/ord-devimprint/*)

## Secret Data Required

### armor-credentials (combined secret for ARMOR deployment)
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: armor-credentials
  namespace: devimprint
type: Opaque
data:
  # From devimprint-armor-mek
  master-encryption-key: <BASE64_ENCODED_MEK>

  # From devimprint-b2
  b2-access-key-id: <BASE64_ENCODED_B2_KEY_ID>
  b2-secret-access-key: <BASE64_ENCODED_B2_SECRET_KEY>
  b2-region: <BASE64_ENCODED_REGION>
  bucket: <BASE64_ENCODED_BUCKET>

  # From devimprint-armor-writer
  auth-access-key: <BASE64_ENCODED_WRITER_ACCESS_KEY>
  auth-secret-key: <BASE64_ENCODED_WRITER_SECRET_KEY>
```

### armor-readonly (for readonly access)
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: armor-readonly
  namespace: devimprint
type: Opaque
data:
  auth-access-key: <BASE64_ENCODED_READONLY_ACCESS_KEY>
  auth-secret-key: <BASE64_ENCODED_READONLY_SECRET_KEY>
```

## Migration Methods

### Method 1: Direct Secret Copy (RECOMMENDED)
Requires: Direct kubeconfig for ardenone-hub (not through proxy)

```bash
# For each secret:
for secret in devimprint-armor-mek devimprint-armor-writer devimprint-armor-readonly devimprint-b2; do
    kubectl get secret "$secret" -n devimprint -o yaml | \
        grep -v "creationTimestamp\|resourceVersion\|selfLink\|uid\|namespace:" | \
        kubectl apply --kubeconfig=<ardenone-cluster-kubeconfig> -n devimprint -f -
done

# Then create the combined armor-credentials secret
# (Need to manually combine the data from the individual secrets)
```

### Method 2: OpenBaa Direct Access
Requires: OpenBaa token with access to rs-manager/ord-devimprint/* paths

```bash
# Get OpenBaa token from rs-manager
OPENBAO_TOKEN=$(kubectl --kubeconfig=<rs-manager-kubeconfig> get secret -n external-secrets openbao-eso-token -o jsonpath='{.data.token}' | base64 -d)

# Get B2 credentials
curl -H "X-Vault-Token: $OPENBAO_TOKEN" \
    https://openbao-rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/b2

# Get MEK
curl -H "X-Vault-Token: $OPENBAO_TOKEN" \
    https://openbao-rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/armor-mek

# Get writer credentials
curl -H "X-Vault-Token: $OPENBAO_TOKEN" \
    https://openbao-rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/armor-writer

# Get readonly credentials
curl -H "X-Vault-Token: $OPENBAO_TOKEN" \
    https://openbao-rs-manager.tail1b1987.ts.net:8200/v1/secret/data/rs-manager/ord-devimprint/armor-readonly
```

### Method 3: Manual Entry
Requires: Access to the secret values through documentation or other sources

1. Create the secrets manually using `kubectl create secret generic`
2. Apply to ardenone-cluster

## Current Blocker

The read-only kubectl-proxy on ardenone-hub prevents secret data extraction:
- `kubectl get secret -o yaml` returns no data field
- `kubectl get secret -o json` returns empty JSON
- Direct exec into pods also restricted by proxy

## Next Steps

1. **Option A**: Get direct kubeconfig for ardenone-hub (not through proxy)
2. **Option B**: Get OpenBaa token and access secrets via API
3. **Option C**: Manually recreate secrets from documentation/source of truth
4. **Option D**: Use aggregator pod to extract credentials (if accessible)

## Aggregator Connection Details

The aggregator on ardenone-hub connects to ARMOR at:
- Service: `armor-svc`
- Port: `9000`
- Environment variables:
  - `S3_ENDPOINT=http://armor-svc:9000`
  - `S3_BUCKET=devimprint`
  - `S3_ACCESS_KEY_ID` (from devimprint-armor-writer secret)
  - `S3_SECRET_ACCESS_KEY` (from devimprint-armor-writer secret)

After migration, the aggregator needs to be updated to point to the new ARMOR endpoint on ardenone-cluster via Tailscale ingress.
