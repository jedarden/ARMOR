-- Fixture rendered by duckdb_compat_test.go from the documented httpfs shape.
INSTALL httpfs;
LOAD httpfs;
SET s3_region='{{REGION}}';
SET s3_endpoint='{{HOST_PORT}}';
SET s3_url_style='path';
SET s3_use_ssl={{USE_SSL}};
