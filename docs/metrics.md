# ARMOR Metrics Documentation

## Overview

ARMOR exports Prometheus metrics at the `/metrics` endpoint (admin port, default `127.0.0.1:9001/metrics`). All metrics are prefixed with `armor_` and follow Prometheus best practices for naming and labeling.

## Metrics Endpoint

### Accessing Metrics

```bash
curl http://localhost:9001/metrics
```

**Response Format:** Plain text (Prometheus exposition format)

**Content-Type:** `text/plain; version=0.0.4`

### Metric Types

- **Counter:** Monotonically increasing counter (always goes up)
- **Gauge:** Point-in-time value (can go up or down)
- **Histogram:** Distribution of values (count, sum, buckets)

## Replication Metrics

### Overview

ARMOR tracks async replication queue operations for secondary backend replication. These metrics help monitor replication health, queue depth, and enqueue operations.

### `armor_replication_enqueued_total`

**Type:** Counter  
**Description:** Total number of items enqueued for replication by operation type  
**Labels:** `operation` - Type of operation that triggered replication enqueue

**Label Values:**
- `put` — Standard object uploads (PutObject)
- `put-streaming` — Streaming uploads (PutObject with streaming)

**Thread-Safety:** This metric is safe for concurrent access from multiple goroutines. The underlying `expvar.Map.Add()` operation is internally synchronized with a mutex.

**Example Output:**
```
# HELP armor_replication_enqueued_total Total number of items enqueued for replication by operation
# TYPE armor_replication_enqueued_total counter
armor_replication_enqueued_total{operation="put"} 1234
armor_replication_enqueued_total{operation="put-streaming"} 567
```

**Example Usage (Go):**
```go
// In upload handler goroutine 1:
go func() {
    metrics.IncReplicationEnqueued("put")
}()

// In upload handler goroutine 2 (simultaneous):
go func() {
    metrics.IncReplicationEnqueued("put-streaming")
}()
```

**Testing:**
```bash
# Query the metric after upload operations
curl -s http://localhost:9001/metrics | grep replication_enqueued_total

# Expected output:
# HELP armor_replication_enqueued_total Total number of items enqueued for replication by operation
# TYPE armor_replication_enqueued_total counter
armor_replication_enqueued_total{operation="put"} 1234
armor_replication_enqueued_total{operation="put-streaming"} 567
```

### `armor_replication_queue_depth`

**Type:** Gauge  
**Description:** Current number of items in the replication queue  
**Labels:** None

**Example Output:**
```
# HELP armor_replication_queue_depth Current number of items in the replication queue
# TYPE armor_replication_queue_depth gauge
armor_replication_queue_depth 42
```

**Alerting Example (Prometheus):**
```yaml
- alert: ReplicationQueueBacklog
  expr: armor_replication_queue_depth > 1000
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Replication queue depth is high"
    description: "Replication queue has {{ $value }} items pending"
```

### `armor_replication_dropped_total`

**Type:** Counter  
**Description:** Total number of items dropped due to full replication queue  
**Labels:** None

**Example Output:**
```
# HELP armor_replication_dropped_total Total number of items dropped due to full replication queue
# TYPE armor_replication_dropped_total counter
armor_replication_dropped_total 0
```

**Alerting Example (Prometheus):**
```yaml
- alert: ReplicationDropsDetected
  expr: increase(armor_replication_dropped_total[5m]) > 0
  labels:
    severity: critical
  annotations:
    summary: "Replication items are being dropped"
    description: "{{ $value }} items dropped in the last 5 minutes due to full queue"
```

## Request Metrics

### `armor_requests_total`

**Type:** Counter  
**Description:** Total number of requests processed  
**Labels:** None

### `armor_requests_in_flight`

**Type:** Gauge  
**Description:** Number of requests currently being processed  
**Labels:** None

### `armor_bytes_uploaded_total`

**Type:** Counter  
**Description:** Total plaintext bytes uploaded by clients  
**Labels:** None

### `armor_bytes_downloaded_total`

**Type:** Counter  
**Description:** Total plaintext bytes downloaded by clients  
**Labels:** None

## Cache Metrics

### `armor_metadata_cache_hits_total`

**Type:** Counter  
**Description:** Total number of metadata cache hits  
**Labels:** None

### `armor_metadata_cache_misses_total`

**Type:** Counter  
**Description:** Total number of metadata cache misses  
**Labels:** None

## Encryption Metrics

### `armor_key_wrap_ops_total`

**Type:** Counter  
**Description:** Total number of key wrap operations  
**Labels:** None

