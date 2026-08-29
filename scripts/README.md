# ARMOR Scripts

This directory contains operational, testing, and monitoring scripts for ARMOR deployments.

## Version Drift Monitoring

Automated version drift monitoring across ARMOR deployments.

**Full documentation:** [docs/drift-check.md](../docs/drift-check.md)

### check-version-drift.sh

Main orchestrator script for version drift monitoring.

**Purpose:** Scans ARMOR deployments across all clusters, compares against GitHub releases, and flags deployments needing attention.

**Inputs:** None (reads from declarative-config and GitHub)

**Options:**
- `--releases N` - Flag deployments N or more releases behind (default: 3)
- `--days N` - Flag deployments N or more days behind (default: 30)
- `--json` - Output machine-readable JSON
- `--output FILE` - Write report to file
- `--sort-by FIELD` - Sort by: cluster, releases, days, correctness (default: correctness)
- `--config FILE` - Use configuration file (JSON)

**Examples:**
```bash
# Basic check with default thresholds
./scripts/check-version-drift.sh

# Custom thresholds
./scripts/check-version-drift.sh --releases 5 --days 60

# JSON output for automation
./scripts/check-version-drift.sh --json --output report.json

# Sort by cluster name
./scripts/check-version-drift.sh --sort-by cluster
```

**Exit codes:** 0 (all OK), 1 (drift detected), 2 (error)

---

### version-drift-check.py

Python pipeline that wires together all version drift components.

**Purpose:** Integrates github-release-fetcher, find-armor-deployments, and compare-version-drift into a single pipeline.

**Inputs:** Command-line arguments for thresholds and endpoints

**Examples:**
```bash
# Run with default settings
python3 scripts/version-drift-check.py

# Custom thresholds
python3 scripts/version-drift-check.py --releases-threshold 5 --days-threshold 60
```

---

### find-armor-deployments.py

Discovers ARMOR deployments in declarative-config.

**Purpose:** Scans jedarden/declarative-config for ARMOR deployment files and extracts image tags.

**Inputs:** Path to declarative-config (defaults to ../declarative-config)

**Outputs:** JSON with deployment metadata per cluster

**Examples:**
```bash
# Scan default location
python3 scripts/find-armor-deployments.py

# Scan specific path
python3 scripts/find-armor-deployments.py --path /path/to/declarative-config
```

---

### compare-version-drift.py

Compares deployments against releases to detect drift.

**Purpose:** Takes deployment and release JSON, calculates drift metrics, and flags outdated deployments.

**Inputs:**
- `--deployments FILE` - Output from find-armor-deployments.py
- `--releases FILE` - Output from github-release-fetcher.py
- `--releases-threshold N` - Flag N+ releases behind (default: 50)
- `--days-threshold N` - Flag N+ days behind (default: 30)

**Outputs:** Human-readable report or JSON with per-cluster drift analysis

**Examples:**
```bash
# Generate drift report
python3 scripts/compare-version-drift.py \
  --deployments deployments.json \
  --releases releases.json

# JSON output with custom thresholds
python3 scripts/compare-version-drift.py \
  --deployments deployments.json \
  --releases releases.json \
  --releases-threshold 10 \
  --days-threshold 7 \
  --json
```

---

### github-release-fetcher.py

Fetches ARMOR releases from GitHub API.

**Purpose:** Retrieves release history from GitHub and classifies correctness/security releases.

**Inputs:** GitHub repository (hardcoded to jedarden/ARMOR)

**Outputs:** JSON array with release metadata (tag, published_at, is_correctness, url)

**Examples:**
```bash
python3 scripts/github-release-fetcher.py
```

**Correctness keywords:** correctness, fix, critical, security, bug, patch, hotfix, urgent, vulnerability, cve, issue, regression

---

### armor-version-drift-check.cron

Cron job definition for scheduled drift monitoring.

**Purpose:** Runs daily version drift checks at 9:17 AM (avoids :00/:00 load spikes).

**Setup:** Use setup-version-drift-schedule.sh to install

---

### setup-version-drift-schedule.sh

Installs the cron job for automated drift monitoring.

**Purpose:** Sets up scheduled daily checks via cron.

**Inputs:** None

**Examples:**
```bash
bash scripts/setup-version-drift-schedule.sh
```

**Logs:** Written to logs/version-drift-check.log

---

## Testing & Validation

### test-armor-endpoints.sh

Tests all ARMOR endpoints for expected responses and status codes.

**Purpose:** Verifies health, readiness, and S3 endpoints return correct status codes.

**Inputs:** Environment variables for endpoint configuration
- `NAMESPACE` - Kubernetes namespace (default: armor)
- `SERVICE` - Kubernetes service name (default: armor)
- `S3_PORT` - S3 API port (default: 9000)
- `ADMIN_PORT` - Admin API port (default: 9001)
- `TIMEOUT` - Request timeout in seconds (default: 5)

