package handlers_test

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

func TestMultiKeyRoutingEncryptsAndDecryptsByPath(t *testing.T) {
	defaultMEK := testMEK(0x11)
	piiMEK := testMEK(0x22)
	archiveMEK := testMEK(0x33)
	routes, err := keymanager.ParseKeyRoutes("data/pii/*=pii,archive/*=archive,*=default")
	if err != nil {
		t.Fatalf("ParseKeyRoutes: %v", err)
	}
	km, err := keymanager.New(defaultMEK, map[string][]byte{
		"pii":     piiMEK,
		"archive": archiveMEK,
	}, routes)
	if err != nil {
		t.Fatalf("New key manager: %v", err)
	}

	cfg := &config.Config{BlockSize: 65536, AuthAccessKey: "test", AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE"}
	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	h := handlers.New(cfg, mb, cache, backend.NewFooterCache(1000, 300), km, nil)

	objects := map[string]struct {
		body  string
		keyID string
		mek   []byte
	}{
		"data/pii/customer.json":   {body: "private", keyID: "pii", mek: piiMEK},
		"archive/2026/report.json": {body: "archived", keyID: "archive", mek: archiveMEK},
		"data/public/report.json":  {body: "public", keyID: "default", mek: defaultMEK},
	}

	for key, want := range objects {
		req := httptest.NewRequest(http.MethodPut, "/bucket/"+key, strings.NewReader(want.body))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("PUT %s: status %d, body %s", key, w.Code, w.Body.String())
		}

		mb.mu.Lock()
		meta := mb.meta["bucket/"+key]
		mb.mu.Unlock()
		armorMeta, ok := backend.ParseARMORMetadata(meta)
		if !ok {
			t.Fatalf("PUT %s did not create ARMOR metadata", key)
		}
		gotKeyID := armorMeta.KeyID
		if gotKeyID == "" {
			gotKeyID = "default"
		}
		if gotKeyID != want.keyID {
			t.Errorf("PUT %s key ID = %q, want %q", key, gotKeyID, want.keyID)
		}

		// The PUT path writes the fingerprinted form v2:<fp16>:<base64>; the
		// fingerprint must be the routed key's, so a reader can resolve the
		// DEK without consulting the (informational) key-id metadata.
		dekParts := strings.SplitN(meta["x-amz-meta-armor-wrapped-dek"], ":", 3)
		if len(dekParts) != 3 || dekParts[0] != "v2" {
			t.Fatalf("wrapped DEK for %s is not v2:<fp>:<base64>: %q", key, meta["x-amz-meta-armor-wrapped-dek"])
		}
		if wantFp := crypto.MEKFingerprint(want.mek); dekParts[1] != wantFp {
			t.Errorf("wrapped DEK fingerprint for %s = %q, want routed key %q fingerprint %q", key, dekParts[1], want.keyID, wantFp)
		}
		wrapped, err := base64.StdEncoding.DecodeString(dekParts[2])
		if err != nil {
			t.Fatalf("decode wrapped DEK for %s: %v", key, err)
		}
		if _, err := crypto.UnwrapDEK(want.mek, wrapped); err != nil {
			t.Errorf("%s was not wrapped with routed key %q: %v", key, want.keyID, err)
		}
		for otherID, otherMEK := range map[string][]byte{"default": defaultMEK, "pii": piiMEK, "archive": archiveMEK} {
			if otherID == want.keyID {
				continue
			}
			if _, err := crypto.UnwrapDEK(otherMEK, wrapped); err == nil {
				t.Errorf("%s unexpectedly unwraps with unrelated key %q", key, otherID)
			}
		}

		req = httptest.NewRequest(http.MethodGet, "/bucket/"+key, nil)
		w = httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), []byte(want.body)) {
			t.Errorf("GET %s: status %d, body %q; want %q", key, w.Code, w.Body.String(), want.body)
		}
	}
}

func TestMultiKeyRoutingRejectsUnknownMetadataKey(t *testing.T) {
	defaultMEK := testMEK(0x44)
	km, err := keymanager.New(defaultMEK, nil, nil)
	if err != nil {
		t.Fatalf("New key manager: %v", err)
	}
	cfg := &config.Config{BlockSize: 65536, AuthAccessKey: "test", AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE"}
	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	h := handlers.New(cfg, mb, cache, backend.NewFooterCache(1000, 300), km, nil)

	req := httptest.NewRequest(http.MethodPut, "/bucket/object.txt", strings.NewReader("payload"))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT: status %d, body %s", w.Code, w.Body.String())
	}

	// The reader resolves the DEK by the fingerprint embedded in the wrapped
	// DEK (v2:<fp16>:<base64>) with active/ring lookup; the key-id metadata is
	// informational. An object whose fingerprint no configured key matches
	// must fail closed rather than serve ciphertext as plaintext.
	mb.mu.Lock()
	orig := mb.meta["bucket/object.txt"]["x-amz-meta-armor-wrapped-dek"]
	parts := strings.SplitN(orig, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		mb.mu.Unlock()
		t.Fatalf("PUT did not store a v2 fingerprinted wrapped DEK: %q", orig)
	}
	mb.meta["bucket/object.txt"]["x-amz-meta-armor-wrapped-dek"] = "v2:0000000000000000:" + parts[2]
	mb.mu.Unlock()
	cache.Clear()

	req = httptest.NewRequest(http.MethodGet, "/bucket/object.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GET with unknown key fingerprint: status %d, body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not found in active or ring keys") {
		t.Errorf("GET with unknown key fingerprint error = %q, want fingerprint-not-found context", w.Body.String())
	}
}

func testMEK(value byte) []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = value
	}
	return mek
}
