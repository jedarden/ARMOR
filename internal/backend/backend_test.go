package backend

import (
	"testing"
)

func TestListObjectVersionsResultTypes(t *testing.T) {
	// Test that the types are correctly defined and can be instantiated
	result := &ListObjectVersionsResult{
		Versions: []ObjectVersionInfo{
			{
				Key:            "test/object.txt",
				VersionID:      "v1",
				Size:           1024,
				ETag:           "\"abc123\"",
				IsLatest:       true,
				IsDeleteMarker: false,
			},
		},
		IsTruncated:         false,
		NextKeyMarker:       "",
		NextVersionIDMarker: "",
		CommonPrefixes:      []string{"prefix/"},
	}

	if len(result.Versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(result.Versions))
	}

	v := result.Versions[0]
	if v.Key != "test/object.txt" {
		t.Errorf("expected key 'test/object.txt', got %s", v.Key)
	}
	if v.VersionID != "v1" {
		t.Errorf("expected version ID 'v1', got %s", v.VersionID)
	}
	if v.Size != 1024 {
		t.Errorf("expected size 1024, got %d", v.Size)
	}
	if !v.IsLatest {
		t.Error("expected IsLatest to be true")
	}
	if v.IsDeleteMarker {
		t.Error("expected IsDeleteMarker to be false")
	}

	if len(result.CommonPrefixes) != 1 {
		t.Errorf("expected 1 common prefix, got %d", len(result.CommonPrefixes))
	}
	if result.CommonPrefixes[0] != "prefix/" {
		t.Errorf("expected common prefix 'prefix/', got %s", result.CommonPrefixes[0])
	}
}

func TestObjectVersionInfoDeleteMarker(t *testing.T) {
	// Test delete marker version info
	marker := ObjectVersionInfo{
		Key:            "deleted/object.txt",
		VersionID:      "v2",
		IsLatest:       true,
		IsDeleteMarker: true,
	}

	if !marker.IsDeleteMarker {
		t.Error("expected IsDeleteMarker to be true")
	}
	if marker.Size != 0 {
		t.Errorf("expected size 0 for delete marker, got %d", marker.Size)
	}
}
