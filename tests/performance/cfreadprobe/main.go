// Command cfreadprobe measures aggregate throughput of the Cloudflare read
// path that ARMOR's B2 backend uses for pipelined object reads
// (internal/backend/b2.go fetchRange + the ADR-013 read pipeline).
//
// It exists to answer one question (armor-409733e3): do all pipelined ranged
// fetches multiplex onto a single HTTP/2 connection and cap the aggregate,
// and does forcing one HTTP/1.1 TCP connection per worker remove that cap?
//
// The Cloudflare download path (https://<CF domain>/file/<bucket>/<key>) is
// unauthenticated from the client's side — the Cloudflare front injects the
// B2 credential — so the read matrix runs with no credentials at all.
// Listing, uploading (the fixed probe object and the upload-burst arms) and
// cleanup talk to B2's S3 API directly with SigV4, using the same
// bucket-scoped application key the live deployment consumes
// (PROBE_B2_AK/PROBE_B2_SK); through-armor arms sign with the armor auth
// pair (PROBE_ARMOR_AK/PROBE_ARMOR_SK).
//
// Modes:
//
//	list           ListObjectsV2 over the bucket prefix (key + size rows).
//	upload         Upload the fixed probe object (-object-mib).
//	headers        One ranged GET over CF; print the wire headers.
//	single         One connection, whole-object read; reference point.
//	matrix         The {proto} x {concurrency} x {fetch size} read grid,
//	               optionally with a concurrent SigV4 upload burst per arm.
//	cleanup        Delete the probe object and burst objects.
//
// RESULT lines are the record format; one per measured arm:
//
//	RESULT arm=<label> mibps=<f> reqs=<n> mib=<f> secs=<f> errs=<n>
//	       ttfbP50ms=<f> ttfbP90ms=<f> protoSeen=HTTP/2.0:123,HTTP/1.1:0
//	       cache=HIT:12,MISS:4,DYNAMIC:98
//
// protoSeen proves which HTTP version the transport actually negotiated —
// an h1 arm that still reports HTTP/2.0 means the transport override did
// not take.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// A shared, deterministic body pattern: probe uploads are bandwidth, not
// entropy, so bytes cycle through a 64 KiB block instead of costing rand.
var pattern = func() []byte {
	b := make([]byte, 64<<10)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}()

