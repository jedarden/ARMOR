# Multipart-Era Corruption Audit - Credential Access Status

**Bead**: bf-1ebnuz
**Date**: 2026-08-03
**Status**: CREDENTIAL-GATED - Audit blocked by lack of valid credentials

## Summary

The multipart-era corruption audit of unaudited ARMOR buckets is **CREDENTIAL-GATED**. This audit requires live B2 credentials or ARMOR HTTP access with valid authentication tokens, which are not available in the current environment.

## Affected Buckets

The audit targets 4 unaudited buckets that were potentially exposed to the multipart corruption bug (versions 0.1.35-0.1.41, fixed in 0.1.42+):

| Bucket | Cluster | Current Version | Risk Level | Access Status |
|--------|---------|-----------------|------------|---------------|
| armor-apexalgo | iad-acb | fcbf6d3 (unknown) | CRITICAL | CREDENTIAL-GATED |
| iad-ci | iad-ci | 0.1.24 | MEDIUM | CREDENTIAL-GATED |
| iad-kalshi | iad-kalshi | 0.1.13 | MEDIUM | CREDENTIAL-GATED |
| rs-manager | rs-manager | 0.1.13 | MEDIUM | CREDENTIAL-GATED |

## Credential Access Analysis

### iad-ci Cluster
**Kubeconfig**: `/home/coding/.kube/iad-ci.kubeconfig`
- **Status**: ❌ EXPIRED TOKEN
- **Authentication**: ServiceAccount token (`argocd-manager`)
- **Error**: "the server has asked for the client to provide credentials"
- **Memory Status**: iad-ci is "audit-ready (0.1.1901, creds in armor-secrets)" but we lack access to those secrets
- **Required**: Renewed ServiceAccount token or B2 credentials

### armor-apexalgo (iad-acb) Cluster  
**Kubeconfig**: `/home/coding/.kube/iad-acb.kubeconfig`
- **Status**: ❌ UNREACHABLE
- **Authentication**: Tailscale proxy (`http://traefik-iad-acb:8001`) - no auth required
- **Error**: Timeout - cluster endpoint not reachable from current environment
- **Required**: Valid Tailscale connection or B2 credentials

### iad-kalshi Cluster
**Status**: ❌ CREDENTIAL-GATED
- **Access Method**: kubectl-proxy over Tailscale (`http://kubectl-proxy-iad-kalshi:8001`)
- **Permissions**: Read-only only (explicitly denies secret access)
- **Issue**: Read-only proxy cannot access ARMOR auth credentials (stored as secrets)
- **Required**: Direct kubeconfig with secret access OR B2 credentials

### rs-manager Cluster
**Status**: ❌ CREDENTIAL-GATED  
- **Access Method**: Should have read/write kubeconfig at `/home/coding/.kube/rs-manager.kubeconfig`
- **Issue**: Kubeconfig not present in environment
- **Required**: Deploy rs-manager kubeconfig OR B2 credentials

## Why This Audit Cannot Be Automated

The corruption audit requires:

1. **Object enumeration**: List all objects >5MiB with creation timestamps
2. **Version window cross-reference**: Match object timestamps against affected deployment windows  
3. **HMAC verification**: Use `armor-decrypt` to verify each candidate object

Each step requires **authenticated access** to the ARMOR HTTP endpoint or B2 API:

- **Step 1**: Needs ARMOR HTTP API auth (via `armor ls`) or B2 list access
- **Step 2**: Needs armor_deployments.json (✅ available) + object timestamps (from Step 1)
- **Step 3**: Needs ARMOR HTTP API auth (via `armor-decrypt`) or B2 download + MEK

Without valid credentials, **none of these steps can execute**.

## What an Operator Must Provide

To complete this audit, an operator must supply **one of the following access methods for each bucket**:

### Method 1: B2 Credentials (Preferred)

```bash
export ARMOR_B2_REGION="us-east-005"  # or appropriate region
export ARMOR_B2_ENDPOINT="https://s3.us-east-005.backblazeb2.com"
export ARMOR_B2_ACCESS_KEY_ID="your-key-id"
export ARMOR_B2_SECRET_ACCESS_KEY="your-secret-key"
```

### Method 2: ARMOR HTTP Credentials via Valid Kubeconfig

For each cluster, provide a kubeconfig with secret access (not read-only observer):

```bash
# iad-ci
kubectl --kubeconfig=/path/to/valid-iad-ci.kubeconfig get secret -n armor armor-secrets

# armor-apexalgo (iad-acb)  
kubectl --kubeconfig=/path/to/valid-iad-acb.kubeconfig get secret -n armor armor-secrets

# iad-kalshi
kubectl --kubeconfig=/path/to/valid-iad-kalshi.kubeconfig get secret -n armor armor-secrets

# rs-manager
kubectl --kubeconfig=/path/to/valid-rs-manager.kubeconfig get secret -n armor armor-secrets
```

