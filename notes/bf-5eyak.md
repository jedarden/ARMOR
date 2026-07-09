# Test Run Results for bf-5eyak

## Full Test Suite

**Date:** 2026-07-09

**Result:** ✅ ALL TESTS PASS

```
ok  	github.com/jedarden/armor/cmd/armor-decrypt	(cached)
ok  	github.com/jedarden/armor/internal/b2keys	(cached)
ok  	github.com/jedarden/armor/internal/backend	(cached)
ok  	github.com/jedarden/armor/internal/canary	(cached)
ok  	github.com/jedarden/armor/internal/config	(cached)
ok  	github.com/jedarden/armor/internal/crypto	(cached)
ok  	github.com/jedarden/armor/internal/dashboard	0.015s
ok  	github.com/jedarden/armor/internal/keymanager	(cached)
ok  	github.com/jedarden/armor/internal/logging	(cached)
ok  	github.com/jedarden/armor/internal/manifest	(cached)
ok  	github.com/jedarden/armor/internal/metrics	(cached)
ok  	github.com/jedarden/armor/internal/presign	(cached)
ok  	github.com/jedarden/armor/internal/provenance	(cached)
ok  	github.com/jedarden/armor/internal/server	(cached)
ok  	github.com/jedarden/armor/internal/server/handlers	(cached)
ok  	github.com/jedarden/armor/internal/yamlutil	(cached)
```

## Dashboard Tests (internal/dashboard)

**Total tests:** 58 tests
**Result:** ✅ ALL PASS
**Duration:** 0.015s

### Test Coverage Areas:
- **Dashboard Handlers:** TestDashboardHandler, TestDashboardHandlerWithPrefix, TestDashboardHandlerListError, TestEmptyBucket
- **Object Detail:** TestObjectDetailHandler, TestObjectDetailHandlerMissingKey, TestObjectDetailHandlerNotFound, TestNonARMORObjectDetail
- **Metrics:** TestMetricsHandler, TestMetricsHandlerComputedFields
- **Formatting:** TestFormatBytes, TestFormatUptime, TestParseExpvarInt
- **Encryption Stats:** TestEncryptionStatsHandler, TestEncryptionStatsHandlerFolderExclusion, TestEncryptionStatsHandlerAuth, TestEncryptionCoveragePanelInDashboard, TestEncryptionCoveragePanelHiddenWhenEmpty, TestFullEncryptionCoverage
- **Auth Middleware:** TestAuthMiddlewareBasicAuth, TestAuthMiddlewareBearerToken, TestAuthMiddlewareNoAuth, TestDashboardHandlerWithAuth, TestDashboardHandlerWithBearerToken, TestMetricsHandlerWithAuth, TestObjectDetailHandlerWithAuth
- **ARMOR Display:** TestARMORObjectDisplay, TestBreadcrumbs, TestSpecialCharacterKeys
- **Cache Metrics:** TestCacheHitRateCalculation, TestZeroCacheHitRate
- **Content Types:** TestDashboardContentType, TestMetricsContentType, TestObjectDetailContentType
- **New Stat Cards:** TestNewStatCardsInHTML, TestTemplateParsing
- **Concurrency:** TestConcurrentRequests
- **Navigation:** TestCommonPrefixesDisplayed, TestCommonPrefixLinksNavigateByPrefix, TestBreadcrumbLinksNavigateBack
- **Canary Status:** TestCanaryStatusNotStarted, TestCanaryStatusHealthy, TestCanaryStatusUnhealthy
- **HTML Structure:** TestDashboardHTMLStructure
- **List API:** TestListAPIHandlerRoot, TestListAPIHandlerWithPrefix, TestListAPIHandlerEncryptedVsPlain, TestListAPIHandlerWithAuth, TestListAPIHandlerMethodNotAllowed, TestListAPIHandlerListError
- **Key Rotate Status:** TestKeyRotateStatusHandlerNoRotation, TestKeyRotateStatusHandlerWithAuth, TestKeyRotateStatusHandlerMethodNotAllowed
- **Key Rotate:** TestKeyRotateHandlerSuccess, TestKeyRotateHandlerWithAuth, TestKeyRotateHandlerMethodNotAllowed, TestKeyRotateHandlerAdminAPIFailure, TestKeyRotateHandlerDefaultURL

## Summary

All 58 dashboard tests pass successfully, covering:
- All dashboard behaviors (list, metrics, object detail, encryption stats)
- Authentication middleware (basic auth, bearer tokens)
- Content type handling
- Navigation features (breadcrumbs, common prefixes)
- Error handling (missing keys, not found, list errors)
- Key rotation API endpoints
- Cache metrics and stat cards
- Canary status display
- Concurrent request handling
