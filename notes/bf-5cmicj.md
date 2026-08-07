# stripPrefix Pattern in ListObjectsV2

## Overview
The `stripPrefix` pattern is used in ARMOR to support a configurable key prefix (via `ARMOR_PREFIX`) that is prepended to all objects stored in the backend, but transparent to S3 clients.

## Pattern Location
**File:** `internal/server/handlers/handlers.go`

**Function:** `ListObjectsV2` (lines 1630-1743)

## The Two-Phase Pattern

### Phase 1: Apply Prefix for Backend Query
```go
// Line 1647
backendPrefix := h.applyPrefix(prefix)
```

This transforms the client-provided prefix parameter by prepending the configured `ARMOR_PREFIX`. For example, if:
- `ARMOR_PREFIX="apps/prod/"`
- Client requests `prefix="data/"`

Then `backendPrefix` becomes `"apps/prod/data/"` for the backend query.

### Phase 2: Strip Prefix from Client Response
```go
// Lines 1713-1724
for _, obj := range result.Objects {
    // Strip the configured prefix from object keys before returning to client.
    // Clients don't know about the prefix, so we need to remove it from the keys.
    strippedKey := h.stripPrefix(obj.Key)
    resp.Contents = append(resp.Contents, Contents{
        Key:          strippedKey,
        // ... other fields
    })
}
```

For each object key returned from the backend, the configured prefix is stripped before including it in the S3 response.

### Phase 3: Strip Prefix from Common Prefixes
```go
// Lines 1726-1730
for _, p := range result.CommonPrefixes {
    // Strip the configured prefix from common prefixes before returning to client.
    // Common prefixes are used for directory-like listings with delimiters.
    strippedPrefix := h.stripPrefixFromCommonPrefix(p)
    resp.CommonPrefixes = append(resp.CommonPrefixes, CommonPrefix{Prefix: strippedPrefix})
}
```

When using a delimiter (e.g., `/`), S3 returns "common prefixes" representing directory-like groupings. These also need the prefix stripped.

## Helper Functions

### applyPrefix (lines 3071-3076)
```go
func (h *Handlers) applyPrefix(key string) string {
    if h.config.Prefix == "" {
        return key
    }
    return h.config.Prefix + key
}
```
**Purpose:** Prepends the configured prefix to a key for backend operations.

### stripPrefix (lines 3080-3088)
```go
func (h *Handlers) stripPrefix(key string) string {
    if h.config.Prefix == "" {
        return key
    }
    if strings.HasPrefix(key, h.config.Prefix) {
        return strings.TrimPrefix(key, h.config.Prefix)
    }
    return key
}
```
**Purpose:** Removes the configured prefix from a key for client responses. If the key doesn't start with the prefix, returns it unchanged (defensive programming).

### stripPrefixFromCommonPrefix (lines 3092-3100)
```go
func (h *Handlers) stripPrefixFromCommonPrefix(commonPrefix string) string {
    if h.config.Prefix == "" {
        return commonPrefix
    }
    if strings.HasPrefix(commonPrefix, h.config.Prefix) {
        return strings.TrimPrefix(commonPrefix, h.config.Prefix)
    }
    return commonPrefix
}
```
**Purpose:** Same as stripPrefix but named for clarity when handling common prefixes (directory paths ending in `/`).

## Key Insights

1. **Round-trip symmetry:** `applyPrefix()` and `stripPrefix()` are inverse operations. Tests verify this round-trip behavior.

2. **Defensive stripping:** Both strip functions return the key unchanged if:
   - No prefix is configured
   - The key doesn't start with the prefix
   
3. **Two places to strip in ListObjectsV2:**
   - Object keys (`result.Objects`)
   - Common prefixes (`result.CommonPrefixes`)

## Application to ListObjectVersions

The same pattern must be applied to `ListObjectVersions`:
1. Apply prefix to the backend query parameters (prefix, key-marker, version-id-marker)
2. Strip prefix from each object version's Key in the response
3. Strip prefix from any common prefixes in the response (when using delimiters)
4. Strip prefix from the DeleteMarker fields (if any)

## Test Coverage

The pattern is thoroughly tested in `internal/server/handlers/handlers_internal_test.go`:
- `TestApplyPrefix` - lines 10-78
- `TestStripPrefix` - lines 80-166
- `TestStripPrefixFromCommonPrefix` - lines 168-240
- `TestPrefixRoundTrip` - verifies apply/strip are inverses
