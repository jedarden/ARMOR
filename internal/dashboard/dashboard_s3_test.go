// Package dashboard provides tests for S3 operations through the dashboard.
package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
)

// mockBackendWithACL implements backend.Backend with ACL-aware behavior.
type mockBackendWithACL struct {
	*mockBackend
	allowPut    bool
	allowGet    bool
	allowDelete bool
}

func (m *mockBackendWithACL) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if !m.allowPut {
		return errors.New("access denied: put not allowed")
	}
	return m.mockBackend.Put(ctx, bucket, key, body, size, meta)
}

func (m *mockBackendWithACL) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if !m.allowGet {
		return nil, nil, errors.New("access denied: get not allowed")
	}
	return m.mockBackend.Get(ctx, bucket, key)
}

func (m *mockBackendWithACL) Delete(ctx context.Context, bucket, key string) error {
	if !m.allowDelete {
		return errors.New("access denied: delete not allowed")
	}
	return m.mockBackend.Delete(ctx, bucket, key)
}

// TestUploadHandlerWithGetListOnlyCredential tests upload is denied with get+list-only credential.
func TestUploadHandlerWithGetListOnlyCredential(t *testing.T) {
	mb := &mockBackendWithACL{
		mockBackend: newMockBackend(),
		allowPut:    false, // get+list-only = no put
		allowGet:    true,
		allowDelete: false,
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "getlist-only",
		AccessKey: "test_key",
		SecretKey: "test_secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	// Create multipart upload request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write([]byte("test content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	d.uploadHandlerImpl()(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for upload with get+list-only credential, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("Expected 'access denied' error message, got: %s", rec.Body.String())
	}
}

// TestUploadHandlerWithFullCredential tests upload succeeds with full credential.
func TestUploadHandlerWithFullCredential(t *testing.T) {
	mb := &mockBackendWithACL{
		mockBackend: newMockBackend(),
		allowPut:    true, // full access
		allowGet:    true,
		allowDelete: true,
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "full-access",
		AccessKey: "full_key",
		SecretKey: "full_secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	// Create multipart upload request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "upload-test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write([]byte("upload test content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	d.uploadHandlerImpl()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for upload with full credential, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify object was uploaded
	if _, exists := mb.objects["upload-test.txt"]; !exists {
		t.Error("Object was not uploaded to backend")
	}
}

// TestDownloadHandlerWithGetListOnlyCredential tests download succeeds (get is allowed).
func TestDownloadHandlerWithGetListOnlyCredential(t *testing.T) {
	mb := &mockBackendWithACL{
		mockBackend: newMockBackend(),
		allowPut:    false,
		allowGet:    true, // get+list-only allows get
		allowDelete: false,
	}
	mb.objects["existing-file.txt"] = &backend.ObjectInfo{
		Key:          "existing-file.txt",
		Size:         100,
		ContentType:  "text/plain",
		LastModified: time.Now(),
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "getlist-only",
		AccessKey: "test_key",
		SecretKey: "test_secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/download?key=existing-file.txt", nil)
	rec := httptest.NewRecorder()

	d.downloadHandlerImpl()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for download with get+list-only credential, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}

	contentDisp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisp, "existing-file.txt") {
		t.Errorf("Expected Content-Disposition to contain filename, got %s", contentDisp)
	}
}

// TestDownloadHandlerNotFound tests download of non-existent object.
func TestDownloadHandlerNotFound(t *testing.T) {
	mb := &mockBackendWithACL{
		mockBackend: newMockBackend(),
		allowPut:    true,
		allowGet:    true,
		allowDelete: true,
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "full-access",
		AccessKey: "full_key",
		SecretKey: "full_secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/download?key=nonexistent.txt", nil)
	rec := httptest.NewRecorder()

	d.downloadHandlerImpl()(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent object, got %d", rec.Code)
	}
}

// TestDeleteHandlerWithGetListOnlyCredential tests delete is denied with get+list-only credential.
func TestDeleteHandlerWithGetListOnlyCredential(t *testing.T) {
	mb := &mockBackendWithACL{
		mockBackend: newMockBackend(),
		allowPut:    false,
		allowGet:    true,
		allowDelete: false, // get+list-only = no delete
	}
	mb.objects["to-delete.txt"] = &backend.ObjectInfo{
		Key:          "to-delete.txt",
		Size:         100,
		ContentType:  "text/plain",
		LastModified: time.Now(),
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "getlist-only",
		AccessKey: "test_key",
		SecretKey: "test_secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/delete?key=to-delete.txt", nil)
	rec := httptest.NewRecorder()

	d.deleteHandlerImpl()(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for delete with get+list-only credential, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("Expected 'access denied' error message, got: %s", rec.Body.String())
	}

	// Verify object still exists
	if _, exists := mb.objects["to-delete.txt"]; !exists {
		t.Error("Object should still exist after failed delete")
	}
}

// TestDeleteHandlerWithFullCredential tests delete succeeds with full credential.
func TestDeleteHandlerWithFullCredential(t *testing.T) {
	mb := &mockBackendWithACL{
		mockBackend: newMockBackend(),
		allowPut:    true,
		allowGet:    true,
		allowDelete: true, // full access allows delete
	}
	mb.objects["delete-me.txt"] = &backend.ObjectInfo{
		Key:          "delete-me.txt",
		Size:         100,
		ContentType:  "text/plain",
		LastModified: time.Now(),
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "full-access",
		AccessKey: "full_key",
		SecretKey: "full_secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/delete?key=delete-me.txt", nil)
	rec := httptest.NewRecorder()

	d.deleteHandlerImpl()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for delete with full credential, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify object was deleted
	if _, exists := mb.objects["delete-me.txt"]; exists {
		t.Error("Object should have been deleted")
	}
}

// TestUploadHandlerWithoutCredential tests upload fails when no credential configured.
func TestUploadHandlerWithoutCredential(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: nil, // No credential configured
		serverBaseURL: "http://localhost:9000",
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("test"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	d.uploadHandlerImpl()(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 without dashboard credential, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Dashboard credential not configured") {
		t.Errorf("Expected credential not configured message, got: %s", rec.Body.String())
	}
}

// TestDownloadHandlerWithoutCredential tests download fails when no credential configured.
func TestDownloadHandlerWithoutCredential(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: nil,
		serverBaseURL: "http://localhost:9000",
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard/download?key=test.txt", nil)
	rec := httptest.NewRecorder()

	d.downloadHandlerImpl()(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 without dashboard credential, got %d", rec.Code)
	}
}

// TestDeleteHandlerWithoutCredential tests delete fails when no credential configured.
func TestDeleteHandlerWithoutCredential(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: nil,
		serverBaseURL: "http://localhost:9000",
	}

	req := httptest.NewRequest(http.MethodDelete, "/dashboard/delete?key=test.txt", nil)
	rec := httptest.NewRecorder()

	d.deleteHandlerImpl()(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 without dashboard credential, got %d", rec.Code)
	}
}

// TestUploadHandlerMethodNotAllowed tests only POST is allowed for upload.
func TestUploadHandlerMethodNotAllowed(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/upload", nil)
	rec := httptest.NewRecorder()

	d.UploadHandler()(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for GET request, got %d", rec.Code)
	}
}

// TestDownloadHandlerMethodNotAllowed tests only GET is allowed for download.
func TestDownloadHandlerMethodNotAllowed(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/download", nil)
	rec := httptest.NewRecorder()

	d.DownloadHandler()(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST request, got %d", rec.Code)
	}
}

// TestDeleteHandlerAcceptsPost tests delete accepts both DELETE and POST.
func TestDeleteHandlerAcceptsPost(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test.txt"] = &backend.ObjectInfo{
		Key:          "test.txt",
		Size:         100,
		LastModified: time.Now(),
	}
	m := metrics.NewMetrics()

	cred := &DashboardCredential{
		Name:      "full",
		AccessKey: "key",
		SecretKey: "secret",
	}

	d := &Dashboard{
		backend:       mb,
		bucket:        "test-bucket",
		metrics:       m,
		dashboardCred: cred,
		serverBaseURL: "http://localhost:9000",
	}

	// Test POST method
	req := httptest.NewRequest(http.MethodPost, "/dashboard/delete", nil)
	req.PostForm = map[string][]string{"key": {"test.txt"}}
	rec := httptest.NewRecorder()

	d.deleteHandlerImpl()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for POST request, got %d", rec.Code)
	}
}

