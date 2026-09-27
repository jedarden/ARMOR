package awsclicompat

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/server"
)

// TestVerify_MultiKeyRoutingLifecycle pins the end-to-end multi-key contract:
// route selection happens from the client key for both single-PUT and
// multipart writes, all routed objects survive an active-key rotation through
// the configured rings, and the normal S3 lifecycle remains byte-exact.
func TestVerify_MultiKeyRoutingLifecycle(t *testing.T) {
	if isCompatEndpointMode() {
		t.Skip("multi-key routing conformance requires the in-process backend to inspect stored MEKs")
	}

	ctx := context.Background()
	bucket := "multi-key-compat-bucket"
	oldDefault := testMEKWithValue(0x11)
	oldPII := testMEKWithValue(0x22)
	oldArchive := testMEKWithValue(0x33)
	newDefault := testMEKWithValue(0x44)
	newPII := testMEKWithValue(0x55)
	newArchive := testMEKWithValue(0x66)

	store := newMockBackend()
	oldServer := startMultiKeyRoutingServer(t, store, multiKeyRoutingConfig(
		oldDefault,
		map[string][]byte{"pii": oldPII, "archive": oldArchive},
		nil,
	))
	client := newSDKClient(t, oldServer.URL)

	objects := map[string]multiKeyObject{
		"public/single.bin": {
			body: conformancePayload(100_000), keyName: "default", mek: oldDefault,
		},
		"pii/single.bin": {
			body: conformancePayload(90_000), keyName: "pii", mek: oldPII,
		},
		"archive/single.bin": {
			body: conformancePayload(80_000), keyName: "archive", mek: oldArchive,
		},
	}
	for key, object := range objects {
		key, object := key, object
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket, Key: &key, Body: bytes.NewReader(object.body),
		}); err != nil {
			t.Fatalf("single-PUT %s: %v", key, err)
		}
		assertStoredMultiKeyMetadata(t, store, bucket, key, object.keyName, object.mek)
	}

	multipartObjects := map[string]multiKeyObject{
		"pii/multipart.bin": {
			body: multipartPayload(5*1024*1024, 20_000), keyName: "pii", mek: oldPII,
		},
		"archive/multipart.bin": {
			body: multipartPayload(5*1024*1024, 21_000), keyName: "archive", mek: oldArchive,
		},
	}
	for key, object := range multipartObjects {
		key, object := key, object
		putMultiKeyMultipart(t, client, bucket, key, object.body[:5*1024*1024], object.body[5*1024*1024:])
		assertStoredMultiKeyMetadata(t, store, bucket, key+".armor-manifest", object.keyName, object.mek)
	}

	// Replace the active key set while retaining every key that encrypted the
	// objects above in the corresponding retired ring.
	oldServer.Close()
	rotatedServer := startMultiKeyRoutingServer(t, store, multiKeyRoutingConfig(
		newDefault,
		map[string][]byte{"pii": newPII, "archive": newArchive},
		map[string][]byte{"default": oldDefault, "pii": oldPII, "archive": oldArchive},
	))
	client = newSDKClient(t, rotatedServer.URL)

	for key, object := range objects {
		assertSDKObject(t, client, bucket, key, object.body)
	}
	for key, object := range multipartObjects {
		assertSDKObject(t, client, bucket, key, object.body)
	}

	// Ranges deliberately cross the 64 KiB encryption boundary on both a
	// single-PUT object and a multipart object after the key-ring rotation.
	assertSDKRange(t, client, bucket, "public/single.bin", objects["public/single.bin"].body[65_530:65_551])
	assertSDKRange(t, client, bucket, "archive/multipart.bin", multipartObjects["archive/multipart.bin"].body[65_530:65_551])

	// Overwrites use the new active MEK selected by the same route, while an
	// untouched object continues to carry the retired MEK fingerprint.
	overwrites := map[string]multiKeyObject{
		"public/single.bin": {
			body: conformancePayload(70_000), keyName: "default", mek: newDefault,
		},
		"pii/single.bin": {
			body: conformancePayload(71_000), keyName: "pii", mek: newPII,
		},
		"archive/single.bin": {
			body: conformancePayload(72_000), keyName: "archive", mek: newArchive,
		},
	}
	for key, object := range overwrites {
		key, object := key, object
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucket, Key: &key, Body: bytes.NewReader(object.body),
		}); err != nil {
			t.Fatalf("overwrite %s: %v", key, err)
		}
		objects[key] = object
		assertStoredMultiKeyMetadata(t, store, bucket, key, object.keyName, object.mek)
		assertSDKObject(t, client, bucket, key, object.body)
	}
	assertStoredMultiKeyMetadata(t, store, bucket, "archive/multipart.bin.armor-manifest", "archive", oldArchive)

	// Delete a single-PUT object and verify the S3 read path no longer exposes
	// it. (Multipart objects retain their manifest/ciphertext pair until their
	// dedicated lifecycle cleanup path is exercised elsewhere.)
	deleteKey := "archive/single.bin"
	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &deleteKey}); err != nil {
		t.Fatalf("DeleteObject %s: %v", deleteKey, err)
	}
	_, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &deleteKey})
	assertS3Error(t, err, "NoSuchKey", 404)
	if _, err := store.Head(ctx, bucket, deleteKey); err == nil {
		t.Fatalf("backend still contains deleted object %s", deleteKey)
	}
	delete(objects, deleteKey)

	// Final verification exercises the ordinary HEAD and full GET contracts
	// for every surviving object and proves that ring-backed and newly written
	// objects are still cryptographically bound to their intended MEKs.
	for key, object := range objects {
		assertStoredMultiKeyMetadata(t, store, bucket, key, object.keyName, object.mek)
		assertSDKObject(t, client, bucket, key, object.body)
	}
	for key, object := range multipartObjects {
		assertStoredMultiKeyMetadata(t, store, bucket, key+".armor-manifest", object.keyName, object.mek)
		assertSDKObject(t, client, bucket, key, object.body)
	}
	t.Logf("multi-key routing OK: %d objects across single-PUT, multipart, range, overwrite, delete, and ring-rotation verification", len(objects)+len(multipartObjects))
}

