package handlers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
)

func TestMultipartLocksRemovedAfterCompleteAndAbort(t *testing.T) {
	be, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	km, err := keymanager.New(bytes.Repeat([]byte{7}, 32), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := New(&config.Config{BlockSize: 65536, FormatWriteVersion: 3}, be, nil, nil, km, nil)
	const bucket = "test-bucket"
	const key = "object"
	makeUpload := func() string {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploads", nil)
		w := httptest.NewRecorder()
		h.CreateMultipartUpload(w, r, bucket, key)
		if w.Code != http.StatusOK {
			t.Fatalf("initiate: status %d: %s", w.Code, w.Body.String())
		}
		var result struct {
			UploadID string `xml:"UploadId"`
		}
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil || result.UploadID == "" {
			t.Fatalf("initiate response: upload ID %q, error %v", result.UploadID, err)
		}
		return result.UploadID
	}
	uploadPart := func(id string) string {
		t.Helper()
		r := httptest.NewRequest(http.MethodPut,
			fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", bucket, key, id), bytes.NewReader([]byte("part")))
		w := httptest.NewRecorder()
		h.UploadPart(w, r, bucket, key, id)
		if w.Code != http.StatusOK {
			t.Fatalf("upload part: status %d: %s", w.Code, w.Body.String())
		}
		return w.Header().Get("ETag")
	}
	assertEmpty := func(stage string) {
		t.Helper()
		h.multipartLocksMu.Lock()
		defer h.multipartLocksMu.Unlock()
		if len(h.multipartLocks) != 0 {
			t.Fatalf("%s: %d multipart locks remain", stage, len(h.multipartLocks))
		}
	}

	completedID := makeUpload()
	etag := uploadPart(completedID)
	completeBody := fmt.Sprintf("<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part></CompleteMultipartUpload>", etag)
	r := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploadId="+completedID, bytes.NewBufferString(completeBody))
	w := httptest.NewRecorder()
	h.CompleteMultipartUpload(w, r, bucket, key, completedID)
	if w.Code != http.StatusOK {
		t.Fatalf("complete: status %d: %s", w.Code, w.Body.String())
	}
	assertEmpty("complete")

	abortedID := makeUpload()
	uploadPart(abortedID)
	r = httptest.NewRequest(http.MethodDelete, "/"+bucket+"/"+key+"?uploadId="+abortedID, nil)
	w = httptest.NewRecorder()
	h.AbortMultipartUpload(w, r, bucket, key, abortedID)
	if w.Code != http.StatusNoContent {
		t.Fatalf("abort: status %d: %s", w.Code, w.Body.String())
	}
	assertEmpty("abort")
}

func TestMultipartLockCleanupKeepsActiveMutexAndSweepsIdle(t *testing.T) {
	h := &Handlers{}
	const key = "upload/part/1"
	release := h.multipartLock(key)
	h.cleanupMultipartLocks("upload")
	h.multipartLocksMu.Lock()
	entry := h.multipartLocks[key]
	h.multipartLocksMu.Unlock()
	if entry == nil {
		t.Fatal("cleanup replaced an active lock")
	}
	release()
	h.multipartLocksMu.Lock()
	_, exists := h.multipartLocks[key]
	h.multipartLocksMu.Unlock()
	if exists {
		t.Fatal("retired lock remained after release")
	}

	release = h.multipartLock(key)
	release()
	h.multipartLocksMu.Lock()
	h.multipartLocks[key].lastUsed = time.Now().Add(-multipartLockIdleTTL - time.Second)
	h.sweepMultipartLocksLocked(time.Now())
	_, exists = h.multipartLocks[key]
	h.multipartLocksMu.Unlock()
	if exists {
		t.Fatal("expired abandoned-upload lock remained after sweep")
	}
}
