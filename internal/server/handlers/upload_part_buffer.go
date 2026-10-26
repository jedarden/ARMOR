package handlers

import (
	"crypto/sha256"
	"errors"
	"io"
	"sync"
)

// Most production multipart requests have a 64 MiB Content-Length. Reusing
// that single buffer avoids io.ReadAll's geometric growth and lets v3 encrypt
// the bytes in place. Other known lengths are allocated exactly; unknown or
// larger lengths retain the original read-all behavior.
const pooledUploadPartSize = 64 << 20

var uploadPartBodyPool sync.Pool

type uploadPartDigestSource interface {
	CanVerifyPayloadSHA256() bool
	VerifiedPayloadSHA256() ([sha256.Size]byte, bool)
}

func readUploadPartBody(body io.Reader, contentLength int64) (data []byte, digest [sha256.Size]byte, release func(), err error) {
	var source uploadPartDigestSource
	if candidate, ok := body.(uploadPartDigestSource); ok && candidate.CanVerifyPayloadSHA256() {
		source = candidate
	}
	hasher := sha256.New()
	var reader io.Reader = body
	if source == nil {
		reader = io.TeeReader(body, hasher)
	}
	release = func() {}
	if contentLength < 0 || contentLength > pooledUploadPartSize {
		data, err = io.ReadAll(reader)
	} else {
		if contentLength == pooledUploadPartSize {
			if reused := uploadPartBodyPool.Get(); reused != nil {
				data = reused.([]byte)[:pooledUploadPartSize]
			} else {
				data = make([]byte, pooledUploadPartSize)
			}
			pooled := data
			release = func() { uploadPartBodyPool.Put(pooled) }
		} else {
			data = make([]byte, int(contentLength))
		}
		if _, err = io.ReadFull(reader, data); err != nil {
			release()
			return nil, digest, nil, err
		}

		// net/http enforces Content-Length, but a custom request body need not.
		// Preserve io.ReadAll's behavior if it supplies additional bytes.
		var one [1]byte
		for {
			var n int
			n, err = reader.Read(one[:])
			if n != 0 {
				readErr := err
				// A full pooled slice must remain ours while append copies it
				// into a larger allocation. Another request may Get it as soon
				// as release runs.
				data = append(data, one[:n]...)
				if contentLength == pooledUploadPartSize {
					release()
					release = func() {}
				}
				var rest []byte
				rest, err = io.ReadAll(reader)
				data = append(data, rest...)
				if readErr != nil && readErr != io.EOF {
					err = readErr
				}
				break
			}
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		release()
		return nil, digest, nil, err
	}
	if source != nil {
		var verified bool
		digest, verified = source.VerifiedPayloadSHA256()
		if !verified {
			release()
			return nil, digest, nil, errors.New("signed payload digest unavailable after body read")
		}
	} else {
		hasher.Sum(digest[:0])
	}
	return data, digest, release, nil
}
