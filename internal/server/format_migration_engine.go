package server

// This file is the durable Phase 8.11 migration engine. The older helpers in
// format_migration.go remain useful for fixture compatibility, but all admin
// migrations use this engine: work is detached from the request, checkpoints
// are persisted per object, and the source is changed only by a conditional
// publish from a migration-scoped staging key.

import (
	"bytes"
	"context"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

const migrationLeaseTTL = 5 * time.Minute

type migrationContract struct {
	Bucket      string   `json:"bucket"`
	Target      uint8    `json:"target"`
	Include     []string `json:"include"`
	DryRun      bool     `json:"dry_run"`
	Concurrency int      `json:"concurrency"`
	KeyID       string   `json:"key_id"`
	KeyPrefix   string   `json:"key_prefix"`
}

// RunDurable executes a resumable migration. It is intentionally synchronous
// so the caller can own the worker goroutine and report a final result in unit
// tests; the HTTP layer calls it from a detached background job.
func (fm *FormatMigrator) RunDurable(ctx context.Context, dryRun bool, concurrency int) (*MigrationResult, error) {
	if concurrency < 1 {
		concurrency = 1
	}
	digestBytes, err := json.Marshal(migrationContract{
		Bucket: fm.bucket, Target: fm.currentWriteVersion, Include: fm.includeVersions,
		DryRun: dryRun, Concurrency: concurrency, KeyID: fm.keyID, KeyPrefix: fm.keyPrefix,
	})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(digestBytes)
	configDigest := hex.EncodeToString(digest[:])

	state, loadErr := fm.loadState(ctx)
	if loadErr != nil && !migrationStateMissing(loadErr) {
		return nil, fmt.Errorf("load migration state: %w", loadErr)
	}
	if state == nil || state.Status == "completed" || state.DryRun != dryRun || (state.ConfigDigest != "" && state.ConfigDigest != configDigest) {
		state = &MigrationState{
			ID:                  fmt.Sprintf("format-migration-%d", time.Now().UnixNano()),
			StartTime:           time.Now().UTC(),
			LastUpdated:         time.Now().UTC(),
			Status:              "initialized",
			IncludeVersions:     append([]string(nil), fm.includeVersions...),
			CurrentWriteVersion: fm.currentWriteVersion,
			DryRun:              dryRun,
			Concurrency:         concurrency,
			ConfigDigest:        configDigest,
			LeaseKey:            fm.keyPrefix + ".armor/format-migration-lease.json",
			Checkpoints:         make(map[string]MigrationCheckpoint),
		}
	} else {
		if state.Checkpoints == nil {
			state.Checkpoints = make(map[string]MigrationCheckpoint)
		}
		// A process restart may change worker count, but never the migration
		// contract that protects existing checkpoints.
		if state.ConfigDigest == "" {
			state.ConfigDigest = configDigest
		}
		if state.LeaseKey == "" {
			state.LeaseKey = fm.keyPrefix + ".armor/format-migration-lease.json"
		}
		fm.includeVersions = append([]string(nil), state.IncludeVersions...)
		fm.currentWriteVersion = state.CurrentWriteVersion
	}
	fm.stateMu.Lock()
	fm.state = state
	fm.state.Status = "in_progress"
	fm.state.LastUpdated = time.Now().UTC()
	fm.stateMu.Unlock()

	if err := fm.acquireMigrationLease(ctx, state.ID); err != nil {
		return nil, err
	}
	defer fm.releaseMigrationLease(context.Background())
	if err := fm.saveState(ctx); err != nil {
		return nil, fmt.Errorf("save migration start: %w", err)
	}
	if err := fm.countObjects(ctx); err != nil {
		fm.finishState("failed", err.Error())
		_ = fm.saveState(context.Background())
		return nil, err
	}

	jobs := make(chan backend.ObjectInfo, concurrency*2)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range jobs {
				if err := fm.processDurableObject(ctx, obj, dryRun); err != nil {
					errMu.Lock()
					if firstErr == nil && errors.Is(err, context.Canceled) {
						firstErr = err
					}
					errMu.Unlock()
				}
			}
		}()
	}

	var continuation string
	listErr := error(nil)
	for {
		if ctx.Err() != nil {
			listErr = ctx.Err()
			break
		}
		page, err := fm.backend.List(ctx, fm.bucket, "", "", continuation, 1000)
		if err != nil {
			listErr = err
			break
		}
		for _, obj := range page.Objects {
			if fm.isInternalKey(obj.Key) {
				continue
			}
			fm.stateMu.Lock()
			cp := fm.state.Checkpoints[obj.Key]
			fm.stateMu.Unlock()
			if cp.Status == "committed" || cp.Status == "skipped" {
				continue
			}
			select {
			case jobs <- obj:
			case <-ctx.Done():
				listErr = ctx.Err()
			}
			if listErr != nil {
				break
			}
		}
		if listErr != nil || !page.IsTruncated {
			break
		}
		continuation = page.NextToken
	}
	close(jobs)
	wg.Wait()
	if listErr != nil {
		fm.finishState("interrupted", listErr.Error())
		_ = fm.saveState(context.Background())
		return fm.durableResult("interrupted"), listErr
	}
	if firstErr != nil && errors.Is(firstErr, context.Canceled) {
		fm.finishState("interrupted", firstErr.Error())
		_ = fm.saveState(context.Background())
		return fm.durableResult("interrupted"), firstErr
	}
	fm.finishState("completed", "")
	if err := fm.saveState(context.Background()); err != nil {
		return fm.durableResult("failed"), err
	}
	return fm.durableResult("completed"), nil
}

