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

## GitHub Release Fetcher

Fetches ARMOR releases from GitHub API and distinguishes correctness-labeled releases from routine version bumps.

### Files

- `github-release-fetcher.py` - Fetches and categorizes releases

### Usage

Run the fetcher:
```bash
python3 scripts/github-release-fetcher.py
```

### Output

Returns structured JSON with release information:
```json
[
  {
    "tag": "v0.1.43",
    "published_at": "2026-03-28T13:09:18Z",
    "is_correctness": true,
    "url": "https://github.com/jedarden/ARMOR/releases/tag/v0.1.43"
  }
]
```

Fields:
- `tag` - Release tag name
- `published_at` - ISO 8601 timestamp when published
- `is_correctness` - Boolean indicating if release is correctness/security-labeled
- `url` - GitHub release page URL

### Correctness Detection

Releases are classified as correctness-labeled if any of these keywords appear in the tag name, release title, or release notes:
- correctness
- fix
- critical
- security
- bug
- patch
- hotfix
- urgent
- vulnerability
- cve
- issue
- regression

### Error Handling

The script handles:
- GitHub API rate limits (includes small delays between paginated requests)
- Network errors and timeouts
- JSON parsing errors
- Empty release lists

All errors are reported to stderr with non-zero exit codes.
