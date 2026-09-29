package performance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/server/srvtest"
)

// Remote targets. Every one is strictly opt-in via environment variables and
// none runs in CI. Credentials travel by environment reference only and are
// never written to results. A production run needs all three targets:
//
//	ARMOR_PERF_ENDPOINT   authenticated ARMOR S3 edge (upload + plaintext reads)
//	ARMOR_PERF_B2_ENDPOINT direct B2 S3 endpoint (same encrypted object)
//	ARMOR_PERF_CF_BASE    Cloudflare/B2 public host, with optional /file/<bucket>
//	ARMOR_PERF_BUCKET     bucket to use on all targets
//	ARMOR_PERF_REGION     SigV4 region (default us-east-005)
//	AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
//	                      ARMOR credentials; retained for CLI/SigV4 convention
//	ARMOR_PERF_B2_ACCESS_KEY_ID / ARMOR_PERF_B2_SECRET_ACCESS_KEY
//	                      optional B2 credentials; fall back to AWS_* when absent
//	ARMOR_PERF_B2_KEY_PREFIX optional physical B2 key prefix, including slash
//	ARMOR_PERF_SIZES / ARMOR_PERF_UPLOAD_SAMPLES / ARMOR_PERF_READ_SAMPLES
//	ARMOR_PERF_RANGE_SAMPLES / ARMOR_PERF_OUT
//
// ARMOR_PERF_CF_BASE is normally just https://cf.example.com. For compatibility
// with the older runbook it may also be https://cf.example.com/file/<bucket>.
// Cloudflare and direct B2 reads return ciphertext; ARMOR reads return the
// decrypted plaintext. The harness verifies both against the correct payload.
func TestRemoteBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("remote baseline is opt-in measurement work")
	}

	cfg, err := loadRemoteConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.endpoint == "" && cfg.b2Endpoint == "" && cfg.cfBase == "" {
		t.Skip("set ARMOR_PERF_ENDPOINT / ARMOR_PERF_B2_ENDPOINT / ARMOR_PERF_CF_BASE to run remote measurements")
	}
	if cfg.endpoint == "" {
		t.Fatal("production benchmark requires ARMOR_PERF_ENDPOINT for authenticated upload and plaintext verification")
	}
	if cfg.cfBase != "" && cfg.b2Endpoint == "" {
		t.Fatal("Cloudflare measurements require ARMOR_PERF_B2_ENDPOINT to verify the same ciphertext at the B2 origin")
	}
	if cfg.b2Endpoint != "" && (cfg.b2Credentials.access == "" || cfg.b2Credentials.secret == "") {
		t.Fatal("ARMOR_PERF_B2_ENDPOINT requires ARMOR_PERF_B2_ACCESS_KEY_ID / ARMOR_PERF_B2_SECRET_ACCESS_KEY, or AWS_* fallback")
	}
	if cfg.credentials.access == "" || cfg.credentials.secret == "" {
		t.Fatal("ARMOR_PERF_ENDPOINT requires AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY")
	}

	c := &http.Client{Timeout: 30 * time.Minute}
	runID := time.Now().UTC().Format("20060102T150405.000000000Z")
	var uploadedKeys []string
	defer func() {
		for _, key := range uploadedKeys {
			remoteDelete(c, cfg.endpoint, cfg.bucket, key, cfg.credentials)
		}
	}()

	var results []remoteScenarioResult
	for _, sizeString := range cfg.sizes {
		size, err := ParseSize(sizeString)
		if err != nil {
			t.Fatal(err)
		}
		if size < 40<<20 {
			t.Fatalf("remote large-object benchmark requires at least 40MiB, got %s", sizeString)
		}
		body := SyntheticBody(int(size))
		plaintextSHA := hashHex(body)
		keys := make([]string, 0, cfg.uploadSamples)
		uploadResults := make([]remoteScenarioResult, 0, cfg.uploadSamples)

		for sample := 0; sample < cfg.uploadSamples; sample++ {
			key := fmt.Sprintf("%sproduction/%s/%s-%d-%d", BenchPrefix, runID, strings.ToLower(sizeString), size, sample)
			keys = append(keys, key)
			uploadedKeys = append(uploadedKeys, key)

			result, err := measureRemotePut(c, cfg.endpoint, cfg.bucket, key, body, cfg.credentials)
			if err != nil {
				t.Fatalf("ARMOR upload %s sample %d: %v", sizeString, sample+1, err)
			}
			uploadResults = append(uploadResults, result)

			// Every acknowledged upload gets an untimed, ordinary ARMOR GET
			// verification. This is deliberately outside the upload sample's
			// duration so upload numbers do not include verification work.
			if err := verifyRemoteFullGet(c, cfg.endpoint, cfg.bucket, key, plaintextSHA, size, cfg.credentials); err != nil {
				t.Fatalf("ARMOR upload verification %s sample %d: %v", sizeString, sample+1, err)
			}
		}
		results = append(results, aggregateRemoteResults(
			fmt.Sprintf("upload-single-put/armor-service/%s", remoteFormatBytes(size)),
			"armor-service", "PUT", size, size, uploadResults,
		))

		key := keys[len(keys)-1]
		full, err := measureRemoteFullGet(c, "armor-service", cfg.endpoint, cfg.bucket, key, plaintextSHA, size, cfg.readSamples, cfg.credentials, false, nil)
		if err != nil {
			t.Fatalf("ARMOR full GET %s: %v", sizeString, err)
		}
		results = append(results, full)
		for _, spec := range remoteRangeSpecs(size) {
			result, err := measureRemoteRange(c, "armor-service", cfg.endpoint, cfg.bucket, key, size, spec, body, cfg.rangeSamples, cfg.credentials, false)
			if err != nil {
				t.Fatalf("ARMOR range %s/%s: %v", sizeString, spec.name, err)
			}
			results = append(results, result)
		}

		// A direct B2 read supplies the actual encrypted bytes for this object.
		// That makes the public Cloudflare rows content-verified without ever
		// assuming that a public CDN response is plaintext.
		if cfg.b2Endpoint != "" {
			physicalKey := cfg.physicalKey(key)
			raw, err := fetchRemoteBody(c, cfg.b2Endpoint, cfg.bucket, physicalKey, cfg.b2Credentials, "b2-origin", "full")
			if err != nil {
				t.Fatalf("B2 ciphertext reference %s: %v", sizeString, err)
			}
			rawSize := int64(len(raw))
			rawSHA := hashHex(raw)

			result, err := measureRemoteFullGet(c, "b2-origin", cfg.b2Endpoint, cfg.bucket, physicalKey, rawSHA, rawSize, cfg.readSamples, cfg.b2Credentials, true, raw)
			if err != nil {
				t.Fatalf("B2 full GET %s: %v", sizeString, err)
			}
			results = append(results, result)
			for _, spec := range remoteRangeSpecs(rawSize) {
				result, err := measureRemoteRange(c, "b2-origin", cfg.b2Endpoint, cfg.bucket, physicalKey, rawSize, spec, raw, cfg.rangeSamples, cfg.b2Credentials, true)
				if err != nil {
					t.Fatalf("B2 range %s/%s: %v", sizeString, spec.name, err)
				}
				results = append(results, result)
			}

			if cfg.cfBase != "" {
				result, err := measureRemoteFullGet(c, "cloudflare", cfg.cfBase, cfg.bucket, physicalKey, rawSHA, rawSize, cfg.readSamples, remoteCredentials{}, true, raw)
				if err != nil {
					t.Fatalf("Cloudflare full GET %s: %v", sizeString, err)
				}
				results = append(results, result)
				for _, spec := range remoteRangeSpecs(rawSize) {
					result, err := measureRemoteRange(c, "cloudflare", cfg.cfBase, cfg.bucket, physicalKey, rawSize, spec, raw, cfg.rangeSamples, remoteCredentials{}, true)
					if err != nil {
						t.Fatalf("Cloudflare range %s/%s: %v", sizeString, spec.name, err)
					}
					results = append(results, result)
				}
			}
		}
	}

	if err := writeRemoteReport(cfg, results); err != nil {
		t.Fatal(err)
	}
	t.Logf("remote results written to %s/remote-results.json", cfg.outDir)
}

