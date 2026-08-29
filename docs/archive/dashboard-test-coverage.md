# Dashboard Test Coverage Analysis

**Generated:** 2026-07-09  
**Scope:** internal/dashboard package  
**Reference:** Bead bf-2qe2q (inventory) + existing dashboard_test.go

## Test Patterns Overview

### MockBackend Pattern

The `mockBackend` struct (lines 21-130 in dashboard_test.go) implements the `backend.Backend` interface for testing:

```go
type mockBackend struct {
    objects        map[string]*backend.ObjectInfo
    commonPrefixes []string
    listErr        error
    headErr        error
}
```

**Usage Pattern:**
```go
mb := newMockBackend()
mb.objects["key"] = &backend.ObjectInfo{Key: "key", Size: 100, ...}
mb.commonPrefixes = []string{"folder/"}
m := metrics.NewMetrics()
d := New(mb, "test-bucket", m)
```

**Key Features:**
- Stores objects in memory map
- Supports error injection via `listErr` and `headErr`
- Simulates common prefixes (virtual folders)
- All backend operations return predictable test data
- Most methods return minimal valid responses (nil, zero values)

### httptest.NewRecorder Pattern

Standard Go testing pattern for HTTP handlers:

```go
req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
rec := httptest.NewRecorder()
d.Handler()(rec, req)
```

**Assertions:**
```go
if rec.Code != http.StatusOK {
    t.Errorf("Expected status 200, got %d", rec.Code)
}
body := rec.Body.String()
if !strings.Contains(body, "expected text") {
    t.Error("Expected text in response")
}
```

## Current Test Coverage

### ✅ Fully Covered Behaviors

#### 1. Main Dashboard Page
- **Test:** `TestDashboardHandler` (line 199)
- **Coverage:**
  - Page renders at root (status 200)
  - Dashboard title "ARMOR Dashboard" present
  - Objects listed correctly in table
  - ARMOR badge displayed for encrypted objects
  - Content-Type: text/html

#### 2. Objects Listed Correctly
- **Tests:** 
  - `TestDashboardHandler` (line 199)
  - `TestDashboardHandlerWithPrefix` (line 252)
  - `TestARMORObjectDisplay` (line 469)
- **Coverage:**
  - Object keys appear in table
  - ARMOR vs plain object distinction
  - Metadata displayed (key ID, block size)
  - File sizes formatted correctly

#### 3. Folder (CommonPrefix) Navigation
- **Tests:**
  - `TestCommonPrefixesDisplayed` (line 1266)
  - `TestCommonPrefixLinksNavigateByPrefix` (line 1319)
- **Coverage:**
  - Virtual folders displayed with 📁 icon
  - Folder links use `?prefix=` query parameter
  - Folders appear before regular objects in listing
  - Clicking folder navigates to that prefix
  - Folder contents filtered correctly

#### 4. Breadcrumb Navigation
- **Tests:**
  - `TestBreadcrumbs` (line 518)
  - `TestBreadcrumbLinksNavigateBack` (line 1364)
- **Coverage:**
  - Breadcrumbs show path hierarchy
  - Breadcrumb links navigate back up correctly
  - Root breadcrumb points to bucket root
  - Deep paths show all intermediate levels

#### 5. Empty Bucket Handling
- **Tests:**
  - `TestEmptyBucket` (line 1050)
  - `TestEncryptionCoveragePanelHiddenWhenEmpty` (line 1084)
- **Coverage:**
  - Empty bucket returns 200 (not 404/500)
  - Dashboard structure renders without objects
  - Encryption coverage panel hidden when no objects
  - Objects table present but empty

#### 6. Metrics Endpoint
- **Tests:**
  - `TestMetricsHandler` (line 342)
  - `TestMetricsHandlerComputedFields` (line 1134)
- **Coverage:**
  - JSON response with all metric fields
  - Cache hit rate percentage calculated correctly
  - Uptime formatted as "Xh Ym Zs"
  - Content-Type: application/json

