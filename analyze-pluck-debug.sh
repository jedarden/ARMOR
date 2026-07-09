#!/bin/bash
# Pluck Debug Log Analyzer
# This script analyzes captured debug logs and provides structured output

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

LOG_FILE="${1:?Usage: $0 <log_file>}"

if [[ ! -f "$LOG_FILE" ]]; then
    echo -e "${RED}Error: Log file not found: $LOG_FILE${NC}"
    exit 1
fi

echo -e "${BLUE}=== Pluck Debug Log Analysis ===${NC}"
echo "File: $LOG_FILE"
echo "Size: $(wc -c < "$LOG_FILE") bytes"
echo "Lines: $(wc -l < "$LOG_FILE") lines"
echo ""

# Overall statistics
echo -e "${CYAN}=== Overall Statistics ===${NC}"
echo "Lines containing 'pluck': $(grep -ci 'pluck' "$LOG_FILE" || echo '0')"
echo "Lines containing 'filter': $(grep -ci 'filter' "$LOG_FILE" || echo '0')"
echo "Lines containing 'candidate': $(grep -ci 'candidate' "$LOG_FILE" || echo '0')"
echo "Lines containing 'exclude': $(grep -ci 'exclude' "$LOG_FILE" || echo '0')"
echo "Lines containing 'split': $(grep -ci 'split' "$LOG_FILE" || echo '0')"
echo ""

# Pluck strand evaluation
echo -e "${CYAN}=== Pluck Strand Evaluation ===${NC}"
if grep -q "Pluck strand evaluation starting" "$LOG_FILE"; then
    grep "Pluck strand evaluation starting" "$LOG_FILE" | head -1
else
    echo -e "${YELLOW}No Pluck strand evaluation found${NC}"
fi
echo ""

# Filtering decisions
echo -e "${CYAN}=== Filtering Decisions ===${NC}"
FILTER_COUNT=$(grep -c "filtering" "$LOG_FILE" 2>/dev/null)
FILTER_COUNT=${FILTER_COUNT:-0}
echo "Total filtering operations: $FILTER_COUNT"

if grep -q "Filtering.*candidates" "$LOG_FILE"; then
    echo ""
    echo "Label filtering:"
    grep "Filtering.*candidates.*label" "$LOG_FILE" | head -5
fi

if grep -q "excluded" "$LOG_FILE"; then
    echo ""
    echo "Exclusion decisions:"
    grep "excluded" "$LOG_FILE" | head -10
fi
echo ""

# Candidate information
echo -e "${CYAN}=== Candidate Information ===${NC}"
if grep -q "candidates" "$LOG_FILE"; then
    echo "Candidate counts:"
    grep "candidates" "$LOG_FILE" | grep -E "(count|returned|remaining)" | head -10
fi

if grep -q "Sorting.*candidates" "$LOG_FILE"; then
    echo ""
    echo "Sorting operations:"
    grep "Sorting.*candidates" "$LOG_FILE" | head -5
fi
echo ""

# Split decisions
echo -e "${CYAN}=== Split Decisions ===${NC}"
if grep -q "split" "$LOG_FILE"; then
    grep -i "split" "$LOG_FILE" | head -10
else
    echo "No split operations found"
fi
echo ""

# Bead store queries
echo -e "${CYAN}=== Bead Store Queries ===${NC}"
if grep -q "Querying bead store" "$LOG_FILE"; then
    grep "Querying bead store" "$LOG_FILE" | head -5
else
    echo "No bead store queries found"
fi
echo ""

# Errors and warnings
echo -e "${CYAN}=== Errors and Warnings ===${NC}"
ERROR_COUNT=$(grep -c "ERROR" "$LOG_FILE" 2>/dev/null || echo "0")
WARN_COUNT=$(grep -c "WARN" "$LOG_FILE" 2>/dev/null || echo "0")

echo "Errors: $ERROR_COUNT"
echo "Warnings: $WARN_COUNT"

if [[ $ERROR_COUNT -gt 0 ]]; then
    echo ""
    echo "Error details:"
    grep "ERROR" "$LOG_FILE" | head -10
fi

if [[ $WARN_COUNT -gt 0 ]]; then
    echo ""
    echo "Warning details:"
    grep "WARN" "$LOG_FILE" | head -10
fi
echo ""

# Final results
echo -e "${CYAN}=== Final Results ===${NC}"
if grep -q "Result:" "$LOG_FILE"; then
    grep "Result:" "$LOG_FILE" | tail -5
elif grep -q "Returning.*candidates" "$LOG_FILE"; then
    grep "Returning.*candidates" "$LOG_FILE" | tail -5
else
    echo "No final results found"
fi
echo ""

# Detailed analysis options
echo -e "${BLUE}=== Detailed Analysis Commands ===${NC}"
echo "For deeper analysis, you can use:"
echo "  grep 'bead_id=' $LOG_FILE | head -20"
echo "  grep 'priority=' $LOG_FILE | head -20"
echo "  grep 'labels=' $LOG_FILE | head -20"
echo "  grep -A 2 -B 2 'filtering' $LOG_FILE"
echo ""

# Quick diagnosis
echo -e "${CYAN}=== Quick Diagnosis ===${NC}"

# Check if Pluck debug logging is working
if grep -q "needle::strand::pluck" "$LOG_FILE" || grep -q "strand.pluck" "$LOG_FILE"; then
    echo -e "${GREEN}✓ Pluck debug logging is active${NC}"
else
    echo -e "${YELLOW}⚠ No Pluck-specific debug output found${NC}"
    echo "  This might indicate:"
    echo "  - RUST_LOG not set correctly"
    echo "  - No Pluck evaluation occurred"
    echo "  - Worker claimed a bead immediately"
fi

# Check for detailed filtering output
if grep -q "Filtering.*candidates" "$LOG_FILE"; then
    echo -e "${GREEN}✓ Detailed filtering output present${NC}"
else
    echo -e "${YELLOW}⚠ No detailed filtering output found${NC}"
    echo "  Try using 'detailed' or 'comprehensive' mode"
fi

# Check for candidates
CANDIDATE_CHECK=$(grep -c "candidates" "$LOG_FILE" 2>/dev/null || echo "0")
COUNT_CHECK=$(grep -c "count" "$LOG_FILE" 2>/dev/null || echo "0")
if [[ "$CANDIDATE_CHECK" -gt 0 ]] && [[ "$COUNT_CHECK" -gt 0 ]]; then
    echo -e "${GREEN}✓ Candidate information present${NC}"
else
    echo -e "${YELLOW}⚠ No candidate information found${NC}"
fi

echo ""
echo -e "${BLUE}=== Analysis Complete ===${NC}"
