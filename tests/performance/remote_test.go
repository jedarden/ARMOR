package performance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/server/srvtest"
)

// Remote targets. Every one is strictly opt-in via environment variables and
// none runs in CI. Credentials travel by environment reference only and are
// never written to results — results carry endpoint labels, not secrets or
// unpublished object identifiers.
//
//	ARMOR_PERF_ENDPOINT   https://armor.example:2443  (real ARMOR service; S3, SigV4)
//	ARMOR_PERF_BUCKET     bucket to use on remote targets
//	ARMOR_PERF_CF_BASE    https://cf.example.com/file/<bucket>  (Cloudflare read path; no creds)
//	ARMOR_PERF_DIRECT_S3  https://s3.us-west-002.backblazeb2.com (labeled direct-backend comparison; billed egress)
//	ARMOR_PERF_REGION     SigV4 region (default us-east-005)
//	AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY                       (service + direct-S3 creds)
//	ARMOR_PERF_SIZES / ARMOR_PERF_READ_SAMPLES / ARMOR_PERF_OUT     (shared knobs)
//
// The direct-backend target exists ONLY as an explicitly labeled comparison:
// it reads raw objects over the backend's own S3 API and is billed egress
// (ADR-013). CF comparisons read the SAME ciphertext object the service
// uploaded, which is what makes service-vs-CF like-for-like; therefore
// ARMOR_PERF_CF_BASE requires ARMOR_PERF_ENDPOINT to be set as well.
func TestRemoteBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("remote baseline is opt-in measurement work")
	}

	endpoint := strings.TrimSuffix(os.Getenv("ARMOR_PERF_ENDPOINT"), "/")
	cfBase := strings.TrimSuffix(os.Getenv("ARMOR_PERF_CF_BASE"), "/")
	directS3 := strings.TrimSuffix(os.Getenv("ARMOR_PERF_DIRECT_S3"), "/")
	if endpoint == "" && cfBase == "" && directS3 == "" {
		t.Skip("set ARMOR_PERF_ENDPOINT / ARMOR_PERF_CF_BASE / ARMOR_PERF_DIRECT_S3 to run remote measurements")
	}
	if cfBase != "" && endpoint == "" {
		t.Fatal("ARMOR_PERF_CF_BASE requires ARMOR_PERF_ENDPOINT: CF comparisons must read the ciphertext object the service uploaded")
	}
	bucket := remoteBucket()

	sizes := []string{"64KiB", "1MiB", "64MiB"}
	if v := os.Getenv("ARMOR_PERF_SIZES"); v != "" {
		sizes = strings.Split(v, ",")
	}
	samples := 5
	if v := os.Getenv("ARMOR_PERF_READ_SAMPLES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			samples = n
		}
	}

	c := &http.Client{Timeout: 30 * time.Minute}
	var results []ScenarioResult
	var uploadedKeys []string // cleanup is scoped to exactly these

	for _, sizeStr := range sizes {
		size := MustParseSize(sizeStr)
		key := BenchPrefix + fmt.Sprintf("remote-%d", size)

		var wantSHA string
		if endpoint != "" {
			body := SyntheticBody(int(size))
			wantSHA = hashHex(body)
			if err := remotePut(c, endpoint, bucket, key, body); err != nil {
				t.Errorf("armor-service upload %s: %v", sizeStr, err)
				continue
			}
			uploadedKeys = append(uploadedKeys, key)

			// service read path
			r, err := measureRemoteFullGet(c, "armor-service", endpoint, bucket, key, wantSHA, size, samples)
			if err != nil {
				t.Errorf("armor-service read %s: %v", sizeStr, err)
			} else {
				results = append(results, *r)
			}
			// CF read path over the SAME ciphertext object
			if cfBase != "" {
				r, err := measureRemoteFullGet(c, "cloudflare-read", cfBase, bucket, key, wantSHA, size, samples)
				if err != nil {
					t.Errorf("cloudflare read %s: %v", sizeStr, err)
				} else {
					results = append(results, *r)
				}
			}
		}

		// Explicitly labeled direct-backend comparison: uploads its own raw
		// object and reads it over the backend's S3 API. Billed egress.
		if directS3 != "" {
			body := SyntheticBody(int(size))
			directKey := key + ".direct-comparison"
			if err := remotePut(c, directS3, bucket, directKey, body); err != nil {
				t.Errorf("direct-backend upload %s: %v", sizeStr, err)
				continue
			}
			uploadedKeys = append(uploadedKeys, directKey)
			r, err := measureRemoteFullGet(c, "direct-backend-egress-billed", directS3, bucket, directKey, hashHex(body), size, samples)
			if err != nil {
				t.Errorf("direct-backend read %s: %v", sizeStr, err)
			} else {
				results = append(results, *r)
			}
		}
	}

	// cleanup: exactly the objects this run uploaded
	for _, key := range uploadedKeys {
		if endpoint != "" {
			remoteDelete(c, endpoint, bucket, key)
		}
		if directS3 != "" && strings.HasSuffix(key, ".direct-comparison") {
			remoteDelete(c, directS3, bucket, key)
		}
	}

	outDir := os.Getenv("ARMOR_PERF_OUT")
	if outDir == "" {
		outDir = os.TempDir() + "/armor-perf-results"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("out dir: %v", err)
	}
	j, _ := json.MarshalIndent(map[string]interface{}{
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"armor_commit":  vcsRevision(),
		"environment":   environment(),
		"object_format": "v3 ciphertext via service/CF; raw object via direct-backend comparison",
		"cf_cache_note": "cloudflare-read scenarios record CF-Cache-Status per sample; HIT/MISS/EXPIRED/DYNAMIC are distinguished and reported",
		"scenarios":     results,
	}, "", "  ")
	if err := os.WriteFile(outDir+"/remote-results.json", j, 0o644); err != nil {
		t.Fatalf("write results: %v", err)
	}
	t.Logf("remote results (%d scenarios) written to %s/remote-results.json", len(results), outDir)
}

