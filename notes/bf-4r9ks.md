# Dashboard Test Coverage Review and Testing Approach

**Bead:** bf-4r9ks  
**Related Inventory Bead:** bf-2qe2q  
**Date:** 2025-01-09

## Overview

This document reviews the existing dashboard test coverage, documents the mockBackend testing pattern, and identifies remaining behaviors to test.

## Test Coverage from bf-2qe2q Inventory

The inventory bead requested coverage for the following behaviors. Here's what's already covered:

| Behavior | Status | Test Names |
|----------|--------|------------|
| Page renders at root | ✅ COVERED | `TestDashboardHandler`, `TestDashboardHandlerListError`, `TestDashboardHTMLStructure`, `TestTemplateParsing` |
| Objects listed correctly | ✅ COVERED | `TestDashboardHandler`, `TestDashboardHandlerWithPrefix`, `TestCommonPrefixesDisplayed`, `TestListAPIHandlerRoot` |
| Folder (commonPrefix) links navigate via ?prefix= | ✅ COVERED | `TestCommonPrefixLinksNavigateByPrefix`, `TestCommonPrefixesDisplayed` |
| Breadcrumbs link back up the hierarchy | ✅ COVERED | `TestBreadcrumbLinksNavigateBack`, `TestBreadcrumbs` |
| Empty bucket renders sanely | ✅ COVERED | `TestEmptyBucket`, `TestEncryptionCoveragePanelHiddenWhenEmpty` |

**Conclusion:** All acceptance behaviors from bf-2qe2q are covered by existing tests.

## mockBackend Testing Pattern

The `dashboard_test.go` file uses a `mockBackend` struct that implements the full `backend.Backend` interface for testing. This is a clean, focused pattern.

### Pattern Structure

```go
type mockBackend struct {
    objects        map[string]*backend.ObjectInfo
    commonPrefixes []string
    listErr        error
    headErr        error
}
```

### Key Features

1. **Minimal Implementation**: Only the methods used by dashboard handlers are meaningfully implemented:
   - `List()`: Filters objects and commonPrefixes by prefix
   - `Head()`: Returns object info or error
   - `Put()`: Stores objects for testing (minimal implementation)
   - All other methods: Return nil or empty values

2. **Error Injection**: `listErr` and `headErr` fields allow testing error conditions:
   ```go
   mb.listErr = context.DeadlineExceeded
   // Tests backend failure handling
   ```

3. **Data Isolation**: Each test creates its own `mockBackend` instance:
   ```go
   mb := newMockBackend()
   mb.objects["test/file1.txt"] = &backend.ObjectInfo{...}
   ```

4. **HTTP Testing Pattern**: Uses `httptest` for request/response recording:
   ```go
   req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
   rec := httptest.NewRecorder()
   d.Handler()(rec, req)
   // Validate rec.Code, rec.Body.String(), rec.Header()
   ```

### Benefits of This Pattern

- **Fast**: No real S3 backend required
- **Deterministic**: Same inputs always produce same outputs
- **Isolated**: Tests don't share state
- **Clear**: Test setup makes expectations obvious

## Existing Test Categories

The test suite is well-organized into logical categories:

### 1. Core Dashboard Rendering (12 tests)
- Basic rendering at root
- Prefix-based filtering
- Empty bucket handling
- HTML structure validation
- Template parsing

### 2. ARMOR Encryption Display (10 tests)
- Encrypted vs plaintext object display
- Encryption stats endpoint
- Coverage panel rendering
- Key ID display
- Folder exclusion from stats

### 3. Folder Navigation (4 tests)
- Common prefix display
- Folder links with ?prefix= navigation
- Breadcrumb hierarchy navigation
- Multi-level folder support

### 4. Object Details API (3 tests)
- ARMOR object metadata
- Non-ARMOR object details
- Missing key error handling
- Not found errors

### 5. Metrics Endpoint (4 tests)
- Basic metrics JSON
- Cache hit rate calculation
- Computed fields (uptime, range bytes saved)

### 6. Authentication (9 tests)
- Basic Auth middleware
- Bearer token middleware
- No-auth configuration
- Authenticated dashboard/metrics/object handlers

### 7. List API (6 tests)
- JSON list endpoint at root
- Prefix filtering
- Encrypted vs plaintext handling
- Authentication
- Method not allowed
- Error handling

### 8. Key Rotation Admin API (5 tests)
- Rotation status endpoint
- Rotate endpoint
- Authentication
- Method not allowed
- Admin API failure handling

### 9. Utility Functions (3 tests)
- `formatBytes()`: Byte size formatting
- `formatUptime()`: Duration formatting
- `parseExpvarInt()`: Integer parsing

### 10. Edge Cases & Error Handling (7 tests)
- List errors (timeout, backend failure)
- Special characters in keys
- Concurrent requests
- Content-Type validation

### 11. Benchmarks (1 test)
- `BenchmarkDashboardHandler`: Performance testing

## Test Coverage Gaps

The test suite is comprehensive, but potential gaps exist in these areas:

