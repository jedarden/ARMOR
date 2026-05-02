# armor-s8k.3.2.2 - DuckDB httpfs Query Test

## Task
Exec into aggregator pod on ord-devimprint and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Blocker - Expired OIDC Credentials

### Issue
The ord-devimprint cluster kubeconfig uses OIDC authentication with credentials that expired on 2026-04-27.

### Attempted Remedies
1. Static token (ngpc-user): Also expired
2. OIDC refresh: Requires browser-based authentication flow - not available in headless environment
3. Alternative kubeconfigs: Same OIDC auth, also expired

### Required Command
kubectl exec into aggregator pod on ord-devimprint with DuckDB query using ARMOR as S3 endpoint.

### Acceptance Criteria
- COUNT(*) returns a non-zero integer with no errors
- No InvalidInputException in output
- Timestamps are valid

## Next Steps
User needs to refresh OIDC credentials on a machine with browser access.

## Cluster Details
- Cluster: ord-devimprint (Rackspace Spot, ord region)
- Auth Method: OIDC via login.spot.rackspace.com
- Target Pod: aggregator in devimprint namespace
- S3 Proxy: ARMOR service at armor:9000 (cluster-local)
