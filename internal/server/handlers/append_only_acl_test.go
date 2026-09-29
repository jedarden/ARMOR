package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/presign"
	armorserver "github.com/jedarden/armor/internal/server"
)

const (
	appendOnlyAccessKey = "APPEND-ONLY-ACCESS"
	appendOnlySecretKey = "append-only-secret-for-tests"
	noReadAccessKey     = "PUT-LIST-ONLY-ACCESS"
	noReadSecretKey     = "put-list-only-secret-for-tests"
	testBucket          = "append-only-bucket"
	adminToken          = "append-only-admin-token"
)

type appendOnlyFixture struct {
	s3URL     string
	adminURL  string
	server    *armorserver.Server
	appendKey string
}

func newAppendOnlyFixture(t *testing.T) *appendOnlyFixture {
	t.Helper()

	storage, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}

	credentials := map[string]*config.Credential{
		appendOnlyAccessKey: {
			AccessKey: appendOnlyAccessKey,
			SecretKey: appendOnlySecretKey,
			ACLs: []acl.ACLEntry{{
				Bucket:  testBucket,
				Prefix:  "append/",
				Actions: map[string]bool{acl.ActionGet: true, acl.ActionPut: true, acl.ActionList: true},
			}},
		},
		noReadAccessKey: {
			AccessKey: noReadAccessKey,
			SecretKey: noReadSecretKey,
			ACLs: []acl.ACLEntry{{
				Bucket:  testBucket,
				Prefix:  "append/",
				Actions: map[string]bool{acl.ActionPut: true, acl.ActionList: true},
			}},
		},
	}

	cfg := &config.Config{
		Bucket:         testBucket,
		B2Region:       "us-east-005",
		BlockSize:      65536,
		MEK:            bytes.Repeat([]byte{0x11}, 32),
		Credentials:    credentials,
		PresignEnabled: true,
		PresignSecret:  bytes.Repeat([]byte{0x22}, 32),
		AdminToken:     adminToken,
	}

	srv, err := armorserver.NewWithBackend(cfg, storage)
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}

	// Seed a pre-existing object outside the S3 encryption path. This keeps the
	// overwrite assertion focused on append-only authorization, while the new
	// key below exercises the real encrypted PUT and GET handlers.
	const existingBody = "original object"
	if err := storage.Put(context.Background(), testBucket, "append/existing.txt", bytes.NewReader([]byte(existingBody)), int64(len(existingBody)), map[string]string{"Content-Type": "text/plain"}); err != nil {
		t.Fatalf("seed existing object: %v", err)
	}

	s3Server := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Server.Close)
	srv.SetPresigner(presign.NewSigner(cfg.PresignSecret, s3Server.URL+"/share"))
	adminServer := httptest.NewServer(srv.AdminHandler())
	t.Cleanup(adminServer.Close)

	return &appendOnlyFixture{
		s3URL:     s3Server.URL,
		adminURL:  adminServer.URL,
		server:    srv,
		appendKey: "append/existing.txt",
	}
}

func signedACLRequest(t *testing.T, method, rawURL, accessKey, secretKey string, body []byte) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	payloadHash := sha256.Sum256(body)
	payloadHashHex := hex.EncodeToString(payloadHash[:])
	req.Header.Set("X-Amz-Content-Sha256", payloadHashHex)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := v4.NewSigner().SignHTTP(
		context.Background(),
		aws.Credentials{AccessKeyID: accessKey, SecretAccessKey: secretKey},
		req,
		payloadHashHex,
		"s3",
		"us-east-005",
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("sign request: %v", err)
	}
	return req
}

func presignedACLRequest(t *testing.T, rawURL, accessKey, secretKey string, body []byte) *http.Request {
	t.Helper()
	// The server's query-auth verifier intentionally treats presigned request
	// bodies as empty. Rebuild the request using the empty payload hash and
	// retain the JSON body for handlePresign to decode.
	querySigner := v4.NewSigner()
	emptyHash := sha256.Sum256(nil)
	emptyHashHex := hex.EncodeToString(emptyHash[:])
	unsigned := httptest.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	unsigned.Header.Set("Content-Type", "application/json")
	uri, headers, err := querySigner.PresignHTTP(
		context.Background(),
		aws.Credentials{AccessKeyID: accessKey, SecretAccessKey: secretKey},
		unsigned,
		emptyHashHex,
		"s3",
		"us-east-005",
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("presign request: %v", err)
	}
	queryReq, err := http.NewRequest(http.MethodPost, uri, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create presigned request: %v", err)
	}
	for key, values := range headers {
		for _, value := range values {
			queryReq.Header.Add(key, value)
		}
	}
	queryReq.Header.Set("Content-Type", "application/json")
	queryReq.Header.Set("Authorization", "Bearer "+adminToken)
	return queryReq
}

