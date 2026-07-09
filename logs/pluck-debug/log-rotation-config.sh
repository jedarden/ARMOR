#!/bin/bash
# Simplified Log Rotation Configuration for Pluck Debug Logs
# Provides automated log rotation and cleanup policies for long-running processes

# Configuration
LOG_DIR="${LOG_DIR:-/home/coding/ARMOR/logs/pluck-debug}"
MAX_SIZE_MB=${MAX_SIZE_MB:-10}      # Rotate logs when they exceed 10MB
MAX_AGE_DAYS=${MAX_AGE_DAYS:-7}      # Remove logs older than 7 days
MAX_LOG_FILES=${MAX_LOG_FILES:-50}   # Keep maximum 50 log files

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

# Ensure log directory exists
ensure_log_dir() {
    if [[ ! -d "$LOG_DIR" ]]; then
        log_info "Creating log directory: $LOG_DIR"
        mkdir -p "$LOG_DIR"
    fi
}

# Get file size in MB
get_file_size_mb() {
    local file=$1
    if [[ -f "$file" ]]; then
        size=$(du -m "$file" 2>/dev/null | cut -f1)
        echo "${size:-0}"
    else
        echo "0"
    fi
}

# Rotate a single log file
rotate_log_file() {
    local log_file=$1
    local base_name=$(basename "$log_file")
    local size_mb=$(get_file_size_mb "$log_file")

    if [[ $size_mb -ge $MAX_SIZE_MB ]]; then
        log_info "Rotating log file: $base_name (size: ${size_mb}MB)"

        # Find next available rotation number
        local rot_num=1
        while [[ -f "${log_file}.${rot_num}" ]]; do
            ((rot_num++))
        done

        # Rotate the file
        mv "$log_file" "${log_file}.${rot_num}"
        log_info "Rotated to: ${base_name}.${rot_num}"
    fi
}

# Clean up old log files
cleanup_old_logs() {
    log_info "Cleaning up old logs (older than ${MAX_AGE_DAYS} days)..."
    local old_count=0

    # Find and remove old log files
    while IFS= read -r file; do
        if [[ -n "$file" ]]; then
            log_warn "Removing old log: $(basename "$file")"
            rm -f "$file"
            ((old_count++))
        fi
    done < <(find "$LOG_DIR" -type f -name "*.log" -mtime +$MAX_AGE_DAYS 2>/dev/null)

    log_info "Removed $old_count old log files"
}

# Enforce maximum file count
enforce_max_files() {
    log_info "Enforcing maximum log file count: $MAX_LOG_FILES"

    # Count current log files
    local current_count=$(find "$LOG_DIR" -type f -name "*.log" 2>/dev/null | wc -l)

    if [[ $current_count -gt $MAX_LOG_FILES ]]; then
        local excess=$((current_count - MAX_LOG_FILES))
        log_warn "Found $current_count log files, removing $excess oldest files"

        # Remove oldest files
        find "$LOG_DIR" -type f -name "*.log" -printf '%T@ %p\n' 2>/dev/null | \
            sort -n | head -n "$excess" | cut -d' ' -f2- | while IFS= read -r file; do
                if [[ -n "$file" ]]; then
                    rm -f "$file"
                fi
            done

        log_info "Removed $excess oldest log files"
    else
        log_info "Log file count within limit: $current_count/$MAX_LOG_FILES"
    fi
}

# Check and rotate all active log files
rotate_active_logs() {
    log_info "Checking for log files to rotate..."
    local rotated_count=0
    local rotated_files=0

    # Check all .log files in the directory
    while IFS= read -r log_file; do
        if [[ -n "$log_file" ]]; then
            rotate_log_file "$log_file"
            ((rotated_count++))
            # Check if file was actually rotated
            if [[ ! -f "$log_file" && -f "${log_file}.1" ]]; then
                ((rotated_files++))
            fi
        fi
    done < <(find "$LOG_DIR" -type f -name "*.log" -not -name "* rotated *" 2>/dev/null)

    log_info "Checked $rotated_count log files, rotated $rotated_files files"
}

# Display usage information
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Pluck Log Rotation Management Tool

Options:
  -d, --dry-run          Show what would be done without making changes
  -h, --help             Show this help message

Configuration:
  MAX_SIZE_MB:     Maximum log file size before rotation (default: 10MB)
  MAX_AGE_DAYS:    Maximum age of log files to keep (default: 7 days)
  MAX_LOG_FILES:   Maximum number of log files to keep (default: 50)

Examples:
  $0                          # Run rotation with default settings
  MAX_SIZE_MB=5 $0            # Rotate logs at 5MB instead of 10MB
  MAX_AGE_DAYS=3 $0           # Remove logs older than 3 days

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -d|--dry-run)
                DRY_RUN=true
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
    parse_args "$@"

    ensure_log_dir

    log_section "Log Rotation Process"
    log_info "Log Directory: $LOG_DIR"
    log_info "Configuration:"
    log_info "  Max Size: ${MAX_SIZE_MB}MB"
    log_info "  Max Age: ${MAX_AGE_DAYS} days"
    log_info "  Max Files: $MAX_LOG_FILES"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN MODE - No changes will be made"
        log_section "Dry Run Analysis"
        log_info "Would check for files larger than ${MAX_SIZE_MB}MB"
        log_info "Would remove files older than ${MAX_AGE_DAYS} days"
        log_info "Would enforce maximum of $MAX_LOG_FILES files"
        exit 0
    fi

    # Perform rotation tasks
    rotate_active_logs
    cleanup_old_logs
    enforce_max_files

    log_section "Rotation Complete"

    # Display summary
    local total_files=$(find "$LOG_DIR" -type f -name "*.log" 2>/dev/null | wc -l)
    local total_size=$(du -sh "$LOG_DIR" 2>/dev/null | cut -f1)

    log_info "Current log directory status:"
    log_info "  Total files: $total_files"
    log_info "  Total size: $total_size"
}

# Run main function
main "$@"