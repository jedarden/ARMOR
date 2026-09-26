// barman-cloud compatibility leg — a real barman-cloud-backup multipart
// upload followed by barman-cloud-restore and a live PostgreSQL query.
//
// The source database is deliberately large enough to cross barman's
// --min-chunk-size=5MB threshold.  Barman's tar writer flushes on tar-block
// boundaries, so the regular parts are not ARMOR encryption-block aligned;
// this is the production request shape covered by ADR-011.  The test uses a
// temporary local PostgreSQL cluster, so the release gate needs no database
// credential or network service beyond the ARMOR endpoint under test.
package awsclicompat

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type barmanConfig struct {
	Endpoint    string
	Region      string
	AccessKey   string
	SecretKey   string
	Destination string
	Server      string
	ChunkSize   string
}

func loadBarmanFixture(t *testing.T, endpoint, bucket string) barmanConfig {
	t.Helper()
	accessKey, secretKey := compatCredentials(t)

	rendered := string(renderFixture(t, "barman-cloud.env", map[string]string{
		"ENDPOINT":   endpoint,
		"REGION":     testRegion,
		"ACCESS_KEY": accessKey,
		"SECRET_KEY": secretKey,
		"BUCKET":     bucket,
	}))
	values := make(map[string]string)
	for _, line := range strings.Split(rendered, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid barman fixture line %q", line)
		}
		values[key] = value
	}
	return barmanConfig{
		Endpoint:    values["AWS_ENDPOINT_URL"],
		Region:      values["AWS_REGION"],
		AccessKey:   values["AWS_ACCESS_KEY_ID"],
		SecretKey:   values["AWS_SECRET_ACCESS_KEY"],
		Destination: values["BARMAN_DESTINATION"],
		Server:      values["BARMAN_SERVER"],
		ChunkSize:   values["BARMAN_MIN_CHUNK_SIZE"],
	}
}

func postgresBinDir(t *testing.T) string {
	t.Helper()
	requireClientBin(t, "pg_config", "install PostgreSQL server development tools")
	out := mustRun(t, "pg_config", nil, "--bindir")
	dir := strings.TrimSpace(out)
	for _, name := range []string{"initdb", "pg_ctl", "psql"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			if isCompatEndpointMode() {
				t.Fatalf("PostgreSQL tool %s is missing from pg_config --bindir=%s", name, dir)
			}
			t.Skipf("PostgreSQL tool %s is missing from pg_config --bindir=%s", name, dir)
		}
	}
	return dir
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve PostgreSQL port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startCompatPostgres(t *testing.T, binDir string) (dataDir string, port int) {
	t.Helper()
	dataDir = filepath.Join(t.TempDir(), "postgres")
	socketDir := filepath.Join(t.TempDir(), "socket")
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		t.Fatalf("create PostgreSQL socket directory: %v", err)
	}
	port = freeTCPPort(t)

	initdb := filepath.Join(binDir, "initdb")
	pgCtl := filepath.Join(binDir, "pg_ctl")
	psql := filepath.Join(binDir, "psql")
	mustRun(t, initdb, nil, "--no-locale", "--encoding=UTF8", "--auth=trust", "-D", dataDir)

	// The temporary cluster is local and disposable.  Trust authentication
	// keeps the fixture free of passwords while allowing barman's pg_basebackup
	// connection and the verification queries to use the same superuser.
	postgresConf := fmt.Sprintf("listen_addresses = '127.0.0.1'\nport = %d\nunix_socket_directories = '%s'\nwal_level = replica\nmax_wal_senders = 4\nfsync = off\nfull_page_writes = off\n", port, socketDir)
	if err := os.WriteFile(filepath.Join(dataDir, "postgresql.conf"), []byte(postgresConf), 0o600); err != nil {
		t.Fatalf("write PostgreSQL configuration: %v", err)
	}
	hba := "local all all trust\nhost all all 127.0.0.1/32 trust\nhost replication all 127.0.0.1/32 trust\n"
	if err := os.WriteFile(filepath.Join(dataDir, "pg_hba.conf"), []byte(hba), 0o600); err != nil {
		t.Fatalf("write PostgreSQL authentication configuration: %v", err)
	}

	pgCtlArgs := []string{"-D", dataDir, "-o", fmt.Sprintf("-p %d -k %s", port, socketDir), "-w", "start"}
	mustRun(t, pgCtl, nil, pgCtlArgs...)
	t.Cleanup(func() {
		if out, err := run(t, pgCtl, nil, "-D", dataDir, "-m", "immediate", "-w", "stop"); err != nil {
			t.Logf("stop temporary PostgreSQL: %v\n%s", err, out)
		}
	})

	// Seed ~25 MiB of incompressible-enough, deterministic relation data.  The
	// data pages make the barman tar stream span several 5MB chunks.
	seedSQL := `CREATE TABLE armor_compat (id integer PRIMARY KEY, payload text NOT NULL);
INSERT INTO armor_compat
SELECT i, (SELECT string_agg(md5(i::text || ':' || j::text), '') FROM generate_series(1, 512) AS g(j))
FROM generate_series(1, 3000) AS s(i);
CHECKPOINT;`
	mustRun(t, psql, nil, "-h", "127.0.0.1", "-p", strconv.Itoa(port), "-U", "postgres", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", seedSQL)
	return dataDir, port
}

func barmanEnv(t *testing.T, cfg barmanConfig) []string {
	t.Helper()
	return mergeEnv(os.Environ(), map[string]string{
		"AWS_ACCESS_KEY_ID":         cfg.AccessKey,
		"AWS_SECRET_ACCESS_KEY":     cfg.SecretKey,
		"AWS_DEFAULT_REGION":        cfg.Region,
		"AWS_REGION":                cfg.Region,
		"AWS_EC2_METADATA_DISABLED": "true",
	})
}

