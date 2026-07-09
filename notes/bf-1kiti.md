# Debug Configuration Files Inventory

**Generated:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**Bead:** bf-1kiti

## Overview
Complete inventory of all configuration files in the ARMOR repository that require syntax validation for debug purposes.

## Summary Statistics

| File Type | Count | Locations |
|-----------|-------|-----------|
| YAML (.yaml, .yml) | 9 | Workspace root, deploy/kubernetes/, notes/ |
| JSON (.json) | 0 | N/A |
| TOML (.toml) | 0 | N/A |
| **Total** | **9** | **3 directories** |

> **Note:** This inventory excludes build artifacts (node_modules/, target/), version control (.git/), and beads system database (.beads/) to focus on application debug configurations.

## YAML Configuration Files

### Debug Configuration Files (2 files)

These files contain debug-specific settings and should be prioritized for syntax validation:

1. **`pluck-config.yaml`** (Primary debug config)
   - Path: `/home/coding/ARMOR/pluck-config.yaml`
   - Purpose: Controls Pluck strand debug logging and filtering behavior
   - Contains:
     - Debug level settings (info/debug/trace/off)
     - Filtering decision logging
     - Bead store query logging
     - Split threshold evaluation logging
     - Module-specific debug flags (strand, worker, bead_store, dispatch, claim)
     - Output configuration (log files, timestamps, source location, rotation)
   - Validation priority: **HIGH**

2. **`.needle.yaml`** (NEEDLE framework config)
   - Path: `/home/coding/ARMOR/.needle.yaml`
   - Purpose: Configures NEEDLE strand behavior
   - Contains:
     - Pluck strand configuration
     - Label exclusions
     - Auto-split settings
   - Note: References external debug documentation
   - Validation priority: **MEDIUM**

### Tool Configuration Files (1 file)

3. **`.golangci.yml`** (Go linter configuration)
   - Path: `/home/coding/ARMOR/.golangci.yml`
   - Purpose: golangci-lint configuration
   - Contains: Go version (1.25), linter settings (govet, ineffassign, staticcheck, unused)
   - Validation priority: **LOW** (tool-specific)

### Kubernetes Configuration Files (5 files)

4. **`deployment.yaml`** (Kubernetes Deployment)
   - Path: `/home/coding/ARMOR/deploy/kubernetes/deployment.yaml`
   - Purpose: Kubernetes deployment manifest for ARMOR
   - Contains: Container configuration, environment variables, probes
   - Validation priority: **MEDIUM** (production deployment)

5. **`kustomization.yaml`** (Kustomize configuration)
   - Path: `/home/coding/ARMOR/deploy/kubernetes/kustomization.yaml`
   - Purpose: Kustomize base configuration
   - Contains: Resource references, common labels
   - Validation priority: **MEDIUM** (production deployment)

6. **`service.yaml`** (Kubernetes Service)
   - Path: `/home/coding/ARMOR/deploy/kubernetes/service.yaml`
   - Purpose: Kubernetes service manifest
   - Validation priority: **MEDIUM** (production deployment)

7. **`secret.yaml`** (Kubernetes Secret template)
   - Path: `/home/coding/ARMOR/deploy/kubernetes/secret.yaml`
   - Purpose: Kubernetes secret template
   - Validation priority: **LOW** (template file)

8. **`ingress-dashboard.yaml`** (Kubernetes Ingress)
   - Path: `/home/coding/ARMOR/deploy/kubernetes/ingress-dashboard.yaml`
   - Purpose: Kubernetes ingress for dashboard
   - Validation priority: **MEDIUM** (production deployment)

### Test/Debug Job Files (1 file)

9. **`armor-s8k.3.2.2-duckdb-test-job.yml`** (Test job manifest)
   - Path: `/home/coding/ARMOR/notes/armor-s8k.3.2.2-duckdb-test-job.yml`
   - Purpose: One-shot Kubernetes Job for DuckDB httpfs testing
   - Contains: Test container configuration, S3 credentials, test script
   - Validation priority: **LOW** (historical test artifact)

## JSON Configuration Files

**None found in workspace** (excluded node_modules/, target/, .git/, .beads/)

## TOML Configuration Files

**None found in workspace** (excluded node_modules/, target/, .git/, .beads/)

## Parsing Priority Order

For the next phase (syntax validation), files should be parsed in this order:

1. **HIGH Priority:** `pluck-config.yaml` - Primary debug configuration
2. **MEDIUM Priority:** `.needle.yaml`, all Kubernetes manifests
3. **LOW Priority:** Tool configs, test artifacts

## File Details by Location

### Workspace Root (/home/coding/ARMOR/)
- `.golangci.yml` - Go linter config
- `.needle.yaml` - NEEDLE framework config
- `pluck-config.yaml` - **Primary debug configuration**

### Kubernetes Deployment (/home/coding/ARMOR/deploy/kubernetes/)
- `deployment.yaml` - Main deployment
- `ingress-dashboard.yaml` - Dashboard ingress
- `kustomization.yaml` - Kustomize config
- `secret.yaml` - Secret template
- `service.yaml` - Service definition

### Notes Directory (/home/coding/ARMOR/notes/)
- `armor-s8k.3.2.2-duckdb-test-job.yml` - Historical test job

## Excluded Directories

The following directories were excluded from the search to focus on application debug configurations:
- `node_modules/` - Node.js dependencies
- `target/` - Rust build artifacts
- `.git/` - Git metadata
- `.beads/` - Beads database files (system internal, not debug configs)

## Parsing Readiness

✅ **Ready for parsing phase**
- All files have been identified and categorized
- File paths are absolute and accessible
- YAML files span configuration, deployment, and debug purposes
- Debug configuration is consolidated in `pluck-config.yaml`
- No JSON or TOML configuration files present for this workspace

## Next Steps

1. Implement YAML parser for the 9 YAML files
2. Validate syntax and structure
3. Report any parsing errors or inconsistencies
4. Focus initial parsing on high-priority debug configurations

---
*Inventory created for bead bf-1kiti - Debug Configuration Files Inventory*