Then extract ARMOR auth credentials:
```bash
export ARMOR_AUTH_ACCESS_KEY=$(kubectl get secret -n armor armor-secrets -o jsonpath='{.data.access-key}' | base64 -d)
export ARMOR_AUTH_SECRET_KEY=$(kubectl get secret -n armor armor-secrets -o jsonpath='{.data.secret-key}' | base64 -d)
export ARMOR_MEK=$(kubectl get secret -n armor armor-secrets -o jsonpath='{.data.mek}' | base64 -d)
```

### Method 3: Port-Forward with Valid Kubeconfig

If kubeconfigs are available but direct HTTP access is blocked:

```bash
# Terminal 1: iad-ci
kubectl --kubeconfig=/path/to/valid-iad-ci.kubeconfig port-forward -n armor svc/armor 9000:9000

# Terminal 2: armor-apexalgo (iad-acb)
kubectl --kubeconfig=/path/to/valid-iad-acb.kubeconfig port-forward -n armor svc/armor 9004:9000

# Terminal 3: iad-kalshi  
kubectl --kubeconfig=/path/to/valid-iad-kalshi.kubeconfig port-forward -n armor svc/armor 9001:9000

# Terminal 4: rs-manager
kubectl --kubeconfig=/path/to/valid-rs-manager.kubeconfig port-forward -n armor svc/armor 9002:9000
```

Then use ARMOR with `--endpoint http://localhost:PORT` for each cluster.

## Complete Audit Procedure (For Operator)

Once credentials are available, follow this procedure:

### Step 1: Enumerate Objects >5MiB

For each bucket, enumerate large objects with timestamps:

```bash
# Using B2 credentials
b2 ls --long --recursive <bucket-name> | awk '$5 > 5242880 {print $0}'

# Or using ARMOR HTTP API  
armor --endpoint http://localhost:PORT ls --recursive | jq '.[] | select(.size > 5242880) | {key: .key, size: .size, modified: .last_modified}'
```

Save output to `<bucket>_large_objects.json`

### Step 2: Cross-Reference Against Deployment Windows

Load `armor_deployments.json` and match object timestamps against affected version windows:

```python
import json
from datetime import datetime

# Load deployment data
with open('armor_deployments.json') as f:
    deployments = json.load(f)['simplified_mapping']

# Affected versions
AFFECTED_VERSIONS = ['0.1.35', '0.1.36', '0.1.37', '0.1.38', '0.1.39', '0.1.40', '0.1.41']
FIXED_VERSION = '0.1.42'

# For each large object, check if it was written during affected window
# This requires parsing deployment timestamps from git history or logs
```

### Step 3: Verify Candidates with armor-decrypt

For each object written during the affected window:

```bash
# Using ARMOR HTTP API
armor-decrypt \
  --endpoint http://localhost:PORT \
  --access-key "$ARMOR_AUTH_ACCESS_KEY" \
  --secret-key "$ARMOR_AUTH_SECRET_KEY" \
  --mek "$ARMOR_MEK" \
  --bucket <bucket-name> \
  --key <object-key> \
  --verify-only
```

### Step 4: Generate Corruption Inventory

Document results in the inventory template format:

```markdown
| Bucket | Object Key | Size | Created During Affected Window | Verification Status | Action Required |
|--------|------------|------|-------------------------------|---------------------|-----------------|
| iad-ci | path/to/object | 10MB | Yes | VERIFIED_CLEAN | None |
| iad-kalshi | path/to/other | 25MB | Yes | CORRUPTED | RE_UPLOAD |
```

## Existing Documentation

The following documentation exists and should be used when credentials become available:

- **Audit Guide**: `docs/bf-659opq-corruption-audit-guide.md` - Complete step-by-step instructions
- **Inventory Template**: `docs/bf-659opq-corruption-inventory-template.md` - Expected output format
- **Deployment Data**: `armor_deployments.json` - Current version mappings for all clusters
- **ADR**: `docs/adr/002-multipart-corruption-detection-gaps.md` - Full incident analysis

## Recommendation

This bead should remain **OPEN** until an operator can provide valid credentials. Once credentials are available:

1. Run the comprehensive audit framework: `scripts/corruption-audit-framework.py`
2. Generate corruption inventory
3. Create remediation plan for any corrupted objects found
4. Close the bead with full documentation

## Conclusion

The multipart-era corruption audit is **well-documented and ready to execute** but completely **blocked by credential access**. No automated or unattended worker can complete this task without live B2 or ARMOR HTTP credentials that must be provided by an operator with cluster access.

The audit framework, documentation, and procedures are all in place - only the authentication tokens are missing.
