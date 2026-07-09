#!/bin/bash
# Debug Configuration File Validation Script
# Validates syntax and structure of all debug configuration files

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

ERRORS=0
WARNINGS=0
TOTAL_FILES=0
VALID_FILES=0

echo -e "${BLUE}=== Debug Configuration File Validation ===${NC}"
echo ""

# Primary configuration files
echo -e "${BLUE}=== Primary Configuration Files ===${NC}"

# pluck-config.yaml
TOTAL_FILES=$((TOTAL_FILES + 1))
echo -n "Validating pluck-config.yaml... "

FILE="/home/coding/ARMOR/pluck-config.yaml"
if [[ ! -r "$FILE" ]]; then
    echo -e "${RED}✗ NOT READABLE${NC}"
    ERRORS=$((ERRORS + 1))
else
    # Check basic structure
    if grep -q "^debug:" "$FILE" && grep -q "^modules:" "$FILE" && grep -q "^filtering:" "$FILE" && grep -q "^output:" "$FILE"; then
        echo -e "${GREEN}✓ VALID${NC}"
        VALID_FILES=$((VALID_FILES + 1))

        # Check debug section structure
        echo "  Checking debug section..."
        if grep -q "  level:" "$FILE" && grep -q "  log_filtering_decisions:" "$FILE"; then
            echo "    ${GREEN}✓${NC} Debug structure complete"
        else
            echo "    ${YELLOW}⚠${NC} Debug structure incomplete"
            WARNINGS=$((WARNINGS + 1))
        fi
    else
        echo -e "${RED}✗ INVALID STRUCTURE${NC}"
        ERRORS=$((ERRORS + 1))
    fi
fi

# .env.pluck-debug
TOTAL_FILES=$((TOTAL_FILES + 1))
echo -n "Validating .env.pluck-debug... "

FILE="/home/coding/ARMOR/.env.pluck-debug"
if [[ ! -r "$FILE" ]]; then
    echo -e "${RED}✗ NOT READABLE${NC}"
    ERRORS=$((ERRORS + 1))
else
    # Check for valid export statements
    if grep -q "^export RUST_LOG=" "$FILE"; then
        echo -e "${GREEN}✓ VALID${NC}"
        VALID_FILES=$((VALID_FILES + 1))

        # Check RUST_LOG format
        RUST_LOG_VALUE=$(grep "^export RUST_LOG=" "$FILE" | cut -d'=' -f2)
        if [[ -n "$RUST_LOG_VALUE" ]]; then
            echo "  RUST_LOG configured: ${RUST_LOG_VALUE:0:50}..."
        fi
    else
        echo -e "${RED}✗ INVALID${NC}"
        ERRORS=$((ERRORS + 1))
    fi
fi

# Shell scripts
echo ""
echo -e "${BLUE}=== Shell Script Files ===${NC}"

SCRIPTS=(
    "pluck-debug-config.sh"
    "capture-pluck-debug.sh"
    "analyze-pluck-debug.sh"
)

for script in "${SCRIPTS[@]}"; do
    TOTAL_FILES=$((TOTAL_FILES + 1))
    echo -n "Validating $script... "

    FILE="/home/coding/ARMOR/$script"
    if [[ ! -r "$FILE" ]]; then
        echo -e "${RED}✗ NOT READABLE${NC}"
        ERRORS=$((ERRORS + 1))
    elif [[ ! -x "$FILE" ]]; then
        echo -e "${YELLOW}⚠ NOT EXECUTABLE${NC}"
        WARNINGS=$((WARNINGS + 1))
        VALID_FILES=$((VALID_FILES + 1))
    else
        # Check shebang
        if head -1 "$FILE" | grep -q "^#!/"; then
            # Check syntax
            if bash -n "$FILE" 2>/dev/null; then
                echo -e "${GREEN}✓ VALID${NC}"
                VALID_FILES=$((VALID_FILES + 1))
            else
                echo -e "${RED}✗ SYNTAX ERROR${NC}"
                ERRORS=$((ERRORS + 1))
            fi
        else
            echo -e "${YELLOW}⚠ NO SHEBANG${NC}"
            WARNINGS=$((WARNINGS + 1))
        fi
    fi
done

# Structure validation for YAML
echo ""
echo -e "${BLUE}=== YAML Structure Validation ===${NC}"

# Check for expected keys in pluck-config.yaml
echo -n "Checking pluck-config.yaml expected keys... "
EXPECTED_KEYS=("debug" "modules" "filtering" "output")
FOUND_KEYS=0

for key in "${EXPECTED_KEYS[@]}"; do
    if grep -q "^${key}:" "/home/coding/ARMOR/pluck-config.yaml" 2>/dev/null; then
        FOUND_KEYS=$((FOUND_KEYS + 1))
    fi
done

if [[ $FOUND_KEYS -eq ${#EXPECTED_KEYS[@]} ]]; then
    echo -e "${GREEN}✓ COMPLETE${NC}"
    echo "  Found all $FOUND_KEYS expected top-level keys"
else
    echo -e "${YELLOW}⚠ INCOMPLETE${NC}"
    echo "  Found $FOUND_KEYS out of ${#EXPECTED_KEYS[@]} expected keys"
    WARNINGS=$((WARNINGS + 1))
fi

# Check debug section structure
echo -n "Checking debug section structure... "
DEBUG_KEYS=("level" "log_filtering_decisions" "log_bead_store_queries" "log_split_evaluation")
DEBUG_FOUND=0

for key in "${DEBUG_KEYS[@]}"; do
    if grep -q "  ${key}:" "/home/coding/ARMOR/pluck-config.yaml" 2>/dev/null; then
        DEBUG_FOUND=$((DEBUG_FOUND + 1))
    fi
done

if [[ $DEBUG_FOUND -eq ${#DEBUG_KEYS[@]} ]]; then
    echo -e "${GREEN}✓ COMPLETE${NC}"
    echo "  Found all $DEBUG_FOUND expected debug keys"
else
    echo -e "${YELLOW}⚠ INCOMPLETE${NC}"
    echo "  Found $DEBUG_FOUND out of ${#DEBUG_KEYS[@]} expected debug keys"
    WARNINGS=$((WARNINGS + 1))
fi

# Final summary
echo ""
echo -e "${BLUE}=== Validation Summary ===${NC}"
echo "Total files validated: $TOTAL_FILES"
echo -e "Valid files: ${GREEN}$VALID_FILES${NC}"
echo -e "Errors: ${RED}$ERRORS${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"

if [[ $ERRORS -eq 0 ]]; then
    echo ""
    echo -e "${GREEN}✓ ALL VALIDATION CHECKS PASSED${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}✗ VALIDATION FAILED${NC}"
    exit 1
fi
