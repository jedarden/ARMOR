// cmd_verify_binary_contract_test.go runs the real armor binary end to end
// against an in-process fake S3 endpoint and pins the 'armor verify' contract
// the way AGENTS.md documents it (armor-121caf08): mixed OK/CORRUPTED/ERROR
// inventories, fingerprinted-DEK resolution with escrow ring fallback, v1/v2
// and v3 multipart verification through the ADR-016 manifest, exactly one
// report row per selected object, and the exit-code table (0 all OK, 1 any
// failure or empty key set, 2 usage). Everything in-process above this file
// (cmd_verify_test.go, cmd_verify_multipart_test.go) stubs the backend; only
// here does flag parsing, env handling, the aws-sdk-go-v2 wire, report
// emission and process exit status all run for real.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// --- fake S3 endpoint -------------------------------------------------------
//
// verify talks to storage exclusively through backend.NewB2Backend, an
// aws-sdk-go-v2 S3 client in path-style mode. The fake below serves exactly
// the three operations the verify walk issues -- HeadObject, ranged GetObject
// and ListObjectsV2 -- with the wire shapes real B2 produces.

// fakeS3Object is one stored object. meta uses ARMOR's canonical metadata key
// names (x-amz-meta-armor-*), matching what the server's Put puts in the SDK
// Metadata map.
type fakeS3Object struct {
	data         []byte
	meta         map[string]string
	lastModified time.Time
}

// fakeS3Bucket is a single-bucket fake S3 endpoint.
type fakeS3Bucket struct {
	mu      sync.Mutex
	bucket  string
	objects map[string]fakeS3Object
}

// newFakeS3Bucket starts the fake and registers its shutdown with the test.
func newFakeS3Bucket(t *testing.T) (*fakeS3Bucket, *httptest.Server) {
	t.Helper()
	b := &fakeS3Bucket{bucket: "verify-contract", objects: map[string]fakeS3Object{}}
	server := httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(server.Close)
	return b, server
}

// put stores one object; meta keys are ARMOR-canonical (x-amz-meta-armor-*).
func (b *fakeS3Bucket) put(key string, data []byte, meta map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[key] = fakeS3Object{data: data, meta: meta, lastModified: time.Now().Add(-time.Hour)}
}

// putFixtureV2 stores a v1/v2 multipart fixture with its ARMOR parameters on
// the object's own head (the metadata survives single-PUT-style on this fake;
// the manifest is not involved) plus the flat JSON sidecar at its key.
func (b *fakeS3Bucket) putFixtureV2(t *testing.T, f *multipartFixture) {
	t.Helper()
	b.put(f.storedKey(), f.ciphertext, f.meta)
	b.put(f.sidecarObjectKey(), f.sidecarJSON, nil)
}

// putFixture stores a multipart fixture the way CompleteMultipartUpload leaves
// it on B2: ciphertext under the stored key with a metadata-free head, the
// HMAC sidecar at .armor/hmac/<sha256(client key)> (gzipped for v3), and the
// ADR-016 manifest beside the object. manifestMeta mirrors the fixture's
// putHeadEmpty: when true the manifest object's own headers carry the ARMOR
// map, when false only the manifest JSON body does.
func (b *fakeS3Bucket) putFixture(t *testing.T, f *multipartFixture, manifestMeta bool) {
	t.Helper()
	sidecar := f.sidecarJSON
	if f.isV3 {
		sidecar = f.gzipSidecar()
	}
	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: f.storedKey(),
		UploadID:         "binary-contract-upload",
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata:         f.meta,
	})
	if err != nil {
		t.Fatalf("marshal manifest body: %v", err)
	}
	b.put(f.storedKey(), f.ciphertext, map[string]string{})
	b.put(f.sidecarObjectKey(), sidecar, nil)
	b.put(f.storedKey()+verifyManifestSuffix, manifestJSON, orNilMeta(manifestMeta, f.meta))
}

func orNilMeta(with bool, meta map[string]string) map[string]string {
	if with {
		return meta
	}
	return nil
}

