# ARMOR Web Dashboard

The ARMOR web dashboard provides a visual interface for browsing encrypted buckets, viewing encryption status, and monitoring cache statistics.

## Accessing the Dashboard

### Local Development

When running ARMOR locally, the dashboard is available on the admin port (default `127.0.0.1:9001`):

```bash
curl http://localhost:9001/dashboard
```

Or open in a browser:
```
http://localhost:9001/dashboard
```

### Kubernetes Deployment

The dashboard is exposed via the admin-api service on port 9001:

```bash
# Port-forward to access locally
kubectl port-forward svc/armor 9001:9001

# Then open in browser
open http://localhost:9001/dashboard
```

### External Access via Ingress

For production deployments, you can expose the dashboard externally using Kubernetes Ingress. Create an Ingress resource that routes to the admin-api port:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: armor-dashboard
  annotations:
    # Use your Ingress controller annotations
    # For NGINX:
    nginx.ingress.kubernetes.io/rewrite-target: /
    # For Traefik:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
spec:
  ingressClassName: nginx
  rules:
  - host: armor-dashboard.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: armor
            port:
              number: 9001
  tls:
  - hosts:
    - armor-dashboard.example.com
    secretName: armor-dashboard-tls
```

## Dashboard Authentication

The dashboard supports two authentication methods to protect against unauthorized access.

### HTTP Basic Authentication

Set username and password via environment variables:

```yaml
env:
  - name: ARMOR_DASHBOARD_USER
    value: "admin"
  - name: ARMOR_DASHBOARD_PASS
    value: "secure-password-here"
```

When accessing the dashboard, provide credentials:
```bash
# Using curl
curl -u admin:secure-password-here http://localhost:9001/dashboard

# Browser will prompt for username/password
```

### Bearer Token Authentication

Alternatively, use a bearer token:

```yaml
env:
  - name: ARMOR_DASHBOARD_TOKEN
    value: "your-secure-token-here"
```

When accessing the dashboard, provide the token in the Authorization header:
```bash
curl -H "Authorization: Bearer your-secure-token-here" http://localhost:9001/dashboard
```

### Security Notes

- If neither authentication method is configured, the dashboard is open to anyone with network access to the admin port
- Health check endpoints (`/healthz`, `/readyz`) remain unauthenticated for Kubernetes probes
- For production deployments, always enable authentication or use network policies to restrict access
- Consider using a strong random password or token generated with `openssl rand -hex 32`

## Dashboard Features

### 1. Bucket Browsing

Browse objects in your encrypted bucket with prefix-based navigation:

- **Breadcrumb navigation** - Shows current path and allows quick navigation to parent folders
- **Prefix filtering** - Navigate into folders by clicking on them
- **Object listing** - Shows all objects in the current prefix

### 2. Encryption Status Visualization

See at a glance which objects are encrypted:

- **ARMOR badge** - Green badge on encrypted objects showing the key ID used
- **Key ID display** - Shows which MEK (default, sensitive, archive, etc.) was used
- **Plain text indicator** - Shows "plain" for unencrypted objects

Example display:
```
📄 data/sensor-readings.parquet  [ARMOR [sensitive]]  1.2 GB
📄 config.json                  [plain]               2.3 KB
📁 logs/                                              —
```

### 3. Cache Statistics

Monitor the effectiveness of the metadata cache:

- **Cache Hit Rate** - Percentage of requests served from cache vs. B2 API calls
- **Cache Hits / Misses** - Absolute counts for monitoring
- Higher hit rates = fewer B2 API calls = lower costs (after May 2026)

### 4. System Metrics

Real-time operational metrics:

- **Total Requests** - Cumulative request count
- **Bytes Uploaded** - Plaintext bytes received from clients
- **Bytes Downloaded** - Plaintext bytes served to clients
- **Uptime** - Server uptime duration
- **Canary Status** - Health status from the canary integrity monitor

### 5. Object Details

Click any object to view detailed metadata:

```json
{
  "key": "data/sensor-readings.parquet",
  "size": 128974848,
  "content_type": "application/octet-stream",
  "etag": "a1b2c3d4e5f6",
  "last_modified": "2026-05-07T12:00:00Z",
  "is_armor": true,
  "armor": {
    "plaintext_size": 128974848,
    "block_size": 65536,
    "key_id": "sensitive",
    "iv": "0123456789abcdef0123456789abcdef",
    "wrapped_dek": "base64encodedwrappedkey",
    "sha256": "hexdigestofplaintext"
  }
}
```

### 6. JSON Metrics Endpoint

For programmatic access and monitoring integrations:

```bash
curl http://localhost:9001/dashboard/metrics
```

Returns:
```json
{
  "requests_total": 15234,
  "requests_in_flight": 3,
  "bytes_uploaded": 5368709120,
  "bytes_downloaded": 21474836480,
  "bytes_fetched_from_b2": 12884901888,
  "range_reads_total": 456,
  "range_bytes_saved": 10737418240,
  "cache_hits": 12000,
  "cache_misses": 3234,
  "key_wrap_ops": 150,
  "key_unwrap_ops": 15000,
  "canary_checks": 240,
  "canary_failures": 0,
  "active_multipart": 0,
  "provenance_entries": 150,
  "uptime_seconds": 86400.0
}
```

## Dashboard Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/dashboard` | GET | Main dashboard UI (HTML) |
| `/dashboard?prefix=data/` | GET | Browse with prefix filter |
| `/dashboard/object?key=path/to/file` | GET | Object details (JSON) |
| `/dashboard/metrics` | GET | All metrics (JSON) |