type multiKeyObject struct {
	body    []byte
	keyName string
	mek     []byte
}

func multiKeyRoutingConfig(defaultMEK []byte, namedKeys, rings map[string][]byte) *config.Config {
	return &config.Config{
		B2Region:  testRegion,
		MEK:       defaultMEK,
		NamedKeys: namedKeys,
		KeyRings:  rings,
		KeyRoutes: []config.KeyRoute{
			{Prefix: "pii/", KeyName: "pii"},
			{Prefix: "archive/", KeyName: "archive"},
			{Prefix: "*", KeyName: "default"},
		},
		BlockSize:          65_536,
		CacheMaxEntries:    1000,
		CacheTTL:           300,
		AuthAccessKey:      testAccessKey,
		AuthSecretKey:      testSecretKey,
		FormatWriteVersion: 2,
		Credentials: map[string]*config.Credential{
			testAccessKey: {
				AccessKey: testAccessKey,
				SecretKey: testSecretKey,
			},
		},
	}
}

func startMultiKeyRoutingServer(t *testing.T, store *mockBackend, cfg *config.Config) *httptest.Server {
	t.Helper()
	srv, err := server.NewWithBackend(cfg, store)
	if err != nil {
		t.Fatalf("NewWithBackend: %v", err)
	}
	httpServer := httptest.NewServer(srv.Handler())
	t.Cleanup(httpServer.Close)
	return httpServer
}

