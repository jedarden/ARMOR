package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// TestAbortMultipartUpload tests S3 AbortMultipartUpload
func TestAbortMultipartUpload(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a multipart upload
	req := httptest.NewRequest(http.MethodPost, "/test-bucket/test-abort.txt?uploads=", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d", w.Code)
	}

	// Parse upload ID from response
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse CreateMultipartUpload response: %v", err)
	}
	uploadID := result.UploadID

	// Abort the multipart upload
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/test-bucket/test-abort.txt?uploadId=%s", uploadID), nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for abort, got %d", w.Code)
	}

	// Verify state was cleaned up
	mb.mu.Lock()
	stateKey := fmt.Sprintf(".armor/multipart/%s.state", uploadID)
	_, exists := mb.objects[stateKey]
	mb.mu.Unlock()
	if exists {
		t.Error("multipart state should have been deleted")
	}
}

// TestListParts tests S3 ListParts operation

// TestListParts tests S3 ListParts operation
func TestListParts(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create a multipart upload
	req := httptest.NewRequest(http.MethodPost, "/test-bucket/test-list-parts.txt?uploads=", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d", w.Code)
	}

	// Parse upload ID from response
	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse CreateMultipartUpload response: %v", err)
	}
	uploadID := result.UploadID

	// Upload a part (must be block-aligned - multiple of 65536 bytes)
	// Use 5MiB which is a common part size and is block-aligned (5,242,880 % 65536 = 0)
	partContent := make([]byte, 5*1024*1024)
	for i := range partContent {
		partContent[i] = byte('A' + i%26)
	}
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/test-bucket/test-list-parts.txt?partNumber=1&uploadId=%s", uploadID), bytes.NewReader(partContent))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UploadPart failed: status %d", w.Code)
	}

	// List parts
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/test-bucket/test-list-parts.txt?uploadId=%s", uploadID), nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListParts failed: status %d", w.Code)
	}

	// Verify response is XML
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", contentType)
	}

	// Verify response contains ListPartsResult
	body := w.Body.String()
	if !strings.Contains(body, "ListPartsResult") {
		t.Error("response should contain ListPartsResult element")
	}
	if !strings.Contains(body, uploadID) {
		t.Error("response should contain upload ID")
	}
}

// TestListMultipartUploads tests S3 ListMultipartUploads operation

// TestListMultipartUploads tests S3 ListMultipartUploads operation
func TestListMultipartUploads(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create multiple multipart uploads
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/test-bucket/test-list-uploads-%d.txt?uploads=", i), nil)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("CreateMultipartUpload %d failed: status %d", i, w.Code)
		}
	}

	// List multipart uploads
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListMultipartUploads failed: status %d", w.Code)
	}

	// Verify response is XML
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", contentType)
	}

	// Verify response contains ListMultipartUploadsResult
	body := w.Body.String()
	if !strings.Contains(body, "ListMultipartUploadsResult") {
		t.Error("response should contain ListMultipartUploadsResult element")
	}
	if !strings.Contains(body, "test-bucket") {
		t.Error("response should contain bucket name")
	}
}

// TestListMultipartUploads_PrefixCaptureAndStrip tests that the prefix argument
// is correctly captured by the backend and that upload keys have the prefix
// stripped in the handler response when ARMOR_PREFIX is configured.

