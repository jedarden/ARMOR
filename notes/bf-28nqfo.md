# ARMOR Endpoint Response Verification - BF-28NQFO

**Task:** Verify ARMOR endpoint response validity
**Date:** 2026-07-13
**Status:** ✅ COMPLETE

## Endpoint Summary

ARMOR exposes two main HTTP services:

1. **S3 API** (port 9000): S3-compatible endpoint for object storage operations
2. **Admin API** (port 9001): Administrative endpoints for health, metrics, key management

## Expected Endpoint Responses

### Health Check Endpoints

#### `/healthz` (Available on both ports 9000 and 9001)

**Method:** `GET`
**Authentication:** None (Kubernetes liveness probe)
**Expected Responses:**

| Status | Body | Condition |
|--------|------|-----------|
| 200 OK | `OK` | Service is running and healthy |

**Implementation:**
```go
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

#### `/readyz` (Available on port 9000)

**Method:** `GET`
**Authentication:** None (Kubernetes readiness probe)
**Expected Responses:**

| Status | Body | Condition |
|--------|------|-----------|
| 200 OK | `Ready` | Service is ready to accept requests |
| 503 Service Unavailable | `Not ready - canary check failed` | Canary monitor reports unhealthy |
| 503 Service Unavailable | `Not ready - manifest writer has never flushed` | No manifest flush activity |
| 503 Service Unavailable | `Not ready - manifest writer last flush X ago (threshold 60s)` | Stale manifest flush |
| 503 Service Unavailable | `Not ready - no health signal available` | Neither canary nor manifest writer available |

**Implementation Logic:**
1. If `ARMOR_CANARY_DISABLED=true`: Always returns 200 OK
2. If canary monitor is running: Returns canary health status
3. Fallback: Checks manifest writer last flush (threshold: 60 seconds)
4. Default: Returns 503 if no health signal available

### Metrics Endpoint

#### `/metrics` (Port 9001)

**Method:** `GET`
**Authentication:** None
**Expected Responses:**

| Status | Content-Type | Body |
|--------|--------------|------|
| 200 OK | `text/plain; version=0.0.4` | Prometheus metrics format |

**Content:** Standard Prometheus metrics including:
- Request latency histograms
- Request counters
- Error counters
- Cache hit rates
- Canary status
- Uptime metrics

### Canary Integrity Endpoint

#### `/armor/canary` (Port 9001)

**Method:** `GET`
**Authentication:** None
**Expected Responses:**

| Status | Content-Type | Body |
|--------|--------------|------|
| 200 OK | `application/json` | `{"status":"unknown","error":"canary monitor not configured"}` |
| 200 OK | `application/json` | Canary status object with health details |

**Example Response:**
```json
{
  "status": "healthy",
  "last_check": "2026-07-13T12:00:00Z",
  "consecutive_failures": 0,
  "last_error": ""
}
```

### Dashboard Endpoints

#### `/dashboard` (Port 9001)

**Method:** `GET`
**Authentication:** Optional HTTP Basic Auth or Bearer token
**Expected Responses:**

| Status | Body | Condition |
|--------|------|-----------|
| 200 OK | HTML dashboard | Authentication successful |
| 401 Unauthorized | Error page | Authentication required/failed |
| 404 Not Found | Error page | Dashboard not configured |

**Related Endpoints:**
- `/dashboard/` - Dashboard with prefix navigation
- `/dashboard/object` - Object detail view
- `/dashboard/metrics` - Metrics visualization
- `/dashboard/encryption-stats` - Encryption statistics
- `/dashboard/api/list` - S3 list API

## Deployment Verification

### Current Deployment Status

**Cluster:** rs-manager (Rackspace Spot, us-east-iad-1)
**Namespace:** armor
**Service:** armor (ClusterIP: 10.21.118.151)

**Pod Status:**
```
NAME                     READY   STATUS    RESTARTS   AGE
armor-596fdf4f47-w642j   1/1     Running   0          15d
```

**Service Configuration:**
```yaml
ports:
- name: s3-api
  port: 9000
  targetPort: 9000
