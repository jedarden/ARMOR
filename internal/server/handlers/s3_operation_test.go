package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestS3OperationForRequest pins the operation labels armor_errors_total
// carries (ADR-008, docs/metrics.md). The values must stay the canonical S3
// operation names — dashboards key InvalidPartSize alerting on
// CompleteMultipartUpload, which the ADR-012 verb classifier cannot express.
// Each case mirrors a HandleRoot dispatch branch.
func TestS3OperationForRequest(t *testing.T) {
	cases := []struct {
		name string
		req  *http.Request
		want string
	}{
		{"get object", httptest.NewRequest(http.MethodGet, "/bucket/key.dat", nil), "GetObject"},
		{"head object", httptest.NewRequest(http.MethodHead, "/bucket/key.dat", nil), "HeadObject"},
		{"put object", httptest.NewRequest(http.MethodPut, "/bucket/key.dat", nil), "PutObject"},
		{"delete object", httptest.NewRequest(http.MethodDelete, "/bucket/key.dat", nil), "DeleteObject"},
		{"complete multipart", httptest.NewRequest(http.MethodPost, "/bucket/key.dat?uploadId=u", nil), "CompleteMultipartUpload"},
		{"create multipart", httptest.NewRequest(http.MethodPost, "/bucket/key.dat?uploads", nil), "CreateMultipartUpload"},
		{"upload part", httptest.NewRequest(http.MethodPut, "/bucket/key.dat?partNumber=1&uploadId=u", nil), "UploadPart"},
		{"abort multipart", httptest.NewRequest(http.MethodDelete, "/bucket/key.dat?uploadId=u", nil), "AbortMultipartUpload"},
		{"list parts", httptest.NewRequest(http.MethodGet, "/bucket/key.dat?uploadId=u", nil), "ListParts"},
		{"copy object", func() *http.Request {
			r := httptest.NewRequest(http.MethodPut, "/bucket/dst", nil)
			r.Header.Set("x-amz-copy-source", "/bucket/src")
			return r
		}(), "CopyObject"},
		{"list objects v2", httptest.NewRequest(http.MethodGet, "/bucket?list-type=2", nil), "ListObjectsV2"},
		{"list buckets", httptest.NewRequest(http.MethodGet, "/", nil), "ListBuckets"},
		{"head bucket", httptest.NewRequest(http.MethodHead, "/bucket", nil), "HeadBucket"},
		{"delete objects", httptest.NewRequest(http.MethodPost, "/bucket?delete", nil), "DeleteObjects"},
		{"unsupported method", httptest.NewRequest(http.MethodPatch, "/bucket/key", nil), ""},
	}
	for _, tc := range cases {
		if got := s3OperationForRequest(tc.req); got != tc.want {
			t.Errorf("%s: s3OperationForRequest = %q, want %q", tc.name, got, tc.want)
		}
	}
}