func backupIDFromOutput(t *testing.T, output string) string {
	t.Helper()
	match := regexp.MustCompile(`\b[0-9]{8}T[0-9]{6}\b`).FindString(output)
	if match == "" {
		t.Fatalf("barman-cloud-backup did not report a backup id:\n%s", output)
	}
	return match
}

func postgresDigest(t *testing.T, binDir string, port int) string {
	t.Helper()
	return strings.TrimSpace(mustRun(t, filepath.Join(binDir, "psql"), nil,
		"-h", "127.0.0.1", "-p", strconv.Itoa(port), "-U", "postgres", "-d", "postgres",
		"-At", "-v", "ON_ERROR_STOP=1", "-c",
		"SELECT count(*) || ':' || md5(string_agg(payload, '' ORDER BY id)) FROM armor_compat;"))
}

func TestBarmanCloud_BackupRestoreNonUniformParts(t *testing.T) {
	requireClientBin(t, "barman-cloud-backup", "install Barman 3.19.x and its cloud dependencies")
	requireClientBin(t, "barman-cloud-restore", "install Barman 3.19.x and its cloud dependencies")
	binDir := postgresBinDir(t)
	endpoint := startArmorServer(t)
	bucket := compatBucket(t)
	cfg := loadBarmanFixture(t, endpoint, bucket)
	dataDir, port := startCompatPostgres(t, binDir)
	env := barmanEnv(t, cfg)

	want := postgresDigest(t, binDir, port)
	backupOutput := mustRun(t, "barman-cloud-backup", env,
		"--cloud-provider", "aws-s3",
		"--endpoint-url", cfg.Endpoint,
		"--jobs", "1",
		"--min-chunk-size", cfg.ChunkSize,
		"--immediate-checkpoint",
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"--user", "postgres",
		cfg.Destination, cfg.Server)
	backupID := backupIDFromOutput(t, backupOutput)

	// Confirm that this was a multipart-sized base backup, rather than a
	// successful but too-small single PUT.  The configured 5MB threshold and
	// barman's tar-block flushes are the non-uniform ADR-011 request shape.
	client := newSDKClient(t, endpoint)
	prefix := strings.TrimPrefix(cfg.Destination, "s3://")
	if i := strings.IndexByte(prefix, '/'); i >= 0 {
		prefix = prefix[i+1:]
	}
	prefix = strings.TrimSuffix(prefix, "/") + "/" + cfg.Server + "/base/" + backupID
	listed, err := client.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{
		Bucket: bucketPtr(bucket),
		Prefix: &prefix,
	})
	if err != nil {
		t.Fatalf("list barman backup objects: %v", err)
	}
	var dataTarSize int64
	for _, object := range listed.Contents {
		if object.Key != nil && strings.HasSuffix(*object.Key, "/data.tar") && object.Size != nil {
			dataTarSize = *object.Size
		}
	}
	if dataTarSize < 10*1024*1024 {
		t.Fatalf("barman backup data.tar was %d bytes; expected a multipart-sized (>10MiB) backup", dataTarSize)
	}

	restoredDir := filepath.Join(t.TempDir(), "restored")
	mustRun(t, "barman-cloud-restore", env,
		"--cloud-provider", "aws-s3",
		"--endpoint-url", cfg.Endpoint,
		cfg.Destination, cfg.Server, backupID, restoredDir)
	for _, marker := range []string{"standby.signal", "recovery.signal"} {
		_ = os.Remove(filepath.Join(restoredDir, marker))
	}
	restorePort := freeTCPPort(t)
	restoreSocket := filepath.Join(t.TempDir(), "socket")
	if err := os.MkdirAll(restoreSocket, 0o700); err != nil {
		t.Fatalf("create restored PostgreSQL socket directory: %v", err)
	}
	restoreConf := fmt.Sprintf("listen_addresses = '127.0.0.1'\nport = %d\nunix_socket_directories = '%s'\n", restorePort, restoreSocket)
	if err := os.WriteFile(filepath.Join(restoredDir, "postgresql.conf"), []byte(restoreConf), 0o600); err != nil {
		t.Fatalf("write restored PostgreSQL configuration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(restoredDir, "pg_hba.conf"), []byte("local all all trust\nhost all all 127.0.0.1/32 trust\n"), 0o600); err != nil {
		t.Fatalf("write restored PostgreSQL authentication configuration: %v", err)
	}
	mustRun(t, filepath.Join(binDir, "pg_ctl"), nil, "-D", restoredDir, "-o", fmt.Sprintf("-p %d -k %s", restorePort, restoreSocket), "-w", "start")
	t.Cleanup(func() {
		if out, err := run(t, filepath.Join(binDir, "pg_ctl"), nil, "-D", restoredDir, "-m", "immediate", "-w", "stop"); err != nil {
			t.Logf("stop restored PostgreSQL: %v\n%s", err, out)
		}
	})

	got := postgresDigest(t, binDir, restorePort)
	if got != want {
		t.Fatalf("barman backup/recovery changed database contents: source=%s restored=%s", want, got)
	}
	t.Logf("barman-cloud backup/recovery OK: backup=%s data.tar=%d bytes, digest=%s", backupID, dataTarSize, got)

	// Keep the source cluster referenced so the cleanup order is obvious to
	// readers: both the source and the restored cluster are temporary only.
	_ = dataDir
}
