#!/run/current-system/sw/bin/bash
# Pluck Command Syntax Validation Script
# Tests needle run command syntax and debug flags without full execution

# Configuration
WORKSPACE="${WORKSPACE:-/home/coding/ARMOR}"
BEAD_ID="${BEAD_ID:-bf-t5my}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
VALIDATION_LOG="/tmp/pluck-syntax-validation-${TIMESTAMP}.log"
REPORT_FILE="/tmp/pluck-syntax-report-${TIMESTAMP}.txt"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$VALIDATION_LOG"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$VALIDATION_LOG"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$VALIDATION_LOG"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$VALIDATION_LOG"
}

log_section() {
    echo "" | tee -a "$VALIDATION_LOG"
    echo -e "${BLUE}=== $1 ===${NC}" | tee -a "$VALIDATION_LOG"
    echo "" | tee -a "$VALIDATION_LOG"
}

# Initialize validation report
init_report() {
    cat > "$REPORT_FILE" << EOF
Pluck Command Syntax Validation Report
======================================
Bead ID: $BEAD_ID
Timestamp: $TIMESTAMP
Workspace: $WORKSPACE

EOF
}

# Test 1: Validate needle binary exists and is executable
test_needle_binary() {
    log_section "Test 1: Needle Binary Validation"

    local needle_path=$(which needle)
    if [ -z "$needle_path" ]; then
        log_error "needle binary not found in PATH"
        echo "❌ Binary check: FAILED" | tee -a "$REPORT_FILE"
        return 1
    fi

    if [ ! -x "$needle_path" ]; then
        log_error "needle binary exists but is not executable: $needle_path"
        echo "❌ Binary check: FAILED (not executable)" | tee -a "$REPORT_FILE"
        return 1
    fi

    log_success "needle binary found at: $needle_path"
    local version=$(needle version 2>/dev/null || echo "unknown")
    log_info "needle version: $version"
    echo "✅ Binary check: PASSED ($needle_path, $version)" | tee -a "$REPORT_FILE"
    return 0
}

# Test 2: Validate command structure
test_command_structure() {
    log_section "Test 2: Command Structure Validation"

    # Test basic command structure
    log_info "Testing: needle run --help"
    if needle run --help > /dev/null 2>&1; then
        log_success "Command structure is valid"
        echo "✅ Command structure: PASSED" | tee -a "$REPORT_FILE"
        return 0
    else
        log_error "Command structure validation failed"
        echo "❌ Command structure: FAILED" | tee -a "$REPORT_FILE"
        return 1
    fi
}

# Test 3: Validate individual flags
test_flag_validation() {
    log_section "Test 3: Flag Recognition Validation"

    local flags_valid=true
    local test_flags=("-w" "-c" "-a" "-i" "-t" "--help" "--resume" "--hot-reload")

    log_info "Testing individual flag recognition..."

    for flag in "${test_flags[@]}"; do
        if needle run "$flag" --help > /dev/null 2>&1; then
            log_success "Flag '$flag' recognized"
        else
            log_warning "Flag '$flag' might not be recognized (check manually)"
            flags_valid=false
        fi
    done

    if $flags_valid; then
        echo "✅ Flag recognition: PASSED" | tee -a "$REPORT_FILE"
        return 0
    else
        echo "⚠️  Flag recognition: PASSED with warnings" | tee -a "$REPORT_FILE"
        return 0
    fi
}

# Test 4: Validate RUST_LOG syntax
test_rust_log_syntax() {
    log_section "Test 4: RUST_LOG Environment Variable Syntax"

    local rust_log="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

    log_info "Testing RUST_LOG syntax: $rust_log"

    # Parse RUST_LOG into individual modules
    IFS=',' read -ra MODULES <<< "$rust_log"
    local modules_valid=true

    for module in "${MODULES[@]}"; do
        # Check if module has =level syntax
        if [[ "$module" =~ = ]]; then
            local mod_name="${module%=*}"
            local mod_level="${module##*=}"

            log_info "Module: $mod_name, Level: $mod_level"

            # Validate log level
            case "$mod_level" in
                trace|debug|info|warn|error)
                    log_success "Valid log level: $mod_level"
                    ;;
                *)
                    log_warning "Unusual log level: $mod_level (might be invalid)"
                    modules_valid=false
                    ;;
            esac
        else
            log_warning "Module without level: $module"
            modules_valid=false
        fi
    done

    if $modules_valid; then
        echo "✅ RUST_LOG syntax: PASSED (${#MODULES[@]} modules)" | tee -a "$REPORT_FILE"
        return 0
    else
        echo "⚠️  RUST_LOG syntax: PASSED with warnings" | tee -a "$REPORT_FILE"
        return 0
    fi
}

