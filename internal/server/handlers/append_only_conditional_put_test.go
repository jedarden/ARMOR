package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	armorserver "github.com/jedarden/armor/internal/server"
)

// headHiddenBackend hides selected keys from Head so a create-only PUT's
// advisory first gate (manifest lookup, then backend HEAD) misses while the
// backend's authoritative PutIfAbsent still collides with the stored object.
// This isolates the second conflict site the way a cold manifest or a
// HEAD-to-write race does: the rejection then comes from storePutObject's
// PutIfAbsent rather than the early gate.
type headHiddenBackend struct {
	*backend.FSBackend

	hiddenMu sync.RWMutex
	hidden   map[string]bool
}

func (b *headHiddenBackend) hideFromHead(bucket, key string) {
	b.hiddenMu.Lock()
	defer b.hiddenMu.Unlock()
	b.hidden[bucket+"/"+key] = true
}

func (b *headHiddenBackend) isHidden(bucket, key string) bool {
	b.hiddenMu.RLock()
	defer b.hiddenMu.RUnlock()
	return b.hidden[bucket+"/"+key]
}

func (b *headHiddenBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	if b.isHidden(bucket, key) {
		return nil, io.EOF
	}
	return b.FSBackend.Head(ctx, bucket, key)
}

type appendOnlyConditionalFixture struct {
	s3URL   string
	storage *headHiddenBackend
}

// newAppendOnlyConditionalFixture is the ADR-017 counterpart of
// newAppendOnlyFixture: the same put-without-delete credential over the full
// authenticated S3 pipeline, against a filesystem backend that can be told to
// hide keys from Head.
func newAppendOnlyConditionalFixture(t *testing.T) *appendOnlyConditionalFixture {
	t.Helper()

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}
	storage := &headHiddenBackend{FSBackend: fs, hidden: make(map[string]bool)}

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
	}

	cfg := &config.Config{
		Bucket:      testBucket,
		B2Region:    "us-east-005",
		BlockSize:   65536,
		MEK:         bytes.Repeat([]byte{0x33}, 32),
		Credentials: credentials,
	}

	srv, err := armorserver.NewWithBackend(cfg, storage)
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}

	s3Server := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Server.Close)

	return &appendOnlyConditionalFixture{s3URL: s3Server.URL, storage: storage}
}

func seedRawObject(t *testing.T, storage *headHiddenBackend, key, body string) {
	t.Helper()
	err := storage.Put(context.Background(), testBucket, key, strings.NewReader(body), int64(len(body)), map[string]string{"Content-Type": "text/plain"})
	if err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// signedAppendOnlyPut signs a PUT as the append-only credential with the
// conditional header (or its absence) already part of the signature.
func signedAppendOnlyPut(t *testing.T, rawURL string, body []byte, conditional bool) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodPut, rawURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	payloadHashHex := sha256.Sum256(body)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(payloadHashHex[:]))
	req.Header.Set("Content-Type", "application/octet-stream")
	if conditional {
		req.Header.Set("If-None-Match", "*")
	}
	if err := v4.NewSigner().SignHTTP(
		context.Background(),
		aws.Credentials{AccessKeyID: appendOnlyAccessKey, SecretAccessKey: appendOnlySecretKey},
		req,
		hex.EncodeToString(payloadHashHex[:]),
		"s3",
		"us-east-005",
		time.Now().UTC(),
	); err != nil {
		t.Fatalf("sign PUT request: %v", err)
	}
	return req
}