func migrationStateMissing(err error) bool {
	if errors.Is(err, backend.ErrObjectNotFound) {
		return true
	}
	// FSBackend wraps os.ErrNotExist while B2 wraps NoSuchKey. Both are an
	// absent state file, not corrupt durable state.
	s := err.Error()
	return strings.Contains(s, "no such file") || strings.Contains(s, "NoSuchKey") || strings.Contains(s, "NotFound")
}

func (fm *FormatMigrator) finishState(status, message string) {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	fm.state.Status = status
	fm.state.ErrorMessage = message
	fm.state.LastUpdated = time.Now().UTC()
	fm.state.CommitPoint = "complete:" + status
}

func (fm *FormatMigrator) durableResult(status string) *MigrationResult {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	return &MigrationResult{
		TotalObjects: fm.state.TotalObjects, ProcessedObjects: fm.state.ProcessedObjects,
		SkippedObjects: fm.state.SkippedObjects, FailedObjects: fm.state.FailedObjects,
		Failures: append([]MigrationFailure(nil), fm.state.Failures...), Duration: time.Since(fm.state.StartTime),
		Status: status, ErrorMessage: fm.state.ErrorMessage, DryRun: fm.state.DryRun,
		Classification: fm.state.Classification.copy(),
	}
}

func (fm *FormatMigrator) processDurableObject(ctx context.Context, obj backend.ObjectInfo, dryRun bool) error {
	// Per-key exclusion protects a retry or a duplicate listing from creating
	// two staging commits for the same logical object.
	keyMu := fm.migrationKeyLock(obj.Key)
	keyMu.Lock()
	defer keyMu.Unlock()

	rawMeta, err := fm.objectMetadata(ctx, obj)
	if err != nil {
		return fm.checkpointFailure(obj.Key, fmt.Sprintf("metadata: %v", err), false)
	}
	armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
	if !ok || armorMeta == nil {
		return fm.checkpointSkipped(obj.Key)
	}
	if !fm.shouldMigrateVersion(uint8(armorMeta.Version)) || uint8(armorMeta.Version) == fm.currentWriteVersion {
		return fm.checkpointSkipped(obj.Key)
	}
	if err := fm.validatePartAccounting(rawMeta); err != nil {
		return fm.checkpointFailure(obj.Key, err.Error(), false)
	}
	if err := fm.validateEnvelopeVersion(ctx, obj, rawMeta, armorMeta); err != nil {
		return fm.checkpointFailure(obj.Key, err.Error(), false)
	}

	info, err := fm.backend.Head(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fm.checkpointFailure(obj.Key, fmt.Sprintf("source head: %v", err), false)
	}
	checkpoint := MigrationCheckpoint{Key: obj.Key, SourceETag: info.ETag, SourceVersion: info.VersionID, Status: "running", UpdatedAt: time.Now().UTC()}
	fm.stateMu.Lock()
	fm.state.Checkpoints[obj.Key] = checkpoint
	fm.state.LastKey = obj.Key
	fm.state.LastUpdated = time.Now().UTC()
	fm.stateMu.Unlock()
	_ = fm.saveState(context.Background())

	if dryRun {
		if err := fm.verifyStreamingSource(ctx, obj.Key, rawMeta, armorMeta, info); err != nil {
			return fm.checkpointFailure(obj.Key, err.Error(), errors.Is(err, ErrIntegrityVerification))
		}
		return fm.checkpointProcessed(obj.Key, "skipped")
	}
	staging := fm.stagingKey(obj.Key)
	if err := fm.stageStreamingObject(ctx, obj.Key, rawMeta, armorMeta, info, staging); err != nil {
		_ = fm.backend.Delete(context.Background(), fm.bucket, staging)
		return fm.checkpointFailure(obj.Key, err.Error(), errors.Is(err, ErrIntegrityVerification))
	}
	checkpoint.StagingKey = staging
	checkpoint.Status = "staged"
	checkpoint.UpdatedAt = time.Now().UTC()
	fm.stateMu.Lock()
	fm.state.Checkpoints[obj.Key] = checkpoint
	fm.state.Rollback = append(fm.state.Rollback, MigrationRollback{Key: obj.Key, StagingKey: staging, CreatedAt: time.Now().UTC()})
	fm.stateMu.Unlock()
	if err := fm.saveState(context.Background()); err != nil {
		return err
	}

	cb, ok := fm.backend.(backend.ConditionalCopyBackend)
	if !ok {
		return fm.checkpointFailure(obj.Key, backend.ErrConditionalPutUnsupported.Error(), false)
	}
	if err := cb.CopyIfMatch(ctx, fm.bucket, staging, fm.bucket, obj.Key, info.ETag, nil, false); err != nil {
		return fm.checkpointFailure(obj.Key, fmt.Sprintf("commit: %v", err), false)
	}
	// A multipart v3 object keeps its block table in a sidecar. Publish that
	// sidecar only after the source conditional copy succeeds; a racing
	// overwrite therefore cannot have its source replaced by this migration.
	if committedInfo, headErr := fm.backend.Head(ctx, fm.bucket, obj.Key); headErr == nil && committedInfo.Metadata[armorMetaMultipart] == "true" {
		stagingSidecar := backend.GetSidecarKey(fm.keyPrefix, strings.TrimPrefix(staging, fm.keyPrefix))
		finalSidecar := backend.GetSidecarKey(fm.keyPrefix, strings.TrimPrefix(obj.Key, fm.keyPrefix))
		if err := fm.backend.Copy(ctx, fm.bucket, stagingSidecar, fm.bucket, finalSidecar, nil, false); err != nil {
			return fm.checkpointFailure(obj.Key, fmt.Sprintf("publish multipart sidecar: %v", err), false)
		}
		_ = fm.backend.Delete(context.Background(), fm.bucket, stagingSidecar)
	}
	if fm.idx != nil {
		if committedInfo, headErr := fm.backend.Head(ctx, fm.bucket, obj.Key); headErr == nil {
			if committedMeta, parsed := backend.ParseARMORMetadata(committedInfo.Metadata); parsed {
				fm.idx.Put(fm.bucket, obj.Key, &manifest.Entry{
					PlaintextSize: committedMeta.PlaintextSize, PlaintextSHA256: committedMeta.PlaintextSHA,
					IV: committedMeta.IV, WrappedDEK: committedMeta.WrappedDEK, MEKFingerprint: committedMeta.MEKFingerprint,
					BlockSize: committedMeta.BlockSize, ContentType: committedInfo.ContentType, ETag: committedInfo.ETag,
					CiphertextSize: committedInfo.StoredSize, LastModified: committedInfo.LastModified,
				})
			}
		}
	}
	_ = fm.backend.Delete(context.Background(), fm.bucket, staging)
	fm.stateMu.Lock()
	checkpoint.Status = "committed"
	checkpoint.UpdatedAt = time.Now().UTC()
	fm.state.Checkpoints[obj.Key] = checkpoint
	fm.state.ProcessedObjects++
	fm.state.Classification.recordOutcome(outcomeProcessed)
	fm.state.CommitPoint = "object:" + obj.Key
	fm.state.LastUpdated = time.Now().UTC()
	fm.stateMu.Unlock()
	return fm.saveState(context.Background())
}

