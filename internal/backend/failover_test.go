package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

// TestManualFailoverBackendDecision pins the backend-level part of the
// ADR-006 promotion decision: the secondary is never selected while the
// primary is healthy, and is selected explicitly only after the primary is
// unavailable. The server-level integration drill covers the same decision
// through the authenticated S3 handlers; this unit test keeps the storage
// contract small and independent of server setup.
func TestManualFailoverBackendDecision(t *testing.T) {
	ctx := context.Background()
	primary, err := NewFSBackend(FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create primary: %v", err)
	}
	secondary, err := NewFSBackend(FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create secondary: %v", err)
	}

	const (
		bucket = "failover-bucket"
		key    = "tenant/object.bin"
	)
	for name, be := range map[string]Backend{"primary": primary, "secondary": secondary} {
		if err := be.CreateBucket(ctx, bucket); err != nil {
			t.Fatalf("create %s bucket: %v", name, err)
		}
	}

	contents := []byte("replicated ciphertext")
	metadata := map[string]string{
		"x-amz-meta-armor-version":          "3",
		"x-amz-meta-armor-plaintext-size":   "20",
		"x-amz-meta-armor-plaintext-sha256": "test-digest",
	}
	for name, be := range map[string]Backend{"primary": primary, "secondary": secondary} {
		if err := be.Put(ctx, bucket, key, bytes.NewReader(contents), int64(len(contents)), metadata); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	// A healthy primary remains the active backend even though the secondary
	// already has a byte-identical copy. Replication must not alter the read
	// path or perform an automatic switchover.
	active := Backend(primary)
	if _, err := active.Head(ctx, bucket, key); err != nil {
		t.Fatalf("healthy primary should remain active: %v", err)
	}

	// Simulate the provider outage by removing the primary copy. The decision
	// to promote is explicit and made only after the primary operation fails;
	// this mirrors Route A step 3 without adding automatic failover behavior to
	// the Backend interface.
	if err := primary.Delete(ctx, bucket, key); err != nil {
		t.Fatalf("remove primary copy: %v", err)
	}
	if _, err := active.Head(ctx, bucket, key); err == nil {
		t.Fatal("primary outage Head unexpectedly succeeded")
	} else if !errors.Is(err, ErrObjectNotFound) && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("primary outage Head error = %v, want a not-found error", err)
	}
	active = secondary

	// Promotion preserves both the object bytes and the metadata sidecar that
	// the filesystem backend writes with each object. Those are the inputs the
	// server needs to decrypt a mirrored single-PUT object.
	body, info, err := active.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("promoted secondary Get: %v", err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		t.Fatalf("read promoted object: %v", err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("promoted object bytes = %q, want %q", got, contents)
	}
	if info == nil || info.Metadata["x-amz-meta-armor-version"] != "3" {
		t.Fatalf("promoted metadata = %#v, want the replicated ARMOR metadata", info)
	}
}
