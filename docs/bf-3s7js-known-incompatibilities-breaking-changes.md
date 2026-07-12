# Known Incompatibilities and Breaking Changes Research Report

**Bead ID:** bf-3s7js  
**Created:** 2026-07-12  
**Status:** ✅ COMPLETE  
**Research Scope:** ARMOR project dependencies and Pluck/NEEDLE integration  
**Research Date:** 2026-07-12

---

## Executive Summary

This report documents known incompatibilities and breaking changes across ARMOR's core dependencies, focusing on issues that could affect Pluck integration and production operations.

### Key Findings

| Category | Status | Issues | Action Required |
|----------|--------|--------|-----------------|
| **Breaking Changes** | ⚠️ **WARNING** | 2 specific issues | Testing recommended |
| **Security Vulnerabilities** | 🔴 **CRITICAL** | 27 CVEs (separate report) | Immediate upgrades required |
| **Known Incompatibilities** | ✅ **MINIMAL** | 1 TLS config issue | Workaround available |
| **Version Compatibility** | ✅ **GOOD** | No major compatibility issues | None |

---

## 1. Go Toolchain Breaking Changes

### 1.1 Go 1.25.0 → 1.25.12 Release Notes

#### **Major Changes in Go 1.25.0**
- **Maintains Go 1 compatibility promise**: No breaking changes for most codebases
- **Experimental JSON package**: `encoding/json/v2` and `encoding/json/jsontext`
- **New CSRF protection**: `net/http.CrossOriginProtection` for token-free CSRF protection
- **New sync.WaitGroup.Go method**: Simplified concurrent testing support
- **Runtime improvements**: New GOMAXPROCS design
- **Toolchain enhancements**: Various build tool improvements

