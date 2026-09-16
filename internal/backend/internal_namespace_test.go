package backend

// Tests for the dual-location internal-namespace listing filter (ADR-001
// "Internal Namespaces"). Once ARMOR_PREFIX is configured, isInternalKey must
// hide .armor/ objects at the bucket root — where a pre-prefix era left
// internal objects behind — AND at <prefix>.armor/, where the reserved
// namespace moves to. ListRaw keeps both locations visible to trusted
// internal callers (manifest, provenance, compactor).

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestIsInternalKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   bool
	}{
		// No prefix configured: only the bucket-root namespace is internal.
		{"root internal without a prefix", "", ".armor/manifest/w/delta.jsonl", true},
		{"visible key without a prefix", "", "data/file.txt", false},
		{"prefixed location is an ordinary key without a configured prefix", "", "tenant/.armor/state.json", false},
		{"near miss without the slash", "", ".armorx/state.json", false},
		{"nested .armor is not the reserved namespace", "", "data/.armor/state.json", false},

		// Prefix configured: both locations must read as internal.
		{"internal under the prefix", "tenant/", "tenant/.armor/manifest/w/delta.jsonl", true},
		{"bucket-root internal survives under a prefix", "tenant/", ".armor/manifest/legacy/delta.jsonl", true},
		{"visible prefixed key", "tenant/", "tenant/data/file.txt", false},
		{"near miss under the prefix", "tenant/", "tenant/.armorx/state.json", false},
		{"prefix must align at the key start", "tenant/", "xtenant/.armor/state.json", false},
		{"nested .armor under the prefix is not internal", "tenant/", "tenant/data/.armor/state.json", false},

		{"empty key with a prefix", "tenant/", "", false},
		{"empty key without a prefix", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInternalKey(tt.prefix, tt.key); got != tt.want {
				t.Errorf("isInternalKey(%q, %q) = %v, want %v", tt.prefix, tt.key, got, tt.want)
			}
		})
	}
}

// seedFSObjects uploads stored keys directly. Keys are STORED keys — the
// ADR-001 prefix, when configured, is already part of them, exactly as the
// handler layer writes them.
func seedFSObjects(t *testing.T, fs *FSBackend, bucket string, keys ...string) {
	t.Helper()
	ctx := context.Background()
	for _, key := range keys {
		payload := fmt.Sprintf("payload for %s", key)
		if err := fs.Put(ctx, bucket, key, strings.NewReader(payload), int64(len(payload)), nil); err != nil {
			t.Fatalf("Put(%q): %v", key, err)
		}
	}
}

func sortedObjectKeys(result *ListResult) []string {
	keys := make([]string, 0, len(result.Objects))
	for _, obj := range result.Objects {
		keys = append(keys, obj.Key)
	}
	sort.Strings(keys)
	return keys
}

func assertNoInternalKeys(t *testing.T, where string, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if strings.Contains(key, ".armor/") {
			t.Errorf("%s: internal key %q leaked into the listing", where, key)
		}
	}
}