func main() {
	cfBase := flag.String("cf-base", "https://b2-us-west-002.ardenone.com/file/iad-ci", "CF download base: https://<domain>/file/<bucket>")
	s3Endpoint := flag.String("s3-endpoint", "https://s3.us-west-002.backblazeb2.com", "B2 S3 endpoint for SigV4 legs")
	bucket := flag.String("bucket", "iad-ci", "bucket")
	key := flag.String("key", "probe/armor-409733e3/cfread-256mib.bin", "probe object key")
	prefix := flag.String("prefix", "probe/armor-409733e3/", "prefix for list/cleanup")
	armorBase := flag.String("armor-base", "", "armor S3 base URL (http://host:port) for through-armor arms")
	armorRegion := flag.String("armor-region", "us-east-1", "armor SigV4 region")
	mode := flag.String("mode", "headers", "list|upload|headers|single|matrix|cleanup")
	proto := flag.String("proto", "h2", "h2|h1 (comma list for matrix)")
	conc := flag.String("concurrency", "16", "parallel ranged fetch workers (comma list for matrix)")
	fetchMiB := flag.String("fetch-mib", "1", "ranged fetch size MiB (comma list for matrix)")
	objectMiB := flag.Float64("object-mib", 256, "probe object size in MiB (upload/single)")
	duration := flag.Float64("duration", 20, "measured window seconds per arm")
	upburst := flag.Int("upburst", 0, "concurrent upload-burst workers during each matrix arm")
	nocache := flag.Bool("nocache", false, "bust CF cache per request (every fetch is a MISS/origin fetch)")
	keysFlag := flag.String("keys", "", "comma list of keys to round-robin across (multi-file arms)")
	burstMiB := flag.Float64("burst-mib", 8, "upload-burst body size in MiB")
	ak := os.Getenv("PROBE_B2_AK")
	sk := os.Getenv("PROBE_B2_SK")
	armorAK := os.Getenv("PROBE_ARMOR_AK")
	armorSK := os.Getenv("PROBE_ARMOR_SK")
	flag.Parse()

	if *mode == "list" {
		runList(*s3Endpoint, *bucket, *prefix, ak, sk)
		return
	}
	if *mode == "upload" {
		runUpload(*s3Endpoint, *bucket, *key, *objectMiB, ak, sk)
		return
	}
	if *mode == "cleanup" {
		runCleanup(*s3Endpoint, *bucket, *prefix, ak, sk)
		return
	}

	objBytes := int64(*objectMiB * (1 << 20))
	if *mode == "headers" {
		runHeaders(*cfBase, *key, int64(mustFloat(*fetchMiB)*(1<<20)), *proto)
		return
	}
	if *mode == "single" {
		res := runArm("single-"+*proto, *proto, 1, objBytes, objBytes, 0, 300,
			reader{cfBase: *cfBase, key: *key})
		fmt.Println(res.line())
		return
	}
	if *mode != "matrix" {
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}

	rd := reader{cfBase: *cfBase, key: *key, s3Endpoint: *s3Endpoint,
		bucket: *bucket, ak: ak, sk: sk, burstMiB: *burstMiB}
	if *nocache {
		atomic.StoreInt64(&rd.nocache, 1)
	}
	if *keysFlag != "" {
		for _, k := range strings.Split(*keysFlag, ",") {
			rd.keys = append(rd.keys, strings.TrimSpace(k))
		}
	}
	var burstDesc string
	if *upburst > 0 {
		burstDesc = fmt.Sprintf(" upburst=%d", *upburst)
	}
	fmt.Printf("# matrix begin %s bucket=%s object=%s (%.0f MiB) window=%.0fs%s\n",
		time.Now().UTC().Format(time.RFC3339), *bucket, *key, *objectMiB, *duration, burstDesc)

	warm(*cfBase, *key)
	for _, p := range strings.Split(*proto, ",") {
		for _, c := range splitInts(*conc) {
			for _, f := range splitFloats(*fetchMiB) {
				res := runArm(fmt.Sprintf("proto=%s conc=%d fetch=%.0fMiB", p, c, f),
					p, c, objBytes, int64(f*(1<<20)), *upburst, *duration, rd)
				fmt.Println(res.line())
				os.Stdout.Sync()
			}
		}
	}

	if *armorBase != "" && armorAK != "" {
		// Through-armor arms anchor the probe to what production logs
		// measure: SigV4 GETs against the live armor S3 surface, whose
		// backend leg is the CF pipeline under its own configured knobs.
		for _, f := range []float64{1, 8} {
			c := splitInts(*conc)[0]
			res := runArm(fmt.Sprintf("via-armor conc=%d fetch=%.0fMiB", c, f),
				"h2", c, objBytes, int64(f*(1<<20)), 0, *duration,
				reader{armorBase: *armorBase, armorRegion: *armorRegion,
					armorAK: armorAK, armorSK: armorSK,
					bucket: *bucket, key: *key})
			fmt.Println(res.line())
			os.Stdout.Sync()
		}
	}
	fmt.Printf("# matrix end %s\n", time.Now().UTC().Format(time.RFC3339))
}

// ---- transport ----

// newTransport mirrors internal/backend/transport.go: a DefaultTransport
// clone with the idle pool raised. proto h2 keeps ForceAttemptHTTP2 (all
// in-flight requests multiplex onto one connection per host); proto h1
// disables the h2 upgrade so each in-flight request holds its own TCP+TLS
// connection. This is exactly the behavioral difference the ARMOR_CF_READ_H1
// knob introduces in the backend read client.
func newTransport(proto string, idle int) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = idle
	tr.MaxIdleConns = idle
	if proto == "h1" {
		tr.ForceAttemptHTTP2 = false
		tr.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}
	return tr
}

// ---- measurement ----

type reader struct {
	cfBase      string
	key         string // primary key; keys below round-robin when set
	keys        []string
	armorBase   string
	armorRegion string
	armorAK     string
	armorSK     string
	bucket      string
	s3Endpoint  string
	ak, sk      string
	burstMiB    float64
	// nocache appends a unique query string per request so every fetch is a
	// CF MISS (origin fetch from B2), the state most production reads are
	// in; 0 disables busting.
	nocache int64
}

var nocacheSeq int64

