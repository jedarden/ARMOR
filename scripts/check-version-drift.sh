#!/usr/bin/env bash
# ARMOR Version Drift Check Script
# Orchestrates discovery, fetching, and drift detection for ARMOR deployments
#
# Usage:
#   ./scripts/check-version-drift.sh [OPTIONS]
#
# Options:
#   --releases N     Flag deployments N or more releases behind (default: 3)
#   --days N        Flag deployments N or more days behind (default: 30)
#   --json          Output machine-readable JSON instead of human-readable format
#   --output FILE   Write report to file (in addition to stdout)
#   --sort-by FIELD Sort by field: cluster, releases, days, correctness (default: correctness)
#   --config FILE   Use configuration file (JSON)
#
# Exit codes:
#   0 - All deployments within thresholds
#   1 - One or more deployments exceed thresholds
#   2 - Error occurred
#
# Examples:
#   ./scripts/check-version-drift.sh
#   ./scripts/check-version-drift.sh --releases 5 --days 60
#   ./scripts/check-version-drift.sh --json --output report.json
#   ./scripts/check-version-drift.sh --sort-by cluster

set -euo pipefail

# Script directory and repo root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default configuration
DEFAULT_RELEASES_THRESHOLD=3
DEFAULT_DAYS_THRESHOLD=30
DEFAULT_SORT_BY="correctness"

# Colors for terminal output
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

# Check Python 3 availability
check_python() {
    if ! command -v python3 &> /dev/null; then
        log_error "python3 is required but not found"
        exit 2
    fi
}

# Check if required Python scripts exist
check_scripts() {
    local required_scripts=(
        "version-drift-check.py"
        "github-release-fetcher.py"
        "find-armor-deployments.py"
        "compare-version-drift.py"
    )

    for script in "${required_scripts[@]}"; do
        if [ ! -f "$SCRIPT_DIR/$script" ]; then
            log_error "Required script not found: $script"
            exit 2
        fi
    done
}

# Main function
main() {
    local releases_threshold="$DEFAULT_RELEASES_THRESHOLD"
    local days_threshold="$DEFAULT_DAYS_THRESHOLD"
    local sort_by="$DEFAULT_SORT_BY"
    local output_json=false
    local output_file=""
    local config_file=""

    # Parse command-line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --releases)
                releases_threshold="$2"
                shift 2
                ;;
            --days)
                days_threshold="$2"
                shift 2
                ;;
            --json)
                output_json=true
                shift
                ;;
            --output)
                output_file="$2"
                shift 2
                ;;
            --sort-by)
                sort_by="$2"
                shift 2
                ;;
            --config)
                config_file="$2"
                shift 2
                ;;
            -h|--help)
                grep -E '^# ' "$0" | sed 's/^# //' | sed '1,7d;$d'
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                log_warn "Use --help for usage information"
                exit 2
                ;;
        esac
    done

    log_info "ARMOR Version Drift Check"
    log_info "Thresholds: > $releases_threshold releases, > $days_threshold days"

    # Check prerequisites
    check_python
    check_scripts

    # Build command for Python script
    local python_cmd=(python3 "$SCRIPT_DIR/version-drift-check.py")

    # Add configuration file if specified
    if [ -n "$config_file" ]; then
        python_cmd+=(--config "$config_file")
    fi

    # Add threshold overrides
    python_cmd+=(--releases-threshold "$releases_threshold")
    python_cmd+=(--days-threshold "$days_threshold")

    # Add output format
    if [ "$output_json" = true ]; then
        python_cmd+=(--json)
    fi

    # Add output file if specified
    if [ -n "$output_file" ]; then
        python_cmd+=(--output "$output_file")
    fi

    # Add sort option
    python_cmd+=(--sort-by "$sort_by")

    # Change to repo root to ensure consistent working directory
    cd "$REPO_ROOT"

    # Run the Python script
    log_info "Running version drift check..."
    if "${python_cmd[@]}"; then
        log_info "Check completed successfully"
        exit 0
    else
        local exit_code=$?
        if [ $exit_code -eq 1 ]; then
            log_warn "Drift detected - one or more deployments exceed thresholds"
        else
            log_error "Check failed with exit code $exit_code"
        fi
        exit $exit_code
    fi
}

main "$@"
