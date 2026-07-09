# Pluck Settings That Hide Beads - Complete Analysis

**Date:** 2026-07-09  
**Bead:** bf-4351  
**Source:** Analysis of bf-ogec (exclude_labels) and bf-1jwl (filter configurations)

## Executive Summary

Pluck has **7 distinct settings** that can hide beads from selection, divided into two categories:

1. **Direct Hiding (4 settings)** - Explicitly filter beads from the candidate list
2. **Indirect Hiding (3 settings)** - Create conditions that prevent bead selection

---

## 1. Direct Hiding Settings

These settings explicitly remove beads from the candidate list.

### 1.1 `exclude_labels` - Primary Filter Mechanism

**Type:** Configurable (via `PluckStrand::new()`)  
**Default Value:** `["deferred", "human", "blocked"]`  
**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:13`

**How it hides beads:**
- **Hides any bead** with a label matching any entry in the `exclude_labels` list
- Applied at **TWO layers** for defense in depth:
  1. Store query: `Filters { exclude_labels: self.exclude_labels.clone() }`
  2. Defensive guard: `candidates.retain(|b| !b.labels.iter().any(|l| self.exclude_labels.contains(l)))`

**Impact:** ⚠️ **HIGH** - This is the primary bead visibility control

**Behavior rules:**
- Empty `exclude_labels` → Uses defaults: `["deferred", "human", "blocked"]`
- Non-empty `exclude_labels` → **Replaces** defaults (no merge)
- Matching is **ANY match** (one excluded label = bead hidden)

**Examples of hidden beads:**
```rust
// With default exclude_labels ["deferred", "human", "blocked"]
bead.labels = ["deferred"]           // ❌ HIDDEN
bead.labels = ["human", "bug"]       // ❌ HIDDEN (has "human")
bead.labels = ["blocked", "P2"]      // ❌ HIDDEN (has "blocked")
bead.labels = ["bug", "P2"]          // ✅ VISIBLE (no excluded labels)
```

---

### 1.2 `assignee` Filter - Actor-Specific Filtering

**Type:** Configurable (via `Filters.assignee`)  
**Default Value:** `None` (no filter)  
**Location:** `/home/coding/NEEDLE/src/bead_store/mod.rs:76-82`

**How it hides beads:**
- **Hides beads NOT assigned** to the specified actor
- Only returns beads where `bead.assignee == specified_actor`
- When `None`, no assignee filtering is applied

**Impact:** ⚠️ **MEDIUM** - Context-dependent (powerful in multi-actor environments)

**Examples:**
```rust
// Filters { assignee: Some("claude-worker") }
bead.assignee = Some("claude-worker")  // ✅ VISIBLE
bead.assignee = Some("human-worker")   // ❌ HIDDEN
bead.assignee = None                   // ❌ HIDDEN
```

**Use case:** Ensures each worker only sees beads assigned to them in distributed systems.

---

### 1.3 Status Filter - In-Progress Hiding

**Type:** Hardcoded (non-configurable)  
**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:131`

**How it hides beads:**
- **Hides beads with `in_progress` status**
- Applied via: `matches!(b.status, crate::types::BeadStatus::InProgress)`
- Prevents multiple workers from claiming the same bead

**Impact:** ⚠️ **CRITICAL** - Prevents race conditions (cannot be disabled)

**Examples:**
```rust
bead.status = InProgress   // ❌ HIDDEN (claimed by another worker)
bead.status = Open          // ✅ VISIBLE (if no other filters apply)
bead.status = Closed       // ✅ VISIBLE (if no other filters apply)
```

**Purpose:** Distributed locking mechanism - ensures each bead is processed by at most one worker.

---

### 1.4 Stale Assignee Filter - Orphaned Bead Prevention

**Type:** Hardcoded (non-configurable)  
**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:132`

**How it hides beads:**
- **Hides `Open` beads that have an assignee**
- Applied via: `b.status == crate::types::BeadStatus::Open && b.assignee.is_some()`
- Prevents hot-loops retrying beads with stale assignees

**Impact:** ⚠️ **HIGH** - Prevents SELECTING→CLAIMING→RETRYING spin loops

**Examples:**
```rust
bead.status = Open, bead.assignee = Some("worker-1")  // ❌ HIDDEN (stale assignee)
bead.status = Open, bead.assignee = None              // ✅ VISIBLE (unassigned open bead)
bead.status = InProgress, bead.assignee = Some("worker-2")  // ❌ HIDDEN (in_progress filter also applies)
```

**Purpose:** Prevents workers from endlessly retrying beads that failed to release their assignee after a crash.

---

## 2. Indirect Hiding Settings

These settings create conditions that can effectively hide beads through side effects.

### 2.1 `split_after_failures` - Auto-Split Trigger

**Type:** Configurable (via `PluckStrand::with_split_threshold()`)  
**Default Value:** `3`  
**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:141-148`

