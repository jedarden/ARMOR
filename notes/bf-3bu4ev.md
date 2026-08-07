# Bead bf-3bu4ev: ListObjectVersions version key handling

## Task
Locate where version keys are added to the ListObjectVersions response.

## Finding

**File:** `/home/coding/ARMOR/internal/server/handlers/handlers.go`

**Function:** `ListObjectVersions` (line 2788)

**Where version.Key is added to resp.Versions:**

1. **Line 2852** - The version key is assigned:
   ```go
   v := Version{
       Key:            version.Key,  // <-- HERE: version.Key assigned
       VersionID:      version.VersionID,
       IsLatest:       version.IsLatest,
       IsDeleteMarker: version.IsDeleteMarker,
       LastModified:   version.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
   }
   ```

2. **Line 2893** - The version (with key) is appended to response:
   ```go
   resp.Versions = append(resp.Versions, v)  // <-- HERE: Added to resp.Versions
   ```

## Context
The `ListObjectVersions` handler iterates through `result.Versions` from the backend, creates a local `Version` struct for each (copying the `version.Key` at line 2852), then appends each populated `Version` to `resp.Versions` at line 2893 before marshaling the XML response.