func putMultiKeyMultipart(t *testing.T, client *s3.Client, bucket, key string, parts ...[]byte) {
	t.Helper()
	ctx := context.Background()
	started, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucket, Key: &key,
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload %s: %v", key, err)
	}

	completed := make([]types.CompletedPart, 0, len(parts))
	for i, part := range parts {
		partNumber := int32(i + 1)
		partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket: &bucket, Key: &key, UploadId: started.UploadId,
			PartNumber: &partNumber, Body: bytes.NewReader(part),
		})
		if err != nil {
			t.Fatalf("UploadPart %s part %d: %v", key, partNumber, err)
		}
		completed = append(completed, types.CompletedPart{
			ETag: partResult.ETag, PartNumber: &partNumber,
		})
	}

	// Complete in reverse order so the test also covers part-number assembly
	// through the same path used by the compatibility suite's other multipart
	// conformance test.
	for left, right := 0, len(completed)-1; left < right; left, right = left+1, right-1 {
		completed[left], completed[right] = completed[right], completed[left]
	}
	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: &bucket, Key: &key, UploadId: started.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	}); err != nil {
		t.Fatalf("CompleteMultipartUpload %s: %v", key, err)
	}
}

func assertStoredMultiKeyMetadata(t *testing.T, store *mockBackend, bucket, storedKey, keyName string, mek []byte) {
	t.Helper()
	store.mu.Lock()
	meta := make(map[string]string, len(store.meta[bucket+"/"+storedKey]))
	for key, value := range store.meta[bucket+"/"+storedKey] {
		meta[key] = value
	}
	store.mu.Unlock()

	armorMeta, ok := backend.ParseARMORMetadata(meta)
	if !ok {
		t.Fatalf("stored object %s/%s has no ARMOR metadata", bucket, storedKey)
	}
	gotKeyName := armorMeta.KeyID
	if gotKeyName == "" {
		gotKeyName = "default"
	}
	if gotKeyName != keyName {
		t.Errorf("stored object %s/%s key ID = %q, want %q", bucket, storedKey, gotKeyName, keyName)
	}
	wantFingerprint := crypto.MEKFingerprint(mek)
	if armorMeta.MEKFingerprint != wantFingerprint {
		t.Errorf("stored object %s/%s fingerprint = %q, want %q", bucket, storedKey, armorMeta.MEKFingerprint, wantFingerprint)
	}
	if _, err := crypto.UnwrapDEK(mek, armorMeta.WrappedDEK); err != nil {
		t.Errorf("stored object %s/%s does not unwrap with routed MEK: %v", bucket, storedKey, err)
	}
}

func assertSDKObject(t *testing.T, client *s3.Client, bucket, key string, want []byte) {
	t.Helper()
	ctx := context.Background()
	out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		t.Fatalf("GetObject %s: %v", key, err)
	}
	got, readErr := io.ReadAll(out.Body)
	out.Body.Close()
	if readErr != nil {
		t.Fatalf("read GetObject %s: %v", key, readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("GetObject %s mismatch: got %d bytes, want %d", key, len(got), len(want))
	}

	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		t.Fatalf("HeadObject %s: %v", key, err)
	}
	if head.ContentLength == nil || *head.ContentLength != int64(len(want)) {
		t.Fatalf("HeadObject %s content length = %v, want %d", key, head.ContentLength, len(want))
	}
}

func assertSDKRange(t *testing.T, client *s3.Client, bucket, key string, want []byte) {
	t.Helper()
	// The two boundary assertions use a fixed range so their expected slices
	// remain tied to the source payload in the test body.
	rangeValue := "bytes=65530-65550"
	out, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket, Key: &key, Range: &rangeValue,
	})
	if err != nil {
		t.Fatalf("GetObject range %s %s: %v", key, rangeValue, err)
	}
	got, readErr := io.ReadAll(out.Body)
	out.Body.Close()
	if readErr != nil {
		t.Fatalf("read GetObject range %s: %v", key, readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("GetObject range %s mismatch: got %d bytes, want %d", key, len(got), len(want))
	}
}

func multipartPayload(first, second int) []byte {
	return append(conformancePayload(first), conformancePayload(second)...)
}

func testMEKWithValue(value byte) []byte {
	m := make([]byte, 32)
	for i := range m {
		m[i] = value
	}
	return m
}
