//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// headArmorManifest returns the descriptive ARMOR metadata of a completed
// multipart object, read through ARMOR from the object's ADR-016 manifest
// (<key>.armor-manifest).
//
// The manifest object is where ARMOR's format facts are client-visible. HEAD
// and GET of the logical object carry the client's own metadata only — a
// multipart object reads exactly like a single-PUT one — so a test that wants
// to know how an object was armored asks the manifest, not the object.
//
// Keys are the lowercase form aws-sdk-go-v2 produces ("armor-version", not
// "Armor-Version"): the SDK lowercases everything after the x-amz-meta-
// prefix, so the canonical-case lookups these tests used to make could never
// match, whatever the server returned.
func headArmorManifest(t *testing.T, ctx context.Context, client *s3.Client, bucketName, key string) map[string]string {
	t.Helper()
	resp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key + ".armor-manifest"),
	})
	if err != nil {
		t.Fatalf("HeadObject of the manifest for %s failed: %v", key, err)
	}
	for _, private := range []string{"armor-iv", "armor-wrapped-dek", "armor-key-id"} {
		if _, leaked := resp.Metadata[private]; leaked {
			t.Errorf("manifest HEAD for %s exposes x-amz-meta-%s; key material must stay private", key, private)
		}
	}
	return resp.Metadata
}

// requireNoArmorMetadata fails the test when a logical-object response
// carries any x-amz-meta-armor-* field: ARMOR's namespace is private on
// object reads.
func requireNoArmorMetadata(t *testing.T, what string, metadata map[string]string) {
	t.Helper()
	for k := range metadata {
		if len(k) >= 6 && k[:6] == "armor-" {
			t.Errorf("%s exposes x-amz-meta-%s; ARMOR metadata belongs on the manifest object only", what, k)
		}
	}
}