// tally aggregates wire observations across an arm's workers.
type tally struct {
	mu     sync.Mutex
	proto  map[string]int
	cache  map[string]int
	status map[string]int
	ttfb   []float64
}

func newTally() *tally {
	return &tally{proto: map[string]int{}, cache: map[string]int{}, status: map[string]int{}}
}

func (t *tally) noteStatus(code int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status[strconv.Itoa(code)]++
}

func (t *tally) note(resp *http.Response, ttfb float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.proto[resp.Proto]++
	if v := resp.Header.Get("CF-Cache-Status"); v != "" {
		t.cache[v]++
	}
	t.ttfb = append(t.ttfb, ttfb)
}

func (t *tally) snapshot() (p50, p90 float64, proto, cache, status map[string]int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	sort.Float64s(t.ttfb)
	pct := func(p float64) float64 {
		if len(t.ttfb) == 0 {
			return 0
		}
		return t.ttfb[int(float64(len(t.ttfb)-1)*p)]
	}
	return pct(0.50), pct(0.90), t.proto, t.cache, t.status
}

type result struct {
	arm       string
	mibPerSec float64
	reqs      int64
	mib       float64
	secs      float64
	errs      int64
	p50, p90  float64
	proto     map[string]int
	cache     map[string]int
	status    map[string]int
}

func (r result) line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "RESULT arm=%s mibps=%.2f reqs=%d mib=%.1f secs=%.1f errs=%d ttfbP50ms=%.0f ttfbP90ms=%.0f",
		r.arm, r.mibPerSec, r.reqs, r.mib, r.secs, r.errs, r.p50, r.p90)
	b.WriteString(" protoSeen=" + counts(r.proto))
	b.WriteString(" cache=" + counts(r.cache))
	b.WriteString(" status=" + counts(r.status))
	return b.String()
}

func counts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+":"+strconv.Itoa(m[k]))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

// runArm runs one measured window: conc workers pulling fetch-size ranged
// chunks off the object in offset order (the pipelined reader's shape),
// optionally with upburst concurrent SigV4 PUT workers generating write
// traffic in parallel.
func runArm(label, proto string, conc int, objBytes, fetchBytes int64, upburst int, durSecs float64, rd reader) result {
	client := &http.Client{Transport: newTransport(proto, conc), Timeout: 90 * time.Second}
	burstClient := &http.Client{Timeout: 120 * time.Second}

	t := newTally()
	var bytesRead, reqs, errs int64
	stop := make(chan struct{})
	var wg sync.WaitGroup

	nChunks := (objBytes + fetchBytes - 1) / fetchBytes
	var chunk int64

	start := time.Now()
	if upburst > 0 && rd.ak != "" {
		for i := 0; i < upburst; i++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				seq := 0
				for {
					select {
					case <-stop:
						return
					default:
					}
					body := make([]byte, int64(rd.burstMiB*(1<<20)))
					for i := range body {
						body[i] = pattern[i%len(pattern)]
					}
					bkey := fmt.Sprintf("probe/armor-409733e3/burst/w%d-%d-%d.bin", w, time.Now().UnixNano(), seq)
					if err := sigPut(burstClient, rd.s3Endpoint, rd.bucket, bkey, body, rd.ak, rd.sk); err != nil {
						atomic.AddInt64(&errs, 1)
					}
					seq++
				}
			}(i)
		}
	}

	for w := 0; w < conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				idx := atomic.AddInt64(&chunk, 1)
				off := ((idx - 1) % nChunks) * fetchBytes
				length := fetchBytes
				if off+length > objBytes {
					length = objBytes - off
				}
				key := rd.key
				if len(rd.keys) > 0 {
					key = rd.keys[int((idx-1)%int64(len(rd.keys)))]
				}
				n, ttfb, resp, err := rangedGet(client, rd, key, off, length)
				if err != nil {
					atomic.AddInt64(&errs, 1)
					if resp != nil {
						t.noteStatus(resp.StatusCode)
					}
					continue
				}
				t.note(resp, ttfb)
				atomic.AddInt64(&bytesRead, n)
				atomic.AddInt64(&reqs, 1)
			}
		}()
	}

	time.Sleep(time.Duration(durSecs * float64(time.Second)))
	close(stop)
	elapsed := time.Since(start).Seconds()
	wg.Wait()

	p50, p90, protoSeen, cacheSeen, statusSeen := t.snapshot()
	mib := float64(atomic.LoadInt64(&bytesRead)) / (1 << 20)
	return result{
		arm: label, mibPerSec: mib / elapsed,
		reqs: atomic.LoadInt64(&reqs), errs: atomic.LoadInt64(&errs),
		mib: mib, secs: elapsed,
		p50: p50, p90: p90,
		proto: protoSeen, cache: cacheSeen, status: statusSeen,
	}
}

