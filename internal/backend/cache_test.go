package backend

import (
	"testing"
	"time"
)

func TestListCache_GetSet(t *testing.T) {
	c := NewListCache(10, 60)

	result := &ListResult{
		Objects:        []ObjectInfo{{Key: "a/b.txt", Size: 100}},
		IsTruncated:    false,
		NextToken:      "",
		CommonPrefixes: []string{"a/"},
	}

	_, ok := c.Get("bucket", "a/", "/", 1000, "")
	if ok {
		t.Fatal("expected cache miss before Set")
	}

	c.Set("bucket", "a/", "/", 1000, "", result)

	got, ok := c.Get("bucket", "a/", "/", 1000, "")
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if len(got.Objects) != 1 || got.Objects[0].Key != "a/b.txt" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestListCache_KeyDistinction(t *testing.T) {
	c := NewListCache(10, 60)

	r1 := &ListResult{Objects: []ObjectInfo{{Key: "a.txt"}}}
	r2 := &ListResult{Objects: []ObjectInfo{{Key: "b.txt"}}}

	c.Set("bucket", "a/", "/", 100, "", r1)
	c.Set("bucket", "b/", "/", 100, "", r2)

	got1, ok := c.Get("bucket", "a/", "/", 100, "")
	if !ok || got1.Objects[0].Key != "a.txt" {
		t.Error("wrong result for prefix a/")
	}

	got2, ok := c.Get("bucket", "b/", "/", 100, "")
	if !ok || got2.Objects[0].Key != "b.txt" {
		t.Error("wrong result for prefix b/")
	}

	_, ok = c.Get("bucket", "a/", "/", 100, "tok")
	if ok {
		t.Error("different continuationToken should be a cache miss")
	}
}

func TestListCache_Expiry(t *testing.T) {
	c := NewListCache(10, 0) // TTL = 0 seconds → expires immediately
	// Set TTL to 1 nanosecond effectively by using 0s duration
	c.ttl = time.Nanosecond

	c.Set("bucket", "", "", 1000, "", &ListResult{})
	time.Sleep(2 * time.Millisecond)

	_, ok := c.Get("bucket", "", "", 1000, "")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestListCache_InvalidatePrefix(t *testing.T) {
	c := NewListCache(10, 60)

	c.Set("bucket", "a/", "/", 100, "", &ListResult{Objects: []ObjectInfo{{Key: "a/1.txt"}}})
	c.Set("bucket", "a/b/", "/", 100, "", &ListResult{Objects: []ObjectInfo{{Key: "a/b/2.txt"}}})
	c.Set("bucket", "c/", "/", 100, "", &ListResult{Objects: []ObjectInfo{{Key: "c/3.txt"}}})

	c.InvalidatePrefix("bucket", "a/")

	_, ok := c.Get("bucket", "a/", "/", 100, "")
	if ok {
		t.Error("a/ entry should have been invalidated")
	}

	_, ok = c.Get("bucket", "a/b/", "/", 100, "")
	if ok {
		t.Error("a/b/ entry should have been invalidated (shares prefix)")
	}

	_, ok = c.Get("bucket", "c/", "/", 100, "")
	if !ok {
		t.Error("c/ entry should not have been invalidated")
	}
}

func TestListCache_Clear(t *testing.T) {
	c := NewListCache(10, 60)

	c.Set("bucket", "a/", "/", 100, "", &ListResult{})
	c.Set("bucket", "b/", "/", 100, "", &ListResult{})

	c.Clear()

	_, ok := c.Get("bucket", "a/", "/", 100, "")
	if ok {
		t.Error("expected cache miss after Clear")
	}
}

func TestListCache_EvictionAtCapacity(t *testing.T) {
	c := NewListCache(2, 60)

	c.Set("b", "p1/", "", 100, "", &ListResult{})
	c.Set("b", "p2/", "", 100, "", &ListResult{})
	// Third entry must evict one of the first two
	c.Set("b", "p3/", "", 100, "", &ListResult{})

	total := 0
	for _, p := range []string{"p1/", "p2/", "p3/"} {
		if _, ok := c.Get("b", p, "", 100, ""); ok {
			total++
		}
	}
	if total > 2 {
		t.Errorf("expected at most 2 entries after eviction, got %d", total)
	}
}
