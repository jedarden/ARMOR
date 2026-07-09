# Batch Configuration File Validation

## Overview

The batch validation orchestrator provides comprehensive validation of all configuration files in the ARMOR workspace. It integrates the inventory reader with parser factory to deliver file-by-file validation with detailed reporting and CI/CD integration.

## Usage

### Basic Usage (ARMOR workspace)
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py"
```

### Custom Workspace
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace /path/to/workspace"
```

### Direct Python (with dependencies installed)
```bash
python3 scripts/debug-config-parser/batch_validate.py --workspace /home/coding/ARMOR
```

## Output Format

### Successful Validation
```
Batch Configuration File Validation
======================================================================

Step 1: Discovering configuration files...
  Found 9 configuration files
  YAML: 9
  JSON: 0
  TOML: 0

Step 2: Validating file syntax...
  ✓ .golangci.yml (YAML)
  ✓ .needle.yaml (YAML)
  ✓ deploy/kubernetes/deployment.yaml (YAML)
  ...

======================================================================
VALIDATION SUMMARY
======================================================================
Total files:   9
Successful:    9
Warnings:      0
Errors:        0
```

### With Errors
```
Files with errors:
  ✗ invalid.yaml
    Location: Line 1, Column 14
  Error Type: STRUCTURE
  Message: Line 1, Column 14: mapping values are not allowed here
  Suggestion: Verify the YAML structure matches expected format
```

## Acceptance Criteria ✅

- ✅ **Batch processor validates all files in inventory** - Discovers and validates all config files
- ✅ **Comprehensive report with file-by-file results** - Shows status for each file processed
- ✅ **Summary statistics** - Total/Success/Failed counts in validation summary
- ✅ **Lists files with syntax errors** - Detailed error section with location and suggestions
- ✅ **CI/CD integration** - Exit code 0 (success) or 1 (errors found)
- ✅ **Integration-ready** - CLI interface with argparse, workspace argument support

## CI/CD Integration

### GitHub Actions (via Argo Workflows)
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: armor-config-validation-
spec:
  entrypoint: validate-configs
  templates:
  - name: validate-configs
    container:
      image: nixos/nix:latest
      command:
        - nix-shell
        - -p
        - python3.pkgs.pyyaml
        - python3.pkgs.tomli
        - --run
        - "python3 scripts/debug-config-parser/batch_validate.py --workspace /workspace"
```

### Shell Script Wrapper
```bash
#!/usr/bin/env bash
set -e

echo "Validating ARMOR configuration files..."
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace /home/coding/ARMOR"

if [ $? -eq 0 ]; then
  echo "✓ All configuration files are valid"
else
  echo "✗ Configuration validation failed"
  exit 1
fi
```

## Exit Codes

- **0** - All configuration files validated successfully
- **1** - One or more configuration files have syntax errors

## Supported File Types

- **YAML** - `*.yaml`, `*.yml` 
- **JSON** - `*.json`
- **TOML** - `*.toml`

## Excluded Directories

The validator automatically excludes:
- `.git/` - Version control
- `.beads/` - Bead tracking
- `target/` - Rust build artifacts
- `node_modules/` - Node.js dependencies
- `logs/` - Log files
- `.cache/` - Cache directories
- `__pycache__/` - Python cache
- `.pytest_cache/` - pytest cache
- `dist/`, `build/` - Build artifacts

## Technical Details

### Components
- **DebugFileInventoryReader** - Discovers and categorizes configuration files
- **ParserFactory** - Routes files to appropriate parser (YAML/JSON/TOML)
- **BatchValidator** - Orchestrates validation and reporting

### Error Handling
- Detailed error messages with location information
- Error categorization (STRUCTURE, SYNTAX, etc.)
- Contextual suggestions for common issues
- Proper exit codes for automation

## Dependencies

- Python 3.12+
- PyYAML (`python3.pkgs.pyyaml`)
- tomli (`python3.pkgs.tomli`)

Install via nix-shell:
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli
```

## Examples

### ARMOR Workspace (current)
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py"
```

### Specific Directory
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace ~/other-project"
```

### With Exit Code Capture
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py; echo \"Exit: \$?\""
```

## Related Documentation

- [INVENTORY_READER.md](INVENTORY_READER.md) - File discovery and inventory details
- [README.md](README.md) - Main parser documentation
- [parsers/](parsers/) - Individual parser implementations