type remoteCredentials struct {
	access string
	secret string
}

type remoteConfig struct {
	endpoint      string
	b2Endpoint    string
	cfBase        string
	bucket        string
	region        string
	sizes         []string
	uploadSamples int
	readSamples   int
	rangeSamples  int
	outDir        string
	credentials   remoteCredentials
	b2Credentials remoteCredentials
	b2KeyPrefix   string
}

type remoteRangeSpec struct {
	name          string
	header        string
	start         int64
	responseBytes int64
}

type remoteScenarioResult struct {
	Name          string         `json:"name"`
	Target        string         `json:"target"`
	Operation     string         `json:"operation"`
	Range         string         `json:"range,omitempty"`
	ObjectBytes   int64          `json:"object_bytes"`
	ResponseBytes int64          `json:"response_bytes"`
	Samples       int            `json:"samples"`
	LatencyMsP50  float64        `json:"latency_ms_p50"`
	LatencyMsP95  float64        `json:"latency_ms_p95"`
	TTFBMsP50     float64        `json:"ttfb_ms_p50"`
	TTFBMsP95     float64        `json:"ttfb_ms_p95"`
	MBpsP50       float64        `json:"mbps_p50"`
	MBpsP95       float64        `json:"mbps_p95"`
	AggregateMBps float64        `json:"aggregate_mbps"`
	StatusCounts  map[string]int `json:"status_counts"`
	CFCacheStatus map[string]int `json:"cf_cache_status,omitempty"`
	Verified      bool           `json:"verified"`
	VerifyNote    string         `json:"verify_note,omitempty"`
}

