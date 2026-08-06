# ADR-008: Server-Side Observability for Multipart Part-Size Rejections

**Status:** Proposed
**Date:** 2026-08-06

## Context

`queue-db`, a CNPG PostgreSQL cluster on `iad-ci` doing `barman-cloud` base backups through ARMOR, has failed **100% of its backup attempts since 2026-05-26 — 609 consecutive failures**. Live investigation this session traced the real cause from barman-cloud-backup's own logs:

```
InvalidPartSize... Part size 4228497 is not a multiple of the block size (65536 bytes).
```

This is a **correct rejection**. ADR-005 established a uniform-part-size contract requiring every non-final multipart part to be an exact multiple of ARMOR's fixed 64 KiB encryption block size (`docs/adr/005-out-of-order-multipart-uniform-part-size.md`), because part offsets and per-block HMAC indices are derived from block-aligned arithmetic — accepting a misaligned part would silently produce unreadable/corrupt ciphertext, exactly the class of bug ADR-002/003/005 already fought once. ARMOR refusing barman's 4,228,497-byte part is the system working as designed.

barman-cloud's own side is unhelpful here: it logs only `exit status 1`, with no indication of *why* the upload failed. (Separately, real config work is underway directly in `declarative-config` to fix barman's part-size configuration so this stops recurring on `iad-ci` — that is not this ADR's concern. This ADR is about making ARMOR itself a better citizen for the *next* S3 client that hits the same wall, regardless of which client it is — there will be a next time, since any client with a non-block-aligned default chunking strategy hits the identical constraint.)

### What's already fixed, and what isn't

Tracing the rejection through `internal/server/handlers/handlers.go`'s `UploadPart` handler turned up a wrinkle worth recording precisely, so the next investigation doesn't have to redo this triage:

