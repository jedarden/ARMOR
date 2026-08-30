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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/presign"
	"github.com/jedarden/armor/internal/server"
	"github.com/klauspost/compress/zstd"
)

// Test credentials and region. The SigV4 verifier authenticates against the
// client-claimed region and looks the access key up in cfg.Credentials, so the
// CLI simply needs to be configured with the same pair.
const (
	testAccessKey     = "ARMORCOMPAT"
	testSecretKey     = "armorcompatsecretkey0123456789abcdef"
	testRegion        = "us-east-1"
	testPresignSecret = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
)

// isCompatEndpointMode returns true if ARMOR_COMPAT_ENDPOINT is set,
// indicating tests should target an external server instead of the in-process mock.
func isCompatEndpointMode() bool {
	return os.Getenv("ARMOR_COMPAT_ENDPOINT") != ""
}

// compatEndpointConfig returns the endpoint, access key, and secret key from
// environment variables when running in compat endpoint mode.
func compatEndpointConfig() (endpoint, accessKey, secretKey string) {
	endpoint = os.Getenv("ARMOR_COMPAT_ENDPOINT")
	accessKey = os.Getenv("ARMOR_COMPAT_ACCESS_KEY")
	secretKey = os.Getenv("ARMOR_COMPAT_SECRET_KEY")
	return
}

// compatBucket returns the bucket name to use. When ARMOR_COMPAT_ENDPOINT
// is set, reads from ARMOR_BUCKET; otherwise uses the test constant.
func compatBucket(t *testing.T) string {
	t.Helper()
	if isCompatEndpointMode() {
		bucket := os.Getenv("ARMOR_BUCKET")
		if bucket == "" {
			t.Fatalf("ARMOR_COMPAT_ENDPOINT requires ARMOR_BUCKET to be set")
		}
		return bucket
	}
	return testBucket
}

// bucketPtr returns a pointer to the bucket name string (for SDK calls that
// require *string parameters).
func bucketPtr(bucket string) *string {
	return &bucket
}

// testBucket is a variable so tests can take its address for SDK calls.
var testBucket = "compat-bucket"

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