func doACLRequest(t *testing.T, req *http.Request) (int, []byte) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp.StatusCode, body
}

// TestAppendOnlyACLThroughS3AndSharePaths proves the complete consumer
// boundary: an explicit get+put+list ACL can create and read new objects and
// list its prefix, but cannot overwrite an existing key or delete it. The
// same GET grant is required to mint a share link; a put+list-only credential
// cannot use either the object GET or the presign endpoint.
func TestAppendOnlyACLThroughS3AndSharePaths(t *testing.T) {
	fixture := newAppendOnlyFixture(t)

	newKey := "append/new.txt"
	put := signedACLRequest(t, http.MethodPut, fixture.s3URL+"/"+testBucket+"/"+newKey, appendOnlyAccessKey, appendOnlySecretKey, []byte("new object"))
	if status, body := doACLRequest(t, put); status != http.StatusOK {
		t.Fatalf("new-key PUT status = %d, want 200: %s", status, body)
	}

	get := signedACLRequest(t, http.MethodGet, fixture.s3URL+"/"+testBucket+"/"+newKey, appendOnlyAccessKey, appendOnlySecretKey, nil)
	if status, body := doACLRequest(t, get); status != http.StatusOK || string(body) != "new object" {
		t.Fatalf("GET after append status/body = %d/%q, want 200/%q", status, body, "new object")
	}

	list := signedACLRequest(t, http.MethodGet, fixture.s3URL+"/"+testBucket+"?list-type=2&prefix=append%2F", appendOnlyAccessKey, appendOnlySecretKey, nil)
	if status, body := doACLRequest(t, list); status != http.StatusOK {
		t.Fatalf("scoped LIST status = %d, want 200: %s", status, body)
	}

	overwrite := signedACLRequest(t, http.MethodPut, fixture.s3URL+"/"+testBucket+"/"+fixture.appendKey, appendOnlyAccessKey, appendOnlySecretKey, []byte("replacement"))
	if status, body := doACLRequest(t, overwrite); status != http.StatusForbidden {
		t.Fatalf("overwrite PUT status = %d, want 403: %s", status, body)
	}

	deleteReq := signedACLRequest(t, http.MethodDelete, fixture.s3URL+"/"+testBucket+"/"+fixture.appendKey, appendOnlyAccessKey, appendOnlySecretKey, nil)
	if status, body := doACLRequest(t, deleteReq); status != http.StatusForbidden {
		t.Fatalf("DELETE status = %d, want 403: %s", status, body)
	}

	getExisting := signedACLRequest(t, http.MethodGet, fixture.s3URL+"/"+testBucket+"/"+fixture.appendKey, appendOnlyAccessKey, appendOnlySecretKey, nil)
	if status, body := doACLRequest(t, getExisting); status != http.StatusOK || string(body) != "original object" {
		t.Fatalf("existing object after denied mutations = %d/%q, want 200/original object", status, body)
	}

	presignBody, err := json.Marshal(map[string]string{"bucket": testBucket, "key": fixture.appendKey})
	if err != nil {
		t.Fatalf("marshal presign body: %v", err)
	}
	presignReq := presignedACLRequest(t, fixture.adminURL+"/admin/presign", appendOnlyAccessKey, appendOnlySecretKey, presignBody)
	status, body := doACLRequest(t, presignReq)
	if status != http.StatusOK {
		t.Fatalf("presign with GET grant status = %d, want 200: %s", status, body)
	}
	var presignResponse struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &presignResponse); err != nil {
		t.Fatalf("decode presign response: %v", err)
	}
	shareResp, err := http.Get(presignResponse.URL)
	if err != nil {
		t.Fatalf("GET share URL: %v", err)
	}
	shareBody, err := io.ReadAll(shareResp.Body)
	shareResp.Body.Close()
	if err != nil {
		t.Fatalf("read share response: %v", err)
	}
	if shareResp.StatusCode != http.StatusOK || string(shareBody) != "original object" {
		t.Fatalf("share GET status/body = %d/%q, want 200/original object", shareResp.StatusCode, shareBody)
	}

	noReadPresign := presignedACLRequest(t, fixture.adminURL+"/admin/presign", noReadAccessKey, noReadSecretKey, presignBody)
	if status, body := doACLRequest(t, noReadPresign); status != http.StatusForbidden {
		t.Fatalf("presign without GET grant status = %d, want 403: %s", status, body)
	}
	noReadGet := signedACLRequest(t, http.MethodGet, fixture.s3URL+"/"+testBucket+"/"+fixture.appendKey, noReadAccessKey, noReadSecretKey, nil)
	if status, body := doACLRequest(t, noReadGet); status != http.StatusForbidden {
		t.Fatalf("GET without GET grant status = %d, want 403: %s", status, body)
	}
}
