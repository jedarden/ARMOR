# armor-s8k.3: DuckDB httpfs verification with fixed ARMOR v0.1.11

## Date: 2026-05-01

## Summary
Verified end-to-end that DuckDB can query Parquet files through ARMOR v0.1.11 via httpfs without date parse errors.

## Environment
- Cluster: ord-devimprint
- ARMOR version: v0.1.11 (ronaldraygun/armor:0.1.11)
- ARMOR endpoint: armor:9000 (HTTP, path-style)
- DuckDB version: 1.5.2

## Test Results

### Test 1: glob() expansion (LIST operation)
**Status: PASS**
```python
glob('s3://devimprint/commits/**/*.parquet')
```
- Successfully listed files without InvalidInputException
- No date parse errors in output
- DuckDB parsed ISO 8601 LastModified timestamps correctly

### Test 2: Single file read (GET operation)
**Status: PASS**
```python
read_parquet('s3://devimprint/commits/year=1972/month=07/day=18/clone-worker-77cdf844d9-765km-1777040614.parquet')
```
- Successfully read 1 row from file
- No InvalidInputException or date parse errors

### ARMOR Logs
**Status: CLEAN**
- No errors, warnings, or invalid entries during test period
- All requests completed with HTTP 200 status

## Verification of Fix

The ISO 8601 fix (commit 961c610) is confirmed working:
- All LastModified HTTP headers use format: `"2006-01-02T15:04:05.000Z"`
- All XML LastModified fields use format: `"2006-01-02T15:04:05.000Z"`
- DuckDB httpfs correctly parses both formats

## Acceptance Criteria
- ✅ DuckDB httpfs glob expansion works without errors
- ✅ No InvalidInputException occurred
- ✅ No date parse errors in output
- ✅ ARMOR logs show no errors
- ✅ File reading works correctly

## Notes
- The boto3-based approach was already working (backward compatible)
- DuckDB httpfs now provides the same functionality with better performance
- No changes needed to existing client code