// ---- wire ----

// rangedGet performs one ranged GET and returns (bytes read, TTFB ms, the
// response for header tallying, error). The response body is fully drained
// before return so the connection returns to the pool, exactly like
// readRangeBody does in the backend.
func rangedGet(client *http.Client, rd reader, key string, off, length int64) (int64, float64, *http.Response, error) {
	var target string
	var signed bool
	switch {
	case rd.armorBase != "":
		target = fmt.Sprintf("%s/%s/%s", rd.armorBase, rd.bucket, key)
		signed = true
	default:
		target = fmt.Sprintf("%s/%s", rd.cfBase, key)
		if n := atomic.LoadInt64(&rd.nocache); n != 0 {
			target = fmt.Sprintf("%s?probe=%d", target, atomic.AddInt64(&nocacheSeq, n))
		}
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+length-1))
	if signed {
		sign(req, rd.armorAK, rd.armorSK, rd.armorRegion, sha256Empty)
	}
	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, nil, err
	}
	defer resp.Body.Close()
	ttfb := float64(time.Since(t0).Microseconds()) / 1000
	if resp.StatusCode/100 != 2 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return 0, ttfb, resp, fmt.Errorf("status %d", resp.StatusCode)
	}
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return n, ttfb, resp, err
	}
	return n, ttfb, resp, nil
}

// ---- SigV4 (B2 S3 and armor), header auth ----

const sha256Empty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func hmacSha(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sign(req *http.Request, ak, sk, region, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := amzDate[:8]
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	ch := "host;x-amz-content-sha256;x-amz-date"
	canonical := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		canonicalQuery(req.URL.RawQuery),
		"host:" + req.URL.Host + "\nx-amz-content-sha256:" + payloadHash + "\nx-amz-date:" + amzDate + "\n",
		ch,
		payloadHash,
	}, "\n")
	crSum := sha256.Sum256([]byte(canonical))
	scope := strings.Join([]string{date, region, "s3", "aws4_request"}, "/")
	sts := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, scope, hex.EncodeToString(crSum[:]),
	}, "\n")
	k := hmacSha([]byte("AWS4"+sk), []byte(date))
	k = hmacSha(k, []byte(region))
	k = hmacSha(k, []byte("s3"))
	k = hmacSha(k, []byte("aws4_request"))
	sig := hex.EncodeToString(hmacSha(k, []byte(sts)))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		ak, scope, ch, sig))
}

func canonicalQuery(raw string) string {
	if raw == "" {
		return ""
	}
	pairs := strings.Split(raw, "&")
	sort.Strings(pairs)
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		kv := strings.SplitN(p, "=", 2)
		k, _ := url.QueryUnescape(kv[0])
		v := ""
		if len(kv) == 2 {
			v, _ = url.QueryUnescape(kv[1])
		}
		out = append(out, uriEncode(k)+"="+uriEncode(v))
	}
	return strings.Join(out, "&")
}

func uriEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func sigPut(client *http.Client, endpoint, bucket, key string, body []byte, ak, sk string) error {
	sum := sha256.Sum256(body)
	target := fmt.Sprintf("%s/%s/%s", endpoint, bucket, key)
	req, err := http.NewRequest(http.MethodPut, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	sign(req, ak, sk, "us-west-002", hex.EncodeToString(sum[:]))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("PUT %s: status %d", key, resp.StatusCode)
	}
	return nil
}

