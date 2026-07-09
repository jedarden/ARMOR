#!/bin/bash
# Pluck Debug Configuration Manager
# This script provides preset configurations for different debug levels

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

WORKSPACE="${1:-/home/coding/ARMOR}"
OUTPUT="${2:-pluck-debug-$(date +%Y%m%d-%H%M%S).log}"
MODE="${3:-standard}"
COUNT="${4:-1}"

# Configuration presets
declare -A PRESETS=(
    ["minimal"]="needle::strand::pluck=info"
    ["standard"]="needle::strand::pluck=debug"
    ["detailed"]="needle::strand::pluck=trace"
    ["comprehensive"]="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug"
    ["full"]="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug"
    ["maximum"]="trace"
)

show_usage() {
    echo -e "${BLUE}Pluck Debug Configuration Manager${NC}"
    echo ""
    echo "Usage: $0 [workspace] [output_file] [mode] [count]"
    echo ""
    echo "Modes:"
    echo "  minimal       - INFO level: High-level strand operations only"
    echo "  standard      - DEBUG level: Filtering decisions and statistics (default)"
    echo "  detailed      - TRACE level: Complete execution details"
    echo "  comprehensive - TRACE + supporting modules (bead_store, worker)"
    echo "  full          - All NEEDLE modules at DEBUG/TRACE level"
    echo "  maximum       - Everything at TRACE level (very verbose)"
    echo ""
    echo "Examples:"
    echo "  $0 /home/coding/ARMOR output.log standard 1"
    echo "  $0 /home/coding/ARMOR output.log detailed"
    echo "  $0 /home/coding/ARMOR output.log comprehensive"
    echo ""
}

show_configuration() {
    local mode=$1
    local rust_log=${PRESETS[$mode]}

    echo -e "${GREEN}Configuration: $mode${NC}"
    echo -e "RUST_LOG: ${YELLOW}$rust_log${NC}"
    echo ""
    echo "Output will be saved to: $OUTPUT"
    echo "Workspace: $WORKSPACE"
    echo "Count: $COUNT"
    echo ""
}

run_debug_capture() {
    local mode=$1
    local rust_log=${PRESETS[$mode]}

    show_configuration "$mode"

    echo -e "${BLUE}Starting NEEDLE with $mode debug logging...${NC}"
    echo ""

    # Export and run
    export RUST_LOG="$rust_log"
    RUST_LOG="$rust_log" needle run -w "$WORKSPACE" -c "$COUNT" 2>&1 | tee "$OUTPUT"

    echo ""
    echo -e "${GREEN}Capture complete!${NC}"
    echo -e "Output saved to: ${YELLOW}$OUTPUT${NC}"

    # Show summary
    echo ""
    echo -e "${BLUE}=== Capture Summary ===${NC}"
    echo "File size: $(wc -c < "$OUTPUT") bytes"
    echo "Line count: $(wc -l < "$OUTPUT") lines"
    echo ""
    echo -e "${BLUE}=== Quick Analysis ===${NC}"
    echo "Lines containing 'pluck': $(grep -ci 'pluck' "$OUTPUT" || echo '0')"
    echo "Lines containing 'filter': $(grep -ci 'filter' "$OUTPUT" || echo '0')"
    echo "Lines containing 'candidate': $(grep -ci 'candidate' "$OUTPUT" || echo '0')"
    echo "Lines containing 'exclude': $(grep -ci 'exclude' "$OUTPUT" || echo '0')"
}

# Check if help is requested
if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    show_usage
    exit 0
fi

# Validate mode
if [[ -z "${PRESETS[$MODE]}" ]]; then
    echo -e "${RED}Error: Invalid mode '$MODE'${NC}"
    echo ""
    show_usage
    exit 1
fi

# Check if workspace exists
if [[ ! -d "$WORKSPACE" ]]; then
    echo -e "${RED}Error: Workspace directory does not exist: $WORKSPACE${NC}"
    exit 1
fi

# Run the debug capture
run_debug_capture "$MODE"

echo ""
echo -e "${BLUE}=== Analysis Commands ===${NC}"
echo "To analyze the output:"
echo "  grep -i 'pluck' $OUTPUT"
echo "  grep -i 'filter' $OUTPUT"
echo "  grep -i 'exclude' $OUTPUT"
echo "  grep -i 'candidate' $OUTPUT"
echo "  grep -i 'split' $OUTPUT"
