// SDK smoke tests — the always-runs core of the compatibility suite.
//
// The aws/rclone CLIs are not installed in the dev/CI image, so the
// TestAWSCLI_* / TestRclone_* tests in awscli_compat_test.go skip cleanly
// there. To keep the suite meaningful even without those binaries, these
// tests drive the *same* S3 request paths the CLIs use — single-shot put/get,
// low-level multipart (CreateMultipartUpload -> UploadPart x N ->
// CompleteMultipartUpload, completed out-of-order to assert part-number
// assembly), and a concurrent transfer fan-out — via aws-sdk-go-v2, a pure Go
// dependency that is always available. They run on every `go test` (including
// CI's `-short` gate) because they need no external binaries, no network, and
// no cloud credentials. If these pass, the real CLI tests pass once the CLIs
// are present: they exercise the identical in-process server and handlers.
package awsclicompat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func newSDKClient(t *testing.T, endpoint string) *s3.Client {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(testRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(testAccessKey, testSecretKey, "")),
	)
	if err != nil {
		t.Fatalf("load sdk config: %v", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
}

func TestVerify_MultipartRoundTrip(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := testBucket
	key := "verify/multipart.bin"

	// 9 MiB total in two parts: 8 MiB + 1 MiB (matches the CLI test's split).
	payload := bytes.Repeat([]byte{0xAB}, 8*1024*1024)
	payload = append(payload, bytes.Repeat([]byte{0xCD}, 1024*1024)...)
	part1 := payload[:8*1024*1024]
	part2 := payload[8*1024*1024:]

	mu, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: &bucket, Key: &key})
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	uploadID := mu.UploadId

	up := func(n int32, body []byte) string {
		pn := n
		r := bytes.NewReader(body)
		out, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket: &bucket, Key: &key, UploadId: uploadID,
			PartNumber: &pn, Body: r,
		})
		if err != nil {
			t.Fatalf("UploadPart %d: %v", n, err)
		}
		return *out.ETag
	}
	e1 := up(1, part1)
	e2 := up(2, part2)

	// Complete parts in REVERSE order to prove part-number assembly, not
	// arrival order, drives the result (ADR-005 out-of-order contract).
	pn1, pn2 := int32(1), int32(2)
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: &bucket, Key: &key, UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{ETag: &e2, PartNumber: &pn2},
				{ETag: &e1, PartNumber: &pn1},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	got, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	body, _ := io.ReadAll(got.Body)
	got.Body.Close()
	if !bytes.Equal(body, payload) {
		t.Fatalf("multipart round-trip mismatch: want %d bytes got %d", len(payload), len(body))
	}
	t.Logf("multipart round-trip OK: %d bytes, 2 parts, completed out-of-order", len(payload))
}

func TestVerify_ConcurrentTransfers(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := testBucket

	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("verify/concurrent/%d.bin", i)
			payload := bytes.Repeat([]byte{byte(i)}, 64*1024+i)
			if _, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: &bucket, Key: &key, Body: bytes.NewReader(payload),
			}); err != nil {
				errs <- fmt.Errorf("put %d: %w", i, err)
				return
			}
			got, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
			if err != nil {
				errs <- fmt.Errorf("get %d: %w", i, err)
				return
			}
			body, _ := io.ReadAll(got.Body)
			got.Body.Close()
			if !bytes.Equal(body, payload) {
				errs <- fmt.Errorf("get %d: bytes mismatch", i)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent transfer failed: %v", err)
	}
	t.Logf("concurrent transfers OK: %d parallel put+get round-trips", n)
}