func (fm *FormatMigrator) checkpointSkipped(key string) error {
	fm.stateMu.Lock()
	fm.state.Checkpoints[key] = MigrationCheckpoint{Key: key, Status: "skipped", UpdatedAt: time.Now().UTC()}
	fm.state.SkippedObjects++
	fm.state.Classification.recordOutcome(outcomeSkipped)
	fm.state.LastKey = key
	fm.state.LastUpdated = time.Now().UTC()
	fm.stateMu.Unlock()
	return fm.saveState(context.Background())
}

func (fm *FormatMigrator) checkpointProcessed(key, status string) error {
	fm.stateMu.Lock()
	fm.state.Checkpoints[key] = MigrationCheckpoint{Key: key, Status: status, UpdatedAt: time.Now().UTC()}
	fm.state.ProcessedObjects++
	fm.state.Classification.recordOutcome(outcomeProcessed)
	fm.state.LastKey = key
	fm.state.LastUpdated = time.Now().UTC()
	fm.stateMu.Unlock()
	return fm.saveState(context.Background())
}

func (fm *FormatMigrator) checkpointFailure(key, reason string, integrity bool) error {
	f := fm.recordFailure(key, reason)
	fm.stateMu.Lock()
	fm.state.Checkpoints[key] = MigrationCheckpoint{Key: key, Status: "failed", Error: reason, UpdatedAt: time.Now().UTC()}
	fm.state.FailedObjects++
	fm.state.Failures = append(fm.state.Failures, f)
	if integrity {
		fm.state.Classification.recordOutcome(outcomeIntegrityFailed)
	} else {
		fm.state.Classification.recordOutcome(outcomeFailed)
	}
	fm.state.LastKey = key
	fm.state.LastUpdated = time.Now().UTC()
	fm.stateMu.Unlock()
	return fm.saveState(context.Background())
}

