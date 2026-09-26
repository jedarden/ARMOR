// Protocol conformance tests — the S3 wire behaviors behind README.md's
// client-compatibility claim, pinned on every `go test` run.
//
// README no longer claims that *any* S3 client works unmodified; it claims
// the tested contract: authentication, reads, byte-range reads, listing,
// overwrite, deletion, single-PUT writes, and multipart with default
// concurrency. Multipart round-trip and concurrent transfers are pinned by
// zz_verify_sdk_test.go (TestVerify_MultipartRoundTrip /
// TestVerify_ConcurrentTransfers); this file pins the rest of that contract
// through aws-sdk-go-v2 — a real SigV4 signer — against the same in-process
// ARMOR server the CLI legs use. Like the other TestVerify_* tests they run
// on every `go test` (including CI's `-short` gate): no external binaries,
// no network, no cloud credentials. The full lifecycle re-runs against a
// real B2 backend in zz_lifecycle_integration_test.go, and the real-client
// matrix (AWS CLI, rclone, boto3, DuckDB, litestream, barman) runs in full
// mode and against each built image in CI.
package awsclicompat

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// conformancePayload returns a deterministic payload whose bytes vary at
// every position, so a wrong-offset range read cannot alias to the right
// content. The seed is arbitrary but fixed.
func conformancePayload(n int) []byte {
	buf := make([]byte, n)
	x := uint32(0x9fca25b2)
	for i := range buf {
		x = x*1664525 + 1013904223
		buf[i] = byte(x >> 24)
	}
	return buf
}

// newSDKClientWithCredentials builds an S3 client against endpoint with an
// explicit credential pair. It resolves endpoint-mode credentials the same
// way newSDKClient does, so callers can corrupt exactly one field.
func newSDKClientWithCredentials(t *testing.T, endpoint, accessKey, secretKey string) *s3.Client {
	t.Helper()

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(testRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("load sdk config: %v", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
}

// sdkCredentials returns the credential pair the suite's clients use,
// honoring endpoint mode so negative-path tests corrupt a real key.
func sdkCredentials(t *testing.T) (string, string) {
	t.Helper()
	if isCompatEndpointMode() {
		_, ak, sk := compatEndpointConfig()
		if ak == "" || sk == "" {
			t.Fatalf("ARMOR_COMPAT_ENDPOINT requires ARMOR_COMPAT_ACCESS_KEY and ARMOR_COMPAT_SECRET_KEY")
		}
		return ak, sk
	}
	return testAccessKey, testSecretKey
}

// assertS3Error fails the test unless err is an S3 error with the given code
// (and, when the HTTP response is reachable, the given HTTP status).
func assertS3Error(t *testing.T, err error, wantCode string, wantStatus int) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s error, got success", wantCode)
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want S3 APIError with code %s, got %T: %v", wantCode, err, err)
	}
	if apiErr.ErrorCode() != wantCode {
		t.Fatalf("error code = %s, want %s", apiErr.ErrorCode(), wantCode)
	}
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.Response.StatusCode != wantStatus {
		t.Fatalf("%s returned HTTP %d, want %d", wantCode, respErr.Response.StatusCode, wantStatus)
	}
}

// TestVerify_Authentication pins the auth contract: correctly signed
// requests are accepted, and both rejection paths a misconfigured client can
// hit — wrong secret and unknown access key — fail closed with the standard
// S3 error codes and HTTP 403, as does an unsigned request.
func TestVerify_Authentication(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := compatBucket(t)
	key := "verify/conformance/auth.bin"
	body := conformancePayload(4096)

	// Positive control: correctly signed requests are accepted.
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &key, Body: bytes.NewReader(body),
	}); err != nil {
		t.Fatalf("PutObject with valid credentials: %v", err)
	}

	accessKey, secretKey := sdkCredentials(t)

	// Wrong secret: the SigV4 signature cannot validate.
	badSecret := newSDKClientWithCredentials(t, endpoint, accessKey, secretKey+"-corrupted")
	_, err := badSecret.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	assertS3Error(t, err, "SignatureDoesNotMatch", http.StatusForbidden)

	// Unknown access key.
	unknownKey := newSDKClientWithCredentials(t, endpoint, "ARMORUNKNOWNKEY", secretKey)
	_, err = unknownKey.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	assertS3Error(t, err, "InvalidAccessKeyId", http.StatusForbidden)

	// Unsigned request: no Authorization header at all.
	resp, err := http.Get(endpoint + "/" + bucket + "/" + key)
	if err != nil {
		t.Fatalf("unsigned GET: %v", err)
	}
	defer resp.Body.Close()
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unsigned GET returned HTTP %d, want 403", resp.StatusCode)
	}
	t.Logf("authentication OK: valid credentials accepted; wrong secret -> SignatureDoesNotMatch, unknown key -> InvalidAccessKeyId, unsigned -> 403")
}