func (m *mockBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return m.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
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

func (m *mockBackend) ListMultipartUploads(_ context.Context, _, _ string) (*backend.ListMultipartUploadsResult, error) {
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
	} else {
		// For uncompressed objects, fall back to Content-Type from metadata
		if ct, ok := meta["Content-Type"]; ok {
			info.ContentType = ct
		}
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
// the test ends. When ARMOR_COMPAT_ENDPOINT is set, returns that endpoint
// instead (for testing against a running server).
func startArmorServer(t *testing.T) string {
	t.Helper()
	if isCompatEndpointMode() {
		endpoint, _, _ := compatEndpointConfig()
		t.Logf("Using external ARMOR endpoint: %s", endpoint)
		return endpoint
	}
	cfg := &config.Config{
		B2Region:         testRegion,
		MEK:              testMEK(),
		BlockSize:        65536,
		CacheMaxEntries:  1000,
		CacheTTL:         300,
		AuthAccessKey:    testAccessKey,
		AuthSecretKey:    testSecretKey,
		FormatWriteVersion: 2, // Explicitly use v2 format - mock backend doesn't support v3 multipart storage
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
	// Initialize presigner for share token generation
	presignSecret, _ := hex.DecodeString(testPresignSecret)
	srv.SetPresigner(presign.NewSigner(presignSecret, ""))
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return hs.URL
}

// startArmorServerWithPresigner brings up an in-process ARMOR server and returns
// both the base URL and the presigner for generating share tokens in tests.
// When ARMOR_COMPAT_ENDPOINT is set, returns that endpoint and nil presigner
// (share token tests require the in-process server).
func startArmorServerWithPresigner(t *testing.T) (string, *presign.Signer) {
	t.Helper()
	if isCompatEndpointMode() {
		endpoint, _, _ := compatEndpointConfig()
		t.Logf("Using external ARMOR endpoint: %s (share token tests will be skipped)", endpoint)
		return endpoint, nil
	}
	cfg := &config.Config{
		B2Region:         testRegion,
		MEK:              testMEK(),
		BlockSize:        65536,
		CacheMaxEntries:  1000,
		CacheTTL:         300,
		AuthAccessKey:    testAccessKey,
		AuthSecretKey:    testSecretKey,
		FormatWriteVersion: 2, // Explicitly use v2 format - mock backend doesn't support v3 multipart storage
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
	// Initialize presigner for share token generation
	presignSecret, _ := hex.DecodeString(testPresignSecret)
	signer := presign.NewSigner(presignSecret, "")
	srv.SetPresigner(signer)
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return hs.URL, signer
}

// startRealArmorServer brings up an in-process ARMOR HTTP server backed by a REAL
// B2 backend (not mockBackend). This enables full HTTP API lifecycle testing that
// exercises ARMOR's S3 handler layer (SigV4 auth, routing, encryption/decryption)
// while still using real B2 storage. Returns the server URL for testing.
func startRealArmorServer(t *testing.T) string {
	t.Helper()

	// Get B2 credentials from environment
	region := os.Getenv("ARMOR_B2_REGION")
	accessKey := os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	secretKey := os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("ARMOR_BUCKET")
	cfDomain := os.Getenv("ARMOR_CF_DOMAIN") // optional

	if region == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("Set ARMOR_B2_REGION, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY, and ARMOR_BUCKET to run HTTP API lifecycle tests against real ARMOR server")
	}

	// Generate a test MEK if not provided
	mekStr := os.Getenv("ARMOR_MEK")
	var mek []byte
	if mekStr == "" {
		mek = testMEK()
	} else {
		var err error
		mek, err = hex.DecodeString(mekStr)
		if err != nil {
			t.Fatalf("Invalid ARMOR_MEK: %v", err)
		}
		if len(mek) != 32 {
			t.Fatalf("ARMOR_MEK must be 32 bytes (64 hex chars), got %d bytes", len(mek))
		}
	}

	// Get ARMOR auth credentials from environment (or generate test credentials)
	authAccessKey := os.Getenv("ARMOR_AUTH_ACCESS_KEY")
	authSecretKey := os.Getenv("ARMOR_AUTH_SECRET_KEY")
	if authAccessKey == "" || authSecretKey == "" {
		authAccessKey = testAccessKey
		authSecretKey = testSecretKey
	}

	// Build B2 endpoint URL for the region
	b2Endpoint := fmt.Sprintf("https://s3.%s.backblazeb2.com", region)

	cfg := &config.Config{
		B2Region:          region,
		B2Endpoint:        b2Endpoint,
		B2AccessKeyID:     accessKey,
		B2SecretAccessKey: secretKey,
		Bucket:            bucket,
		CFDomain:          cfDomain,
		MEK:               mek,
		BlockSize:         65536,
		CacheMaxEntries:   1000,
		CacheTTL:          300,
		AuthAccessKey:     authAccessKey,
		AuthSecretKey:     authSecretKey,
		Credentials: map[string]*config.Credential{
			authAccessKey: {
				AccessKey: authAccessKey,
				SecretKey: authSecretKey,
				ACLs:      nil, // full access
			},
		},
	}

	srv, err := server.New(cfg) // Uses real B2 backend, not mockBackend
	if err != nil {
		t.Fatalf("server.New with real B2 backend: %v", err)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	t.Logf("Started real ARMOR server at %s (bucket=%s, region=%s)", hs.URL, bucket, region)
	return hs.URL
}

// requireAWSCLI skips the test unless the `aws` binary is available (and not
// running under -short). The message explains how to install it so the suite
// actually runs. When ARMOR_COMPAT_ENDPOINT is set, missing aws is a fatal
// error rather than a skip.
func requireAWSCLI(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping AWS CLI compatibility test in -short mode")
	}
	if _, err := exec.LookPath("aws"); err != nil {
		if isCompatEndpointMode() {
			t.Fatalf("aws CLI not installed on PATH but required in ARMOR_COMPAT_ENDPOINT mode " +
				"(install with e.g. `pip install awscli` or the official AWS CLI v2 bundle)")
		}
		t.Skip("aws CLI not installed on PATH — skipping AWS CLI compatibility test " +
			"(install with e.g. `pip install awscli` or the official AWS CLI v2 bundle)")
	}
}

// requireRclone skips unless the `rclone` binary is available (and not -short).
// When ARMOR_COMPAT_ENDPOINT is set, missing rclone is a fatal error rather
// than a skip.
func requireRclone(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping rclone compatibility test in -short mode")
	}
	if _, err := exec.LookPath("rclone"); err != nil {
		if isCompatEndpointMode() {
			t.Fatalf("rclone not installed on PATH but required in ARMOR_COMPAT_ENDPOINT mode " +
				"(install from https://rclone.org/install/)")
		}
		t.Skip("rclone not installed on PATH — skipping rclone compatibility test " +
			"(install from https://rclone.org/install/)")
	}
}

// awsEnv builds the environment for an `aws` invocation: credentials and region
// via env vars (which override any user config), path-style addressing and an
// optional low multipart threshold via a throwaway AWS config file. region from
// the env. When ARMOR_COMPAT_ENDPOINT is set, uses credentials from the
// environment instead of the test constants.
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

	// Use credentials from environment when in endpoint mode
	accessKey := testAccessKey
	secretKey := testSecretKey
	if isCompatEndpointMode() {
		_, ak, sk := compatEndpointConfig()
		if ak == "" || sk == "" {
			t.Fatalf("ARMOR_COMPAT_ENDPOINT requires ARMOR_COMPAT_ACCESS_KEY and ARMOR_COMPAT_SECRET_KEY")
		}
		accessKey = ak
		secretKey = sk
	}

	env := mergeEnv(os.Environ(), map[string]string{
		"AWS_ACCESS_KEY_ID":         accessKey,
		"AWS_SECRET_ACCESS_KEY":     secretKey,
		"AWS_DEFAULT_REGION":        testRegion,
		"AWS_CONFIG_FILE":           cfgPath,
		"AWS_EC2_METADATA_DISABLED": "true",
		"AWS_ENDPOINT_URL":          endpoint,
	})
	return env
}

// rcloneConf writes an rclone.conf with an S3 remote named "armor" pointing at
// the in-process server and returns (configPath, remoteName). When
// ARMOR_COMPAT_ENDPOINT is set, uses credentials from the environment instead
// of the test constants.
func rcloneConf(t *testing.T, endpoint string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "rclone.conf")

	// Use credentials from environment when in endpoint mode
	accessKey := testAccessKey
	secretKey := testSecretKey
	if isCompatEndpointMode() {
		_, ak, sk := compatEndpointConfig()
		if ak == "" || sk == "" {
			t.Fatalf("ARMOR_COMPAT_ENDPOINT requires ARMOR_COMPAT_ACCESS_KEY and ARMOR_COMPAT_SECRET_KEY")
		}
		accessKey = ak
		secretKey = sk
	}

	body := fmt.Sprintf("[armor]\ntype = s3\nprovider = Other\nendpoint = %s\n"+
		"access_key_id = %s\nsecret_access_key = %s\nregion = %s\n"+
		"force_path_style = true\nno_check_bucket = true\n",
		endpoint, accessKey, secretKey, testRegion)
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

// GETResponse represents a parsed GET operation response with status code,
// body data, and extracted metadata.
type GETResponse struct {
	StatusCode    int
	Body          []byte
	ContentLength int64
	ETag          string
	ContentType   string
	LastModified  string
	Headers       map[string]string
}

// parseGETResponse parses an HTTP GET response, extracting the body data,
// status code, and object metadata headers. Returns an error if the response
// cannot be read or the status code indicates failure.
func parseGETResponse(resp *http.Response) (*GETResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading GET response body: %w", err)
	}
	defer resp.Body.Close()

	// Extract metadata from headers
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Build parsed response
	parsed := &GETResponse{
		StatusCode:    resp.StatusCode,
		Body:          body,
		ContentLength: resp.ContentLength,
		Headers:       headers,
	}

	// Extract common S3 metadata headers
	parsed.ETag = resp.Header.Get("ETag")
	parsed.ContentType = resp.Header.Get("Content-Type")
	parsed.LastModified = resp.Header.Get("Last-Modified")

	return parsed, nil
}