// TestListMultipartUploads_PrefixCaptureAndStrip tests that the prefix argument
// is correctly captured by the backend and that upload keys have the prefix
// stripped in the handler response when ARMOR_PREFIX is configured.
func TestListMultipartUploads_PrefixCaptureAndStrip(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "armor-prefix/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test with a client-provided prefix parameter
	clientPrefix := "uploads/"

	// List multipart uploads with prefix
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=&prefix="+clientPrefix, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify the mockBackend captured the expected prefix value
	// The handler should have prepended the ARMOR prefix to the client's prefix
	expectedBackendPrefix := "armor-prefix/" + clientPrefix
	if mb.capturedListMultipartUploadsPrefix != expectedBackendPrefix {
		t.Errorf("expected backend to receive prefix %q, got %q", expectedBackendPrefix, mb.capturedListMultipartUploadsPrefix)
	}

	// Parse the XML response
	var result struct {
		Uploads []struct {
			Key string `xml:"Key"`
		} `xml:"Upload"`
		NextKeyMarker string `xml:"NextKeyMarker"`
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Verify that upload keys have the ARMOR prefix stripped
	for _, upload := range result.Uploads {
		if strings.HasPrefix(upload.Key, "armor-prefix/") {
			t.Errorf("upload key %q still has ARMOR prefix, expected it to be stripped", upload.Key)
		}
		// The mock backend returns keys with the backend prefix prepended
		// so we should see the client prefix without the ARMOR prefix
		if upload.Key == "armor-prefix/"+clientPrefix+"test-multipart-upload.txt" {
			t.Errorf("upload key was not stripped correctly: got %q, want %s", upload.Key, clientPrefix+"test-multipart-upload.txt")
		}
	}

	// Verify that NextKeyMarker has the ARMOR prefix stripped
	if result.NextKeyMarker != "" && strings.HasPrefix(result.NextKeyMarker, "armor-prefix/") {
		t.Errorf("NextKeyMarker %q still has ARMOR prefix, expected it to be stripped", result.NextKeyMarker)
	}
}

// TestListMultipartUploads_NoPrefix tests that prefix capture and strip work
// correctly when no prefix is provided by the client.

// TestListMultipartUploads_NoPrefix tests that prefix capture and strip work
// correctly when no prefix is provided by the client.
func TestListMultipartUploads_NoPrefix(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "armor-prefix/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// List multipart uploads without prefix parameter
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify the mockBackend captured the ARMOR prefix (with empty client prefix)
	expectedBackendPrefix := "armor-prefix/"
	if mb.capturedListMultipartUploadsPrefix != expectedBackendPrefix {
		t.Errorf("expected backend to receive prefix %q, got %q", expectedBackendPrefix, mb.capturedListMultipartUploadsPrefix)
	}

	// Parse the XML response
	var result struct {
		Uploads []struct {
			Key string `xml:"Key"`
		} `xml:"Upload"`
		NextKeyMarker string `xml:"NextKeyMarker"`
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Verify that upload keys have the ARMOR prefix stripped
	for _, upload := range result.Uploads {
		if strings.HasPrefix(upload.Key, "armor-prefix/") {
			t.Errorf("upload key %q still has ARMOR prefix, expected it to be stripped", upload.Key)
		}
	}

	// Verify that NextKeyMarker has the ARMOR prefix stripped
	if result.NextKeyMarker != "" && strings.HasPrefix(result.NextKeyMarker, "armor-prefix/") {
		t.Errorf("NextKeyMarker %q still has ARMOR prefix, expected it to be stripped", result.NextKeyMarker)
	}
}

// TestListMultipartUploads_NoArmorPrefix tests that behavior is correct when
// ARMOR_PREFIX is not configured (prefix should not be added or stripped).

// TestListMultipartUploads_NoArmorPrefix tests that behavior is correct when
// ARMOR_PREFIX is not configured (prefix should not be added or stripped).
func TestListMultipartUploads_NoArmorPrefix(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "", // No ARMOR prefix configured
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	clientPrefix := "data/"

	// List multipart uploads with prefix
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=&prefix="+clientPrefix, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Verify the mockBackend captured the client prefix unchanged (no ARMOR prefix added)
	if mb.capturedListMultipartUploadsPrefix != clientPrefix {
		t.Errorf("expected backend to receive prefix %q, got %q", clientPrefix, mb.capturedListMultipartUploadsPrefix)
	}

	// Parse the XML response
	var result struct {
		Uploads []struct {
			Key string `xml:"Key"`
		} `xml:"Upload"`
		NextKeyMarker string `xml:"NextKeyMarker"`
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Verify that upload keys are returned as-is (no stripping happened)
	// The mock returns keys with the client prefix prepended
	for _, upload := range result.Uploads {
		if !strings.HasPrefix(upload.Key, clientPrefix) {
			t.Errorf("upload key %q should start with client prefix %q", upload.Key, clientPrefix)
		}
	}
}

// multipartUploadsListCaptureBackend wraps mockBackend and records the arguments passed
// to ListMultipartUploads, so tests can assert the handler forwards the correct
// (configured + client) prefix to the backend.

// multipartUploadsListCaptureBackend wraps mockBackend and records the arguments passed
// to ListMultipartUploads, so tests can assert the handler forwards the correct
// (configured + client) prefix to the backend.
type multipartUploadsListCaptureBackend struct {
	*mockBackend
	mu                 sync.Mutex
	listMultipartCalls int
	gotPrefix          string
}

func (c *multipartUploadsListCaptureBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	c.mu.Lock()
	c.listMultipartCalls++
	c.gotPrefix = prefix
	c.mu.Unlock()
	return c.mockBackend.ListMultipartUploads(ctx, bucket, prefix)
}

// lastPrefix returns the prefix argument from the most recent ListMultipartUploads call.

// lastPrefix returns the prefix argument from the most recent ListMultipartUploads call.
func (c *multipartUploadsListCaptureBackend) lastPrefix() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gotPrefix
}

// multipartUploadsMarkerStubBackend returns a fixed, stored-space upload set
// from ListMultipartUploads so the handler-side key-marker/upload-id-marker
// filtering and max-uploads truncation can be asserted without a live backend.
// Keys carry the configured prefix so the marker composition and strip
// round-trip is exercised alongside the filter itself. Each call receives a
// fresh copy of the fixed set: the handler filters in place over
// result.Uploads, so a shared backing array would be mutated across calls.

// multipartUploadsMarkerStubBackend returns a fixed, stored-space upload set
// from ListMultipartUploads so the handler-side key-marker/upload-id-marker
// filtering and max-uploads truncation can be asserted without a live backend.
// Keys carry the configured prefix so the marker composition and strip
// round-trip is exercised alongside the filter itself. Each call receives a
// fresh copy of the fixed set: the handler filters in place over
// result.Uploads, so a shared backing array would be mutated across calls.
type multipartUploadsMarkerStubBackend struct {
	*mockBackend
	mu      sync.Mutex
	uploads []backend.UploadInfo
	calls   int
}

func (s *multipartUploadsMarkerStubBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	s.mu.Lock()
	s.calls++
	uploads := append([]backend.UploadInfo(nil), s.uploads...)
	s.mu.Unlock()
	return &backend.ListMultipartUploadsResult{
		Bucket:      bucket,
		Uploads:     uploads,
		IsTruncated: false,
	}, nil
}

// TestListMultipartUploads_KeyMarkerExcludesMarkedKey verifies S3 key-marker
// semantics on a prefixed deployment: with key-marker set and no
// upload-id-marker, uploads whose key equals the marker are excluded — only
// keys lexicographically greater are returned (armor-4e5b7a10). This is the
// same assertion TestListMultipartUploads_WithKeyMarker makes against a live
// prefixed endpoint in the armor-integration gate.

// TestListMultipartUploads_KeyMarkerExcludesMarkedKey verifies S3 key-marker
// semantics on a prefixed deployment: with key-marker set and no
// upload-id-marker, uploads whose key equals the marker are excluded — only
// keys lexicographically greater are returned (armor-4e5b7a10). This is the
// same assertion TestListMultipartUploads_WithKeyMarker makes against a live
// prefixed endpoint in the armor-integration gate.
func TestListMultipartUploads_KeyMarkerExcludesMarkedKey(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "vault/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	sb := &multipartUploadsMarkerStubBackend{
		mockBackend: mb,
		uploads: []backend.UploadInfo{
			{UploadID: "id-a", Key: "vault/pfx/m-a", Initiated: time.Now()},
			{UploadID: "id-b", Key: "vault/pfx/m-b", Initiated: time.Now()},
			{UploadID: "id-c", Key: "vault/pfx/m-c", Initiated: time.Now()},
		},
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, sb, cache, footerCache, km, nil)

	// The client marks the middle key in client space; the handler must compose
	// the configured prefix onto it and exclude the marked key itself.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=&prefix=pfx/&key-marker=pfx/m-b", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Uploads []struct {
			Key string `xml:"Key"`
		} `xml:"Upload"`
	}
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	if len(result.Uploads) != 1 {
		t.Fatalf("ListMultipartUploads returned %d uploads (%v), want exactly the one key strictly after the marker", len(result.Uploads), result.Uploads)
	}
	if got, want := result.Uploads[0].Key, "pfx/m-c"; got != want {
		t.Errorf("ListMultipartUploads key-marker=pfx/m-b returned key %q, want %q", got, want)
	}
}

