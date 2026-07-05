# bf-3ym4: Adjust ListObjectsV2 Prefix/Delimiter params for bucket-prefix offset

## Changes Made

Modified `/home/coding/ARMOR/internal/server/handlers/handlers.go` in the `ListObjectsV2` function to properly handle `ARMOR_PREFIX` configuration:

### 1. Apply prefix to backend list query
- Added `backendPrefix := h.applyPrefix(prefix)` before the backend list call
- This ensures that when ARMOR_PREFIX is configured, the backend searches in the correct prefixed namespace
- Cache lookups also use `backendPrefix` to maintain consistency

### 2. Strip prefix from returned results
- Applied `h.stripPrefix(obj.Key)` to object keys before adding to response
- Applied `h.stripPrefixFromCommonPrefix(p)` to common prefixes before adding to response
- This ensures clients receive keys without the internal ARMOR_PREFIX

## Why This Matters

When ARMOR_PREFIX is configured (e.g., `ARMOR_PREFIX="armor-prefix/"`):
- All objects are stored in B2 with the prefix prepended: `armor-prefix/file.parquet`
- But clients don't know about this prefix and request with their own keys: `file.parquet`
- Without this fix, listing with `prefix="data/"` would search for `data/` in B2 instead of `armor-prefix/data/`, returning empty results

## Testing

- Build succeeds with no errors
- Unit test `TestListObjectsV2` passes
