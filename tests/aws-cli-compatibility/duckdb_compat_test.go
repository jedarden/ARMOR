// DuckDB compatibility leg — httpfs range reads over an encrypted Parquet
// object served by the real ARMOR request pipeline.
//
// README promises DuckDB can "query encrypted Parquet files with column
// pruning and predicate pushdown intact". Reading a Parquet object over S3 is
// inherently a range-read workload — DuckDB GETs the footer, then pulls only
// the row-group column chunks a query touches — so an aggregate over the
// remote object both exercises HTTP Range decryption and proves the
// plaintext survives encryption end to end. The same aggregate is computed
// over the local file first and the two must agree exactly.
package awsclicompat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// duckdbFlags are the CLI flags every duckdb invocation in this leg shares:
// batch mode, list output without headers (stable machine-readable rows), and
// -no-stdin so a TTY-attached run cannot hang.
func duckdbFlags() []string {
	return []string{"-batch", "-list", "-noheader", "-no-stdin"}
}

// httpfsSetupSQL builds the DuckDB httpfs configuration block for an ARMOR
// endpoint. DuckDB takes s3_endpoint as host[:port] without a scheme and
// needs s3_url_style=path because ARMOR does not serve virtual-hosted style
// (docs/connection-guide.md).
func httpfsSetupSQL(t *testing.T, endpoint string) string {
	t.Helper()
	useSSL, hostPort := false, endpoint
	switch {
	case strings.HasPrefix(endpoint, "https://"):
		useSSL, hostPort = true, strings.TrimPrefix(endpoint, "https://")
	case strings.HasPrefix(endpoint, "http://"):
		hostPort = strings.TrimPrefix(endpoint, "http://")
	default:
		t.Fatalf("unexpected endpoint scheme: %s", endpoint)
	}

	return string(renderFixture(t, "duckdb-httpfs.sql", map[string]string{
		"REGION":    testRegion,
		"HOST_PORT": hostPort,
		"USE_SSL":   fmt.Sprintf("%t", useSSL),
	}))
}

func duckDBEnv(t *testing.T) []string {
	t.Helper()
	accessKey, secretKey := compatCredentials(t)
	return mergeEnv(os.Environ(), map[string]string{
		"AWS_ACCESS_KEY_ID":         accessKey,
		"AWS_SECRET_ACCESS_KEY":     secretKey,
		"AWS_DEFAULT_REGION":        testRegion,
		"AWS_EC2_METADATA_DISABLED": "true",
	})
}

func TestDuckDB_HTTPFSParquetRangeRead(t *testing.T) {
	requireClientBin(t, "duckdb",
		"install from https://duckdb.org (the gate pins duckdb 1.5.5; httpfs is fetched from extensions.duckdb.org on first INSTALL)")
	endpoint := startArmorServer(t)
	bucket := compatBucket(t)
	work := t.TempDir()

	// Build a Parquet file with DuckDB itself: 50000 rows so the file spans
	// multiple row groups and every read is chunked, then upload it via the
	// SDK. (Encryption happens server-side on PUT; DuckDB only ever speaks
	// plaintext S3, which is the point.)
	parquet := filepath.Join(work, "armor-compat.parquet")
	mustRun(t, "duckdb", nil, append(duckdbFlags(),
		"-c", fmt.Sprintf("COPY (SELECT i::BIGINT AS id, md5(i::VARCHAR) AS hash, i*7 AS num FROM range(50000) t(i)) TO '%s' (FORMAT PARQUET)",
			parquet))...)

	key := "duckdb/armor-compat.parquet"
	f, err := os.Open(parquet)
	if err != nil {
		t.Fatalf("open %s: %v", parquet, err)
	}
	client := newSDKClient(t, endpoint)
	if _, err := client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: bucketPtr(bucket),
		Key:    bucketPtr(key),
		Body:   f,
	}); err != nil {
		f.Close()
		t.Fatalf("PutObject %s: %v", key, err)
	}
	f.Close()

	// The same two queries against the local file and against the encrypted
	// object over httpfs must agree byte-for-byte on every output value:
	//   - a full-file aggregate (every row group read),
	//   - a narrow id window (footer + selected column chunks only — the
	//     column-pruning range-read shape README promises).
	remote := "'s3://" + bucket + "/" + key + "'"
	local := "'" + parquet + "'"
	queries := map[string]string{
		"full aggregate":    "SELECT count(*), sum(num), md5(string_agg(hash, '' ORDER BY id)) FROM %s",
		"id window 100-103": "SELECT id, hash, num FROM %s WHERE id BETWEEN 100 AND 103 ORDER BY id",
	}
	for name, q := range queries {
		wantOut, err := run(t, "duckdb", nil, append(duckdbFlags(), "-c", fmt.Sprintf(q, local))...)
		if err != nil {
			t.Fatalf("local %s query failed: %v\n%s", name, err, wantOut)
		}
		gotOut, err := run(t, "duckdb", duckDBEnv(t), append(duckdbFlags(),
			"-c", httpfsSetupSQL(t, endpoint)+fmt.Sprintf(q, remote))...)
		if err != nil {
			t.Fatalf("httpfs %s query against ARMOR failed: %v\n%s", name, err, gotOut)
		}
		if got := strings.TrimSpace(gotOut); got == "" {
			t.Fatalf("httpfs %s query returned no rows", name)
		} else if got != strings.TrimSpace(wantOut) {
			t.Fatalf("httpfs %s over ARMOR disagrees with the local file:\nlocal:  %s\nremote: %s",
				name, wantOut, gotOut)
		} else {
			t.Logf("duckdb httpfs %s OK: %s", name, got)
		}
	}
}
