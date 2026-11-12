package server

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

func TestDurableMigrationStreamsAndConditionallyCommits(t *testing.T) {
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatal(err)
	}
	const bucket = "bucket"
	if err := fs.CreateBucket(context.Background(), bucket); err != nil {
		t.Fatal(err)
	}
	mock := NewMockBackend()
	mek := clsTestMEK()
	plaintext := []byte("test data for migration")
	putRealV1SinglePUT(t, mock, mek, "object")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	mock.objects["object"].Metadata[armorMetaPlaintextSHA] = hex.EncodeToString(plaintextSHA[:])
	obj := mock.objects["object"]
	if err := fs.Put(context.Background(), bucket, "object", bytesReader(obj.Data), int64(len(obj.Data)), obj.Metadata); err != nil {
		t.Fatal(err)
	}
	fm := NewFormatMigrator(fs, bucket, mek, "default", crypto.Version3, []string{"1"}, nil)
	result, err := fm.RunDurable(context.Background(), false, 2)
	if err != nil {
		t.Fatalf("durable migration: %v", err)
	}
	if result.Status != "completed" || result.ProcessedObjects != 1 {
		t.Fatalf("result = %+v", result)
	}
	info, err := fs.Head(context.Background(), bucket, "object")
	if err != nil {
		t.Fatal(err)
	}
	if info.Metadata[armorMetaVersion] != "3" {
		t.Fatalf("version = %q, want 3", info.Metadata[armorMetaVersion])
	}
	state, err := fm.loadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Checkpoints["object"].Status != "committed" {
		t.Fatalf("checkpoint = %+v", state.Checkpoints["object"])
	}
}
