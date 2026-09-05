package config

import (
	"net/url"
	"strings"
)

// ParseBucketAliases parses an ARMOR_BUCKET_ALIASES value into the list stored
// on Config.BucketAliases.
//
// The value is a comma-separated list of legacy bucket names. Whitespace around
// a name is ignored, empty entries and duplicates are dropped, and a name equal
// to canonical is dropped too — it already resolves to the configured bucket,
// so recording it as an alias would only make the table claim a consolidation
// that has not happened.
func ParseBucketAliases(list, canonical string) []string {
	if strings.TrimSpace(list) == "" {
		return nil
	}

	seen := make(map[string]bool)
	var aliases []string
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" || name == canonical || seen[name] {
			continue
		}
		seen[name] = true
		aliases = append(aliases, name)
	}
	return aliases
}

// BucketKey is the bucket and key an S3 request names.
type BucketKey struct {
	// Bucket is the bucket exactly as the client named it. Empty when the
	// request named no bucket at all — a root GET, which is ListBuckets.
	Bucket string

	// Key is the URL-decoded object key. Empty for bucket-level requests.
	// DuckDB httpfs encodes special characters such as "=" as %3D.
	Key string
}

// IsBucketAlias reports whether name is a configured alias of Bucket.
func (c *Config) IsBucketAlias(name string) bool {
	if c == nil || name == "" {
		return false
	}
	for _, alias := range c.BucketAliases {
		if alias == name {
			return true
		}
	}
	return false
}

// ResolveBucket maps the bucket a client named onto the bucket ARMOR serves the
// request from. An alias and an empty name both resolve to Bucket; anything
// else — including a name matching no configured bucket — is returned
// unchanged, so an unknown bucket still fails in the backend exactly as it did
// before aliases existed.
//
// Every consumer of a request's bucket name must go through here. Matching on
// the raw name instead means a consolidated tenant's objects vanish behind its
// old bucket name, and ACL entries written against Bucket stop matching for
// clients that still send the legacy name.
func (c *Config) ResolveBucket(name string) string {
	if c == nil {
		return name
	}
	if name == "" || c.IsBucketAlias(name) {
		return c.Bucket
	}
	return name
}

// ParseBucketKey splits an S3 request URL into the bucket the client named and
// the object key. Only path-style addressing (/bucket/key) is recognised,
// because that is the only shape ARMOR documents support for; see
// extractBucketAndKey for why the Host header is deliberately not consulted.
func ParseBucketKey(urlPath string) BucketKey {
	// Remove the leading slash, then split on the first remaining slash:
	// "<bucket>/<key>" for an object request, "<bucket>" for a bucket-level
	// request, and "" for a root ListBuckets.
	bucket, key, _ := strings.Cut(strings.TrimPrefix(urlPath, "/"), "/")
	if decoded, err := url.PathUnescape(key); err == nil {
		key = decoded
	}
	return BucketKey{Bucket: bucket, Key: key}
}
