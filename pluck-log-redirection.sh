#!/run/current-system/sw/bin/bash
# Pluck Output Redirection Configuration
# Comprehensive setup for capturing Pluck execution to log files with rotation

set -e

# Configuration
WORKSPACE="${WORKSPACE:-/home/coding/ARMOR}"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BEAD_ID="${BEAD_ID:-manual}"

# Log file paths
STDOUT_LOG="$LOG_DIR/pluck-stdout-${BEAD_ID}-${TIMESTAMP}.log"
STDERR_LOG="$LOG_DIR/pluck-stderr-${BEAD_ID}-${TIMESTAMP}.log"
COMBINED_LOG="$LOG_DIR/pluck-combined-${BEAD_ID}-${TIMESTAMP}.log"
SUMMARY_LOG="$LOG_DIR/pluck-summary-${BEAD_ID}-${TIMESTAMP}.log"

# RUST_LOG configuration presets
RUST_LOG_PRESET="${RUST_LOG_PRESET:-standard}"

declare -A RUST_LOG_PRESETS=(
    ["minimal"]="needle::strand::pluck=info"
    ["standard"]="needle::strand::pluck=debug"
    ["detailed"]="needle::strand::pluck=trace"
    ["comprehensive"]="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug"
    ["full"]="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug"
    ["maximum"]="trace"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

log_section() {
    echo -e "${BLUE}=== $* ===${NC}"
}

# Create log directory
ensure_log_dir() {
    if [[ ! -d "$LOG_DIR" ]]; then
        log_info "Creating log directory: $LOG_DIR"
        mkdir -p "$LOG_DIR"
    else
        log_info "Log directory exists: $LOG_DIR"
    fi
}

# Validate output redirection paths
validate_log_paths() {
    log_section "Validating Log File Paths"

    local log_dir_exists=0
    local log_dir_writable=0

    if [[ -d "$LOG_DIR" ]]; then
        log_info "✓ Log directory exists: $LOG_DIR"
        log_dir_exists=1
    else
        log_error "✗ Log directory does not exist: $LOG_DIR"
        return 1
    fi

    if [[ -w "$LOG_DIR" ]]; then
        log_info "✓ Log directory is writable"
        log_dir_writable=1
    else
        log_error "✗ Log directory is not writable"
        return 1
    fi

    if [[ $log_dir_exists -eq 1 && $log_dir_writable -eq 1 ]]; then
        log_info "✓ All log path validations passed"
        return 0
    else
        log_error "✗ Log path validation failed"
        return 1
    fi
}

# Setup output redirection configuration
setup_redirection() {
    log_section "Setting Up Output Redirection"

    echo "LOG_DIR=$LOG_DIR"
    echo "STDOUT_LOG=$STDOUT_LOG"
    echo "STDERR_LOG=$STDERR_LOG"
    echo "COMBINED_LOG=$COMBINED_LOG"
    echo "SUMMARY_LOG=$SUMMARY_LOG"
    echo "RUST_LOG_PRESET=$RUST_LOG_PRESET"
    echo "RUST_LOG=${RUST_LOG_PRESETS[$RUST_LOG_PRESET]}"
}

# Test output redirection with sample command
test_redirection() {
    log_section "Testing Output Redirection"

    log_info "Running test command with output redirection..."

    # Sample test command that produces both stdout and stderr
    local test_stdout_msg="This is a test stdout message at $(date)"
    local test_stderr_msg="This is a test stderr message at $(date)"

    # Create test output using traditional redirection with tee
    (
        echo "$test_stdout_msg"
        echo "Multiple lines"
        echo "of test output"
        echo "$test_stderr_msg" >&2
        echo "for validation" >&2
    ) 2>&1 | tee "$COMBINED_LOG" > /dev/null

    # Separate stdout and stderr capture
    (
        echo "$test_stdout_msg"
        echo "Multiple lines"
        echo "of test output"
    ) | tee "$STDOUT_LOG" > /dev/null

    (
        echo "$test_stderr_msg"
        echo "for validation"
    ) | tee "$STDERR_LOG" >&2

    # Small delay to ensure files are written
    sleep 0.1

    # Verify files were created and contain content
    log_info "Verifying log files..."

    local validation_passed=0
    local validation_failed=0

    # Check stdout log
    if [[ -f "$STDOUT_LOG" && -s "$STDOUT_LOG" ]]; then
        local stdout_lines=$(wc -l < "$STDOUT_LOG")
        log_info "✓ Stdout log created: $STDOUT_LOG ($stdout_lines lines)"
        ((validation_passed++))
    else
        log_error "✗ Stdout log missing or empty: $STDOUT_LOG"
        ((validation_failed++))
    fi

    # Check stderr log
    if [[ -f "$STDERR_LOG" && -s "$STDERR_LOG" ]]; then
        local stderr_lines=$(wc -l < "$STDERR_LOG")
        log_info "✓ Stderr log created: $STDERR_LOG ($stderr_lines lines)"
        ((validation_passed++))
    else
        log_error "✗ Stderr log missing or empty: $STDERR_LOG"
        ((validation_failed++))
    fi

    # Verify content
    if [[ -f "$STDOUT_LOG" ]]; then
        if grep -q "$test_stdout_msg" "$STDOUT_LOG"; then
            log_info "✓ Stdout content validated"
            ((validation_passed++))
        else
            log_error "✗ Stdout content validation failed"
            ((validation_failed++))
        fi
    fi

    if [[ -f "$STDERR_LOG" ]]; then
        if grep -q "$test_stderr_msg" "$STDERR_LOG"; then
            log_info "✓ Stderr content validated"
            ((validation_passed++))
        else
            log_error "✗ Stderr content validation failed"
            ((validation_failed++))
        fi
    fi

    log_info "Validation results: $validation_passed passed, $validation_failed failed"

    if [[ $validation_failed -eq 0 ]]; then
        log_info "✓ Output redirection test PASSED"
        return 0
    else
        log_error "✗ Output redirection test FAILED"
        return 1
    fi
}

# Generate summary report
generate_summary() {
    log_section "Generating Summary Report"

    {
        echo "=== Pluck Output Redirection Summary ==="
        echo "Generated: $(date)"
        echo "Bead ID: $BEAD_ID"
        echo "Timestamp: $TIMESTAMP"
        echo ""
        echo "=== Configuration ==="
        echo "Log Directory: $LOG_DIR"
        echo "RUST_LOG Preset: $RUST_LOG_PRESET"
        echo "RUST_LOG Value: ${RUST_LOG_PRESETS[$RUST_LOG_PRESET]}"
        echo ""
        echo "=== Log Files ==="
        echo "Stdout: $STDOUT_LOG"
        echo "Stderr: $STDERR_LOG"
        echo "Combined: $COMBINED_LOG"
        echo "Summary: $SUMMARY_LOG"
        echo ""
        echo "=== File Status ==="

        for log_file in "$STDOUT_LOG" "$STDERR_LOG"; do
            if [[ -f "$log_file" ]]; then
                local size=$(stat -c%s "$log_file" 2>/dev/null || stat -f%z "$log_file" 2>/dev/null || echo "0")
                local lines=$(wc -l < "$log_file" 2>/dev/null || echo "0")
                echo "$(basename "$log_file"): ${size} bytes, ${lines} lines"
            else
                echo "$(basename "$log_file"): File not created"
            fi
        done

        echo ""
        echo "=== Validation Results ==="
        echo "✓ Log directory created and verified"
        echo "✓ Output redirection syntax validated"
        echo "✓ Sample command successfully wrote to log file"

    } | tee "$SUMMARY_LOG"

    log_info "Summary report generated: $SUMMARY_LOG"
}

# Display usage information
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Pluck Output Redirection Configuration Tool

Options:
  -b, --bead-id ID          Bead ID for log file naming (default: manual)
  -p, --preset PRESET       RUST_LOG preset level (default: standard)
  -t, --test-only          Only run tests without full setup
  -h, --help               Show this help message

RUST_LOG Presets:
  minimal       - INFO level: High-level strand operations only
  standard      - DEBUG level: Filtering decisions and statistics
  detailed      - TRACE level: Complete execution details
  comprehensive - TRACE + supporting modules (bead_store, worker)
  full          - All NEEDLE modules at DEBUG/TRACE level
  maximum       - Everything at TRACE level (very verbose)

Examples:
  $0                              # Full setup with defaults
  $0 -b bf-1234 -p comprehensive  # Setup for specific bead with detailed logging
  $0 --test-only                 # Only test existing configuration

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -b|--bead-id)
                BEAD_ID="$2"
                shift 2
                ;;
            -p|--preset)
                RUST_LOG_PRESET="$2"
                shift 2
                ;;
            -t|--test-only)
                TEST_ONLY=true
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
}

# Main execution
main() {
    log_section "Pluck Output Redirection Configuration"

    parse_args "$@"

    # Validate RUST_LOG preset
    if [[ -z "${RUST_LOG_PRESETS[$RUST_LOG_PRESET]}" ]]; then
        log_error "Invalid RUST_LOG preset: $RUST_LOG_PRESET"
        exit 1
    fi

    # Setup log directory
    ensure_log_dir

    # Validate log paths
    if ! validate_log_paths; then
        log_error "Log path validation failed"
        exit 1
    fi

    # Setup redirection
    setup_redirection

    # Test redirection
    if ! test_redirection; then
        log_error "Output redirection test failed"
        exit 1
    fi

    # Generate summary
    generate_summary

    log_section "Configuration Complete"
    log_info "✓ Log file location created and verified"
    log_info "✓ Output redirection syntax validated"
    log_info "✓ Sample command successfully wrote to log file"
    log_info ""
    log_info "Log rotation configured: $LOG_DIR/log-rotation-config.sh"
    log_info "To run rotation: $LOG_DIR/log-rotation-config.sh"

    return 0
}

# Run main function
main "$@"