func getStoredBytes(t *testing.T, storage *headHiddenBackend, key string) []byte {
	t.Helper()
	rc, _, err := storage.Get(context.Background(), testBucket, key)
	if err != nil {
		t.Fatalf("get stored %s: %v", key, err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read stored %s: %v", key, err)
	}
	return body
}

func s3Get(t *testing.T, s3URL, key string) (int, []byte) {
	t.Helper()
	req := signedACLRequest(t, http.MethodGet, s3URL+"/"+testBucket+"/"+key, appendOnlyAccessKey, appendOnlySecretKey, nil)
	return doACLRequest(t, req)
}

// TestAppendOnlyConditionalPutConflictReturnsPreconditionFailed pins the
// ADR-017 conflict contract at the early manifest/HEAD gate: an append-only
// credential's explicit If-None-Match: * PUT against an existing key returns
// the standard 412 PreconditionFailed a full-access credential already gets
// for the identical request shape, its unconditional overwrite attempt stays
// 403 AccessDenied, repeated conditional requests keep reporting the conflict,
// stored bytes never change, and a new-key conditional PUT still succeeds.
func TestAppendOnlyConditionalPutConflictReturnsPreconditionFailed(t *testing.T) {
	fixture := newAppendOnlyConditionalFixture(t)
	seedRawObject(t, fixture.storage, "append/existing.txt", "original object")
	objectURL := fixture.s3URL + "/" + testBucket + "/append/existing.txt"

	conditionalPut := func(body []byte) (int, []byte) {
		return doACLRequest(t, signedAppendOnlyPut(t, objectURL, body, true))
	}

	// Explicit conditional conflict → 412 with the S3 PreconditionFailed code.
	status, body := conditionalPut([]byte("retry bytes"))
	if status != http.StatusPreconditionFailed || !strings.Contains(string(body), "<Code>PreconditionFailed</Code>") {
		t.Fatalf("conditional conflict = %d/%q, want 412 with PreconditionFailed code", status, body)
	}

	// The conflict leaves the stored object untouched.
	if status, body := s3Get(t, fixture.s3URL, "append/existing.txt"); status != http.StatusOK || string(body) != "original object" {
		t.Fatalf("object after conditional conflict = %d/%q, want 200/original object", status, body)
	}

	// Retries of the identical conditional request keep reporting the same
	// conflict — the condition, not a one-shot race outcome.
	for i := 0; i < 2; i++ {
		if status, body := conditionalPut([]byte("retry bytes")); status != http.StatusPreconditionFailed {
			t.Fatalf("repeated conditional PUT %d = %d/%q, want 412", i+1, status, body)
		}
	}

	// Unconditional overwrite of the same key stays an authorization denial.
	if status, body := doACLRequest(t, signedAppendOnlyPut(t, objectURL, []byte("overwrite"), false)); status != http.StatusForbidden || !strings.Contains(string(body), "<Code>AccessDenied</Code>") {
		t.Fatalf("unconditional overwrite = %d/%q, want 403 with AccessDenied code", status, body)
	}
	if status, body := s3Get(t, fixture.s3URL, "append/existing.txt"); status != http.StatusOK || string(body) != "original object" {
		t.Fatalf("object after denied overwrite = %d/%q, want 200/original object", status, body)
	}

	// A new-key conditional PUT still creates the object.
	newURL := fixture.s3URL + "/" + testBucket + "/append/conditional-new.txt"
	if status, body := doACLRequest(t, signedAppendOnlyPut(t, newURL, []byte("created"), true)); status != http.StatusOK {
		t.Fatalf("new-key conditional PUT = %d/%q, want 200", status, body)
	}
	if status, body := s3Get(t, fixture.s3URL, "append/conditional-new.txt"); status != http.StatusOK || string(body) != "created" {
		t.Fatalf("new-key conditional PUT result = %d/%q, want 200/created", status, body)
	}
}

// TestAppendOnlyConditionalPutConflictAtBackendGate pins the same contract at
// the backend PutIfAbsent collision — the authoritative second gate — for both
// the buffered small path and the streaming path, with the advisory
// manifest/HEAD first gate blinded. An explicit conditional conflict is 412,
// an unconditional overwrite attempt stays 403, and neither changes the
// stored bytes.
func TestAppendOnlyConditionalPutConflictAtBackendGate(t *testing.T) {
	fixture := newAppendOnlyConditionalFixture(t)
	seedRawObject(t, fixture.storage, "append/ghost-small.txt", "ghost small")
	seedRawObject(t, fixture.storage, "append/ghost-stream.txt", "ghost stream")
	fixture.storage.hideFromHead(testBucket, "append/ghost-small.txt")
	fixture.storage.hideFromHead(testBucket, "append/ghost-stream.txt")

	// One kilobyte over the streaming threshold forces putObjectStreaming.
	streamingBody := bytes.Repeat([]byte("w"), 10*1024*1024+1024)

	for _, test := range []struct {
		name   string
		key    string
		seeded string
		body   []byte
	}{
		{name: "small", key: "append/ghost-small.txt", seeded: "ghost small", body: []byte("replacement small")},
		{name: "streaming", key: "append/ghost-stream.txt", seeded: "ghost stream", body: streamingBody},
	} {
		t.Run(test.name, func(t *testing.T) {
			objectURL := fixture.s3URL + "/" + testBucket + "/" + test.key

			if status, body := doACLRequest(t, signedAppendOnlyPut(t, objectURL, test.body, true)); status != http.StatusPreconditionFailed || !strings.Contains(string(body), "<Code>PreconditionFailed</Code>") {
				t.Fatalf("conditional collision = %d/%q, want 412 with PreconditionFailed code", status, body)
			}
			if status, body := doACLRequest(t, signedAppendOnlyPut(t, objectURL, test.body, false)); status != http.StatusForbidden || !strings.Contains(string(body), "<Code>AccessDenied</Code>") {
				t.Fatalf("unconditional collision = %d/%q, want 403 with AccessDenied code", status, body)
			}

			if got := string(getStoredBytes(t, fixture.storage, test.key)); got != test.seeded {
				t.Fatalf("stored object changed after conflicts: %q, want %q", got, test.seeded)
			}
		})
	}

	if got := string(getStoredBytes(t, fixture.storage, "append/ghost-small.txt")); got != "ghost small" {
		t.Fatalf("small object changed after conflicts: %q", got)
	}
	if got := string(getStoredBytes(t, fixture.storage, "append/ghost-stream.txt")); got != "ghost stream" {
		t.Fatalf("streaming object changed after conflicts: %q", got)
	}

	// The create-only path itself is unchanged: a new-key conditional PUT
	// still succeeds on the streaming path.
	newURL := fixture.s3URL + "/" + testBucket + "/append/conditional-stream-new.txt"
	if status, body := doACLRequest(t, signedAppendOnlyPut(t, newURL, streamingBody, true)); status != http.StatusOK {
		t.Fatalf("new-key streaming conditional PUT = %d/%q, want 200", status, body)
	}
	if status, body := s3Get(t, fixture.s3URL, "append/conditional-stream-new.txt"); status != http.StatusOK || !bytes.Equal(body, streamingBody) {
		t.Fatalf("new-key streaming conditional PUT result = %d (%d bytes), want 200 (%d bytes)", status, len(body), len(streamingBody))
	}
}
