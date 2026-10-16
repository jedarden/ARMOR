package server

import (
	"bytes"
	"context"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
)

// The monitor's production read callback must reach GetObject for both v3
// formats. A failure here would otherwise leave /readyz healthy after a
// storage or handler regression that clients can see.
func TestCanaryV3UsesGetObjectReadPath(t *testing.T) {
	ctx := context.Background()
	store, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateBucket(ctx, "canary-read"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Bucket:             "canary-read",
		Prefix:             "tenant/",
		MEK:                bytes.Repeat([]byte{0x42}, 32),
		BlockSize:          crypto.DefaultBlockSize,
		FormatWriteVersion: 3,
	}
	s, err := NewWithBackend(cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	m := canary.NewMonitor(canary.Config{
		Backend:            store,
		Bucket:             cfg.Bucket,
		Prefix:             cfg.Prefix,
		MEK:                cfg.MEK,
		BlockSize:          cfg.BlockSize,
		InstanceID:         "read-path-test",
		FormatWriteVersion: cfg.FormatWriteVersion,
		ReadObject: func(ctx context.Context, bucket, key string) ([]byte, error) {
			return readCanaryObject(ctx, s.config, s.backend, s.cache, s.footerCache, s.keyManager, s.listCache, bucket, key)
		},
	})
	m.Start(ctx)
	m.Stop()
	status := m.GetStatus()
	if status.Status != canary.StatusHealthy || status.MultipartHealthy != canary.StatusHealthy {
		t.Fatalf("v3 canary through GetObject: small=%s (%s), multipart=%s (%s)", status.Status, status.LastError, status.MultipartHealthy, status.MultipartLastError)
	}
}
