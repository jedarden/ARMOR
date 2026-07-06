# Bead bf-3bzz: Pluck Configuration Files Location Investigation

## Executive Summary

**Finding:** Pluck configuration does NOT exist as separate configuration files within the ARMOR codebase. Pluck is a component of the NEEDLE fleet orchestrator system that manages workspaces including ARMOR.

## Investigation Results

### 1. ARMOR Workspace Configuration

**File:** `.beads/config.yaml`
```yaml
issue_prefix: armor
default_priority: 2
default_type: task
```

**Purpose:** Configures the bead store (br/bead-forge), NOT Pluck directly
**Format:** YAML
**Location:** `/home/coding/ARMOR/.beads/config.yaml`

### 2. Pluck Configuration Locations (External to ARMOR)

Pluck configuration exists in the NEEDLE system, not in individual workspace repositories:

#### Source Code
**Path:** `/home/coding/NEEDLE/src/strand/pluck.rs`
**Type:** Rust source code
**Configuration mechanism:** Compiled constants and defaults

#### NEEDLE System Configuration
**Path:** `~/.needle/config.yaml`
**Type:** YAML
**Scope:** Global NEEDLE configuration affecting all workspaces

#### Documentation References
- `/home/coding/ytt/docs/pluck-configuration.md`
- `/home/coding/claude-governor/docs/plan/pluck-configuration.md`

### 3. Current Pluck Configuration for ARMOR

Since no custom Pluck configuration exists for ARMOR, it uses NEEDLE defaults:

#### Default Exclude Labels
**Source:** Hardcoded in `/home/coding/NEEDLE/src/strand/pluck.rs:13`
```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked", "starvation-alert"];
```

**Labels excluded from bead selection:**
- `deferred` - Beads marked for later processing
- `human` - Beads requiring human intervention
- `blocked` - Beads with blocking dependencies
- `starvation-alert` - Beads from alerting system

#### Split Threshold
**Default:** `3` (beads auto-split after 3 consecutive failures)
**Configuration:** Compiled default, no override

#### Sorting Order
**Behavior:** Hardcoded (not configurable)
1. Priority (ASC) - P0 before P1 before P2
2. Created at (ASC) - Older beads first
3. Bead ID (ASC) - Lexicographic tie-breaker

### 4. Workspace Configuration Format

**ARMOR bead store:** `.beads/config.yaml`
**Format:** YAML
**Accessibility:** Readable and accessible
**Current settings:**
- `issue_prefix: armor` - Bead ID prefix
- `default_priority: 2` - Default priority for new beads
- `default_type: task` - Default bead type

### 5. NEEDLE Integration

ARMOR workspace is managed by NEEDLE through:
- **Bead store database:** `.beads/beads.db` (SQLite)
- **JSONL checkpoint:** `.beads/issues.jsonl`
- **Learnings capture:** `.beads/learnings.md` (auto-managed by NEEDLE)

## Configuration File Summary

| Setting | Location | Format | Type |
|---------|----------|--------|------|
| **Pluck source** | `/home/coding/NEEDLE/src/strand/pluck.rs` | Rust | Compiled |
| **NEEDLE config** | `~/.needle/config.yaml` | YAML | System-wide |
| **ARMOR bead store** | `/home/coding/ARMOR/.beads/config.yaml` | YAML | Workspace-specific |
| **Exclude labels** | Compiled into NEEDLE binary | N/A | Runtime constant |
| **Split threshold** | Compiled into NEEDLE binary | N/A | Runtime constant |

## Key Findings

1. **No Pluck configuration files exist in ARMOR codebase** - Pluck is external
2. **ARMOR uses default Pluck behavior** - No custom overrides configured
3. **Configuration is centralized in NEEDLE** - Changes affect all workspaces
4. **Custom workspace config only affects bead store** - Not Pluck behavior
5. **All files are readable and accessible** - No permission issues found

## Search Methods Used

1. Searched ARMOR codebase for "pluck" and "Pluck" references
2. Examined all YAML/JSON/TOML configuration files
3. Checked deployment manifests
4. Searched git commit history
5. Verified NEEDLE system configuration
6. Cross-referenced with other workspace documentation

## Conclusion

Pluck configuration for ARMOR is handled entirely through the NEEDLE system's defaults. There are no Pluck-specific configuration files within the ARMOR repository. Any changes to Pluck behavior would require:
1. Modifying NEEDLE source code (for default behavior)
2. Configuring NEEDLE system-wide settings (affecting all workspaces)
3. Implementing workspace-specific overrides (currently not supported)

**Date:** 2026-07-06
**Workspace:** /home/coding/ARMOR
**Bead ID:** bf-3bzz