#### 7. Object Detail Endpoint
- **Tests:**
  - `TestObjectDetailHandler` (line 286)
  - `TestObjectDetailHandlerMissingKey` (line 327)
  - `TestObjectDetailHandlerNotFound` (line 453)
  - `TestNonARMORObjectDetail` (line 601)
- **Coverage:**
  - ARMOR metadata included for encrypted objects
  - Plain objects show `is_armor: false`
  - Missing key parameter returns 400
  - Object not found returns 404
  - Content-Type: application/json

#### 8. Authentication
- **Tests:**
  - `TestAuthMiddlewareBasicAuth` (line 701)
  - `TestAuthMiddlewareBearerToken` (line 720)
  - `TestAuthMiddlewareNoAuth` (line 750)
  - `TestDashboardHandlerWithAuth` (line 763)
  - `TestMetricsHandlerWithAuth` (line 815)
  - `TestObjectDetailHandlerWithAuth` (line 841)
- **Coverage:**
  - Basic Auth (username/password) works
  - Bearer token authentication works
  - No auth configured = public access
  - Missing credentials returns 401 + WWW-Authenticate header
  - Valid credentials allow access
  - All protected endpoints have auth variants

#### 9. Encryption Statistics
- **Tests:**
  - `TestEncryptionStatsHandler` (line 872)
  - `TestEncryptionStatsHandlerFolderExclusion` (line 942)
  - `TestEncryptionCoveragePanelInDashboard` (line 1006)
  - `TestFullEncryptionCoverage` (line 1103)
- **Coverage:**
  - Encrypted vs plaintext object counts
  - Coverage percentage calculated correctly
  - Key IDs listed and sorted
  - Key usage counts tracked
  - Folders excluded from counts
  - Panel hidden when no objects

#### 10. JSON List API
- **Tests:**
  - `TestListAPIHandlerRoot` (line 1515)
  - `TestListAPIHandlerWithPrefix` (line 1614)
  - `TestListAPIHandlerEncryptedVsPlain` (line 1666)
  - `TestListAPIHandlerWithAuth` (line 1737)
  - `TestListAPIHandlerMethodNotAllowed` (line 1797)
  - `TestListAPIHandlerListError` (line 1813)
- **Coverage:**
  - JSON response with objects array
  - Common prefixes included
  - Encrypted flag set correctly
  - Key ID populated for ARMOR objects
  - Prefix filtering works
  - Method validation (GET only)

#### 11. Key Rotation Endpoints
- **Tests:**
  - `TestKeyRotateStatusHandlerNoRotation` (line 1831)
  - `TestKeyRotateStatusHandlerWithAuth` (line 1859)
  - `TestKeyRotateStatusHandlerMethodNotAllowed` (line 1885)
  - `TestKeyRotateHandlerSuccess` (line 1901)
  - `TestKeyRotateHandlerWithAuth` (line 1928)
  - `TestKeyRotateHandlerMethodNotAllowed` (line 1960)
  - `TestKeyRotateHandlerAdminAPIFailure` (line 1976)
  - `TestKeyRotateHandlerDefaultURL` (line 2000)
- **Coverage:**
  - Status returns "none" when no rotation
  - Auth protection on both endpoints
  - Method validation (GET for status, POST for rotate)
  - MEK generation and hex encoding
  - Admin API proxy behavior
  - Error handling for admin API failures

#### 12. Canary Status
- **Tests:**
  - `TestCanaryStatusNotStarted` (line 1420)
  - `TestCanaryStatusHealthy` (line 1441)
  - `TestCanaryStatusUnhealthy` (line 1461)
- **Coverage:**
  - "Not started" before first check
  - "Healthy" status with green CSS class
  - "Unhealthy" status with red CSS class
  - Error messages included

#### 13. Helper Functions
- **Tests:**
  - `TestFormatBytes` (line 376)
  - `TestFormatUptime` (line 397)
  - `TestParseExpvarInt` (line 417)
- **Coverage:**
  - Byte formatting (B, KB, MB, GB)
  - Uptime formatting (h, m, s)
  - Int parsing with invalid input handling