// TestVerify_RangeReads pins byte-range reads on a single-PUT object: exact
// windows, block-boundary crossings (the 64 KiB seekable-encryption block),
// open-ended and suffix forms, and clamping when the end exceeds the object.
func TestVerify_RangeReads(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := compatBucket(t)
	key := "verify/conformance/range.bin"

	payload := conformancePayload(200_000) // spans three 64 KiB blocks
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &key, Body: bytes.NewReader(payload),
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	getRange := func(spec string, want []byte) {
		t.Helper()
		r := spec
		out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key, Range: &r})
		if err != nil {
			t.Fatalf("GetObject Range %q: %v", spec, err)
		}
		got, err := io.ReadAll(out.Body)
		out.Body.Close()
		if err != nil {
			t.Fatalf("read body Range %q: %v", spec, err)
		}
		if len(got) != len(want) || !bytes.Equal(got, want) {
			t.Fatalf("Range %q: got %d bytes (content mismatch), want %d", spec, len(got), len(want))
		}
	}

	getRange("bytes=0-0", payload[:1])
	getRange("bytes=0-65535", payload[:65536])           // exactly one block
	getRange("bytes=65530-65550", payload[65530:65551])  // crosses a block boundary
	getRange("bytes=100000-", payload[100000:])          // open-ended
	getRange("bytes=-1024", payload[len(payload)-1024:]) // suffix
	getRange("bytes=199990-200100", payload[199990:])    // end beyond EOF clamps

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket, Key: &key, Range: awsString("bytes=65530-65550"),
	})
	if err != nil {
		t.Fatalf("ContentLength probe: %v", err)
	}
	io.Copy(io.Discard, out.Body)
	out.Body.Close()
	wantLen := int64(len(payload[65530:65551]))
	if out.ContentLength == nil || *out.ContentLength != wantLen {
		t.Fatalf("ContentLength = %v, want %d", out.ContentLength, wantLen)
	}
	t.Logf("range reads OK: 6 specs byte-exact across block boundaries, suffix and clamped forms")
}

// TestVerify_List pins listing: uploaded objects appear with their correct
// sizes, prefix filters scope the results, and HEAD agrees with LIST.
func TestVerify_List(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := compatBucket(t)

	want := map[string]int{
		"verify/conformance/list/alpha":        1000,
		"verify/conformance/list/beta":         20000,
		"verify/conformance/list/nested/gamma": 333,
	}
	for key, size := range want {
		k := key
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket, Key: &k, Body: bytes.NewReader(conformancePayload(size)),
		}); err != nil {
			t.Fatalf("PutObject %s: %v", key, err)
		}
	}

	list := func(prefix string) map[string]int {
		t.Helper()
		p := prefix
		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &p})
		if err != nil {
			t.Fatalf("ListObjectsV2 prefix %q: %v", prefix, err)
		}
		got := make(map[string]int, len(out.Contents))
		for _, obj := range out.Contents {
			size := 0
			if obj.Size != nil {
				size = int(*obj.Size)
			}
			got[*obj.Key] = size
		}
		return got
	}

	all := list("verify/conformance/list/")
	for key, size := range want {
		if got, ok := all[key]; !ok {
			t.Errorf("list missing %s", key)
		} else if got != size {
			t.Errorf("list size for %s = %d, want %d", key, got, size)
		}
	}
	if len(all) != len(want) {
		t.Errorf("list returned %d keys, want %d: %v", len(all), len(want), all)
	}

	nested := list("verify/conformance/list/nested")
	if len(nested) != 1 {
		t.Fatalf("nested prefix returned %d keys, want 1: %v", len(nested), nested)
	}
	if nested["verify/conformance/list/nested/gamma"] != want["verify/conformance/list/nested/gamma"] {
		t.Fatalf("nested prefix size mismatch: %v", nested)
	}

	// HEAD must agree with LIST on size.
	size := want["verify/conformance/list/beta"]
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: awsString("verify/conformance/list/beta")})
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if head.ContentLength == nil || *head.ContentLength != int64(size) {
		t.Fatalf("HeadObject size = %v, want %d", head.ContentLength, size)
	}
	t.Logf("listing OK: %d objects with correct sizes, prefix scoping, HEAD agreement", len(want))
}