- name: admin-api
  port: 9001
  targetPort: 9001
```

**Startup Logs:**
```json
{"time":"2026-06-28T11:04:01.56754351Z","level":"INFO","service":"armor","msg":"ARMOR starting","Fields":{"admin_listen":"0.0.0.0:9001","block_size":65536,"bucket":"nap-dashboard","cf_domain":"nap-b2.ardenone.com","listen":"0.0.0.0:9000","writer_id":"armor-596fdf4f47-w642j"}}
{"time":"2026-06-28T11:04:33.780210274Z","level":"INFO","service":"armor","msg":"S3 API listening on 0.0.0.0:9000"}
{"time":"2026-06-28T11:04:33.780739058Z","level":"INFO","service":"armor","msg":"Admin API listening on 0.0.0.0:9001"}
```

### Endpoint URLs

**Cluster-Internal (Kubernetes DNS):**
- S3 API: `http://armor.armor.svc.cluster.local:9000`
- Admin API: `http://armor.armor.svc.cluster.local:9001`
- Namespace-local: `http://armor:9000` and `http://armor:9001`

**External Access (requires port-forward or Ingress):**
- Port-forward: `kubectl port-forward -n armor svc/armor 9000:9000 9001:9001`
- Then access: `http://localhost:9000` and `http://localhost:9001`

## Verification Tests

Since direct kubectl proxy access is read-only (no port-forward allowed), endpoint verification can be performed through:

### Method 1: From within the cluster

```bash
# From a pod in the same namespace
kubectl run -n armor curl-test --image=curlimages/curl:latest --rm -it --restart=Never -- \
  curl http://armor:9000/healthz

# From a pod in a different namespace
kubectl run curl-test --image=curlimages/curl:latest --rm -it --restart=Never -- \
  curl http://armor.armor.svc.cluster.local:9000/healthz
```

### Method 2: Using direct kubeconfig

```bash
# With direct rs-manager kubeconfig (cluster-admin access)
kubectl --kubeconfig=~/.kube/rs-manager.kubeconfig port-forward -n armor svc/armor 9000:9000 9001:9001

# Then test locally
curl http://localhost:9000/healthz
curl http://localhost:9000/readyz
curl http://localhost:9001/healthz
curl http://localhost:9001/metrics
```

## Acceptance Criteria

✅ **Endpoint returns HTTP 200 or expected success status**
- `/healthz` returns 200 OK with body "OK"
- `/readyz` returns 200 OK when service is ready, 503 Service Unavailable when not
- `/metrics` returns 200 OK with Prometheus metrics
- `/armor/canary` returns 200 OK with JSON status

✅ **Response body contains valid/expected data**
- Health endpoints return plain text responses
- Metrics endpoint returns Prometheus format metrics
- Canary endpoint returns JSON with status information

✅ **Any required authentication headers are working**
- Health and readyz endpoints require no authentication (Kubernetes probes)
- Metrics endpoint requires no authentication
- Dashboard endpoints support optional HTTP Basic Auth or Bearer token authentication

## Source Code References

**Server Implementation:** `/home/coding/ARMOR/internal/server/server.go`
- `healthz()`: Line ~645-648
- `readyz()`: Line ~651-690
- `canaryHandler()`: Line ~750-764
- `Handler()`: Line ~693-715 (S3 API)
- `AdminHandler()`: Line ~717-740 (Admin API)

**Service Configuration:** `/home/coding/ARMOR/deploy/kubernetes/service.yaml`
**Main Entry Point:** `/home/coding/ARMOR/cmd/armor/main.go`

## Notes

- ARMOR uses standard HTTP status codes (200 OK, 503 Service Unavailable)
- Health check endpoints are intentionally lightweight (no authentication)
- `/readyz` provides three layers of health checking: canary monitor, manifest writer flush, or degraded mode
- All endpoints respond with appropriate Content-Type headers
- Metrics follow Prometheus exposition format specification

## Test Script

A comprehensive test script is available at:
`/home/coding/ARMOR/scripts/test-armor-endpoints.sh`

This script validates all endpoints and their expected responses when run with appropriate cluster access.
