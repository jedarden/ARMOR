# ARMOR Version Drift Check

## Overview

The ARMOR Version Drift Check automatically detects when deployed ARMOR versions fall behind the latest release, with special attention to correctness-labeled releases (bug fixes, security patches).

## Components

### Scripts

- **`scripts/version-drift-check.py`** - Unified wrapper that orchestrates the complete drift check pipeline
- **`scripts/github-release-fetcher.py`** - Fetches ARMOR releases from GitHub API
- **`scripts/find-armor-deployments.py`** - Scans declarative-config for ARMOR deployments
- **`scripts/compare-version-drift.py`** - Compares deployments against releases to detect drift

### Kubernetes Resources

- **`k8s/armor-drift-check-workflowtemplate.yml`** - Argo WorkflowTemplate for running drift checks
- **`k8s/armor-drift-check-cronworkflow.yml`** - Scheduled workflow (runs daily at 9 AM UTC)

### Configuration

- **`config/drift-config.json`** - Default configuration for thresholds and settings

## Usage

### Manual Execution

```bash
# Run with default configuration
python3 scripts/version-drift-check.py

# Run with JSON output
python3 scripts/version-drift-check.py --json

# Run with custom thresholds
python3 scripts/version-drift-check.py --releases-threshold 25 --days-threshold 14

# Run with configuration file
python3 scripts/version-drift-check.py --config config/drift-config.json

# Save report to file
python3 scripts/version-drift-check.py --output /tmp/drift-report.json

# Sort by different fields
python3 scripts/version-drift-check.py --sort-by correctness  # Highlight correctness drift first
python3 scripts/version-drift-check.py --sort-by releases       # Show most releases behind first
python3 scripts/version-drift-check.py --sort-by days          # Show oldest deployments first
```

### Scheduled Execution (Argo Workflow)

The drift check runs automatically daily at 9 AM UTC via the CronWorkflow. To run manually:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: armor-drift-check-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: armor-drift-check
  arguments:
    parameters:
      - name: releases-threshold
        value: "50"
      - name: days-threshold
        value: "30"
EOF
```

## Configuration

Edit `config/drift-config.json` to customize:

```json
{
  "releases_threshold": 50,      // Flag if behind by N releases
  "days_threshold": 30,           // Flag if behind by M days
  "declarative_config_path": "~/declarative-config",
  "github_repo": "jedarden/ARMOR",
  "sort_by": "correctness"         // Default sort field
}
```

## Exit Codes

- **0**: No drift or only routine version bumps
- **1**: Correctness drift detected (missing bug/security fixes)
- **2**: Script error (failed to run)

## Output Format

### Human-Readable

```
================================================================================
ARMOR Version Drift Report
================================================================================
Generated: 2026-07-16 13:57:23 UTC
Thresholds: > 50 releases, > 30 days

Total deployments: 6
With drift: 2
With correctness drift: 1

================================================================================

🔴 iad-kalshi
   Deployed: 0.1.13
   Latest:   v0.1.42
   Releases behind: 29
   Days behind: 45
   🚨 CORRECTNESS DRIFT: Missing correctness releases!

🟡 rs-manager
   Deployed: 0.1.13
   Latest:   v0.1.42
   Releases behind: 29
   Days behind: 45
```

### JSON

```json
{
  "thresholds": {
    "releases": 50,
    "days": 30
  },
  "summary": {
    "total_deployments": 6,
    "with_drift": 2,
    "with_correctness_drift": 1
  },
  "deployments": [
    {
      "cluster": "iad-kalshi",
      "deployed_tag": "0.1.13",
      "latest_tag": "v0.1.42",
      "releases_behind": 29,
      "days_behind": 45,
      "is_drift": true,
      "is_correctness_drift": true,
      "deployed_date": "2026-06-01T00:00:00Z",
      "latest_date": "2026-07-15T00:00:00Z",
      "filepath": "/home/coding/declarative-config/k8s/iad-kalshi/armor/armor-deployment.yml"
    }
  ],
  "generated_at": "2026-07-16T13:57:23.234567",
  "config": {
    "releases_threshold": 50,
    "days_threshold": 30
  }
}
```

## Correctness-Labeled Releases

Releases are flagged as correctness-related if the release notes or tag contain keywords:
- fix, bug, security, correctness, critical, patch, hotfix, urgent, vulnerability, cve, issue, regression

These releases get highest priority in the report and trigger exit code 1 when drift is detected.