// assertGETSuccess verifies that a GET response succeeded (HTTP 200) and
// returns the parsed response for further inspection. Fatals on failure.
func assertGETSuccess(t *testing.T, resp *http.Response) *GETResponse {
	t.Helper()

	parsed, err := parseGETResponse(resp)
	if err != nil {
		t.Fatalf("GET response parsing failed: %v", err)
	}

	if parsed.StatusCode != http.StatusOK {
		t.Fatalf("GET returned status %d, body: %s", parsed.StatusCode, string(parsed.Body))
	}

	return parsed
}

// TestHarness_GET_BasicUncompressedObject tests basic GET operation on an uncompressed object.
// This test verifies that GET can be invoked from the test framework and returns expected results.
func TestHarness_GET_BasicUncompressedObject(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()

	// Test data - small uncompressed object
	testData := []byte("Basic uncompressed test object content")
	key := "get-test/basic-uncompressed.txt"

	// First, PUT the object using SDK
	putIn := &s3.PutObjectInput{
		Bucket: bucketPtr(compatBucket(t)),
		Key:    &key,
		Body:   bytes.NewReader(testData),
	}

	if _, err := client.PutObject(ctx, putIn); err != nil {
		t.Fatalf("PUT failed: %v", err)
	}

	t.Logf("PUT succeeded for %s", key)

	// Now perform GET operation using SDK
	getIn := &s3.GetObjectInput{
		Bucket: bucketPtr(compatBucket(t)),
		Key:    &key,
	}

	getOut, err := client.GetObject(ctx, getIn)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getOut.Body.Close()

	// Read response body
	body, err := io.ReadAll(getOut.Body)
	if err != nil {
		t.Fatalf("Failed to read GET response body: %v", err)
	}

	// Verify content matches
	if !bytes.Equal(body, testData) {
		t.Fatalf("GET content mismatch: got %d bytes, want %d bytes", len(body), len(testData))
	}

	// Verify Content-Length header
	if getOut.ContentLength == nil || *getOut.ContentLength != int64(len(testData)) {
		t.Errorf("GET Content-Length mismatch: got %v, want %d", getOut.ContentLength, len(testData))
	}

	t.Logf("GET basic uncompressed object test passed: %d bytes retrieved", len(body))
}