func sigGetBody(endpoint, bucket, query, ak, sk string) ([]byte, int, error) {
	target := fmt.Sprintf("%s/%s?%s", endpoint, bucket, query)
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	sign(req, ak, sk, "us-west-002", sha256Empty)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

// ---- modes ----

func runList(endpoint, bucket, prefix, ak, sk string) {
	needCreds(ak, sk)
	q := "list-type=2&max-keys=50&prefix=" + uriEncode(prefix)
	body, status, err := sigGetBody(endpoint, bucket, q, ak, sk)
	if err != nil || status != 200 {
		fmt.Fprintf(os.Stderr, "list failed: status=%d err=%v body=%.200s\n", status, err, string(body))
		os.Exit(1)
	}
	var key, size string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "<Key>") {
			if key != "" {
				fmt.Printf("key=%s size=%s\n", key, size)
			}
			key = between(line, "<Key>", "</Key>")
			size = "?"
		}
		if strings.Contains(line, "<Size>") {
			size = between(line, "<Size>", "</Size>")
		}
	}
	if key != "" {
		fmt.Printf("key=%s size=%s\n", key, size)
	}
}

func needCreds(ak, sk string) {
	if ak == "" || sk == "" {
		fmt.Fprintln(os.Stderr, "this mode needs PROBE_B2_AK/PROBE_B2_SK in the environment")
		os.Exit(2)
	}
}

func between(s, a, b string) string {
	i := strings.Index(s, a)
	if i < 0 {
		return ""
	}
	s = s[i+len(a):]
	j := strings.Index(s, b)
	if j < 0 {
		return s
	}
	return s[:j]
}

func runUpload(endpoint, bucket, key string, mib float64, ak, sk string) {
	needCreds(ak, sk)
	body := make([]byte, int64(mib*(1<<20)))
	for i := range body {
		body[i] = pattern[i%len(pattern)]
	}
	t0 := time.Now()
	if err := sigPut(http.DefaultClient, endpoint, bucket, key, body, ak, sk); err != nil {
		fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
		os.Exit(1)
	}
	secs := time.Since(t0).Seconds()
	fmt.Printf("UPLOADED key=%s mib=%.0f secs=%.1f mibps=%.1f\n", key, mib, secs, mib/secs)
}

func runCleanup(endpoint, bucket, prefix, ak, sk string) {
	needCreds(ak, sk)
	q := "list-type=2&max-keys=1000&prefix=" + uriEncode(prefix)
	body, status, err := sigGetBody(endpoint, bucket, q, ak, sk)
	if err != nil || status != 200 {
		fmt.Fprintf(os.Stderr, "cleanup list failed: status=%d err=%v\n", status, err)
		os.Exit(1)
	}
	var keys []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, "<Key>") {
			keys = append(keys, between(line, "<Key>", "</Key>"))
		}
	}
	for _, k := range keys {
		target := fmt.Sprintf("%s/%s/%s", endpoint, bucket, k)
		req, _ := http.NewRequest(http.MethodDelete, target, nil)
		sign(req, ak, sk, "us-west-002", sha256Empty)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("DELETE %s err=%v\n", k, err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		fmt.Printf("DELETE %s status=%d\n", k, resp.StatusCode)
	}
	fmt.Printf("CLEANUP deleted=%d\n", len(keys))
}

func runHeaders(cfBase, key string, fetchBytes int64, proto string) {
	client := &http.Client{Transport: newTransport(proto, 1), Timeout: 120 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, cfBase+"/"+key, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", fetchBytes-1))
	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GET failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, resp.Body)
	fmt.Printf("PROTO=%s STATUS=%d BYTES=%d MS=%.0f\n", resp.Proto, resp.StatusCode, n,
		float64(time.Since(t0).Microseconds())/1000)
	for _, h := range []string{"CF-Cache-Status", "Cache-Control", "Content-Range", "Server", "X-Bz-File-Id", "Age", "Content-Length"} {
		if v := resp.Header.Get(h); v != "" {
			fmt.Printf("%s: %s\n", h, v)
		}
	}
}

func warm(cfBase, key string) {
	client := &http.Client{Transport: newTransport("h2", 1), Timeout: 120 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, cfBase+"/"+key, nil)
	req.Header.Set("Range", "bytes=0-1048575")
	if resp, err := client.Do(req); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func splitInts(csv string) []int {
	var out []int
	for _, s := range strings.Split(csv, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err == nil {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		out = []int{16}
	}
	return out
}

func splitFloats(csv string) []float64 {
	var out []float64
	for _, s := range strings.Split(csv, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err == nil {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		out = []float64{1}
	}
	return out
}

func mustFloat(csv string) float64 {
	vs := splitFloats(csv)
	return vs[0]
}
