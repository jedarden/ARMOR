// Package awsclicompat contains the AWS CLI / rclone compatibility test suite
// for ARMOR described in docs/plan/plan.md ("Compatibility Tests").
//
// The suite spins up the real ARMOR S3 request pipeline (SigV4 auth,
// aws-chunked streaming decode, ACL enforcement, and the handlers in
// internal/server/handlers) backed by an in-memory mock backend — no B2 or
// Cloudflare credentials required — and then shells out to the real `aws` and
// `rclone` binaries pointed at the in-process server via --endpoint-url / an
// rclone S3 remote. Each round-tripped object is checked byte-for-byte against
// the original.
//
// Neither `aws` nor `rclone` is installed in the dev/CI image today, so every
// test detects binary absence (and the -short flag) and skips cleanly via
// t.Skip rather than failing. This keeps `go test ./...` and the Dockerfile
// test-gate green on machines without the CLIs. Installing the CLIs in CI so
// the suite actually runs there is a separate infra decision (see plan.md).
package awsclicompat

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server"
)

// Test credentials and region. The SigV4 verifier authenticates against the
// client-claimed region and looks the access key up in cfg.Credentials, so the
// CLI simply needs to be configured with the same pair.
const (
	testAccessKey = "ARMORCOMPAT"
	testSecretKey = "armorcompatsecretkey0123456789abcdef"
	testRegion    = "us-east-1"
	testBucket    = "compat-bucket"
)

// cmdTimeout bounds each CLI invocation so a hung subprocess cannot stall the
// test binary.
const cmdTimeout = 120 * time.Second

// mockBackend is an in-memory backend.Backend. It faithfully implements both the
// object primitives (Put/Get/Head/GetRange/Copy/Delete/List) used for single-shot
// uploads AND the dedicated multipart methods the handlers call for multipart
// uploads — internal/server/handlers/handlers.go drives multipart through
// backend.CreateMultipartUpload/UploadPart/CompleteMultipartUpload (not via
// Put/Get). UploadPart buffers each encrypted part; CompleteMultipartUpload
// concatenates the parts in part-number order into the final object, mirroring
// how B2 assembles parts server-side. The handler then stamps the ARMOR envelope
// onto the object via a CopyObject-with-replaceMetadata call, so a multipart
// upload round-trips through this mock exactly as it would through B2.
//
// All map access is guarded by mu: real CLI invocations (aws s3 sync, rclone
// copy) transfer multiple objects concurrently, and aws-cli uploads multiple
// parts of one object concurrently, so the backend must be safe for concurrent
// calls. Composite methods (GetRange → GetRangeWithHeaders, GetDirect → Get,
// HeadVersion → Head, DeleteObjects → Delete) delegate rather than re-locking to
// avoid a self-deadlock; the *Locked helpers do the work under a held lock.
type mockBackend struct {
	mu        sync.Mutex
	uploadSeq int64
	objects   map[string][]byte            // "bucket/key" -> ciphertext bytes
	meta      map[string]map[string]string // "bucket/key" -> S3 metadata headers
	modTime   map[string]time.Time         // "bucket/key" -> LastModified (real B2 always has one)
	parts     map[string]map[int32][]byte  // uploadID -> partNumber -> ciphertext bytes
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		objects: make(map[string][]byte),
		meta:    make(map[string]map[string]string),
		modTime: make(map[string]time.Time),
		parts:   make(map[string]map[int32][]byte),
	}
}

func (m *mockBackend) Put(_ context.Context, bucket, key string, body io.Reader, _ int64, meta map[string]string) error {
	// Read the body before taking the lock so a slow reader never blocks other calls.
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	k := bucket + "/" + key
	mc := make(map[string]string, len(meta))
	for mk, mv := range meta {
		mc[mk] = mv
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[k] = data
	m.meta[k] = mc
	m.modTime[k] = time.Now()
	return nil
}

func (m *mockBackend) getLocked(k, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	data, ok := m.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), m.infoFor(k, key, data), nil
}

func (m *mockBackend) Get(_ context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getLocked(bucket+"/"+key, key)
}

func (m *mockBackend) getRangeWithHeadersLocked(k, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	data, ok := m.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	if offset < 0 || offset > int64(len(data)) {
		return nil, nil, fmt.Errorf("offset out of range")
	}
	end := offset + length
	if end > int64(len(data)) || length < 0 {
		end = int64(len(data))
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), map[string]string{}, nil
}

// GetRange delegates to GetRangeWithHeaders (which locks). Do not lock here.
func (m *mockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	rc, _, err := m.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return rc, err
}

func (m *mockBackend) GetRangeWithHeaders(_ context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getRangeWithHeadersLocked(bucket+"/"+key, key, offset, length)
}