// TestVerify_Overwrite pins overwrite semantics: a second PUT to the same
// key replaces the object wholesale — new content, new length — with no
// residue of the previous version.
func TestVerify_Overwrite(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := compatBucket(t)
	key := "verify/conformance/overwrite.bin"

	v1 := conformancePayload(100_000)
	v2 := conformancePayload(30_000) // different length and content

	k := key
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &k, Body: bytes.NewReader(v1),
	}); err != nil {
		t.Fatalf("first PutObject: %v", err)
	}
	getAll := func() []byte {
		t.Helper()
		out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &k})
		if err != nil {
			t.Fatalf("GetObject: %v", err)
		}
		got, err := io.ReadAll(out.Body)
		out.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return got
	}
	if got := getAll(); !bytes.Equal(got, v1) {
		t.Fatalf("before overwrite: got %d bytes (content mismatch), want %d", len(got), len(v1))
	}

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &k, Body: bytes.NewReader(v2),
	}); err != nil {
		t.Fatalf("overwriting PutObject: %v", err)
	}
	if got := getAll(); !bytes.Equal(got, v2) {
		t.Fatalf("after overwrite: got %d bytes (content mismatch), want %d — overwrite left residue or served a stale version", len(got), len(v2))
	}
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: &k})
	if err != nil {
		t.Fatalf("HeadObject after overwrite: %v", err)
	}
	if head.ContentLength == nil || *head.ContentLength != int64(len(v2)) {
		t.Fatalf("HeadObject size after overwrite = %v, want %d", head.ContentLength, len(v2))
	}
	t.Logf("overwrite OK: %d-byte v1 fully replaced by %d-byte v2", len(v1), len(v2))
}

// TestVerify_Delete pins deletion: DELETE removes the object (GET 404s with
// NoSuchKey, HEAD with NotFound, LIST omits it) and is idempotent — deleting
// a missing key succeeds, as S3 clients are entitled to assume.
func TestVerify_Delete(t *testing.T) {
	endpoint := startArmorServer(t)
	client := newSDKClient(t, endpoint)
	ctx := context.Background()
	bucket := compatBucket(t)
	key := "verify/conformance/delete.bin"

	k := key
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &k, Body: bytes.NewReader(conformancePayload(4096)),
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &k}); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	_, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &k})
	assertS3Error(t, err, "NoSuchKey", http.StatusNotFound)

	if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: &k}); err == nil {
		t.Fatalf("HeadObject after delete succeeded, want NotFound")
	} else {
		var apiErr smithy.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NotFound" {
			t.Fatalf("HeadObject after delete: want NotFound, got %v", err)
		}
	}

	p := "verify/conformance/delete.bin"
	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &p})
	if err != nil {
		t.Fatalf("ListObjectsV2 after delete: %v", err)
	}
	if len(out.Contents) != 0 {
		t.Fatalf("list still returns deleted key: %v", out.Contents)
	}

	// Idempotency: deleting an already-deleted key must succeed.
	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &k}); err != nil {
		t.Fatalf("idempotent DeleteObject: %v", err)
	}
	t.Logf("delete OK: GET NoSuchKey, HEAD NotFound, LIST empty, second DELETE idempotent")
}

// awsString is a tiny helper for the &str pattern the SDK inputs need.
func awsString(s string) *string { return &s }