var migrationKeyLocks sync.Map

func (fm *FormatMigrator) migrationKeyLock(key string) *sync.Mutex {
	v, _ := migrationKeyLocks.LoadOrStore(fm.bucket+"\x00"+key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (fm *FormatMigrator) stagingKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return fm.keyPrefix + ".armor/migrations/" + fm.state.ID + "/objects/" + hex.EncodeToString(h[:])
}

func (fm *FormatMigrator) acquireMigrationLease(ctx context.Context, migrationID string) error {
	cp, ok := fm.backend.(backend.ConditionalPutBackend)
	if !ok {
		return backend.ErrConditionalPutUnsupported
	}
	lease := map[string]interface{}{"migration_id": migrationID, "acquired_at": time.Now().UTC()}
	b, _ := json.Marshal(lease)
	err := cp.PutIfAbsent(ctx, fm.bucket, fm.state.LeaseKey, bytesReader(b), int64(len(b)), map[string]string{"Content-Type": "application/json"})
	if err == nil {
		return nil
	}
	if !errors.Is(err, backend.ErrPreconditionFailed) {
		return err
	}
	r, _, readErr := fm.backend.GetDirect(ctx, fm.bucket, fm.state.LeaseKey)
	if readErr != nil {
		return fmt.Errorf("migration lease held: %w", err)
	}
	defer r.Close()
	var current struct {
		MigrationID string    `json:"migration_id"`
		AcquiredAt  time.Time `json:"acquired_at"`
	}
	if json.NewDecoder(r).Decode(&current) == nil && time.Since(current.AcquiredAt) < migrationLeaseTTL {
		return fmt.Errorf("migration lease held by %s", current.MigrationID)
	}
	// A lease older than the bounded TTL is recoverable after a pod crash.
	return fm.backend.Put(ctx, fm.bucket, fm.state.LeaseKey, bytesReader(b), int64(len(b)), map[string]string{"Content-Type": "application/json"})
}

func (fm *FormatMigrator) releaseMigrationLease(ctx context.Context) {
	if fm.state != nil && fm.state.LeaseKey != "" {
		_ = fm.backend.Delete(ctx, fm.bucket, fm.state.LeaseKey)
	}
}

func (fm *FormatMigrator) verifyStreamingSource(ctx context.Context, key string, raw map[string]string, meta *backend.ARMORMetadata, info *backend.ObjectInfo) error {
	plain, done, expected, err := fm.openPlaintextStream(ctx, key, raw, meta, info)
	if err != nil {
		return err
	}
	defer done()
	h := sha256.New()
	if _, err := io.Copy(io.Discard, io.TeeReader(plain, h)); err != nil {
		return fmt.Errorf("stream decrypt: %w", err)
	}
	if expected != "" && hex.EncodeToString(h.Sum(nil)) != expected {
		return fmt.Errorf("%w: expected %s got %s", ErrIntegrityVerification, expected, hex.EncodeToString(h.Sum(nil)))
	}
	return nil
}

func (fm *FormatMigrator) stageStreamingObject(ctx context.Context, key string, raw map[string]string, meta *backend.ARMORMetadata, info *backend.ObjectInfo, staging string) error {
	plain, done, expected, err := fm.openPlaintextStream(ctx, key, raw, meta, info)
	if err != nil {
		return err
	}
	defer done()
	plaintextSize := meta.PlaintextSize
	if plaintextSize < 0 {
		return fmt.Errorf("negative plaintext size")
	}
	dek := make([]byte, 32)
	if _, err := io.ReadFull(cryptoRand.Reader, dek); err != nil {
		return err
	}
	wrapped, err := crypto.WrapDEK(fm.mek, dek)
	if err != nil {
		return err
	}
	iv := make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, iv); err != nil {
		return err
	}
	blockSize := 65536
	if plaintextSize <= int64(fm.multipartThreshold()) {
		blockSize = 4096
	}
	var sum [32]byte
	if expected != "" {
		decoded, err := hex.DecodeString(expected)
		if err == nil && len(decoded) == len(sum) {
			copy(sum[:], decoded)
		}
	}
	if plaintextSize > int64(fm.multipartThreshold()) {
		return fm.stageStreamingMultipart(ctx, key, raw, plain, dek, iv, wrapped, blockSize, plaintextSize, sum[:], staging)
	}
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, plaintextSize, blockSize, sum, fm.currentWriteVersion)
	if err != nil {
		return err
	}
	headerBytes, err := header.Encode()
	if err != nil {
		return err
	}
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, fm.currentWriteVersion)
	if err != nil {
		return err
	}
	outputSize := int64(len(headerBytes)) + plaintextSize
	if fm.currentWriteVersion == crypto.Version3 {
		outputSize += int64(crypto.ComputeBlockCount(plaintextSize, blockSize)) * crypto.BlockTableEntrySize
	} else {
		outputSize += int64(crypto.ComputeBlockCount(plaintextSize, blockSize)) * crypto.HMACSize
	}
	newMeta := fm.buildNewMetadata64(raw, iv, wrapped, blockSize, plaintextSize, sum[:], crypto.MEKFingerprint(fm.mek))
	newMeta["Content-Type"] = raw["Content-Type"]

	outR, outW := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if _, err := outW.Write(headerBytes); err != nil {
			errCh <- err
			_ = outW.CloseWithError(err)
			return
		}
		var encErr error
		if fm.currentWriteVersion == crypto.Version3 {
			_, encErr = encryptor.EncryptV3Stream(plain, outW, plaintextSize, false)
		} else {
			var table []byte
			table, encErr = encryptor.EncryptStream(plain, outW, plaintextSize)
			if encErr == nil {
				_, encErr = outW.Write(table)
			}
		}
		if encErr != nil {
			_ = outW.CloseWithError(encErr)
			errCh <- encErr
			return
		}
		_ = outW.Close()
		errCh <- nil
	}()
	if err := fm.backend.Put(ctx, fm.bucket, staging, outR, outputSize, newMeta); err != nil {
		_ = outR.CloseWithError(err)
		return err
	}
	if err := <-errCh; err != nil {
		return fmt.Errorf("stream encrypt: %w", err)
	}
	return nil
}

