package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/jedarden/armor/internal/server/handlers"
)

// The x-amz-content-sha256 payload modes ARMOR honours. The header is part of
// the signed SigV4 canonical request, so a mode ARMOR cannot honour has to be
// rejected before the handler reads the body — honouring it silently would
// either store framing the client never signed (an unknown streaming format)
// or accept a body that is not the one the signature covers.
const (
	unsignedPayloadHeader  = "UNSIGNED-PAYLOAD"
	streamingPayloadHeader = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"
)

type payloadMode int

const (
	// payloadDigest: the header is a hex SHA-256 digest of the body. The body
	// is hashed as the handler reads it and the upload fails if it disagrees.
	payloadDigest payloadMode = iota
	// payloadUnsigned: UNSIGNED-PAYLOAD (or no header) — the body is not
	// bound to the signature and passes through unmodified.
	payloadUnsigned
	// payloadStreaming: STREAMING-AWS4-HMAC-SHA256-PAYLOAD — the aws-chunked
	// framing is stripped and the chained per-chunk signatures are verified
	// against the Authorization seed (see aws_chunked.go).
	payloadStreaming
)

// classifyPayloadHashMode validates the x-amz-content-sha256 header value and
// reports how the request body must be handled. An absent header behaves like
// UNSIGNED-PAYLOAD: the SigV4 canonical request then binds the empty-string
// digest, not the body. Anything that is not one of the three supported modes
// is an error whose message names them.
func classifyPayloadHashMode(value string) (payloadMode, error) {
	switch value {
	case "":
		return payloadUnsigned, nil
	case unsignedPayloadHeader:
		return payloadUnsigned, nil
	case streamingPayloadHeader:
		return payloadStreaming, nil
	}
	if isHexSHA256(value) {
		return payloadDigest, nil
	}
	return payloadDigest, fmt.Errorf(
		"unsupported x-amz-content-sha256 value %q: supported modes are a hex-encoded SHA-256 digest, %s, or %s",
		value, unsignedPayloadHeader, streamingPayloadHeader)
}

// isHexSHA256 reports whether s is a 64-character hexadecimal SHA-256 digest.
func isHexSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// payloadHashVerifier computes SHA-256 over the request body as the handler
// reads it. When the body is exhausted it compares the digest against the one
// the client signed; on disagreement the read fails with a
// *handlers.ContentSHA256MismatchError, which writeErrorCause renders as a
// 400 XAmzContentSHA256Mismatch. The handler's normal body-read error paths
// therefore abandon the upload before anything is written to the backend.
type payloadHashVerifier struct {
	rc       io.ReadCloser
	hash     hash.Hash
	expected string
	err      error
}

// newPayloadHashVerifier wraps a request body whose signed
// x-amz-content-sha256 value is expected, a hex SHA-256 digest.
func newPayloadHashVerifier(rc io.ReadCloser, expected string) *payloadHashVerifier {
	return &payloadHashVerifier{
		rc:       rc,
		hash:     sha256.New(),
		expected: strings.ToLower(expected),
	}
}

func (v *payloadHashVerifier) Read(p []byte) (int, error) {
	if v.err != nil {
		return 0, v.err
	}
	n, err := v.rc.Read(p)
	if n > 0 {
		v.hash.Write(p[:n])
	}
	if err == io.EOF {
		if got := hex.EncodeToString(v.hash.Sum(nil)); got != v.expected {
			v.err = &handlers.ContentSHA256MismatchError{Expected: v.expected, Got: got}
		} else {
			v.err = io.EOF
		}
		return n, v.err
	}
	if err != nil {
		v.err = err
	}
	return n, err
}

func (v *payloadHashVerifier) Close() error {
	return v.rc.Close()
}