func TestFSBackend_ListHidesInternalNamespaceFromBothLocations(t *testing.T) {
	fs, err := NewFSBackend(FSConfig{BasePath: t.TempDir(), KeyPrefix: "tenant/"})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	ctx := context.Background()
	const bucket = "shared-bucket"
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	seedFSObjects(t, fs, bucket,
		// Pre-prefix era internal objects, sitting at the bucket root.
		".armor/manifest/legacy-writer/delta-0000000001.jsonl",
		".armor/chain-head/legacy-writer",
		// Current-era internal objects, under the ADR-001 prefix.
		"tenant/.armor/manifest/current-writer/delta-0000000001.jsonl",
		// Visible objects from both eras, plus a near miss that must stay visible.
		"legacy-visible.txt",
		"tenant/data/file1.txt",
		"tenant/.armor-notes.txt",
	)

	wantVisible := []string{"legacy-visible.txt", "tenant/.armor-notes.txt", "tenant/data/file1.txt"}

	t.Run("full listing hides both locations", func(t *testing.T) {
		result, err := fs.List(ctx, bucket, "", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := sortedObjectKeys(result)
		assertNoInternalKeys(t, "full listing", got...)
		if !reflect.DeepEqual(got, wantVisible) {
			t.Errorf("List() keys = %v, want %v", got, wantVisible)
		}
	})

	t.Run("prefix listing under the configured prefix hides both locations", func(t *testing.T) {
		result, err := fs.List(ctx, bucket, "tenant/", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := sortedObjectKeys(result)
		assertNoInternalKeys(t, "tenant/ listing", got...)
		if !reflect.DeepEqual(got, []string{"tenant/.armor-notes.txt", "tenant/data/file1.txt"}) {
			t.Errorf("List(tenant/) keys = %v", got)
		}
	})

	t.Run("listing inside the root namespace returns nothing", func(t *testing.T) {
		result, err := fs.List(ctx, bucket, ".armor/", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(result.Objects) != 0 {
			t.Errorf("List(.armor/) returned %v, want none", sortedObjectKeys(result))
		}
	})

	t.Run("listing inside the prefixed namespace returns nothing", func(t *testing.T) {
		result, err := fs.List(ctx, bucket, "tenant/.armor/", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(result.Objects) != 0 {
			t.Errorf("List(tenant/.armor/) returned %v, want none", sortedObjectKeys(result))
		}
	})

	t.Run("delimiter listing emits no internal common prefixes", func(t *testing.T) {
		result, err := fs.List(ctx, bucket, "", "/", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		assertNoInternalKeys(t, "common prefixes", result.CommonPrefixes...)
		if !reflect.DeepEqual(result.CommonPrefixes, []string{"tenant/"}) {
			t.Errorf("CommonPrefixes = %v, want [tenant/]", result.CommonPrefixes)
		}
		if !reflect.DeepEqual(sortedObjectKeys(result), []string{"legacy-visible.txt"}) {
			t.Errorf("delimiter Contents = %v", sortedObjectKeys(result))
		}
	})

	t.Run("prefixed delimiter listing emits no internal common prefixes", func(t *testing.T) {
		result, err := fs.List(ctx, bucket, "tenant/", "/", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		assertNoInternalKeys(t, "common prefixes", result.CommonPrefixes...)
		if !reflect.DeepEqual(result.CommonPrefixes, []string{"tenant/data/"}) {
			t.Errorf("CommonPrefixes = %v, want [tenant/data/]", result.CommonPrefixes)
		}
	})

	t.Run("ListRaw still sees both locations", func(t *testing.T) {
		raw, err := fs.ListRaw(ctx, bucket, "", "", "", 0)
		if err != nil {
			t.Fatalf("ListRaw: %v", err)
		}
		got := sortedObjectKeys(raw)
		wantAll := []string{
			".armor/chain-head/legacy-writer",
			".armor/manifest/legacy-writer/delta-0000000001.jsonl",
			"legacy-visible.txt",
			"tenant/.armor-notes.txt",
			"tenant/.armor/manifest/current-writer/delta-0000000001.jsonl",
			"tenant/data/file1.txt",
		}
		if !reflect.DeepEqual(got, wantAll) {
			t.Errorf("ListRaw() keys = %v, want %v", got, wantAll)
		}
	})

	t.Run("version listing inherits the filter", func(t *testing.T) {
		versions, err := fs.ListObjectVersions(ctx, bucket, "", "", "", "", 0)
		if err != nil {
			t.Fatalf("ListObjectVersions: %v", err)
		}
		keys := make([]string, 0, len(versions.Versions))
		for _, v := range versions.Versions {
			keys = append(keys, v.Key)
		}
		sort.Strings(keys)
		assertNoInternalKeys(t, "versions", keys...)
		if !reflect.DeepEqual(keys, wantVisible) {
			t.Errorf("ListObjectVersions() keys = %v, want %v", keys, wantVisible)
		}
	})
}

// TestFSBackend_ListHidesOnlyRootNamespaceWithoutPrefix pins the no-prefix
// baseline: with no ADR-001 prefix configured, the bucket-root namespace stays
// hidden. Note the filesystem traversal prunes every directory named .armor by
// basename (see fs.list), so nested .armor/ subtrees disappear here too —
// broader than isInternalKey, which only matches the root and prefixed
// locations (the B2 backend follows the predicate).
func TestFSBackend_ListHidesOnlyRootNamespaceWithoutPrefix(t *testing.T) {
	fs, err := NewFSBackend(FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	ctx := context.Background()
	const bucket = "shared-bucket"
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	seedFSObjects(t, fs, bucket,
		".armor/state.json",
		"tenant/.armor/state.json",
		"tenant/data/file1.txt",
		"visible.txt",
	)

	result, err := fs.List(ctx, bucket, "", "", "", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := sortedObjectKeys(result)
	want := []string{"tenant/data/file1.txt", "visible.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() keys = %v, want %v", got, want)
	}
	assertNoInternalKeys(t, "full listing", got...)
}

// fakeS3Entry is one <Contents>/<Version>/<DeleteMarker> entry in a fake S3
// listing response.
type fakeS3Entry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag,omitempty"`
	Size         int64  `xml:"Size,omitempty"`
	StorageClass string `xml:"StorageClass,omitempty"`
	VersionID    string `xml:"VersionId,omitempty"`
	IsLatest     bool   `xml:"IsLatest,omitempty"`
}

type fakeS3ListResult struct {
	XMLName     xml.Name      `xml:"ListBucketResult"`
	Xmlns       string        `xml:"xmlns,attr"`
	Name        string        `xml:"Name"`
	Prefix      string        `xml:"Prefix"`
	KeyCount    int           `xml:"KeyCount"`
	IsTruncated bool          `xml:"IsTruncated"`
	Contents    []fakeS3Entry `xml:"Contents"`
}

type fakeS3VersionsResult struct {
	XMLName       xml.Name      `xml:"ListVersionsResult"`
	Xmlns         string        `xml:"xmlns,attr"`
	Name          string        `xml:"Name"`
	Prefix        string        `xml:"Prefix"`
	IsTruncated   bool          `xml:"IsTruncated"`
	Versions      []fakeS3Entry `xml:"Version"`
	DeleteMarkers []fakeS3Entry `xml:"DeleteMarker"`
}

// fakeS3Store mirrors a shared prefixed bucket: internal objects at both
// locations (bucket-root from a pre-prefix era, <prefix>.armor/ current),
// visible objects from both eras, plus a near miss that must stay visible.
var fakeS3Store = []fakeS3Entry{
	{Key: ".armor/chain-head/legacy-writer", LastModified: fakeS3Time, Size: 10},
	{Key: ".armor/manifest/legacy-writer/delta-0000000001.jsonl", LastModified: fakeS3Time, Size: 20},
	{Key: "legacy-visible.txt", LastModified: fakeS3Time, Size: 30},
	{Key: "tenant/.armor-notes.txt", LastModified: fakeS3Time, Size: 40},
	{Key: "tenant/.armor/manifest/current-writer/delta-0000000001.jsonl", LastModified: fakeS3Time, Size: 50},
	{Key: "tenant/data/file1.txt", LastModified: fakeS3Time, Size: 60},
}

// fakeS3VersionStore additionally exercises the version and delete-marker
// filters: internal objects appear in both lists, and one visible delete
// marker must survive.
var fakeS3VersionStore = struct {
	versions      []fakeS3Entry
	deleteMarkers []fakeS3Entry
}{
	versions: []fakeS3Entry{
		{Key: ".armor/manifest/legacy-writer/delta-0000000001.jsonl", VersionID: "1", IsLatest: true, LastModified: fakeS3Time, Size: 20},
		{Key: "legacy-visible.txt", VersionID: "2", IsLatest: true, LastModified: fakeS3Time, Size: 30},
		{Key: "tenant/.armor/manifest/current-writer/delta-0000000001.jsonl", VersionID: "3", IsLatest: true, LastModified: fakeS3Time, Size: 50},
		{Key: "tenant/data/file1.txt", VersionID: "4", IsLatest: true, LastModified: fakeS3Time, Size: 60},
	},
	deleteMarkers: []fakeS3Entry{
		{Key: ".armor/chain-head/legacy-writer", VersionID: "5", IsLatest: true, LastModified: fakeS3Time},
		{Key: "tenant/old.bin", VersionID: "6", IsLatest: true, LastModified: fakeS3Time},
	},
}

const fakeS3Time = "2026-09-16T00:00:00.000Z"

func scopedFakeS3Objects(entries []fakeS3Entry, prefix string) []fakeS3Entry {
	scoped := make([]fakeS3Entry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, prefix) {
			scoped = append(scoped, e)
		}
	}
	return scoped
}

// TestB2Backend_ListHidesInternalNamespaceFromBothLocations drives the
// production B2 listing path (the primary backend) against a fake S3 endpoint
// holding internal objects in both locations. The fake scopes its entries by
// the requested prefix the way a real S3 endpoint would, so the
// prefix-scoped assertions below are meaningful.
func TestB2Backend_ListHidesInternalNamespaceFromBothLocations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Query().Has("versions") {
			resp := fakeS3VersionsResult{
				Xmlns:         "http://s3.amazonaws.com/doc/2006-03-01/",
				Name:          "shared-bucket",
				Prefix:        prefix,
				Versions:      scopedFakeS3Objects(fakeS3VersionStore.versions, prefix),
				DeleteMarkers: scopedFakeS3Objects(fakeS3VersionStore.deleteMarkers, prefix),
			}
			_ = xml.NewEncoder(w).Encode(resp)
			return
		}
		contents := scopedFakeS3Objects(fakeS3Store, prefix)
		for i := range contents {
			contents[i].StorageClass = "STANDARD"
		}
		resp := fakeS3ListResult{
			Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:     "shared-bucket",
			Prefix:   prefix,
			KeyCount: len(contents),
			Contents: contents,
		}
		_ = xml.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	b, err := NewB2Backend(context.Background(), B2Config{
		Region:      "us-east-1",
		Endpoint:    srv.URL,
		AccessKeyID: "test-access",
		SecretKey:   "test-secret",
		KeyPrefix:   "tenant/",
	})
	if err != nil {
		t.Fatalf("NewB2Backend: %v", err)
	}

	ctx := context.Background()
	const bucket = "shared-bucket"

	t.Run("List hides both locations", func(t *testing.T) {
		result, err := b.List(ctx, bucket, "", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := sortedObjectKeys(result)
		assertNoInternalKeys(t, "full listing", got...)
		want := []string{"legacy-visible.txt", "tenant/.armor-notes.txt", "tenant/data/file1.txt"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("List() keys = %v, want %v", got, want)
		}
	})

	t.Run("List inside the prefixed namespace returns nothing", func(t *testing.T) {
		result, err := b.List(ctx, bucket, "tenant/.armor/", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(result.Objects) != 0 {
			t.Errorf("List(tenant/.armor/) returned %v, want none", sortedObjectKeys(result))
		}
	})

	t.Run("List inside the root namespace returns nothing", func(t *testing.T) {
		result, err := b.List(ctx, bucket, ".armor/", "", "", 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(result.Objects) != 0 {
			t.Errorf("List(.armor/) returned %v, want none", sortedObjectKeys(result))
		}
	})

	t.Run("ListRaw still sees both locations", func(t *testing.T) {
		raw, err := b.ListRaw(ctx, bucket, "", "", "", 0)
		if err != nil {
			t.Fatalf("ListRaw: %v", err)
		}
		got := sortedObjectKeys(raw)
		wantAll := []string{
			".armor/chain-head/legacy-writer",
			".armor/manifest/legacy-writer/delta-0000000001.jsonl",
			"legacy-visible.txt",
			"tenant/.armor-notes.txt",
			"tenant/.armor/manifest/current-writer/delta-0000000001.jsonl",
			"tenant/data/file1.txt",
		}
		if !reflect.DeepEqual(got, wantAll) {
			t.Errorf("ListRaw() keys = %v, want %v", got, wantAll)
		}
	})

	t.Run("ListObjectVersions filters versions and delete markers in both locations", func(t *testing.T) {
		result, err := b.ListObjectVersions(ctx, bucket, "", "", "", "", 0)
		if err != nil {
			t.Fatalf("ListObjectVersions: %v", err)
		}
		got := make([]string, 0, len(result.Versions))
		for _, v := range result.Versions {
			got = append(got, v.Key)
			if v.IsDeleteMarker && v.Key != "tenant/old.bin" {
				t.Errorf("delete marker for internal object %q survived the filter", v.Key)
			}
			if !v.IsDeleteMarker && v.Key == "tenant/old.bin" {
				t.Errorf("tenant/old.bin should surface as a delete marker, got a version")
			}
		}
		sort.Strings(got)
		assertNoInternalKeys(t, "versions", got...)
		want := []string{"legacy-visible.txt", "tenant/data/file1.txt", "tenant/old.bin"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ListObjectVersions() keys = %v, want %v", got, want)
		}
	})
}
