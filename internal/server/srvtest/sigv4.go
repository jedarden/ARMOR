package srvtest

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Test credential constants. These are throwaway strings local to the test
// harness — SigV4 HMAC inputs for the in-process server this package builds —
// not credentials to anything.
const (
	TestAccessKey = "armor-srvtest-harness-access-key"  // gitleaks:allow
	TestSecretKey = "armor-srvtest-harness-signing-key" // gitleaks:allow
	TestRegion    = "us-east-005"
)

// SignS3Request adds a header-based AWS SigV4 Authorization entry to req,
// signing host, x-amz-content-sha256, and x-amz-date. The canonical request is
// built exactly the way internal/server's SigV4Auth.buildCanonicalRequest
// rebuilds it (same query escaping, same host handling, payload hash taken
// from x-amz-content-sha256), so requests signed here verify against the
// server's auth middleware.
func SignS3Request(req *http.Request, body []byte, accessKey, secretKey, region string) {
	SignS3RequestWithPayloadHash(req, sha256Hex(body), accessKey, secretKey, region)
}

// SignS3RequestWithPayloadHash signs a request whose x-amz-content-sha256 is
// an explicit value rather than the digest of the body about to be sent. The
// payload-hash gate tests need this to replay a captured signed request whose
// body no longer matches the digest the signature covers — the same thing a
// real replayed request looks like to the server.
func SignS3RequestWithPayloadHash(req *http.Request, payloadHash, accessKey, secretKey, region string) {
	signS3Request(req, payloadHash, accessKey, secretKey, region)
}

// StreamingSigner carries the SigV4 signing context of a request signed for
// STREAMING-AWS4-HMAC-SHA256-PAYLOAD: the request is already authenticated by
// the seed signature on its Authorization header, and EncodeBody wraps
// payloads in an aws-chunked body whose per-chunk signatures chain from that
// seed, exactly the way the AWS SDKs build one.
type StreamingSigner struct {
	seed       string
	signingKey []byte
	amzDate    string
	scope      string
}

// SignS3RequestStreaming signs req as a streaming-chunked upload (the
// x-amz-content-sha256 header is the streaming marker) and returns a signer
// that produces correctly chained chunk bodies for it.
func SignS3RequestStreaming(req *http.Request, accessKey, secretKey, region string) *StreamingSigner {
	seed, amzDate, scope, signingKey := signS3Request(req, streamingPayloadMarker, accessKey, secretKey, region)
	return &StreamingSigner{
		seed:       seed,
		signingKey: signingKey,
		amzDate:    amzDate,
		scope:      scope,
	}
}

const streamingPayloadMarker = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"

// EncodeBody signs payload into the aws-chunked wire format, returning the
// bytes to send as the request body. chunkSize splits the payload; when
// mutate is non-nil it is applied to each chunk's data after that chunk's
// signature is computed over the original bytes, so a test can corrupt the
// body in ways the signatures no longer cover — the same thing a bit-flip in
// transit or a tampering client looks like to the server. mutate must return
// same-length data so the chunk framing stays aligned.
func (s *StreamingSigner) EncodeBody(payload []byte, chunkSize int, mutate func([]byte) []byte) []byte {
	var buf bytes.Buffer
	empty := sha256Hex(nil)
	prev := s.seed
	chain := func(dataHash string) string {
		stringToSign := strings.Join([]string{
			"AWS4-HMAC-SHA256-PAYLOAD",
			s.amzDate,
			s.scope,
			prev,
			empty,
			dataHash,
		}, "\n")
		sig := hex.EncodeToString(hmacSHA256(s.signingKey, stringToSign))
		prev = sig
		return sig
	}
	for off := 0; off < len(payload); off += chunkSize {
		end := off + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		data := payload[off:end]
		sig := chain(sha256Hex(data))
		wire := data
		if mutate != nil {
			wire = mutate(wire)
		}
		fmt.Fprintf(&buf, "%x;chunk-signature=%s\r\n", len(data), sig)
		buf.Write(wire)
		buf.WriteString("\r\n")
	}
	sig := chain(empty)
	fmt.Fprintf(&buf, "0;chunk-signature=%s\r\n\r\n", sig)
	return buf.Bytes()
}

// signS3Request adds header-based AWS SigV4 Authorization to req for host,
// x-amz-content-sha256, and x-amz-date, returning the signature, the amz date
// and credential scope it signed under, and the raw signing key — everything
// a chunk-signature chain needs to extend the seed.
func signS3Request(req *http.Request, payloadHash, accessKey, secretKey, region string) (signature, amzDate, scope string, signingKey []byte) {
	amzDate = time.Now().UTC().Format("20060102T150405Z")
	dateShort := amzDate[:8]

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	sort.Strings(signedHeaders)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQueryString(req),
		canonicalHeaders(req, signedHeaders),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	scope = dateShort + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey = hmacSHA256([]byte("AWS4"+secretKey), dateShort)
	signingKey = hmacSHA256(signingKey, region)
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	signature = hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, strings.Join(signedHeaders, ";"), signature))
	return signature, amzDate, scope, signingKey
}

func canonicalURI(r *http.Request) string {
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	return path
}

// canonicalQueryString mirrors SigV4Auth.buildCanonicalQueryString: sorted
// keys, sorted values, QueryEscape on both sides.
func canonicalQueryString(r *http.Request) string {
	query := r.URL.Query()
	if len(query) == 0 {
		return ""
	}
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

// canonicalHeaders mirrors SigV4Auth.buildCanonicalHeaders: the host header is
// taken from r.Host, other signed headers from their (space-collapsed) values.
func canonicalHeaders(r *http.Request, signedHeaders []string) string {
	var lines []string
	for _, h := range signedHeaders {
		if h == "host" {
			lines = append(lines, "host:"+strings.TrimSpace(r.Host))
			continue
		}
		values := r.Header.Values(h)
		if len(values) == 0 {
			continue
		}
		trimmed := make([]string, len(values))
		for i, v := range values {
			trimmed[i] = strings.Join(strings.Fields(v), " ")
		}
		lines = append(lines, h+":"+strings.Join(trimmed, ","))
	}
	return strings.Join(lines, "\n") + "\n"
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
