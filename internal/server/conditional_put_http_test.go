package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
)

type countingRequestBody struct {
	reader io.Reader
	reads  int
}

func (b *countingRequestBody) Read(p []byte) (int, error) {
	b.reads++
	return b.reader.Read(p)
}

func signedPutRequest(t *testing.T, method, rawURL, accessKey, secretKey string, body io.Reader, bodyHash string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-Amz-Content-Sha256", bodyHash)
	if err := v4.NewSigner().SignHTTP(
		context.Background(),
		aws.Credentials{AccessKeyID: accessKey, SecretAccessKey: secretKey},
		req,
		bodyHash,
		"s3",
		"us-east-005",
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("sign request: %v", err)
	}
	return req
}

func TestConditionalPutExpectContinueRejectsBeforeBodyRead(t *testing.T) {
	const (
		bucket    = "conditional-bucket"
		accessKey = "conditional-access"
		secretKey = "conditional-secret"
	)
	storage, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}
	cfg := &config.Config{
		Backend:     "filesystem",
		FSPath:      t.TempDir(),
		Bucket:      bucket,
		B2Region:    "us-east-005",
		Credentials: map[string]*config.Credential{accessKey: {AccessKey: accessKey, SecretKey: secretKey}},
		MEK:         bytes.Repeat([]byte{0x11}, 32),
		BlockSize:   65536,
	}
	srv, err := NewWithBackend(cfg, storage)
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}
	var logs strings.Builder
	srv.logger.SetOutput(&logs)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	seed := []byte("seed object")
	seedHash := sha256.Sum256(seed)
	seedReq := signedPutRequest(t, http.MethodPut, ts.URL+"/"+bucket+"/immutable", accessKey, secretKey, bytes.NewReader(seed), hex.EncodeToString(seedHash[:]))
	seedResp, err := http.DefaultClient.Do(seedReq)
	if err != nil {
		t.Fatalf("send seed PUT: %v", err)
	}
	seedResp.Body.Close()
	if seedResp.StatusCode != http.StatusOK {
		t.Fatalf("seed PUT status = %d, want 200", seedResp.StatusCode)
	}

	payload := bytes.Repeat([]byte("x"), 24*1024)
	payloadHash := sha256.Sum256(payload)
	body := &countingRequestBody{reader: bytes.NewReader(payload)}
	req := signedPutRequest(t, http.MethodPut, ts.URL+"/"+bucket+"/immutable", accessKey, secretKey, body, hex.EncodeToString(payloadHash[:]))
	req.ContentLength = int64(len(payload))
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("Expect", "100-continue")

	client := &http.Client{Transport: &http.Transport{ExpectContinueTimeout: 5 * time.Second}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send conditional PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("conditional PUT status = %d, want 412", resp.StatusCode)
	}
	if body.reads != 0 {
		t.Fatalf("client body was read %d times despite early 412", body.reads)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, `"status":412`) {
		t.Fatalf("access log missing 412 response:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, `"bytes_in":0`) {
		t.Fatalf("early 412 access log did not report bytes_in=0:\n%s", logOutput)
	}
}
