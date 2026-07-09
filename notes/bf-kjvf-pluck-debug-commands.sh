#!/bin/bash
# Pluck Debug Command Examples - bf-kjvf
# This file contains validated Pluck debug command examples

# Basic validation tests
echo "=== Pluck Debug Command Validation ==="

# Test 1: RUST_LOG configuration
echo "Test 1: RUST_LOG environment variable"
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"
echo "✓ RUST_LOG configured: $RUST_LOG"

# Test 2: needle availability
echo -e "\nTest 2: needle binary"
which needle
echo "✓ needle found at: $(which needle)"
needle --version
echo "✓ needle version validated"

# Test 3: workspace path
echo -e "\nTest 3: workspace path"
test -d /home/coding/ARMOR && echo "✓ Workspace exists: /home/coding/ARMOR"

# Test 4: Configuration script
echo -e "\nTest 4: pluck-debug-config.sh script"
test -x pluck-debug-config.sh && echo "✓ Script is executable"

# Primary debug commands
echo -e "\n=== Primary Pluck Debug Commands ==="

echo -e "\n1. COMPREHENSIVE (Recommended):"
echo 'RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" needle run -w /home/coding/ARMOR -c 1'

echo -e "\n2. With log capture:"
echo 'RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug" needle run -w /home/coding/ARMOR -c 1 2>&1 | tee logs/pluck-debug/pluck-debug-capture-$(date +%Y%m%d-%H%M%S).log'

echo -e "\n3. Using configuration script (standard):"
echo './pluck-debug-config.sh /home/coding/ARMOR pluck-debug-output.log standard'

echo -e "\n4. Using configuration script (comprehensive):"
echo './pluck-debug-config.sh /home/coding/ARMOR pluck-debug-comprehensive.log comprehensive'

echo -e "\n5. Using configuration script (full with count):"
echo './pluck-debug-config.sh /home/coding/ARMOR pluck-debug-full.log full 3'

# Debug level presets
echo -e "\n=== Debug Level Presets ==="

echo -e "\nMINIMAL (info):"
echo 'RUST_LOG="needle::strand::pluck=info" needle run -w /home/coding/ARMOR -c 1'

echo -e "\nSTANDARD (debug - recommended for normal debugging):"
echo 'RUST_LOG="needle::strand::pluck=debug" needle run -w /home/coding/ARMOR -c 1'

echo -e "\nDETAILED (trace):"
echo 'RUST_LOG="needle::strand::pluck=trace" needle run -w /home/coding/ARMOR -c 1'

echo -e "\nCOMPREHENSIVE (multi-module):"
echo 'RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug" needle run -w /home/coding/ARMOR -c 1'

echo -e "\nFULL (all NEEDLE modules):"
echo 'RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::claim=debug" needle run -w /home/coding/ARMOR -c 1'

echo -e "\nMAXIMUM (everything):"
echo 'RUST_LOG="trace" needle run -w /home/coding/ARMOR -c 1'

# Additional options
echo -e "\n=== Additional Command Options ==="

echo -e "\nWith agent specification:"
echo 'RUST_LOG="..." needle run -w /home/coding/ARMOR -c 1 -a <agent_type>'

echo -e "\nWith worker identifier:"
echo 'RUST_LOG="..." needle run -w /home/coding/ARMOR -c 1 -i <identifier>'

echo -e "\nWith timeout:"
echo 'RUST_LOG="..." needle run -w /home/coding/ARMOR -c 1 -t <timeout_seconds>'

echo -e "\n=== Validation Complete ==="