func (b *fakeS3Bucket) lookup(key string) (fakeS3Object, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	obj, ok := b.objects[key]
	return obj, ok
}

func (b *fakeS3Bucket) keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := make([]string, 0, len(b.objects))
	for k := range b.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// serve dispatches one request. HEAD strips nothing; GET serves full bodies or
// single ranges; GET /{bucket}?list-type=2 is ListObjectsV2.
func (b *fakeS3Bucket) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("list-type") != "" {
		b.serveList(w, r)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/"+b.bucket+"/")
	if key == r.URL.Path || key == "" {
		writeS3Error(w, r, http.StatusNotFound, "NoSuchBucket", b.bucket)
		return
	}
	obj, ok := b.lookup(key)
	if !ok {
		writeS3Error(w, r, http.StatusNotFound, "NoSuchKey", key)
		return
	}

	// ARMOR's metadata maps already carry x-amz-meta- in their keys and the
	// SDK serializer adds ONE MORE on the wire, so a stored object's headers
	// are doubled (x-amz-meta-x-amz-meta-armor-...); the HEAD/GET deserializer
	// strips exactly one, restoring the canonical key. Emit the wire form.
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Last-Modified", obj.lastModified.UTC().Format(http.TimeFormat))
	for k, v := range obj.meta {
		h.Set("x-amz-meta-"+k, v)
	}

	switch r.Method {
	case http.MethodHead:
		h.Set("Content-Length", strconv.Itoa(len(obj.data)))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		b.serveGet(w, r, obj)
	default:
		writeS3Error(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", r.Method)
	}
}

// serveGet answers a GetObject: the whole body without a Range header, else
// the requested single byte range as 206 (clamped at EOF the way real S3
// behaves). Metadata headers were already set by serve.
func (b *fakeS3Bucket) serveGet(w http.ResponseWriter, r *http.Request, obj fakeS3Object) {
	h := w.Header()
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		h.Set("Content-Length", strconv.Itoa(len(obj.data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(obj.data)
		return
	}

	start, end, ok := parseByteRange(rangeHeader, int64(len(obj.data)))
	if !ok {
		writeS3Error(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", rangeHeader)
		return
	}
	body := obj.data[start : end+1]
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(obj.data)))
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(body)
}

// parseByteRange parses the single-range "bytes=a-b" form fetchRange issues
// and clamps end to the last byte of the object.
func parseByteRange(value string, size int64) (start, end int64, ok bool) {
	spec := strings.TrimPrefix(value, "bytes=")
	if spec == value {
		return 0, 0, false
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || end < start {
		return 0, 0, false
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}

// serveList answers ListObjectsV2 with every stored key (the real endpoint
// returns the .armor/ internal namespace too; the backend's List filters it).
func (b *fakeS3Bucket) serveList(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	var body bytes.Buffer
	body.WriteString(`<ListBucketResult>`)
	fmt.Fprintf(&body, "<Name>%s</Name>", b.bucket)
	fmt.Fprintf(&body, "<Prefix>%s</Prefix>", prefix)
	body.WriteString(`<IsTruncated>false</IsTruncated>`)
	var matched []string
	for _, key := range b.keys() {
		if strings.HasPrefix(key, prefix) {
			matched = append(matched, key)
		}
	}
	fmt.Fprintf(&body, "<KeyCount>%d</KeyCount>", len(matched))
	fmt.Fprintf(&body, "<MaxKeys>%s</MaxKeys>", r.URL.Query().Get("max-keys"))
	for _, key := range matched {
		obj, _ := b.lookup(key)
		var keyXML bytes.Buffer
		_ = xml.EscapeText(&keyXML, []byte(key))
		body.WriteString("<Contents>")
		fmt.Fprintf(&body, "<Key>%s</Key>", keyXML.String())
		fmt.Fprintf(&body, "<LastModified>%s</LastModified>", obj.lastModified.UTC().Format("2006-01-02T15:04:05.000Z"))
		fmt.Fprintf(&body, "<Size>%d</Size>", len(obj.data))
		body.WriteString("<StorageClass>STANDARD</StorageClass>")
		body.WriteString("</Contents>")
	}
	body.WriteString(`</ListBucketResult>`)

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body.Bytes())
}

func writeS3Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, message)
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, body)
	}
}

