package server

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jedarden/armor/internal/server/handlers"
)

// Constants of the SigV4 streaming chunk-signature scheme: the string-to-sign
// prefix every chunk uses, and the SHA-256 of the empty string, which occupies
// the "previous chunk hash" slot of every chunk string to sign.
const (
	chunkStringToSignPrefix = "AWS4-HMAC-SHA256-PAYLOAD"
	emptySHA256Hex          = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// sigV4SigningKey derives the SigV4 signing key for a secret, credential date,
// region, and service (the raw HMAC key, not hex).
func sigV4SigningKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

// hmacSHA256 computes HMAC-SHA256 over a string key's worth of data.
func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// chunkSignatureVerifier recomputes the chained SigV4 per-chunk signatures of
// a STREAMING-AWS4-HMAC-SHA256-PAYLOAD body. Each chunk's signature is an
// HMAC over the chunk's data hash and the previous chunk's signature, so
// verifying the chain in order binds every chunk to the seed signature that
// authenticated the Authorization header.
type chunkSignatureVerifier struct {
	signingKey []byte
	amzDate    string
	scope      string
	prev       string    // hex signature of the previous chunk (seed first)
	hash       hash.Hash // running SHA-256 of the current chunk's data
	expected   string    // signature the client claimed for the current chunk
	index      int       // zero-based index of the current chunk
}

// newChunkSignatureVerifier starts a chain whose first chunk is verified
// against seedSignature, the signature of the request's Authorization header.
// scope is the credential scope "date/region/s3/aws4_request" the request
// signed.
func newChunkSignatureVerifier(signingKey []byte, amzDate, scope, seedSignature string) *chunkSignatureVerifier {
	return &chunkSignatureVerifier{
		signingKey: signingKey,
		amzDate:    amzDate,
		scope:      scope,
		prev:       strings.ToLower(seedSignature),
		hash:       sha256.New(),
	}
}

// start begins a chunk claimed to carry expected as its signature.
func (v *chunkSignatureVerifier) start(expected string) {
	v.hash.Reset()
	v.expected = expected
}

// computeAndAdvance seals the current chunk: it derives the chunk signature,
// makes it the previous signature for the next chunk, and returns it.
func (v *chunkSignatureVerifier) computeAndAdvance() string {
	stringToSign := strings.Join([]string{
		chunkStringToSignPrefix,
		v.amzDate,
		v.scope,
		v.prev,
		emptySHA256Hex,
		hex.EncodeToString(v.hash.Sum(nil)),
	}, "\n")
	sig := hex.EncodeToString(hmacSHA256(v.signingKey, stringToSign))
	v.prev = sig
	v.hash.Reset()
	return sig
}

// finish seals the current chunk and verifies it against the signature the
// client claimed. The chain advances past a bad chunk only via computeAndAdvance;
// finish reports the mismatch as a *handlers.ChunkSignatureMismatchError
// naming the failing chunk.
func (v *chunkSignatureVerifier) finish() error {
	sig := v.computeAndAdvance()
	idx := v.index
	v.index++
	if !strings.EqualFold(sig, v.expected) {
		return &handlers.ChunkSignatureMismatchError{Chunk: idx}
	}
	return nil
}

// awsChunkedReader decodes the AWS streaming chunked body format used when
// X-Amz-Content-Sha256 is STREAMING-AWS4-HMAC-SHA256-PAYLOAD.
//
// Each chunk looks like:
//
//	<hex-size>;chunk-signature=<64-hex-chars>\r\n
//	<data bytes>\r\n
//
// Terminated by a zero-size chunk, optionally followed by trailer headers:
//
//	0;chunk-signature=<64-hex-chars>\r\n
//	[x-amz-checksum-...: value\r\n]...
//	\r\n
//
// When verifier is non-nil the reader verifies each chunk's signature against
// the chained SigV4 computation as the body is consumed and fails the read at
// the first bad chunk; a nil verifier strips the framing without verifying
// (requests authenticated any other way, e.g. presigned URLs).
type awsChunkedReader struct {
	br        *bufio.Reader
	orig      io.ReadCloser
	remaining int64
	done      bool
	verifier  *chunkSignatureVerifier
	err       error // sticky: once a chunk fails verification every read fails
}

func newAWSChunkedReader(rc io.ReadCloser, verifier *chunkSignatureVerifier) *awsChunkedReader {
	return &awsChunkedReader{
		br:       bufio.NewReaderSize(rc, 64*1024),
		orig:     rc,
		verifier: verifier,
	}
}

// chunkSignatureParam extracts the chunk-signature value from the parameters
// following the size in a chunk header line, or "" when absent.
func chunkSignatureParam(params string) string {
	for _, field := range strings.Split(params, ";") {
		if value, ok := strings.CutPrefix(field, "chunk-signature="); ok {
			return value
		}
	}
	return ""
}

func (r *awsChunkedReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.done {
		return 0, io.EOF
	}

	total := 0
	for len(p) > 0 {
		if r.remaining == 0 {
			// Read the chunk header line, e.g. "400;chunk-signature=abc...\r\n"
			line, err := r.br.ReadString('\n')
			if err != nil {
				if total > 0 {
					return total, nil
				}
				return 0, err
			}
			line = strings.TrimRight(line, "\r\n")

			// The chunk size is everything before the first ';'
			sizePart := line
			sig := ""
			if i := strings.IndexByte(line, ';'); i >= 0 {
				sizePart = line[:i]
				sig = chunkSignatureParam(line[i+1:])
			}
			chunkSize, err := strconv.ParseInt(sizePart, 16, 64)
			if err != nil {
				r.err = fmt.Errorf("aws-chunked: bad chunk size %q: %w", sizePart, err)
				return total, r.err
			}

			if chunkSize == 0 {
				if r.verifier != nil {
					// The zero chunk has no data; its signature chains the
					// final state and must verify like any other.
					r.verifier.start(sig)
					if verr := r.verifier.finish(); verr != nil {
						r.err = verr
						return total, verr
					}
				}
				r.skipTrailingHeaders()
				r.done = true
				return total, io.EOF
			}
			if r.verifier != nil {
				if sig == "" {
					r.err = &handlers.ChunkSignatureMismatchError{Chunk: r.verifier.index}
					return total, r.err
				}
				r.verifier.start(sig)
			}
			r.remaining = chunkSize
		}

		// Read up to remaining bytes from the current chunk.
		limit := int64(len(p))
		if limit > r.remaining {
			limit = r.remaining
		}
		n, err := r.br.Read(p[:limit])
		if n > 0 && r.verifier != nil {
			r.verifier.hash.Write(p[:n])
		}
		total += n
		r.remaining -= int64(n)
		p = p[n:]

		if r.remaining == 0 {
			if r.verifier != nil {
				if verr := r.verifier.finish(); verr != nil {
					// The bytes handed back were consumed under a signature
					// that does not verify; fail the read now and stickily.
					r.err = verr
					return total, verr
				}
			}
			// Consume the CRLF that follows the chunk data.
			r.br.ReadString('\n')
		}

		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// skipTrailingHeaders consumes everything after the zero chunk up to and
// including the final blank line. AWS clients may append signed checksum
// trailers there; ARMOR does not act on them, but they must be drained so the
// connection can be reused.
func (r *awsChunkedReader) skipTrailingHeaders() {
	for i := 0; i < 64; i++ {
		line, err := r.br.ReadString('\n')
		if err != nil || strings.TrimRight(line, "\r\n") == "" {
			return
		}
	}
}

// streamingChunkVerifier builds the chunk-signature verifier for a streaming
// request that presented an Authorization header: the seed is the signature
// the auth middleware just verified, and the signing key derives from the
// request's own credential date and region. Requests authenticated any other
// way (presigned URLs, public paths) get a nil verifier, which strips the
// framing without verifying.
func (s *Server) streamingChunkVerifier(r *http.Request) *chunkSignatureVerifier {
	cred := CredentialFromContext(r.Context())
	if cred == nil {
		return nil
	}
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return nil
	}
	parsed, err := ParseAuthHeader(r.Header.Get("Authorization"))
	if err != nil || parsed.Signature == "" {
		return nil
	}
	scope := parsed.CredentialDate + "/" + parsed.Region + "/s3/aws4_request"
	signingKey := sigV4SigningKey(cred.SecretKey, parsed.CredentialDate, parsed.Region, "s3")
	return newChunkSignatureVerifier(signingKey, amzDate, scope, parsed.Signature)
}

func (r *awsChunkedReader) Close() error {
	return r.orig.Close()
}
