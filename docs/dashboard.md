# ARMOR Web Dashboard

The ARMOR web dashboard provides a visual interface for browsing encrypted buckets, viewing encryption status, and monitoring cache statistics.

## Accessing the Dashboard

### Local Development

When running `armor demo`, the dashboard is available on the admin port (default
`127.0.0.1:9001`). The command prints a generated dashboard username and
password, an admin token, and the dashboard URL. Open that URL in a browser and
use the printed dashboard credentials when prompted:

```bash
go run ./cmd/armor demo -dir ./armor-demo-data
```

The demo stores its MEK in `./armor-demo-data/.armor-demo-mek` with mode 0600
and reuses it on later runs, so objects written before a restart remain
readable. A different relative `-mek-file` path is resolved under `-dir`; an
absolute `-mek-file` path is used as given. Without `-dir`, the demo uses a
temporary directory and an in-memory MEK, and removes the directory on exit.

To set the dashboard credentials yourself, export
`ARMOR_DASHBOARD_USER` and `ARMOR_DASHBOARD_PASS` before starting the demo.
The command uses those values and prints them with the dashboard URL. The
separate `ARMOR_ADMIN_TOKEN` is also accepted and printed; when unset, the demo
generates one for the session.

### Kubernetes Deployment

The dashboard is exposed via the admin-api service on port 9001:

```bash
# Port-forward to access locally
kubectl -n armor port-forward svc/armor 9001:9001

# Then open in browser
open http://localhost:9001/dashboard
```

### Remote Access

Use the port-forward above for operator access. If a cluster needs a persistent
route, define a Traefik `IngressRoute` for the admin service in
`declarative-config`, following that cluster's access and TLS rules. The iad-ci
ARMOR manifests currently expose the S3 port through a VPN route; they do not
publish the dashboard through an ingress route.

## Dashboard Authentication

The dashboard supports two authentication methods to protect against unauthorized access.

### HTTP Basic Authentication

Set `ARMOR_DASHBOARD_USER` and `ARMOR_DASHBOARD_PASS` in the server environment.
For a cluster deployment, source them from a Secret/ExternalSecret reference.

When opening `/dashboard` in a browser, enter the configured username and
password at the Basic Auth prompt. Supply production values through a
Secret/ExternalSecret reference, never as literal values in a manifest.

### Bearer Token Authentication

Alternatively, set `ARMOR_DASHBOARD_TOKEN` in the server environment through a
Secret/ExternalSecret reference.

API clients can send the token in an `Authorization: Bearer` header. Source the
token from the deployment's Secret/ExternalSecret reference; keep it out of
command arguments and logs.

### Security Notes

- If neither authentication method is configured, the dashboard is not anonymous: its routes fall back to the admin bearer-token gate. When `ARMOR_ADMIN_TOKEN` is unset too, the mount fails closed with `403 admin API disabled`. The read-only `/dashboard/admin/key/status` route remains available to authenticated callers.
- That bearer fallback is awkward for interactive use, since a browser cannot attach the header. Set `ARMOR_DASHBOARD_USER`/`ARMOR_DASHBOARD_PASS` (or `ARMOR_DASHBOARD_TOKEN`) wherever a person is meant to open the dashboard.
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
- **Replication Queue Depth** - Current number of items awaiting replication to secondary backend
- **Replication Enqueued** - Total items queued for replication (by operation type)
- **Replication Dropped** - Total items dropped due to full queue

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
    "wrapped_dek": "0123456789abcdef",
    "sha256": "hexdigestofplaintext"
  }
}
```

### 6. JSON Metrics Endpoint

For programmatic access and monitoring integrations, request
`GET /dashboard/metrics` with the dashboard authentication configured above.

Example response fields:
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
  "cache_hit_rate_pct": 78.77,
  "key_wrap_ops": 150,
  "key_unwrap_ops": 15000,
  "canary_checks": 240,
  "canary_failures": 0,
  "active_multipart": 0,
  "provenance_entries": 150,
  "uptime_seconds": 86400.0,
  "replication_queue_depth": 42,
  "replication_dropped": 0,
  "replication_errors": 0,
  "replication_retries": 0
}
```

