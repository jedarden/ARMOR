package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// ambiguousCompleteBackend models B2 finishing the object while ARMOR times
// out waiting for the response. The client's retry therefore receives
// NoSuchUpload even though the assembled object is already durable.
type ambiguousCompleteBackend struct {
	*recordingBackend
	corruptFinal bool
}

func (b *ambiguousCompleteBackend) CompleteMultipartUpload(
	ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart,
) (string, error) {
	if _, err := b.recordingBackend.CompleteMultipartUpload(
		ctx, bucket, key, uploadID, parts,
	); err != nil {
		return "", err
	}
	if b.corruptFinal {
		b.mu.Lock()
		b.objects[bucket+"/"+key] = append(b.objects[bucket+"/"+key], 0)
		b.mu.Unlock()
	}
	return "", &types.NoSuchUpload{}
}

func ambiguousCompletionTestHandler(
	t *testing.T, corruptFinal bool,
) (*ambiguousCompleteBackend, *handlers.Handlers) {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "test-secret-key",
		Prefix:        "tenant-a/",
	}
	rb := &ambiguousCompleteBackend{
		recordingBackend: newRecordingBackend(),
		corruptFinal:     corruptFinal,
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}
	h := handlers.New(
		cfg, rb, backend.NewMetadataCache(1000, 300),
		backend.NewFooterCache(1000, 300), km, nil,
	)
	return rb, h
}

func TestCompleteMultipartUploadRecoversAmbiguousBackendSuccess(t *testing.T) {
	_, h := ambiguousCompletionTestHandler(t, false)

	bucket, key := "test-bucket", "backups/base/data.tar"
	part := make([]byte, 5*1024*1024)
	for i := range part {
		part[i] = byte(i % 251)
	}
	uploadID := initiateMultipart(t, h, bucket, key)
	etag := uploadPart(t, h, bucket, key, uploadID, 1, part)
	completeMultipart(t, h, bucket, key, uploadID, []string{etag})

	// Recovery must finish the metadata copy, making the object readable rather
	// than merely accepting a raw ciphertext object that B2 assembled.
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after recovered completion: status %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.Bytes(); len(got) != len(part) {
		t.Fatalf("recovered object length=%d want=%d", len(got), len(part))
	}
	for i, value := range w.Body.Bytes() {
		if value != part[i] {
			t.Fatalf("recovered object differs at byte %d", i)
		}
	}
}

func TestCompleteMultipartUploadRejectsAmbiguousWrongSizedObject(t *testing.T) {
	rb, h := ambiguousCompletionTestHandler(t, true)
	bucket, key := "test-bucket", "backups/base/data.tar"
	part := make([]byte, 5*1024*1024)
	uploadID := initiateMultipart(t, h, bucket, key)
	etag := uploadPart(t, h, bucket, key, uploadID, 1, part)

	request := httptest.NewRequest(
		http.MethodPost,
		"/"+bucket+"/"+key+"?uploadId="+uploadID,
		bytes.NewBufferString("<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>"+etag+"</ETag></Part></CompleteMultipartUpload>"),
	)
	response := httptest.NewRecorder()
	h.HandleRoot(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("wrong-sized recovery status=%d want=500: %s", response.Code, response.Body.String())
	}

	rb.mu.Lock()
	metadata := rb.meta[bucket+"/tenant-a/"+key]
	rb.mu.Unlock()
	if metadata["x-amz-meta-armor-version"] != "" {
		t.Fatal("wrong-sized object was incorrectly finalized with ARMOR metadata")
	}
}
