package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// BenchmarkUploadPartFilesystem exercises the complete v3 UploadPart handler
// with eight concurrent 64 MiB parts and the filesystem backend. Run with
// -benchtime=1x and a CPU profile to compare revisions under the same cgroup.
func BenchmarkUploadPartFilesystem(b *testing.B) {
	const (
		partSize = 64 << 20
		parts    = 8
		bucket   = "test-bucket"
		key      = "benchmark-multipart"
	)
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		b.Fatal(err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	cfg := &config.Config{BlockSize: 65536, FormatWriteVersion: 3}
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: b.TempDir()})
	if err != nil {
		b.Fatal(err)
	}
	if err := fs.CreateBucket(context.Background(), bucket); err != nil {
		b.Fatal(err)
	}
	h := handlers.New(cfg, fs, cache, footerCache, km, nil)
	input := bytes.Repeat([]byte{0x5a}, partSize)
	b.SetBytes(parts * partSize)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		response := httptest.NewRecorder()
		h.CreateMultipartUpload(response,
			createUploadRequest(context.Background(), bucket, key, "application/octet-stream"), bucket, key)
		if response.Code != http.StatusOK {
			b.Fatalf("create multipart: %d %s", response.Code, response.Body.String())
		}
		var created struct {
			UploadID string `xml:"UploadId"`
		}
		if err := xml.Unmarshal(response.Body.Bytes(), &created); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		var wg sync.WaitGroup
		for part := 1; part <= parts; part++ {
			wg.Add(1)
			go func(part int) {
				defer wg.Done()
				request := createUploadPartRequest(context.Background(), bucket, key, created.UploadID,
					part, "application/octet-stream", input)
				writer := httptest.NewRecorder()
				h.UploadPart(writer, request, bucket, key, created.UploadID)
				if writer.Code != http.StatusOK {
					b.Errorf("part %d: %d %s", part, writer.Code, writer.Body.String())
				}
			}(part)
		}
		wg.Wait()
		if b.Failed() {
			b.FailNow()
		}
	}
}