### Potential Missing Tests

1. **Deep folder navigation edge cases**
   - Navigation with very deep folder hierarchies (10+ levels)
   - Folder names with special characters in ?prefix= URLs
   - Empty folders (commonPrefixes with no objects)

2. **Pagination/Continuation tokens**
   - No tests for `continuationToken` parameter in List()
   - No tests for maxKeys limits

3. **Large object lists**
   - Rendering with 1000+ objects
   - Performance at scale

4. **Object metadata edge cases**
   - Very long metadata values
   - Unicode in metadata
   - Missing ARMOR metadata fields on encrypted objects

5. **Concurrent modification**
   - What happens if backend state changes during render?

### Low Priority (Probably Not Needed)

1. **Template syntax errors** - Go's `text/template` validates at parse time
2. **JavaScript errors** - Dashboard is server-side rendered only
3. **CSS validation** - Visual testing, out of scope for unit tests
4. **Browser compatibility** - No client-side JS

## Testing Approach for New Tests

When writing new dashboard tests, follow this pattern:

### 1. Set up mockBackend
```go
mb := newMockBackend()
mb.objects["key"] = &backend.ObjectInfo{
    Key:              "key",
    Size:             100,
    ContentType:      "text/plain",
    LastModified:     time.Now(),
    IsARMOREncrypted: false,
    Metadata:         map[string]string{},
}
mb.commonPrefixes = []string{"folder1/", "folder2/"}
```

### 2. Create dashboard instance
```go
m := metrics.NewMetrics()
d := New(mb, "test-bucket", m)
```

### 3. Make HTTP request
```go
req := httptest.NewRequest(http.MethodGet, "/dashboard?prefix=data/", nil)
rec := httptest.NewRecorder()
d.Handler()(rec, req)
```

### 4. Validate response
```go
// Status code
if rec.Code != http.StatusOK {
    t.Errorf("Expected status 200, got %d", rec.Code)
}

// Response body
body := rec.Body.String()
if !strings.Contains(body, "expected text") {
    t.Error("Expected 'expected text' in response")
}

// Headers
ct := rec.Header().Get("Content-Type")
if ct != "text/html" {
    t.Errorf("Expected text/html, got %s", ct)
}

// For JSON responses
var result SomeStruct
if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
    t.Fatalf("Failed to decode JSON: %v", err)
}
if result.Field != expected {
    t.Errorf("Expected %v, got %v", expected, result.Field)
}
```

### 5. Test error conditions
```go
// Inject error
mb.listErr = errors.New("backend failed")
req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
rec := httptest.NewRecorder()
d.Handler()(rec, req)

// Should return error status
if rec.Code != http.StatusInternalServerError {
    t.Errorf("Expected 500, got %d", rec.Code)
}
```

## Test Naming Conventions

Current test naming is good and should be maintained:

- **Feature-focused**: `TestDashboardHandler`, `TestMetricsHandler`
- **Scenario-specific**: `TestEmptyBucket`, `TestFullEncryptionCoverage`
- **Error cases**: `TestDashboardHandlerListError`, `TestObjectDetailHandlerNotFound`
- **Authentication**: `TestDashboardHandlerWithAuth`, `TestAuthMiddlewareBearerToken`

## Recommendations

### For This Bead (bf-4r9ks)

Since this is planning/setup only:

1. ✅ **Current test coverage documented** - See "Test Coverage from bf-2qe2q Inventory" above
2. ✅ **mockBackend pattern understood and documented** - See "mockBackend Testing Pattern" above
3. ✅ **Clear list of remaining behaviors identified** - See "Test Coverage Gaps" above
4. ✅ **Test writing approach documented** - See "Testing Approach for New Tests" above

### For Future Beads (Test Writing)

When writing actual tests:

1. **Use table-driven tests** for multiple similar cases:
   ```go
   tests := []struct {
       name     string
       prefix   string
       expected []string
   }{
       {"root", "", []string{"file1.txt", "folder1/"}},
       {"data folder", "data/", []string{"data/file1.txt"}},
   }
   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) { ... })
   }
   ```

2. **Test helper functions** for common setup:
   ```go
   func setupTestDashboard(objects map[string]*backend.ObjectInfo, prefixes []string) (*Dashboard, *mockBackend) {
       mb := newMockBackend()
       for k, v := range objects {
           mb.objects[k] = v
       }
       mb.commonPrefixes = prefixes
       m := metrics.NewMetrics()
       return New(mb, "test-bucket", m), mb
   }
   ```

3. **Focus on user-facing behaviors**, not implementation details
   - Test that folders are clickable, not just that HTML contains `?prefix=`
   - Test that encrypted objects show ARMOR badge, not just specific CSS classes

4. **Add tests when bugs are found** - Before fixing a bug, write a failing test that reproduces it

## Conclusion

The dashboard test suite is comprehensive and well-structured. All acceptance behaviors from bf-2qe2q are covered. The mockBackend pattern is clean and effective. When writing new tests, follow the established patterns and focus on user-facing behaviors and edge cases.
