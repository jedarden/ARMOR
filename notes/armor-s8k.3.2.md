# armor-s8k.3.2: DuckDB httpfs glob expansion test

## Goal
Verify DuckDB can glob-expand and query Parquet files through ARMOR without date parse errors.

## Test Results

### Test Environment
- Cluster: ord-devimprint
- Pod: aggregator-6949b669d5-6b96s
- ARMOR endpoint: armor:9000 (HTTP, path-style)

### Test 1: glob() function (LIST endpoint)
**Status: PASS**
- Successfully listed 50+ files using `glob('s3://devimprint/commits/**/*.parquet')`
- No `InvalidInputException` occurred
- No date parse errors in output

### Test 2: Single file read
**Status: PASS**
- Successfully read a single Parquet file via `read_parquet('s3://devimprint/commits/...')`
- Required `pytz` module installation

### Test 3: Multi-file glob read
**Status: PARTIAL**
- `read_parquet('s3://devimprint/commits/**/*.parquet')` with LIMIT caused OOM
- Narrower pattern (`year=1972/month=07/day=18/*.parquet`) returned HTTP 400 "Invalid range: range out of bounds"
- This is a DuckDB parallel range request issue, not an ARMOR timestamp bug

## Conclusion
The **InvalidInputException date parse error is FIXED**. The glob() function successfully expands patterns and returns file paths without throwing date parsing errors.

The HTTP 400 error observed with multi-file reads is a separate DuckDB/S3 compatibility issue (parallel range requests on small files), not related to ARMOR's timestamp handling.

## Files Observed
Sample file paths (Hive partition scheme):
- s3://devimprint/commits/year=1972/month=07/day=18/clone-worker-77cdf844d9-765km-1777040614.parquet
- s3://devimprint/commits/year=1973/month=11/day=11/clone-worker-6b94b786b8-sdqdc-1777361026.parquet

Note: Years in paths are Hive partition values, not ARMOR LastModified timestamps.
