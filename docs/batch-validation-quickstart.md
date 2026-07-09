# Batch Validation Quickstart Guide

## Overview

The batch validation system provides automated syntax checking for all configuration files in the ARMOR workspace. It supports YAML, JSON, and TOML formats with detailed error reporting.

## Quick Start

### Basic Usage

```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace /home/coding/ARMOR"
```

### Check Exit Code

```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace /home/coding/ARMOR"

echo "Exit code: $?"  # 0 = success, 1 = errors found
```

### Save Report to File

```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace /home/coding/ARMOR" > validation-report.txt
```

## Understanding Output

### Success Output
```
✓ pluck-config.yaml (YAML)
✓ deploy/kubernetes/deployment.yaml (YAML)

VALIDATION SUMMARY
Total files:   9
Successful:    9
Errors:        0
```

### Error Output
```
✗ config/invalid.yaml: Location: Line 5, Column 3
  Error Type: STRUCTURE
  Message: Unexpected indentation
  Suggestion: Check YAML structure matches expected format

VALIDATION SUMMARY
Total files:   10
Successful:    9
Errors:        1

Files with errors:
  ✗ config/invalid.yaml
```

## CI/CD Integration

### Argo Workflow Example
```yaml
- name: validate-configs
  container:
    image: nixos/nix:latest
  script:
  - nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run "python3 scripts/debug-config-parser/batch_validate.py --workspace /workspace"
```

### GitHub Actions Example
```yaml
- name: Batch Validate Configs
  run: |
    nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
      "python3 scripts/debug-config-parser/batch_validate.py --workspace $GITHUB_WORKSPACE"
```

## Error Types

| Type | Description | Common Causes |
|------|-------------|---------------|
| SYNTAX | Invalid YAML/JSON syntax | Unmatched quotes, brackets |
| INDENTATION | Inconsistent spacing | Mixed tabs and spaces |
| STRUCTURE | Invalid document structure | Wrong mappings, sequences |
| IO | File access issues | Permissions, missing files |

## Troubleshooting

**Missing PyYAML:**
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run "python3 scripts/debug-config-parser/batch_validate.py"
```

**Permission Denied:**
```bash
chmod +r /path/to/config/files
```

**Exit Code Always 0:**
```bash
set -e  # Ensure error propagation
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run "python3 scripts/debug-config-parser/batch_validate.py"
```

## Advanced Usage

### Validate Specific Directories
```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace /home/coding/ARMOR/deploy"
```

### Create Custom Inventory
```bash
python3 scripts/debug-config-parser/inventory.py --workspace /home/coding/ARMOR --files > inventory.txt
```

## Related Documentation

- **Comprehensive Report:** `docs/batch-validation-report.md`
- **Inventory Reader:** `scripts/debug-config-parser/INVENTORY_READER.md`
- **Batch Validation:** `scripts/debug-config-parser/BATCH_VALIDATION_README.md`
- **Parser Documentation:** `scripts/debug-config-parser/parsers/`