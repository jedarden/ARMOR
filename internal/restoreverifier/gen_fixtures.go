//go:build ignore

// Command gen_fixtures regenerates the small real-artifact fixtures under
// testdata/ used by the assertion tests. It is excluded from normal package
// builds via the "ignore" build tag; run it explicitly:
//
//	go run internal/restoreverifier/gen_fixtures.go
//
// The program writes to internal/restoreverifier/testdata and must be run from
// the repository root (the conventional CWD for `go run`).
//
// For each artifact class it writes a valid fixture and a corrupted twin whose
// corruption the corresponding assertion must detect (not swallow). The
// generator prints the result of exercising each corrupt fixture with the same
// detection logic the assertion uses, and exits non-zero if any corruption goes
// undetected — so a regenerated fixture can never silently pass.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
	_ "modernc.org/sqlite"
)

// outDir is relative to the repository root (the CWD when invoked via
// `go run internal/restoreverifier/gen_fixtures.go`).
const outDir = "internal/restoreverifier/testdata"

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", outDir, err)
	}

	mustWrite(filepath.Join(outDir, "valid.sqlite"), genValidSQLite())
	mustWrite(filepath.Join(outDir, "corrupt.sqlite"), genCorruptSQLite())

	validParquet := genValidParquet()
	mustWrite(filepath.Join(outDir, "valid.parquet"), validParquet)
	mustWrite(filepath.Join(outDir, "corrupt.parquet"), genCorruptParquet(validParquet))

	validTarGz := genValidTarGz()
	mustWrite(filepath.Join(outDir, "valid.tar.gz"), validTarGz)
	mustWrite(filepath.Join(outDir, "corrupt.tar.gz"), genCorruptTarGz(validTarGz))

	fmt.Println("fixtures regenerated under", outDir)
}

func mustWrite(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
	fmt.Printf("  wrote %s (%d bytes)\n", path, len(data))
}

// ---------------------------------------------------------------------------
// SQLite
// ---------------------------------------------------------------------------

const sqliteMagic = "SQLite format 3\x00"

// genValidSQLite builds a tiny but well-formed SQLite database with one table
// ("events") holding a handful of rows, suitable for the integrity_check and
// row-count probes.
func genValidSQLite() []byte {
	dir, err := os.MkdirTemp("", "gen-sqlite-")
	if err != nil {
		log.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "valid.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(
		"CREATE TABLE events (id INTEGER PRIMARY KEY, kind TEXT NOT NULL, ts INTEGER NOT NULL)",
	); err != nil {
		log.Fatalf("create table: %v", err)
	}
	for i := 1; i <= 12; i++ {
		if _, err := db.Exec(
			"INSERT INTO events (id, kind, ts) VALUES (?, ?, ?)",
			i, fmt.Sprintf("event-%d", i), 1_700_000_000+i,
		); err != nil {
			log.Fatalf("insert row %d: %v", i, err)
		}
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		log.Fatalf("read db: %v", err)
	}

	// Sanity: the freshly built DB must self-report healthy.
	if got := sqliteIntegrity(data); got != "ok" {
		log.Fatalf("valid.sqlite integrity_check = %q, want %q", got, "ok")
	}
	return data
}

// genCorruptSQLite flips a single 16-byte region chosen so the SQLite engine
// reports damage via PRAGMA integrity_check. It prefers the start of page 2
// (the b-tree page header of the table-data page), then falls back to scanning
// for any single region that trips detection. The 16-byte magic header is left
// intact so the corruption is not caught by the cheap structural pre-check and
// reaches the engine, mirroring real mid-file corruption.
func genCorruptSQLite() []byte {
	orig := genValidSQLite()

	// Candidate offsets, in priority order: page-2 header first (most reliable),
	// then a coarse scan across the file.
	pageSize := 4096
	if len(orig) >= pageSize+16 && orig[pageSize] == 0x0d /* leaf table b-tree */ {
		// Likely the table-data page; corrupting its header is deterministic.
	}
	var candidates []int
	candidates = append(candidates, pageSize) // page-2 header
	for off := 200; off < len(orig)-16; off += 37 {
		candidates = append(candidates, off)
	}

	for _, off := range candidates {
		data := append([]byte(nil), orig...)
		for i := 0; i < 16 && off+i < len(data); i++ {
			data[off+i] ^= 0xFF
		}
		// The magic header must survive so the pre-check passes and the engine
		// (not the structural guard) is what detects the damage.
		if string(data[:len(sqliteMagic)]) != sqliteMagic {
			continue
		}
		if got := sqliteIntegrity(data); got != "ok" {
			fmt.Printf("  sqlite corruption detected at byte %d: %q\n", off, got)
			return data
		}
	}
	log.Fatalf("could not produce a single-region corruption that integrity_check detects")
	return nil
}