// TestHarness_GET_MultipleUncompressedObjects tests GET operations on multiple
// uncompressed objects with different sizes and content patterns.
func TestHarness_GET_MultipleUncompressedObjects(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()

	testCases := []struct {
		name string
		key  string
		data []byte
	}{
		{
			name: "small-text",
			key:  "get-multi/small.txt",
			data: []byte("Small text object"),
		},
		{
			name: "medium-binary",
			key:  "get-multi/medium.bin",
			data: bytes.Repeat([]byte{0xAB, 0xCD, 0xEF}, 1000),
		},
		{
			name: "large-text",
			key:  "get-multi/large.txt",
			data: bytes.Repeat([]byte("Large text pattern "), 1000),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// PUT the object using SDK
			putIn := &s3.PutObjectInput{
				Bucket: bucketPtr(compatBucket(t)),
				Key:    &tc.key,
				Body:   bytes.NewReader(tc.data),
			}

			if _, err := client.PutObject(ctx, putIn); err != nil {
				t.Fatalf("PUT failed: %v", err)
			}

			// GET the object using SDK
			getIn := &s3.GetObjectInput{
				Bucket: bucketPtr(compatBucket(t)),
				Key:    &tc.key,
			}

			getOut, err := client.GetObject(ctx, getIn)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer getOut.Body.Close()

			// Verify GET response
			body, err := io.ReadAll(getOut.Body)
			if err != nil {
				t.Fatalf("Failed to read GET response body: %v", err)
			}

			if !bytes.Equal(body, tc.data) {
				t.Errorf("GET content mismatch for %s: got %d bytes, want %d bytes",
					tc.key, len(body), len(tc.data))
			}

			t.Logf("GET %s passed: %d bytes", tc.name, len(body))
		})
	}
}

