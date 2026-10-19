# ADR-017: Conditional-PUT Conflict Contract for Append-Only WARP Ingest Retries

**Status:** Accepted (implemented 2026-10-19; both conflict sites — the early
manifest/HEAD gate and the backend `PutIfAbsent` collision, on the small and
streaming paths — translate an explicit `If-None-Match: *` conflict to
412 and an unconditional one to 403, pinned by
`TestAppendOnlyConditionalPutConflictReturnsPreconditionFailed` and
`TestAppendOnlyConditionalPutConflictAtBackendGate`)
**Date:** 2026-10-19

## Context

ADR-012 gives an ACL that grants `put` without `delete` a create-only,
append-only contract: a PUT of a new key succeeds, a PUT (or DELETE) against
an existing key is denied, and the backend's `PutIfAbsent` serializes
competing writes within one ARMOR process (`internal/backend/conditional_put.go`).
B2 does not provide cross-replica conditional writes; create-only writers
must use a single ARMOR replica, as documented in `docs/http-api-reference.md`.
In the handler, `appendOnlyPut` (`internal/server/handlers/conditional_put.go`)
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
object already exists, the client's own conditional request tells it so.
The `412` proves key existence, not which request wrote the key or that its
bytes match the attempted write. A retrying client must retrieve and verify
the existing object before treating the retry as success.

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

1. **A conditional conflict on a retry.** WARP's own explicit
   `If-None-Match: *` correctly detected that the object is already there.
   This may be the benign, expected outcome of its retry strategy; WARP must
   compare the stored object with its local exact bytes before treating it as
   success.
2. **An unconditional overwrite attempt.** The credential sent a plain PUT
   (no `If-None-Match`) against an existing key; ARMOR forced create-only
   semantics server-side purely to protect the no-overwrite guarantee the
   credential is supposed to have. The client never asked for a conditional
   check, so there is nothing benign to report back — the write is refused
   because the credential has no right to make it.

Because both cases currently return `403 AccessDenied`, WARP cannot
distinguish "conditional conflict needing verification" from "this credential's
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
   conditional-PUT idiom (WARP among them) can distinguish a precondition
   conflict from an authorization fault, then read and compare the existing
   object before concluding that its earlier write succeeded.
2. **Unconditional conflict → `403 AccessDenied`, unchanged.** When an
   append-only credential's PUT has no explicit `If-None-Match` and the key
   already exists, ARMOR continues to return `403 AccessDenied`, exactly as
   deployed today. The credential did not ask for a conditional check, so
   ARMOR must not report a conditional outcome (`412`) that implies one was
   evaluated at the client's request; it is refusing a write the credential
   has no overwrite right to make.
3. **The backend's create-only enforcement does not change.** Every
   append-only PUT — conditional or not — still uses `PutIfAbsent` in both
   cases. That path is atomic within one ARMOR process; B2 does not provide a
   cross-replica conditional write, so the existing single-replica deployment
   requirement remains. Only the *client-facing status/code translation*
   on conflict depends on whether
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

Until that runtime change is deployed, WARP can recover an ambiguous `403`
from its explicit conditional retry by reading the existing object and
comparing its exact bytes with the local event. The same comparison remains
necessary after ARMOR starts returning `412`.

## Alternatives Considered

1. **Always return `412` for any append-only conflict, dropping `403`
   entirely.** Rejected: an unconditional write against a key the credential
   cannot overwrite is an authorization fact independent of HTTP conditional
   semantics. Reframing a denied destructive write as a precondition failure
   would make a credential-scope problem — or an attack probe — look to
   monitoring, and to clients that don't inspect the error body, like a
   routine conditional-write race.
2. **Leave current behavior (always `403`) unchanged.** Rejected: this is
   the status quo this ADR exists to fix. It collapses "conditional conflict
   needing verification" and "authorization failure" into one signal and forces
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
  similarly-shaped retry clients) can distinguish a precondition conflict
  from an authorization denial, then verify stored bytes before resolving a
  stale retry as success. The append-only no-overwrite guarantee is unchanged.
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
- **Neutral:** no backend write-path change; the client-visible HTTP status
  and S3 error code change for explicit conditional conflicts only.

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
