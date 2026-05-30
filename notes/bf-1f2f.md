# ARMOR Project Location Documentation (Bead bf-1f2f)

## Task
Identify ARMOR project location based on existing conventions.

## Investigation

### Checked Locations
- `~/repos/` — **Does not exist**
- `~/*` (home directory root) — **Contains all project repos**

### Existing Project Pattern
Project repositories are placed directly in `~/` (home directory), each in their own subdirectory:

| Project | Location |
|---------|----------|
| AgentScribe | `~/AgentScribe/` |
| NEEDLE | `~/NEEDLE/` |
| FORGE | `~/FORGE/` |
| CLASP | `~/CLASP/` |
| SIGIL | `~/SIGIL/` |
| mta-my-way | `~/mta-my-way/` |
| news-trader | `~/news-trader/` |
| botburrow-agents | `~/botburrow-agents/` |
| ardenone-cluster | `~/ardenone-cluster/` |
| declarative-config | `~/declarative-config/` |

### CLAUDE.md Convention
Per `/home/coding/CLAUDE.md`:
> **Project repos** are in their own subdirectories (e.g., `~/ardenone-cluster/`)

## Decision

**ARMOR project location: `/home/coding/ARMOR`**

This is the current location and it follows the established convention.

## Rationale

1. **Follows existing pattern** — All project repos live directly in `~/`, not in a `~/repos/` subdirectory
2. **Consistent with tooling projects** — Similar standalone tools (FORGE, NEEDLE, CLASP, SIGIL) use the same pattern
3. **Matches CLAUDE.md guidance** — Project repos are "in their own subdirectories" at the home level
4. **Already in place** — The ARMOR project already exists at `/home/coding/ARMOR` with full git history, README, and repository structure

## Conclusion

No changes needed. ARMOR is already correctly located at `/home/coding/ARMOR`.
