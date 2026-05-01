# armor-s8k.3.2: DuckDB httpfs glob expansion verification

## Test Date
2026-05-01

## Cluster
ord-devimprint (ARMOR v0.1.10)

## Acceptance Criteria Status

### ✓ glob() function without InvalidInputException
**PASS** - Confirmed working
```python
con.execute("SELECT * FROM glob('s3://devimprint/commits/**/*.parquet') LIMIT 10").fetchall()
# Returns 10 files successfully, no errors
```

### ✓ No date parse errors in LIST output
**PASS** - No InvalidInputException occurred during glob expansion

### COUNT(*) query with glob expansion
**PARTIAL** - DuckDB URL-encoding issue prevents read_parquet() from working
- Error: `HTTP GET error reading 'http://armor:9000/devimprint/commits/year%3D1996/...'`
- DuckDB URL-encodes the `=` characters in Hive partition paths
- ARMOR doesn't decode URL-encoded paths in GET requests (known limitation)

## Conclusion

The **InvalidInputException date parse error is FIXED**. The glob() function successfully expands patterns and returns file paths without throwing date parsing errors.

The remaining issue with read_parquet() is a DuckDB httpfs behavior where it URL-encodes special characters in paths. ARMOR would need to add URL path decoding to HandleRoot to fully support this use case.

## Sample glob() output
```
s3://devimprint/commits/year=1972/month=07/day=18/clone-worker-77cdf844d9-765km-1777040614.parquet
s3://devimprint/commits/year=1973/month=11/day=11/clone-worker-6b94b786b8-sdqdc-1777361026.parquet
s3://devimprint/commits/year=1974/month=01/day=20/clone-worker-77cdf844d9-765km-1777040614.parquet
...
```