func (m *mockBackend) Head(_ context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	k := bucket + "/" + key
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[k]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	info := m.infoFor(k, key, data)
	return info, nil
}

func (m *mockBackend) Delete(_ context.Context, bucket, key string) error {
	k := bucket + "/" + key
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, k)
	delete(m.meta, k)
	delete(m.modTime, k)
	return nil
}

// DeleteObjects delegates to Delete (which locks each key). Do not lock here.
func (m *mockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		if err := m.Delete(ctx, bucket, key); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockBackend) List(_ context.Context, bucket, prefix, delimiter, _ string, _ int) (*backend.ListResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefixPath := bucket + "/" + prefix
	var objects []backend.ObjectInfo
	var commonPrefixes []string
	for k, data := range m.objects {
		if prefix != "" && !strings.HasPrefix(k, prefixPath) {
			continue
		}
		key := k[len(bucket)+1:]
		if delimiter != "" {
			if idx := strings.Index(key[len(prefix):], delimiter); idx != -1 {
				cp := key[:len(prefix)+idx+len(delimiter)]
				if !containsStr(commonPrefixes, cp) {
					commonPrefixes = append(commonPrefixes, cp)
				}
				continue
			}
		}
		objects = append(objects, *m.infoFor(k, key, data))
	}
	return &backend.ListResult{Objects: objects, CommonPrefixes: commonPrefixes}, nil
}

func (m *mockBackend) Copy(_ context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	src := srcBucket + "/" + srcKey
	dst := dstBucket + "/" + dstKey
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[src]
	if !ok {
		return fmt.Errorf("source object not found: %s", srcKey)
	}
	m.objects[dst] = data
	m.modTime[dst] = time.Now()
	if replaceMetadata {
		mc := make(map[string]string, len(meta))
		for mk, mv := range meta {
			mc[mk] = mv
		}
		m.meta[dst] = mc
	} else {
		merged := make(map[string]string)
		for mk, mv := range m.meta[src] {
			merged[mk] = mv
		}
		for mk, mv := range meta {
			merged[mk] = mv
		}
		m.meta[dst] = merged
	}
	return nil
}

func (m *mockBackend) ListBuckets(_ context.Context) ([]backend.BucketInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]struct{})
	for k := range m.objects {
		if b := strings.SplitN(k, "/", 2)[0]; b != "" {
			seen[b] = struct{}{}
		}
	}
	out := make([]backend.BucketInfo, 0, len(seen))
	for b := range seen {
		out = append(out, backend.BucketInfo{Name: b, CreationDate: time.Now()})
	}
	return out, nil
}

func (m *mockBackend) CreateBucket(_ context.Context, _ string) error { return nil }

func (m *mockBackend) DeleteBucket(_ context.Context, _ string) error { return nil }

// HeadBucket always succeeds. ARMOR fronts a single configured bucket, so for
// compatibility-test purposes every referenced bucket is treated as existing —
// this matches the real server's behavior for the configured bucket and avoids
// CLI bucket-probing flakiness across aws-cli/rclone versions.
func (m *mockBackend) HeadBucket(_ context.Context, _ string) error { return nil }

// GetDirect delegates to Get (which locks). Do not lock here.
func (m *mockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

// CreateMultipartUpload mints a unique upload id and primes a part buffer for it.
func (m *mockBackend) CreateMultipartUpload(_ context.Context, _, _ string, _ map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploadSeq++
	id := fmt.Sprintf("upload-%d", m.uploadSeq)
	m.parts[id] = make(map[int32][]byte)
	return id, nil
}

// UploadPart buffers the (already-encrypted) part bytes keyed by part number.
func (m *mockBackend) UploadPart(_ context.Context, _, _, uploadID string, partNumber int32, body io.Reader, _ int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parts, ok := m.parts[uploadID]
	if !ok {
		return "", fmt.Errorf("unknown upload id: %s", uploadID)
	}
	parts[partNumber] = data
	return fmt.Sprintf("etag-%d", partNumber), nil
}

// CompleteMultipartUpload concatenates the buffered parts in ascending
// part-number order into the final object — exactly what B2 does server-side —
// so the handler's subsequent metadata-stamping Copy and the final GET see a
// real, contiguous ciphertext object.
func (m *mockBackend) CompleteMultipartUpload(_ context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.parts[uploadID]
	if !ok {
		return "", fmt.Errorf("unknown upload id: %s", uploadID)
	}
	ordered := make([]backend.CompletedPart, len(parts))
	copy(ordered, parts)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].PartNumber < ordered[j].PartNumber })
	var buf bytes.Buffer
	for _, p := range ordered {
		buf.Write(stored[p.PartNumber])
	}
	final := buf.Bytes()
	m.objects[bucket+"/"+key] = final
	m.modTime[bucket+"/"+key] = time.Now()
	// The ARMOR envelope metadata is applied by the handler's following
	// CopyObject(replaceMetadata=true); nothing to set here.
	return fmt.Sprintf("final-etag-%d", len(final)), nil
}