**How it hides beads:**
- **Replaces the bead with a Split result** instead of returning it for processing
- Triggered when `failure-count:N` label ≥ threshold
- Does NOT filter the bead from the list, but **prevents normal processing**

**Impact:** ⚠️ **MEDIUM** - Diverts bead to split handler instead of normal flow

**Examples:**
```rust
// split_after_failures = 3
bead.labels = ["failure-count:2"]   // ✅ PROCESSED NORMALLY (2 < 3)
bead.labels = ["failure-count:3"]   // ❌ DIVERTED TO SPLIT (3 >= 3)
bead.labels = ["failure-count:5"]   // ❌ DIVERTED TO SPLIT (5 >= 3)
```

**Behavior rules:**
- Threshold = `0` → Split disabled (bead always processed normally)
- Threshold > `0` → Split triggered when failure count >= threshold
- Look for labels matching: `failure-count:N`

**Purpose:** Automatic bead decomposition when a bead repeatedly fails processing.

---

### 2.2 Sort Order - Candidate Prioritization

**Type:** Hardcoded (non-configurable)  
**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:79-86`

**How it hides beads:**
- **Indirect hiding** through prioritization
- Sort order: `(priority ASC, created_at ASC, id ASC)`
- Lower-priority beads are **effectively hidden** when higher-priority beads exist

**Impact:** ⚠️ **LOW** - Does not truly hide, but delays visibility

**Examples:**
```rust
// In a queue with 1000 P1 beads:
P1 bead (id: "aaa")  // ✅ PROCESSED FIRST
P1 bead (id: "zzz")  // ✅ PROCESSED EVENTUALLY
P2 bead (id: "aaa")  // ⚠️ EFFECTIVELY HIDDEN until all P1s are done
P3 bead (id: "aaa")  // ⚠️ EFFECTIVELY HIDDEN until all P1s and P2s are done
```

**Purpose:** Ensures deterministic, priority-ordered processing across distributed workers.

---

### 2.3 Empty Queue - No Candidates Available

**Type:** Runtime condition (not a setting)  
**Location:** `/home/coding/NEEDLE/src/strand/pluck.rs:135-138`

**How it hides beads:**
- **All beads are hidden when filter combination returns empty candidate list**
- Returns `StrandResult::NoWork` when `candidates.is_empty()`
- Common when all beads have excluded labels or are in-progress

**Impact:** ⚠️ **VARIABLE** - Depends on filter configuration and queue state

**Examples:**
```rust
// Scenario: Queue has 100 beads, but all are excluded
Queue state:
- 50 beads with "deferred" label
- 30 beads with "in_progress" status
- 20 beads with "blocked" label

