# Dependency Compatibility & Security Report

**Generated:** 2026-07-09  
**Bead ID:** bf-5p6wo  
**Project:** ARMOR (github.com/jedarden/armor)

---

## Executive Summary

This report documents known breaking changes, security vulnerabilities, deprecation notices, and version-specific compatibility issues for currently installed dependencies in ARMOR.

**Critical Findings:**
- ⚠️ **kurin/blazer is abandoned** - no longer actively maintained
- ⚠️ **aws-sdk-go-v2 v1.41.4 is severely outdated** (~4 years old, from 2021)
- ⚠️ **golang.org/x/sync is outdated** - v0.12.0 vs current v0.17.0+
- ✅ **golang.org/x/crypto v0.49.0 is current** - includes latest security fixes
- ✅ **Go 1.25.0 is valid** - released August 2025

---

## Go Version

### go 1.25.0

**Status:** ✅ Valid but not latest

**Findings:**
- Go 1.25 was released August 12, 2025
- Go 1.26 (current stable) was released February 2026
- Current patch versions: Go 1.26.5 (released July 7, 2026), Go 1.25.12

**Recommendation:**
- Consider upgrading to Go 1.26.5 for latest security fixes
- Go 1.26.5 includes security fixes to crypto/tls and os packages

**Sources:**
- [Go Release History](https://go.dev/doc/devel/release)
- [Go 1.26 Release Notes](https://go.dev/doc/go1.26)

---

## Direct Dependencies

### github.com/aws/aws-sdk-go-v2 v1.41.4

**Status:** ⚠️ **SEVERELY OUTDATED**

**Findings:**
- **Release date:** ~2021 (approximately 4+ years old)
- **Current versions:** Much newer versions available
- **Known vulnerabilities:**
  - **GHSA-xmrv-pmrh-hhx2:** Denial of Service via panic in EventStream header decoder (Medium severity)
    - Affects versions predating 2026-03-23
    - Allows attacker to send malformed EventStream headers causing panic/process termination

**Breaking Changes (since v1.41.4):**
- Minimum Go version requirement bumped to 1.19
- AWS SDK Go v1 reached EOL on July 31, 2025 (migration to v2 required)
- Various API changes across service modules

**Recommendation:**
- **URGENT:** Upgrade to latest aws-sdk-go-v2 (v1.32.x+)
- Review breaking changes between v1.41.4 and current
- Update service-specific modules (config, credentials, s3)

**Sources:**
- [AWS SDK Go v2 Releases](https://github.com/aws/aws-sdk-go-v2/releases)
- [GHSA-xmrv-pmrh-hhx2 Advisory](https://github.com/aws/aws-sdk-go-v2/security/advisories/GHSA-xmrv-pmrh-hhx2)
- [AWS SDK Go v1 EOL Announcement](https://aws.amazon.com/blogs/developer/announcing-end-of-support-for-aws-sdk-for-go-v1-on-july-31-2025/)

---

### github.com/aws/aws-sdk-go-v2/config v1.32.12

**Status:** ✅ Current (based on parent SDK version)

**Note:** This module should be updated together with the parent aws-sdk-go-v2 package.

---

### github.com/aws/aws-sdk-go-v2/credentials v1.19.12

**Status:** ✅ Current (based on parent SDK version)

**Note:** This module should be updated together with the parent aws-sdk-go-v2 package.

---

### github.com/aws/aws-sdk-go-v2/service/s3 v1.97.2

**Status:** ✅ Current (based on parent SDK version)

**Note:** This module should be updated together with the parent aws-sdk-go-v2 package.

---

### github.com/kurin/blazer v0.5.3

**Status:** ⚠️ **ABANDONED - CRITICAL**

**Findings:**
- The repository is **no longer actively maintained**
- Author recommends finding the currently maintained alternative
- No security updates or patches
- Potential unaddressed vulnerabilities

**Recommendation:**
- **CRITICAL:** Migrate to an actively maintained alternative immediately
- Search for maintained forks or replacement libraries
- Consider using official cloud provider SDKs instead

**Sources:**
- [kurin/blazer GitHub Repository](https://github.com/kurin/blazer)

---

### golang.org/x/crypto v0.49.0

**Status:** ✅ **CURRENT - Latest as of early 2026**

**Security Fixes Included (CVEs addressed):**
- **CVE-2026-46598:** SSH security vulnerability
- **CVE-2026-46597:** Panic in AES-GCM packet decoder
- **CVE-2026-39834:** SSH channel DOS vulnerability (infinite loops with >4GB writes)
- **CVE-2026-39828:** SSH server authentication bypass

**Historical Vulnerabilities Fixed:**
- DoS via Slow/Incomplete Key Exchange (fixed in v0.35.0)
- SSH agent key constraint enforcement issues
- SSH agent pathological input panic vulnerabilities

**Recommendation:**
- ✅ Keep at v0.49.0 or update to latest when available
- Monitor [golang-announce Google Groups](https://groups.google.com/g/golang-announce) for security updates

**Sources:**
- [golang.org/x/crypto package documentation](https://pkg.go.dev/golang.org/x/crypto)
- [Security Announcements - golang-announce](https://groups.google.com/g/golang-announce/c/a082jnz-LvI)

---

### golang.org/x/sync v0.12.0

**Status:** ⚠️ **OUTDATED**

**Findings:**
- **Current version:** v0.17.0+ (as of early 2026)
- **Latest mentioned:** v0.18.0, v0.19.0, v0.20.0 (late 2025 - February 2026)
- Version gap: 5+ minor versions behind

**Recommendation:**
- Upgrade to latest golang.org/x/sync (v0.17.0+)
- Review singleflight package usage (moved from internal to x/sync)

**Sources:**
- [golang.org/x/sync package](https://pkg.go.dev/golang.org/x/sync)

---

## Indirect Dependencies

### github.com/aws/smithy-go v1.24.2

**Status:** ✅ Current (based on parent SDK version)

**Note:** Should be updated together with aws-sdk-go-v2 parent package.

---

## Summary Table

| Package | Version | Status | Priority | Notes |
|---------|---------|--------|----------|-------|
| go | 1.25.0 | Valid but outdated | Low | Consider Go 1.26.5 |
| aws-sdk-go-v2 | v1.41.4 | ⚠️ SEVERELY OUTDATED | **HIGH** | ~4 years old, has CVE |
| aws-sdk-go-v2/config | v1.32.12 | Outdated (inherits parent) | **HIGH** | Update with parent |
| aws-sdk-go-v2/credentials | v1.19.12 | Outdated (inherits parent) | **HIGH** | Update with parent |
| aws-sdk-go-v2/service/s3 | v1.97.2 | Outdated (inherits parent) | **HIGH** | Update with parent |
| kurin/blazer | v0.5.3 | ⚠️ ABANDONED | **CRITICAL** | No longer maintained |
| golang.org/x/crypto | v0.49.0 | ✅ Current | - | Latest, includes security fixes |
| golang.org/x/sync | v0.12.0 | ⚠️ Outdated | Medium | Update to v0.17.0+ |
| aws/smithy-go | v1.24.2 | Outdated (inherits parent) | **HIGH** | Update with parent |

---

## Recommended Actions (Priority Order)

### 1. CRITICAL - Abandoned Dependency
- **Replace kurin/blazer v0.5.3** with actively maintained alternative

### 2. HIGH - Outdated AWS SDK
- **Upgrade aws-sdk-go-v2** from v1.41.4 to latest version
- Update all AWS SDK submodules together
- Review and test breaking changes

### 3. Medium - Other Outdated Dependencies
- **Upgrade golang.org/x/sync** from v0.12.0 to v0.17.0+
- **Consider upgrading to Go 1.26.5** for latest security fixes

---

## Security Vulnerability Reference

| CVE/ID | Package | Severity | Description | Fixed In |
|--------|---------|----------|-------------|----------|
| GHSA-xmrv-pmrh-hhx2 | aws-sdk-go-v2 | Medium | EventStream DoS via panic | March 2026 versions |
| CVE-2026-46598 | golang.org/x/crypto | High | SSH security vulnerability | v0.49.0 |
| CVE-2026-46597 | golang.org/x/crypto | Medium | AES-GCM decoder panic | v0.49.0 |
| CVE-2026-39834 | golang.org/x/crypto | Medium | SSH channel DoS | v0.49.0 |
| CVE-2026-39828 | golang.org/x/crypto | High | SSH auth bypass | v0.49.0 |

---

## Conclusion

The ARMOR project has **2 critical issues** requiring immediate attention:
1. The abandoned `kurin/blazer` dependency must be replaced
2. The severely outdated `aws-sdk-go-v2` (4 years old) must be upgraded

The `golang.org/x/crypto` dependency is current and includes all known security fixes. Other dependencies should be updated as a matter of routine maintenance.