func (m *mockBackend) AbortMultipartUpload(_ context.Context, _, _, uploadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.parts, uploadID)
	return nil
}

func (m *mockBackend) ListParts(_ context.Context, _, _, uploadID string) (*backend.ListPartsResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := &backend.ListPartsResult{}
	for partNum, data := range m.parts[uploadID] {
		res.Parts = append(res.Parts, backend.PartInfo{
			PartNumber:   partNum,
			Size:         int64(len(data)),
			ETag:         fmt.Sprintf("etag-%d", partNum),
			LastModified: time.Now(),
		})
	}
	return res, nil
}

func (m *mockBackend) ListMultipartUploads(_ context.Context, _ string) (*backend.ListMultipartUploadsResult, error) {
	return &backend.ListMultipartUploadsResult{}, nil
}

func (m *mockBackend) GetBucketLifecycleConfiguration(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("lifecycle configuration not found")
}

func (m *mockBackend) PutBucketLifecycleConfiguration(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *mockBackend) DeleteBucketLifecycleConfiguration(_ context.Context, _ string) error {
	return nil
}

func (m *mockBackend) GetObjectLockConfiguration(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("object lock configuration not found")
}

func (m *mockBackend) PutObjectLockConfiguration(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *mockBackend) GetObjectRetention(_ context.Context, _, _ string) ([]byte, error) {
	return nil, fmt.Errorf("retention not found")
}

func (m *mockBackend) PutObjectRetention(_ context.Context, _, _ string, _ []byte) error { return nil }

func (m *mockBackend) GetObjectLegalHold(_ context.Context, _, _ string) ([]byte, error) {
	return nil, fmt.Errorf("legal hold not found")
}

func (m *mockBackend) PutObjectLegalHold(_ context.Context, _, _ string, _ []byte) error { return nil }

func (m *mockBackend) ListObjectVersions(_ context.Context, bucket, prefix, _, _, _ string, _ int) (*backend.ListObjectVersionsResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := &backend.ListObjectVersionsResult{}
	for k, data := range m.objects {
		if !strings.HasPrefix(k, bucket+"/") {
			continue
		}
		key := strings.TrimPrefix(k, bucket+"/")
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		res.Versions = append(res.Versions, backend.ObjectVersionInfo{
			Key:          key,
			VersionID:    "v1",
			Size:         int64(len(data)),
			LastModified: time.Now(),
			IsLatest:     true,
		})
	}
	return res, nil
}

// HeadVersion delegates to Head (which locks). Do not lock here.
func (m *mockBackend) HeadVersion(ctx context.Context, bucket, key, _ string) (*backend.ObjectInfo, error) {
	return m.Head(ctx, bucket, key)
}

// infoFor builds an ObjectInfo for a stored object, decoding ARMOR envelope
// metadata (plaintext size, content type, etag) when present.
// infoFor builds an ObjectInfo for a stored object, decoding ARMOR envelope
// metadata (plaintext size, content type, etag) when present. The LastModified
// timestamp is the object's stored mod-time — a real B2 backend always returns
// one, and the aws CLI's date parser rejects the zero time ("date value out of
// range"), so we fall back to time.Now() rather than ever emit 0001-01-01.
func (m *mockBackend) infoFor(k, key string, data []byte) *backend.ObjectInfo {
	meta := m.meta[k]
	info := &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(data)),
		Metadata:         meta,
		IsARMOREncrypted: meta["x-amz-meta-armor-version"] != "",
	}
	if mt, ok := m.modTime[k]; ok {
		info.LastModified = mt
	} else {
		info.LastModified = time.Now()
	}
	if am, ok := backend.ParseARMORMetadata(meta); ok {
		info.Size = am.PlaintextSize
		info.ContentType = am.ContentType
		info.ETag = am.ETag
	}
	return info
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// testMEK is a fixed 32-byte master encryption key so the suite is deterministic.
func testMEK() []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i + 1)
	}
	return mek
}

