package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/provenance"
)

func TestAuditEndpointWalksAndVerifiesFilesystemChain(t *testing.T) {
	ctx := context.Background()
	const (
		bucket     = "audit-bucket"
		prefix     = "tenant/"
		writerID   = "audit-writer"
		logicalKey = "data/file.txt"
	)

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("plaintext")))
	manager := provenance.NewManager(fs, bucket, writerID)
	if err := manager.RecordUpload(ctx, logicalKey, plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload: %v", err)
	}
	metadata := (&backend.ARMORMetadata{
		Version:      1,
		PlaintextSHA: plaintextSHA,
	}).ToMetadata()
	if err := fs.Put(ctx, bucket, prefix+logicalKey, strings.NewReader("ciphertext"), int64(len("ciphertext")), metadata); err != nil {
		t.Fatalf("put test object: %v", err)
	}

	s := &Server{
		backend: fs,
		config: &config.Config{
			Bucket: bucket,
			Prefix: prefix,
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder := httptest.NewRecorder()
	s.audit(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var result provenance.AuditResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Status != "valid" || result.TotalEntries != 1 || result.TotalObjects != 1 {
		t.Fatalf("unexpected valid audit response: %+v", result)
	}
	if len(result.Writers) != 1 || !result.Writers[0].Valid {
		t.Fatalf("writer chain was not verified: %+v", result.Writers)
	}

	// Alter a hash-bound field without updating the stored chain hash. The HTTP
	// endpoint must surface the cryptographic failure, including at sequence 1
	// where there is no newer entry to expose a broken link.
	entryKey := provenance.ChainPrefix + writerID + "/1.json"
	body, _, err := fs.GetDirect(ctx, bucket, entryKey)
	if err != nil {
		t.Fatalf("load chain entry: %v", err)
	}
	var entry provenance.Entry
	if err := json.NewDecoder(body).Decode(&entry); err != nil {
		body.Close()
		t.Fatalf("decode chain entry: %v", err)
	}
	body.Close()
	entry.PlaintextSHA256 = strings.Repeat("a", sha256.Size*2)
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal tampered entry: %v", err)
	}
	if err := fs.Put(ctx, bucket, entryKey, bytes.NewReader(entryJSON), int64(len(entryJSON)), map[string]string{"Content-Type": "application/json"}); err != nil {
		t.Fatalf("store tampered entry: %v", err)
	}

	request = httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder = httptest.NewRecorder()
	s.audit(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("invalid audit status code = %d, want 200", recorder.Code)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode invalid response: %v", err)
	}
	if result.Status != "invalid" || len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("tampered chain passed endpoint audit: %+v", result)
	}
}

func TestAuditEndpointRejectsNonGET(t *testing.T) {
	s := &Server{}
	request := httptest.NewRequest(http.MethodPost, "/armor/audit", nil)
	recorder := httptest.NewRecorder()

	s.audit(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
