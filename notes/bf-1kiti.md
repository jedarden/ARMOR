# Debug Configuration Files Inventory

## Overview
Complete inventory of all configuration files in the ARMOR repository that require syntax validation.

## Summary Statistics
- **Total Configuration Files:** 87
- **YAML Files:** 10
- **JSON Files:** 77  
- **TOML Files:** 0

## YAML Files (.yaml, .yml) - 10 files

### Project Configuration (3 files)
- `/home/coding/ARMOR/pluck-config.yaml` - Pluck dependency configuration
- `/home/coding/ARMOR/.needle.yaml` - NEEDLE build system configuration
- `/home/coding/ARMOR/.golangci.yml` - Go linting configuration

### Beads Configuration (1 file)
- `/home/coding/ARMOR/.beads/config.yaml` - Bead tracking system configuration

### Kubernetes Deployment (5 files)
- `/home/coding/ARMOR/deploy/kubernetes/deployment.yaml` - Main deployment configuration
- `/home/coding/ARMOR/deploy/kubernetes/ingress-dashboard.yaml` - Dashboard ingress configuration
- `/home/coding/ARMOR/deploy/kubernetes/kustomization.yaml` - Kustomize configuration
- `/home/coding/ARMOR/deploy/kubernetes/secret.yaml` - Secret resource configuration
- `/home/coding/ARMOR/deploy/kubernetes/service.yaml` - Service configuration

### Notes/Debug (1 file)
- `/home/coding/ARMOR/notes/armor-s8k.3.2.2-duckdb-test-job.yml` - DuckDB test job configuration

## JSON Files (.json) - 77 files

### Beads System Configuration (1 file)
- `/home/coding/ARMOR/.beads/metadata.json` - Beads system metadata

### Trace Metadata Files (76 files)
All located in `/home/coding/ARMOR/.beads/traces/*/metadata.json`

These files track execution traces for various beads:
- Armor system traces (armor-bik, armor-l64, armor-oxd, armor-s8k.*)
- Bead function traces (bf-135k, bf-19os, bf-1bl4, etc.)

**Sample files:**
- `/home/coding/ARMOR/.beads/traces/bf-tr44/metadata.json`
- `/home/coding/ARMOR/.beads/traces/armor-s8k.3/metadata.json`
- `/home/coding/ARMOR/.beads/traces/bf-2928/metadata.json`

**Complete list:** 76 trace metadata files across various bead executions

## TOML Files (.toml) - 0 files
No TOML configuration files found in the repository.

## Parsing Readiness
✅ **Ready for parsing phase**
- All files have been identified and categorized
- File paths are absolute and accessible
- YAML files span configuration, deployment, and debug purposes
- JSON files are primarily metadata files with consistent structure
- No TOML files present, eliminating that parsing requirement

## Next Steps
1. Implement YAML parser for the 10 YAML files
2. Implement JSON parser for the 77 JSON files  
3. Validate syntax and structure
4. Report any parsing errors or inconsistencies

---
*Inventory created for bead bf-1kiti - Debug Configuration Files Inventory*
*Generated: 2026-07-09*