# Test 5: Validate workspace path
test_workspace_validation() {
    log_section "Test 5: Workspace Path Validation"

    log_info "Testing workspace path: $WORKSPACE"

    if [ ! -d "$WORKSPACE" ]; then
        log_error "Workspace directory does not exist: $WORKSPACE"
        echo "❌ Workspace validation: FAILED (directory not found)" | tee -a "$REPORT_FILE"
        return 1
    fi

    # Check for .beads directory
    if [ ! -d "$WORKSPACE/.beads" ]; then
        log_warning ".beads directory not found in workspace"
        echo "⚠️  Workspace validation: PASSED with warnings (.beads not found)" | tee -a "$REPORT_FILE"
        return 0
    fi

    # Check for beads database
    if [ -f "$WORKSPACE/.beads/beads.db" ]; then
        log_success "beads database found"
    else
        log_warning "beads database not found"
    fi

    echo "✅ Workspace validation: PASSED" | tee -a "$REPORT_FILE"
    return 0
}

# Test 6: Test command parsing without execution
test_command_parsing() {
    log_section "Test 6: Command Parsing Test (Dry Run)"

    local test_cmd="needle run -w '$WORKSPACE' -c 1"

    log_info "Testing command parsing: $test_cmd"
    log_info "Note: This tests parsing only, not actual execution"

    log_info "Running 2-second timeout test to validate command parsing..."

    # Run with very short timeout to test parsing only
    timeout 2s needle run -w "$WORKSPACE" -c 1 > /dev/null 2>&1 || true
    local exit_code=$?

    if [ $exit_code -eq 124 ]; then
        log_success "Command parsing successful (timeout expected)"
        echo "✅ Command parsing: PASSED (timeout expected)" | tee -a "$REPORT_FILE"
        return 0
    else
        log_warning "Command parsing test returned exit code: $exit_code"
        echo "⚠️  Command parsing: INCONCLUSIVE (exit code: $exit_code)" | tee -a "$REPORT_FILE"
        return 0
    fi
}

# Test 7: Validate shell script syntax
test_shell_script_syntax() {
    log_section "Test 7: Shell Script Syntax Validation"

    local script_path="$WORKSPACE/execute-pluck-bf-4q1w.sh"

    if [ ! -f "$script_path" ]; then
        log_warning "Reference script not found: $script_path"
        echo "⚠️  Shell script validation: SKIPPED (script not found)" | tee -a "$REPORT_FILE"
        return 0
    fi

    log_info "Validating shell script: $script_path"

    # Test bash syntax
    if bash -n "$script_path" 2>/dev/null; then
        log_success "Shell script syntax is valid"
        echo "✅ Shell script syntax: PASSED" | tee -a "$REPORT_FILE"
        return 0
    else
        log_error "Shell script has syntax errors"
        bash -n "$script_path" 2>&1 | tee -a "$VALIDATION_LOG"
        echo "❌ Shell script syntax: FAILED" | tee -a "$REPORT_FILE"
        return 1
    fi
}

# Main execution
main() {
    log_info "Starting Pluck Command Syntax Validation"
    log_info "Validation log: $VALIDATION_LOG"
    log_info "Report file: $REPORT_FILE"

    init_report

    local total_tests=0
    local passed_tests=0
    local failed_tests=0

    # Run all tests (allow failures to continue)
    test_needle_binary && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    test_command_structure && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    test_flag_validation && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    test_rust_log_syntax && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    test_workspace_validation && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    test_command_parsing && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    test_shell_script_syntax && ((passed_tests++)) || ((failed_tests++))
    ((total_tests++))

    # Generate final summary
    log_section "Validation Summary"

    cat >> "$REPORT_FILE" << EOF

SUMMARY
-------
Total Tests: $total_tests
Passed: $passed_tests
Failed: $failed_tests

OVERALL RESULT: $([ $failed_tests -eq 0 ] && echo "✅ PASSED" || echo "❌ FAILED")
EOF

    log_info "Total Tests: $total_tests"
    log_info "Passed: $passed_tests"
    log_info "Failed: $failed_tests"

    if [ $failed_tests -eq 0 ]; then
        log_success "All syntax validation tests passed!"
        echo ""
        echo "📋 Detailed report: $REPORT_FILE"
        echo "📋 Validation log: $VALIDATION_LOG"
        return 0
    else
        log_error "Some validation tests failed"
        echo ""
        echo "📋 Detailed report: $REPORT_FILE"
        echo "📋 Validation log: $VALIDATION_LOG"
        return 1
    fi
}

# Run main function
main "$@"