// --- subprocess harness -----------------------------------------------------

// verifyBinaryRun captures one invocation of the built binary.
type verifyBinaryRun struct {
	stdout   string
	stderr   string
	exitCode int
}

// buildContractBinary compiles cmd/armor once per test into a t.TempDir that
// the test's cleanup removes; the shared build cache makes every build after
// the first in a session a fast relink.
func buildContractBinary(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(thisFile) {
		t.Fatalf("cannot locate the cmd/armor package directory from %v", thisFile)
	}
	pkgDir := filepath.Dir(thisFile)

	goTool, err := exec.LookPath("go")
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(runtime.GOROOT(), "bin", "go")); statErr == nil { //nolint:staticcheck // fallback for a toolchain unavailable through PATH
			goTool = filepath.Join(runtime.GOROOT(), "bin", "go") //nolint:staticcheck // fallback for a toolchain unavailable through PATH
		} else {
			t.Fatalf("no go toolchain in PATH for the binary build: %v", err)
		}
	}

	bin := filepath.Join(t.TempDir(), "armor")
	cmd := exec.Command(goTool, "build", "-o", bin, ".")
	cmd.Dir = pkgDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build armor binary: %v\n%s", err, out)
	}
	return bin
}

// verifyContractEnv builds the subprocess environment: ambient ARMOR_* and
// AWS_* variables are stripped so the binary sees exactly the storage config
// the test passes, never whatever the invoking shell had.
func verifyContractEnv(overrides map[string]string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "ARMOR_") || strings.HasPrefix(name, "AWS_") {
			continue
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

// runVerifyBinary runs one `armor verify ...` invocation. A failure to start
// (as opposed to a non-zero exit) is fatal: the contract is about exit codes,
// and a binary that could not run has no verdict to assert on.
func runVerifyBinary(t *testing.T, bin string, env map[string]string, args ...string) verifyBinaryRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = verifyContractEnv(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// Snapshot the buffers AFTER Run: a pre-Run String() would race the child
	// and usually read empty.
	run := verifyBinaryRun{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("armor verify %v did not run: %v\nstderr:\n%s", args, err, stderr.String())
		}
		run.exitCode = exitErr.ExitCode()
	}
	return run
}

// b2EnvFor points the binary's backend at the fake endpoint.
func b2EnvFor(serverURL string) map[string]string {
	return map[string]string{
		"ARMOR_B2_REGION":            "us-east-1",
		"ARMOR_B2_ENDPOINT":          serverURL,
		"ARMOR_B2_ACCESS_KEY_ID":     "binary-contract",
		"ARMOR_B2_SECRET_ACCESS_KEY": "binary-contract",
	}
}

// parseVerifyReport decodes the JSON report the run wrote to stdout.
func parseVerifyReport(t *testing.T, run verifyBinaryRun) *VerificationReport {
	t.Helper()
	var report VerificationReport
	if err := json.Unmarshal([]byte(run.stdout), &report); err != nil {
		t.Fatalf("stdout is not a verification report: %v\nstdout:\n%s\nstderr:\n%s", err, run.stdout, run.stderr)
	}
	return &report
}

// reportRow returns the single result row for key, failing the test when the
// report carries none (or more than one).
func reportRow(t *testing.T, report *VerificationReport, key string) ObjectVerificationResult {
	t.Helper()
	var found []ObjectVerificationResult
	for _, row := range report.Results {
		if row.Key == key {
			found = append(found, row)
		}
	}
	if len(found) != 1 {
		t.Fatalf("report has %d rows for key %q (want exactly 1): %+v", len(found), key, report.Results)
	}
	return found[0]
}

// --- fixtures ---------------------------------------------------------------

// distinctMEK returns a deterministic 32-byte key that differs per index, so a
// test can hold an active key, a retired ring key and an unknown key apart.
func distinctMEK(index byte) []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)*3 + index
	}
	return mek
}

