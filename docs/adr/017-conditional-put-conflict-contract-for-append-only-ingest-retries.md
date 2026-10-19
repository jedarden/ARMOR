# ADR-017: Conditional-PUT Conflict Contract for Append-Only WARP Ingest Retries

**Status:** Proposed
**Date:** 2026-10-19

## Context

ADR-012 gives an ACL that grants `put` without `delete` a create-only,
append-only contract: a PUT of a new key succeeds, a PUT (or DELETE) against
an existing key is denied, and the backend's atomic `PutIfAbsent` closes the
concurrent-writer race (`internal/backend/conditional_put.go`). In the
handler, `appendOnlyPut` (`internal/server/handlers/conditional_put.go`)
detects a put-without-delete credential and forces `createOnly = true` for
*every* one of its writes, regardless of what the request itself asked for
(`internal/server/handlers/handlers.go:476-496`, `:715-722`). Whatever the
outcome, the conflict is translated by `writeAppendOnlyPutError`, which
unconditionally maps a create-only conflict to `403 AccessDenied`
(`docs/authentication.md`'s "Append-only PUT semantics"; `docs/error-responses.md`'s
`AccessDenied` entry; pinned by
`TestAppendOnlyACLThroughS3AndSharePaths`).

Separately, ARMOR already honors the standard S3 conditional-write idiom for
full-access credentials: a PUT with an explicit `If-None-Match: *` header
against an existing key returns `412 PreconditionFailed`
(`docs/http-api-reference.md`'s `PutObject` row; `docs/error-responses.md`'s
`PreconditionFailed` entry; pinned by
`TestPutObjectIfNoneMatchCreateOnlySmallAndStreaming`). That is the standard
mechanism a client uses to make a create-only write safely retryable: if the
object already exists, the client's own conditional request tells it so, and
a sane retrying client treats `412` on a retry as "my previous attempt
already landed," not as a failure.

**WARP** is a cross-service, append-only ingest client: it authenticates to
ARMOR with a put-without-delete credential and, on an ambiguous outcome (its
own request timed out, a proxy restarted mid-response, the ack was dropped),
retries by resending the identical object with an explicit
`If-None-Match: *` — exactly the idiom described above, used so a retry of a
write that actually succeeded is detected rather than silently treated as a
second, distinct write attempt. Because the credential is append-only,
today's handler forces create-only semantics on the retry regardless, and the
conflict is still routed through `writeAppendOnlyPutError`, which returns
`403 AccessDenied` — not the `412 PreconditionFailed` the client's own
explicit condition would get under a full-access credential for the
identical request shape.

This collapses two materially different situations into one status code for
every append-only credential:

1. **A stale retry of a write that already landed.** WARP's own explicit
   `If-None-Match: *` correctly detected that the object is already there.
   This is the benign, expected outcome of its retry strategy and should be
   treated as success, not as a denial.
2. **An unconditional overwrite attempt.** The credential sent a plain PUT
   (no `If-None-Match`) against an existing key; ARMOR forced create-only
   semantics server-side purely to protect the no-overwrite guarantee the
   credential is supposed to have. The client never asked for a conditional
   check, so there is nothing benign to report back — the write is refused
   because the credential has no right to make it.

Because both cases currently return `403 AccessDenied`, WARP cannot
distinguish "stale retry, safe to treat as success" from "this credential's
write to this key is genuinely unauthorized" (a revoked key, a narrowed ACL,
a real attack probe) using the status code alone. A retry client that treats
every `403` on this path as benign masks real authorization failures; one
that treats every `403` as a hard failure pages or backs off on every
ordinary stale-ingest retry.

## Decision

1. **Explicit conditional conflict → `412 PreconditionFailed`.** When an
   append-only credential's own PUT carries an explicit `If-None-Match: *`
   and the key already exists, ARMOR returns the standard S3
   `412 PreconditionFailed` response (S3 error code `PreconditionFailed`) —
   the same response a full-access credential already receives for the
   identical request shape. A retrying client using the standard
   conditional-PUT idiom (WARP among them) can then tell a stale,
   already-applied retry from an authorization fault using only the status
   code it already branches on for ordinary S3 conditional writes.
2. **Unconditional conflict → `403 AccessDenied`, unchanged.** When an
   append-only credential's PUT has no explicit `If-None-Match` and the key
   already exists, ARMOR continues to return `403 AccessDenied`, exactly as
   deployed today. The credential did not ask for a conditional check, so
   ARMOR must not report a conditional outcome (`412`) that implies one was
   evaluated at the client's request; it is refusing a write the credential
   has no overwrite right to make.
