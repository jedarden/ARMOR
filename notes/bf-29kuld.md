# Bead bf-29kuld: ListObjectsV2 Prefix Handling

## Task
Add prefix handling to ListObjectsV2 handler

## Acceptance Criteria
- backend.List() receives ARMOR prefix as prefix parameter
- returned keys have ARMOR prefix stripped via stripPrefix()
- client receives unprefixed keys

## Status: ALREADY COMPLETE

The prefix handling was already fully implemented in the ListObjectsV2 handler at the time this bead was created.

## Implementation Details

**File:** `/home/coding/ARMOR/internal/server/handlers/handlers.go`

### 1. Prefix Application (Line 1647)
```go
backendPrefix := h.applyPrefix(prefix)
```
Applies the ARMOR_PREFIX configuration to the client-provided prefix parameter.

### 2. Backend Call with Prefixed Prefix (Line 1657)
```go
result, err = h.backend.List(ctx, bucket, backendPrefix, delimiter, contToken, maxKeys)
```
The backend.List() method receives the prefix with ARMOR_PREFIX prepended.

### 3. Key Stripping (Lines 1713-1724)
```go
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
Object keys returned from the backend have the ARMOR_PREFIX stripped before being sent to the client.

### 4. Common Prefix Stripping (Lines 1726-1731)
```go
for _, p := range result.CommonPrefixes {
    // Strip the configured prefix from common prefixes before returning to client.
    // Common prefixes are used for directory-like listings with delimiters.
    strippedPrefix := h.stripPrefixFromCommonPrefix(p)
    resp.CommonPrefixes = append(resp.CommonPrefixes, CommonPrefix{Prefix: strippedPrefix})
}
```
Common prefixes (for delimiter-based listings) also have the prefix stripped.

## Test Verification

Both prefix handling tests pass:
- `TestListObjectsV2WithPrefix` - Tests basic prefix filtering and stripping
- `TestListObjectsV2WithDelimiterAndPrefix` - Tests common prefixes with delimiters

## Related Code

**Prefix Utility Functions** (lines 3071-3100):
- `applyPrefix()` - Adds configured prefix to keys for backend operations
- `stripPrefix()` - Removes configured prefix from keys for client responses
- `stripPrefixFromCommonPrefix()` - Handles directory path prefix stripping

**Configuration** (`internal/config/config.go`):
- ARMOR_PREFIX environment variable
- `normalizePrefix()` ensures consistent prefix formatting (one trailing slash, no leading slash)

## Conclusion

This bead required no implementation work. The ListObjectsV2 handler already had complete, correct, and tested prefix handling at bead creation time.