// TestCredentialActivityHandler tests the per-credential activity endpoint.
func TestCredentialActivityHandler(t *testing.T) {
	m := metrics.NewMetrics()

	// Simulate credential activity by incrementing metrics
	m.IncRequestsByCredential("cred1", "GET", "allow")
	m.IncRequestsByCredential("cred1", "GET", "allow")
	m.IncRequestsByCredential("cred1", "PUT", "allow")
	m.IncRequestsByCredential("cred1", "DELETE", "deny-acl")

	m.IncRequestsByCredential("cred2", "GET", "allow")
	m.IncRequestsByCredential("cred2", "PUT", "deny-auth")

	m.IncRequestsByCredential("unknown", "GET", "deny-auth")

	// Create a mock admin API server that returns credentials
	adminAPIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/creds" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// Return mock credential list
		w.Write([]byte(`[{"name":"cred1","acls":[],"source":"env","loaded_at":"2024-01-01T00:00:00Z"},{"name":"cred2","acls":[],"source":"file","loaded_at":"2024-01-01T00:00:00Z"}]`))
	}))
	defer adminAPIServer.Close()

	d := &Dashboard{
		backend: newMockBackend(),
		bucket:  "test-bucket",
		metrics: m,
	}

	adminClient := adminAPIServer.Client()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/dashboard/credential-activity", nil)
	rec := httptest.NewRecorder()

	d.credentialActivityHandlerImpl(adminClient, adminAPIServer.URL+"/admin/creds")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Parse response
	var activities []struct {
		Name       string `json:"name"`
		Source     string `json:"source"`
		TotalReqs  int64  `json:"total_requests"`
		AllowCount int64  `json:"allow_count"`
		DenyAuth   int64  `json:"deny_auth_count"`
		DenyACL    int64  `json:"deny_acl_count"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&activities); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify we have 3 credentials (cred1, cred2, and unknown)
	if len(activities) != 3 {
		t.Errorf("Expected 3 credentials, got %d", len(activities))
	}

	// Find cred1 and verify counts
	var cred1, cred2, unknown *struct {
		Name       string `json:"name"`
		Source     string `json:"source"`
		TotalReqs  int64  `json:"total_requests"`
		AllowCount int64  `json:"allow_count"`
		DenyAuth   int64  `json:"deny_auth_count"`
		DenyACL    int64  `json:"deny_acl_count"`
	}

	for i := range activities {
		switch activities[i].Name {
		case "cred1":
			cred1 = &activities[i]
		case "cred2":
			cred2 = &activities[i]
		case "unknown":
			unknown = &activities[i]
		}
	}

	if cred1 == nil {
		t.Fatal("cred1 not found in response")
	}
	if cred2 == nil {
		t.Fatal("cred2 not found in response")
	}
	if unknown == nil {
		t.Fatal("unknown not found in response")
	}

	// Verify cred1 counts (3 total: 2 allow GET, 1 allow PUT, 1 deny-acl DELETE)
	if cred1.TotalReqs != 4 {
		t.Errorf("cred1: expected total requests 4, got %d", cred1.TotalReqs)
	}
	if cred1.AllowCount != 3 {
		t.Errorf("cred1: expected allow count 3, got %d", cred1.AllowCount)
	}
	if cred1.DenyACL != 1 {
		t.Errorf("cred1: expected deny-acl count 1, got %d", cred1.DenyACL)
	}
	if cred1.Source != "env" {
		t.Errorf("cred1: expected source 'env', got '%s'", cred1.Source)
	}

	// Verify cred2 counts (2 total: 1 allow GET, 1 deny-auth PUT)
	if cred2.TotalReqs != 2 {
		t.Errorf("cred2: expected total requests 2, got %d", cred2.TotalReqs)
	}
	if cred2.AllowCount != 1 {
		t.Errorf("cred2: expected allow count 1, got %d", cred2.AllowCount)
	}
	if cred2.DenyAuth != 1 {
		t.Errorf("cred2: expected deny-auth count 1, got %d", cred2.DenyAuth)
	}
	if cred2.Source != "file" {
		t.Errorf("cred2: expected source 'file', got '%s'", cred2.Source)
	}

	// Verify unknown counts (1 total: 1 deny-auth GET)
	if unknown.TotalReqs != 1 {
		t.Errorf("unknown: expected total requests 1, got %d", unknown.TotalReqs)
	}
	if unknown.DenyAuth != 1 {
		t.Errorf("unknown: expected deny-auth count 1, got %d", unknown.DenyAuth)
	}
	if unknown.Source != "auth-failures" {
		t.Errorf("unknown: expected source 'auth-failures', got '%s'", unknown.Source)
	}
}
