# ARMOR Version Drift Check

## Overview

The ARMOR Version Drift Check automatically detects when deployed ARMOR versions fall behind the latest release, with special attention to correctness-labeled releases (bug fixes, security patches).

## Components

### Scripts

- **`scripts/drift_check.py`** - Fleet drift check: live `/version` probing, four-state classification, deduplicated alert-bead filing via `--unique-ref`
- **`scripts/version-drift-check.py`** - Unified wrapper that orchestrates the complete drift check pipeline
- **`scripts/archive/check-armor-version-drift.py`** - Standalone drift check script (legacy, archived)
- **`scripts/github-release-fetcher.py`** - Lists release tags: from the local checkout's `v*` tags when it has any (`--source auto`, the default; run `git fetch --tags` first), otherwise from the GitHub tags API (`--source github`)
- **`scripts/find-armor-deployments.py`** - Scans declarative-config for ARMOR deployments
- **`scripts/compare-version-drift.py`** - Compares deployments against releases to detect drift

### Kubernetes Resources

Both live in declarative-config, where ArgoCD applies them (the copies that
used to sit under this repository's `k8s/` had diverged and were removed on
2026-09-18):

- **`declarative-config/k8s/iad-ci/argo-workflows/armor-drift-check-workflowtemplate.yml`** - Argo WorkflowTemplate for running drift checks
- **`declarative-config/k8s/iad-ci/argo-workflows/armor-drift-check-cronworkflow.yml`** - Scheduled workflow (runs daily at 9 AM UTC)

### Configuration

- **`config/drift-config.json`** - Default configuration for thresholds and settings

## Deployments Monitored

Every `ronaldraygun/armor*` image reference under `declarative-config/k8s/`
is checked (server and restore-verifier images alike); the list is enumerated
by `scripts/find-armor-deployments.py`, never maintained by hand. The clusters
in `config/drift-config.json` (`iad-ci`, `iad-kalshi`, `rs-manager`,
`ord-devimprint`, `apexalgo-iad`, `ardenone-cluster`) are additionally
*expected* to have at least one deployment, so a cluster whose manifests
disappear is reported as `unavailable` rather than silently dropping out.
`iad-native-ads` and `iad-acb` were decommissioned in July 2026 and are no
longer expected.

## Warning Thresholds

Deployments are flagged if they meet any of these criteria:
- **Version drift**: More than 50 versions behind current
- **Time drift**: More than 30 days behind current
- **Correctness releases**: Any bug fix or security release was missed
- **Non-version tags**: Using git SHA instead of version tag

## Deployment States

`scripts/drift_check.py` classifies every deployment into exactly one state:

- **`current`** - deployed tag matches the latest approved release, or is close enough that no threshold above is exceeded (routine version bumps are not drift)
- **`stale`** - parseable version behind latest beyond a threshold, or missing a correctness-labelled release
- **`mismatched`** - the deployed image is not the approved release in a way version math cannot express: a non-version tag (git SHA, `latest`), or a live `/version` probe disagreeing with the declared manifest tag
- **`unavailable`** - the deployment could not be verified at all: the probe failed, no releases were available for comparison, or a configured cluster has no ARMOR manifest in declarative-config

Digest-pinned tags (`0.1.1957@sha256:…`, the manifest convention) are read as
their version, so pinning is never drift on its own; likewise a `/version`
probe reporting `0.1.1957` against a `0.1.1957@sha256:…` manifest is the same
release, not a mismatch. Alert dedup is also version-normalized: a
digest-only rebuild of the same stale version keeps the fingerprint and does
not refile.

## Usage

### Manual Execution

#### drift_check.py (recommended)

```bash
# Classify the fleet against the release tags (local checkout first, GitHub otherwise)
python3 scripts/drift_check.py

# Assert the newest release when tags lag VERSION (e.g. CI has not tagged yet)
python3 scripts/drift_check.py --latest-tag v0.1.1970

# Machine-readable JSON (adds per-deployment state and the drift fingerprint)
python3 scripts/drift_check.py --json

# Probe running versions live and compare against the declared manifest tag
python3 scripts/drift_check.py --probe-url iad-kalshi=https://armor-iad-kalshi.example.com/version

# Treat a configured cluster with no ARMOR manifest as unavailable
python3 scripts/drift_check.py --expected-cluster iad-acb

# File ONE deduplicated alert bead when anything is non-current
python3 scripts/drift_check.py --emit-bead            # files via `bead create --unique-ref drift-check:<fingerprint>`
python3 scripts/drift_check.py --emit-bead --dry-run  # print the bead command instead of running it
```

Alerting deduplicates on a fingerprint: a stable hash of the non-current subset
(cluster, image type, state, deployed tag, latest tag). An unchanged drift
picture replays to `EXISTING` on the `--unique-ref` and files nothing; a changed
fleet picture hashes differently and files fresh. Exit codes: `0` all current,
`1` drift present, `2` error.

#### version-drift-check.py and older scripts

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

Or use the archived legacy script:
```bash
# Run the check
python3 scripts/archive/check-armor-version-drift.py

# Get JSON output for integration
python3 scripts/archive/check-armor-version-drift.py --json
```

