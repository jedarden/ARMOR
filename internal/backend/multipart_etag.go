package backend

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// ComputeCompositeETag returns the S3-standard ETag for an object assembled
// from parts: md5(md5(part1)||…||md5(partN))-N. partETagsHex holds each
// stored part's MD5 as hex, in assembly order.
//
// Real S3 marks every multipart-created object with this composite form, and
// the trailing "-N" is the client-facing signal that the ETag is not a plain
// content MD5. ARMOR's filesystem backend previously reported a bare digest —
// md5 over the whole concatenated ciphertext stream (armor-e8981148) — and
// clients that verify content MD5 on download, rclone among them, read any
// bare 32-hex ETag as that MD5: every download of a multipart object then
// failed with "corrupted on transfer" even though the bytes were exact. The
// composite form is what tells such clients an object was assembled from
// parts and its ETag cannot be checked as a single-stream MD5.
func ComputeCompositeETag(partETagsHex []string) (string, error) {
	if len(partETagsHex) == 0 {
		return "", fmt.Errorf("composite ETag needs at least one part")
	}
	h := md5.New()
	for i, etagHex := range partETagsHex {
		digest, err := hex.DecodeString(etagHex)
		if err != nil || len(digest) != md5.Size {
			return "", fmt.Errorf("invalid MD5 for part %d: %q", i+1, etagHex)
		}
		h.Write(digest)
	}
	return fmt.Sprintf("%x-%d", h.Sum(nil), len(partETagsHex)), nil
}
