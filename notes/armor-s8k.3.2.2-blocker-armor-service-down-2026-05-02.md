# armor-s8k.3.2.2 - ARMOR Service Down - 2026-05-02 16:05 UTC

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## CRITICAL BLOCKER: ARMOR Service Down

### Current Status (2026-05-02 16:05 UTC)

**ARMOR Pods - CrashLoopBackOff:**
```
armor-755d878c84-l8grt   0/1   Running   32 (92s ago)    144m
armor-7c79d57db6-k2j6j   0/1   Running   30 (104s ago)   134m
```

**ARMOR Service - No Endpoints:**
```
NAME        ENDPOINTS   AGE
armor-svc               19d
```

### Pod Behavior
- Pods start, log startup message, then immediately crash
- Liveness/readiness probes fail (http-get :9000/healthz and /readyz)
- Logs show only startup messages before crash:
```
{"time":"2026-05-02T16:01:51.672428422Z","level":"INFO","service":"armor","msg":"ARMOR starting",...}
{"time":"2026-05-02T16:01:51.672790311Z","level":"INFO","service":"armor","msg":"B2 key management disabled"...}
```

### Impact on Aggregator
Aggregator logs show connection failures:
```
botocore.exceptions.EndpointConnectionError: Could not connect to the endpoint URL: "http://armor-svc:9000/devimprint/state/backfill_cursor.txt"
```

## Access Constraints

| Method | Status | Issue |
|--------|--------|-------|
| ord-devimprint.kubeconfig | ❌ Expired | OIDC token expired, requires browser re-auth |
| ardenone-hub proxy (traefik-ardenone-hub:8001) | ❌ Read-only | RBAC blocks exec: "unable to upgrade connection: Forbidden" |
| ardenone-hub kubeconfig | ❌ Missing | No write-access kubeconfig exists |

## Required Query (Cannot Run Without ARMOR Service)
```python
import duckdb, os
con = duckdb.connect()
con.execute("INSTALL httpfs; LOAD httpfs;")
con.execute("SET s3_endpoint='armor:9000';")
con.execute("SET s3_use_ssl=false;")
con.execute(f"SET s3_access_key_id='{os.environ['S3_ACCESS_KEY_ID']}';")
con.execute(f"SET s3_secret_access_key='{os.environ['S3_SECRET_ACCESS_KEY']}';")
con.execute("SET s3_url_style='path';")
result = con.execute("SELECT COUNT(*) FROM read_parquet('s3://devimprint/commits/**/*.parquet')").fetchone()
print('Row count:', result[0])
```

## Resolution Required
1. **FIX ARMOR PODS** (CRITICAL - blocking all S3 access)
   - Get write access to ardenone-hub cluster
   - Check armor pod logs for crash reason (likely secrets or config issue)
   - Verify secrets: devimprint-b2, devimprint-armor-mek, devimprint-armor-writer, devimprint-armor-readonly
2. Refresh ord-devimprint.kubeconfig via Rackspace Spot dashboard (browser)
3. Create write-access kubeconfig for ardenone-hub cluster

## Status
**BLOCKED** - ARMOR service is completely down; cannot run DuckDB query without S3 endpoint
