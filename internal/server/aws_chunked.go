package server

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// awsChunkedReader decodes the AWS streaming chunked body format used when
// X-Amz-Content-Sha256 is STREAMING-AWS4-HMAC-SHA256-PAYLOAD.
//
// Each chunk looks like:
//
//	<hex-size>;chunk-signature=<64-hex-chars>\r\n
//	<data bytes>\r\n
//
// Terminated by a zero-size chunk:
//
//	0;chunk-signature=<64-hex-chars>\r\n
//	\r\n
//
// ARMOR does not re-verify the per-chunk signatures — the seed signature on
// the initial Authorization header already authenticates the request.
type awsChunkedReader struct {
	br        *bufio.Reader
	orig      io.ReadCloser
	remaining int64
	done      bool
}

func newAWSChunkedReader(rc io.ReadCloser) *awsChunkedReader {
	return &awsChunkedReader{br: bufio.NewReaderSize(rc, 64*1024), orig: rc}
}

func (r *awsChunkedReader) Read(p []byte) (int, error) {
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
			if i := strings.IndexByte(line, ';'); i >= 0 {
				sizePart = line[:i]
			}
			chunkSize, err := strconv.ParseInt(sizePart, 16, 64)
			if err != nil {
				return total, fmt.Errorf("aws-chunked: bad chunk size %q: %w", sizePart, err)
			}

			if chunkSize == 0 {
				// Terminal chunk — consume the trailing \r\n
				r.br.ReadString('\n')
				r.done = true
				return total, io.EOF
			}
			r.remaining = chunkSize
		}

		// Read up to remaining bytes from the current chunk.
		limit := int64(len(p))
		if limit > r.remaining {
			limit = r.remaining
		}
		n, err := r.br.Read(p[:limit])
		total += n
		r.remaining -= int64(n)
		p = p[n:]

		if r.remaining == 0 {
			// Consume the CRLF that follows the chunk data.
			r.br.ReadString('\n')
		}

		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (r *awsChunkedReader) Close() error {
	return r.orig.Close()
}