// sqliteIntegrity writes data to a temp DB and returns the first
// integrity_check row (a healthy DB returns "ok").
func sqliteIntegrity(data []byte) string {
	dir, err := os.MkdirTemp("", "gen-sqlite-check-")
	if err != nil {
		log.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "check.db")
	if err := os.WriteFile(dbPath, data, 0o600); err != nil {
		log.Fatalf("write check db: %v", err)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&immutable=1")
	if err != nil {
		return fmt.Sprintf("open-error: %v", err)
	}
	defer db.Close()

	var first string
	rows, err := db.Query("PRAGMA integrity_check;")
	if err != nil {
		return fmt.Sprintf("query-error: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return fmt.Sprintf("scan-error: %v", err)
		}
		if first == "" {
			first = msg
		}
	}
	if first == "" {
		first = "(no rows)"
	}
	return first
}

// ---------------------------------------------------------------------------
// Parquet
// ---------------------------------------------------------------------------

var parquetMagic = []byte("PAR1")

// parquetRow is the row schema for the valid Parquet fixture.
type parquetRow struct {
	ID    int64  `parquet:"id"`
	Kind  string `parquet:"kind"`
	Value int64  `parquet:"value"`
}

func genValidParquet() []byte {
	rows := make([]parquetRow, 0, 20)
	for i := int64(1); i <= 20; i++ {
		rows = append(rows, parquetRow{ID: i, Kind: fmt.Sprintf("k-%d", i), Value: i * 7})
	}
	var buf bytes.Buffer
	if err := parquet.Write(&buf, rows); err != nil {
		log.Fatalf("parquet write: %v", err)
	}
	data := buf.Bytes()
	if err := parquetSanityOK(data); err != nil {
		log.Fatalf("valid.parquet self-check: %v", err)
	}
	return data
}

// genCorruptParquet clobbers the 4-byte footer-length field that sits just
// before the trailing PAR1 magic. Both magics stay intact, but with a bogus
// footer length the reader seeks outside the file / reads garbage and the
// footer parse fails — exactly the "footer parse" detection path.
func genCorruptParquet(valid []byte) []byte {
	data := append([]byte(nil), valid...)
	n := len(data)
	// Layout tail: [footer metadata][footer-length: 4 bytes LE][PAR1: 4 bytes].
	// Clobber the footer-length bytes.
	for i := n - 8; i < n-4; i++ {
		data[i] = 0xFF
	}
	if err := parquetSanityOK(data); err == nil {
		log.Fatalf("corrupt.parquet still parses — corruption ineffective")
	} else {
		fmt.Printf("  parquet footer corruption detected: %v\n", err)
	}
	return data
}

// parquetSanityOK mirrors the Parquet assertion's structural check: it returns
// nil only if the file has valid PAR1 bookends and a parseable footer.
func parquetSanityOK(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("too small")
	}
	if !bytes.Equal(data[:4], parquetMagic) || !bytes.Equal(data[len(data)-4:], parquetMagic) {
		return fmt.Errorf("bad magic")
	}
	f, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("footer parse: %w", err)
	}
	if f.NumRows() == 0 || len(f.RowGroups()) == 0 {
		return fmt.Errorf("no rows/row groups")
	}
	return nil
}

// ---------------------------------------------------------------------------
// tar.gz
// ---------------------------------------------------------------------------

// genValidTarGz builds a tar.gz with 16 entries of varying size so that the
// sampling extraction (every 8th entry starting at 1) exercises at least two
// sampled entries (entries 1 and 9).
func genValidTarGz() []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i := 0; i < 16; i++ {
		name := fmt.Sprintf("data/file-%02d.txt", i)
		body := bytes.Repeat([]byte(fmt.Sprintf("line %d\n", i)), (i+1)*7)
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatalf("tar write header: %v", err)
		}
		if _, err := tw.Write(body); err != nil {
			log.Fatalf("tar write body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		log.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		log.Fatalf("gzip close: %v", err)
	}

	data := buf.Bytes()
	if err := tarGzSanityOK(data); err != nil {
		log.Fatalf("valid.tar.gz self-check: %v", err)
	}
	return data
}

// genCorruptTarGz flips bytes in the middle of the gzip payload. This breaks
// the compressed stream mid-archive: the tar reader either hits a gzip
// decompression/CRC error or a malformed tar header while listing or extracting
// a sampled entry — the failure must surface, not be swallowed.
func genCorruptTarGz(valid []byte) []byte {
	data := append([]byte(nil), valid...)
	// Skip the 10-byte gzip header; corrupt a cluster of payload bytes near the
	// middle of the stream.
	mid := len(data) / 2
	for i := mid; i < mid+64 && i < len(data)-8; i++ {
		data[i] ^= 0xFF
	}
	if err := tarGzSanityOK(data); err == nil {
		log.Fatalf("corrupt.tar.gz still parses — corruption ineffective")
	} else {
		fmt.Printf("  tar.gz payload corruption detected: %v\n", err)
	}
	return data
}

// tarGzSanityOK mirrors the tar.gz assertion: it fails if the stream cannot be
// fully listed or if a sampled entry fails to extract at its declared size.
func tarGzSanityOK(data []byte) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip header: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	entries := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("entry %d: %w", entries, err)
		}
		entries++
		if entries%8 == 1 {
			n, err := io.Copy(io.Discard, tr)
			if err != nil {
				return fmt.Errorf("extract %q: %w", hdr.Name, err)
			}
			if n != hdr.Size {
				return fmt.Errorf("size mismatch %q: %d != %d", hdr.Name, n, hdr.Size)
			}
		}
	}
	if entries == 0 {
		return fmt.Errorf("no entries")
	}
	return nil
}
