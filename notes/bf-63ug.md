# Pluck Configuration Files Location

**Date:** 2026-07-08  
**Workspace:** `/home/coding/ARMOR`  
**Bead:** bf-63ug

## Finding

**No external Pluck configuration files exist.** Pluck configuration is compiled into the NEEDLE binary.

## Search Results

### ARMOR Repository
- ✅ Searched root directory: No `.pluck.yml`, `pluck.yml`, `.pluckconfig`, or `pluckconfig` files found
- ✅ Searched `.beads/` directory: Only contains beads configuration (`config.yaml`)

### Home Directory
- ✅ No `~/.pluck.yml` file
- ✅ No `~/.pluckrc` file  
- ✅ No `~/.config/pluck/` directory

### System
- ✅ Pluck binary not installed in PATH (`which pluck` returns "not found")

## Actual Pluck Configuration

Pluck filtering rules are **compiled into NEEDLE**, not configured via external files:

**Source**: `/home/coding/NEEDLE/src/strand/pluck.rs:13`

**Default exclude labels** (hardcoded in Rust):
```rust
const DEFAULT_EXCLUDE_LABELS: &[&str] = &["deferred", "human", "blocked", "starvation-alert"];
```

## Beads Configuration (Not Pluck)

The `.beads/config.yaml` file found is for the beads system, not Pluck:

```yaml
# Beads Project Configuration
issue_prefix: armor
default_priority: 2
default_type: task
```

## Conclusion

Pluck does not use external configuration files in this environment. All configuration is embedded in the NEEDLE binary at compile time. To change Pluck behavior, the NEEDLE source code must be modified and recompiled.

## Related Work

This finding clarifies the investigation from beads `bf-1hm4`, `bf-4axz`, and others that referenced Pluck configuration - the configuration they were looking for is the compiled-in defaults, not external files.
