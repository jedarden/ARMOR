package handlers

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"runtime"
	"testing"
)

func TestReadUploadPartBodyUsesDeclaredSizeAndHashesActualBytes(t *testing.T) {
	plain := []byte("upload-part-digest\x00and-trailing-bytes")
	for _, tc := range []struct {
		name   string
		length int64
	}{
		{"known", int64(len(plain))},
		{"unknown", -1},
		{"understated", int64(len(plain) - 3)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, digest, release, err := readUploadPartBody(bytes.NewReader(plain), tc.length)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if !bytes.Equal(got, plain) || digest != sha256.Sum256(plain) {
				t.Fatal("body or digest differs from the bytes read")
			}
		})
	}
}

func TestReadUploadPartBodyPropagatesTruncatedBody(t *testing.T) {
	_, _, _, err := readUploadPartBody(bytes.NewReader([]byte("short")), 100)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated body error = %v, want unexpected EOF", err)
	}
}

// An aws-chunked request can advertise a decoded length shorter than its
// actual body. The full pooled prefix must be copied before another request
// can reuse that buffer.
func TestReadUploadPartBodyUnderstatedPooledLengthConcurrent(t *testing.T) {
	plain := bytes.Repeat([]byte{0x5a}, pooledUploadPartSize+1)
	plain[len(plain)-1] = 0x8b
	extraRead := make(chan struct{})
	reader := &signalExtraRead{reader: bytes.NewReader(plain), extraRead: extraRead}
	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		<-extraRead
		for {
			select {
			case <-stop:
				return
			default:
			}
			if reused := uploadPartBodyPool.Get(); reused != nil {
				buffer := reused.([]byte)
				buffer[0] ^= 0xff
				uploadPartBodyPool.Put(buffer)
			}
			runtime.Gosched()
		}
	}()
	got, digest, release, err := readUploadPartBody(reader, pooledUploadPartSize)
	close(stop)
	<-finished
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if !bytes.Equal(got, plain) || digest != sha256.Sum256(plain) {
		t.Fatal("understated body or digest was corrupted during concurrent pool reuse")
	}
}

type signalExtraRead struct {
	reader    *bytes.Reader
	extraRead chan struct{}
}

func (r *signalExtraRead) Read(p []byte) (int, error) {
	if r.reader.Len() == 1 {
		close(r.extraRead)
	}
	return r.reader.Read(p)
}
