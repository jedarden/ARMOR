#!/run/current-system/sw/bin/bash
# Real-time Pluck log monitoring and analysis tool for bead bf-y4qr
# Monitors log files for activity and analyzes patterns, errors, and progress

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

BEAD_ID="bf-y4qr"
WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"

# Function to show usage
show_usage() {
    cat << EOF
${CYAN}Usage:${NC}
  $0 [COMMAND] [OPTIONS]

${CYAN}Commands:${NC}
  ${GREEN}watch${NC} [LOG_FILE]           Watch a log file in real-time with pattern highlighting
  ${GREEN}analyze${NC} [LOG_FILE]         Analyze a log file for patterns and statistics
  ${GREEN}monitor${NC} [LOG_DIR]          Monitor all log files in a directory
  ${GREEN}errors${NC} [LOG_FILE]           Show only errors and warnings from a log file
  ${GREEN}progress${NC} [LOG_FILE]         Show progress indicators from a log file
  ${GREEN}summary${NC} [LOG_DIR]           Generate summary of all logs in directory
  ${GREEN}compare${NC} [LOG1] [LOG2]       Compare two log files

${CYAN}Examples:${NC}
  $0 watch pluck-debug-${BEAD_ID}-stdout-20260709-031217.log
  $0 analyze pluck-debug-${BEAD_ID}-stdout-20260709-031217.log
  $0 monitor $LOG_DIR
  $0 errors pluck-debug-${BEAD_ID}-stderr-20260709-031217.log
  $0 progress pluck-debug-${BEAD_ID}-stdout-20260709-031217.log
  $0 summary $LOG_DIR

EOF
}

# Function to watch a log file with pattern highlighting
watch_log() {
    local log_file="$1"

    if [[ ! -f "$log_file" ]]; then
        echo -e "${RED}Error: Log file not found: $log_file${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Watching log file: $log_file ===${NC}"
    echo -e "${CYAN}Press Ctrl+C to stop${NC}"
    echo ""

    tail -f "$log_file" | while IFS= read -r line; do
        # Highlight different patterns
        if echo "$line" | grep -qi "error\|fatal\|panic"; then
            echo -e "${RED}$line${NC}"
        elif echo "$line" | grep -qi "warn"; then
            echo -e "${YELLOW}$line${NC}"
        elif echo "$line" | grep -qi "pluck\|filter\|candidate"; then
            echo -e "${GREEN}$line${NC}"
        elif echo "$line" | grep -qi "worker\|bead\|strand"; then
            echo -e "${CYAN}$line${NC}"
        else
            echo "$line"
        fi
    done
}