func loadRemoteConfig() (remoteConfig, error) {
	readInt := func(name string, fallback int) (int, error) {
		value := os.Getenv(name)
		if value == "" {
			return fallback, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return 0, fmt.Errorf("%s must be a positive integer", name)
		}
		return n, nil
	}
	uploadSamples, err := readInt("ARMOR_PERF_UPLOAD_SAMPLES", 3)
	if err != nil {
		return remoteConfig{}, err
	}
	readSamples, err := readInt("ARMOR_PERF_READ_SAMPLES", 5)
	if err != nil {
		return remoteConfig{}, err
	}
	rangeSamples, err := readInt("ARMOR_PERF_RANGE_SAMPLES", 3)
	if err != nil {
		return remoteConfig{}, err
	}

	sizes := []string{"64MiB"}
	if value := os.Getenv("ARMOR_PERF_SIZES"); value != "" {
		sizes = strings.Split(value, ",")
	}
	for i := range sizes {
		sizes[i] = strings.TrimSpace(sizes[i])
		if sizes[i] == "" {
			return remoteConfig{}, fmt.Errorf("ARMOR_PERF_SIZES contains an empty size")
		}
	}

	armorCredentials := remoteCredentials{
		access: firstNonEmpty(os.Getenv("ARMOR_PERF_ACCESS_KEY_ID"), os.Getenv("AWS_ACCESS_KEY_ID")),
		secret: firstNonEmpty(os.Getenv("ARMOR_PERF_SECRET_ACCESS_KEY"), os.Getenv("AWS_SECRET_ACCESS_KEY")),
	}
	b2Credentials := remoteCredentials{
		access: firstNonEmpty(os.Getenv("ARMOR_PERF_B2_ACCESS_KEY_ID"), os.Getenv("AWS_ACCESS_KEY_ID")),
		secret: firstNonEmpty(os.Getenv("ARMOR_PERF_B2_SECRET_ACCESS_KEY"), os.Getenv("AWS_SECRET_ACCESS_KEY")),
	}
	outDir := os.Getenv("ARMOR_PERF_OUT")
	if outDir == "" {
		outDir = os.TempDir() + "/armor-perf-results"
	}
	return remoteConfig{
		endpoint:      strings.TrimSuffix(os.Getenv("ARMOR_PERF_ENDPOINT"), "/"),
		b2Endpoint:    strings.TrimSuffix(firstNonEmpty(os.Getenv("ARMOR_PERF_B2_ENDPOINT"), os.Getenv("ARMOR_PERF_DIRECT_S3")), "/"),
		cfBase:        strings.TrimSuffix(os.Getenv("ARMOR_PERF_CF_BASE"), "/"),
		bucket:        remoteBucket(),
		region:        remoteRegion(),
		sizes:         sizes,
		uploadSamples: uploadSamples,
		readSamples:   readSamples,
		rangeSamples:  rangeSamples,
		outDir:        outDir,
		credentials:   armorCredentials,
		b2Credentials: b2Credentials,
		b2KeyPrefix:   os.Getenv("ARMOR_PERF_B2_KEY_PREFIX"),
	}, nil
}