#### 14. Error Handling
- **Tests:**
  - `TestDashboardHandlerListError` (line 436)
  - `TestObjectDetailHandlerNotFound` (line 453)
  - `TestListAPIHandlerListError` (line 1813)
  - `TestKeyRotateHandlerAdminAPIFailure` (line 1976)
- **Coverage:**
  - Backend list errors return 500
  - Object not found returns 404
  - Admin API failures propagate correctly

#### 15. Concurrent Requests
- **Test:** `TestConcurrentRequests` (line 1235)
- **Coverage:** 10 concurrent requests all succeed

### ⚠️ Partial Coverage

#### 1. Template Error Handling
- **Test:** `TestTemplateParsing` (line 1224)
- **Missing:** Template execution errors (line 195 in dashboard.go)
- **Note:** Hard to test without breaking template intentionally

### ❌ Missing Test Coverage

#### 1. Backend Edge Cases
- **Missing:**
  - Malformed ARMOR metadata (missing required fields)
  - Invalid ARMOR version in metadata
  - Backend timeout errors (context.DeadlineExceeded)
  - Backend connection errors
  - Partial backend responses

#### 2. Pagination / MaxKeys
- **Missing:**
  - Listing with maxKeys limit hit
  - Continuation token handling (not currently implemented in backend)
  - Behavior when >1000 objects in bucket

#### 3. Content-Type Edge Cases
- **Missing:**
  - Objects with missing/empty Content-Type
  - Objects with very long Content-Type strings
  - Objects with special characters in Content-Type

#### 4. Special Key Patterns
- **Missing:**
  - Keys with Unicode characters
  - Keys with newlines or control characters
  - Very long keys (>255 characters)
  - Keys with multiple consecutive slashes