// TestHarness_GET_NonexistentObject verifies that GET returns 404 for
// objects that don't exist.
func TestHarness_GET_NonexistentObject(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()

	key := "get-test/does-not-exist.txt"

	// Try to GET a non-existent object using SDK
	getIn := &s3.GetObjectInput{
		Bucket: bucketPtr(compatBucket(t)),
		Key:    &key,
	}

	_, err := client.GetObject(ctx, getIn)

	// Verify we get a NotFound error
	if err == nil {
		t.Fatalf("GET expected error for non-existent object, got nil error")
	}

	var noSuchKey *types.NoSuchKey
	if !strings.Contains(err.Error(), "NoSuchKey") && !errors.As(err, &noSuchKey) {
		t.Logf("GET error for non-existent object: %v", err)
		// The error should mention the object doesn't exist
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "NoSuchKey") {
			t.Errorf("Expected NotFound/NoSuchKey error, got: %v", err)
		}
	}

	t.Logf("GET nonexistent object correctly returned error: %v", err)
}

// TestMockBackend_GET_UncompressedObject tests GET operations directly on the mockBackend
// for uncompressed objects, bypassing HTTP and signature complexity.
// This verifies that the backend correctly stores and retrieves uncompressed objects.
func TestMockBackend_GET_UncompressedObject(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	// Test data - uncompressed object
	testData := []byte("Uncompressed test data for GET operation")
	key := "get-uncompressed/test.txt"
	meta := map[string]string{
		"Content-Type": "text/plain",
	}

	// PUT the object
	err := backend.Put(ctx, compatBucket(t), key, bytes.NewReader(testData), int64(len(testData)), meta)
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}

	// GET the object
	body, info, err := backend.Get(ctx, compatBucket(t), key)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer body.Close()

	// Verify content
	retrievedData, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read GET response body: %v", err)
	}

	if !bytes.Equal(retrievedData, testData) {
		t.Errorf("GET content mismatch: got %d bytes, want %d bytes", len(retrievedData), len(testData))
		t.Logf("Retrieved: %q", retrievedData)
		t.Logf("Expected:  %q", testData)
	}

	// Verify metadata
	if info.Size != int64(len(testData)) {
		t.Errorf("GET size mismatch: got %d, want %d", info.Size, len(testData))
	}

	if info.ContentType != "text/plain" {
		t.Errorf("GET Content-Type mismatch: got %s, want text/plain", info.ContentType)
	}

	if info.Key != key {
		t.Errorf("GET key mismatch: got %s, want %s", info.Key, key)
	}

	t.Logf("GET uncompressed object test passed: %d bytes retrieved", len(retrievedData))
}

// TestMockBackend_GET_MultipleUncompressedObjects tests GET operations on multiple
// uncompressed objects of different sizes and content patterns.
func TestMockBackend_GET_MultipleUncompressedObjects(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	testCases := []struct {
		name        string
		key         string
		data        []byte
		contentType string
	}{
		{
			name:        "small-text",
			key:         "get-multi/small.txt",
			data:        []byte("Small text object"),
			contentType: "text/plain",
		},
		{
			name:        "medium-binary",
			key:         "get-multi/medium.bin",
			data:        bytes.Repeat([]byte{0xAB, 0xCD, 0xEF}, 1000),
			contentType: "application/octet-stream",
		},
		{
			name:        "large-text",
			key:         "get-multi/large.txt",
			data:        bytes.Repeat([]byte("Large text pattern "), 1000),
			contentType: "text/plain",
		},
		{
			name:        "empty-object",
			key:         "get-multi/empty.txt",
			data:        []byte{},
			contentType: "text/plain",
		},
		{
			name:        "single-byte",
			key:         "get-multi/single.txt",
			data:        []byte{0x42},
			contentType: "application/octet-stream",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			meta := map[string]string{"Content-Type": tc.contentType}

			// PUT the object
			err := backend.Put(ctx, compatBucket(t), tc.key, bytes.NewReader(tc.data), int64(len(tc.data)), meta)
			if err != nil {
				t.Fatalf("PUT failed: %v", err)
			}

			// GET the object
			body, info, err := backend.Get(ctx, compatBucket(t), tc.key)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer body.Close()

			// Verify content
			retrievedData, err := io.ReadAll(body)
			if err != nil {
				t.Fatalf("Failed to read GET response body: %v", err)
			}

			if !bytes.Equal(retrievedData, tc.data) {
				t.Errorf("GET content mismatch for %s: got %d bytes, want %d bytes",
					tc.key, len(retrievedData), len(tc.data))
			}

			// Verify metadata
			if info.Size != int64(len(tc.data)) {
				t.Errorf("GET size mismatch for %s: got %d, want %d",
					tc.key, info.Size, len(tc.data))
			}

			if info.ContentType != tc.contentType {
				t.Errorf("GET Content-Type mismatch for %s: got %s, want %s",
					tc.key, info.ContentType, tc.contentType)
			}

			t.Logf("GET %s passed: %d bytes", tc.name, len(retrievedData))
		})
	}
}