### `armor_key_unwrap_ops_total`

**Type:** Counter  
**Description:** Total number of key unwrap operations  
**Labels:** None

## Canary Metrics

### `armor_canary_checks_total`

**Type:** Counter  
**Description:** Total number of canary checks performed  
**Labels:** None

### `armor_canary_check_failures_total`

**Type:** Counter  
**Description:** Total number of canary check failures  
**Labels:** None

### `armor_canary_healthy`

**Type:** Gauge  
**Description:** Canary health status (1 = healthy, 0 = unhealthy)  
**Labels:** None

## Testing Metrics

### Verifying Metrics Endpoint

```bash
# Check metrics endpoint is accessible
curl -f http://localhost:9001/metrics

# Check specific replication metrics
curl -s http://localhost:9001/metrics | grep -A 3 "replication"

# Check all metrics start with armor_ prefix
curl -s http://localhost:9001/metrics | grep "^armor_"

# Count number of metrics exported
curl -s http://localhost:9001/metrics | grep "^armor_" | wc -l
```

### Verifying Thread Safety

The replication metrics have been verified for thread-safety:

1. **Code Review:** `expvar.Map.Add()` is internally synchronized with a mutex
2. **Race Detector:** Tests pass with `go test -race` flag
3. **Stress Tests:** Concurrent access tests (100 goroutines × 100 increments) pass consistently
4. **Implementation:** See `internal/metrics/replication_metric_test.go` for test details

## Prometheus Alerting Examples

### Replication Health Dashboard

```yaml
groups:
  - name: armor_replication
    interval: 30s
    rules:
      # Replication queue backlog alert
      - alert: ReplicationQueueBacklog
        expr: armor_replication_queue_depth > 1000
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Replication queue backlog detected"
          description: "Queue depth: {{ $value }} items"

      # Replication drops alert (CRITICAL)
      - alert: ReplicationDropsDetected
        expr: increase(armor_replication_dropped_total[5m]) > 0
        labels:
          severity: critical
        annotations:
          summary: "Replication items are being dropped"
          description: "{{ $value }} items dropped in 5m due to full queue"

      # Replication enqueue rate monitoring
      - record: armor_replication_enqueue_rate
        expr: rate(armor_replication_enqueued_total[5m])
```

## Grafana Dashboard Queries

### Replication Queue Depth

```promql
armor_replication_queue_depth
```

### Replication Enqueue Rate (by operation)

```promql
sum by (operation) (rate(armor_replication_enqueued_total[5m]))
```

### Replication Drop Rate

```promql
rate(armor_replication_dropped_total[5m])
```

### Queue Utilization (assuming max queue size of 10000)

```promql
(armor_replication_queue_depth / 10000) * 100
```

## Troubleshooting

### High Queue Depth

**Symptoms:**
- `armor_replication_queue_depth` consistently high (> 1000)
- `armor_replication_dropped_total` increasing

**Possible Causes:**
1. Secondary backend is slow or unavailable
2. Replication worker is not processing items fast enough
3. Network latency to secondary backend

**Investigation:**
```bash
# Check queue depth trend
curl -s http://localhost:9001/metrics | grep replication_queue_depth

# Check if drops are occurring
curl -s http://localhost:9001/metrics | grep replication_dropped_total

# Check enqueue rate by operation
curl -s http://localhost:9001/metrics | grep replication_enqueued_total
```

### Metrics Not Updating

**Symptoms:**
- Metric values stay at 0 or don't change
- `/metrics` endpoint returns stale data

**Investigation:**
1. Verify metrics endpoint is accessible:
   ```bash
   curl -v http://localhost:9001/metrics
   ```

2. Check ARMOR server is running and processing requests

3. Verify metrics are being called in the code:
   ```bash
   grep -r "IncReplicationEnqueued" internal/
   ```

### Race Conditions

**Symptoms:**
- Panic messages about concurrent map writes
- Metric values appear corrupted

**Solution:**
The replication metrics use `expvar.Map.Add()` which is thread-safe. If you see race conditions:
1. Ensure you're using the provided `IncReplicationEnqueued()` method
2. Run with race detector: `go test -race ./internal/metrics/`
3. Check test logs for any warnings

## See Also

- [Internal Metrics Implementation](../internal/metrics/metrics.go)
- [Replication Metric Tests](../internal/metrics/replication_metric_test.go)
- [Admin Endpoints Documentation](admin-endpoint-error-response-headers.md)
- [Dashboard Documentation](dashboard.md)