**Source:** [Go 1.25 Release Notes - The Go Programming Language](https://go.dev/doc/go1.25)

#### **Security-Focused Updates (1.25.8, 1.25.12)**
The significant security changes came in later point releases, not 1.25.0:

**Go 1.25.8 (Released 2026-03-05):**
- Fixed **CVE-2026-27142**: URL escaping vulnerability in HTML `<meta>` tag attributes
- Additional security fixes to `html/template`, `net/url`, and `os` packages
- **Backward compatible**: No code changes required

**Sources:**
- [Release History - The Go Programming Language](https://go.dev/doc/devel/release)
- [html/template: fix bypass for CVE-2026-27142 · Issue #78913](https://github.com/golang/go/issues/78913)

### 1.2 Critical Breaking Change: TLS Config In-Place Modification

**🔴 CRITICAL INCOMPATIBILITY:**

**GitHub Issue #72100**: net/http breaks compatibility by modifying tls.Config in-place
- **Affected**: Go 1.24+ and 1.25.x series
- **Problem**: Go now modifies `tls.Config` objects in-place when they're shared between multiple servers
- **Impact**: Breaks applications that reuse TLS configs between net/http and gRPC servers
- **Example Scenario**: 
  ```go
  // BROKEN: Same config shared between servers
  config := &tls.Config{...}
  httpServer := &http.Server{TLSConfig: config}
  grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(config)))
  ```

**Workaround:**
```go
// FIXED: Create separate configs for each server
httpConfig := config.Clone()
grpcConfig := config.Clone()
```

**Source:** [GitHub Issue #72100: net/http breaks compatibility by modifying tls-config in-place](https://github.com/golang/go/issues/72100)

### 1.3 html/template XSS Security Issues

**🟠 HIGH SECURITY ISSUES:**

Multiple XSS vulnerabilities discovered in Go's `html/template` package:

**CVE-2026-27142:**
- **Vulnerability**: URLs not correctly escaped inside `<meta content>` attribute
- **Impact**: XSS attacks possible via meta refresh redirects
- **Fixed in**: Go 1.25.8
- **ARMOR Impact**: Dashboard uses `html/template` for admin interface

**Context Tracking Issues:**
- **GitHub Issue #78331**: JS template literal context incorrectly tracked
- **Problem**: Context not properly tracked across template branches for JavaScript template literals
- **Impact**: Possibly incorrect escaping of content in certain template scenarios

**Ongoing Hardening Efforts:**
- **GitHub Issue #27926**: Proposal for hardened `html/template` package
- **Status**: Discussions ongoing for security engineering best practices
- **Challenge**: Some fixes require breaking backward compatibility

**Sources:**
- [JS template literal context incorrectly tracked · Issue #78331](https://github.com/golang/go/issues/78331)
- [html/template: fix bypass for CVE-2026-27142 · Issue #78913](https://github.com/golang/go/issues/78913)
- [html/template: add a hardened version to standard library · Issue #27926](https://github.com/golang/go/issues/27926)

---

## 2. Rust Toolchain Breaking Changes

### 2.1 Rust 1.75 → 1.96 Compatibility

**✅ NO MAJOR BREAKING CHANGES:**

**Rust 1.96.0 Release (2026-05-28):**
- **Edition System**: Maintains backward compatibility through Rust's edition mechanism
- **No Interface Breaking**: "Most changes do not affect any public interfaces of Rust"
- **One Breaking Change**: C double type alignment fix (affects C interop only)

**Key Compatibility Features:**
- **Rust Edition Mechanism**: Allows adoption of new features while maintaining compatibility
- **MSRV Policies**: Major crates like tokio provide 6+ months notice for MSRV increases
- **Backward Compatibility**: Strong commitment to not breaking existing code

**Sources:**
- [Rust 1.96.0 is out : r/rust - Reddit](https://www.reddit.com/r/rust/comments/1tqd97o/rust_1960_is_out/)
- [Go, Backwards Compatibility, and GODEBUG](https://go.dev/doc/godebug)

### 2.2 Tokio MSRV Policy

**✅ STRONG COMPATIBILITY GUARANTEES:**

**Tokio Project Policy:**
- **Rolling MSRV**: At least 6 months notice before MSRV increases
- **MSRV as Breaking Change**: Project considers MSRV bumps as breaking changes
- **6-Month Rule**: New Rust version must be released 6+ months ago before MSRV increase
- **Gradual Increases**: MSRV changes happen gradually to minimize ecosystem disruption

**Current Status (Tokio 1.52.3):**
- **Compatible**: Works with Rust 1.75+ (ARMOR uses 1.96.1)
- **No Issues**: No known compatibility problems between Rust 1.75 and 1.96
- **Safe**: Large version buffer provides stability

**Sources:**
- [docs.rs: Tokio README](https://docs.rs/crate/tokio/latest/source/README.md)
- [Can you please document the minimum required Rust version... Issue #936](https://github.com/tokio-rs/tracing/issues/936)

### 2.3 Community MSRV Concerns

**⚠️ ECOSYSTEM CONCERNS (Not Affecting ARMOR):**

**Rust Community Discussions:**
- **MSRV Churn**: Some developers experience broken crates due to MSRV changes in transitive dependencies
- **Cargo.toml Changes**: No backward compatibility guarantees for Cargo.toml modifications
- **RFC 3537**: Ongoing discussion about MSRV resolver improvements

**Impact on ARMOR:**
- **Minimal**: ARMOR uses recent Rust 1.96.1, well above MSRV requirements
- **Future Consideration**: Monitor if NEEDLE increases MSRV beyond 1.75

**Sources:**
- [Rust forums: Rust version requirement change discussion](https://users.rust-lang.org/t/rust-version-requirement-change-as-semver-breaking-or-not/20980?page=2)
- [Reddit: A rant about MSRV](https://www.reddit.com/r/rust/comments/1jmcv5v/a_rant_about_msrv/)
- [RFC 3537 on MSRV resolver](https://rust-lang.github.io/rfcs/3537-msrv-resolver.html)

---

## 3. AWS SDK v2 Breaking Changes

### 3.1 S3 Service Breaking Changes

**🔴 SECURITY UPDATE (v1.97.2 → v1.97.3):**

**Version v1.97.3:**
- **Type**: Security update (DOSS vulnerability fix)
- **Service**: S3
- **Breaking Change**: No, backward compatible security fix
- **Action Required**: Upgrade for security (addressed in separate security report)

**Known Breaking Changes in AWS SDK v2:**
- **Data Integrity Checks**: Recent breaking changes related to AWS data integrity checks
- **GitHub Issues**: #6440, #6313 (data integrity issues)
- **Community Impact**: Some projects experienced compatibility problems

**Current Status (ARMOR):**
- **Version**: v1.97.2 → Needs upgrade to v1.97.3+
- **Compatibility**: Generally backward compatible
- **Testing Required**: S3/B2 operations verification post-upgrade

**Sources:**
- [CHANGELOG.md - aws-sdk-go-v2 S3 Service](https://github.com/aws/aws-sdk-go-v2/blob/main/service/s3/CHANGELOG.md)
- [Releases · aws/aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2/releases)
- [Pocketbase discussion on AWS SDK breaking changes](https://github.com/pocketbase/pocketbase/discussions/6562)

---

## 4. Dependency Version Gaps Analysis

### 4.1 Critical Gaps Identified

| Dependency | Current | Minimum | Secure | Gap | Status |
|------------|---------|---------|--------|-----|--------|
| **Go** | 1.25.0 | 1.25.0 | 1.25.12 | -0.12 | 🔴 UPGRADE REQUIRED |
| **AWS SDK S3** | v1.97.2 | - | v1.97.3+ | -0.1 | 🔴 UPGRADE REQUIRED |
| **Rust** | 1.96.1 | 1.75+ | - | +0.21.1 | ✅ EXCEEDS |
| **Tokio** | 1.52.3 | 1.x | - | Multiple minors | ✅ EXCEEDS |

### 4.2 Compatibility Impact Assessment

**Go 1.25.0 Issues:**
- **Security**: 27 vulnerabilities (detailed in separate report bf-4zsbd)
- **TLS Config**: Potential in-place modification issue
- **XSS**: Dashboard template vulnerabilities (fixed in 1.25.8+)
- **Compatibility**: Generally backward compatible

**AWS SDK v1.97.2 Issues:**
- **Security**: DoS vulnerability in EventStream decoder
- **Data Integrity**: Recent breaking changes in integrity checks
- **Compatibility**: Minor version update, should be compatible

**Rust/NEEDLE Stack:**
- **No Critical Issues**: All dependencies meet or exceed requirements
- **Strong Buffers**: Healthy version margins provide stability
- **MSRV Safe**: Current toolchain well above minimum requirements

---

## 5. Breaking Changes by Category

### 5.1 🔴 CRITICAL Breaking Changes

| Change | Component | Impact | Workaround | Status |
|--------|-----------|--------|------------|--------|
| **TLS Config In-Place Modification** | Go 1.24+ crypto/tls | Shared TLS configs broken | Clone configs separately | ⚠️ REVIEW NEEDED |
| **AWS SDK EventStream DoS** | service/s3 v1.97.2 | Service disruption | Upgrade to v1.97.3+ | 🔴 UPGRADE REQUIRED |

### 5.2 🟠 HIGH Breaking Changes

| Change | Component | Impact | Workaround | Status |
|--------|-----------|--------|------------|--------|
| **CVE-2026-27142 XSS** | html/template (Go 1.25.0) | Dashboard XSS | Upgrade to Go 1.25.8+ | 🟠 UPGRADE RECOMMENDED |
| **AWS Data Integrity Checks** | aws-sdk-go-v2 | Potential data issues | Review implementation | ⚠️ MONITOR |

### 5.3 🟡 MODERATE Breaking Changes

| Change | Component | Impact | Workaround | Status |
|--------|-----------|--------|------------|--------|
| **JS Template Context Tracking** | html/template | Incorrect escaping | Review templates | ⚠️ REVIEW |
| **C Double Alignment** | Rust 1.96 | C interop only | None for pure Rust | ✅ IGNORE |

---

## 6. Known Incompatibilities

### 6.1 TLS Config Sharing Incompatibility

**❌ KNOWN INCOMPATIBILITY:**

**Scenario**: Sharing TLS configurations between multiple HTTP/gRPC servers
- **Broke in**: Go 1.24+
- **Symptoms**: Unexpected TLS behavior, connection failures
- **Root Cause**: In-place modification of shared tls.Config objects
- **ARMOR Impact**: Review if ARMOR shares TLS configs

**Detection:**
```bash
# Search for potential TLS config sharing
grep -r "tls.Config" internal/
```

**Resolution**: Clone TLS configs for each server instance

### 6.2 html/template Context Tracking

**⚠️ KNOWN ISSUE:**

**Scenario**: JavaScript template literals in branching templates
- **Problem**: Context not tracked across template branches
- **Impact**: Potential XSS in complex templates
- **ARMOR Impact**: Review dashboard template complexity

**Detection:**
```bash
# Review dashboard templates for JS literals
grep -r "{{" internal/dashboard/ | grep -i "script\|js"
```

**Resolution**: Review and simplify complex template structures

---

## 7. Security Vulnerabilities Summary

**🔴 CRITICAL SECURITY ISSUES (Detailed in Separate Report):**

This research identified **27 security vulnerabilities** across ARMOR's dependencies, documented in comprehensive security report `bf-4zsbd-version-compatibility-report.md`:

**Critical Vulnerabilities:**
- **GO-2026-5856**: crypto/tls (CRITICAL)
- **GO-2026-5764**: aws-sdk-go-v2/service/s3 (CRITICAL - DoS)
- **GO-2026-5039**: net/textproto (HIGH)
- **GO-2026-5037**: crypto/x509 (HIGH)
- **GO-2026-4982/4980**: html/template XSS (HIGH)

**All vulnerabilities documented in**: `/home/coding/ARMOR/notes/bf-4zsbd-version-compatibility-report.md`

---

## 8. Compatibility Testing Requirements

### 8.1 Required Testing Post-Upgrade

**Go 1.25.0 → 1.25.12 Upgrade Testing:**
```bash
# 1. Build verification
go clean -cache
go build -o armor ./cmd/armor

# 2. Unit tests
go test ./... -short

# 3. Static analysis
go vet ./...

# 4. Security scan
govulncheck ./...

# 5. TLS config verification
# Test all TLS connections to B2/Cloudflare
```

**AWS SDK Upgrade Testing:**
```bash
# 1. Update dependencies
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.97.3
go mod tidy

# 2. S3/B2 operations test
# Requires valid credentials
go test ./tests/integration/... -v

# 3. Data integrity verification
# Test multipart uploads and downloads
```

### 8.2 Compatibility Validation Checklist

- [ ] **TLS Config Sharing**: Review for shared configs between servers
- [ ] **Template Escaping**: Review dashboard templates for XSS vectors
- [ ] **S3 Operations**: Verify all B2/S3 operations work correctly
- [ ] **Certificate Validation**: Test TLS certificate handling
- [ ] **HTTP/2 Operations**: Verify no DoS vulnerabilities
- [ ] **Memory Safety**: Monitor for memory exhaustion issues

---

## 9. Recommendations

### 9.1 Immediate Actions (Within 7 Days)

1. **🔴 CRITICAL**: Upgrade Go to 1.25.12
   - Addresses 27 security vulnerabilities
   - Fixes TLS config issues
   - Resolves dashboard XSS vulnerabilities

2. **🔴 CRITICAL**: Update AWS SDK to v1.97.3+
   - Fixes DoS vulnerability in S3 operations
   - Maintains backward compatibility

3. **⚠️ HIGH**: Review TLS config usage
   - Check for shared configs between servers
   - Implement config cloning if needed

### 9.2 Short-Term Actions (Within 30 Days)

1. **🟠 MEDIUM**: Review dashboard templates
   - Check for XSS vulnerabilities
   - Simplify complex template structures
   - Add security testing

2. **🟠 MEDIUM**: Implement automated security scanning
   - Add govulncheck to CI/CD pipeline
   - Schedule monthly dependency audits

### 9.3 Long-Term Maintenance

1. **📋 ONGOING**: Monitor Rust MSRV changes
   - Track NEEDLE MSRV updates
   - Plan for future toolchain upgrades

2. **📋 ONGOING**: Dependency update schedule
   - Monthly security updates
   - Quarterly compatibility reviews
   - Annual toolchain upgrades

---

## 10. Conclusion

### Summary Assessment

**Breaking Changes Status**: ⚠️ **WARNING** - 2 critical issues identified
- **TLS Config In-Place Modification**: Requires code review
- **AWS SDK DoS Vulnerability**: Requires immediate upgrade

**Security Vulnerabilities**: 🔴 **CRITICAL** - 27 CVEs identified
- **All documented in separate report**: `bf-4zsbd-version-compatibility-report.md`
- **Immediate action required**: Upgrade Go to 1.25.12 and AWS SDK to v1.97.3+

**Compatibility Status**: ✅ **GOOD** - No major compatibility issues
- **Rust/NEEDLE**: Excellent version buffers, no breaking changes
- **Go Stack**: Generally backward compatible with security fixes
- **AWS SDK**: Minor version update, should be compatible

**Production Readiness**: 
- **Current**: ⚠️ **SECURITY ISSUES** - Not production ready due to vulnerabilities
- **Post-Mitigation**: ✅ **READY** - All issues can be resolved with documented upgrades

### Research Sources

All research sources cited throughout this report:
- [Go 1.25 Release Notes - The Go Programming Language](https://go.dev/doc/go1.25)
- [Release History - The Go Programming Language](https://go.dev/doc/devel/release)
- [GitHub Issue #72100: net/http breaks compatibility by modifying tls-config in-place](https://github.com/golang/go/issues/72100)
- [html/template: fix bypass for CVE-2026-27142 · Issue #78913](https://github.com/golang/go/issues/78913)
- [JS template literal context incorrectly tracked · Issue #78331](https://github.com/golang/go/issues/78331)
- [html/template: add a hardened version to standard library · Issue #27926](https://github.com/golang/go/issues/27926)
- [Rust 1.96.0 is out : r/rust - Reddit](https://www.reddit.com/r/rust/comments/1tqd97o/rust_1960_is_out/)
- [docs.rs: Tokio README](https://docs.rs/crate/tokio/latest/source/README.md)
- [Can you please document the minimum required Rust version... Issue #936](https://github.com/tokio-rs/tracing/issues/936)
- [RFC 3537 on MSRV resolver](https://rust-lang.github.io/rfcs/3537-msrv-resolver.html)
- [CHANGELOG.md - aws-sdk-go-v2 S3 Service](https://github.com/aws/aws-sdk-go-v2/blob/main/service/s3/CHANGELOG.md)
- [Releases · aws/aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2/releases)

---

**Report Metadata:**
- **Bead**: bf-3s7js
- **Date**: 2026-07-12
- **Status**: ✅ COMPLETE
- **Related Reports**: 
  - `bf-4zsbd-version-compatibility-report.md` (Security vulnerabilities)
  - `bf-dw1bm.md` (Installed vs requirements comparison)
  - `bf-647lq-pluck-minimum-version-requirements.md` (Minimum versions)

---

**End of Known Incompatibilities and Breaking Changes Research Report**
