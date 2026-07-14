# ARMOR Endpoint URL Documentation

## Task: Identify ARMOR endpoint URL

**Date:** 2026-07-13  
**Bead ID:** bf-65daai

---

## Summary

The ARMOR endpoint URL is configured through multiple layers depending on the deployment context. This document identifies all endpoint configurations and their sources.

---

## ARMOR Endpoint URLs

### 1. Container Internal Configuration

**Source:** `/home/coding/ARMOR/deploy/kubernetes/deployment.yaml`

ARMOR service listens on two ports within the container:

| Port | Protocol | Purpose | Environment Variable |
|------|----------|---------|---------------------|
| **9000** | HTTP | S3-compatible API | `ARMOR_LISTEN="0.0.0.0:9000"` |
| **9001** | HTTP | Admin API / Dashboard | `ARMOR_ADMIN_LISTEN="0.0.0.0:9001"` |

**Code reference:**
```yaml
# From deployment.yaml
- name: ARMOR_LISTEN
  value: "0.0.0.0:9000"
- name: ARMOR_ADMIN_LISTEN
  value: "0.0.0.0:9001"
```

---

### 2. Kubernetes Service Configuration

**Source:** `/home/coding/ARMOR/deploy/kubernetes/service.yaml`

ARMOR is exposed via Kubernetes Services:

| Service Name | Type | Port | Target Port | Purpose |
|-------------|------|------|-------------|---------|
| `armor` | ClusterIP | 9000 | 9000 | S3 API |
| `armor` | ClusterIP | 9001 | 9001 | Admin API |
| `armor-headless` | ClusterIP (None) | 9000 | 9000 | S3 API (StatefulSet) |
| `armor-headless` | ClusterIP (None) | 9001 | 9001 | Admin API (StatefulSet) |

**Cluster-internal endpoint URL:** `http://armor:9000`

---

### 3. Ingress Configuration (Dashboard)

**Source:** `/home/coding/ARMOR/deploy/kubernetes/ingress-dashboard.yaml`

The ARMOR web dashboard has an Ingress configuration:

| Setting | Value |
|---------|-------|
| **Host** | `armor-dashboard.example.com` (placeholder) |
| **Backend Service** | `armor` |
| **Backend Port** | 9001 (Admin API) |
| **Path** | `/` |

**External endpoint URL:** `https://armor-dashboard.example.com` (requires proper TLS cert and domain configuration)

**Note:** The Ingress configuration uses a placeholder domain. Actual deployments should update this to the proper hostname.

---

### 4. Client Configuration Examples

**Source:** `/home/coding/ARMOR/README.md`

Clients connect to ARMOR using the S3-compatible endpoint:

#### AWS CLI
```bash
aws --endpoint-url http://localhost:9000 s3 cp file.txt s3://bucket/key
```

#### boto3 (Python)
```python
import boto3
s3 = boto3.client('s3',
    endpoint_url='http://localhost:9000',
    aws_access_key_id='my-access-key',
    aws_secret_access_key='my-secret-key')
```

#### DuckDB
```sql
INSTALL httpfs;
LOAD httpfs;
SET s3_endpoint='localhost:9000';
SET s3_access_key_id='my-access-key';
SET s3_secret_access_key='my-secret-key';
SELECT * FROM read_parquet('s3://bucket/data.parquet');
```

---

## Endpoint URL Sources Summary

| Context | Endpoint URL | Source File | Source Type |
|---------|--------------|-------------|-------------|
| **Docker local** | `http://localhost:9000` | README.md | Documentation |
| **Kubernetes internal** | `http://armor:9000` | service.yaml + deployment.yaml | K8s Service + Env |
| **Kubernetes external** | `https://<configured-domain>` | ingress-dashboard.yaml | Ingress |
| **Admin/Dashboard** | `http://localhost:9001` or `http://armor:9001` | deployment.yaml | Env var |

---

## Environment Variables Controlling Endpoint

| Variable | Default | Description |
|----------|---------|-------------|
| `ARMOR_LISTEN` | `0.0.0.0:9000` | S3 API listen address |
| `ARMOR_ADMIN_LISTEN` | `0.0.0.0:9001` | Admin API listen address |

---

## Deployment-Specific Endpoints (from notes)

Based on historical deployment notes, ARMOR has been accessed at:

| Cluster | Endpoint URL | Access Method |
|---------|--------------|---------------|
| **ord-devimprint** | `http://100.80.255.8:9000` | Direct Tailscale IP |
| **ord-devimprint** (internal) | `http://armor:9000` | Cluster-internal service |
| **ardenone-hub** | `http://armor:9000` | Cluster-internal service |

---

## Verification Commands

To verify ARMOR endpoint connectivity:

```bash
# From within Kubernetes cluster
curl http://armor:9000/healthz
curl http://armor:9001/healthz

# From localhost (with port-forward)
kubectl port-forward svc/armor 9000:9000
curl http://localhost:9000/healthz

# Check dashboard
curl http://localhost:9001/dashboard
```

---

## Acceptance Criteria Met

✅ **ARMOR endpoint URL is located in config or code** - Found in multiple configuration files  
✅ **URL is documented** - This document provides comprehensive endpoint documentation  
✅ **Source of URL is identified** - All sources (deployment.yaml, service.yaml, ingress-dashboard.yaml, README.md) are documented

---

## Related Configuration Files

- `/home/coding/ARMOR/deploy/kubernetes/deployment.yaml` - Container port configuration
- `/home/coding/ARMOR/deploy/kubernetes/service.yaml` - Kubernetes Service definitions
- `/home/coding/ARMOR/deploy/kubernetes/ingress-dashboard.yaml` - Ingress configuration
- `/home/coding/ARMOR/README.md` - Client usage examples

---

**Task Status:** ✅ COMPLETE  
**Documentation created:** `/home/coding/ARMOR/notes/bf-65daai-endpoint-documentation.md`
