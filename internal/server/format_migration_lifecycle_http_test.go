package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
)

// pausedMigrationBackend keeps the inventory walk behind an explicit gate so
// the HTTP test can prove admission and polling survive a disconnected client.
type pausedMigrationBackend struct {
	*backend.FSBackend
	armed       atomic.Bool
	once        sync.Once
	entered     chan struct{}
	release     chan struct{}
	sourceReads atomic.Int32
}

func (b *pausedMigrationBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	if !strings.HasPrefix(key, ".armor/") {
		b.sourceReads.Add(1)
	}
	return b.FSBackend.GetRange(ctx, bucket, key, offset, length)
}

func (b *pausedMigrationBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if !strings.HasPrefix(key, ".armor/") {
		b.sourceReads.Add(1)
	}
	return b.FSBackend.GetDirect(ctx, bucket, key)
}

func (b *pausedMigrationBackend) List(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int) (*backend.ListResult, error) {
	if b.armed.Load() && maxKeys == 1000 {
		b.once.Do(func() { close(b.entered) })
		select {
		case <-b.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return b.FSBackend.List(ctx, bucket, prefix, delimiter, token, maxKeys)
}

func TestMigrationHTTPInventoryOnlyClassifiesWithoutBodyReads(t *testing.T) {
	ctx := context.Background()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	const bucket = "inventory-only"
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatal(err)
	}
	mek := bytes.Repeat([]byte{0x44}, 32)
	fixtures := NewMockBackend()
	putRealV1SinglePUT(t, fixtures, mek, "v1-object")
	fixtures.objects["v1-object"].Data[0] ^= 0xff
	fixtures.objects["unknown-object"] = &MockObject{
		Data: []byte("unknown"), Metadata: map[string]string{armorMetaVersion: "not-a-version"},
	}
	for key, obj := range fixtures.objects {
		if err := fs.Put(ctx, bucket, key, bytes.NewReader(obj.Data), int64(len(obj.Data)), obj.Metadata); err != nil {
			t.Fatal(err)
		}
	}
	storage := &pausedMigrationBackend{FSBackend: fs}
	srv, err := NewWithBackend(&config.Config{
		Bucket: bucket, BlockSize: 65536, MEK: mek, FormatWriteVersion: 3,
		AdminToken: migrationTestToken,
	}, storage)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(srv.AdminHandler())
	defer httpServer.Close()
	call := func(method, query string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, httpServer.URL+"/admin/format/migrate?"+query, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+migrationTestToken)
		resp, err := httpServer.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, body
	}
	status, body := call(http.MethodPost, "target=3&include=1,2&inventory_only=true")
	if status != http.StatusAccepted {
		t.Fatalf("inventory start = %d: %s", status, body)
	}
	var admitted struct {
		OperationID string `json:"operation_id"`
	}
	if err := json.Unmarshal(body, &admitted); err != nil || admitted.OperationID == "" {
		t.Fatalf("admission = %s: %v", body, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		status, body = call(http.MethodGet, "operation_id="+admitted.OperationID)
		if status == http.StatusInternalServerError && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if status != http.StatusOK {
			t.Fatalf("inventory status = %d: %s", status, body)
		}
		var state MigrationState
		if err := json.Unmarshal(body, &state); err != nil {
			t.Fatal(err)
		}
		if state.Status == "completed" {
			if !state.DryRun || !state.InventoryOnly || state.FailedObjects != 0 || state.UnknownObjects != 1 || state.Classification.V1SinglePut != 1 || state.Classification.Malformed != 1 {
				t.Fatalf("inventory result = %+v", state)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("inventory did not complete: %s", body)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if reads := storage.sourceReads.Load(); reads != 0 {
		t.Fatalf("inventory read %d source bodies, want zero", reads)
	}
	info, err := fs.Head(ctx, bucket, "v1-object")
	if err != nil || info.Metadata[armorMetaVersion] != "1" {
		t.Fatalf("inventory changed source object: info=%v err=%v", info, err)
	}
}

func TestMigrationHTTPOperationSurvivesDisconnectAndReportsFailures(t *testing.T) {
	ctx := context.Background()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	const bucket = "migration-lifecycle"
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatal(err)
	}
	mek := bytes.Repeat([]byte{0x44}, 32)
	fixtures := NewMockBackend()
	putRealV1SinglePUT(t, fixtures, mek, "good-v1")
	putRealV1SinglePUT(t, fixtures, mek, "bad-v1")
	plaintextSHA := sha256.Sum256([]byte("test data for migration"))
	fixtures.objects["good-v1"].Metadata[armorMetaPlaintextSHA] = hex.EncodeToString(plaintextSHA[:])
	// A valid candidate with a broken envelope must appear in failed_objects
	// and failures, while the other object still commits to v3.
	fixtures.objects["bad-v1"].Data[0] ^= 0xff
	for key, obj := range fixtures.objects {
		if err := fs.Put(ctx, bucket, key, bytes.NewReader(obj.Data), int64(len(obj.Data)), obj.Metadata); err != nil {
			t.Fatal(err)
		}
	}
	storage := &pausedMigrationBackend{FSBackend: fs, entered: make(chan struct{}), release: make(chan struct{})}
	defer func() {
		select {
		case <-storage.release:
		default:
			close(storage.release)
		}
	}()
	srv, err := NewWithBackend(&config.Config{
		Bucket: bucket, BlockSize: 65536, MEK: mek, FormatWriteVersion: 3,
		AdminToken: migrationTestToken,
	}, storage)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(srv.AdminHandler())
	defer httpServer.Close()
	storage.armed.Store(true)

	request := func(ctx context.Context, method, query string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, httpServer.URL+"/admin/format/migrate?"+query, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+migrationTestToken)
		resp, err := httpServer.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		return resp, body
	}

	startCtx, disconnect := context.WithCancel(ctx)
	resp, body := request(startCtx, http.MethodPost, "target=3&include=1,2")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("start status = %d, body = %s", resp.StatusCode, body)
	}
	var admitted struct {
		OperationID string `json:"operation_id"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal(body, &admitted); err != nil || admitted.OperationID == "" {
		t.Fatalf("start response = %s, decode error = %v", body, err)
	}
	disconnect()
	select {
	case <-storage.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("background inventory did not start")
	}
	resp, body = request(ctx, http.MethodGet, "operation_id="+admitted.OperationID)
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(admitted.OperationID)) {
		t.Fatalf("reconnect status = %d, body = %s", resp.StatusCode, body)
	}
	resp, _ = request(ctx, http.MethodGet, "operation_id=wrong-operation")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong operation status = %d, want 404", resp.StatusCode)
	}
	close(storage.release)

	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, body = request(ctx, http.MethodGet, "operation_id="+admitted.OperationID)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("watch status = %d, body = %s", resp.StatusCode, body)
		}
		var state MigrationState
		if err := json.Unmarshal(body, &state); err != nil {
			t.Fatal(err)
		}
		if state.Status == "completed" {
			if state.ID != admitted.OperationID || state.FailedObjects != 1 || len(state.Failures) != 1 || state.Failures[0].Key != "bad-v1" {
				t.Fatalf("terminal failed-object report = %+v", state)
			}
			if state.Classification.V1SinglePut != 2 || state.Classification.OutcomeFailed != 1 || state.Classification.OutcomeProcessed != 1 {
				t.Fatalf("per-version/layout progress = %+v", state.Classification)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("migration did not complete; last state = %s", body)
		}
		time.Sleep(20 * time.Millisecond)
	}
	info, err := fs.Head(ctx, bucket, "good-v1")
	if err != nil || info.Metadata[armorMetaVersion] != "3" {
		t.Fatalf("good object version after migration = %v, err = %v", info, err)
	}
}
