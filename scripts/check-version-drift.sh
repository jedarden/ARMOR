#!/usr/bin/env bash
# ARMOR Version Drift Check Tool
# Scans deployed ARMOR versions across clusters and compares against latest releases
# Outputs JSON with drift analysis and flags for deployments needing attention

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null || echo "$SCRIPT_DIR/..")"
DECLARATIVE_CONFIG="${DECLARATIVE_CONFIG:-$HOME/declarative-config}"
MAX_RELEASES_BEHIND=${MAX_RELEASES_BEHIND:-50}  # Flag if >N releases behind
MAX_DAYS_BEHIND=${MAX_DAYS_BEHIND:-30}          # Flag if >M days behind
GITHUB_REPO=${GITHUB_REPO:-jedarden/ARMOR}

# Colors for terminal output (only when not piping)
if [ -t 1 ]; then
    RED='\033[0;31m'
    YELLOW='\033[1;33m'
    GREEN='\033[0;32m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    RED=''
    YELLOW=''
    GREEN=''
    BLUE=''
    NC=''
fi

log_info() { echo -e "${BLUE}[INFO]${NC} $*" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*" >&2; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# Get all ARMOR deployment files from declarative-config
get_deployment_files() {
    if [ ! -d "$DECLARATIVE_CONFIG" ]; then
        log_error "declarative-config not found at $DECLARATIVE_CONFIG"
        return 1
    fi
    find "$DECLARATIVE_CONFIG/k8s" -name "armor-deployment.yml" -type f
}

# Extract cluster name from file path
extract_cluster_name() {
    local file="$1"
    # Path format: .../k8s/<cluster>/.../armor-deployment.yml
    echo "$file" | sed -n 's|.*/k8s/\([^/]*\).*|\1|p'
}

# Extract image version from deployment file
extract_deployed_version() {
    local file="$1"
    grep -oP 'image: ronaldraygun/armor:\K[0-9]+\.[0-9]+\.[0-9]+' "$file" 2>/dev/null | head -1
}

# Get version from git commit (auto-bumped versions)
get_latest_git_version() {
    git -C "$REPO_ROOT" log --oneline --grep="ci: auto-bump version to" | head -1 | sed 's/.*ci: auto-bump version to //'
}

# Get version from git tag (official releases)
get_latest_tag_version() {
    git -C "$REPO_ROOT" tag -l "v*" --sort=-v:refname | head -1 | sed 's/^v//'
}

# Fetch GitHub releases and get latest
get_latest_github_release() {
    local releases_json
    releases_json=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases?per_page=10" 2>/dev/null)
    if [ -z "$releases_json" ] || echo "$releases_json" | grep -q "API rate limit exceeded"; then
        log_warn "GitHub API rate limited or unavailable, using git tags"
        return 1
    fi

    # Get latest published (non-draft) release
    echo "$releases_json" | jq -r '[.[] | select(.draft == false and .prerelease == false)] | sort_by(.published_at) | reverse | .[0]'
}

# Parse version string into components
parse_version() {
    local version="$1"
    echo "$version" | awk -F. '{print $1, $2, $3}'
}

# Compare two versions
# Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
compare_versions() {
    local v1="$1"
    local v2="$2"

    local major1 minor1 patch1 major2 minor2 patch2
    read -r major1 minor1 patch1 <<< "$(parse_version "$v1")"
    read -r major2 minor2 patch2 <<< "$(parse_version "$v2")"

    if [ "$major1" -lt "$major2" ]; then echo -1; return
    elif [ "$major1" -gt "$major2" ]; then echo 1; return
    fi

    if [ "$minor1" -lt "$minor2" ]; then echo -1; return
    elif [ "$minor1" -gt "$minor2" ]; then echo 1; return
    fi

    if [ "$patch1" -lt "$patch2" ]; then echo -1; return
    elif [ "$patch1" -gt "$patch2" ]; then echo 1; return
    fi

    echo 0
}

# Calculate version difference
calculate_version_diff() {
    local v1="$1"  # deployed version
    local v2="$2"  # latest version

    local major1 minor1 patch1 major2 minor2 patch2
    read -r major1 minor1 patch1 <<< "$(parse_version "$v1")"
    read -r major2 minor2 patch2 <<< "$(parse_version "$v2")"

    # Simple patch-level difference (assumes no major/minor changes in normal flow)
    echo "$((patch2 - patch1))"
}

# Get release date from version tag
get_release_date() {
    local version="$1"
    local tag="v$version"

    # Try GitHub API first
    local date
    date=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/tags/$tag" | jq -r '.published_at // empty' 2>/dev/null)

    if [ -n "$date" ]; then
        echo "$date"
        return
    fi

    # Fallback to git tag date
    git -C "$REPO_ROOT" tag -l "$tag" --format='%(taggerdate:iso8601)' | head -1
}

# Calculate days difference between dates
calculate_days_diff() {
    local date1="$1"  # ISO8601 date
    local date2="$2"  # ISO8601 date

    local epoch1 epoch2
    epoch1=$(date -d "$date1" +%s 2>/dev/null || echo 0)
    epoch2=$(date -d "$date2" +%s 2>/dev/null || echo 0)

    if [ "$epoch1" -eq 0 ] || [ "$epoch2" -eq 0 ]; then
        echo 0
        return
    fi

    echo "$(( (epoch2 - epoch1) / 86400 ))"
}

# Check if release is correctness-labeled (contains bug/fix/correct/security)
check_correctness_label() {
    local release_body="$1"
    echo "$release_body" | grepqiE "(bug|fix|correct|security|critical)" && echo "true" || echo "false"
}

# Main function
main() {
    log_info "Starting ARMOR version drift check..."

    # Get latest versions
    local latest_git_version latest_tag_version latest_github_release
    latest_git_version=$(get_latest_git_version)
    latest_tag_version=$(get_latest_tag_version)

    log_info "Latest git version: ${latest_git_version:-unknown}"
    log_info "Latest tag version: ${latest_tag_version:-unknown}"

    # Try to get GitHub release
    latest_github_release=$(get_latest_github_release 2>/dev/null || echo "")

    if [ -n "$latest_github_release" ]; then
        local latest_github_tag latest_github_date
        latest_github_tag=$(echo "$latest_github_release" | jq -r '.tag_name' | sed 's/^v//')
        latest_github_date=$(echo "$latest_github_release" | jq -r '.published_at')
        log_info "Latest GitHub release: $latest_github_tag (published: $latest_github_date)"
    fi

    # ARMOR uses continuous auto-bumping - use git version as primary latest
    # GitHub releases are for correctness-critical tagged releases only
    local latest_version="$latest_git_version"
    if [ -z "$latest_version" ]; then
        log_error "Could not determine latest version from git"
        exit 1
    fi

    # Build deployment info JSON
    local deployments_json="["
    local first=true

    # Get deployment files and process them
    local deploy_files
    deploy_files=$(get_deployment_files)

    while IFS= read -r deploy_file; do
        [ -f "$deploy_file" ] || continue

        local cluster deployed_version
        cluster=$(extract_cluster_name "$deploy_file")
        deployed_version=$(extract_deployed_version "$deploy_file")

        if [ -z "$deployed_version" ]; then
            log_warn "No version found in $deploy_file"
            continue
        fi

        log_info "Found $cluster: $deployed_version"

        # Calculate drift
        local releases_behind versions_diff days_diff release_date
        versions_diff=$(calculate_version_diff "$deployed_version" "$latest_version")
        releases_behind=$versions_diff  # Assume each patch is a release for now

        # Get release date for deployed version
        release_date=$(get_release_date "$deployed_version")

        # Calculate days behind
        if [ -n "$release_date" ] && [ -n "$latest_github_date" ]; then
            days_diff=$(calculate_days_diff "$release_date" "$latest_github_date")
        else
            days_diff=0
        fi

        # Check if needs attention
        local needs_attention="false"
        local attention_reason=""

        if [ "$releases_behind" -gt "$MAX_RELEASES_BEHIND" ]; then
            needs_attention="true"
            attention_reason="${attention_reason} $releases_behind releases behind threshold ($MAX_RELEASES_BEHIND)"
        fi

        if [ "$days_diff" -gt "$MAX_DAYS_BEHIND" ]; then
            needs_attention="true"
            attention_reason="${attention_reason} $days_diff days behind threshold ($MAX_DAYS_BEHIND)"
        fi

        # Build JSON object
        local json_entry
        json_entry=$(cat <<EOF
{
  "cluster": "$cluster",
  "deployed_version": "$deployed_version",
  "latest_version": "$latest_version",
  "releases_behind": $releases_behind,
  "days_behind": $days_diff,
  "deployment_file": "$deploy_file",
  "release_date": "${release_date:-unknown}",
  "needs_attention": $needs_attention,
  "attention_reason": "$(echo "$attention_reason" | xargs)",
  "latest_github_release": "${latest_github_tag:-none}",
  "latest_github_date": "${latest_github_date:-none}"
}
EOF
)

        if [ "$first" = true ]; then
            first=false
        else
            deployments_json="${deployments_json},"
        fi
        deployments_json="${deployments_json}${json_entry}"

    done <<< "$deploy_files"

    deployments_json="${deployments_json}]"

    # Output full JSON with summary
    local output_json
    output_json=$(cat <<EOF
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "armor_repo": "$GITHUB_REPO",
  "latest_git_version": "${latest_git_version:-unknown}",
  "latest_tag_version": "${latest_tag_version:-unknown}",
  "latest_github_release": "${latest_github_tag:-none}",
  "latest_github_date": "${latest_github_date:-none}",
  "thresholds": {
    "max_releases_behind": $MAX_RELEASES_BEHIND,
    "max_days_behind": $MAX_DAYS_BEHIND
  },
  "deployments": $deployments_json
}
EOF
)

    # Output to stdout (JSON)
    echo "$output_json"

    # Also print human-readable summary to stderr
    echo -e "\n${BLUE}=== ARMOR Version Drift Summary ===${NC}" >&2

    # Count clusters needing attention
    local needs_count
    needs_count=$(echo "$deployments_json" | jq -r '[.[] | select(.needs_attention == true)] | length')

    echo -e "Latest version: ${GREEN}${latest_version}${NC}" >&2
    echo -e "Clusters checked: $(echo "$deployments_json" | jq -r '. | length')" >&2
    echo -e "Clusters needing attention: ${RED}${needs_count}${NC}" >&2
    echo -e "Thresholds: >${MAX_RELEASES_BEHIND} releases or >${MAX_DAYS_BEHIND} days${NC}" >&2

    if [ "$needs_count" -gt 0 ]; then
        echo -e "\n${YELLOW}⚠ Clusters requiring attention:${NC}" >&2
        echo "$deployments_json" | jq -r '.[] | select(.needs_attention == true) | "\(.cluster): \(.deployed_version) → \(.latest_version) (\(.attention_reason))"' >&2
    fi

    # Return exit code based on whether attention is needed
    [ "$needs_count" -gt 0 ] && exit 1 || exit 0
}

main "$@"
