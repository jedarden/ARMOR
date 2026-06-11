# Workspace Learnings

This file is automatically managed by NEEDLE. Learnings from completed beads are captured here.

### 2026-06-10 | bead: armor-l64 | worker: claude-code-glm-4.7-delta | type: other | reinforced: 177
- **Observation:** For health check debugging on pods without curl/wget, use kubectl port-forward to access endpoints locally.
- **Confidence:** high
- **Source:** reusable-pattern from armor-l64

### 2026-06-10 | bead: armor-l64 | worker: claude-code-glm-4.7-delta | type: other | reinforced: 0
- **Observation:** The issue appears to have already been resolved - likely by the version upgrade from 0.1.8 to 0.1.11 and/or the ExternalSecret refresh.
- **Confidence:** medium
- **Source:** surprise from armor-l64

### 2026-06-10 | bead: bf-520v | worker: claude-code-glm-4.7-oscar | type: other | reinforced: 5
- **Observation:** Using cached secrets for migration avoided OpenBao dependency; production log verification was accepted when RBAC blocked exec
- **Confidence:** medium
- **Source:** what-worked from bf-520v

### 2026-06-10 | bead: bf-520v | worker: claude-code-glm-4.7-oscar | type: other | reinforced: 0
- **Observation:** Attempting kubectl exec through read-only proxy; ExternalSecrets sync remains unresolved but doesn't block operations
- **Confidence:** low
- **Source:** what-didnt-work from bf-520v

### 2026-06-10 | bead: session-0989caa8-ede8-47eb-92dc-8443862a6b86 | worker: needle | type: other | reinforced: 460
- **Observation:** Action-outcome: Bash → Exit: (Bash completed with no output)
- **Confidence:** medium
- **Source:** transcript action-outcome: 0989caa8-ede8-47eb-92dc-8443862a6b86

### 2026-06-10 | bead: session-0989caa8-ede8-47eb-92dc-8443862a6b86 | worker: needle | type: bug-fix | reinforced: 0
- **Observation:** Error pattern: bash: {"command":"br close bf-2928","description":"Close the completed bead"} — Exit code 1
- **Confidence:** high
- **Source:** transcript error: 0989caa8-ede8-47eb-92dc-8443862a6b86

### 2026-06-10 | bead: session-f6d9ed7f-893e-453b-92c9-af01beab7fab | worker: needle | type: other | reinforced: 7
- **Observation:** Reasoning pattern: I have unstaged changes. Let me check what they are and stash them, then pull and rebase.
- **Confidence:** low
- **Source:** transcript: f6d9ed7f-893e-453b-92c9-af01beab7fab

### 2026-06-10 | bead: drift-drift-0-tool | worker: needle-drift | type: other | reinforced: 9
- **Observation:** Tool usage drift across sessions: Session B used `Read` tool, session A did not
- **Confidence:** medium
- **Source:** drift-cluster: drift-0