**Examples:**
```bash
# Test default configuration
./scripts/test-armor-endpoints.sh

# Test specific namespace
NAMESPACE=armor-production ./scripts/test-armor-endpoints.sh

# Test with custom timeout
TIMEOUT=10 ./scripts/test-armor-endpoints.sh
```

---

### test_filesystem_backend.sh

Verifies filesystem backend works with aws-cli.

**Purpose:** Full integration test of filesystem backend including PUT/GET/range/multipart/DELETE cycle.

**Inputs:** Environment variables
- `ARMOR_BINARY` - Path to armor binary (default: ./armor)
- Auto-generated test data and directories

**Examples:**
```bash
# Test with local armor binary
ARMOR_BINARY=./build/armor ./scripts/test_filesystem_backend.sh

# Test with default binary
./scripts/test_filesystem_backend.sh
```

**Acceptance:** ARMOR_BACKEND=filesystem ARMOR_FS_PATH=/tmp/x ARMOR_MEK=... ARMOR_AUTH_ACCESS_KEY=... ARMOR_AUTH_SECRET_KEY=... armor serve starts and passes full S3 cycle

---

### test_invalid_credential_rejection.py

Verifies ARMOR properly rejects invalid credentials.

**Purpose:** Tests that invalid AWS credentials return 403 Forbidden with meaningful error messages, and rejection happens quickly (no long timeouts).

