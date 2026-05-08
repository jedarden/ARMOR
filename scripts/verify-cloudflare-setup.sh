#!/usr/bin/env bash
# Cloudflare B2 Proxy Setup Verification Script
# This script verifies that Cloudflare is correctly configured for ARMOR's B2 proxy
#
# Usage: ./verify-cloudflare-setup.sh <cf-domain> <b2-bucket-name>
# Example: ./verify-cloudflare-setup.sh armor-b2.example.com my-armor-bucket

set -euo pipefail

CF_DOMAIN="${1:?Usage: $0 <cf-domain> <b2-bucket-name>}"
B2_BUCKET="${2:?Usage: $0 <cf-domain> <b2-bucket-name>}"

echo "=== ARMOR Cloudflare B2 Proxy Setup Verification ==="
echo "Cloudflare Domain: ${CF_DOMAIN}"
echo "B2 Bucket: ${B2_BUCKET}"
echo

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0
warn_count=0

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((pass_count++))
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((fail_count++))
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((warn_count++))
}

# 1. DNS Resolution
echo "1. Checking DNS resolution..."
if dig +short "${CF_DOMAIN}" | grep -q '\.'; then
    CF_IP=$(dig +short "${CF_DOMAIN}" | head -1)
    check_pass "DNS resolves: ${CF_DOMAIN} → ${CF_IP}"

    # Verify it's a Cloudflare IP
    if dig +short "${CF_DOMAIN}" | xargs -I {} whois {} 2>/dev/null | grep -qi "cloudflare"; then
        check_pass "IP belongs to Cloudflare"
    else
        check_warn "Could not verify IP ownership (whois may not be available)"
    fi
else
    check_fail "DNS does not resolve for ${CF_DOMAIN}"
fi
echo

# 2. SSL/TLS Configuration
echo "2. Checking SSL/TLS configuration..."
if command -v curl &> /dev/null; then
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "https://${CF_DOMAIN}" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "000" ]; then
        check_fail "Cannot connect via HTTPS"
    else
        check_pass "HTTPS accessible (HTTP ${HTTP_CODE})"

        # Check for Cloudflare headers
        CF_HEADERS=$(curl -s -I "https://${CF_DOMAIN}/" 2>/dev/null || echo "")
        if echo "$CF_HEADERS" | grep -qi "server: cloudflare"; then
            check_pass "Proxied through Cloudflare (server: cloudflare header)"
        else
            check_fail "Not proxied through Cloudflare (orange cloud disabled?)"
        fi
    fi
else
    check_warn "curl not available - skipping HTTPS check"
fi
echo

# 3. B2 Bucket Friendly Hostname
echo "3. Determining B2 bucket friendly hostname..."
echo "   Please provide your B2 bucket's friendly hostname"
echo "   (Find this in B2 web UI: Upload any file → check Download URL)"
read -p "   B2 friendly hostname (e.g., f004.backblazeb2.com): " B2_HOSTNAME

if [ -n "$B2_HOSTNAME" ]; then
    check_pass "B2 hostname noted: ${B2_HOSTNAME}"

    # Verify CNAME points to B2
    CNAME_TARGET=$(dig +short CNAME "${CF_DOMAIN}" 2>/dev/null || echo "")
    if [ -n "$CNAME_TARGET" ]; then
        if echo "$CNAME_TARGET" | grep -qi "backblazeb2.com"; then
            check_pass "CNAME points to B2: ${CNAME_TARGET}"
        else
            check_fail "CNAME does not point to B2: ${CNAME_TARGET}"
        fi
    else
        check_warn "Could not verify CNAME (may be an A/AAAA record)"
    fi
else
    check_fail "B2 hostname not provided"
fi
echo

# 4. Cloudflare Cache Headers Check
echo "4. Checking cache configuration..."
if command -v curl &> /dev/null; then
    # Check for CF-Cache-Status header
    CF_CACHE=$(curl -s -I "https://${CF_DOMAIN}/file/${B2_BUCKET}/" 2>/dev/null | grep -i "cf-cache-status" || echo "")
    if [ -n "$CF_CACHE" ]; then
        check_pass "Cache header present: ${CF_CACHE}"
    else
        check_warn "No CF-Cache-Status header found (may need a valid object path)"
    fi

    # Check for Cache-Control
    CACHE_CONTROL=$(curl -s -I "https://${CF_DOMAIN}/file/${B2_BUCKET}/" 2>/dev/null | grep -i "cache-control" || echo "")
    if [ -n "$CACHE_CONTROL" ]; then
        check_pass "Cache-Control header: ${CACHE_CONTROL}"
        if echo "$CACHE_CONTROL" | grep -qi "public"; then
            check_pass "Cache-Control includes 'public'"
        fi
    else
        check_warn "No Cache-Control header found"
    fi
fi
echo

# 5. B2 Headers Cleanup Check
echo "5. Checking for B2-specific headers (should be stripped)..."
if command -v curl &> /dev/null; then
    B2_HEADERS=$(curl -s -I "https://${CF_DOMAIN}/file/${B2_BUCKET}/" 2>/dev/null | grep -i "x-bz-" || echo "")
    if [ -z "$B2_HEADERS" ]; then
        check_pass "No x-bz-* headers present (cleanup working)"
    else
        check_warn "B2 headers still present: ${B2_HEADERS}"
    fi
fi
echo

# 6. SXG Check
echo "6. Signed Exchanges (SXG) check..."
echo "   Please verify in Cloudflare dashboard:"
echo "   Speed → Optimization → Automatic Signed Exchanges → OFF"
read -p "   Is SXG disabled? (y/n): " SXG_DISABLED
if [[ "$SXG_DISABLED" =~ ^[Yy]$ ]]; then
    check_pass "SXG is disabled"
else
    check_fail "SXG must be disabled for B2 compatibility"
fi
echo

# 7. Summary
echo "=== Verification Summary ==="
echo -e "${GREEN}Passed:${NC} ${pass_count}"
echo -e "${YELLOW}Warnings:${NC} ${warn_count}"
echo -e "${RED}Failed:${NC} ${fail_count}"
echo

if [ $fail_count -eq 0 ]; then
    echo -e "${GREEN}All critical checks passed!${NC}"
    echo
    echo "Next steps:"
    echo "1. Set ARMOR_CF_DOMAIN=${CF_DOMAIN}"
    echo "2. Set ARMOR_BUCKET=${B2_BUCKET}"
    echo "3. Test with: curl -I https://${CF_DOMAIN}/file/${B2_BUCKET}/test-file.txt"
    exit 0
else
    echo -e "${RED}Some checks failed. Please review and fix the issues above.${NC}"
    echo
    echo "Common issues:"
    echo "- Orange cloud (proxy) not enabled in Cloudflare DNS"
    echo "- SSL mode not set to Full (strict)"
    echo "- SXG not disabled"
    echo "- CNAME not pointing to correct B2 hostname"
    exit 1
fi