- **The client-facing error message is already good.** As of commit `b521651d4` (2026-07-19, part of ADR-005's implementation), the block-alignment check in `UploadPart` returns:

  > `Part size %d is not a multiple of the block size (%d bytes). ARMOR's uniform-part-size contract (ADR-005) requires block-aligned parts. Use a part size that's a multiple of %d (e.g., 5,242,880 for 5MiB, 16,777,216 for 16MiB).`

  This names the received size, the required block size, and points at ADR-005 by name — it is the *specific, actionable message* this ADR would otherwise be proposing. It didn't help in the `queue-db` case for two independent, already-tracked reasons: (1) `queue-db`'s failure window (starting 2026-05-26) predates this fix (2026-07-19) entirely, and (2) deployed ARMOR versions are documented in `docs/plan/plan.md` Phase 5 as lagging repo HEAD by weeks-to-months on a known, separately-tracked basis (version-drift). This ADR does not duplicate that tracking — it addresses a gap that exists independent of which ARMOR version is deployed.
- **barman-cloud doesn't surface the response body it received.** Even a perfect `<Message>` only helps if the client prints it, and the evidence here is that it doesn't — `exit status 1` is all barman logged. This is a real, structural ceiling on how much value further polishing the wire-level message can add: it only helps *clients that choose to print it*.
- **ARMOR has no server-side observability tied to *why* a request was rejected — this part is still true today, independent of client behavior.** Confirmed by reading the actual code:
  - `internal/server/handlers/handlers.go` contains zero logging calls (`grep -c "log\.\|logger\.\|slog\.\|logging\."` on the file returns 0). None of the ~10 `writeError` call sites in the multipart path — `InvalidPartSize` (×4, including the exact one hit here), `InvalidPart` (contradiction/poisoning), `SlowDown` (part-1-pinning deferral), `NoSuchUpload` — emit any log line distinguishing which one fired or why.
  - The only per-request log line lives in `internal/server/server.go`'s `wrapHandler` ("request completed", Info level) and carries `method`, `path`, `status` (the raw HTTP code), and `duration_ms` — never the S3 `<Code>`/`<Message>` that was actually written to the client. A 400 from `InvalidPartSize` is indistinguishable in the log stream from a 400 from `InvalidRequest`, `NoSuchUpload` on a bad key, or anything else.
  - `internal/metrics/metrics.go`'s only generic request counter is `IncRequestsTotal(operation, httpStatus)` (method + HTTP status code, wired at `server.go:741`) — every other counter in that file is subsystem-specific (canary, key rotation, restore-verifier). Nothing breaks down failures by S3 error `Code`.

  So an ARMOR operator looking at VictoriaLogs (the log backend actually deployed per `docs/plan/plan.md` Phase 6 — Prometheus is not scraping these clusters) for a bucket with repeated backup failures sees a stream of `"status": 400` lines with no field to filter or alert on `InvalidPartSize` specifically, and no metric to chart it. They would have to reproduce the request or read the Go source to know the 400 was a block-alignment rejection — this session's own reverse-engineering path, except this session at least had the client's raw error text to start from (`exit status 1` plus manually correlating a re-run against ARMOR's logs); an operator working from ARMOR's operational surface alone doesn't get even that starting point today.

## Decision

Close the operator-side observability gap, without changing the already-correct wire-level rejection behavior or message content:

1. **Every S3-API error response ARMOR writes on the multipart part-size/contract paths must be logged server-side**, at a level distinguishable from ordinary request completion (e.g. `Warn` for 4xx client-input rejections such as `InvalidPartSize`/`InvalidPart`/`SlowDown`, reserving `Error` for 5xx). Each log entry should carry at minimum: the S3 error `Code`, bucket, key, and whatever operation-specific identifiers are already in scope at the call site — upload ID, part number, and (for size violations) both the received size and the expected/required size (block size or pinned `P`). This is naturally centralized in — or layered onto — the existing `writeError` helpers (`internal/server/handlers/handlers.go` and `internal/server/server.go` currently have near-duplicate implementations of the same XML-writing logic) rather than hand-added at each of the ~10+ call sites individually; e.g. an optional structured-fields parameter/variant on `writeError`.
2. **Add an S3-error-`Code`-labeled counter** (e.g. `armor_s3_errors_total{code, operation}` or equivalent), following the existing `expvar`-based pattern already used throughout `internal/metrics/metrics.go`, distinct from and additive to the existing generic `IncRequestsTotal(method, httpStatus)` counter. This gives operators a Code-granular axis (`InvalidPartSize` specifically, not just "some 400 happened") that the existing counter structurally cannot provide, chartable/alertable in Grafana the same way `multipart_healthy` and the restore-verifier gauges already are.
3. **Initial implementation scope is the ADR-005 multipart part-size/contract rejection paths** in `UploadPart` — `InvalidPartSize` (block-alignment and contradiction-detection variants), `InvalidPart` (poisoned-upload rejections), and `SlowDown` (part-1-pinning deferrals). These are the exact paths this incident exercised, and B2/CNPG-class continuous-backup workloads are the primary realistic source of block-misaligned parts going forward. Extending the same log-field/metric-label pattern to every other `writeError` call site in the codebase (auth errors, ACL denials, generic `InternalError`s, etc.) is valuable but explicitly **out of scope** for the initial bead — a low-risk, high-value follow-up once the pattern is proven here.
4. **No change to the `<Code>`/`<Message>` content returned to clients.** It is already specific and actionable as of ADR-005's 2026-07-19 implementation (see Context). This ADR is scoped entirely to ARMOR's own operational observability — logs and metrics an ARMOR operator can act on directly — not the client-facing wire contract, which does not need further work.

Implementation detail (exact log field names, whether the metric is a new `expvar.Map` keyed by code or a set of per-code counters, how `writeError` is restructured to carry optional context) is left to whoever picks up the implementation bead; this ADR fixes the *behavior* required, not the Go shape.

## Alternatives Considered

- **Make the wire-level error message even richer** (e.g. custom XML extension elements with a machine-readable required-multiple/received-size pair). Rejected: the barman-cloud evidence shows S3 clients frequently don't print the existing, already-good `<Message>` text at all — return-payload sophistication has a low ceiling when the client discards the body. The leverage is on ARMOR's own operational side, not the wire contract.
- **Rely solely on fixing fleet version drift** (the Phase 5-tracked, separately-owned problem of deployed versions lagging repo HEAD) so every deployment eventually carries ADR-005's message. Rejected as sufficient alone: even a fully current fleet gives ARMOR operators no way to see *how often* or *for which bucket/upload* `InvalidPartSize` is firing, without reproducing the failure or reading source. Version currency and this ADR's observability are complementary, not substitutes for each other.
- **A dedicated admin-API endpoint** (e.g. `/armor/multipart-errors`, mirroring the existing `/armor/canary` and `/armor/audit` endpoints) surfacing recent rejections. Considered but deferred: a log field plus a metric follows the exact pattern already established for every other subsystem in this codebase (canary, key rotation, restore-verifier) and is far cheaper to ship and consume via the existing VictoriaLogs/Grafana tooling. A dedicated endpoint can be revisited later if logs/metrics prove insufficient for triage in practice.
- **Do nothing further, since the message is already good as of ADR-005.** Rejected: the message being good on the wire doesn't help an ARMOR operator triaging from ARMOR's own logs/metrics (the barman-cloud case shows the client may not even relay it), and doesn't help distinguish this failure class from any other 4xx in ARMOR's own operational surface. The gap this ADR closes is real and independent of the wire-message quality.

## Consequences

- VictoriaLogs queries and alerts can filter on the S3 error `Code` directly (e.g. `InvalidPartSize`) instead of grepping a downstream client's logs across a different system, or reverse-engineering the cause from a bare HTTP status code.
- Grafana dashboards can chart error-`Code` frequency per bucket/deployment, turning "another client hits the block-alignment wall" into a dashboard signal an operator notices proactively, instead of a multi-hour investigation triggered by a downstream symptom (e.g. 609 silently-failing CNPG backups).
- Small log-volume increase confined to the 4xx/5xx multipart-rejection path — the hot successful-request path is untouched.
- No change to any client-observable behavior — zero compatibility risk, no coordination needed with client-side teams or configs.
- Explicitly leaves "log every `writeError` call site in the codebase" and "unify the two near-duplicate `writeError` implementations" as future follow-on work rather than doing it all under this ADR — keeps the initial implementation bead scoped and reviewable.
