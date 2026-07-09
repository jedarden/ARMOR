# Dashboard Test Coverage Mapping

## Browser-UI Acceptance Behaviors

### ✅ Covered by Existing Tests

| Behavior | Test Name | Status | Notes |
|----------|-----------|--------|-------|
| Page renders at root | `TestDashboardHandler` | PASS | Lines 187-238, verifies "ARMOR Dashboard" title appears |
| Objects listed | `TestDashboardHandler` | PASS | Lines 187-238, checks `test/file1.txt` and `test/file2.txt` appear in response |
| Empty bucket renders sanely | `TestEmptyBucket` | PASS | Lines 1038-1069, verifies dashboard renders without errors, shows empty objects table, hides encryption coverage panel |

### ❌ Currently Failing Tests (Gaps in Coverage)

| Behavior | Test Name | Status | Issue |
|----------|-----------|--------|-------|
| Folder (commonPrefix) links navigate via ?prefix= | `TestCommonPrefixLinksNavigateByPrefix` | **FAIL** | Lines 1306-1346, expects `href="?prefix=folder1/"` but not found |
| Breadcrumbs link back up | `TestBreadcrumbLinksNavigateBack` | **FAIL** | Lines 1350-1402, expects breadcrumb links with `?prefix=` format |
| Common prefixes displayed | `TestCommonPrefixesDisplayed` | **FAIL** | Lines 1254-1302, expects `href="?prefix=data/"` format for folder links |

### Additional Test Coverage (Beyond Core Requirements)

| Area | Tests |
|------|-------|
| Prefix navigation | `TestDashboardHandlerWithPrefix` |
| ARMOR object display with badges | `TestARMORObjectDisplay` |
| Breadcrumbs displayed | `TestBreadcrumbs` |
| HTML structure | `TestDashboardHTMLStructure` |
| Content-Type headers | `TestDashboardContentType`, `TestMetricsContentType`, `TestObjectDetailContentType` |
| Authentication (Basic Auth & Bearer) | `TestAuthMiddlewareBasicAuth`, `TestAuthMiddlewareBearerToken`, `TestDashboardHandlerWithAuth`, `TestDashboardHandlerWithBearerToken`, `TestMetricsHandlerWithAuth`, `TestObjectDetailHandlerWithAuth` |
| Object detail endpoint | `TestObjectDetailHandler`, `TestObjectDetailHandlerNotFound`, `TestObjectDetailHandlerMissingKey` |
| Metrics endpoint | `TestMetricsHandler`, `TestMetricsHandlerComputedFields` |
| Encryption stats | `TestEncryptionStatsHandler`, `TestEncryptionStatsHandlerFolderExclusion` |
| Encryption coverage panel | `TestEncryptionCoveragePanelInDashboard`, `TestEncryptionCoveragePanelHiddenWhenEmpty`, `TestFullEncryptionCoverage` |
| List API endpoint | `TestListAPIHandlerRoot`, `TestListAPIHandlerWithPrefix`, `TestListAPIHandlerEncryptedVsPlain`, `TestListAPIHandlerWithAuth`, `TestListAPIHandlerMethodNotAllowed`, `TestListAPIHandlerListError` |
| Cache hit rate | `TestCacheHitRateCalculation`, `TestZeroCacheHitRate` |
| Canary status | `TestCanaryStatusNotStarted`, `TestCanaryStatusHealthy`, `TestCanaryStatusUnhealthy` |
| Error handling | `TestDashboardHandlerListError`, `TestObjectDetailHandlerNotFound` |
| Special characters in keys | `TestSpecialCharacterKeys` |
| Concurrent requests | `TestConcurrentRequests` |
| Template parsing | `TestTemplateParsing` |

## Gaps Identified

1. **Folder link navigation via ?prefix=** - Test exists (`TestCommonPrefixLinksNavigateByPrefix`) but FAILS because the HTML doesn't generate `?prefix=` links for folders
2. **Breadcrumb navigation** - Test exists (`TestBreadcrumbLinksNavigateBack`) but FAILS because breadcrumbs don't have `?prefix=` links
3. **Common prefix display** - Test exists (`TestCommonPrefixesDisplayed`) but FAILS for same reason

## Root Cause

All three failing tests expect folder links and breadcrumb links to use the format `href="?prefix=data/"` for navigation, but the current implementation doesn't generate these links in that format. The `?prefix=` query parameter is the expected mechanism for navigating folders in the browser UI.

## Test Patterns Used

- **mockBackend**: A mock backend implementation that supports `List()`, `Head()`, `Put()`, `Get()`, etc. with configurable objects, commonPrefixes, and error states
- **httptest**: Uses `httptest.NewRequest` and `httptest.NewRecorder` for HTTP handler testing
- **String checking**: Tests verify HTML output using `strings.Contains()` for key elements

## Recommendations

1. Fix the failing tests by implementing `?prefix=` link generation for:
   - Common prefix folder links in the object list
   - Breadcrumb navigation links
2. Consider adding acceptance tests for:
   - Sorting behavior (if applicable)
   - Pagination (if applicable)
   - Object detail link generation