// TestMockBackend_GET_NonexistentObject verifies that GET returns an error
// for objects that don't exist.
func TestMockBackend_GET_NonexistentObject(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	key := "does-not-exist.txt"

	// Try to GET a non-existent object
	body, info, err := backend.Get(ctx, compatBucket(t), key)

	if err == nil {
		defer body.Close()
		t.Errorf("GET expected error for non-existent object, got nil error and info: %+v", info)
	}

	expectedErrMsg := "object not found"
	if err == nil || !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("GET error message should contain %q, got: %v", expectedErrMsg, err)
	}

	t.Logf("GET nonexistent object correctly returned error: %v", err)
}

// TestMockBackend_GET_ConcurrentOperations tests that GET operations work correctly
// under concurrent access.
func TestMockBackend_GET_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	// Create multiple objects
	numObjects := 10
	objects := make(map[string][]byte)
	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("concurrent/obj%d.txt", i)
		data := []byte(fmt.Sprintf("Object %d data", i))
		meta := map[string]string{"Content-Type": "text/plain"}

		err := backend.Put(ctx, compatBucket(t), key, bytes.NewReader(data), int64(len(data)), meta)
		if err != nil {
			t.Fatalf("PUT failed for %s: %v", key, err)
		}
		objects[key] = data
	}

	// Concurrently GET all objects
	var wg sync.WaitGroup
	errors := make(chan error, numObjects)

	for key := range objects {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			body, info, err := backend.Get(ctx, compatBucket(t), k)
			if err != nil {
				errors <- fmt.Errorf("GET failed for %s: %v", k, err)
				return
			}
			defer body.Close()

			retrievedData, err := io.ReadAll(body)
			if err != nil {
				errors <- fmt.Errorf("read failed for %s: %v", k, err)
				return
			}

			if !bytes.Equal(retrievedData, objects[k]) {
				errors <- fmt.Errorf("content mismatch for %s: got %d bytes, want %d bytes",
					k, len(retrievedData), len(objects[k]))
			}

			if info.Size != int64(len(objects[k])) {
				errors <- fmt.Errorf("size mismatch for %s: got %d, want %d",
					k, info.Size, len(objects[k]))
			}
		}(key)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent GET operation error: %v", err)
	}

	t.Logf("Concurrent GET operations completed successfully for %d objects", numObjects)
}

// compressData compresses data using zstd for testing
func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	if _, err := encoder.Write(data); err != nil {
		encoder.Close()
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to close encoder: %w", err)
	}

	return buf.Bytes(), nil
}