Result: ❌ ALL HIDDEN → NoWork returned
```

**Purpose:** Prevents hot-loops when no work is available.

---

## 3. Combined Hiding Effects

Multiple filters can combine to hide beads. Here are common combinations:

### Combination 1: "Stale Orphan" Hide
```
bead.status = Open
bead.assignee = Some("dead-worker")
bead.labels = ["failure-count:5"]
```
**Result:** ❌ **HIDDEN by stale assignee filter** (even though it has high failure count)

### Combination 2: "Blocked Human Task" Hide
```
bead.status = Open
bead.assignee = None
bead.labels = ["human", "blocked", "review-needed"]
```
**Result:** ❌ **HIDDEN by exclude_labels** (matches BOTH "human" AND "blocked")

### Combination 3: "Assigned In-Progress" Hide
```
bead.status = InProgress
bead.assignee = Some("active-worker")
bead.labels = []
```
**Result:** ❌ **HIDDEN by status filter** (in_progress takes precedence)

### Combination 4: "Failed Excluded" Hide
```
bead.status = Open
bead.assignee = None
bead.labels = ["deferred", "failure-count:10"]
```
**Result:** ❌ **HIDDEN by exclude_labels** ("deferred" match, failure count irrelevant)

---

## 4. Settings Summary Matrix

| Setting | Type | Default | Direct Hide? | Configurable? | Impact |
|---------|------|---------|--------------|---------------|--------|
| `exclude_labels` | Vec<String> | ["deferred","human","blocked"] | ✅ YES | ✅ YES | ⚠️ HIGH |
| `assignee` | Option<String> | None | ✅ YES | ✅ YES | ⚠️ MEDIUM |
| Status filter | Hardcoded | InProgress | ✅ YES | ❌ NO | ⚠️ CRITICAL |
| Stale assignee filter | Hardcoded | Open + assignee | ✅ YES | ❌ NO | ⚠️ HIGH |
| `split_after_failures` | u32 | 3 | ⚠️ INDIRECT | ✅ YES | ⚠️ MEDIUM |
| Sort order | Hardcoded | (priority, created_at, id) | ⚠️ INDIRECT | ❌ NO | ⚠️ LOW |
| Empty queue | Runtime condition | N/A | ✅ YES | N/A | ⚠️ VARIABLE |

---

## 5. Configuration Recommendations

### To MAXIMIZE bead visibility:
```rust
// Use empty exclude_labels (note: this activates DEFAULTS!)
PluckStrand::new(vec![])  // Actually applies defaults

// Must explicitly provide empty list to disable:
// Modify source code to use empty DEFAULT_EXCLUDE_LABELS
// Or pass custom list: PluckStrand::new(vec!["only-this-label"])

// Disable split trigger:
PluckStrand::with_split_threshold(0)  // 0 = disabled

// No assignee filter:
Filters { assignee: None, exclude_labels: vec![] }
```

### To MINIMIZE bead visibility (strict filtering):
```rust
// Aggressive exclusions:
PluckStrand::new(vec![
    "deferred", "human", "blocked",  // defaults
    "waiting", "review", "external",  // custom
])

// Low split threshold:
PluckStrand::with_split_threshold(1)  // split on first failure

// Actor-specific:
Filters { assignee: Some("specific-worker"), exclude_labels: custom_labels }
```

---

## 6. Debugging Hidden Beads

### Debug Command to Check Why a Bead Is Hidden:

```bash
# Check bead labels
br show <bead-id> | grep -A 10 "Labels:"

# Check bead status and assignee
br show <bead-id> | grep -E "(Status|Assignee):"

# Check if bead is in ready queue
br ready --json | jq '.[] | select(.id == "<bead-id>")'
```

### Why a Bead Might Be Hidden:

1. **Has excluded label** → Check bead labels against `exclude_labels` list
2. **Wrong assignee** → Check if bead.assignee matches Filters.assignee
3. **In progress** → Check if bead.status is InProgress
4. **Stale assignee** → Check if bead.status is Open but has assignee
5. **High failure count** → Check if bead has `failure-count:N` ≥ threshold
6. **Low priority** → Check if higher-priority beads are queueing ahead
7. **Empty queue** → Check if all beads are filtered out

---

## 7. Key Insights

1. **Exclude labels are powerful**: The `exclude_labels` setting is the MOST COMMON way beads are hidden. Default labels (`deferred`, `human`, `blocked`) cover most deferral scenarios.

2. **Defense in depth**: Pluck applies label filtering TWICE - once in the store query, once in the strand. This ensures consistency even when backends omit label data.

3. **Hardcoded filters matter**: The status and stale assignee filters CANNOT be disabled. They are critical for distributed system correctness.

4. **Custom overrides replace defaults**: Providing custom `exclude_labels` REPLACES the defaults entirely - there is no merge. This is a common source of confusion.

5. **Failure counts trigger splits**: The `failure-count:N` label pattern is special - it can divert beads from normal processing flow without filtering them from the queue.

6. **Sort order creates effective hiding**: Low-priority beads can be effectively hidden indefinitely when higher-priority work exists, even though they pass all filters.

---

## 8. Related Beads

- **bf-ogec** - Extract exclude_labels settings from Pluck config
- **bf-1jwl** - Extract all filter configurations from Pluck
- **bf-3ax3** - Capture Pluck filtering debug output
- **bf-euin** - Parse filtering decisions from debug logs

---

**Status:** ✅ Complete - All Pluck settings that can hide beads have been identified and documented
