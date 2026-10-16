package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVerifyRequest_SignatureComparison(t *testing.T) {
	const accessKey = "test-access-key"
	const secretKey = "test-secret-key"
	auth := NewSigV4Auth(accessKey, secretKey, "us-east-005")
	req := createSignedRequestForAuthTest(t, "GET", "/bucket/key", "", accessKey, secretKey, nil)

	if _, err := auth.VerifyRequest(req, nil); err != nil {
		t.Fatalf("valid signature was rejected: %v", err)
	}

	header := req.Header.Get("Authorization")
	marker := "Signature="
	start := strings.LastIndex(header, marker)
	if start < 0 {
		t.Fatal("authorization header has no signature")
	}
	start += len(marker)
	req.Header.Set("Authorization", header[:start]+changeLastByte(header[start:]))
	if _, err := auth.VerifyRequest(req, nil); err != ErrSignatureMismatch {
		t.Fatalf("one-byte signature difference returned %v, want ErrSignatureMismatch", err)
	}
}

func TestVerifyQueryAuth_SignatureComparison(t *testing.T) {
	const accessKey = "test-access-key"
	const secretKey = "test-secret-key"
	const region = "us-east-005"
	auth := NewSigV4Auth(accessKey, secretKey, region)
	date := formatAmzDate(time.Now().UTC())
	query := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {fmt.Sprintf("%s/%s/%s/s3/aws4_request", accessKey, date[:8], region)},
		"X-Amz-Date":          {date},
		"X-Amz-Expires":       {"3600"},
		"X-Amz-SignedHeaders": {"host"},
	}
	const host = "bucket.s3.us-east-005.backblazeb2.com"
	unsignedReq := httptest.NewRequest("GET", "/bucket/key?"+query.Encode(), nil)
	unsignedReq.Host = host
	signature := calculateQuerySignature(t, auth, secretKey, date, region, unsignedReq, []string{"host"})
	query.Set("X-Amz-Signature", signature)

	requestWithQuery := func() *http.Request {
		req := httptest.NewRequest("GET", "/bucket/key?"+query.Encode(), nil)
		req.Host = host
		return req
	}
	if _, err := auth.VerifyQueryAuth(requestWithQuery()); err != nil {
		t.Fatalf("valid presigned signature was rejected: %v", err)
	}
	query.Set("X-Amz-Signature", changeLastByte(signature))
	if _, err := auth.VerifyQueryAuth(requestWithQuery()); err != ErrSignatureMismatch {
		t.Fatalf("one-byte presigned signature difference returned %v, want ErrSignatureMismatch", err)
	}
}

func changeLastByte(value string) string {
	if len(value) == 0 {
		return value
	}
	changed := []byte(value)
	if changed[len(changed)-1] == '0' {
		changed[len(changed)-1] = '1'
	} else {
		changed[len(changed)-1] = '0'
	}
	return string(changed)
}