// buildSinglePUTObject encrypts plaintext as a v2 single-PUT object whose
// wrapped DEK is fingerprinted (v2:<fp16>:<base64> — the format the server
// writes today), returning the stored bytes and metadata.
func buildSinglePUTObject(t *testing.T, wrapMEK []byte, plaintext []byte) (data []byte, meta map[string]string) {
	t.Helper()
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i ^ 0x55)
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}
	wrappedDEK, err := crypto.WrapDEKWithFingerprint(wrapMEK, dek)
	if err != nil {
		t.Fatalf("WrapDEKWithFingerprint: %v", err)
	}
	blockSize := 65536
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version2)
	if err != nil {
		t.Fatalf("NewEnvelopeHeaderWithVersion: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("header encode: %v", err)
	}
	enc, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	encrypted, hmacTable, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	data = append(data, headerBytes...)
	data = append(data, encrypted...)
	data = append(data, hmacTable...)
	meta = map[string]string{
		"x-amz-meta-armor-version":          "2",
		"x-amz-meta-armor-block-size":       fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-wrapped-dek":      wrappedDEK,
		"x-amz-meta-armor-plaintext-sha256": hex.EncodeToString(plaintextSHA[:]),
	}
	return data, meta
}

// corruptCiphertextByte flips one byte just inside the encrypted region of a
// single-PUT object: the envelope still parses, the body fails its HMAC.
func corruptCiphertextByte(data []byte) {
	data[crypto.HeaderSize+10] ^= 0xFF
}

// corruptEnvelopeByte flips the magic byte, so no envelope can be decoded.
func corruptEnvelopeByte(data []byte) {
	data[0] ^= 0xFF
}