// measureRemoteFullGet takes n full-GET samples of one already-present object,
// hash-verifying each against the payload that was uploaded.
func measureRemoteFullGet(c *http.Client, name, baseURL, bucket, key, wantSHA string, size int64, samples int) (*ScenarioResult, error) {
	var ttfs, durs, mbps []float64
	cfStatuses := map[string]int{}
	var firstSHA string
	verified := true

	for i := 0; i < samples; i++ {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/"+bucket+"/"+key, nil)
		if err != nil {
			return nil, err
		}
		start := time.Now()
		resp, err := c.Do(req)
		if err != nil {
			return nil, fmt.Errorf("get: %w", err)
		}
		if st := resp.Header.Get("CF-Cache-Status"); st != "" {
			cfStatuses[st]++
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("get: status %d", resp.StatusCode)
		}
		h := sha256.New()
		buf := make([]byte, 512<<10)
		var n int64
		first := true
		for {
			k, rerr := resp.Body.Read(buf)
			if k > 0 {
				if first {
					ttfs = append(ttfs, fms(time.Since(start)))
					first = false
				}
				n += int64(k)
				h.Write(buf[:k])
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("body: %w", rerr)
			}
		}
		dur := time.Since(start)
		resp.Body.Close()

		sha := hex.EncodeToString(h.Sum(nil))
		if wantSHA != "" && sha != wantSHA {
			verified = false
		}
		if firstSHA == "" {
			firstSHA = sha
		} else if sha != firstSHA {
			verified = false
		}
		if n != size {
			verified = false
		}
		durs = append(durs, fms(dur))
		mbps = append(mbps, mBps(n, dur))
	}

	r := &ScenarioResult{
		Name:              fmt.Sprintf("read-full-warm/%s/%dB", name, size),
		ObjectBytes:       size,
		Samples:           samples,
		ClientConcurrency: 1,
		TTFBMsP50:         percentile(ttfs, 50),
		TTFBMsP95:         percentile(ttfs, 95),
		CompletionMsP50:   percentile(durs, 50),
		CompletionMsP95:   percentile(durs, 95),
		MBpsP50:           percentile(mbps, 50),
		MBpsP95:           percentile(mbps, 95),
		AggregateMBps:     mBps(size*int64(samples), sumDur(durs)),
		Verified:          verified,
		VerifyNote:        "every sample SHA-256-verified against the uploaded payload; size checked",
	}
	if len(cfStatuses) > 0 {
		r.VerifyNote += "; CF-Cache-Status " + joinCounts(cfStatuses)
	}
	return r, nil
}

// remotePut signs (if creds are configured) and performs one PUT. The ARMOR
// service requires SigV4; the direct-backend S3 API does too.
func remotePut(c *http.Client, baseURL, bucket, key string, body []byte) error {
	access := os.Getenv("AWS_ACCESS_KEY_ID")
	secret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if access == "" || secret == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY not set (credentials travel by environment reference only)")
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+"/"+bucket+"/"+key, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	srvtest.SignS3Request(req, body, access, secret, remoteRegion())
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func remoteDelete(c *http.Client, baseURL, bucket, key string) {
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/"+bucket+"/"+key, nil)
	if err != nil {
		return
	}
	access := os.Getenv("AWS_ACCESS_KEY_ID")
	secret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if access != "" && secret != "" {
		srvtest.SignS3Request(req, nil, access, secret, remoteRegion())
	}
	if resp, err := c.Do(req); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
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
