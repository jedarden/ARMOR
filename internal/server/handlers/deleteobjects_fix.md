# DeleteObjects Per-Key ACL Enforcement Fix

## Problem
DeleteObjects (bulk delete via POST ?delete) has no per-key ACL enforcement. The authorization check happens before the request body is parsed, so it only validates bucket-level access. Once inside the handler, all keys in the XML body are deleted without individual ACL verification.

## Root Cause
1. In `server.go`, the ACL check at line 906 uses the URL path key, which is empty for DeleteObjects (POST /bucket?delete)
2. CheckACL with empty key only validates bucket-level access or ACLs with empty prefixes
3. The DeleteObjects handler (line 2221) receives the actual keys to delete from XML body
4. No per-key ACL verification happens for those keys

## Solution
1. Store credential in request context after successful authentication (completed)
2. Modify DeleteObjects handler to retrieve credential and check each key
3. Only delete keys that pass ACL checks
4. Return S3-compliant error response for denied keys

## Implementation
- Added `server_context.go` for context key management
- Modified `wrapHandler` to store credential in context
- Need to update `DeleteObjects` handler to perform per-key ACL checks

## S3 DeleteObjects Response Format
```xml
<DeleteResult>
  <Deleted>
    <Key>key1</Key>
  </Deleted>
  <Error>
    <Key>key2</Key>
    <Code>AccessDenied</Code>
    <Message>Access Denied</Message>
  </Error>
</DeleteResult>
```
