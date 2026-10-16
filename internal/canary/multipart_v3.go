package canary

import (
	"bytes"
	"context"
	cryptoRand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/metrics"
	"golang.org/x/sync/errgroup"
)

// checkMultipartV3 uses production's v3 multipart layout: independently
// encrypted ciphertext parts and a compressed sidecar with per-part HMACs.
func (m *Monitor) checkMultipartV3(ctx context.Context) (*Result, error) {
	if m.multipartSize < 2*b2MinPartSize+1 {
		return nil, fmt.Errorf("v3 multipart canary needs two non-final parts of at least %d bytes", b2MinPartSize)
	}
	result := &Result{LastCheck: time.Now()}
	timestamp := time.Now().UnixNano()
	plaintext := make([]byte, m.multipartSize)
	if _, err := cryptoRand.Read(plaintext); err != nil {
		return nil, fmt.Errorf("generate multipart canary content: %w", err)
	}
	binary.LittleEndian.PutUint64(plaintext[:8], uint64(timestamp))
	clientKey := fmt.Sprintf(".armor/canary-multipart-v3/%s/%d", m.instanceID, timestamp)
	key := m.applyPrefix(clientKey)
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}
	wrappedDEK, err := crypto.WrapDEK(m.mek, dek)
	if err != nil {
		return nil, fmt.Errorf("wrap DEK: %w", err)
	}
	sha := crypto.ComputePlaintextSHA256(plaintext)
	meta := (&backend.ARMORMetadata{
		Version:        3,
		BlockSize:      m.blockSize,
		PlaintextSize:  int64(len(plaintext)),
		ContentType:    "application/octet-stream",
		IV:             iv,
		WrappedDEK:     wrappedDEK,
		MEKFingerprint: crypto.MEKFingerprint(m.mek),
		PlaintextSHA:   hex.EncodeToString(sha[:]),
		ETag:           backend.ComputeETag(plaintext),
	}).ToMetadata()
	meta["x-amz-meta-armor-multipart"] = "true"
	manager := backend.NewMultipartStateManager(m.backend, m.bucket).WithKeyPrefix(m.prefix)
	uploadStart := time.Now()
	uploadID, err := m.backend.CreateMultipartUpload(ctx, m.bucket, key, meta)
	if err != nil {
		return nil, fmt.Errorf("create multipart upload: %w", err)
	}
	completed := false
	defer func() {
		if !completed {
			_ = m.backend.AbortMultipartUpload(context.Background(), m.bucket, key, uploadID)
		}
	}()

	partSizes := generateV3PartSizes(len(plaintext))
	parts := make([]backend.CompletedPart, len(partSizes))
	sidecarParts := make([]backend.HMACPartV3, len(partSizes))
	g, gctx := errgroup.WithContext(ctx)
	offset := 0
	for i, size := range partSizes {
		i, size, partPlaintext := i, size, plaintext[offset:offset+size]
		offset += size
		g.Go(func() error {
			partNumber := uint16(i + 1)
			ciphertext, hmacs, err := crypto.EncryptPartV3(dek, iv, partNumber, m.blockSize, partPlaintext)
			if err != nil {
				return fmt.Errorf("encrypt part %d: %w", partNumber, err)
			}
			etag, err := m.backend.UploadPart(gctx, m.bucket, key, uploadID, int32(partNumber), bytes.NewReader(ciphertext), int64(len(ciphertext)))
			if err != nil {
				return fmt.Errorf("upload part %d: %w", partNumber, err)
			}
			blocks := make([][]string, len(hmacs)/crypto.HMACSize)
			for block := range blocks {
				length := m.blockSize
				if remaining := size - block*m.blockSize; remaining < length {
					length = remaining
				}
				lengthBytes := make([]byte, 4)
				binary.BigEndian.PutUint32(lengthBytes, uint32(length))
				blocks[block] = []string{
					base64.StdEncoding.EncodeToString(hmacs[block*crypto.HMACSize : (block+1)*crypto.HMACSize]),
					base64.StdEncoding.EncodeToString(lengthBytes),
				}
			}
			parts[i] = backend.CompletedPart{PartNumber: int32(partNumber), ETag: etag}
			sidecarParts[i] = backend.HMACPartV3{N: int(partNumber), PlaintextLen: int64(size), CiphertextLen: int64(len(ciphertext)), Blocks: blocks}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if _, err := m.backend.CompleteMultipartUpload(ctx, m.bucket, key, uploadID, parts); err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}
	completed = true
	defer func() {
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = m.backend.Delete(cleanupCtx, m.bucket, key)
			_ = m.backend.Delete(cleanupCtx, m.bucket, backend.GetSidecarKey(m.prefix, clientKey))
		}()
	}()
	if err := manager.SaveHMACTableV3(ctx, clientKey, m.blockSize, sidecarParts); err != nil {
		return nil, fmt.Errorf("save v3 multipart sidecar: %w", err)
	}
	result.UploadLatencyMs = time.Since(uploadStart).Milliseconds()
	metrics.DefaultMetrics.RecordMultipartUpload("upload", "success", time.Since(uploadStart))

	downloadStart := time.Now()
	body, headers, err := m.backend.GetRangeWithHeaders(ctx, m.bucket, key, 0, int64(len(plaintext)))
	if err != nil {
		return nil, fmt.Errorf("download multipart canary: %w", err)
	}
	ciphertext, readErr := io.ReadAll(body)
	body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read multipart canary: %w", readErr)
	}
	if len(ciphertext) != len(plaintext) {
		return nil, fmt.Errorf("multipart ciphertext size mismatch: got %d, want %d", len(ciphertext), len(plaintext))
	}
	if cfStatus := headers["CF-Cache-Status"]; cfStatus == "HIT" || cfStatus == "STALE" || cfStatus == "REVALIDATED" {
		result.CFCacheHit = true
	}
	// A separate range read covers the boundary between independently uploaded parts.
	boundary := int64(partSizes[0])
	rangeStart := boundary - 512*1024
	rangeBody, _, err := m.backend.GetRangeWithHeaders(ctx, m.bucket, key, rangeStart, 1024*1024)
	if err != nil {
		return nil, fmt.Errorf("download multipart boundary range: %w", err)
	}
	boundaryBytes, readErr := io.ReadAll(rangeBody)
	rangeBody.Close()
	if readErr != nil || !bytes.Equal(boundaryBytes, ciphertext[rangeStart:rangeStart+1024*1024]) {
		return nil, fmt.Errorf("multipart boundary range mismatch: %v", readErr)
	}
	stored, err := m.backend.Head(ctx, m.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("read multipart metadata: %w", err)
	}
	storedMeta, ok := backend.ParseARMORMetadata(stored.Metadata)
	if !ok || storedMeta.Version != 3 || stored.Metadata["x-amz-meta-armor-multipart"] != "true" || storedMeta.PlaintextSize != int64(len(plaintext)) {
		return nil, fmt.Errorf("v3 multipart metadata is missing or inconsistent")
	}
	readDEK, err := crypto.UnwrapDEK(m.mek, storedMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap multipart DEK: %w", err)
	}
	decryptor, err := crypto.NewDecryptorWithVersion(readDEK, storedMeta.IV, storedMeta.BlockSize, crypto.Version3)
	if err != nil {
		return nil, fmt.Errorf("create multipart decryptor: %w", err)
	}
	sidecar, err := manager.LoadHMACTableV3(ctx, clientKey)
	if err != nil {
		return nil, fmt.Errorf("load v3 multipart sidecar: %w", err)
	}
	if sidecar.Version != 3 || sidecar.BlockSize != storedMeta.BlockSize || len(sidecar.Parts) != len(partSizes) {
		return nil, fmt.Errorf("v3 multipart sidecar is inconsistent")
	}
	decrypted := make([]byte, 0, len(plaintext))
	offset = 0
	for i, part := range sidecar.Parts {
		if part.N != i+1 || part.CiphertextLen != int64(partSizes[i]) {
			return nil, fmt.Errorf("v3 multipart sidecar part %d is inconsistent", i+1)
		}
		table := crypto.NewBlockTable(m.blockSize, len(part.Blocks))
		for _, block := range part.Blocks {
			if len(block) != 2 {
				return nil, fmt.Errorf("invalid v3 multipart sidecar block")
			}
			hmacBytes, hmacErr := base64.StdEncoding.DecodeString(block[0])
			lengthBytes, lengthErr := base64.StdEncoding.DecodeString(block[1])
			if hmacErr != nil || lengthErr != nil || len(hmacBytes) != crypto.HMACSize || len(lengthBytes) != 4 {
				return nil, fmt.Errorf("invalid v3 multipart sidecar block encoding")
			}
			var hmac [crypto.HMACSize]byte
			copy(hmac[:], hmacBytes)
			if err := table.AddEntry(crypto.NewBlockTableEntry(hmac, binary.BigEndian.Uint32(lengthBytes), false)); err != nil {
				return nil, fmt.Errorf("invalid v3 multipart block entry: %w", err)
			}
		}
		end := offset + int(part.CiphertextLen)
		partPlaintext, err := decryptor.DecryptV3(ciphertext[offset:end], uint16(part.N), table)
		if err != nil {
			return nil, fmt.Errorf("decrypt v3 multipart part %d: %w", part.N, err)
		}
		decrypted = append(decrypted, partPlaintext...)
		offset = end
	}
	decryptedSHA := crypto.ComputePlaintextSHA256(decrypted)
	if !bytes.Equal(decrypted, plaintext) || hex.EncodeToString(decryptedSHA[:]) != storedMeta.PlaintextSHA {
		return nil, fmt.Errorf("v3 multipart plaintext does not match the probe")
	}
	if m.readObject != nil {
		throughServer, err := m.readObject(ctx, m.bucket, clientKey)
		if err != nil {
			return nil, fmt.Errorf("v3 multipart GetObject failed: %w", err)
		}
		if !bytes.Equal(throughServer, plaintext) {
			return nil, fmt.Errorf("v3 multipart GetObject plaintext does not match the probe")
		}
	}
	result.DownloadLatencyMs = time.Since(downloadStart).Milliseconds()
	result.DecryptVerified = true
	result.HMACVerified = true
	result.Status = StatusHealthy
	result.MultipartHealthy = StatusHealthy
	result.MultipartHealthyBool = true
	metrics.DefaultMetrics.RecordMultipartUpload("verify", "success", time.Since(downloadStart))
	return result, nil
}
