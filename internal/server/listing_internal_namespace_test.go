package server

// Client-visible integration test for the dual-location internal-namespace
// listing filter (ADR-001 "Internal Namespaces"). With ARMOR_PREFIX set, the
// reserved namespace lives at <prefix>.armor/, while a bucket that gained its
// prefix after ARMOR had been writing to it still holds pre-prefix internal
// objects at the bucket root. Neither location may ever surface in a client
// listing.
//
// Unlike the backend unit tests this drives the full stack: real HTTP server,
// SigV4-authenticated S3 client, manifest writer producing genuine internal
// delta objects, and the same backend.List the public ListObjectsV2 endpoint
// serves from.

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/jedarden/armor/internal/config"
)

// newListingTestServer builds a server over a fresh filesystem store with the
// given ADR-001 prefix, the same env configuration a real shared-bucket tenant
// runs with. The manifest writer is left running so client uploads record ops;
// the test flushes it explicitly to land delta objects.
func newListingTestServer(t *testing.T, root, tenantPrefix, writerID, accessKey, secretKey string) *Server {
	t.Helper()

	const mek = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	pairs := []string{
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", root,
		"ARMOR_BUCKET", "shared-bucket",
		"ARMOR_MEK", mek,
		"ARMOR_AUTH_ACCESS_KEY", accessKey,
		"ARMOR_AUTH_SECRET_KEY", secretKey,
		"ARMOR_WRITER_ID", writerID,
		"ARMOR_PREFIX", tenantPrefix,
	}
	originals := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		originals[k] = os.Getenv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Setenv(%s): %v", k, err)
		}
	}
	// ARMOR_MANIFEST_PREFIX and the secondary backend must not leak in from
	// another test: the defaults are what a shared-bucket tenant runs with.
	optionalUnsets := []string{
		"ARMOR_MANIFEST_PREFIX",
		"ARMOR_SECONDARY_BACKEND",
		"ARMOR_SECONDARY_BACKEND_TYPE",
		"ARMOR_SECONDARY_BACKEND_PATH",
	}
	unsetOriginals := make(map[string]string, len(optionalUnsets))
	for _, k := range optionalUnsets {
		v, had := os.LookupEnv(k)
		unsetOriginals[k] = v
		if had {
			if err := os.Unsetenv(k); err != nil {
				t.Fatalf("Unsetenv(%s): %v", k, err)
			}
		}
	}

	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
		for k, v := range unsetOriginals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load(): %v", err)
	}
	if cfg.ManifestPrefix != tenantPrefix+".armor/manifest" {
		t.Fatalf("got ManifestPrefix %q, want %q", cfg.ManifestPrefix, tenantPrefix+".armor/manifest")
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if srv.manifestWriter == nil {
		t.Fatal("manifest writer is nil — internal objects would never be written")
	}

	t.Cleanup(srv.StopManifestCompactor)
	t.Cleanup(srv.StopManifestWriter)
	return srv
}

