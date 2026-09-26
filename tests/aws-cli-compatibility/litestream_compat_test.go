// litestream compatibility leg — `litestream replicate` + `litestream
// restore` round-trip against the real ARMOR request pipeline.
//
// litestream is the client that motivated ADR-015 (it runs multipart
// snapshots at a fixed internal concurrency with no serial knob) and the
// client behind the two production incidents recorded under ADR-002/ADR-004,
// so the README's "litestream works unmodified" claim is gated here through
// the binary itself, configured exactly the way
// `armor client-config --for litestream` and
// docs/multipart-client-compatibility.md document it.
//
// The round-trip: a deterministic SQLite database is built, replicated to
// ARMOR with the documented replica block, restored into a fresh file, and
// verified by content digest (integrity_check + every row). Snapshot upload
// size is ~25 MiB so the replica leg exercises litestream's own transfer
// behavior rather than a lone small PUT.
package awsclicompat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// litestreamConf writes a litestream.yml replica config in the documented
// shape (endpoint, bucket, region, access keys — the block
// `armor client-config --for litestream` emits) plus the two settings a
// path-style, single-endpoint deployment needs: force-path-style, and a short
// sync-interval so the gate's snapshot lands quickly.
func litestreamConf(t *testing.T, endpoint, bucket, dbPath string) string {
	t.Helper()
	confPath := filepath.Join(t.TempDir(), "litestream.yml")

	accessKey, secretKey := compatCredentials(t)

	body := renderFixture(t, "litestream.yml", map[string]string{
		"DB_PATH":    dbPath,
		"ENDPOINT":   endpoint,
		"BUCKET":     bucket,
		"REGION":     testRegion,
		"ACCESS_KEY": accessKey,
		"SECRET_KEY": secretKey,
	})
	if err := os.WriteFile(confPath, body, 0o600); err != nil {
		t.Fatalf("write litestream.yml: %v", err)
	}
	return confPath
}

// buildSQLiteDatabase creates a deterministic SQLite database at path via
// python3's stdlib sqlite3 module: 20000 rows of ~1.2 KiB each (~25 MiB), then
// a TRUNCATE checkpoint so all content lives in the main db file before
// replication starts.
const sqliteBuilderScript = `
import sqlite3, sys
con = sqlite3.connect(sys.argv[1])
con.execute("CREATE TABLE kv (k INTEGER PRIMARY KEY, v TEXT NOT NULL)")
rows = [(i, ("arm0r-compat-" * 100) + str(i)) for i in range(20000)]
con.executemany("INSERT INTO kv VALUES (?,?)", rows)
con.commit()
con.execute("PRAGMA wal_checkpoint(TRUNCATE)")
con.close()
`

// sqliteDigestScript prints one digest line per database given on the command
// line: row count, integrity_check verdict, and a SHA-256 over every row. Two
// databases belong to the same logical content iff their lines match.
const sqliteDigestScript = `
import sqlite3, sys, hashlib
for path in sys.argv[1:]:
    con = sqlite3.connect("file:%s?mode=ro" % path, uri=True)
    ic = con.execute("PRAGMA integrity_check").fetchall()
    h = hashlib.sha256()
    for k, v in con.execute("SELECT k, v FROM kv ORDER BY k"):
        h.update(("%d:%s\n" % (k, v)).encode())
    n = con.execute("SELECT count(*) FROM kv").fetchone()[0]
    print(n, ic, h.hexdigest())
    con.close()
`

func TestLitestream_ReplicateRestoreRoundTrip(t *testing.T) {
	requireClientBin(t, "litestream",
		"install from https://litestream.io/install/ (the gate pins litestream 0.5.x)")
	requireClientBin(t, "python3",
		"python3 with the stdlib sqlite3 module is needed to build and digest the database")
	endpoint := startArmorServer(t)
	bucket := compatBucket(t)
	work := t.TempDir()

	// Deterministic source database (~25 MiB, checkpointed).
	dbPath := filepath.Join(work, "compat.sqlite")
	builder := writeFile(t, work, "build_db.py", []byte(sqliteBuilderScript))
	mustRun(t, "python3", nil, builder, dbPath)

	conf := litestreamConf(t, endpoint, bucket, dbPath)

	// Replicate in the background; litestream takes its first snapshot on
	// startup and re-syncs at sync-interval.
	d := startDaemon(t, "litestream", nil, "replicate", "-config", conf)

	// Restore is the readiness probe: it fails with "no snapshots" until the
	// first snapshot lands, then succeeds. Poll it instead of scraping the
	// daemon's log format, which is not an interface.
	restored := filepath.Join(work, "restored.sqlite")
	deadline := time.Now().Add(120 * time.Second)
	var lastErr string
	for {
		if out, err := run(t, "litestream", nil, "restore", "-config", conf, "-o", restored, dbPath); err != nil {
			lastErr = fmt.Sprintf("%v\n%s", err, out)
			if time.Now().After(deadline) {
				t.Fatalf("litestream restore never became ready within 120s; last restore attempt: %s\ndaemon log:\n%s",
					lastErr, d.daemonLog(t))
			}
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	d.stop(t)

	// The restored database must carry the source's exact logical content.
	digester := writeFile(t, work, "digest_db.py", []byte(sqliteDigestScript))
	want := mustRun(t, "python3", nil, digester, dbPath)
	got := mustRun(t, "python3", nil, digester, restored)
	if firstLine(got) != firstLine(want) {
		t.Fatalf("restored database content differs from source:\nsource:   %s\nrestored: %s", want, got)
	}
	t.Logf("litestream round-trip OK: %s", strings.TrimSpace(got))
}

// firstLine returns everything up to the first newline.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