# Function to analyze a log file
analyze_log() {
    local log_file="$1"

    if [[ ! -f "$log_file" ]]; then
        echo -e "${RED}Error: Log file not found: $log_file${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Log File Analysis: $log_file ===${NC}"
    echo ""

    # File info
    local file_size=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
    local line_count=$(wc -l < "$log_file" 2>/dev/null || echo "0")

    echo -e "${BLUE}File Information:${NC}"
    echo "  Size: $file_size bytes"
    echo "  Lines: $line_count"
    echo ""

    # Pattern counts
    echo -e "${BLUE}Pattern Counts:${NC}"
    echo "  Errors: $(grep -ci "error" "$log_file" 2>/dev/null || echo "0")"
    echo "  Warnings: $(grep -ci "warn" "$log_file" 2>/dev/null || echo "0")"
    echo "  Fatal: $(grep -ci "fatal" "$log_file" 2>/dev/null || echo "0")"
    echo "  Panic: $(grep -ci "panic" "$log_file" 2>/dev/null || echo "0")"
    echo "  Pluck: $(grep -ci "pluck" "$log_file" 2>/dev/null || echo "0")"
    echo "  Filter: $(grep -ci "filter" "$log_file" 2>/dev/null || echo "0")"
    echo "  Candidate: $(grep -ci "candidate" "$log_file" 2>/dev/null || echo "0")"
    echo "  Strand: $(grep -ci "strand" "$log_file" 2>/dev/null || echo "0")"
    echo "  Bead: $(grep -ci "bead" "$log_file" 2>/dev/null || echo "0")"
    echo ""

    # Time range
    local first_line=$(head -1 "$log_file" 2>/dev/null || echo "")
    local last_line=$(tail -1 "$log_file" 2>/dev/null || echo "")

    echo -e "${BLUE}Time Range:${NC}"
    echo "  First: $first_line"
    echo "  Last: $last_line"
    echo ""

    # Critical indicators
    echo -e "${BLUE}Critical Status Indicators:${NC}"

    if grep -qi "worker booted" "$log_file"; then
        echo -e "  ${GREEN}✅ Worker successfully booted${NC}"
    else
        echo -e "  ${YELLOW}⚠️  Worker boot status unclear${NC}"
    fi

    if grep -qi "claimed bead" "$log_file"; then
        local claimed=$(grep -i "claimed bead" "$log_file" | head -1)
        echo -e "  ${GREEN}✅ Bead claimed: $claimed${NC}"
    else
        echo -e "  ${YELLOW}⚠️  No bead claim detected${NC}"
    fi

    if grep -qi "agent dispatched" "$log_file"; then
        echo -e "  ${GREEN}✅ Agent dispatched successfully${NC}"
    else
        echo -e "  ${YELLOW}⚠️  Agent dispatch status unclear${NC}"
    fi

    echo ""
}

