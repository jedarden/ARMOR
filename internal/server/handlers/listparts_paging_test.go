package handlers_test

import (
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// listPartsPagingStubBackend returns a fixed part set from ListParts so the
// handler-side paging (max-parts truncation, part-number-marker exclusion,
// IsTruncated + NextPartNumberMarker cursor, response echo) can be asserted
// without a live backend, the same way multipartUploadsMarkerStubBackend
// serves ListMultipartUploads. The fixed set is deliberately unsorted: S3
// ListParts pages parts in ascending part-number order, so the handler's
// sort is part of the contract under test. Each call receives a fresh copy
// of the fixed set: the handler filters in place over result.Parts
// (filtered := result.Parts[:0]), so a shared backing array would be
// mutated across calls.
type listPartsPagingStubBackend struct {
	*mockBackend
	mu    sync.Mutex
	parts []backend.PartInfo
	calls int
}

func (s *listPartsPagingStubBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	s.mu.Lock()
	s.calls++
	parts := append([]backend.PartInfo(nil), s.parts...)
	s.mu.Unlock()
	return &backend.ListPartsResult{
		Bucket:      bucket,
		Key:         key,
		UploadID:    uploadID,
		Parts:       parts,
		IsTruncated: false,
	}, nil
}

// listPartsPage mirrors the ListPartsResult document the handler renders,
// with the paging fields the S3 contract exposes.
type listPartsPage struct {
	PartNumberMarker     int                 `xml:"PartNumberMarker"`
	NextPartNumberMarker int                 `xml:"NextPartNumberMarker"`
	MaxParts             int                 `xml:"MaxParts"`
	IsTruncated          bool                `xml:"IsTruncated"`
	Parts                []listPartsPagePart `xml:"Part"`
}

type listPartsPagePart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
	Size       int64  `xml:"Size"`
}

// newListPartsPagingHarness builds a handler whose backend reports a live
// five-part upload (parts 1..5, deliberately out of order in stored space)
// and returns the upload ID created through the real CreateMultipartUpload
// path, so the v2 multipart state the ListParts handler loads exists.
func newListPartsPagingHarness(t *testing.T) (*handlers.Handlers, string) {
	t.Helper()

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}
	parts := make([]backend.PartInfo, 0, 5)
	for _, n := range []int32{5, 1, 4, 2, 3} {
		parts = append(parts, backend.PartInfo{
			PartNumber:   n,
			ETag:         fmt.Sprintf("etag-%d", n),
			Size:         int64(n) * 1024,
			LastModified: time.Now(),
		})
	}
	sb := &listPartsPagingStubBackend{mockBackend: newMockBackend(), parts: parts}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	h := handlers.New(cfg, sb, cache, footerCache, km, nil)

	req := httptest.NewRequest(http.MethodPost, "/test-bucket/test-paging.bin?uploads=", nil)
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d, body: %s", w.Code, w.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse CreateMultipartUpload response: %v", err)
	}
	if created.UploadID == "" {
		t.Fatal("CreateMultipartUpload returned an empty upload ID")
	}
	return h, created.UploadID
}

// listPartsPageOf issues a ListParts request with the given query parameters
// and decodes the ListPartsResult document. A non-200 response fails the test.
func listPartsPageOf(t *testing.T, h *handlers.Handlers, uploadID, query string) listPartsPage {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/test-bucket/test-paging.bin?uploadId=%s&%s", uploadID, query), nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListParts (%s) failed: status %d, body: %s", query, w.Code, w.Body.String())
	}
	var page listPartsPage
	if err := xml.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("failed to parse ListParts response (%s): %v", query, err)
	}
	return page
}

// listPartsPageNumbers returns the part numbers in response order.
func listPartsPageNumbers(page listPartsPage) []int {
	nums := make([]int, 0, len(page.Parts))
	for _, part := range page.Parts {
		nums = append(nums, part.PartNumber)
	}
	return nums
}

