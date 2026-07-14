# ARMOR Version Drift Check

## Overview

The ARMOR Version Drift Check tool monitors deployed ARMOR versions across all clusters and flags deployments that are significantly behind the current release.

## Deployments Monitored

The following clusters are checked:
- **iad-ci**: `iad-ci/armor/armor-deployment.yaml`
- **iad-kalshi**: `iad-kalshi/armor/armor-deployment.yml`
- **rs-manager**: `rs-manager/armor/armor-deployment.yml`
- **ord-devimprint**: `ord-devimprint/devimprint/armor-deployment.yml`
- **iad-native-ads**: `iad-native-ads/armor/armor-deployment.yml`
- **iad-acb**: `iad-acb/ai-code-battle/acb-armor-deployment.yml`

## Warning Thresholds

Deployments are flagged if they meet any of these criteria:
- **Version drift**: More than 50 versions behind current
- **Time drift**: More than 30 days behind current
- **Correctness releases**: Any bug fix or security release was missed
- **Non-version tags**: Using git SHA instead of version tag

## Usage

### Manual Check

```bash
# Run the check
./scripts/check-armor-version-drift.py

# Get JSON output for integration
./scripts/check-armor-version-drift.py --json
```

### Scheduling

Run the setup script to add a daily cron job:

```bash
./scripts/setup-version-drift-schedule.sh
```

This schedules the check to run daily at 9:17 AM (avoiding :00 and :30 marks to reduce API load).

#### Manual Cron Setup

If you prefer to manually configure the schedule:

```bash
# Edit crontab
crontab -e

# Add this line (runs daily at 9:17 AM)
17 9 * * * /home/coding/ARMOR/scripts/check-armor-version-drift.py >> /home/coding/ARMOR/logs/version-drift-check.log 2>&1
```

## Output

The tool provides:

1. **Human-readable output**: Shows each cluster's deployed version, drift metrics, and any correctness releases missed
2. **JSON output**: Structured data suitable for integration with monitoring systems
3. **Exit codes**: 0 for success, non-zero for errors

### Example Output

```
ARMOR Version Drift Check
Current version: 0.1.1804
Current version date: 2026-07-14
Warning thresholds: > 50 versions, > 30 days
================================================================================

🔴 iad-ci: 0.1.24
   Deployed: 2026-07-09
   Versions behind: 1780
   Days behind: 5
   ⚠️  UPDATE RECOMMENDED: 1780 versions behind

================================================================================
SUMMARY
Total deployments checked: 6
Deployments needing update: 5
Using non-version tags: 1
```

## Correctness Release Detection

The tool distinguishes between routine version bumps and correctness/security releases by examining the commit messages associated with each version. Commits containing keywords like "fix", "bug", "security", "correct", "vulnerability", or "patch" are flagged as correctness releases.

These are highlighted separately in the output:

```
⚠️  MISSED CORRECTNESS RELEASES (3):
   - 0.1.1802: fix: multipart upload regression
   - 0.1.1795: security: strengthen S3 auth validation
   - 0.1.1788: fix: correct credential rejection logic
```

## Monitoring Integration

### Example: Alert on Critical Drift

```python
import json
import subprocess

result = subprocess.run(
    ["./scripts/check-armor-version-drift.py", "--json"],
    capture_output=True,
    text=True
)

data = json.loads(result.stdout)

# Check for deployments needing update
needs_update = [d for d in data["deployments"] if d["needs_update"]]

if len(needs_update) > 0:
    print(f"ALERT: {len(needs_update)} deployments need updates!")
    for deployment in needs_update:
        print(f"  - {deployment['cluster']}: {deployment['deployed_tag']}")
```

### Example: Check for Non-Version Tags

```python
import json

result = subprocess.run(
    ["./scripts/check-armor-version-drift.py", "--json"],
    capture_output=True,
    text=True
)

data = json.loads(result.stdout)

non_version = [d for d in data["deployments"] if d["using_non_version_tag"]]

if len(non_version) > 0:
    print(f"WARNING: {len(non_version)} deployments using non-version tags")
```

## Troubleshooting

### Deployment File Not Found

If you see warnings about deployment files not being found:
1. Check that `~/declarative-config` exists and is up to date
2. Verify the deployment path in the script matches the actual file location

### Git History Issues

If commit date lookups fail:
1. Ensure you're in the ARMOR repository
2. Check that git history is available: `git log --oneline`

### Large Version Numbers

ARMOR uses auto-versioning on every commit, so version numbers can be large (e.g., 0.1.1804). This is expected behavior. The tool focuses on relative drift rather than absolute version numbers.

## Maintenance

To update the list of monitored deployments:
1. Edit `scripts/check-armor-version-drift.py`
2. Modify the `DEPLOYMENTS` list with new `(cluster, path)` tuples
3. Test the changes: `./scripts/check-armor-version-drift.py`