## Dashboard Endpoints

See the [dashboard routes in the HTTP API reference](http-api-reference.md#dashboard-routes)
for the current method, authentication, success, and error contract of every
route, including upload, download, delete, share links, and credential activity.

### JSON List Endpoint

The `/dashboard/api/list` endpoint returns a JSON response with objects and common prefixes (virtual folders), enabling programmatic bucket browsing.

**Request:** authenticated `GET /dashboard/api/list?prefix=data/`.

**Response:**
```json
{
  "prefix": "data/",
  "objects": [
    {
      "key": "data/file1.txt",
      "size": 1024,
      "last_modified": "2026-06-11T12:00:00Z",
      "encrypted": true,
      "key_id": "default"
    },
    {
      "key": "data/file2.txt",
      "size": 2048,
      "last_modified": "2026-06-11T11:30:00Z",
      "encrypted": false
    }
  ],
  "common_prefixes": ["data/logs/", "data/reports/"]
}
```

**Fields:**
- `prefix`: The requested prefix filter
- `objects`: Array of objects with encryption status
  - `key`: Object key (full path)
  - `size`: Object size in bytes (plaintext size for encrypted objects)
  - `last_modified`: ISO 8601 timestamp
  - `encrypted`: Boolean indicating ARMOR encryption
  - `key_id`: The key ID used for encryption (omitted for plain objects)
- `common_prefixes`: Array of virtual folder prefixes (delimiter-based)

**Authentication:** This endpoint requires the same authentication as other dashboard routes (Basic Auth or Bearer token when configured).

## Performance Considerations

- **Listing performance**: The dashboard fetches up to 1000 objects per request. For buckets with many objects, use prefix navigation to narrow the scope.
- **Cache warm-up**: After ARMOR restarts, the cache hit rate will be low until metadata is loaded. The manifest index (if enabled) will warm the cache on startup.
- **Auth fallback**: with no dashboard credential configured, dashboard routes require the `ARMOR_ADMIN_TOKEN` bearer token and fail closed with 403 when that is unset too. See Security Notes above.

## Troubleshooting

### Dashboard shows "Failed to list objects"

1. Check B2 connectivity: `kubectl -n armor exec deploy/armor -- armor check` (the
   image is `scratch` and has no `curl`), or port-forward and
   `curl -s localhost:9001/healthz` from your machine
2. Verify the configured B2 credential Secret/ExternalSecret has synced
3. Check logs: `kubectl -n armor logs -f deploy/armor`

### Can't access the dashboard

1. Verify the Service exists: `kubectl -n armor get svc armor`
2. Check the port-forward is running: `kubectl -n armor port-forward svc/armor 9001:9001`
3. For a persistent route, inspect its Traefik `IngressRoute`, TLS certificate,
   and DNS in `declarative-config`

### Cache hit rate is 0%

This is normal for:
- Fresh ARMOR deployments (cache not yet warmed)
- Buckets with poor access locality (random object access)
- After cache clears (TTL expiration)

The cache warms as objects are accessed. Check `/dashboard/metrics` for the
measured hit rate; it depends on the workload and cache configuration.

## Future Enhancements

Potential improvements for the dashboard:

- **Historical metrics** - Graph metrics over time using Prometheus/Grafana
- **Object search** - Full-text search across object keys
- **Bulk operations** - Select multiple objects for deletion or re-encryption
- **Key management UI** - Add a read-only view of key-ring status. Rotate keys through the empty-body admin API request in the [key-ring runbook](key-rotation-runbook.md).
- **Real-time updates** - WebSocket updates for live metrics and new objects

## Contributing

The dashboard HTML/CSS/JavaScript is embedded in Go. Its Agentation toolbar
loads through the page's import map. To modify the UI:

1. Edit `internal/dashboard/dashboard.go`
2. Update the `dashboardHTML` template constant
3. Ship it like any other change: commit, then cut a release
   (`scripts/cut-release.sh <version>`) so CI builds and publishes the image,
   and roll the fleet forward by bumping the image tag in
   `declarative-config` for ArgoCD to apply. Never mutate a live deployment
   directly — see
   [docs/release-process.md](release-process.md) and `AGENTS.md`.