// TestClientListingsHideInternalNamespaceInBothLocations uploads through the
// public API (which creates real internal manifest state under the prefix),
// seeds bucket-root internal objects a pre-prefix era would have left behind,
// and then verifies from the client's seat that no listing exposes either
// location.
func TestClientListingsHideInternalNamespaceInBothLocations(t *testing.T) {
	const (
		bucket       = "shared-bucket"
		tenantPrefix = "p/"
		accessKey    = "test-access-key"
		secretKey    = "test-secret-key"
		visibleKey   = "listing/visible.bin"
		visibleBody  = "visible payload"
		writerID     = "writer-listing"
	)

	root := t.TempDir()
	srv := newListingTestServer(t, root, tenantPrefix, writerID, accessKey, secretKey)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	client := newListingS3Client(t, ts.URL, accessKey, secretKey)
	ctx := context.Background()

	// 1. A client upload. With the manifest enabled this enqueues an op;
	// stopping the writer drains it as a delta object at
	// p/.armor/manifest/<writer>/delta-*.jsonl — a genuine internal object in
	// the prefixed location, created by ordinary client activity.
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(visibleKey),
		Body:   bytes.NewReader([]byte(visibleBody)),
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	srv.StopManifestWriter()

	// 2. Internal objects at the bucket root, as a pre-prefix era leaves
	// behind. Written straight into the store because nothing writes there
	// once a prefix is configured.
	legacyDelta := filepath.Join(root, bucket, ".armor", "manifest", "legacy-writer", "delta-0000000001.jsonl")
	if err := os.MkdirAll(filepath.Dir(legacyDelta), 0o755); err != nil {
		t.Fatalf("mkdir legacy internal dir: %v", err)
	}
	if err := os.WriteFile(legacyDelta, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("seed legacy internal object: %v", err)
	}

	// Sanity: both internal locations genuinely exist in the store before any
	// listing is asserted on. Without this the assertions below could pass
	// vacuously.
	assertDirHasFiles(t, filepath.Join(root, bucket, ".armor"),
		"bucket-root .armor/ (pre-prefix internal objects)")
	assertDirHasFiles(t, filepath.Join(root, bucket, "p", ".armor"),
		"p/.armor/ (prefixed internal objects)")

	// 3. Client-visible listings must expose neither location.

	// Full bucket listing: exactly the visible object, nothing internal.
	list, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("ListObjectsV2: %v", err)
	}
	var listedKeys []string
	for _, obj := range list.Contents {
		listedKeys = append(listedKeys, aws.ToString(obj.Key))
	}
	for _, key := range listedKeys {
		if strings.Contains(key, ".armor/") {
			t.Errorf("internal key %q leaked into the full listing", key)
		}
	}
	if len(listedKeys) != 1 || listedKeys[0] != visibleKey {
		t.Errorf("full listing = %v, want [%s]", listedKeys, visibleKey)
	}

	// Delimiter listing: pruned internal directories must not resurface as
	// common prefixes.
	delim, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2(delimiter): %v", err)
	}
	for _, cp := range delim.CommonPrefixes {
		if strings.Contains(aws.ToString(cp.Prefix), ".armor") {
			t.Errorf("internal directory surfaced as common prefix %q", aws.ToString(cp.Prefix))
		}
	}
	if len(delim.CommonPrefixes) != 1 || aws.ToString(delim.CommonPrefixes[0].Prefix) != "listing/" {
		t.Errorf("delimiter common prefixes = %v, want [listing/]", delim.CommonPrefixes)
	}

	// A client asking for the reserved namespace by its client-visible name
	// gets nothing: the handler maps that request onto the stored prefixed
	// location (handlers.applyPrefix), and the filter empties it.
	internal, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(".armor/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2(prefix=.armor/): %v", err)
	}
	if len(internal.Contents) != 0 {
		t.Errorf("prefix=.armor/ listing returned %d objects, want 0", len(internal.Contents))
	}
	// Naming the stored form literally — prefix and all — must be equally
	// fruitless: the stored namespace is not part of the client key space.
	prefixedInternal, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(tenantPrefix + ".armor/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2(prefix=%s.armor/): %v", tenantPrefix, err)
	}
	if len(prefixedInternal.Contents) != 0 {
		t.Errorf("prefix=%s.armor/ listing returned %d objects, want 0",
			tenantPrefix, len(prefixedInternal.Contents))
	}

	// Versioned listing inherits the same filter (filesystem backend serves
	// versions from the same filtered listing).
	versions, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("ListObjectVersions: %v", err)
	}
	var versionKeys []string
	for _, v := range versions.Versions {
		versionKeys = append(versionKeys, aws.ToString(v.Key))
	}
	for _, key := range versionKeys {
		if strings.Contains(key, ".armor/") {
			t.Errorf("internal key %q leaked into the version listing", key)
		}
	}
	if len(versionKeys) != 1 || versionKeys[0] != visibleKey {
		t.Errorf("version listing = %v, want [%s]", versionKeys, visibleKey)
	}
}

// newListingS3Client builds an S3 client against the test server, signing
// with the credentials the server was configured with.
func newListingS3Client(t *testing.T, endpoint, accessKey, secretKey string) *s3.Client {
	t.Helper()

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
}

// assertDirHasFiles fails the test when dir holds no regular files anywhere
// below it, naming it with the given description in the failure message.
func assertDirHasFiles(t *testing.T, dir, description string) {
	t.Helper()
	found := false
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = true
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("%s: %v", description, walkErr)
	}
	if !found {
		t.Fatalf("%s: %s holds no files — the leak assertions below would pass vacuously", description, dir)
	}
}
