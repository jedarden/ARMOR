package restoreverifier

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// Exclusion of tenant prefixes (Config.ExcludePrefixes /
// VERIFIER_EXCLUDE_PREFIXES): a shared-bucket verifier holds one MEK domain, so discovery must
// skip candidates under other tenants' ARMOR_PREFIXes instead of picking keys
// no held MEK can unwrap. Reproduces the rs-manager shape (armor-bf592560):
// the tenant writer is both the most recent and the dominant writer, so
// without exclusion the unconditional latest-object verify false-alarms every
// cycle and the reservoir sample is spent on foreign objects.

// TestGetLatestObject_SkipsExcludedPrefixes: the newest object in the bucket
// sits under an excluded tenant prefix; the latest-object pick must fall
// through to the newest verifiable (root-namespace) object. The same
// population without an exclusion picks the tenant object — proving the
// fixture really models the incident, not a vacuous pass.
func TestGetLatestObject_SkipsExcludedPrefixes(t *testing.T) {
	now := time.Now()
	objects := []backend.ObjectInfo{
		{Key: "registry/blobs/sha256:aaa", LastModified: now.Add(-3 * time.Hour)},
		{Key: "nap/2026-09-18.parquet", LastModified: now.Add(-1 * time.Hour)},
		{Key: "tradegraph-platform/caches/historical-options/old.parquet", LastModified: now.Add(-30 * time.Minute)},
		{Key: "tradegraph-platform/caches/historical-options/new.parquet", LastModified: now.Add(-1 * time.Minute)},
	}

	// Control: with no exclusion the tenant writer (most recent) wins — the
	// pre-fix behavior the incident reported.
	mb := &paginatingBackend{objects: objects, pageSize: 2}
	vNoExcl := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{})
	latest, err := vNoExcl.getLatestObject(context.Background(), "test-bucket")
	if err != nil {
		t.Fatalf("getLatestObject (no exclusion): %v", err)
	}
	if !strings.HasPrefix(latest.Key, "tradegraph-platform/") {
		t.Fatalf("control: latest = %q, want a tradegraph-platform/ key (fixture must reproduce the incident)", latest.Key)
	}

	// With the tenant prefix excluded, the newest VERIFIABLE object wins.
	v := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{
		ExcludePrefixes: []string{"tradegraph-platform/"},
	})
	latest, err = v.getLatestObject(context.Background(), "test-bucket")
	if err != nil {
		t.Fatalf("getLatestObject (excluded tenant prefix): %v", err)
	}
	if latest.Key != "nap/2026-09-18.parquet" {
		t.Fatalf("latest = %q, want %q (newest non-excluded object)", latest.Key, "nap/2026-09-18.parquet")
	}
}

// TestGetLatestObject_AllCandidatesExcluded: when every non-internal object
// sits under excluded prefixes, latest-object discovery must fail with the
// same error an empty bucket produces — not pick a key the instance cannot
// verify.
func TestGetLatestObject_AllCandidatesExcluded(t *testing.T) {
	objects := []backend.ObjectInfo{
		{Key: ".armor/canary/latest", LastModified: time.Now()},
		{Key: "tradegraph-platform/caches/historical-options/a.parquet", LastModified: time.Now().Add(-time.Minute)},
		{Key: "tradegraph-platform/exports/b.parquet", LastModified: time.Now().Add(-2 * time.Minute)},
	}
	mb := &paginatingBackend{objects: objects, pageSize: 2}
	v := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{
		ExcludePrefixes: []string{"tradegraph-platform"},
	})
	if _, err := v.getLatestObject(context.Background(), "test-bucket"); err == nil {
		t.Fatal("getLatestObject succeeded with all candidates excluded, want the no-non-internal-objects error")
	}
}