// TestListMultipartUploads_KeyMarkerWithUploadIDMarker verifies the paired
// cursor form: with key-marker AND upload-id-marker, uploads for the marked
// key return when their upload ID is lexicographically greater than the
// upload-id-marker, alongside every key strictly greater than the key-marker.

// TestListMultipartUploads_KeyMarkerWithUploadIDMarker verifies the paired
// cursor form: with key-marker AND upload-id-marker, uploads for the marked
// key return when their upload ID is lexicographically greater than the
// upload-id-marker, alongside every key strictly greater than the key-marker.
func TestListMultipartUploads_KeyMarkerWithUploadIDMarker(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "vault/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	sb := &multipartUploadsMarkerStubBackend{
		mockBackend: mb,
		uploads: []backend.UploadInfo{
			{UploadID: "id-b1", Key: "vault/pfx/m-b", Initiated: time.Now()},
			{UploadID: "id-b2", Key: "vault/pfx/m-b", Initiated: time.Now()},
			{UploadID: "id-c", Key: "vault/pfx/m-c", Initiated: time.Now()},
		},
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, sb, cache, footerCache, km, nil)

	// The client resumes from the middle key with a specific upload ID; the
	// handler must exclude uploads of the marked key at or before that ID while
	// keeping the same key's greater upload IDs and all greater keys.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=&prefix=pfx/&key-marker=pfx/m-b&upload-id-marker=id-b1", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Uploads []struct {
			Key      string `xml:"Key"`
			UploadID string `xml:"UploadId"`
		} `xml:"Upload"`
	}
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	want := []struct {
		key      string
		uploadID string
	}{
		{"pfx/m-b", "id-b2"},
		{"pfx/m-c", "id-c"},
	}
	if len(result.Uploads) != len(want) {
		t.Fatalf("ListMultipartUploads returned %d uploads (%v), want %d", len(result.Uploads), result.Uploads, len(want))
	}
	for i, expected := range want {
		if got := result.Uploads[i].Key; got != expected.key {
			t.Errorf("upload %d key = %q, want %q", i, got, expected.key)
		}
		if got := result.Uploads[i].UploadID; got != expected.uploadID {
			t.Errorf("upload %d UploadId = %q, want %q", i, got, expected.uploadID)
		}
	}
}