// TestMockBackend_GET_CompressedObject tests GET operations directly on the mockBackend
// for compressed objects, bypassing HTTP and signature complexity.
// This verifies that compressed objects can be stored and retrieved correctly.
func TestMockBackend_GET_CompressedObject(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	// Test data - compressible content
	originalData := bytes.Repeat([]byte("Compressible test data pattern "), 100)
	key := "get-compressed/test.txt"

	// Compress the data
	compressedData, err := compressData(originalData)
	if err != nil {
		t.Fatalf("Failed to compress test data: %v", err)
	}

	// Verify compression actually happened
	if bytes.Equal(compressedData, originalData) {
		t.Fatal("Compressed data should differ from original")
	}

	// Store the compressed data with ARMOR compression metadata
	meta := map[string]string{
		"Content-Type":                    "text/plain",
		"x-amz-meta-armor-compressed":     "true",
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(originalData)),
	}

	err = backend.Put(ctx, compatBucket(t), key, bytes.NewReader(compressedData), int64(len(compressedData)), meta)
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}

	// GET the compressed object
	body, info, err := backend.Get(ctx, compatBucket(t), key)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer body.Close()

	// Read the compressed data from backend
	retrievedData, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read GET response body: %v", err)
	}

	// Verify the retrieved data is still compressed (backend stores as-is)
	if !bytes.Equal(retrievedData, compressedData) {
		t.Errorf("Backend should return compressed data as-is")
		t.Logf("Retrieved: %d bytes, Stored: %d bytes", len(retrievedData), len(compressedData))
	}

	// Verify metadata indicates compression
	if compressedFlag, ok := info.Metadata["x-amz-meta-armor-compressed"]; !ok || compressedFlag != "true" {
		t.Errorf("Expected x-amz-meta-armor-compressed=true, got: %v", compressedFlag)
	}

	t.Logf("GET compressed object test passed: %d bytes stored (original was %d bytes)",
		len(retrievedData), len(originalData))
}

// TestMockBackend_GET_CompressedAndUncompressed verifies that both compressed
// and uncompressed objects with the same content can be retrieved correctly.
func TestMockBackend_GET_CompressedAndUncompressed(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	// Test data - highly compressible content
	originalData := []byte(strings.Repeat("AAAAABBBBBCCCCCDDDDDEEEEE", 200))

	// Create compressed version
	compressedData, err := compressData(originalData)
	if err != nil {
		t.Fatalf("Failed to compress test data: %v", err)
	}

	compressedKey := "test-comparison/compressed.bin"
	uncompressedKey := "test-comparison/uncompressed.bin"

	// PUT compressed object
	compressedMeta := map[string]string{
		"Content-Type":                    "application/octet-stream",
		"x-amz-meta-armor-compressed":     "true",
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(originalData)),
	}

	err = backend.Put(ctx, compatBucket(t), compressedKey, bytes.NewReader(compressedData), int64(len(compressedData)), compressedMeta)
	if err != nil {
		t.Fatalf("PUT compressed failed: %v", err)
	}

	// PUT uncompressed object
	uncompressedMeta := map[string]string{
		"Content-Type":                    "application/octet-stream",
		"x-amz-meta-armor-compressed":     "false",
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(originalData)),
	}

	err = backend.Put(ctx, compatBucket(t), uncompressedKey, bytes.NewReader(originalData), int64(len(originalData)), uncompressedMeta)
	if err != nil {
		t.Fatalf("PUT uncompressed failed: %v", err)
	}

	// GET compressed object and verify
	body1, info1, err := backend.Get(ctx, compatBucket(t), compressedKey)
	if err != nil {
		t.Fatalf("GET compressed failed: %v", err)
	}
	defer body1.Close()

	retrievedCompressed, err := io.ReadAll(body1)
	if err != nil {
		t.Fatalf("Failed to read compressed GET response: %v", err)
	}

	// Verify compressed data matches what we stored
	if !bytes.Equal(retrievedCompressed, compressedData) {
		t.Errorf("Compressed data mismatch: got %d bytes, want %d bytes",
			len(retrievedCompressed), len(compressedData))
	}

	// GET uncompressed object and verify
	body2, info2, err := backend.Get(ctx, compatBucket(t), uncompressedKey)
	if err != nil {
		t.Fatalf("GET uncompressed failed: %v", err)
	}
	defer body2.Close()

	retrievedUncompressed, err := io.ReadAll(body2)
	if err != nil {
		t.Fatalf("Failed to read uncompressed GET response: %v", err)
	}

	// Verify uncompressed data matches original
	if !bytes.Equal(retrievedUncompressed, originalData) {
		t.Errorf("Uncompressed data mismatch: got %d bytes, want %d bytes",
			len(retrievedUncompressed), len(originalData))
	}

	// Verify metadata flags are different
	if info1.Metadata["x-amz-meta-armor-compressed"] != "true" {
		t.Errorf("Expected compressed=true for compressed object, got: %v",
			info1.Metadata["x-amz-meta-armor-compressed"])
	}

	if info2.Metadata["x-amz-meta-armor-compressed"] != "false" {
		t.Errorf("Expected compressed=false for uncompressed object, got: %v",
			info2.Metadata["x-amz-meta-armor-compressed"])
	}

	t.Logf("Both compressed and uncompressed objects retrieved successfully")
	t.Logf("Compressed: %d bytes -> %d bytes (%.1f%% reduction)",
		len(originalData), len(compressedData),
		100.0*(1.0-float64(len(compressedData))/float64(len(originalData))))
}

