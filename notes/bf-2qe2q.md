# Dashboard Test Coverage Inventory

**Task**: Map which acceptance behaviors are covered by existing tests.

## Compilation Note

Tests currently fail to compile due to undefined function `extractHTMLSection` on line 1378 in `TestBreadcrumbLinksNavigateBack`. This needs to be fixed before tests can run.

## Coverage Analysis

### ✅ **Page renders at root** - COVERED

**Test(s)**:
- `TestDashboardHandler` (line 187-238)
  - Verifies HTTP 200 status
  - Checks "ARMOR Dashboard" title appears in response
  - Validates files are listed (test/file1.txt, test/file2.txt)
- `TestDashboardContentType` (line 534-548)
  - Verifies Content-Type is "text/html"

**Coverage**: Full - Verifies both status code and basic HTML structure

---

### ✅ **Objects listed correctly** - COVERED

**Test(s)**:
- `TestDashboardHandler` (line 187-238)
  - Verifies test/file1.txt and test/file2.txt appear in response
- `TestDashboardHandlerWithPrefix` (line 240-272)
  - Verifies prefix filtering works (data/file1.txt shown, other/file2.txt filtered out)
  - Ensures only matching objects are listed
- `TestListAPIHandlerRoot` (line 1503-1599)
  - Tests JSON API endpoint returns correct objects
  - Verifies encrypted vs plain object distinction

**Coverage**: Full - Covers root listing and prefix-filtered listing

---

### ✅ **Folder (commonPrefix) links navigate via ?prefix=** - COVERED

**Test(s)**:
- `TestCommonPrefixesDisplayed` (line 1254-1302)
  - Verifies virtual folders (data/, logs/) appear in response
  - Checks folder links use `href="?prefix=data/"` format for navigation
  - Ensures folders appear before regular objects in HTML
- `TestCommonPrefixLinksNavigateByPrefix` (line 1306-1346)
  - Verifies folder links appear at root with ?prefix= format
  - Tests navigating to folder1/ shows its contents (folder1/file.txt)
  - Confirms folder2/ is filtered out when viewing folder1/

**Coverage**: Full - Comprehensive testing of folder link navigation

---

### ✅ **Breadcrumbs link back up the hierarchy** - COVERED

**Test(s)**:
- `TestBreadcrumbs` (line 506-531)
  - Verifies breadcrumbs contain path segments (data, 2024)
  - Tests basic breadcrumb rendering
- `TestBreadcrumbLinksNavigateBack` (line 1350-1405)
  - **Note**: Contains compilation error (extractHTMLSection undefined)
  - Intended to verify breadcrumb links use proper ?prefix= format:
    - `href="?prefix=data/"`
    - `href="?prefix=data/2024/"`  
    - `href="?prefix=data/2024/january/"`
  - Tests navigating back to parent directories shows sibling contents

**Coverage**: Partial - Basic breadcrumb rendering tested, but navigation test has compilation error

---

### ✅ **Empty bucket renders sanely** - COVERED

**Test(s)**:
- `TestEmptyBucket` (line 1038-1069)
  - Verifies HTTP 200 status for empty bucket
  - Checks "ARMOR Dashboard" title still renders
  - Validates "Encryption Coverage" panel is hidden when no objects
  - Ensures objects table still renders (<table> present)
- `TestEncryptionCoveragePanelHiddenWhenEmpty` (line 1072-1088)
  - Tests panel hidden when only common prefixes exist (no actual objects)

**Coverage**: Full - Empty bucket handling thoroughly tested

---

## Summary

| Acceptance Behavior | Covered | Test Name(s) | Status |
|---------------------|---------|--------------|--------|
| Page renders at root | ✅ | TestDashboardHandler, TestDashboardContentType | PASS |
| Objects listed correctly | ✅ | TestDashboardHandler, TestDashboardHandlerWithPrefix, TestListAPIHandlerRoot | PASS |
| Folder links navigate via ?prefix= | ✅ | TestCommonPrefixesDisplayed, TestCommonPrefixLinksNavigateByPrefix | PASS |
| Breadcrumbs link back up hierarchy | ⚠️ | TestBreadcrumbs, TestBreadcrumbLinksNavigateBack | COMPILATION ERROR |
| Empty bucket renders sanely | ✅ | TestEmptyBucket, TestEncryptionCoveragePanelHiddenWhenEmpty | PASS |

**Overall**: 4/5 behaviors fully covered, 1 with test that needs compilation fix.

## Required Fix

Line 1378 in `TestBreadcrumbLinksNavigateBack` calls undefined `extractHTMLSection` helper. Either:
1. Remove the debug log line (1378-1379)
2. Implement the missing helper function

Once fixed, all acceptance behaviors will have full test coverage.