# Function to monitor all logs in a directory
monitor_logs() {
    local log_dir="$1"

    if [[ ! -d "$log_dir" ]]; then
        echo -e "${RED}Error: Log directory not found: $log_dir${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Monitoring log directory: $log_dir ===${NC}"
    echo -e "${CYAN}Press Ctrl+C to stop${NC}"
    echo ""

    declare -A last_sizes
    declare -A file_counts

    # Initialize tracking
    for log_file in "$log_dir"/*.log; do
        if [[ -f "$log_file" ]]; then
            local filename=$(basename "$log_file")
            last_sizes["$filename"]=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
            file_counts["$filename"]=0
        fi
    done

    while true; do
        sleep 2
        local timestamp=$(date '+%Y-%m-%d %H:%M:%S')

        for log_file in "$log_dir"/*.log; do
            if [[ -f "$log_file" ]]; then
                local filename=$(basename "$log_file")
                local current_size=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
                local last_size=${last_sizes["$filename"]}

                if [[ $current_size -gt $last_size ]]; then
                    local growth=$((current_size - last_size))
                    local file_count=${file_counts["$filename"]}
                    file_counts["$filename"]=$((file_count + 1))

                    echo -e "${GREEN}[$timestamp]${NC} $filename grew by $growth bytes (${current_size} total) - Update #${file_counts["$filename"]}"

                    # Check for new errors
                    if grep -qi "error\|fatal\|panic" "$log_file" 2>/dev/null; then
                        echo -e "  ${RED}⚠️  Contains errors/fatal/panic messages${NC}"
                    fi

                    # Check for progress indicators
                    if grep -qi "pluck\|filter\|candidate" "$log_file" 2>/dev/null; then
                        echo -e "  ${CYAN}🔄 Contains progress indicators${NC}"
                    fi
                fi

                last_sizes["$filename"]=$current_size
            fi
        done
    done
}

# Function to show only errors and warnings
show_errors() {
    local log_file="$1"

    if [[ ! -f "$log_file" ]]; then
        echo -e "${RED}Error: Log file not found: $log_file${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Errors and Warnings: $log_file ===${NC}"
    echo ""

    local error_count=$(grep -ci "error" "$log_file" 2>/dev/null | head -1 || echo "0")
    local warn_count=$(grep -ci "warn" "$log_file" 2>/dev/null | head -1 || echo "0")
    local fatal_count=$(grep -ci "fatal" "$log_file" 2>/dev/null | head -1 || echo "0")
    local panic_count=$(grep -ci "panic" "$log_file" 2>/dev/null | head -1 || echo "0")

    # Clean up counts (remove all non-digit characters)
    error_count=$(echo "$error_count" | tr -cd '0-9')
    warn_count=$(echo "$warn_count" | tr -cd '0-9')
    fatal_count=$(echo "$fatal_count" | tr -cd '0-9')
    panic_count=$(echo "$panic_count" | tr -cd '0-9')

    echo -e "${BLUE}Summary:${NC} Errors: $error_count, Warnings: $warn_count, Fatal: $fatal_count, Panic: $panic_count"
    echo ""

    if [[ $error_count -gt 0 ]]; then
        echo -e "${RED}=== ERRORS ===${NC}"
        grep -i "error" "$log_file" | while IFS= read -r line; do
            echo -e "${RED}$line${NC}"
        done
        echo ""
    fi

    if [[ $warn_count -gt 0 ]]; then
        echo -e "${YELLOW}=== WARNINGS ===${NC}"
        grep -i "warn" "$log_file" | while IFS= read -r line; do
            echo -e "${YELLOW}$line${NC}"
        done
        echo ""
    fi

    if [[ $fatal_count -gt 0 ]]; then
        echo -e "${RED}=== FATAL ===${NC}"
        grep -i "fatal" "$log_file" | while IFS= read -r line; do
            echo -e "${RED}$line${NC}"
        done
        echo ""
    fi

    if [[ $panic_count -gt 0 ]]; then
        echo -e "${MAGENTA}=== PANIC ===${NC}"
        grep -i "panic" "$log_file" | while IFS= read -r line; do
            echo -e "${MAGENTA}$line${NC}"
        done
        echo ""
    fi

    if [[ $error_count -eq 0 && $warn_count -eq 0 && $fatal_count -eq 0 && $panic_count -eq 0 ]]; then
        echo -e "${GREEN}✅ No errors or warnings found!${NC}"
    fi
}

# Function to show progress indicators
show_progress() {
    local log_file="$1"

    if [[ ! -f "$log_file" ]]; then
        echo -e "${RED}Error: Log file not found: $log_file${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Progress Indicators: $log_file ===${NC}"
    echo ""

    echo -e "${BLUE}Pluck-related activity:${NC}"
    grep -i "pluck" "$log_file" | head -10
    echo ""

    echo -e "${BLUE}Filtering activity:${NC}"
    grep -i "filter" "$log_file" | head -10
    echo ""

    echo -e "${BLUE}Candidate processing:${NC}"
    grep -i "candidate" "$log_file" | head -10
    echo ""

    echo -e "${BLUE}Strand activity:${NC}"
    grep -i "strand" "$log_file" | head -10
    echo ""

    echo -e "${BLUE}Bead operations:${NC}"
    grep -i "bead" "$log_file" | head -10
    echo ""

    local total_activity=$(grep -ci "pluck\|filter\|candidate\|strand\|bead" "$log_file" 2>/dev/null || echo "0")
    echo -e "${GREEN}Total activity mentions: $total_activity${NC}"
}

# Function to generate summary of all logs
generate_summary() {
    local log_dir="$1"

    if [[ ! -d "$log_dir" ]]; then
        echo -e "${RED}Error: Log directory not found: $log_dir${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Log Directory Summary: $log_dir ===${NC}"
    echo ""

    local total_size=0
    local total_files=0

    echo -e "${BLUE}Log Files:${NC}"
    for log_file in "$log_dir"/*.log; do
        if [[ -f "$log_file" ]]; then
            local filename=$(basename "$log_file")
            local file_size=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
            local file_lines=$(wc -l < "$log_file" 2>/dev/null || echo "0")

            echo "  📄 $filename"
            echo "     Size: $file_size bytes, Lines: $file_lines"

            # Quick health check
            local errors=$(grep -ci "error" "$log_file" 2>/dev/null | head -1 || echo "0")
            local warnings=$(grep -ci "warn" "$log_file" 2>/dev/null | head -1 || echo "0")

            # Clean up counts (remove all non-digit characters)
            errors=$(echo "$errors" | tr -cd '0-9')
            warnings=$(echo "$warnings" | tr -cd '0-9')

            if [[ $errors -gt 0 ]]; then
                echo -e "     ${RED}⚠️  $errors error(s)${NC}"
            elif [[ $warnings -gt 0 ]]; then
                echo -e "     ${YELLOW}⚠️  $warnings warning(s)${NC}"
            else
                echo -e "     ${GREEN}✅ Clean${NC}"
            fi

            total_size=$((total_size + file_size))
            total_files=$((total_files + 1))
        fi
    done

    echo ""
    echo -e "${BLUE}Directory Statistics:${NC}"
    echo "  Total files: $total_files"
    echo "  Total size: $total_size bytes"
    echo ""
}

# Function to compare two log files
compare_logs() {
    local log1="$1"
    local log2="$2"

    if [[ ! -f "$log1" || ! -f "$log2" ]]; then
        echo -e "${RED}Error: One or both log files not found${NC}"
        exit 1
    fi

    echo -e "${CYAN}=== Log File Comparison ===${NC}"
    echo ""

    echo -e "${BLUE}File 1:${NC} $log1"
    echo -e "${BLUE}File 2:${NC} $log2"
    echo ""

    local size1=$(stat -f%z "$log1" 2>/dev/null || stat -c%s "$log1" 2>/dev/null || echo "0")
    local size2=$(stat -f%z "$log2" 2>/dev/null || stat -c%s "$log2" 2>/dev/null || echo "0")

    echo -e "${BLUE}Size Comparison:${NC}"
    echo "  File 1: $size1 bytes"
    echo "  File 2: $size2 bytes"
    echo "  Difference: $((size2 - size1)) bytes"
    echo ""

    echo -e "${BLUE}Pattern Comparison:${NC}"
    for pattern in error warn fatal panic pluck filter candidate strand bead; do
        local count1=$(grep -ci "$pattern" "$log1" 2>/dev/null || echo "0")
        local count2=$(grep -ci "$pattern" "$log2" 2>/dev/null || echo "0")
        local diff=$((count2 - count1))

        echo "  $pattern: $count1 vs $count2 (diff: $diff)"
    done
    echo ""
}

# Main command processing
case "${1:-}" in
    watch)
        if [[ -z "${2:-}" ]]; then
            echo -e "${RED}Error: Please specify a log file to watch${NC}"
            show_usage
            exit 1
        fi
        watch_log "$2"
        ;;
    analyze)
        if [[ -z "${2:-}" ]]; then
            echo -e "${RED}Error: Please specify a log file to analyze${NC}"
            show_usage
            exit 1
        fi
        analyze_log "$2"
        ;;
    monitor)
        monitor_logs "${2:-$LOG_DIR}"
        ;;
    errors)
        if [[ -z "${2:-}" ]]; then
            echo -e "${RED}Error: Please specify a log file${NC}"
            show_usage
            exit 1
        fi
        show_errors "$2"
        ;;
    progress)
        if [[ -z "${2:-}" ]]; then
            echo -e "${RED}Error: Please specify a log file${NC}"
            show_usage
            exit 1
        fi
        show_progress "$2"
        ;;
    summary)
        generate_summary "${2:-$LOG_DIR}"
        ;;
    compare)
        if [[ -z "${2:-}" || -z "${3:-}" ]]; then
            echo -e "${RED}Error: Please specify two log files to compare${NC}"
            show_usage
            exit 1
        fi
        compare_logs "$2" "$3"
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