// TestListMultipartUploads_MaxUploadsTruncationEcho verifies handler-side
// max-uploads truncation returns S3-shaped cursor fields: IsTruncated=true,
// NextKeyMarker stripped to client space, NextUploadIDMarker set from the last
// returned upload — so a client can resume with key-marker + upload-id-marker.

// TestListMultipartUploads_MaxUploadsTruncationEcho verifies handler-side
// max-uploads truncation returns S3-shaped cursor fields: IsTruncated=true,
// NextKeyMarker stripped to client space, NextUploadIDMarker set from the last
// returned upload — so a client can resume with key-marker + upload-id-marker.
func TestListMultipartUploads_MaxUploadsTruncationEcho(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "vault/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	sb := &multipartUploadsMarkerStubBackend{
		mockBackend: mb,
		uploads: []backend.UploadInfo{
			{UploadID: "id-a", Key: "vault/pfx/m-a", Initiated: time.Now()},
			{UploadID: "id-b", Key: "vault/pfx/m-b", Initiated: time.Now()},
			{UploadID: "id-c", Key: "vault/pfx/m-c", Initiated: time.Now()},
		},
	}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, sb, cache, footerCache, km, nil)

	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=&prefix=pfx/&max-uploads=2", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Uploads []struct {
			Key      string `xml:"Key"`
			UploadID string `xml:"UploadId"`
		} `xml:"Upload"`
		NextKeyMarker      string `xml:"NextKeyMarker"`
		NextUploadIDMarker string `xml:"NextUploadIdMarker"`
		IsTruncated        bool   `xml:"IsTruncated"`
	}
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	if len(result.Uploads) != 2 {
		t.Fatalf("ListMultipartUploads returned %d uploads, want 2 under max-uploads=2", len(result.Uploads))
	}
	if got, want := result.Uploads[1].Key, "pfx/m-b"; got != want {
		t.Errorf("last returned upload key = %q, want %q", got, want)
	}
	if !result.IsTruncated {
		t.Error("IsTruncated = false, want true when results exceed max-uploads")
	}
	if got, want := result.NextKeyMarker, "pfx/m-b"; got != want {
		t.Errorf("NextKeyMarker = %q, want %q in client space (configured prefix stripped)", got, want)
	}
	if got, want := result.NextUploadIDMarker, "id-b"; got != want {
		t.Errorf("NextUploadIdMarker = %q, want %q", got, want)
	}
}

// TestListMultipartUploads_PrefixEndToEnd verifies the full prefix round-trip when
// a client supplies a prefix on top of a configured ARMOR_PREFIX:
//  1. the client prefix is composed with the configured prefix and passed to the backend,
//  2. the backend filters using that composed prefix,
//  3. the configured prefix is stripped from the returned upload keys and NextKeyMarker,
//  4. the client sees only its own unprefixed keys.
//
// This test must FAIL if applyPrefix() or stripPrefix() is removed — it is a regression
// tripwire, not a vacuous assertion.

