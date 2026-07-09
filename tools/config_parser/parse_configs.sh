#!/usr/bin/env bash
#
# Configuration Parser Wrapper Script
# This script ensures PyYAML is available via nix-shell before running the parser
#

set -euo pipefail

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARSER_SCRIPT="${SCRIPT_DIR}/parse_configs.py"

# Check if PyYAML is available in the current environment
if python3 -c "import yaml" 2>/dev/null; then
    # PyYAML is available, run directly
    python3 "${PARSER_SCRIPT}" "$@"
else
    # PyYAML not available, use nix-shell to provide it
    echo "PyYAML not available in current environment, using nix-shell..." >&2
    nix-shell -p python3Packages.pyyaml --run "python3 '${PARSER_SCRIPT}' '$*'"
fi
