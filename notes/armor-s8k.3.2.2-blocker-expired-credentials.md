# armor-s8k.3.2.2 - Blocked: Expired Kubeconfig Credentials

## Date: 2026-05-02

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Blocker
**ord-devimprint kubeconfig credentials expired**

### Attempted Access Methods

1. **Direct kubeconfig (expired)**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint -l app=aggregator
   # Error: You must be logged in to the server (the server has asked for the client to provide credentials)
   ```

2. **YAML kubeconfig variant (expired)**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.yaml get pods -n devimprint -l app=aggregator
   # Error: You must be logged in to the server
   ```

3. **Tailscale kubectl-proxy (not available)**
   ```bash
   kubectl --server=http://traefik-ord-devimprint:8001 get pods -n devimprint
   # No such host: traefik-ord-devimprint
   ```

   Other clusters have working proxies:
   - traefik-ardenone-manager:8001 ✅
   - traefik-ardenone-hub:8001 ✅
   - traefik-rs-manager:8001 ✅
   - traefik-ord-devimprint:8001 ❌ (not configured)

### Target Pod
- **Cluster:** ord-devimprint
- **Namespace:** devimprint
- **Pod:** aggregator-*
- **Purpose:** Run DuckDB httpfs query over ARMOR-encrypted Parquet data

### Query to Run (from parent bead)
```python
import duckdb

con = duckdb.connect(':memory:')
con.execute('''
    INSTALL httpfs;
    LOAD httpfs;
    SET s3_region='us-east-1';
    SET s3_endpoint='http://armor:9000';
    SET s3_access_key_id='...');
    SET s3_secret_access_key='...');
''')

result = con.execute('''
    SELECT COUNT(*) FROM 's3://devimprint/commits/**/*.parquet'
''').fetchone()

print(f'COUNT(*): {result[0]}')
```

## Resolution Required
1. Refresh `/home/coding/.kube/ord-devimprint.kubeconfig` credentials, OR
2. Configure kubectl-proxy pod on ord-devimprint cluster (traefik-ord-devimprint:8001), OR
3. Provide alternative access method

## Related
- Parent bead: armor-s8k.3.2.1 (verified ARMOR v0.1.8 running on ord-devimprint)
- Verification note: armor-s8k.3-duckdb-httpfs-final-verification-2026-05-02.md