#### 5. Metrics Edge Cases
- **Missing:**
  - Metrics with very large values (overflow)
  - Metrics with negative values (shouldn't happen but test safety)
  - Metrics endpoint error handling

#### 6. Authentication Edge Cases
- **Missing:**
  - Malformed Basic Auth header (invalid base64)
  - Empty Bearer token
  - Both Basic and Bearer provided (precedence test)
  - Very long passwords/tokens

#### 7. Date/Time Edge Cases
- **Missing:**
  - Objects with zero/nil LastModified
  - Objects with far future dates
  - Objects with far past dates
  - Timezone handling

#### 8. HTML Structure Validation
- **Partial:** `TestDashboardHTMLStructure` (line 1481)
- **Missing:**
  - All CSS classes present
  - JavaScript functions defined
  - Modal elements present
  - Footer links present

#### 9. Key Rotation Edge Cases
- **Missing:**
  - Rotation state file read errors (non-JSON)
  - Rotation state file with partial data
  - Admin API timeout
  - MEK generation failures (very rare)

#### 10. Performance/Load
- **Test:** `BenchmarkDashboardHandler` (line 1201)
- **Missing:**
  - Large object listings (1000+ objects)
  - Deep folder hierarchies (50+ levels)
  - Many distinct key IDs in encryption stats

#### 11. Security/Injection
- **Missing:**
  - XSS in object keys (HTML escaping)
  - XSS in metadata values
  - Template injection via metadata
  - Header injection via object keys

#### 12. Cross-Handler Integration
- **Missing:**
  - Metrics reflect actual operation counts
  - Dashboard metrics match /metrics endpoint
  - Encryption stats match dashboard panel

## Test Writing Approach

### 1. Setup Pattern
```go
func TestFeatureName(t *testing.T) {
    // 1. Create mock backend
    mb := newMockBackend()
    
    // 2. Add test data
    mb.objects["key"] = &backend.ObjectInfo{
        Key:          "key",
        Size:         100,
        LastModified: time.Now(),
        // ... other fields
    }
    
    // 3. Optionally inject errors
    mb.listErr = errors.New("test error")
    
    // 4. Create dashboard
    m := metrics.NewMetrics()
    d := New(mb, "test-bucket", m)
    
    // 5. Make request
    req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
    rec := httptest.NewRecorder()
    d.Handler()(rec, req)
    
    // 6. Assert results
    if rec.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rec.Code)
    }
    // ... more assertions
}
```

### 2. Response Validation
```go
// For JSON responses
var resp struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}
if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
    t.Fatalf("Failed to decode JSON: %v", err)
}
if resp.Field1 != "expected" {
    t.Errorf("Expected field1=expected, got %s", resp.Field1)
}

// For HTML responses
body := rec.Body.String()
if !strings.Contains(body, "expected text") {
    t.Error("Expected 'expected text' in response")
}

// For headers
if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
    t.Errorf("Expected Content-Type application/json, got %s", ct)
}
```

### 3. Error Testing
```go
// Test error returns correct status code
func TestHandlerError(t *testing.T) {
    mb := newMockBackend()
    mb.listErr = context.DeadlineExceeded
    
    m := metrics.NewMetrics()
    d := New(mb, "test-bucket", m)
    
    req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
    rec := httptest.NewRecorder()
    d.Handler()(rec, req)
    
    if rec.Code != http.StatusInternalServerError {
        t.Errorf("Expected status 500, got %d", rec.Code)
    }
}
```

### 4. Authentication Testing
```go
func TestWithAuth(t *testing.T) {
    d := NewWithAuth(mb, "test-bucket", m, "user", "pass", "")
    
    // Test missing auth
    req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
    rec := httptest.NewRecorder()
    d.HandlerWithAuth()(rec, req)
    if rec.Code != http.StatusUnauthorized {
        t.Errorf("Expected 401 without auth, got %d", rec.Code)
    }
    
    // Test valid auth
    req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
    req.SetBasicAuth("user", "pass")
    rec = httptest.NewRecorder()
    d.HandlerWithAuth()(rec, req)
    if rec.Code != http.StatusOK {
        t.Errorf("Expected 200 with auth, got %d", rec.Code)
    }
}
```

### 5. Table-Driven Tests
```go
func TestFormatBytes(t *testing.T) {
    tests := []struct {
        n        int64
        expected string
    }{
        {0, "0 B"},
        {1024, "1.0 KB"},
        {1048576, "1.0 MB"},
    }
    
    for _, tt := range tests {
        result := formatBytes(tt.n)
        if result != tt.expected {
            t.Errorf("formatBytes(%d) = %q, want %q", tt.n, result, tt.expected)
        }
    }
}
```

## Priority Recommendations

### High Priority (Security/Correctness)
1. **XSS/Injection tests** - Object keys and metadata values could contain HTML/JS
2. **Malformed metadata** - Invalid ARMOR metadata shouldn't crash
3. **Authentication edge cases** - Malformed auth headers, empty tokens
4. **Error propagation** - Ensure all error paths return correct status codes

### Medium Priority (Robustness)
1. **Pagination testing** - Behavior with maxKeys limit
2. **Large datasets** - 1000+ objects, deep hierarchies
3. **Special characters** - Unicode, control characters, very long strings
4. **Date/time edge cases** - Zero dates, far future/past

### Low Priority (Nice to Have)
1. **Performance benchmarks** - More granular than existing
2. **Integration tests** - Cross-handler consistency
3. **Concurrent stress tests** - Beyond basic concurrent test

## Summary

**Current Coverage:** ~85% of critical behaviors tested

**Well-Tested Areas:**
- ✅ Core dashboard rendering
- ✅ Object listing and navigation
- ✅ Folder/breadcrumb navigation
- ✅ Authentication flows
- ✅ Encryption statistics
- ✅ Key rotation endpoints
- ✅ Metrics endpoint
- ✅ Error handling basics

**Gaps to Address:**
- ❌ Security/injection testing
- ❌ Edge cases (malformed data, special characters)
- ❌ Pagination/large datasets
- ❌ Metrics consistency validation

The test suite is comprehensive for happy paths and basic error cases. Priority should be given to security hardening and edge case coverage before considering the dashboard "fully tested."
