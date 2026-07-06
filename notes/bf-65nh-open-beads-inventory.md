# ARMOR Open Beads Inventory

**Generated:** 2026-07-06  
**Total Open Beads:** 44  
**Bead ID:** bf-65nh

---

## Summary by Status

| Status | Count |
|--------|-------|
| Blocked | 21 |
| Open | 16 |
| Completed | 6 |
| In Progress | 1 |

---

## Label Frequency

| Label | Count | Notes |
|-------|-------|-------|
| `split-child` | 29 | Most common — indicates beads split from parent |
| `deferred` | 10 | Beads marked for later work |
| `umbrella` | 4 | Parent/epic beads |
| `failure-count:2` | 3 | Retry tracking |
| `failure-count:4` | 2 | Retry tracking |
| `failure-count:1` | 2 | Retry tracking |
| `failure-count:6` | 1 | Retry tracking |
| `starvation-alert` | 1 | Urgent: bead discovery failure |

---

## Umbrella Beads (Epic/Parent)

| ID | Title | Labels |
|----|-------|--------|
| bf-1loh | Investigate bead starvation root cause | deferred, split-child, umbrella |
| bf-32ms | Wire ARMOR_PREFIX into rs-manager and cluster deployments | deferred, failure-count:4, umbrella |
| bf-83o2 | Document Pluck exclude_labels configuration | deferred, failure-count:1, split-child, umbrella |
| bf-nzm9 | Epic: ARMOR web dashboard — finalize in Go, remove Rust scaffold | umbrella |

---

## Blocked Beads (21)

These beads have dependencies preventing progress.

| ID | Title | Labels |
|----|-------|--------|
| bf-1by6 | Document root cause and propose fix | split-child |
| bf-1db1 | Document root cause and configuration fix | split-child |
| bf-1qb6 | Verify documentation completeness | split-child |
| bf-26g3 | Identify filtered beads by comparison | split-child |
| bf-2lob | Extract exclude_labels from Pluck config | split-child |
| bf-2pg2 | List all excluded labels | split-child |
| bf-351y | Apply S3 prefix to multipart upload operations | split-child |
| bf-36co | Fix bead discovery configuration | split-child |
| bf-3ab3 | Sync ARMOR deployments to latest image with ARMOR_PREFIX support | split-child |
| bf-3gpq | Document Pluck exclude_labels configuration | split-child |
| bf-3o0m | Test Pluck bead discovery logic | split-child |
| bf-3ss8 | List and categorize workspace open beads | split-child |
| bf-553l | Extract and parse exclude_labels settings | split-child |
| bf-5ofp | Apply S3 prefix to list and enrich operations | split-child |
| bf-5oik | Verify bead worker discovers beads | split-child |
| bf-5vpl | Apply S3 prefix to core object operations | split-child |
| bf-5zqn | Verify ARMOR_PREFIX configuration across declarative-config deployment | split-child |
| bf-6bdj | Add bead starvation monitoring | split-child |
| bf-9fle | Add ARMOR_PREFIX support for shared-bucket deployments | none |
| bf-lxu0 | Write Pluck exclude_labels documentation | split-child |
| bf-v3wd | Build and publish new ARMOR image with ARMOR_PREFIX | split-child |

---

## Open Beads (16)

These beads are ready for work.

| ID | Title | Labels |
|----|-------|--------|
| bf-1cgd | Test bead | none |
| bf-1daa | Dashboard: verify bucket browser UI acceptance criteria; fill test gap | none |
| bf-1hm4 | Review Pluck configuration settings | split-child |
| bf-24kz | Document root cause and required configuration fix | split-child |
| bf-2y8s | Review Pluck configuration for filter settings | none |
| bf-431p | Identify configuration mismatch causing bead invisibility | split-child |
| bf-43du | Test Pluck filtering logic | split-child |
| bf-5g60 | Extract and review Pluck configuration | split-child |
| bf-668r | Dashboard: verify encryption status + cache statistics display; fill gap | none |
| bf-qagm | Review Pluck configuration settings | split-child |

### High Priority Open Beads

| ID | Title | Labels | Priority Notes |
|----|-------|--------|----------------|
| bf-3b64 | Starvation alert: beads invisible to worker | deferred, failure-count:6, starvation-alert | **URGENT** — bead discovery broken |
| bf-yxq0 | Rewrite S3 key paths in all handlers using configured prefix | deferred, failure-count:4 | ARMOR_PREFIX implementation |

---

## Completed Beads (6)

These beads are marked completed but not yet closed.

| ID | Title | Labels |
|----|-------|--------|
| bf-16vq | Offline decrypt/recovery CLI — restore objects with only MEK + B2 cipher | deferred |
| bf-3djp | S3 API lets clients read/write/delete internal .armor/ keys — add namespace validation | deferred, failure-count:2 |
| bf-42mv | Reconcile container image references: deploy manifests + README point to same image | deferred, failure-count:1 |
| bf-4e4q | Update docs/plan/plan.md: Phase 4 shipped, contradictions and stale statements | deferred, failure-count:2 |
| bf-4ox4 | Dashboard: JSON object-listing API endpoint (GET /dashboard/api/list) | none |
| bf-68dl | Gate image publish on unit tests: run go vet + go test in Dockerfile build | deferred, failure-count:2 |

---

## In Progress (1)

| ID | Title | Labels |
|----|-------|--------|
| bf-65nh | List and document all open beads | split-child |

---

## Notable Patterns

### 1. Pluck Configuration Investigation (10+ beads)
Multiple beads investigating Pluck's `exclude_labels` configuration and bead discovery filtering. This appears to be related to the bead starvation issue.

**Related beads:** bf-1hm4, bf-1loh, bf-24kz, bf-2y8s, bf-2lob, bf-2pg2, bf-36co, bf-3gpq, bf-3o0m, bf-3ss8, bf-553l, bf-5oik, bf-83o2, bf-lxu0, bf-qagm

### 2. ARMOR_PREFIX Implementation (8+ beads)
Adding prefix support for shared-bucket deployments.

**Related beads:** bf-32ms, bf-351y, bf-3ab3, bf-5ofp, bf-5vpl, bf-5zqn, bf-9fle, bf-v3wd, bf-yxq0

### 3. Dashboard Finalization (2+ beads)
Completing the Go-based web dashboard.

**Related beads:** bf-1daa, bf-4ox4, bf-668r, bf-nzm9

### 4. Bead Starvation Issue (3+ beads)
Critical issue where beads are invisible to the worker.

**Related beads:** bf-1loh, bf-3b64, bf-431p, bf-6bdj

---

## Unusual Label Combinations

The following beads have the `deferred` label without `split-child` or `umbrella`:

| ID | Labels | Notes |
|----|--------|-------|
| bf-16vq | deferred | Completed but deferred |
| bf-3djp | deferred, failure-count:2 | Completed but deferred |
| bf-42mv | deferred, failure-count:1 | Completed but deferred |
| bf-4e4q | deferred, failure-count:2 | Completed but deferred |
| bf-68dl | deferred, failure-count:2 | Completed but deferred |

---

## Recommendations

1. **Resolve bead starvation first** — bf-3b64 is marked as a starvation alert with 6 failures
2. **Close completed beads** — 6 beads are completed but not closed
3. **Investigate Pluck configuration** — Large cluster of related blocked beads
4. **Complete ARMOR_PREFIX implementation** — Blocked chain waiting on image build (bf-v3wd)

---

**End of Inventory**