// stageStreamingMultipart keeps only one S3 part plus one encrypted part in
// memory. The migration output is uploaded under the staging key; the source
// key is not touched until the caller's conditional publish succeeds.
func (fm *FormatMigrator) stageStreamingMultipart(ctx context.Context, key string, raw map[string]string, plain io.Reader, dek, iv, wrapped []byte, blockSize int, plaintextSize int64, plaintextSHA []byte, staging string) error {
	uploadID, err := fm.backend.CreateMultipartUpload(ctx, fm.bucket, staging, nil)
	if err != nil {
		return fmt.Errorf("create staged multipart upload: %w", err)
	}
	completed := false
	defer func() {
		if !completed {
			_ = fm.backend.AbortMultipartUpload(context.Background(), fm.bucket, staging, uploadID)
		}
	}()

	const partSize = 8 << 20
	parts := make([]backend.CompletedPart, 0, (plaintextSize+partSize-1)/partSize)
	var v3Parts []backend.HMACPartV3
	var v2HMACs [][]byte
	blockOffset := uint32(0)
	remaining := plaintextSize
	partNumber := int32(1)
	for remaining > 0 {
		nwant := int64(partSize)
		if remaining < nwant {
			nwant = remaining
		}
		buf := make([]byte, nwant)
		n, readErr := io.ReadFull(plain, buf)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			return fmt.Errorf("read plaintext part %d: %w", partNumber, readErr)
		}
		if n == 0 {
			break
		}
		buf = buf[:n]
		var encrypted, hmacs []byte
		if fm.currentWriteVersion == crypto.Version3 {
			encrypted, hmacs, err = crypto.EncryptPartV3(dek, iv, uint16(partNumber), blockSize, buf)
		} else {
			enc, encErr := crypto.NewEncryptorWithVersion(dek, iv, blockSize, fm.currentWriteVersion)
			if encErr != nil {
				return encErr
			}
			encrypted, hmacs, err = enc.EncryptWithStartingCounter(buf, blockOffset)
		}
		if err != nil {
			return fmt.Errorf("encrypt part %d: %w", partNumber, err)
		}
		etag, err := fm.backend.UploadPart(ctx, fm.bucket, staging, uploadID, partNumber, bytes.NewReader(encrypted), int64(len(encrypted)))
		if err != nil {
			return fmt.Errorf("upload staged part %d: %w", partNumber, err)
		}
		parts = append(parts, backend.CompletedPart{PartNumber: partNumber, ETag: etag})
		if fm.currentWriteVersion == crypto.Version3 {
			part := backend.HMACPartV3{N: int(partNumber), PlaintextLen: int64(n), CiphertextLen: int64(len(encrypted))}
			for off, block := 0, 0; off < len(hmacs); off, block = off+crypto.HMACSize, block+1 {
				part.Blocks = append(part.Blocks, []string{base64.StdEncoding.EncodeToString(hmacs[off : off+crypto.HMACSize]), fmt.Sprintf("%d", minInt64(int64(blockSize), int64(n-block*blockSize)))})
			}
			v3Parts = append(v3Parts, part)
		} else {
			for off := 0; off < len(hmacs); off += crypto.HMACSize {
				v2HMACs = append(v2HMACs, append([]byte(nil), hmacs[off:off+crypto.HMACSize]...))
			}
			blockOffset += crypto.ComputeBlockCount(int64(n), blockSize)
		}
		remaining -= int64(n)
		partNumber++
	}
	if len(parts) == 0 {
		return fmt.Errorf("empty multipart migration output")
	}
	if _, err := fm.backend.CompleteMultipartUpload(ctx, fm.bucket, staging, uploadID, parts); err != nil {
		return fmt.Errorf("complete staged multipart upload: %w", err)
	}
	completed = true
	newMeta := fm.buildNewMetadata64(raw, iv, wrapped, blockSize, plaintextSize, plaintextSHA, crypto.MEKFingerprint(fm.mek))
	newMeta[armorMetaMultipart] = "true"
	newMeta[armorMetaPartSize] = fmt.Sprintf("%d", partSize)
	if err := fm.backend.Copy(ctx, fm.bucket, staging, fm.bucket, staging, newMeta, true); err != nil {
		return fmt.Errorf("set staged multipart metadata: %w", err)
	}
	manager := backend.NewMultipartStateManager(fm.backend, fm.bucket).WithKeyPrefix(fm.keyPrefix)
	stagingClientKey := strings.TrimPrefix(staging, fm.keyPrefix)
	if fm.currentWriteVersion == crypto.Version3 {
		if err := manager.SaveHMACTableV3(ctx, stagingClientKey, blockSize, v3Parts); err != nil {
			return fmt.Errorf("save staged v3 sidecar: %w", err)
		}
	} else if err := manager.SaveHMACTable(ctx, stagingClientKey, v2HMACs, blockSize, int(fm.currentWriteVersion)); err != nil {
		return fmt.Errorf("save staged hmac sidecar: %w", err)
	}
	return nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (fm *FormatMigrator) openPlaintextStream(ctx context.Context, key string, raw map[string]string, meta *backend.ARMORMetadata, info *backend.ObjectInfo) (io.Reader, func(), string, error) {
	var cipherReader io.ReadCloser
	var hmacTable []byte
	var plainSize int64 = meta.PlaintextSize
	var blockSize = meta.BlockSize
	if raw[armorMetaMultipart] == "true" {
		stored := info.StoredSize
		if stored <= 0 {
			stored = info.Size
		}
		var err error
		cipherReader, err = fm.backend.GetRange(ctx, fm.bucket, key, 0, stored)
		if err != nil {
			return nil, nil, "", err
		}
		hmacTable, err = fm.loadHMCTableFromSidecar(key)
		if err != nil {
			cipherReader.Close()
			return nil, nil, "", err
		}
	} else {
		hr, err := fm.backend.GetRange(ctx, fm.bucket, key, 0, crypto.HeaderSize)
		if err != nil {
			return nil, nil, "", err
		}
		headerBytes := make([]byte, crypto.HeaderSize)
		_, readErr := io.ReadFull(hr, headerBytes)
		hr.Close()
		if readErr != nil {
			return nil, nil, "", readErr
		}
		header, err := crypto.DecodeHeader(headerBytes)
		if err != nil {
			return nil, nil, "", err
		}
		plainSize = int64(header.PlaintextSize)
		blockSize = header.BlockSize()
		hmacSize := int64(crypto.ComputeBlockCount(plainSize, blockSize)) * crypto.HMACSize
		cipherReader, err = fm.backend.GetRange(ctx, fm.bucket, key, crypto.HeaderSize, plainSize)
		if err != nil {
			return nil, nil, "", err
		}
		trailer, err := fm.backend.GetRange(ctx, fm.bucket, key, crypto.HeaderSize+plainSize, hmacSize)
		if err != nil {
			cipherReader.Close()
			return nil, nil, "", err
		}
		hmacTable = make([]byte, hmacSize)
		_, err = io.ReadFull(trailer, hmacTable)
		trailer.Close()
		if err != nil {
			cipherReader.Close()
			return nil, nil, "", err
		}
	}
	dek, err := crypto.UnwrapDEK(fm.mek, meta.WrappedDEK)
	if err != nil {
		cipherReader.Close()
		return nil, nil, "", err
	}
	decryptor, err := crypto.NewDecryptorWithVersion(dek, meta.IV, blockSize, uint8(meta.Version))
	if err != nil {
		cipherReader.Close()
		return nil, nil, "", err
	}
	plainR, plainW := io.Pipe()
	done := make(chan struct{})
	go func() {
		h := sha256.New()
		mw := io.MultiWriter(plainW, h)
		err := decryptor.DecryptStream(cipherReader, mw, hmacTable, int(crypto.ComputeBlockCount(plainSize, blockSize)))
		cipherReader.Close()
		if err == nil {
			expected := raw[armorMetaPlaintextSHA]
			if expected == "" {
				expected = meta.PlaintextSHA
			}
			if expected != "" && !backend.IsPlaceholderPlaintextSHA(expected) && hex.EncodeToString(h.Sum(nil)) != expected {
				err = fmt.Errorf("%w: expected %s got %s", ErrIntegrityVerification, expected, hex.EncodeToString(h.Sum(nil)))
			}
		}
		if err != nil {
			_ = plainW.CloseWithError(err)
		} else {
			_ = plainW.Close()
		}
		close(done)
	}()
	expected := raw[armorMetaPlaintextSHA]
	if expected == "" {
		expected = meta.PlaintextSHA
	}
	return plainR, func() { <-done }, expected, nil
}

func (fm *FormatMigrator) buildNewMetadata64(old map[string]string, iv, wrapped []byte, blockSize int, plaintextSize int64, sha []byte, fingerprint string) map[string]string {
	meta := make(map[string]string)
	for k, v := range old {
		if k != armorMetaVersion && !strings.HasPrefix(k, "x-amz-meta-armor-") {
			meta[k] = v
		}
	}
	meta[armorMetaVersion] = fmt.Sprintf("%d", fm.currentWriteVersion)
	meta[armorMetaWrappedDEK] = "v2:" + fingerprint + ":" + base64.StdEncoding.EncodeToString(wrapped)
	meta[armorMetaIV] = base64.StdEncoding.EncodeToString(iv)
	meta[armorMetaBlockSize] = fmt.Sprintf("%d", blockSize)
	meta[armorMetaPlaintextSize] = fmt.Sprintf("%d", plaintextSize)
	meta[armorMetaPlaintextSHA] = hex.EncodeToString(sha)
	return meta
}