// assertListPartsPageEquals compares a page's part numbers and cursor fields
// against the expected page shape.
func assertListPartsPageEquals(t *testing.T, page listPartsPage, wantParts []int, wantTruncated bool, wantNextMarker int) {
	t.Helper()

	gotParts := listPartsPageNumbers(page)
	if !reflect.DeepEqual(gotParts, wantParts) {
		t.Errorf("page returned parts %v, want %v", gotParts, wantParts)
	}
	if page.IsTruncated != wantTruncated {
		t.Errorf("IsTruncated = %v, want %v", page.IsTruncated, wantTruncated)
	}
	if page.NextPartNumberMarker != wantNextMarker {
		t.Errorf("NextPartNumberMarker = %d, want %d", page.NextPartNumberMarker, wantNextMarker)
	}
}

// TestListPartsPaging_MaxPartsTruncatesAndCarriesCursor pins the S3 max-parts
// truncation the armor-integration TestListParts leg exercises
// (tests/integration/s3_operations_test.go: "MaxParts=2 returned all 5
// parts"): a five-part upload listed with max-parts=2 must return exactly the
// first two parts in ascending part-number order, mark the page truncated,
// and carry the cursor for the next page in NextPartNumberMarker.
func TestListPartsPaging_MaxPartsTruncatesAndCarriesCursor(t *testing.T) {
	h, uploadID := newListPartsPagingHarness(t)

	page := listPartsPageOf(t, h, uploadID, "max-parts=2")

	assertListPartsPageEquals(t, page, []int{1, 2}, true, 2)
	if page.MaxParts != 2 {
		t.Errorf("MaxParts echo = %d, want 2", page.MaxParts)
	}
	if page.PartNumberMarker != 0 {
		t.Errorf("PartNumberMarker echo = %d, want 0", page.PartNumberMarker)
	}
}

// TestListPartsPaging_FollowUpPageFromMarkerReturnsLaterParts pins the
// cursor walk: paging from the NextPartNumberMarker page 1 handed back must
// resume after the marked part, not restart or 404, until the upload is
// exhausted with IsTruncated=false.
func TestListPartsPaging_FollowUpPageFromMarkerReturnsLaterParts(t *testing.T) {
	h, uploadID := newListPartsPagingHarness(t)

	page2 := listPartsPageOf(t, h, uploadID, "max-parts=2&part-number-marker=2")
	assertListPartsPageEquals(t, page2, []int{3, 4}, true, 4)
	if page2.PartNumberMarker != 2 {
		t.Errorf("PartNumberMarker echo = %d, want 2", page2.PartNumberMarker)
	}

	page3 := listPartsPageOf(t, h, uploadID, "max-parts=2&part-number-marker=4")
	assertListPartsPageEquals(t, page3, []int{5}, false, 5)
}

// TestListPartsPaging_PartNumberMarkerOnLiveUpload pins the armor-integration
// TestListParts leg that used to fail with 404 NoSuchUpload for a live
// upload (tests/integration/s3_operations_test.go: "ListParts with
// PartNumberMarker returned 404 NoSuchUpload"): part-number-marker on an
// in-flight upload must return 200 and only the parts after the marker,
// echoing the request marker back.
func TestListPartsPaging_PartNumberMarkerOnLiveUpload(t *testing.T) {
	h, uploadID := newListPartsPagingHarness(t)

	page := listPartsPageOf(t, h, uploadID, "part-number-marker=3")

	assertListPartsPageEquals(t, page, []int{4, 5}, false, 5)
	if page.PartNumberMarker != 3 {
		t.Errorf("PartNumberMarker echo = %d, want 3", page.PartNumberMarker)
	}
	if page.MaxParts != 1000 {
		t.Errorf("MaxParts echo = %d, want the 1000 default", page.MaxParts)
	}
}

// TestListPartsPaging_UnknownUploadIDReturnsNoSuchUpload pins the failure
// code for an upload ID that does not exist, marker present or not: the S3
// contract is NoSuchUpload (404), which is what the armor-integration leg
// saw wrongly for a LIVE upload before the paging fix.
func TestListPartsPaging_UnknownUploadIDReturnsNoSuchUpload(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test-paging.bin?uploadId=no-such-upload&part-number-marker=2", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ListParts for an unknown upload ID: status %d, want 404, body: %s", w.Code, w.Body.String())
	}
	var errResp struct {
		Code string `xml:"Code"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp.Code != "NoSuchUpload" {
		t.Errorf("error code = %q, want NoSuchUpload", errResp.Code)
	}
}
