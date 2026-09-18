package performance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server"
	"github.com/jedarden/armor/internal/server/srvtest"
)

// BenchPrefix is the dedicated object-key prefix every benchmark object lives
// under, locally and remotely. Cleanup is scoped to this prefix — never to the
// bucket as a whole.
const BenchPrefix = "armor-bench/"

// benchBucket is the local harness bucket. Remote runs use the bucket named by
// ARMOR_PERF_BUCKET instead.
const benchBucket = "armor-perf-bench"

// localCredentials are throwaway SigV4 inputs for the in-process server — the
// same class of local-only constants srvtest uses. They are not credentials to
// anything.
var localCredentials = struct {
	access string
	secret string
}{
	access: srvtest.TestAccessKey,
	secret: srvtest.TestSecretKey,
}

// LocalEnv is one in-process ARMOR service: the real authenticated S3 mux
// (SigV4, aws-chunked, real handlers) over an instrumented filesystem backend,
// served on a loopback httptest listener so clients measure the full HTTP
// path, not an in-memory shortcut.
type LocalEnv struct {
	Name     string // scenario family label, e.g. "local-v3" / "local-v3-compressed"
	BaseURL  string
	Bucket   string
	Backend  *CountingBackend
	Compress bool

	// scenario bookkeeping used by the matrix runner while it measures.
	scenarioName        string
	scenarioObjectBytes int64
	scenarioSamples     int
	scenarioConc        int
	peakTempDisk        int64
	results             []ScenarioResult

	srv    *httptest.Server
	fsDir  string
	client *http.Client
}

// ParseSize parses "64KiB", "1MiB", "64MiB", "1GiB" (also plain bytes).
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	units := []struct {
		suffix string
		mult   int64
	}{
		{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30},
		{"KB", 1 << 10}, {"MB", 1 << 20}, {"GB", 1 << 30},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			var n float64
			if _, err := fmt.Sscanf(strings.TrimSuffix(s, u.suffix), "%g", &n); err != nil {
				return 0, fmt.Errorf("perf: bad size %q", s)
			}
			return int64(n * float64(u.mult)), nil
		}
	}
	var n float64
	if _, err := fmt.Sscanf(s, "%g", &n); err != nil {
		return 0, fmt.Errorf("perf: bad size %q", s)
	}
	return int64(n), nil
}

// MustParseSize is ParseSize for internal defaults that are known-good.
func MustParseSize(s string) int64 {
	n, err := ParseSize(s)
	if err != nil {
		panic(err)
	}
	return n
}

// NewLocalEnv boots a local ARMOR service over a temp filesystem backend.
// compress=true selects the ADR-007 single-PUT compression path (ranges and
// multipart unsupported there — the harness only runs single-PUT scenarios
// against it).
func NewLocalEnv(name string, compress bool) (*LocalEnv, error) {
	fsDir, err := os.MkdirTemp("", "armor-perf-fs-")
	if err != nil {
		return nil, fmt.Errorf("perf: temp backend dir: %w", err)
	}

	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: fsDir})
	if err != nil {
		os.RemoveAll(fsDir)
		return nil, fmt.Errorf("perf: fs backend: %w", err)
	}
	counting := NewCountingBackend(fsBackend)

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		os.RemoveAll(fsDir)
		return nil, fmt.Errorf("perf: mek: %w", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024,
		MEK:       mek,
		B2Region:  srvtest.TestRegion,
		Credentials: map[string]*config.Credential{
			localCredentials.access: {AccessKey: localCredentials.access, SecretKey: localCredentials.secret},
		},
		CacheMaxEntries: 1000,
		CacheTTL:        300,
		Compress:        compress,
	}
	srv, err := server.NewWithBackend(cfg, counting)
	if err != nil {
		os.RemoveAll(fsDir)
		return nil, fmt.Errorf("perf: server: %w", err)
	}

	httptestSrv := httptest.NewServer(srv.Handler())

	env := &LocalEnv{
		Name:     name,
		BaseURL:  httptestSrv.URL,
		Bucket:   benchBucket,
		Backend:  counting,
		Compress: compress,
		srv:      httptestSrv,
		fsDir:    fsDir,
		client: &http.Client{
			Timeout: 30 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        64,
				MaxIdleConnsPerHost: 64,
			},
		},
	}
	// CreateBucket over the real S3 path so the bucket exists for clients.
	rec, err := env.do(http.MethodPut, "/"+env.Bucket, nil, nil)
	if err != nil {
		httptestSrv.Close()
		os.RemoveAll(fsDir)
		return nil, fmt.Errorf("perf: create bucket: %w", err)
	}
	rec.Body.Close()
	if rec.StatusCode != http.StatusOK && rec.StatusCode != http.StatusConflict {
		httptestSrv.Close()
		os.RemoveAll(fsDir)
		return nil, fmt.Errorf("perf: create bucket: status %d", rec.StatusCode)
	}
	return env, nil
}

