package handlers

import "fmt"

// ContentSHA256MismatchCode is the S3 error code reported when the body a
// client uploads does not hash to the digest it signed in
// x-amz-content-sha256.
const ContentSHA256MismatchCode = "XAmzContentSHA256Mismatch"

// ContentSHA256MismatchError is produced by the payload-hash verifier the
// server middleware wraps around bodies signed with a hex
// x-amz-content-sha256 digest. writeErrorCause maps it to a 400
// XAmzContentSHA256Mismatch response so the upload fails before the object is
// finalised rather than storing an object that does not match its signature.
type ContentSHA256MismatchError struct {
	Expected string // the digest the request signed
	Got      string // the digest computed over the received body
}

func (e *ContentSHA256MismatchError) Error() string {
	return fmt.Sprintf("The provided 'x-amz-content-sha256' header does not match what was computed: expected %s, got %s", e.Expected, e.Got)
}

// ChunkSignatureMismatchCode is the standard S3 error code reported when a
// chunk of a STREAMING-AWS4-HMAC-SHA256-PAYLOAD body carries a signature that
// does not match the chained SigV4 computation.
const ChunkSignatureMismatchCode = "SignatureDoesNotMatch"

// ChunkSignatureMismatchError is produced by the aws-chunked reader when a
// chunk's signature fails the chained verification seeded from the
// Authorization header. writeErrorCause maps it to a 403
// SignatureDoesNotMatch response so the upload fails at the first bad chunk
// instead of storing a body the client did not sign (armor-3b18e5bc).
type ChunkSignatureMismatchError struct {
	Chunk int // zero-based index of the chunk whose signature failed
}

func (e *ChunkSignatureMismatchError) Error() string {
	return fmt.Sprintf("The chunk signature of chunk %d does not match the signature computed from the request", e.Chunk)
}
