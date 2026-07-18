package restoreverifier

import (
	"archive/tar"
	"compress/gzip"
	"encoding/hex"
	"io"
)

// This file holds small test-local helpers shared by the assertion and
// dual-path tests in this package. They are deliberately thin wrappers so the
// tests read at the level of intent ("build a gzip/tar stream", "hex-encode a
// digest") without each call site repeating the stdlib spelling.

// hexEncode hex-encodes b (e.g. a SHA-256 digest) for embedding in ARMOR
// object metadata during dual-path tests.
func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

// newGzipWriter wraps gzip.NewWriter so tar.gz tests build a compressed member
// without repeating the constructor.
func newGzipWriter(w io.Writer) *gzip.Writer {
	return gzip.NewWriter(w)
}

// newTarWriter wraps tar.NewWriter so tar.gz tests build an archive member
// without repeating the constructor.
func newTarWriter(w io.Writer) *tar.Writer {
	return tar.NewWriter(w)
}