// Close stops the listener and removes the backend's temp directory — the
// harness's cleanup is exactly its own artifacts.
func (e *LocalEnv) Close() {
	e.srv.Close()
	e.client.CloseIdleConnections()
	os.RemoveAll(e.fsDir)
}

// TempDir exposes the backend root so the matrix can measure temporary-disk
// usage.
func (e *LocalEnv) TempDir() string { return e.fsDir }

// signedReq builds and signs one request against this env.
func (e *LocalEnv) signedReq(method, path, query string, body []byte, hdr map[string]string) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, e.BaseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	if query != "" {
		req.URL.RawQuery = query
	}
	req.Host = e.srv.Listener.Addr().String()
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	srvtest.SignS3Request(req, body, localCredentials.access, localCredentials.secret, srvtest.TestRegion)
	return req, nil
}

// do performs one signed request and returns the response (caller closes
// body).
func (e *LocalEnv) do(method, path string, body []byte, hdr map[string]string) (*http.Response, error) {
	return e.doQ(method, path, "", body, hdr)
}

func (e *LocalEnv) doQ(method, path, query string, body []byte, hdr map[string]string) (*http.Response, error) {
	req, err := e.signedReq(method, path, query, body, hdr)
	if err != nil {
		return nil, err
	}
	return e.client.Do(req)
}

// PutResult carries what a write reported.
type PutResult struct {
	PlaintextSHA string
	Duration     time.Duration
	Bytes        int64
}

// PutObject performs one single-PUT write of body through the service and
// returns its hash and duration.
func (e *LocalEnv) PutObject(key string, body []byte) (PutResult, error) {
	start := time.Now()
	rec, err := e.do(http.MethodPut, "/"+e.Bucket+"/"+key, body, nil)
	if err != nil {
		return PutResult{}, err
	}
	errBody, _ := io.ReadAll(rec.Body)
	rec.Body.Close()
	if rec.StatusCode != http.StatusOK {
		return PutResult{}, fmt.Errorf("put %s: status %d: %s", key, rec.StatusCode, truncate(string(errBody)))
	}
	return PutResult{
		PlaintextSHA: hashHex(body),
		Duration:     time.Since(start),
		Bytes:        int64(len(body)),
	}, nil
}

// GetSample is one measured read.
type GetSample struct {
	TTFB      time.Duration // time to first body byte
	Duration  time.Duration
	Bytes     int64         // payload bytes read
	Received  []byte        // populated only when capture is requested
	SHA256    string        // hash of the payload actually received
	TotalReqs int           // client-side HTTP requests consumed (redirects etc. always 1 here)
}

// GetObject performs one full GET through the service, capturing TTFB,
// duration, received bytes and their SHA-256. When capture is true the payload
// is also returned for byte-exact comparison.
func (e *LocalEnv) GetObject(key string, capture bool) (GetSample, error) {
	return e.getWithRange(key, "", capture)
}

// GetRange performs one ranged GET (raw Range header value, e.g.
// "bytes=65536-131071" or "bytes=-65536").
func (e *LocalEnv) GetRange(key, rangeHeader string, capture bool) (GetSample, error) {
	return e.getWithRange(key, rangeHeader, capture)
}

func (e *LocalEnv) getWithRange(key, rangeHeader string, capture bool) (GetSample, error) {
	hdr := map[string]string(nil)
	if rangeHeader != "" {
		hdr = map[string]string{"Range": rangeHeader}
	}
	req, err := e.signedReq(http.MethodGet, "/"+e.Bucket+"/"+key, "", nil, hdr)
	if err != nil {
		return GetSample{}, err
	}
	start := time.Now()
	resp, err := e.client.Do(req)
	if err != nil {
		return GetSample{}, err
	}
	defer resp.Body.Close()

	if rangeHeader == "" && resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return GetSample{}, fmt.Errorf("get %s: status %d", key, resp.StatusCode)
	}
	if rangeHeader != "" && resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return GetSample{}, fmt.Errorf("range get %s (%s): status %d", key, rangeHeader, resp.StatusCode)
	}

	sample := GetSample{}
	h := sha256.New()
	buf := make([]byte, 512*1024)
	first := true
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if first {
				sample.TTFB = time.Since(start)
				first = false
			}
			sample.Bytes += int64(n)
			h.Write(buf[:n])
			if capture {
				sample.Received = append(sample.Received, buf[:n]...)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return GetSample{}, fmt.Errorf("get %s body: %w", key, rerr)
		}
	}
	sample.Duration = time.Since(start)
	sample.SHA256 = hex.EncodeToString(h.Sum(nil))
	return sample, nil
}