**Inputs:** Environment variables
- `ARMOR_ENDPOINT` - ARMOR server URL (default: http://localhost:9000)
- `ARMOR_BUCKET` - Test bucket name

**Examples:**
```bash
# Test default endpoint
python3 scripts/test_invalid_credential_rejection.py

# Test specific endpoint
ARMOR_ENDPOINT=http://localhost:9000 python3 scripts/test_invalid_credential_rejection.py
```

**Test cases:** Invalid credentials, malformed signatures, missing auth headers

---

### test_s3_auth_acceptance.py

S3 authentication acceptance test suite.

**Purpose:** Verifies valid AWS Signature V4 authentication is accepted (ARMOR implements V4 only, not deprecated V2).

**Inputs:** Environment variables
- `ARMOR_ENDPOINT` - ARMOR server URL (default: http://localhost:9000)
- `ARMOR_ACCESS_KEY` - Valid access key
- `ARMOR_SECRET_KEY` - Valid secret key
- `ARMOR_BUCKET` - Test bucket name
- `ARMOR_REGION` - AWS region (default: us-east-1)

**Examples:**
```bash
# Test with credentials
ARMOR_ACCESS_KEY=testkey ARMOR_SECRET_KEY=testsecret \
python3 scripts/test_s3_auth_acceptance.py

# Test with full configuration
ARMOR_ENDPOINT=http://localhost:9000 \
ARMOR_ACCESS_KEY=testkey \
ARMOR_SECRET_KEY=testsecret \
ARMOR_BUCKET=test-bucket \
ARMOR_REGION=us-east-1 \
python3 scripts/test_s3_auth_acceptance.py
```

**Note:** ARMOR correctly implements AWS Signature V4 only (not V2) for security. AWS deprecated V2 in 2019.

---

### validate-deployment.sh

Performs 50MB multipart round-trip validation against deployed ARMOR instances.

**Purpose:** Full deployment validation test including multipart upload, download, SHA-256 verification, and metadata checks.

**Inputs:** Positional arguments
1. Cluster name
2. ARMOR endpoint URL
3. S3 bucket name
4. Scratch prefix (optional, default: armor-validation-scratch)

**Environment variables:**
- `ARMOR_AUTH_ACCESS_KEY` - Required authentication key
- `ARMOR_AUTH_SECRET_KEY` - Required secret key

**Examples:**
```bash
# Validate rs-manager deployment
./scripts/validate-deployment.sh \
  rs-manager \
  http://localhost:9000 \
  rs-manager

# Validate with custom scratch prefix
ARMOR_AUTH_ACCESS_KEY=key ARMOR_AUTH_SECRET_KEY=secret \
./scripts/validate-deployment.sh \
  iad-ci \
  http://armor.iad-ci:9000 \
  armor-bucket \
  test-prefix
```

**Tests:** Sequential multipart upload (required by ARMOR), download and SHA-256 verification, object metadata check, cleanup

---

## Deployment & Operations

### verify-cloudflare-setup.sh

Verifies Cloudflare B2 proxy setup for ARMOR.

**Purpose:** Checks DNS resolution, Cloudflare IPs, SSL configuration, and B2 backend connectivity.

**Inputs:** Positional arguments
1. Cloudflare domain
2. B2 bucket name

**Examples:**
```bash
# Verify Cloudflare setup
./scripts/verify-cloudflare-setup.sh armor-b2.example.com my-armor-bucket

# Verify production setup
./scripts/verify-cloudflare-setup.sh armor.ardenone.com armor-prod
```

**Checks:** DNS resolution, Cloudflare IP ownership, SSL certificate, B2 bucket accessibility

---

### cleanup-restore-env.sh

Clean restore environment while preserving logs.

**Purpose:** Archives current logs and cleans working directories for Litestream restore environment.

**Inputs:** Positional argument
1. Restore environment path (optional, default: /home/coding/ARMOR/scratch/litestream-restore)

**Examples:**
```bash
# Clean default location
./scripts/cleanup-restore-env.sh

# Clean specific location
./scripts/cleanup-restore-env.sh /path/to/restore-env

# Show help
./scripts/cleanup-restore-env.sh --help
```

**Operations:** Archives log files, cleans databases and temp directories, logs cleanup action

---

### reset-restore-env.sh

Complete restore environment reset.

**Purpose:** Removes and recreates Litestream restore environment directory structure.

**Inputs:** Positional argument
1. Restore environment path (optional, default: /home/coding/ARMOR/scratch/litestream-restore)

**Examples:**
```bash
# Reset default location
./scripts/reset-restore-env.sh

# Reset specific location
./scripts/reset-restore-env.sh /path/to/restore-env

# Show help
./scripts/reset-restore-env.sh --help
```

**Operations:** Backs up existing logs, recreates directory structure (databases, logs, restored, temp), sets permissions

---

## Development & Debugging

### probe-armor-multipart-alignment.py

Validates ARMOR's part-size alignment requirements for multipart uploads.

**Purpose:** Reproducible test of ADR-005 exemptions for lone parts and final parts, including exact GET/HEAD readback without zero-padding.

**Full documentation:** [probe-armor-multipart-alignment.README.md](probe-armor-multipart-alignment.README.md)

**Inputs:** Environment variables
- `ARMOR_BUCKET` - Required: Target bucket
- `ARMOR_ENDPOINT_URL` - Optional: ARMOR endpoint (defaults to AWS S3)
- `AWS_ACCESS_KEY_ID` - Optional: AWS access key
- `AWS_SECRET_ACCESS_KEY` - Optional: AWS secret key

**Examples:**
```bash
# Test against local ARMOR
export ARMOR_BUCKET=devimprint
export ARMOR_ENDPOINT_URL=http://127.0.0.1:9000
python3 scripts/probe-armor-multipart-alignment.py

# Test against AWS S3
export ARMOR_BUCKET=my-test-bucket
python3 scripts/probe-armor-multipart-alignment.py

# Test with explicit credentials
export ARMOR_BUCKET=devimprint
export ARMOR_ENDPOINT_URL=http://127.0.0.1:9000
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
python3 scripts/probe-armor-multipart-alignment.py
```

**Test cases:** Single misaligned part, single aligned part, aligned first + short final, misaligned first + short final, exactly-one-block part, zero-byte final part, PutObject misaligned length

**Prerequisites:** Python 3 with boto3, AWS credentials configured

---

### verify-multipart-integrity.py

Verification script for multipart-era corruption audit.

**Purpose:** Tests restore/decrypt operations on candidate objects to detect corruption from the multipart alignment bug era.

**Inputs:** Positional arguments
1. candidates.json - Output from cross-reference-affected-objects.py
2. output.json - Optional output file for verification results

**Environment variables:**
- `ARMOR_DECRYPT_PATH` - Path to armor-decrypt binary (default: armor-decrypt from PATH)

**Examples:**
```bash
# Verify candidates
python3 scripts/verify-multipart-integrity.py candidates.json

# Verify with output file
python3 scripts/verify-multipart-integrity.py candidates.json results.json

# Use custom decrypt binary
ARMOR_DECRYPT_PATH=/path/to/armor-decrypt \
python3 scripts/verify-multipart-integrity.py candidates.json
```

**Outputs:** JSON with verification status (VERIFIED, CORRUPTED, FAILED) per object

**Note:** cross-reference-affected-objects.py has been archived to scripts/archive/

---

## Archived Scripts

One-off investigation scripts and superseded tools are archived in `scripts/archive/`:

**Enumeration scripts:**
- enumerate-all-unaudited-buckets.py
- enumerate-armor-apexalgo-bucket.py
- enumerate-large-objects-http.py
- enumerate-large-objects.py
- enumerate-rs-manager-bucket.py
- enumerate-unaudited-buckets.py

**Filtering and analysis:**
- filter-affected-objects.py
- filter-candidate-objects.py
- cross-reference-affected-objects.py
- generate-filtered-output.py
- parse-deployment-windows.py
- corruption-audit-framework.py

**Verification:**
- verify-affected-objects.py
- verify_candidate_objects.sh

**Authentication testing:**
- test_auth.py
- test_auth_v4.py
- test_auth.sh

**Superseded tools:**
- discover_armor_deployments.py (duplicate of find-armor-deployments.py)
- check-armor-version-drift.py (duplicate of version-drift-check.py)
- armor-connectivity-check.sh (replaced by `armor check`)

These scripts are preserved for reference but are not part of the operational toolkit.
