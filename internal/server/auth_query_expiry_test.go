package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

func TestVerifyQueryAuthExpiresBounds(t *testing.T) {
	const (
		accessKey = "presign-test-key"
		secretKey = "presign-test-secret"
		region    = "us-east-005"
	)
	auth := NewSigV4Auth(accessKey, secretKey, region)
	server, err := New(&config.Config{
		Backend:     "filesystem",
		FSPath:      t.TempDir(),
		Bucket:      "test-bucket",
		B2Region:    region,
		Credentials: map[string]*config.Credential{accessKey: {AccessKey: accessKey, SecretKey: secretKey}},
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	for _, tc := range []struct {
		name       string
		expires    string
		include    bool
		dateOffset time.Duration
		wantErr    error
	}{
		{name: "absent", wantErr: ErrInvalidQueryExpires},
		{name: "empty", expires: "", include: true, wantErr: ErrInvalidQueryExpires},
		{name: "zero", expires: "0", include: true, wantErr: ErrInvalidQueryExpires},
		{name: "negative", expires: "-1", include: true, wantErr: ErrInvalidQueryExpires},
		{name: "malformed", expires: "nope", include: true, wantErr: ErrInvalidQueryExpires},
		{name: "too large", expires: "604801", include: true, wantErr: ErrInvalidQueryExpires},
		{name: "overflow", expires: "999999999999999999999999", include: true, wantErr: ErrInvalidQueryExpires},
		{name: "one second", expires: "1", include: true, dateOffset: time.Minute},
		{name: "seven days", expires: "604800", include: true},
		{name: "expired", expires: "1", include: true, dateOffset: -time.Minute, wantErr: ErrRequestExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date := formatAmzDate(time.Now().UTC().Add(tc.dateOffset))
			query := url.Values{
				"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
				"X-Amz-Credential":    {fmt.Sprintf("%s/%s/%s/s3/aws4_request", accessKey, date[:8], region)},
				"X-Amz-Date":          {date},
				"X-Amz-SignedHeaders": {"host"},
			}
			if tc.include {
				query.Set("X-Amz-Expires", tc.expires)
			}
			makeRequest := func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/test-bucket/key?"+query.Encode(), nil)
			}
			query.Set("X-Amz-Signature", calculateQuerySignature(t, auth, secretKey, date, region, makeRequest(), []string{"host"}))
			req := makeRequest()
			cred, err := auth.VerifyQueryAuth(req)
			if err != tc.wantErr {
				t.Fatalf("VerifyQueryAuth error = %v, want %v", err, tc.wantErr)
			}
			if err == nil && (cred == nil || cred.AccessKey != accessKey) {
				t.Fatalf("VerifyQueryAuth credential = %v, want %s", cred, accessKey)
			}
			if tc.wantErr != ErrInvalidQueryExpires {
				return
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, req)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "<Code>AuthorizationQueryParametersError</Code>") {
				t.Fatalf("HTTP response = %d %s, want 400 AuthorizationQueryParametersError", response.Code, response.Body.String())
			}
		})
	}
}
