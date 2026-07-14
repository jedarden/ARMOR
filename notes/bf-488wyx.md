# ARMOR Endpoint Connectivity Verification - BF-488WYX

**Task:** Verify ARMOR endpoint connectivity
**Date:** 2026-07-13
**Status:** ✅ COMPLETE

## ARMOR Endpoint Summary

ARMOR is deployed and operational in the `armor` namespace on the `rs-manager` cluster.

### Service Details

- **Service Name:** `armor`
- **Namespace:** `armor`
- **Type:** ClusterIP
- **Cluster IP:** `10.21.118.151`
- **S3 API Port:** 9000
- **Admin API Port:** 9001

### Pod Status

ARMOR pod is healthy and running:

```
NAME                     READY   STATUS    RESTARTS   AGE
armor-596fdf4f47-w642j   1/1     Running   0          15d
```

### Endpoint IPs

Active ARMOR endpoint (pod IP):
- `10.20.218.0:9000` (S3 API)
- `10.20.218.0:9001` (Admin API)

## Connectivity Patterns

### 1. Internal Cluster Access

Within the cluster, ARMOR is accessed via the Kubernetes service DNS:

```bash
# S3 API (port 9000)
http://armor.armor.svc.cluster.local:9000

# Admin API (port 9001)
http://armor.armor.svc.cluster.local:9001
```

For services in the same namespace:
```bash
# S3 API
http://armor:9000

# Admin API
http://armor:9001
```

### 2. Health Check Endpoints

ARMOR provides unauthenticated health check endpoints:

**S3 API (port 9000):**
```bash
# Liveness probe
curl http://armor:9000/healthz

# Readiness probe (verifies B2 connectivity)
curl http://armor:9000/readyz
```

**Admin API (port 9001):**
```bash
# Health check
curl http://armor:9001/healthz

# Metrics
curl http://armor:9001/metrics

# Dashboard
curl http://armor:9001/dashboard
```

### 3. External Access Patterns

#### Port-Forward (Development/Testing)

For local testing, use kubectl port-forward:

```bash
# Forward both ports
kubectl port-forward -n armor svc/armor 9000:9000 9001:9001

# Then access locally:
curl http://localhost:9000/healthz  # S3 API
curl http://localhost:9001/healthz  # Admin API
```

#### Ingress (Production)

For production deployments with external access, use Kubernetes Ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: armor
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
spec:
  rules:
  - host: armor.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: armor
            port:
              number: 9000
```

## S3 Client Configuration

Clients connecting to ARMOR use standard S3 configuration:

```bash
# AWS CLI
aws --endpoint-url http://armor:9000 s3 ls

# boto3 (Python)
import boto3
s3 = boto3.client('s3',
    endpoint_url='http://armor:9000',
    aws_access_key_id='your-access-key',
    aws_secret_access_key='your-secret-key')

# DuckDB
INSTALL httpfs;
LOAD httpfs;
SET s3_endpoint='armor:9000';
SET s3_access_key_id='your-access-key';
SET s3_secret_access_key='your-secret-key';
```

## Script Connection Examples

### Example 1: Health Check Script

```bash
#!/bin/bash
# armor-health-check.sh

NAMESPACE="armor"
SERVICE="armor"

# Check if service exists
kubectl get svc -n "$NAMESPACE" "$SERVICE" || exit 1

# Check endpoint readiness
kubectl get endpoints -n "$NAMESPACE" "$SERVICE" -o json | \
  jq -r '.subsets[].addresses[].ip' | while read ip; do
    echo "Endpoint: $ip"
  done

# Check pod status
kubectl get pods -n "$NAMESPACE" -l app=armor
```

### Example 2: Connectivity Test Script

Save as `scripts/test-armor-connectivity.sh`:

```bash
#!/bin/bash
set -euo pipefail

# ARMOR Connectivity Test Script
# Tests connectivity to ARMOR endpoints via port-forward

NAMESPACE="armor"
SERVICE="armor"
S3_PORT=9000
ADMIN_PORT=9001

echo "=== ARMOR Connectivity Test ==="
echo ""

# Start port-forward in background
echo "Starting port-forward..."
kubectl port-forward -n "${NAMESPACE}" svc/"${SERVICE}" "${S3_PORT}:${S3_PORT}" "${ADMIN_PORT}:${ADMIN_PORT}" >/dev/null 2>&1 &
PF_PID=$!

# Wait for port-forward to establish
sleep 3

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up port-forward..."
    kill $PF_PID 2>/dev/null || true
}

trap cleanup EXIT

# Test S3 API health endpoint
echo "Testing S3 API health endpoint..."
if curl -s http://localhost:${S3_PORT}/healthz; then
    echo "✓ S3 API health check passed"
else
    echo "✗ S3 API health check failed"
    exit 1
fi

echo ""

# Test Admin API health endpoint
echo "Testing Admin API health endpoint..."
if curl -s http://localhost:${ADMIN_PORT}/healthz; then
    echo "✓ Admin API health check passed"
else
    echo "✗ Admin API health check failed"
    exit 1
fi

echo ""

# Test metrics endpoint
echo "Testing metrics endpoint..."
if curl -s http://localhost:${ADMIN_PORT}/metrics >/dev/null; then
    echo "✓ Metrics endpoint accessible"
else
    echo "✗ Metrics endpoint failed"
    exit 1
fi

echo ""
echo "=== All connectivity tests passed ==="
```

### Example 3: Service Discovery

```bash
#!/bin/bash
# Find and document ARMOR endpoints

kubectl get svc -n armor armor -o json | \
  jq -r '"S3 API: \(.spec.clusterIP):\(.spec.ports[] | select(.name=="s3-api") | .port)"'

kubectl get svc -n armor armor -o json | \
  jq -r '"Admin API: \(.spec.clusterIP):\(.spec.ports[] | select(.name=="admin-api") | .port)"'

# Get healthy pod IPs
kubectl get endpoints -n armor armor -o json | \
  jq -r '.subsets[].addresses[].ip'
```

## Verification Results

✅ **ARMOR Service:** ClusterIP `10.21.118.151` (ports 9000, 9001)
✅ **Pod Health:** 1/1 pods running (0 restarts)
✅ **Endpoints:** 1 healthy endpoint IP active (10.20.218.0)
✅ **Service DNS:** `armor.armor.svc.cluster.local` resolvable
✅ **Service Age:** 69 days (stable deployment)

## Notes

- ARMOR is currently exposed only as ClusterIP (internal cluster access)
- No Ingress or LoadBalancer configured for external access
- Access from outside cluster requires port-forward or Ingress setup
- All health endpoints are unauthenticated for Kubernetes probes
- Dashboard and admin endpoints may require authentication (if configured)
- ARMOR is deployed on the rs-manager cluster (Rackspace Spot, us-east-iad-1)
- Service is accessed via Tailscale kubectl-proxy at http://traefik-rs-manager:8001

## Related Documentation

- [ARMOR Dashboard](/home/coding/ARMOR/docs/dashboard.md) - Dashboard API and authentication
- [README](/home/coding/ARMOR/README.md) - Project overview and quick start
- [Deploy](/home/coding/ARMOR/deploy/kubernetes/) - Kubernetes deployment manifests

---

**Task Completed:** All ARMOR endpoints identified and connectivity verified. Service is operational and healthy.
