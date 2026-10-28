package handlers_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

type pausedManifestBackend struct {
	*recordingBackend
	before chan struct{}
	resume chan struct{}
}

func (b *pausedManifestBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if strings.HasSuffix(key, ".armor-manifest") {
		close(b.before)
		<-b.resume
	}
	return b.recordingBackend.Put(ctx, bucket, key, body, size, meta)
}

func TestListObjectsV2VisibleAfterConcurrentMultipartCompletion(t *testing.T) {
	const bucket = "test-bucket"
	const key = "transcripts/one"
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatal(err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		BlockSize:          65536,
		FormatWriteVersion: 3,
		AuthAccessKey:      "test-access-key",
		AuthSecretKey:      "REMOVED-NOT-A-SECRET-VALUE",
	}
	be := &pausedManifestBackend{
		recordingBackend: newRecordingBackend(),
		before:           make(chan struct{}),
		resume:           make(chan struct{}),
	}
	defer func() {
		select {
		case <-be.resume:
		default:
			close(be.resume)
		}
	}()
	listCache := backend.NewListCache(100, 60)
	h := handlers.New(cfg, be, backend.NewMetadataCache(100, 60),
		backend.NewFooterCache(100, 60), km, listCache)
	uploadID := initiateMultipart(t, h, bucket, key)
	etag := uploadPart(t, h, bucket, key, uploadID, 1, []byte("transcript"))
	body := fmt.Sprintf("<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part></CompleteMultipartUpload>", etag)
	completeReq := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploadId="+uploadID, strings.NewReader(body))
	completed := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.HandleRoot(w, completeReq)
		completed <- w
	}()

	select {
	case <-be.before:
	case <-time.After(10 * time.Second):
		t.Fatal("completion did not reach the manifest write")
	}
	list := func() string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/"+bucket+"?list-type=2&prefix=transcripts/", nil)
		w := httptest.NewRecorder()
		h.ListObjectsV2(w, req, bucket)
		if w.Code != http.StatusOK {
			t.Fatalf("LIST returned %d: %s", w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	manifestElement := "<Key>" + key + ".armor-manifest</Key>"
	if got := list(); strings.Contains(got, manifestElement) {
		t.Fatalf("manifest appeared before its write: %s", got)
	}

	close(be.resume)
	select {
	case w := <-completed:
		if w.Code != http.StatusOK {
			t.Fatalf("complete returned %d: %s", w.Code, w.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("completion did not finish")
	}
	if got := list(); !strings.Contains(got, manifestElement) {
		t.Fatalf("cached LIST omitted manifest after completion: %s", got)
	}
}