// TestHarness_GET_CompressedObjectViaHTTP tests GET operations on compressed objects
// through the full HTTP API using the SDK.
func TestHarness_GET_CompressedObjectViaHTTP(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()

	// Test data - compressible content
	originalData := bytes.Repeat([]byte("HTTP compressed test data "), 50)

	// Compress the data
	compressedData, err := compressData(originalData)
	if err != nil {
		t.Fatalf("Failed to compress test data: %v", err)
	}

	key := "get-http-test/compressed.txt"

	// Create ARMOR metadata for compressed object
	contentType := "text/plain"
	armorMeta := map[string]string{
		"x-amz-meta-armor-version":          "1",
		"x-amz-meta-armor-block-size":       "65536",
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(originalData)),
		"x-amz-meta-armor-content-type":     contentType,
		"x-amz-meta-armor-iv":               hex.EncodeToString([]byte("0123456789123456")),
		"x-amz-meta-armor-wrapped-dek":      hex.EncodeToString(bytes.Repeat([]byte("dek"), 32)),
		"x-amz-meta-armor-plaintext-sha256": hex.EncodeToString(bytes.Repeat([]byte("sha"), 32)),
		"x-amz-meta-armor-etag":             "\"test-etag\"",
		"x-amz-meta-armor-compressed":       "true",
	}

	// PUT the compressed object via SDK
	putIn := &s3.PutObjectInput{
		Bucket:   bucketPtr(compatBucket(t)),
		Key:      &key,
		Body:     bytes.NewReader(compressedData),
		Metadata: armorMeta,
	}

	if _, err := client.PutObject(ctx, putIn); err != nil {
		t.Fatalf("PUT failed: %v", err)
	}

	t.Logf("PUT succeeded for compressed object %s", key)

	// GET the compressed object via SDK
	getIn := &s3.GetObjectInput{
		Bucket: bucketPtr(compatBucket(t)),
		Key:    &key,
	}

	getOut, err := client.GetObject(ctx, getIn)
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer getOut.Body.Close()

	// Read response body
	body, err := io.ReadAll(getOut.Body)
	if err != nil {
		t.Fatalf("Failed to read GET response body: %v", err)
	}

	t.Logf("GET compressed object test passed: %d bytes retrieved", len(body))
}

// TestDecompressionUtilities tests the compression/decompression utilities
func TestDecompressionUtilities(t *testing.T) {
	// Test compression and decompression round-trip
	originalData := []byte("Test data for compression utilities")

	compressed, err := compressData(originalData)
	if err != nil {
		t.Fatalf("compressData failed: %v", err)
	}

	// Verify it's actually compressed (should be different)
	if bytes.Equal(compressed, originalData) {
		t.Error("Compressed data should differ from original for non-trivial input")
	}

	// Verify compression detection
	if !crypto.IsCompressed(compressed) {
		t.Error("IsCompressed should return true for zstd-compressed data")
	}

	// Verify decompression
	decompressed, err := crypto.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !bytes.Equal(decompressed, originalData) {
		t.Errorf("Decompressed data doesn't match original: got %d bytes, want %d bytes",
			len(decompressed), len(originalData))
	}

	// Test that decompressing uncompressed data returns it unchanged
	result, err := crypto.Decompress(originalData)
	if err != nil {
		t.Fatalf("Decompress of uncompressed data failed: %v", err)
	}

	if !bytes.Equal(result, originalData) {
		t.Error("Decompress should return data unchanged if not compressed")
	}

	t.Logf("Compression utilities test passed")
}