## Performance Considerations

- **Listing performance**: The dashboard fetches up to 1000 objects per request. For buckets with many objects, use prefix navigation to narrow the scope.
- **Cache warm-up**: After ARMOR restarts, the cache hit rate will be low until metadata is loaded. The manifest index (if enabled) will warm the cache on startup.
- **No auth required**: The dashboard is currently accessible to anyone with network access to the admin port. Secure it appropriately in production.

## Troubleshooting

### Dashboard shows "Failed to list objects"

1. Check B2 connectivity: `kubectl exec deploy/armor -- curl -s localhost:9001/healthz`
2. Verify credentials: Check B2 credentials in the `armor-secrets` Secret
3. Check logs: `kubectl logs -f deploy/armor`

### Can't access dashboard externally

1. Verify Service exists: `kubectl get svc armor`
2. Check port-forward is running: `kubectl port-forward svc/armor 9001:9001`
3. For Ingress: Verify Ingress controller is running and DNS is configured

### Cache hit rate is 0%

This is normal for:
- Fresh ARMOR deployments (cache not yet warmed)
- Buckets with poor access locality (random object access)
- After cache clears (TTL expiration)

The cache will warm up as objects are accessed. For frequently-accessed objects (e.g., Parquet footers, hot data), you should see 60-90% hit rates during normal operation.

## Future Enhancements

Potential improvements for the dashboard:

- **Historical metrics** - Graph metrics over time using Prometheus/Grafana
- **Object search** - Full-text search across object keys
- **Bulk operations** - Select multiple objects for deletion or re-encryption
- **Key management UI** - Visual interface for key rotation and multi-key configuration (key rotation trigger already implemented)
- **Upload/download** - Direct file upload and download through the web UI
- **Real-time updates** - WebSocket updates for live metrics and new objects

## Contributing

The dashboard is implemented in Go with no external frontend dependencies. The HTML/CSS/JavaScript is embedded directly in the Go code for simplicity. To modify the UI:

1. Edit `internal/dashboard/dashboard.go`
2. Update the `dashboardHTML` template constant
3. Rebuild and redeploy: `docker build -t armor:local . && kubectl rollout restart deployment/armor`
