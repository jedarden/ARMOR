# ARMOR Scripts

This directory contains utility scripts for ARMOR operations and monitoring.

## Version Drift Check

Automated version drift monitoring across ARMOR deployments.

### Files

- `check-version-drift.sh` - **Main orchestrator script** (recommended)
- `version-drift-check.py` - Python pipeline that wires together all components
- `github-release-fetcher.py` - Fetches releases from GitHub API
- `find-armor-deployments.py` - Discovers ARMOR deployments in declarative-config
- `compare-version-drift.py` - Compares deployments against releases
- `armor-version-drift-check.cron` - Cron job definition
- `setup-version-drift-schedule.sh` - Setup script to install cron job

### Usage

Run the check manually:
```bash
./scripts/check-version-drift.sh
```

With custom thresholds:
```bash
./scripts/check-version-drift.sh --releases 5 --days 60
```

Output JSON format:
```bash
./scripts/check-version-drift.sh --json
```

Write report to file:
```bash
./scripts/check-version-drift.sh --output report.json
```

Sort by specific field:
```bash
./scripts/check-version-drift.sh --sort-by cluster
./scripts/check-version-drift.sh --sort-by correctness
```

### Options

- `--releases N` - Flag deployments N or more releases behind (default: 3)
- `--days N` - Flag deployments N or more days behind (default: 30)
- `--json` - Output machine-readable JSON instead of human-readable format
- `--output FILE` - Write report to file (in addition to stdout)
- `--sort-by FIELD` - Sort output by field: cluster, releases, days, correctness (default: correctness)
- `--config FILE` - Use configuration file (JSON)
- `--help` - Show help message

### Exit Codes

- `0` - All deployments within thresholds
- `1` - One or more deployments exceed thresholds
- `2` - Error occurred

### What It Checks

The script scans ARMOR deployments across all clusters in `jedarden/declarative-config` and:
1. **Discovery**: Finds all ARMOR deployment files using `find-armor-deployments.py`
2. **Fetching**: Fetches latest GitHub releases using `github-release-fetcher.py`
3. **Comparison**: Compares deployed versions against releases using `compare-version-drift.py`
4. **Flagging**: Flags deployments needing attention based on thresholds

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

## Version Drift Comparison

Compares deployed ARMOR versions against GitHub releases to detect drift.

### Files

- `compare-version-drift.py` - Main comparison script that integrates outputs from github-release-fetcher.py and find-armor-deployments.py

### Usage

Basic usage with default thresholds:
```bash
python3 scripts/compare-version-drift.py \
  --deployments <path-to-deployments.json> \
  --releases <path-to-releases.json>
```

With custom thresholds:
```bash
python3 scripts/compare-version-drift.py \
  --deployments deployments.json \
  --releases releases.json \
  --releases-threshold 10 \
  --days-threshold 7
```

Output machine-readable JSON:
```bash
python3 scripts/compare-version-drift.py \
  --deployments deployments.json \
  --releases releases.json \
  --json
```

Sort by different fields:
```bash
# Sort by releases behind (most behind first)
python3 scripts/compare-version-drift.py \
  --deployments deployments.json \
  --releases releases.json \
  --sort-by releases

# Sort by correctness drift priority
python3 scripts/compare-version-drift.py \
  --deployments deployments.json \
  --releases releases.json \
  --sort-by correctness
```

### Integration

The comparison script integrates the outputs from both child scripts:

```bash
# Generate both inputs
python3 scripts/github-release-fetcher.py > /tmp/releases.json
python3 scripts/find-armor-deployments.py > /tmp/deployments.json

# Run comparison
python3 scripts/compare-version-drift.py \
  --deployments /tmp/deployments.json \
  --releases /tmp/releases.json
```

### Output Format

The script produces a structured report for each cluster:

**Human-readable output:**
- Shows cluster status with emoji indicators (🔴 drift, 🟡 warning, ✅ OK)
- Displays deployed vs latest versions
- Shows releases behind and days behind counts
- Highlights correctness drift with 🚨 emoji
- Summary statistics

**JSON output:**
```json
{
  "thresholds": {
    "releases": 50,
    "days": 30
  },
  "summary": {
    "total_deployments": 6,
    "with_drift": 3,
    "with_correctness_drift": 2
  },
  "deployments": [
    {
      "cluster": "iad-ci",
      "deployed_tag": "0.1.24",
      "latest_tag": "v0.1.50",
      "releases_behind": 2,
      "days_behind": null,
      "is_drift": true,
      "is_correctness_drift": true,
      "deployed_date": null,
      "latest_date": "2026-07-15T16:00:00Z",
      "filepath": "/path/to/deployment.yml"
    }
  ]
}
```

### Acceptance Criteria

✅ **Configurable thresholds**: Customizable N releases and M days thresholds via CLI arguments
✅ **Structured reports**: Per-cluster deployment comparison with all required metrics
✅ **Correctness priority**: Correctness drift flagged with highest priority (🚨 emoji)
✅ **Machine-readable JSON**: Full JSON output with deployment details and summary statistics

### Exit Codes

- `0` - Success (even if drift is detected)
- `1` - Error (invalid inputs, file parsing errors, etc.)