// startArmorServer brings up an in-process ARMOR HTTP server backed by the mock
// backend and returns its base URL. The server is torn down automatically when
// the test ends.
func startArmorServer(t *testing.T) string {
	t.Helper()
	cfg := &config.Config{
		B2Region:        testRegion,
		MEK:             testMEK(),
		BlockSize:       65536,
		CacheMaxEntries: 1000,
		CacheTTL:        300,
		AuthAccessKey:   testAccessKey,
		AuthSecretKey:   testSecretKey,
		Credentials: map[string]*config.Credential{
			testAccessKey: {
				AccessKey: testAccessKey,
				SecretKey: testSecretKey,
				ACLs:      nil, // full access
			},
		},
	}
	srv, err := server.NewWithBackend(cfg, newMockBackend())
	if err != nil {
		t.Fatalf("NewWithBackend: %v", err)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return hs.URL
}

// requireAWSCLI skips the test unless the `aws` binary is available (and not
// running under -short). The message explains how to install it so the suite
// actually runs.
func requireAWSCLI(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping AWS CLI compatibility test in -short mode")
	}
	if _, err := exec.LookPath("aws"); err != nil {
		t.Skip("aws CLI not installed on PATH — skipping AWS CLI compatibility test " +
			"(install with e.g. `pip install awscli` or the official AWS CLI v2 bundle)")
	}
}

// requireRclone skips unless the `rclone` binary is available (and not -short).
func requireRclone(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping rclone compatibility test in -short mode")
	}
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Skip("rclone not installed on PATH — skipping rclone compatibility test " +
			"(install from https://rclone.org/install/)")
	}
}

// awsEnv builds the environment for an `aws` invocation: credentials and region
// via env vars (which override any user config), path-style addressing and an
// optional low multipart threshold via a throwaway AWS config file. region from
// the env.
func awsEnv(t *testing.T, endpoint string, multipart bool) []string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	var s3Block string
	if multipart {
		// Force multipart for files above 8 MiB, split into 8 MiB parts. A 9 MiB
		// test file therefore uploads as two parts, exercising ARMOR's
		// concurrent-part multipart path (ADR-005) at small scale.
		s3Block = "\ts3 =\n\t\taddressing_style = path\n\t\tmultipart_threshold = 8MB\n\t\tmultipart_chunksize = 8MB\n"
	} else {
		s3Block = "\ts3 =\n\t\taddressing_style = path\n"
	}
	configBody := "[default]\nregion = " + testRegion + "\n" + s3Block
	if err := os.WriteFile(cfgPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write aws config: %v", err)
	}

	env := mergeEnv(os.Environ(), map[string]string{
		"AWS_ACCESS_KEY_ID":         testAccessKey,
		"AWS_SECRET_ACCESS_KEY":     testSecretKey,
		"AWS_DEFAULT_REGION":        testRegion,
		"AWS_CONFIG_FILE":           cfgPath,
		"AWS_EC2_METADATA_DISABLED": "true",
		"AWS_ENDPOINT_URL":          endpoint,
	})
	return env
}

// rcloneConf writes an rclone.conf with an S3 remote named "armor" pointing at
// the in-process server and returns (configPath, remoteName).
func rcloneConf(t *testing.T, endpoint string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "rclone.conf")
	body := fmt.Sprintf("[armor]\ntype = s3\nprovider = Other\nendpoint = %s\n"+
		"access_key_id = %s\nsecret_access_key = %s\nregion = %s\n"+
		"force_path_style = true\nno_check_bucket = true\n",
		endpoint, testAccessKey, testSecretKey, testRegion)
	if err := os.WriteFile(confPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write rclone config: %v", err)
	}
	return confPath, "armor"
}

// run executes name with args under env, returning combined stdout/stderr. It
// fatals on a Start error or timeout but returns non-zero exit errors to the
// caller for assertion.
func run(t *testing.T, name string, env []string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	t.Logf("run: %s %s", name, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}

// mustRun fatals if the command exits non-zero, logging its output.
func mustRun(t *testing.T, name string, env []string, args ...string) string {
	t.Helper()
	out, err := run(t, name, env, args...)
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return out
}

// mergeEnv returns base with each key in overrides added or replaced.
func mergeEnv(base []string, overrides map[string]string) []string {
	present := make(map[string]bool, len(overrides))
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if v, ok := overrides[key]; ok {
			out = append(out, key+"="+v)
			present[key] = true
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		if !present[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// writeFile writes data to a file under name in dir.
func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// randomData returns n pseudo-random bytes (deterministic seed not required —
// content equality is checked per-run).
func randomData(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

// sha256File hex-encodes the SHA-256 of a file's contents.
func sha256File(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// assertFilesEqual fatals if the two files differ.
func assertFilesEqual(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	if w, g := sha256File(t, wantPath), sha256File(t, gotPath); w != g {
		t.Fatalf("content mismatch: want %s (sha256 %s), got %s (sha256 %s)", wantPath, w, gotPath, g)
	}
}
