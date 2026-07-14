# ARMOR Scripts

This directory contains utility scripts for ARMOR operations and monitoring.

## Version Drift Check

Automated version drift monitoring across ARMOR deployments.

### Files

- `check-armor-version-drift.py` - Main Python script for checking version drift
- `check-version-drift.sh` - Bash alternative (requires GitHub API access)
- `armor-version-drift-check.cron` - Cron job definition
- `setup-version-drift-schedule.sh` - Setup script to install cron job

### Usage

Run the check manually:
```bash
python3 scripts/check-armor-version-drift.py
```

Output JSON format:
```bash
python3 scripts/check-armor-version-drift.py --json
```

### What It Checks

The script scans ARMOR deployments across all clusters in `jedarden/declarative-config` and:
1. Reads the deployed ARMOR image tag from each deployment file
2. Compares against the current ARMOR version (from `VERSION` file)
3. Calculates version drift (versions behind, days behind)
4. Checks for correctness/security releases between versions
5. Flags deployments needing attention

### Clusters Checked

- iad-ci
- iad-kalshi
- rs-manager
- ord-devimprint
- iad-native-ads
- iad-acb

### Thresholds

By default, deployments are flagged if:
- More than 50 versions behind current
- More than 30 days behind current
- Any correctness/security releases missed

These thresholds can be adjusted in the script.

### Scheduling

To schedule daily checks (if cron is available):
```bash
bash scripts/setup-version-drift-schedule.sh
```

The check runs daily at 9:17 AM (avoiding :00/:00 load spikes).

Logs are written to `logs/version-drift-check.log`.

### Output

The script outputs:
- Human-readable summary to stdout
- Detailed JSON when run with `--json` flag
- Flags for:
  - 🔴 Deployments needing update
  - 🔶 Non-version tags (git SHAs)
  - ⚠️  Missed correctness releases