// writeKeysFile writes a one-key-per-line keys file and returns its path.
func writeKeysFile(t *testing.T, keys ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "keys.txt")
	if err := os.WriteFile(path, []byte(strings.Join(keys, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("write keys file: %v", err)
	}
	return path
}

// --- the contract suite -----------------------------------------------------

// TestVerifyBinaryContract builds the armor binary and drives it against the
// fake endpoint. Every documented behavior gets its own subtest so a single
// `go test ./cmd/armor -run TestVerifyBinaryContract/<name>` re-checks it.
func TestVerifyBinaryContract(t *testing.T) {
	bin := buildContractBinary(t)

	t.Run("mixed_inventory_report_and_exit", func(t *testing.T) {
		bucket, server := newFakeS3Bucket(t)
		active := distinctMEK(1)

		okData, okMeta := buildSinglePUTObject(t, active, bytes.Repeat([]byte("armored payload; "), 128))
		bucket.put("a-ok.bin", okData, okMeta)

		badData, badMeta := buildSinglePUTObject(t, active, bytes.Repeat([]byte("damaged body; "), 128))
		corruptCiphertextByte(badData)
		bucket.put("b-corrupt.bin", badData, badMeta)

		bucket.put("c-not-armor.bin", bytes.Repeat([]byte("plain bytes, never armored"), 90), nil)

		keys := writeKeysFile(t, "a-ok.bin", "b-corrupt.bin", "c-not-armor.bin", "d-missing.bin")

		run := runVerifyBinary(t, bin, b2EnvFor(server.URL),
			"verify", "-bucket", bucket.bucket, "-mek", hex.EncodeToString(active), "-keys-file", keys)

		if run.exitCode != 1 {
			t.Errorf("exit code = %d, want 1 (any CORRUPTED or ERROR row must fail the run)\nstderr:\n%s", run.exitCode, run.stderr)
		}
		report := parseVerifyReport(t, run)

		// Output cardinality: exactly one row per selected object, sorted by
		// key, no row dropped or duplicated even when failures interleave.
		if report.TotalObjects != 4 || len(report.Results) != 4 {
			t.Errorf("report counts: total=%d rows=%d, want 4/4\nreport: %+v", report.TotalObjects, len(report.Results), report)
		}
		seen := map[string]bool{}
		for i, row := range report.Results {
			if seen[row.Key] {
				t.Errorf("duplicate report row for %q", row.Key)
			}
			seen[row.Key] = true
			if i > 0 && report.Results[i-1].Key >= row.Key {
				t.Errorf("rows not sorted by key at index %d: %q >= %q", i, report.Results[i-1].Key, row.Key)
			}
			if row.Status != "OK" && row.Error == "" {
				t.Errorf("failing row %q carries an empty error field", row.Key)
			}
		}
		if report.OKCount != 1 || report.CorruptedCount != 1 || report.ErrorCount != 2 {
			t.Errorf("counts = OK:%d CORRUPTED:%d ERROR:%d, want 1/1/2", report.OKCount, report.CorruptedCount, report.ErrorCount)
		}
		if report.Bucket != bucket.bucket || report.QuickMode {
			t.Errorf("report header mismatch: bucket=%q quick=%v", report.Bucket, report.QuickMode)
		}

		if row := reportRow(t, report, "a-ok.bin"); row.Status != "OK" {
			t.Errorf("a-ok.bin = %s (%s), want OK", row.Status, row.Error)
		}
		if row := reportRow(t, report, "b-corrupt.bin"); row.Status != "CORRUPTED" {
			t.Errorf("b-corrupt.bin = %s (%s), want CORRUPTED for the HMAC failure", row.Status, row.Error)
		}
		if row := reportRow(t, report, "c-not-armor.bin"); row.Status != "ERROR" || !strings.Contains(row.Error, "not ARMOR encrypted") {
			t.Errorf("c-not-armor.bin = %s (%s), want ERROR naming a non-ARMOR object", row.Status, row.Error)
		}
		if row := reportRow(t, report, "d-missing.bin"); row.Status != "ERROR" || !strings.Contains(row.Error, "Failed to get object metadata") {
			t.Errorf("d-missing.bin = %s (%s), want ERROR for the absent object", row.Status, row.Error)
		}

		// Every failure line reaches stderr with its status and key.
		for _, want := range []string{"[CORRUPTED] b-corrupt.bin", "[ERROR] c-not-armor.bin", "[ERROR] d-missing.bin"} {
			if !strings.Contains(run.stderr, want) {
				t.Errorf("stderr missing failure line %q\nstderr:\n%s", want, run.stderr)
			}
		}
	})

	t.Run("all_ok_exits_zero_report_to_output_file", func(t *testing.T) {
		bucket, server := newFakeS3Bucket(t)
		active := distinctMEK(1)

		singleData, singleMeta := buildSinglePUTObject(t, active, bytes.Repeat([]byte("healthy single-put object."), 90))
		bucket.put("single/one.bin", singleData, singleMeta)

		multipart := createV2MultipartFixture(t, active, "multi/two.bin", bytes.Repeat([]byte("multipart block body."), 120))
		bucket.putFixtureV2(t, multipart)

		keys := writeKeysFile(t, "single/one.bin", "multi/two.bin")
		outPath := filepath.Join(t.TempDir(), "report.json")

		run := runVerifyBinary(t, bin, b2EnvFor(server.URL),
			"verify", "-bucket", bucket.bucket, "-mek", hex.EncodeToString(active), "-keys-file", keys, "-output", outPath)

		if run.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0 for an all-OK inventory\nstderr:\n%s", run.exitCode, run.stderr)
		}
		if strings.TrimSpace(run.stdout) != "" {
			t.Errorf("stdout = %q, want empty when -output carries the report", run.stdout)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read -output report: %v", err)
		}
		var report VerificationReport
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatalf("-output file is not a verification report: %v\n%s", err, data)
		}
		if report.OKCount != 2 || report.CorruptedCount != 0 || report.ErrorCount != 0 || len(report.Results) != 2 {
			t.Errorf("report = OK:%d CORRUPTED:%d ERROR:%d rows:%d, want 2 OK rows",
				report.OKCount, report.CorruptedCount, report.ErrorCount, len(report.Results))
		}
		row := reportRow(t, &report, "multi/two.bin")
		if row.Status != "OK" || !strings.Contains(row.Details, "multipart") {
			t.Errorf("multipart row = %s (%s / %s), want OK from the multipart walk", row.Status, row.Error, row.Details)
		}
	})

	t.Run("v3_multipart_manifest_fallback", func(t *testing.T) {
		bucket, server := newFakeS3Bucket(t)
		active := distinctMEK(1)
		// Three parts at the fixture's 1024-byte part size, digest declared:
		// the full per-part HMAC walk and the combined digest check both run.
		healthy := createV3MultipartFixture(t, active, "v3/backup.bin", bytes.Repeat([]byte("v3 multipart segment."), 125), true)
		bucket.putFixture(t, healthy, false) // head carries no metadata; manifest body is the only source

		keys := writeKeysFile(t, "v3/backup.bin")
		run := runVerifyBinary(t, bin, b2EnvFor(server.URL),
			"verify", "-bucket", bucket.bucket, "-mek", hex.EncodeToString(active), "-keys-file", keys)

		if run.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0 for a healthy v3 multipart object\nstderr:\n%s", run.exitCode, run.stderr)
		}
		report := parseVerifyReport(t, run)
		if row := reportRow(t, report, "v3/backup.bin"); row.Status != "OK" {
			t.Errorf("v3/backup.bin = %s (%s), want OK via manifest metadata + gzip sidecar", row.Status, row.Error)
		}

		// The same object with one byte flipped inside the LAST part: the
		// per-part walk must catch corruption beyond part 1.
		damaged := createV3MultipartFixture(t, active, "v3/backup.bin", bytes.Repeat([]byte("v3 multipart segment."), 125), true)
		damaged.corruptPartCiphertext(t, 2)
		bucket.putFixture(t, damaged, false)

		run = runVerifyBinary(t, bin, b2EnvFor(server.URL),
			"verify", "-bucket", bucket.bucket, "-mek", hex.EncodeToString(active), "-keys-file", keys)
		if run.exitCode != 1 {
			t.Errorf("exit code = %d, want 1 for the damaged part\nstderr:\n%s", run.exitCode, run.stderr)
		}
		report = parseVerifyReport(t, run)
		row := reportRow(t, report, "v3/backup.bin")
		if row.Status != "CORRUPTED" || !strings.Contains(row.Error, "v3 multipart verification failed") {
			t.Errorf("damaged v3 row = %s (%s), want CORRUPTED from the per-part walk", row.Status, row.Error)
		}
	})

	t.Run("escrow_ring_key_fallback", func(t *testing.T) {
		bucket, server := newFakeS3Bucket(t)
		active := distinctMEK(1)
		retired := distinctMEK(2)
		neverKnown := distinctMEK(3)

		ringData, ringMeta := buildSinglePUTObject(t, retired, bytes.Repeat([]byte("written under a ring key."), 80))
		bucket.put("ring-object.bin", ringData, ringMeta)
		orphanData, orphanMeta := buildSinglePUTObject(t, neverKnown, bytes.Repeat([]byte("names an unknown fingerprint."), 80))
		bucket.put("orphan-object.bin", orphanData, orphanMeta)

		// Escrow carries the active MEK plus the retired ring key; B2
		// credentials ride in the same file (the escrow is self-contained).
		type ringEntry struct {
			MEK         string `json:"mek"`
			Fingerprint string `json:"fingerprint"`
		}
		escrow := map[string]any{
			"mek": hex.EncodeToString(active),
			"mek_ring": []ringEntry{{
				MEK:         hex.EncodeToString(retired),
				Fingerprint: crypto.MEKFingerprint(retired),
			}},
			"b2": map[string]string{
				"region":     "us-east-1",
				"endpoint":   server.URL,
				"access_key": "binary-contract",
				"secret_key": "binary-contract",
			},
		}
		escrowPath := filepath.Join(t.TempDir(), "escrow.json")
		escrowJSON, err := json.Marshal(escrow)
		if err != nil {
			t.Fatalf("marshal escrow: %v", err)
		}
		if err := os.WriteFile(escrowPath, escrowJSON, 0600); err != nil {
			t.Fatalf("write escrow: %v", err)
		}

		keys := writeKeysFile(t, "ring-object.bin", "orphan-object.bin")
		// No ARMOR_B2_* env and no -mek: everything comes from the escrow.
		run := runVerifyBinary(t, bin, nil,
			"verify", "-bucket", bucket.bucket, "-escrow", escrowPath, "-keys-file", keys)

		if run.exitCode != 1 {
			t.Errorf("exit code = %d, want 1 (the orphan keying gap is an ERROR)\nstderr:\n%s", run.exitCode, run.stderr)
		}
		report := parseVerifyReport(t, run)
		if row := reportRow(t, report, "ring-object.bin"); row.Status != "OK" {
			t.Errorf("ring-object.bin = %s (%s), want OK unwrapped by the escrow ring key", row.Status, row.Error)
		}
		row := reportRow(t, report, "orphan-object.bin")
		if row.Status != "ERROR" {
			t.Errorf("orphan-object.bin = %s (%s), want ERROR for the unknown fingerprint", row.Status, row.Error)
		}
		if !strings.Contains(row.Details, "-escrow") {
			t.Errorf("orphan row details = %q, want the supply-it-via--escrow guidance", row.Details)
		}
	})

	t.Run("quick_mode_flags_and_statuses", func(t *testing.T) {
		bucket, server := newFakeS3Bucket(t)
		active := distinctMEK(1)

		plain := []byte("quick mode checks the envelope, not the body.")
		goodData, goodMeta := buildSinglePUTObject(t, active, bytes.Repeat(plain, 30))
		bucket.put("quick-ok.bin", goodData, goodMeta)

		// Body damage that quick mode by design does not look at.
		bodyDamaged, bodyMeta := buildSinglePUTObject(t, active, bytes.Repeat(plain, 30))
		corruptCiphertextByte(bodyDamaged)
		bucket.put("quick-body-corrupt.bin", bodyDamaged, bodyMeta)

		// Envelope damage quick mode MUST catch.
		brokenData, brokenMeta := buildSinglePUTObject(t, active, bytes.Repeat(plain, 30))
		corruptEnvelopeByte(brokenData)
		bucket.put("quick-envelope-corrupt.bin", brokenData, brokenMeta)

		// Multipart object: quick mode checks DEK + sidecar accounting only.
		multipart := createV3MultipartFixture(t, active, "quick-multipart.bin", bytes.Repeat([]byte("multipart quick body."), 60), true)
		bucket.putFixture(t, multipart, false)

		keys := writeKeysFile(t, "quick-ok.bin", "quick-body-corrupt.bin", "quick-envelope-corrupt.bin", "quick-multipart.bin")

		run := runVerifyBinary(t, bin, b2EnvFor(server.URL),
			"verify", "-bucket", bucket.bucket, "-mek", hex.EncodeToString(active), "-keys-file", keys, "-quick")

		if run.exitCode != 1 {
			t.Errorf("exit code = %d, want 1 for the broken envelope\nstderr:\n%s", run.exitCode, run.stderr)
		}
		report := parseVerifyReport(t, run)
		if !report.QuickMode {
			t.Errorf("report quick_mode = false, want true")
		}
		if len(report.Results) != 4 {
			t.Errorf("rows = %d, want one per selected object", len(report.Results))
		}
		if row := reportRow(t, report, "quick-ok.bin"); row.Status != "OK" {
			t.Errorf("quick-ok.bin = %s (%s), want OK", row.Status, row.Error)
		}
		if row := reportRow(t, report, "quick-body-corrupt.bin"); row.Status != "OK" {
			t.Errorf("quick-body-corrupt.bin = %s (%s), want OK — quick mode skips bodies by design", row.Status, row.Error)
		}
		if row := reportRow(t, report, "quick-envelope-corrupt.bin"); row.Status != "CORRUPTED" {
			t.Errorf("quick-envelope-corrupt.bin = %s (%s), want CORRUPTED", row.Status, row.Error)
		}
		if row := reportRow(t, report, "quick-multipart.bin"); row.Status != "OK" || !strings.Contains(row.Details, "Multipart") {
			t.Errorf("quick-multipart.bin = %s (%s / %s), want OK from the multipart quick walk", row.Status, row.Error, row.Details)
		}
	})

	t.Run("bucket_scan_skips_internal_and_manifest_keys", func(t *testing.T) {
		bucket, server := newFakeS3Bucket(t)
		active := distinctMEK(1)

		singleData, singleMeta := buildSinglePUTObject(t, active, bytes.Repeat([]byte("scanned object."), 90))
		bucket.put("scan/data.bin", singleData, singleMeta)
		multipart := createV3MultipartFixture(t, active, "scan/large.bin", bytes.Repeat([]byte("scanned multipart object."), 70), true)
		bucket.putFixture(t, multipart, false)
		// putFixture stored scan/large.bin.armor-manifest and the internal
		// .armor/hmac/<hash> sidecar; a bucket-wide scan must verify only the
		// two data objects.

		run := runVerifyBinary(t, bin, b2EnvFor(server.URL),
			"verify", "-bucket", bucket.bucket, "-mek", hex.EncodeToString(active))

		if run.exitCode != 0 {
			t.Fatalf("exit code = %d, want 0\nstderr:\n%s", run.exitCode, run.stderr)
		}
		report := parseVerifyReport(t, run)
		if report.TotalObjects != 2 || len(report.Results) != 2 {
			t.Fatalf("scan selected total=%d rows=%d, want exactly the 2 data objects\nreport: %+v", report.TotalObjects, len(report.Results), report)
		}
		gotKeys := map[string]bool{}
		for _, row := range report.Results {
			gotKeys[row.Key] = true
		}
		for _, want := range []string{"scan/data.bin", "scan/large.bin"} {
			if !gotKeys[want] {
				t.Errorf("scan result missing data object %q; rows: %v", want, report.Results)
			}
		}
		if gotKeys["scan/large.bin.armor-manifest"] {
			t.Errorf("the ADR-016 manifest sidecar was verified as an object")
		}
		if !strings.Contains(run.stderr, "Skipping 1 ARMOR manifest sidecar objects") {
			t.Errorf("stderr missing the manifest-skip notice\nstderr:\n%s", run.stderr)
		}
	})

	t.Run("usage_and_input_exit_codes", func(t *testing.T) {
		// Missing -bucket: usage error, exit 2.
		run := runVerifyBinary(t, bin, nil, "verify")
		if run.exitCode != 2 || !strings.Contains(run.stderr, "-bucket is required") {
			t.Errorf("no -bucket: exit=%d stderr=%q, want exit 2 naming the missing flag", run.exitCode, run.stderr)
		}

		// Positional argument: usage error, exit 2.
		run = runVerifyBinary(t, bin, nil, "verify", "-bucket", "b", "stray-argument")
		if run.exitCode != 2 || !strings.Contains(run.stderr, "unexpected arguments") {
			t.Errorf("stray argument: exit=%d stderr=%q, want exit 2 rejecting it", run.exitCode, run.stderr)
		}

		// No key source at all (env scrubbed of ARMOR_*/AWS_*): exit 1.
		run = runVerifyBinary(t, bin, nil, "verify", "-bucket", "b")
		if run.exitCode != 1 || !strings.Contains(run.stderr, "no MEK provided") {
			t.Errorf("no MEK: exit=%d stderr=%q, want exit 1 with the key-source guidance", run.exitCode, run.stderr)
		}

		// An empty selection is a failure, not a silent success: exit 1.
		empty := writeKeysFile(t)
		run = runVerifyBinary(t, bin, map[string]string{
			"ARMOR_B2_REGION":            "us-east-1",
			"ARMOR_B2_ENDPOINT":          "http://127.0.0.1:1", // never dialed: the empty check fires first
			"ARMOR_B2_ACCESS_KEY_ID":     "binary-contract",
			"ARMOR_B2_SECRET_ACCESS_KEY": "binary-contract",
		}, "verify", "-bucket", "b", "-mek", hex.EncodeToString(distinctMEK(1)), "-keys-file", empty)
		if run.exitCode != 1 || !strings.Contains(run.stderr, "No keys to verify") {
			t.Errorf("empty keys file: exit=%d stderr=%q, want exit 1 with the empty-selection notice", run.exitCode, run.stderr)
		}
	})
}
