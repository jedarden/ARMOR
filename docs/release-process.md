# ARMOR Release Process

This document covers ARMOR release processes, with special emphasis on **correctness fix propagation** — ensuring that data-integrity and correctness fixes reach every ARMOR deployment.

## Table of Contents

1. [Correctness Fix Propagation Checklist](#correctness-fix-propagation-checklist)
2. [ARMOR Deployment Inventory](#armor-deployment-inventory)
3. [Verification Procedures](#verification-procedures)
4. [Tracking Pending Deployments](#tracking-pending-deployments)
5. [Release Classification](#release-classification)

---

## Correctness Fix Propagation Checklist

### Definition: Correctness/Data-Integrity Fix

A **correctness fix** is any commit that addresses:
- **Data corruption bugs** — fixes that prevent or recover from corrupted object data
- **Encryption/decryption bugs** — fixes to envelope encryption, DEK wrapping, HMAC verification
- **Metadata handling bugs** — fixes to ARMOR metadata headers (`x-amz-meta-armor-*`)
- **Multipart upload integrity bugs** — fixes to multipart HMAC tables, state management
- **Authentication/authorization bugs** — fixes that expose data to unauthorized parties
- **Race conditions** — fixes that cause data inconsistency under concurrent operations
- **Critical security vulnerabilities** — CVE-level issues that affect data confidentiality

**Non-correctness fixes** (exempt from full propagation):
- UI/dashboard changes
- Logging/metrics improvements
- Performance optimizations that don't affect correctness
- Documentation updates
- Test infrastructure changes

### The Golden Rule

> **A correctness fix is NOT resolved until every known ARMOR deployment is patched or explicitly tracked as pending.**
>
> Merging to `main` is necessary but NOT sufficient. The fix must be propagated to all deployments.
>
> **Before closing any bead that fixes a correctness/data-integrity bug, you MUST:**
> 1. Enumerate every known ARMOR deployment
> 2. Confirm each deployment is patched OR explicitly track it as pending with a linked follow-up bead
> 3. Document the propagation status in the bead closing comment

**Failure to follow this process leaves deployments vulnerable to known bugs.**

### Propagation Checklist (Mandatory Before Closing Correctness Fix Beads)

**⚠️ DO NOT CLOSE a correctness/data-integrity fix bead until this checklist is complete.**

#### Phase 1: Pre-Merge (Before Merging to Main)

- [ ] **1.1 Confirm this is a correctness fix** — Verify the fix addresses data corruption, encryption bugs, race conditions, auth issues, or S3 contract violations (see classification below)
- [ ] **1.2 Identify scope of impact** — Determine which deployments are affected by the bug (all, or subset?)
- [ ] **1.3 Check deployment inventory** — Review `~/declarative-config` for all ARMOR deployments

#### Phase 2: Post-Merge (Before Closing Bead)

- [ ] **2.1 Enumerate all deployments** — List every known ARMOR deployment from the inventory below
- [ ] **2.2 Check each deployment's version** — For each deployment, run `kubectl` to check current ARMOR version
- [ ] **2.3 Apply fix to each deployment** — Update image tags in `declarative-config` or roll out manually
- [ ] **2.4 Verify each deployment is patched** — Confirm pods are running the new image
- [ ] **2.5 Run health checks** — Verify `/armor/canary` returns healthy on each deployment
- [ ] **2.6 Document propagation status** — Create a propagation table (see Step 5)
- [ ] **2.7 Create follow-up beads for pending deployments** — If any deployment cannot be patched immediately, create a tracking bead

#### Phase 3: Final Resolution

- [ ] **3.1 Confirm all deployments are either:**
  - ✅ **Patched and verified**, OR
  - ⏳ **Explicitly tracked as pending** with a linked follow-up bead
- [ ] **3.2 Update bead with propagation table** — Add the propagation table to the bead closing comment
- [ ] **3.3 Close the bead** — Only close after ALL deployments meet Phase 3.1 criteria

#### Step 1: Enumerate All Deployments

List every known ARMOR deployment from the deployment inventory below:

```markdown
- [ ] iad-ci (CI/CD cluster)
- [ ] iad-kalshi (Rackspace Spot cluster)
- [ ] iad-native-ads (Rackspace Spot cluster)
- [ ] rs-manager (management cluster)
- [ ] ord-devimprint (DevImprint cluster)
- [ ] iad-acb (AI Code Battle)
- [ ] apexalgo-iad (AI Code Battle - legacy)
- [ ] ardenone-cluster (if applicable)
- [ ] ardenone-hub (if applicable)
- [ ] iad-options (if applicable)
- [ ] Any external deployments (documented separately)
```

**To refresh this list:**
```bash
cd ~/declarative-config
find . -name "*armor*" -type f | grep -E "(deployment|workflow)" | grep -v "argo-workflows"
```

#### Step 2: Check Each Deployment's Current Version

For each deployment, verify the running ARMOR version:

```bash
# Via kubectl (adjust for each cluster's kubeconfig/proxy)
kubectl --server=http://traefik-<cluster>:8001 get deployment -n <namespace> armor -o jsonpath='{.spec.template.spec.containers[0].image}'

# Example: iad-kalshi
kubectl --server=http://traefik-iad-kalshi:8001 get deployment -n armor armor -o jsonpath='{.spec.template.spec.containers[0].image}'
```

**Expected output format:** `ronaldraygun/armor:<version>`

#### Step 3: Apply Fix to Each Deployment

For each deployment that is NOT running the fixed version:

**Option A: Via ArgoCD (preferred for GitOps-managed deployments)**

1. Update the image tag in `declarative-config`:
   ```bash
   cd ~/declarative-config
   # Find the deployment file
   find . -name "*armor-deployment*.yml" -o -name "*armor-deployment*.yaml"
   
   # Edit to update image tag
   # Example: ronaldraygun/armor:0.1.43 → ronaldraygun/armor:0.1.44
   ```

2. Commit and push to declarative-config:
   ```bash
   git add k8s/<cluster>/<namespace>/armor-deployment.yml
   git commit -m "chore(armor): bump to v0.1.44 for correctness fix"
   git push
   ```

3. Verify ArgoCD syncs the change:
   ```bash
   curl -sk https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications | \
     jq -r '.items[] | select(.metadata.name | contains("armor")) | .metadata.name + ": " + .status.syncStatus'
   ```

**Option B: Manual rollout (if ArgoCD is not managing the deployment)**

```bash
# Via direct kubeconfig (for clusters with write access)
kubectl --kubeconfig=/home/coding/.kube/<cluster>.kubeconfig \
  set image deployment/armor armor=ronaldraygun/armor:<new-version> -n <namespace>

# Via kubectl-proxy read-only + separate write kubeconfig
kubectl --kubeconfig=/home/coding/.kube/<cluster>.kubeconfig \
  rollout restart deployment/armor -n <namespace>
```

#### Step 4: Verify the Fix is Live

For each deployment, run health checks to confirm the fix is active:

```bash
# 1. Check deployment is healthy
kubectl --server=http://traefik-<cluster>:8001 get deployment -n <namespace> armor

# 2. Check pods are running the new image
kubectl --server=http://traefik-<cluster>:8001 get pods -n <namespace> -l app=armor -o jsonpath='{.items[*].spec.containers[0].image}'

# 3. Run canary health check
kubectl --server=http://traefik-<cluster>:8001 exec -n <namespace> deployment/armor -- \
  curl -s http://localhost:9001/armor/canary | jq .

# 4. Verify MEK is correct
kubectl --server=http://traefik-<cluster>:8001 exec -n <namespace> deployment/armor -- \
  curl -s http://localhost:9001/admin/key/verify | jq .
```

**Expected outputs:**
- Deployment: `1/1` replicas ready
- Canary: `{"status": "healthy"}`
- MEK verify: `{"status": "verified"}`

#### Step 5: Document Deployment Status

Create a comment in the closing bead/issue with a propagation table:

```markdown
## Fix Propagation Status

| Cluster | Namespace | Previous Version | New Version | Status | Verified At |
|---------|-----------|------------------|-------------|--------|-------------|
| iad-ci | armor | 0.1.43 | 0.1.44 | ✅ Synced | 2026-07-14T14:30:00Z |
| iad-kalshi | armor | 0.1.43 | 0.1.44 | ✅ Synced | 2026-07-14T14:32:00Z |
| iad-native-ads | armor | 0.1.42 | 0.1.44 | 🔄 Pending | - |
| rs-manager | armor | 0.1.43 | 0.1.44 | ⏸️ Blocked (needs approval) | - |
| ord-devimprint | devimprint | 0.1.43 | 0.1.44 | ✅ Synced | 2026-07-14T14:35:00Z |
| iad-acb | ai-code-battle | 0.1.43 | 0.1.44 | ⚠️ Manual rollout required | - |
```

**Status codes:**
- ✅ Synced: Deployed and verified
- 🔄 Pending: In progress via ArgoCD
- ⏸️ Blocked: Requires approval/action
- ⚠️ Manual: Requires manual intervention
- ❌ Failed: Rollback required

#### Step 6: Track Pending Deployments

For any deployment that CANNOT be immediately patched:

1. Create a tracking bead or issue:
   ```bash
   br create --type bug \
     --title "Propagate ARMOR v0.1.44 to <cluster>" \
     --acceptance "Deployment runs ARMOR v0.1.44"
   ```

2. Link it to the original fix bead as a related issue.

3. Add the pending deployment to the `docs/release-process.md` tracking table (see [Tracking Pending Deployments](#tracking-pending-deployments)).

#### Step 7: Close the Fix Bead

Only close the fix bead when **ALL** deployments meet one of:
- ✅ Patched and verified
- 🔄 Explicitly tracked with a linked follow-up bead

**Do NOT close the bead if any deployment is in unknown state.**

---

## ARMOR Deployment Inventory

### Known Deployments (as of 2026-07-14)

| Cluster | Namespace | ArgoCD App | Image Source | Access Method | Notes |
|---------|-----------|------------|---------------|---------------|-------|
| **iad-ci** | `armor` | `armor-ns-iad-ci` | `ronaldraygun/armor` | Direct kubeconfig | CI/CD cluster, builds armor image |
| **iad-kalshi** | `armor` | `armor-iad-kalshi` | `ronaldraygun/armor` | kubectl-proxy | Rackspace Spot, hosts kalshi-weather |
| **iad-native-ads** | `armor` | `armor-iad-native-ads` | `ronaldraygun/armor` | kubectl-proxy | Rackspace Spot, native ads pipeline |
| **rs-manager** | `armor` | `armor-rs-manager` | `ronaldraygun/armor` | kubectl-proxy | Management cluster |
| **ord-devimprint** | `devimprint` | N/A (manual) | `ronaldraygun/armor` | kubectl-proxy | DevImprint production |
| **iad-acb** | `ai-code-battle` | `acb-armor-iad-acb` | `ronaldraygun/armor` | kubectl-proxy | AI Code Battle |
| **apexalgo-iad** | `ai-code-battle` | N/A (legacy) | `ronaldraygun/armor` | kubectl-proxy | Legacy ACB deployment |
| **ardenone-cluster** | TBD | TBD | TBD | kubectl-proxy | Check if ARMOR is deployed here |
| **ardenone-hub** | TBD | TBD | TBD | kubectl-proxy | Check if ARMOR is deployed here |
| **iad-options** | TBD | TBD | TBD | kubectl-proxy | Check if ARMOR is deployed here |

### Access Patterns

**kubectl-proxy (read-only):**
```bash
kubectl --server=http://traefik-<cluster>:8001 get pods -n <namespace>
```

**Direct kubeconfig (read/write):**
```bash
kubectl --kubeconfig=/home/coding/.kube/<cluster>.kubeconfig get pods -n <namespace>
```

**ArgoCD API (read-only):**
```bash
curl -sk https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications/<app-name>
```

### Deployment Config Locations

All ARMOR deployment configurations live in `~/declarative-config/k8s/`:

```
declarative-config/
├── k8s/
│   ├── iad-ci/armor/
│   │   ├── armor-deployment.yaml
│   │   ├── armor-externalsecret.yaml
│   │   └── armor-configmap.yaml
│   ├── iad-kalshi/armor/
│   │   ├── armor-deployment.yml
│   │   └── armor-externalsecret.yml
│   ├── iad-native-ads/armor/
│   │   ├── armor-deployment.yml
│   │   └── armor-externalsecret.yml
│   ├── rs-manager/armor/
│   │   ├── armor-deployment.yml
│   │   └── armor-externalsecret.yml
│   ├── ord-devimprint/devimprint/
│   │   └── armor-deployment.yml
│   └── iad-acb/ai-code-battle/
│       ├── acb-armor-deployment.yml
│       └── acb-armor-externalsecret.yml
```

To update a deployment, edit the `armor-deployment.yml` file and push to `declarative-config`. ArgoCD will sync automatically.

---

## Verification Procedures

### Quick Health Check

Run this on each deployment after a fix rollout:

```bash
#!/bin/bash
CLUSTER=$1
NAMESPACE=$2

echo "Checking ARMOR health in ${CLUSTER}/${NAMESPACE}..."

# 1. Deployment health
kubectl --server=http://traefik-${CLUSTER}:8001 get deployment -n ${NAMESPACE} armor -o json | jq -r '
  "Replicas: " + 
  (.spec.replicas | tostring) + 
  ", Ready: " + 
  (.status.readyReplicas | tostring)
'

# 2. Pod image versions
kubectl --server=http://traefik-${CLUSTER}:8001 get pods -n ${NAMESPACE} -l app=armor -o json | jq -r '
  .items[].spec.containers[].image
'

# 3. Canary health
kubectl --server=http://traefik-${CLUSTER}:8001 exec -n ${NAMESPACE} deployment/armor -- \
  curl -s http://localhost:9001/armor/canary | jq .

# 4. MEK verification
kubectl --server=http://traefik-${CLUSTER}:8001 exec -n ${NAMESPACE} deployment/armor -- \
  curl -s http://localhost:9001/admin/key/verify | jq .
```

Usage:
```bash
# Check iad-kalshi
./scripts/check-armor-health.sh iad-kalshi armor

# Check iad-native-ads
./scripts/check-armor-health.sh iad-native-ads armor
```

### Correctness-Specific Verification

For encryption/decryption fixes, verify end-to-end:

```bash
#!/bin/bash
CLUSTER=$1
NAMESPACE=$2

# Port-forward to local
kubectl --server=http://traefik-${CLUSTER}:8001 port-forward -n ${NAMESPACE} svc/armor 9000:9000 &
PF_PID=$!
sleep 2

# Test upload
echo "test data" | aws s3 cp --endpoint-url http://localhost:9000 - s3://test-bucket/verify-test.txt

# Test download
DOWNLOADED=$(aws s3 cp --endpoint-url http://localhost:9000 s3://test-bucket/verify-test.txt -)

# Verify
if [ "$DOWNLOADED" = "test data" ]; then
  echo "✅ Encryption/decryption verified"
else
  echo "❌ Encryption/decryption FAILED"
fi

# Cleanup
kill $PF_PID
```

---

## Tracking Pending Deployments

### Pending Deployments Table

Maintain this table in `docs/release-process.md` for outstanding propagations:

| Fix Version | Fix Bead | Deployment | Pending Since | Blocked By | Follow-up Bead |
|-------------|----------|------------|---------------|------------|----------------|
| 0.1.44 | bf-xxxx | iad-native-ads | 2026-07-14 | Needs approval | bf-yyyy |
| 0.1.43 | bf-zzzz | ord-devimprint | 2026-07-10 | Testing | bf-aaaa |

Update this table when:
1. A correctness fix is merged to main (add row for each unpatched deployment)
2. A deployment is patched (remove row or mark as ✅)
3. A follow-up bead is created (link it)

### Audit Procedure

Run this weekly to catch missing propagations:

```bash
#!/bin/bash
# Check for version drift across ARMOR deployments

echo "Checking ARMOR version drift..."
echo ""

# List all deployments and their versions
for cluster in iad-ci iad-kalshi iad-native-ads rs-manager ord-devimprint; do
  echo "=== $cluster ==="
  kubectl --server=http://traefik-${cluster}:8001 get deployment -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.template.spec.containers[0].image}{"\n"}{end}' | grep armor
  echo ""
done

# Find latest version from git
LATEST=$(cd ~/ARMOR && git tag -l "armor-v*" | sort -V | tail -1 | sed 's/armor-v//')
echo "Latest release: $LATEST"
```

If any deployment is running a version older than the latest release that contains a correctness fix, create a propagation bead.

---

## Release Classification

### Release Categories

| Category | Criteria | Propagation Required? | Timeline |
|----------|----------|----------------------|----------|
| **Critical Correctness** | Data corruption, encryption bugs, security vulns | Yes, all deployments immediately | Within 24h |
| **High Correctness** | Race conditions, metadata bugs, auth issues | Yes, all deployments | Within 1 week |
| **Medium Correctness** | Edge case bugs, rare data paths | Yes, all deployments | Within 2 weeks |
| **Non-Correctness** | UI, logging, docs, perf | No, deploy at convenience | Next release cycle |

### Release Template

When releasing a correctness fix, use this template:

```markdown
## ARMOR Release v<VERSION>

### Release Type
- [ ] Critical Correctness
- [ ] High Correctness
- [ ] Medium Correctness
- [ ] Non-Correctness

### Fixes Included
- bead bf-xxxx: Brief description of correctness fix
- bead bf-yyyy: Brief description of another fix

### Deployments Patched
- [ ] iad-ci
- [ ] iad-kalshi
- [ ] iad-native-ads
- [ ] rs-manager
- [ ] ord-devimprint
- [ ] iad-acb
- [ ] apexalgo-iad

### Deployments Pending
- **deployment-name**: Reason for delay, tracked in bf-zzzz

### Verification Steps Performed
- [ ] Canary health checked on all deployments
- [ ] MEK verification passed on all deployments
- [ ] End-to-end encryption/decryption tested on 3+ deployments
- [ ] Integration tests passed

### Rollback Plan (if needed)
If this release causes issues, rollback to previous version: <PREV_VERSION>

Rollback command:
```bash
kubectl --kubeconfig=/home/coding/.kube/<cluster>.kubeconfig \
  rollout undo deployment/armor -n <namespace>
```
```

---

## References

- [Disaster Recovery Runbook](disaster-recovery.md) — MEK escrow, restore drills
- [Deployment Config](https://github.com/jedarden/declarative-config) — All ARMOR manifests
- [ArgoCD API](https://argocd-ro-ardenone-manager-ts.ardenone.com:8444) — Application status
- [ARMOR Build Workflow](https://github.com/jedarden/ARMOR/blob/main/.beads/traces/bf-build/armor-workflowtemplate.yml) — CI/CD pipeline
