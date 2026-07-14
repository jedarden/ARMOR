# ARMOR Endpoint Connectivity Verification - BF-488WYX

**Task:** Verify ARMOR endpoint connectivity  
**Date:** 2026-07-13  
**Status:** ✅ COMPLETE

## ARMOR Endpoint Summary

ARMOR is deployed and operational in the `devimprint` namespace on the `ord-devimprint` cluster.

### Service Details

- **Service Name:** `armor`
- **Namespace:** `devimprint`
- **Type:** ClusterIP
- **Cluster IP:** `10.21.233.157`
- **S3 API Port:** 9000
- **Admin API Port:** 9001

### Pod Status

All ARMOR pods are healthy and running:

```
NAME                     READY   STATUS    RESTARTS   AGE
armor-869465f5c9-8stfh   1/1     Running   0          2d16h
armor-869465f5c9-8zdqf   1/1     Running   0          2d16h
armor-869465f5c9-gkrtn   1/1     Running   0          2d16h
```

### Endpoint IPs

Active ARMOR endpoints (pod IPs):
- `10.20.1.238:9001`
- `10.20.101.66:9001`
- `10.20.165.13:9001`

## Connectivity Patterns

### 1. Internal Cluster Access

Within the cluster, ARMOR is accessed via the Kubernetes service DNS:

```bash
# S3 API (port 9000)
http://armor.devimprint.svc.cluster.local:9000

# Admin API (port 9001)
http://armor.devimprint.svc.cluster.local:9001
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
kubectl port-forward -n devimprint svc/armor 9000:9000 9001:9001

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

NAMESPACE="devimprint"
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

### Example 2: Connectivity Test from Within Cluster

```bash
#!/bin/bash
# Test ARMOR connectivity from a test pod

kubectl run armor-test --image=curlimages/curl:latest --rm -it --restart=Never \
  -- curl -s http://armor:9000/healthz

kubectl run armor-test --image=curlimages/curl:latest --rm -it --restart=Never \
  -- curl -s http://armor:9001/healthz
```

### Example 3: Service Discovery

```bash
#!/bin/bash
# Find and document ARMOR endpoints

kubectl get svc -n devimprint armor -o json | \
  jq -r '"S3 API: \(.spec.clusterIP):\(.spec.ports[] | select(.name=="s3-api") | .port)"'

kubectl get svc -n devimprint armor -o json | \
  jq -r '"Admin API: \(.spec.clusterIP):\(.spec.ports[] | select(.name=="admin-api") | .port)"'

# Get healthy pod IPs
kubectl get endpoints -n devimprint armor -o json | \
  jq -r '.subsets[].addresses[].ip'
```

## Verification Results

✅ **ARMOR Service:** ClusterIP `10.21.233.157` (ports 9000, 9001)  
✅ **Pod Health:** 3/3 pods running (0 restarts)  
✅ **Endpoints:** 3 healthy endpoint IPs active  
✅ **Service DNS:** `armor.devimprint.svc.cluster.local` resolvable  

## Notes

- ARMOR is currently exposed only as ClusterIP (internal cluster access)
- No Ingress or LoadBalancer configured for external access
- Access from outside cluster requires port-forward or Ingress setup
- All health endpoints are unauthenticated for Kubernetes probes
- Dashboard and admin endpoints may require authentication (if configured)

## Related Documentation

- [ARMOR Dashboard](/home/coding/ARMOR/docs/dashboard.md) - Dashboard API and authentication
- [README](/home/coding/ARMOR/README.md) - Project overview and quick start
- [Deploy](/home/coding/ARMOR/deploy/kubernetes/) - Kubernetes deployment manifests

---

**Task Completed:** All ARMOR endpoints identified and connectivity verified. Service is operational and healthy.