// TestListMultipartUploads_PrefixEndToEnd verifies the full prefix round-trip when
// a client supplies a prefix on top of a configured ARMOR_PREFIX:
//  1. the client prefix is composed with the configured prefix and passed to the backend,
//  2. the backend filters using that composed prefix,
//  3. the configured prefix is stripped from the returned upload keys and NextKeyMarker,
//  4. the client sees only its own unprefixed keys.
//
// This test must FAIL if applyPrefix() or stripPrefix() is removed — it is a regression
// tripwire, not a vacuous assertion.
func TestListMultipartUploads_PrefixEndToEnd(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		Prefix:        "vault/",
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackend()
	cb := &multipartUploadsListCaptureBackend{mockBackend: mb}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, cb, cache, footerCache, km, nil)

	// The mock backend returns uploads with keys constructed from whatever prefix it receives.
	// When we pass a client prefix "uploads/", the backend should receive "vault/uploads/"
	// (configured + client), and the response should have "vault/" stripped so the client
	// sees only "uploads/test-multipart-upload.txt".

	clientPrefix := "uploads/"

	// List multipart uploads with prefix
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?uploads=&prefix="+clientPrefix, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListMultipartUploads failed: status %d, body: %s", w.Code, w.Body.String())
	}

	// Criterion 1: the composed prefix (configured + client) reached the backend.
	if got, want := cb.lastPrefix(), "vault/uploads/"; got != want {
		t.Errorf("backend received prefix %q, want %q (configured + client)", got, want)
	}

	// Parse the XML response.
	var result struct {
		Uploads []struct {
			Key string `xml:"Key"`
		} `xml:"Upload"`
		NextKeyMarker string `xml:"NextKeyMarker"`
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse XML: %v\nResponse: %s", err, string(body))
	}

	// Criterion 2: every returned upload key is unprefixed (no "vault/" in client-visible keys).
	for _, upload := range result.Uploads {
		if strings.HasPrefix(upload.Key, "vault/") {
			t.Errorf("upload key %q still carries the configured prefix", upload.Key)
		}
		// The mock backend returns keys with the full backend prefix "vault/uploads/",
		// so after stripping we should see "uploads/test-multipart-upload.txt"
		if upload.Key == "vault/uploads/test-multipart-upload.txt" {
			t.Errorf("upload key was not stripped: got %q, want %q", upload.Key, "uploads/test-multipart-upload.txt")
		}
	}

	// Verify we got the expected unprefixed key
	expectedKey := "uploads/test-multipart-upload.txt"
	found := false
	for _, upload := range result.Uploads {
		if upload.Key == expectedKey {
			found = true
			break
		}
	}
	if !found {
		keys := make([]string, len(result.Uploads))
		for i, u := range result.Uploads {
			keys[i] = u.Key
		}
		t.Errorf("expected unprefixed key %q in response, got keys: %v", expectedKey, keys)
	}

	// Criterion 3: NextKeyMarker has the configured prefix stripped.
	if result.NextKeyMarker != "" && strings.HasPrefix(result.NextKeyMarker, "vault/") {
		t.Errorf("NextKeyMarker %q still carries the configured prefix", result.NextKeyMarker)
	}
	// The mock returns NextKeyMarker with the full backend prefix, so after stripping
	// we should see "uploads/next-upload-marker.txt"
	if result.NextKeyMarker != "" && result.NextKeyMarker == "vault/uploads/next-upload-marker.txt" {
		t.Errorf("NextKeyMarker was not stripped: got %q, want %q", result.NextKeyMarker, "uploads/next-upload-marker.txt")
	}
}

// TestAbortMultipartUploadNotFound tests aborting a non-existent upload

// TestAbortMultipartUploadNotFound tests aborting a non-existent upload
func TestAbortMultipartUploadNotFound(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Try to abort a non-existent upload
	req := httptest.NewRequest(http.MethodDelete, "/test-bucket/test.txt?uploadId=nonexistent-upload-id", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent upload, got %d", w.Code)
	}
}

// TestListPartsNotFound tests listing parts of a non-existent upload

// TestListPartsNotFound tests listing parts of a non-existent upload
func TestListPartsNotFound(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Try to list parts of a non-existent upload
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt?uploadId=nonexistent-upload-id", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent upload, got %d", w.Code)
	}
}

// TestEmptyBucketRendering tests that listing an empty bucket returns valid XML
// with no objects and doesn't cause any errors.