// MultipartPut performs one multipart upload as a single client stream:
// create, one sequential UploadPart per part, complete. Parts within one
// upload are deliberately sequential — on committed main (verified
// 2026-09-18) concurrent part uploads against one upload ID can be refused
// with 503, so the harness models write concurrency at the CLIENT level
// (multiple simultaneous uploads), which is what the baseline scenarios
// measure. Returns the aggregate bytes and duration.
func (e *LocalEnv) MultipartPut(key string, parts [][]byte) (PutResult, error) {
	start := time.Now()

	rec, err := e.do(http.MethodPost, "/"+e.Bucket+"/"+key+"?uploads", nil, nil)
	if err != nil {
		return PutResult{}, err
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	body, _ := io.ReadAll(rec.Body)
	rec.Body.Close()
	if rec.StatusCode != http.StatusOK {
		return PutResult{}, fmt.Errorf("create multipart %s: status %d", key, rec.StatusCode)
	}
	if err := xml.Unmarshal(body, &created); err != nil || created.UploadID == "" {
		return PutResult{}, fmt.Errorf("create multipart %s: parse UploadId: %v", key, err)
	}

	etags := make([]string, len(parts))
	for i, part := range parts {
		prec, perr := e.doQ(http.MethodPut, "/"+e.Bucket+"/"+key,
			fmt.Sprintf("partNumber=%d&uploadId=%s", i+1, created.UploadID), part, nil)
		if perr != nil {
			return PutResult{}, perr
		}
		io.Copy(io.Discard, prec.Body)
		prec.Body.Close()
		if prec.StatusCode != http.StatusOK {
			return PutResult{}, fmt.Errorf("part %d: status %d", i+1, prec.StatusCode)
		}
		etags[i] = strings.Trim(prec.Header.Get("ETag"), `"`)
	}

	var complete bytes.Buffer
	complete.WriteString("<CompleteMultipartUpload>")
	for i := range parts {
		fmt.Fprintf(&complete, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", i+1, etags[i])
	}
	complete.WriteString("</CompleteMultipartUpload>")
	crec, err := e.doQ(http.MethodPost, "/"+e.Bucket+"/"+key, "uploadId="+created.UploadID, complete.Bytes(), nil)
	if err != nil {
		return PutResult{}, err
	}
	io.Copy(io.Discard, crec.Body)
	crec.Body.Close()
	if crec.StatusCode != http.StatusOK {
		return PutResult{}, fmt.Errorf("complete multipart %s: status %d", key, crec.StatusCode)
	}
	return PutResult{
		PlaintextSHA: hashHex(concat(parts)),
		Duration:     time.Since(start),
		Bytes:        int64(totalLen(parts)),
	}, nil
}

// DeletePrefix deletes every object under BenchPrefix in the env's bucket and
// aborts any multipart uploads started under it — the harness's owned
// artifacts, nothing else.
func (e *LocalEnv) DeletePrefix(ctx context.Context) {
	_ = e.deletePrefixErr(ctx)
}

func (e *LocalEnv) deletePrefixErr(ctx context.Context) error {
	for {
		resp, err := e.doQ(http.MethodGet, "/"+e.Bucket, "list-type=2&prefix="+BenchPrefix, nil, nil)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var list struct {
			Contents []struct {
				Key string `xml:"Key"`
			} `xml:"Contents"`
		}
		if err := xml.Unmarshal(body, &list); err != nil {
			return err
		}
		if len(list.Contents) == 0 {
			return nil
		}
		for _, c := range list.Contents {
			dresp, err := e.do(http.MethodDelete, "/"+e.Bucket+"/"+c.Key, nil, nil)
			if err != nil {
				return err
			}
			io.Copy(io.Discard, dresp.Body)
			dresp.Body.Close()
		}
	}
}

// SyntheticBody returns n bytes of deterministic pseudo-random (incompressible)
// data — benchmark payloads are synthetic by policy.
func SyntheticBody(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on Linux
	}
	return b
}

// SyntheticCompressible returns n bytes of deterministic, highly compressible
// data for the compression-path scenarios.
func SyntheticCompressible(n int) []byte {
	b := make([]byte, n)
	pattern := []byte("armor performance baseline — compressible synthetic payload 0123456789\n")
	for i := range b {
		b[i] = pattern[i%len(pattern)]
	}
	return b
}

func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func concat(parts [][]byte) []byte {
	out := make([]byte, 0, totalLen(parts))
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func totalLen(parts [][]byte) (n int64) {
	for _, p := range parts {
		n += int64(len(p))
	}
	return n
}

func truncate(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