### Scheduled Execution

#### Option 1: Argo Workflow (Recommended)

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

#### Option 2: systemd Timer (Recommended for NixOS)

This system (NixOS) may not have traditional cron available. Create a systemd user service and timer:

```bash
# Create the service file
~/.config/systemd/user/armor-version-drift-check.service
```

```ini
[Unit]
Description=ARMOR Version Drift Check
After=network.target

[Service]
Type=oneshot
WorkingDirectory=/home/coding/ARMOR
ExecStart=/usr/bin/env python3 /home/coding/ARMOR/scripts/drift_check.py
StandardOutput=append:/home/coding/ARMOR/logs/version-drift-check.log
StandardError=append:/home/coding/ARMOR/logs/version-drift-check.log
```

```bash
# Create the timer file
~/.config/systemd/user/armor-version-drift-check.timer
```

```ini
[Unit]
Description=ARMOR Version Drift Check (Daily)
Requires=armor-version-drift-check.service

[Timer]
OnCalendar=daily
OnCalendar=09:17
RandomizedDelaySec=600

[Install]
WantedBy=timers.target
```

Enable the timer:
```bash
systemctl --user daemon-reload
systemctl --user enable armor-version-drift-check.timer
systemctl --user start armor-version-drift-check.timer
```

#### Option 3: Claude Code Loop

Use the Claude Code /loop skill:

```
/loop 1d python3 scripts/drift_check.py
```

This runs the check daily within the Claude Code session.

#### Option 4: Traditional Cron

Run the setup script to add a daily cron job:

```bash
./scripts/setup-version-drift-schedule.sh
```

This schedules the check to run daily at 9:17 AM (avoiding :00 and :30 marks to reduce API load).

Or manually configure the schedule:

```bash
# Edit crontab
crontab -e

# Add this line (runs daily at 9:17 AM)
17 9 * * * cd /home/coding/ARMOR && python3 scripts/drift_check.py >> /home/coding/ARMOR/logs/version-drift-check.log 2>&1
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

## Version Probe

ARMOR exposes its version through two mechanisms for drift detection:

### HTTP Header Probe

Every ARMOR response includes a `Server: ARMOR/<version>` header that can be used as a lightweight drift probe:

```bash
# Check deployed ARMOR version via Server header
curl -sI https://armor.example.com/healthz | grep Server:
# Output: Server: ARMOR/0.1.42

# Works on both S3 API and admin API listeners
curl -sI https://armor.example.com/healthz | grep Server:
curl -sI https://armor-admin.example.com/healthz | grep Server:
```

This is the recommended drift check method:
- No authentication required (works on public /healthz endpoint)
- Returns only via HEAD request (minimal bandwidth)
- Available on all ARMOR responses (no special endpoint needed)

### JSON Version Endpoint

For detailed version information, ARMOR provides a `GET /version` endpoint on both listeners:

```bash
curl -s https://armor.example.com/version
# Output: {"version":"0.1.42","format_write_version":2,"go":"1.23.1"}

curl -s https://armor-admin.example.com/version
# Output: {"version":"0.1.42","format_write_version":2,"go":"1.23.1"}
```

The endpoint returns:
- `version`: ARMOR version (set at build time via ldflags)
- `format_write_version`: ARMOR manifest format version (from ARMOR_FORMAT_VERSION env var)
- `go`: Go runtime version (with "go" prefix stripped for cleaner JSON)

**Note:** The `/version` endpoint requires no authentication and is safe to call from monitoring systems.

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

================================================================================
SUMMARY
Total deployments checked: 6
Deployments needing update: 2
Using non-version tags: 0
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

## Exit Codes

`scripts/drift_check.py`:

- **0**: every deployment classified `current`
- **1**: any deployment classified `stale`, `mismatched`, or `unavailable`
- **2**: script error (bad arguments, missing input, fetcher/enumeration failure)

Older scripts (`version-drift-check.py`, `compare-version-drift.py`, the archived legacy script):

- **0**: No drift or only routine version bumps
- **1**: Correctness drift detected (missing bug/security fixes)
- **2**: Script error (failed to run)

## Correctness-Labeled Releases

Releases are flagged as correctness-related if the release notes or tag contain keywords:
- fix, bug, security, correctness, critical, patch, hotfix, urgent, vulnerability, cve, issue, regression

These releases get highest priority in the report and trigger exit code 1 when drift is detected.

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
    ["python3", "scripts/drift_check.py", "--json"],
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
    ["python3", "scripts/drift_check.py", "--json"],
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

## Log Location

All methods write logs to: `/home/coding/ARMOR/logs/version-drift-check.log`

## Maintenance

To update the list of monitored deployments:
1. Deployments are discovered automatically by `scripts/find-armor-deployments.py` — add the manifest to `declarative-config`
2. Add any cluster that must always report (even with no ARMOR manifest) to the `clusters` list in `config/drift-config.json`
3. Test the changes: `python3 scripts/drift_check.py --json`

## Testing

Regardless of scheduling method, test first:

```bash
python3 -m pytest tests/test_drift_check.py -q   # drift_check.py test suite
python3 scripts/drift_check.py --json            # live classification pass
```