func (c remoteConfig) physicalKey(key string) string {
	prefix := strings.Trim(c.b2KeyPrefix, "/")
	if prefix == "" {
		return key
	}
	return prefix + "/" + strings.TrimPrefix(key, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func remoteRangeSpecs(objectSize int64) []remoteRangeSpec {
	block := int64(64 << 10)
	return []remoteRangeSpec{
		{name: "aligned-64KiB", header: fmt.Sprintf("bytes=%d-%d", 32<<20, 32<<20+block-1), start: 32 << 20, responseBytes: block},
		{name: "unaligned-32KiB", header: fmt.Sprintf("bytes=%d-%d", 32<<20+1234, 32<<20+1234+(32<<10)-1), start: 32<<20 + 1234, responseBytes: 32 << 10},
		{name: "aligned-1MiB", header: fmt.Sprintf("bytes=%d-%d", 16<<20, 16<<20+(1<<20)-1), start: 16 << 20, responseBytes: 1 << 20},
		{name: "aligned-8MiB", header: fmt.Sprintf("bytes=%d-%d", 24<<20, 24<<20+(8<<20)-1), start: 24 << 20, responseBytes: 8 << 20},
		{name: "suffix-64KiB", header: "bytes=-65536", start: objectSize - block, responseBytes: block},
	}
}

func TestRemoteRangeSpecs(t *testing.T) {
	specs := remoteRangeSpecs(64 << 20)
	if len(specs) != 5 {
		t.Fatalf("got %d range shapes, want 5", len(specs))
	}
	if specs[0].header != "bytes=33554432-33619967" || specs[0].responseBytes != 64<<10 {
		t.Fatalf("aligned range = %+v", specs[0])
	}
	last := specs[len(specs)-1]
	if last.header != "bytes=-65536" || last.start != (64<<20)-(64<<10) {
		t.Fatalf("suffix range = %+v", last)
	}
}

func TestRemoteCFURLAndPhysicalKey(t *testing.T) {
	for _, test := range []struct {
		base string
		want string
	}{
		{"https://cf.example", "https://cf.example/file/bucket/key"},
		{"https://cf.example/file", "https://cf.example/file/bucket/key"},
		{"https://cf.example/file/bucket", "https://cf.example/file/bucket/key"},
	} {
		if got := remoteCFURL(test.base, "bucket", "key"); got != test.want {
			t.Errorf("remoteCFURL(%q) = %q, want %q", test.base, got, test.want)
		}
	}
	cfg := remoteConfig{b2KeyPrefix: "tenant/"}
	if got := cfg.physicalKey("armor-bench/key"); got != "tenant/armor-bench/key" {
		t.Fatalf("physicalKey = %q", got)
	}
}

func measureRemotePut(c *http.Client, baseURL, bucket, key string, body []byte, credentials remoteCredentials) (remoteScenarioResult, error) {
	start := time.Now()
	req, err := http.NewRequest(http.MethodPut, remoteS3URL(baseURL, bucket, key), bytes.NewReader(body))
	if err != nil {
		return remoteScenarioResult{}, err
	}
	srvtest.SignS3Request(req, body, credentials.access, credentials.secret, remoteRegion())
	resp, err := c.Do(req)
	if err != nil {
		return remoteScenarioResult{}, err
	}
	status := strconv.Itoa(resp.StatusCode)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	duration := time.Since(start)
	if resp.StatusCode != http.StatusOK {
		return remoteScenarioResult{}, fmt.Errorf("PUT status %d", resp.StatusCode)
	}
	return remoteScenarioResult{
		Name:          "upload-single-put/armor-service/" + remoteFormatBytes(int64(len(body))),
		Target:        "armor-service",
		Operation:     "PUT",
		ObjectBytes:   int64(len(body)),
		ResponseBytes: int64(len(body)),
		Samples:       1,
		LatencyMsP50:  fms(duration),
		LatencyMsP95:  fms(duration),
		TTFBMsP50:     fms(duration),
		TTFBMsP95:     fms(duration),
		MBpsP50:       mBps(int64(len(body)), duration),
		MBpsP95:       mBps(int64(len(body)), duration),
		AggregateMBps: mBps(int64(len(body)), duration),
		StatusCounts:  map[string]int{status: 1},
		Verified:      true,
		VerifyNote:    "PUT acknowledged; a separate ordinary ARMOR GET verified the payload",
	}, nil
}

func aggregateRemoteResults(name, target, operation string, objectBytes, responseBytes int64, samples []remoteScenarioResult) remoteScenarioResult {
	latencies := make([]float64, 0, len(samples))
	ttfs := make([]float64, 0, len(samples))
	throughputs := make([]float64, 0, len(samples))
	statuses := map[string]int{}
	cacheStatuses := map[string]int{}
	verified := true
	for _, sample := range samples {
		latencies = append(latencies, sample.LatencyMsP50)
		ttfs = append(ttfs, sample.TTFBMsP50)
		throughputs = append(throughputs, sample.MBpsP50)
		mergeCounts(statuses, sample.StatusCounts)
		mergeCounts(cacheStatuses, sample.CFCacheStatus)
		verified = verified && sample.Verified
	}
	return remoteScenarioResult{
		Name:          name,
		Target:        target,
		Operation:     operation,
		ObjectBytes:   objectBytes,
		ResponseBytes: responseBytes,
		Samples:       len(samples),
		LatencyMsP50:  percentile(latencies, 50),
		LatencyMsP95:  percentile(latencies, 95),
		TTFBMsP50:     percentile(ttfs, 50),
		TTFBMsP95:     percentile(ttfs, 95),
		MBpsP50:       percentile(throughputs, 50),
		MBpsP95:       percentile(throughputs, 95),
		AggregateMBps: mBps(responseBytes*int64(len(samples)), sumDur(latencies)),
		StatusCounts:  statuses,
		CFCacheStatus: cacheStatuses,
		Verified:      verified,
		VerifyNote:    samples[0].VerifyNote,
	}
}

func measureRemoteFullGet(c *http.Client, target, baseURL, bucket, key, expectedSHA string, objectBytes int64, samples int, credentials remoteCredentials, raw bool, expectedBody []byte) (remoteScenarioResult, error) {
	latencies := make([]float64, 0, samples)
	ttfs := make([]float64, 0, samples)
	throughputs := make([]float64, 0, samples)
	statuses := map[string]int{}
	cacheStatuses := map[string]int{}
	verified := true
	for i := 0; i < samples; i++ {
		result, err := measureRemoteGET(c, target, baseURL, bucket, key, "", objectBytes, credentials, expectedSHA, raw, expectedBody)
		if err != nil {
			return remoteScenarioResult{}, err
		}
		latencies = append(latencies, result.LatencyMsP50)
		ttfs = append(ttfs, result.TTFBMsP50)
		throughputs = append(throughputs, result.MBpsP50)
		mergeCounts(statuses, result.StatusCounts)
		mergeCounts(cacheStatuses, result.CFCacheStatus)
		verified = verified && result.Verified
	}
	return remoteScenarioResult{
		Name:          fmt.Sprintf("download-full-warm/%s/%s", target, remoteFormatBytes(objectBytes)),
		Target:        target,
		Operation:     "GET",
		ObjectBytes:   objectBytes,
		ResponseBytes: objectBytes,
		Samples:       samples,
		LatencyMsP50:  percentile(latencies, 50),
		LatencyMsP95:  percentile(latencies, 95),
		TTFBMsP50:     percentile(ttfs, 50),
		TTFBMsP95:     percentile(ttfs, 95),
		MBpsP50:       percentile(throughputs, 50),
		MBpsP95:       percentile(throughputs, 95),
		AggregateMBps: mBps(objectBytes*int64(samples), sumDur(latencies)),
		StatusCounts:  statuses,
		CFCacheStatus: cacheStatuses,
		Verified:      verified,
		VerifyNote:    verificationNote(target, raw),
	}, nil
}

func measureRemoteRange(c *http.Client, target, baseURL, bucket, key string, objectBytes int64, spec remoteRangeSpec, expected []byte, samples int, credentials remoteCredentials, raw bool) (remoteScenarioResult, error) {
	latencies := make([]float64, 0, samples)
	ttfs := make([]float64, 0, samples)
	throughputs := make([]float64, 0, samples)
	statuses := map[string]int{}
	cacheStatuses := map[string]int{}
	verified := true
	for i := 0; i < samples; i++ {
		result, err := measureRemoteGET(c, target, baseURL, bucket, key, spec.header, spec.responseBytes, credentials, "", raw, expected[spec.start:spec.start+spec.responseBytes])
		if err != nil {
			return remoteScenarioResult{}, err
		}
		latencies = append(latencies, result.LatencyMsP50)
		ttfs = append(ttfs, result.TTFBMsP50)
		throughputs = append(throughputs, result.MBpsP50)
		mergeCounts(statuses, result.StatusCounts)
		mergeCounts(cacheStatuses, result.CFCacheStatus)
		verified = verified && result.Verified
	}
	return remoteScenarioResult{
		Name:          fmt.Sprintf("download-range/%s/%s/%s", target, spec.name, remoteFormatBytes(spec.responseBytes)),
		Target:        target,
		Operation:     "GET",
		Range:         spec.header,
		ObjectBytes:   objectBytes,
		ResponseBytes: spec.responseBytes,
		Samples:       samples,
		LatencyMsP50:  percentile(latencies, 50),
		LatencyMsP95:  percentile(latencies, 95),
		TTFBMsP50:     percentile(ttfs, 50),
		TTFBMsP95:     percentile(ttfs, 95),
		MBpsP50:       percentile(throughputs, 50),
		MBpsP95:       percentile(throughputs, 95),
		AggregateMBps: mBps(spec.responseBytes*int64(samples), sumDur(latencies)),
		StatusCounts:  statuses,
		CFCacheStatus: cacheStatuses,
		Verified:      verified,
		VerifyNote:    verificationNote(target, raw) + "; exact requested slice compared",
	}, nil
}

// measureRemoteGET measures one request. For the ARMOR target expectedBody is
// plaintext; for B2/Cloudflare it is the corresponding ciphertext bytes.
func measureRemoteGET(c *http.Client, target, baseURL, bucket, key, rangeHeader string, responseBytes int64, credentials remoteCredentials, expectedSHA string, raw bool, expectedBody []byte) (remoteScenarioResult, error) {
	url := remoteS3URL(baseURL, bucket, key)
	if target == "cloudflare" {
		url = remoteCFURL(baseURL, bucket, key)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return remoteScenarioResult{}, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if credentials.access != "" || credentials.secret != "" {
		srvtest.SignS3Request(req, nil, credentials.access, credentials.secret, remoteRegion())
	}
	start := time.Now()
	resp, err := c.Do(req)
	if err != nil {
		return remoteScenarioResult{}, err
	}
	status := strconv.Itoa(resp.StatusCode)
	cacheStatus := resp.Header.Get("CF-Cache-Status")
	if cacheStatus == "" {
		cacheStatus = "absent"
	}
	var firstByte time.Duration
	h := sha256.New()
	var body bytes.Buffer
	buf := make([]byte, 512<<10)
	var received int64
	for received < responseBytes {
		readBuffer := buf
		remaining := responseBytes - received
		if remaining < int64(len(readBuffer)) {
			readBuffer = readBuffer[:remaining]
		}
		n, readErr := resp.Body.Read(readBuffer)
		if n > 0 {
			if firstByte == 0 {
				firstByte = time.Since(start)
			}
			received += int64(n)
			h.Write(readBuffer[:n])
			if expectedBody != nil {
				body.Write(readBuffer[:n])
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			resp.Body.Close()
			return remoteScenarioResult{}, fmt.Errorf("body read: %w", readErr)
		}
	}
	resp.Body.Close()
	duration := time.Since(start)
	verified := resp.StatusCode == http.StatusOK
	if rangeHeader != "" {
		verified = resp.StatusCode == http.StatusPartialContent
	}
	if received != responseBytes {
		verified = false
	}
	if expectedSHA != "" && hex.EncodeToString(h.Sum(nil)) != expectedSHA {
		verified = false
	}
	if expectedBody != nil && !bytes.Equal(body.Bytes(), expectedBody) {
		verified = false
	}
	return remoteScenarioResult{
		LatencyMsP50:  fms(duration),
		TTFBMsP50:     fms(firstByte),
		MBpsP50:       mBps(received, duration),
		StatusCounts:  map[string]int{status: 1},
		CFCacheStatus: map[string]int{cacheStatus: 1},
		Verified:      verified,
		VerifyNote:    verificationNote(target, raw),
	}, nil
}

func verifyRemoteFullGet(c *http.Client, baseURL, bucket, key, expectedSHA string, size int64, credentials remoteCredentials) error {
	result, err := measureRemoteGET(c, "armor-service", baseURL, bucket, key, "", size, credentials, expectedSHA, false, nil)
	if err != nil {
		return err
	}
	if !result.Verified {
		return fmt.Errorf("response failed SHA-256 or size verification")
	}
	return nil
}

func fetchRemoteBody(c *http.Client, baseURL, bucket, key string, credentials remoteCredentials, target, operation string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, remoteS3URL(baseURL, bucket, key), nil)
	if err != nil {
		return nil, err
	}
	srvtest.SignS3Request(req, nil, credentials.access, credentials.secret, remoteRegion())
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	var body []byte
	var readErr error
	if resp.ContentLength >= 0 {
		body = make([]byte, resp.ContentLength)
		_, readErr = io.ReadFull(resp.Body, body)
	} else {
		body, readErr = io.ReadAll(resp.Body)
	}
	resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("%s %s body: %w", target, operation, readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s status %d", target, operation, resp.StatusCode)
	}
	return body, nil
}

func remoteDelete(c *http.Client, baseURL, bucket, key string, credentials remoteCredentials) {
	req, err := http.NewRequest(http.MethodDelete, remoteS3URL(baseURL, bucket, key), nil)
	if err != nil {
		return
	}
	srvtest.SignS3Request(req, nil, credentials.access, credentials.secret, remoteRegion())
	if resp, err := c.Do(req); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func remoteS3URL(baseURL, bucket, key string) string {
	return strings.TrimSuffix(baseURL, "/") + "/" + bucket + "/" + strings.TrimPrefix(key, "/")
}

func remoteCFURL(baseURL, bucket, key string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(baseURL, "/file") {
		return baseURL + "/" + bucket + "/" + strings.TrimPrefix(key, "/")
	}
	if strings.Contains(baseURL, "/file/") {
		return baseURL + "/" + strings.TrimPrefix(key, "/")
	}
	return baseURL + "/file/" + bucket + "/" + strings.TrimPrefix(key, "/")
}

func writeRemoteReport(cfg remoteConfig, results []remoteScenarioResult) error {
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	report := map[string]interface{}{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"armor_commit": vcsRevision(),
		"go_version":   runtime.Version(),
		"environment":  environment(),
		"workload": map[string]interface{}{
			"sizes":          cfg.sizes,
			"upload_samples": cfg.uploadSamples,
			"read_samples":   cfg.readSamples,
			"range_samples":  cfg.rangeSamples,
			"block_size":     64 << 10,
			"bucket":         cfg.bucket,
			"region":         cfg.region,
			"b2_key_prefix":  cfg.b2KeyPrefix,
		},
		"targets":      []string{"armor-service", "b2-origin", "cloudflare"},
		"payload_note": "ARMOR rows verify synthetic plaintext; B2 and Cloudflare rows verify the exact encrypted bytes stored by ARMOR",
		"scenarios":    results,
		"limits": []string{
			"one client from the benchmark runner; no concurrency or sustained-load claim",
			"upload samples are serialized single-PUTs; multipart upload is not included",
			"B2 and Cloudflare download rows are separate raw-ciphertext transport measurements; ARMOR rows include decryption and plaintext verification",
			"Cloudflare cache status is reported per sample; cache state, edge selection, WAN path, and production contention are time-dependent",
		},
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal remote report: %w", err)
	}
	if err := os.WriteFile(cfg.outDir+"/remote-results.json", data, 0o644); err != nil {
		return fmt.Errorf("write remote JSON: %w", err)
	}
	if err := os.WriteFile(cfg.outDir+"/remote-results.md", []byte(renderRemoteMarkdown(cfg, results)), 0o644); err != nil {
		return fmt.Errorf("write remote Markdown: %w", err)
	}
	return nil
}

func renderRemoteMarkdown(cfg remoteConfig, results []remoteScenarioResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# ARMOR production performance — %s\n\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- ARMOR commit: `%s`\n- Environment: %s\n- Sizes: %s\n- Samples: upload=%d full=%d range=%d\n- Payloads: ARMOR plaintext; B2/Cloudflare ciphertext\n\n", vcsRevision(), environment(), strings.Join(cfg.sizes, ", "), cfg.uploadSamples, cfg.readSamples, cfg.rangeSamples)
	b.WriteString("| scenario | target | object B | response B | samples | p50 ms | p95 ms | p50 MB/s | p95 MB/s | TTFB p50 ms | CF cache | verified |\n|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|\n")
	for _, result := range results {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %.2f | %.2f | %.1f | %.1f | %.2f | %s | %t |\n", result.Name, result.Target, result.ObjectBytes, result.ResponseBytes, result.Samples, result.LatencyMsP50, result.LatencyMsP95, result.MBpsP50, result.MBpsP95, result.TTFBMsP50, joinCounts(result.CFCacheStatus), result.Verified)
	}
	b.WriteString("\n- Direct B2 and Cloudflare rows use the physical ciphertext object produced by the ARMOR upload.\n- Every upload was followed by an untimed ordinary ARMOR GET verification.\n")
	return b.String()
}

func verificationNote(target string, raw bool) string {
	if raw {
		return target + " response SHA-256 and byte count verified against the B2 ciphertext reference"
	}
	return target + " response SHA-256 and byte count verified against the uploaded plaintext"
}

func remoteFormatBytes(n int64) string {
	for _, unit := range []struct {
		name  string
		value int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}} {
		if n >= unit.value && n%unit.value == 0 {
			return fmt.Sprintf("%d%s", n/unit.value, unit.name)
		}
	}
	return fmt.Sprintf("%dB", n)
}

func mergeCounts(dst, src map[string]int) {
	for key, value := range src {
		dst[key] += value
	}
}

func remoteBucket() string {
	if b := os.Getenv("ARMOR_PERF_BUCKET"); b != "" {
		return b
	}
	return "armor-perf-bench"
}

func remoteRegion() string {
	if r := os.Getenv("ARMOR_PERF_REGION"); r != "" {
		return r
	}
	return srvtest.TestRegion
}

func joinCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ",")
}