3. **The backend's atomic create-only enforcement does not change.** Every
   append-only PUT — conditional or not — is still serviced as a single
   atomic `PutIfAbsent` at the backend in both cases. Only the
   *client-facing status/code translation* on conflict depends on whether
   the client's own request was explicitly conditional. This decision does
   not weaken the append-only no-overwrite guarantee; it only lets a client
   that already performs the standard conditional check observe the outcome
   ARMOR already computes internally, instead of a credential-class-specific
   substitute.
4. **This is a decision record, not an implemented change.** The currently
   deployed behavior — every append-only conflict returns `403`, regardless
   of whether the client's own request carried `If-None-Match: *` — is
   unchanged until this is implemented. Implementing it requires carrying
   the client's explicit-conditional intent (the first return value of
   `putObjectCreateOnly`, currently discarded once merged into
   `createOnly = createOnly || appendOnly`) separately through to both
   conflict sites in `handlers.go` (the early manifest/HEAD check and the
   backend `PutIfAbsent` failure), so `writeAppendOnlyPutError` — or its
   caller — can choose `PreconditionFailed` vs. `AccessDenied` based on that
   flag instead of unconditionally choosing `AccessDenied`. The existing
   `TestAppendOnlyACLThroughS3AndSharePaths` (unconditional case, pins `403`)
   and `TestPutObjectIfNoneMatchCreateOnlySmallAndStreaming` (full-access
   explicit case, pins `412`) must both keep passing, and a new test covering
   "append-only credential + explicit `If-None-Match: *` + existing key →
   `412`" is required before this lands.

## Alternatives Considered

1. **Always return `412` for any append-only conflict, dropping `403`
   entirely.** Rejected: an unconditional write against a key the credential
   cannot overwrite is an authorization fact independent of HTTP conditional
   semantics. Reframing a denied destructive write as a precondition failure
   would make a credential-scope problem — or an attack probe — look to
   monitoring, and to clients that don't inspect the error body, like a
   routine conditional-write race.
2. **Leave current behavior (always `403`) unchanged.** Rejected: this is
   the status quo this ADR exists to fix. It collapses "idempotent retry
   landed" and "authorization failure" into one signal and forces
   WARP-shaped retry clients into either over-alerting on ordinary retries
   or masking real denials.
3. **Signal the distinction via a response header or body field instead of
   the status code.** Rejected: it requires every conditional-retry client
   to parse an ARMOR-specific response shape instead of the S3 status code
   it already branches on for ordinary conditional PUTs, and leaves
   non-S3-aware tooling (generic retry middleware, proxies) unable to use
   the distinction at all.

## Consequences

- **Positive:** cross-service append-only ingest clients (WARP and
  similarly-shaped retry clients) can safely treat a stale, already-applied
  retry as success without weakening the append-only no-overwrite guarantee
  or masking genuine authorization failures.
- **Positive:** brings append-only credentials' explicit-conditional-PUT
  behavior into parity with full-access credentials' existing `412`
  contract for the identical request shape, removing an inconsistency
  between the two credential classes.
- **Negative/complexity:** the handler's create-only decision currently
  flattens "the server forced it (append-only)" and "the client asked for it
  (`If-None-Match: *`)" into one boolean. Implementing this decision requires
  threading the client's explicit intent separately through `PutObject`,
  `putObjectStreaming`, and both conflict sites, not just through
  `writeAppendOnlyPutError`.
- **Neutral:** no backend or wire-protocol change; this is purely a
  response-code decision on one existing, already-exercised code path.

## References

- `internal/server/handlers/conditional_put.go` — `putObjectCreateOnly`,
  `appendOnlyPut`, `storePutObject`, `writeAppendOnlyPutError`
- `internal/server/handlers/handlers.go:476-496,715-722` — `PutObject`
  create-only/append-only dispatch
- `internal/server/handlers/append_only_acl_test.go` —
  `TestAppendOnlyACLThroughS3AndSharePaths` (pins today's unconditional-
  overwrite `403`)
- `internal/server/handlers/conditional_put_test.go` —
  `TestPutObjectIfNoneMatchCreateOnlySmallAndStreaming` (pins full-access
  explicit-conditional `412`)
- `docs/authentication.md` — append-only PUT semantics
- `docs/error-responses.md` — `AccessDenied` / `PreconditionFailed` catalogue
  entries
- `docs/http-api-reference.md` — `PutObject` status table
- ADR-012 — authorization action verbs (origin of append-only, no-delete
  credentials)