// TestGetHistoricalSample_ExcludesTenantPrefixes: excluded candidates must
// never enter the reservoir. With sampleSize >= the number of verifiable
// objects the reservoir deterministically holds exactly the verifiable set,
// so any tenant key surviving into the sample is a failure.
func TestGetHistoricalSample_ExcludesTenantPrefixes(t *testing.T) {
	objects := make([]backend.ObjectInfo, 0, 24)
	for i := 0; i < 10; i++ {
		objects = append(objects, backend.ObjectInfo{
			Key:          fmt.Sprintf("nap/part-%04d.parquet", i),
			LastModified: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}
	for i := 0; i < 14; i++ {
		objects = append(objects, backend.ObjectInfo{
			Key:          fmt.Sprintf("tradegraph-platform/caches/historical-options/%04d.parquet", i),
			LastModified: time.Now().Add(-time.Duration(i) * time.Second),
		})
	}

	mb := &paginatingBackend{objects: objects, pageSize: 5}
	v := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{
		ExcludePrefixes: []string{"tradegraph-platform/"},
	})

	sample, err := v.getHistoricalSample(context.Background(), "test-bucket", 10)
	if err != nil {
		t.Fatalf("getHistoricalSample: %v", err)
	}
	if len(sample) != 10 {
		t.Fatalf("sample size = %d, want 10 (the full verifiable population)", len(sample))
	}
	for _, s := range sample {
		if strings.HasPrefix(s.Key, "tradegraph-platform/") {
			t.Fatalf("sample contains excluded tenant key %q", s.Key)
		}
	}
}

// TestExcludePrefixesNormalizationInNew: entries are normalized like
// ARMOR_PREFIX in New — surrounding whitespace, leading slashes and repeated
// trailing slashes all collapse to "tenant/" — and the trailing slash keeps
// the boundary exact so "tenant/" does not exclude a sibling "tenant-2/"
// namespace.
func TestExcludePrefixesNormalizationInNew(t *testing.T) {
	v := New(nil, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{
		ExcludePrefixes: []string{"  tradegraph-platform  ", "/needle-ledger//", "", "/"},
	})

	if !v.isExcludedKey("tradegraph-platform/caches/a") {
		t.Error("unnormalized entry ' tradegraph-platform ' must exclude tradegraph-platform/caches/a")
	}
	if !v.isExcludedKey("needle-ledger/events/2026") {
		t.Error("entry '/needle-ledger//' must exclude needle-ledger/events/2026")
	}
	if v.isExcludedKey("tradegraph-platform-2/caches/a") {
		t.Error("exclusion of tradegraph-platform/ must not match sibling namespace tradegraph-platform-2/")
	}
	if v.isExcludedKey("nap/part-0001.parquet") {
		t.Error("root-namespace key matched an exclusion")
	}
}

// TestExcludePrefixesMatchStoredKeys: exclusions apply to STORED keys — the
// keys List returns, which on a bucket configured with an ARMOR_PREFIX
// (BucketConfig.Prefix) carry that prefix. The tenant exclusion must
// therefore be spelled the stored way, "<bucket-prefix>/<tenant>/"; a
// client-spelled "tradegraph-platform/" matches no stored key and leaves the
// tenant object picked — the trap when the operator copies the namespace
// from the client's perspective.
func TestExcludePrefixesMatchStoredKeys(t *testing.T) {
	now := time.Now()
	objects := []backend.ObjectInfo{
		{Key: "nap-dashboard/nap/2026-09-18.parquet", LastModified: now.Add(-1 * time.Hour)},
		{Key: "nap-dashboard/tradegraph-platform/caches/historical-options/x.parquet", LastModified: now},
	}
	mb := &paginatingBackend{objects: objects, pageSize: 2}
	cfg := func(excl []string) Config {
		return Config{
			Buckets:         []BucketConfig{{Bucket: "test-bucket", Enabled: true, Prefix: "nap-dashboard/"}},
			ExcludePrefixes: excl,
		}
	}

	// Client-spelled exclusion (no bucket prefix): matches no stored key, so
	// the tenant object — the most recent writer — still wins.
	vClient := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, cfg([]string{"tradegraph-platform/"}))
	latest, err := vClient.getLatestObject(context.Background(), "test-bucket")
	if err != nil {
		t.Fatalf("getLatestObject (client-spelled exclusion): %v", err)
	}
	if !strings.HasPrefix(latest.Key, "nap-dashboard/tradegraph-platform/") {
		t.Fatalf("client-spelled exclusion: latest = %q, want the tenant object (exclusions match stored keys)", latest.Key)
	}

	// Stored-key spelling: the newest verifiable object under the bucket
	// prefix wins.
	vStored := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, cfg([]string{"nap-dashboard/tradegraph-platform/"}))
	latest, err = vStored.getLatestObject(context.Background(), "test-bucket")
	if err != nil {
		t.Fatalf("getLatestObject (stored-key exclusion): %v", err)
	}
	if latest.Key != "nap-dashboard/nap/2026-09-18.parquet" {
		t.Fatalf("stored-key exclusion: latest = %q, want %q", latest.Key, "nap-dashboard/nap/2026-09-18.parquet")
	}
}
