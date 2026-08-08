# Future consideration: multi-key / overlapping-MEK rotation

Not an ADR — no decision has been made here. This captures a question raised
2026-08-08 during an unrelated ARMOR status check, for whoever picks up MEK
rotation work next.

## Correction to a common assumption

Rotating the MEK does **not** require re-encrypting existing data. ARMOR
already uses envelope encryption — the MEK wraps a per-file DEK, not the
plaintext directly — so rotation is an O(N) `CopyObject` metadata re-wrap
(old-MEK-wrapped DEK → new-MEK-wrapped DEK), not a data re-upload. This is
implemented (`internal/server/key_rotation.go`, `POST /admin/key/rotate`),
documented in [key-rotation-runbook.md](key-rotation-runbook.md), and covered
by tests including mixed single-PUT/multipart prefixes, interrupted-rotation
resume via `.armor/rotation-state.json`, and the B2 5 GiB `CopyObject`
ceiling (objects above it come back as typed exceptions requiring a manual
multipart copy, not a silent skip).

## Treat "tested" as unproven until exercised live

The rotation test suite (`bf-3hwoly`, closed 2026-07-20) is real, non-theater
code — but as far as this note's author could verify, nobody has run an
actual MEK rotation against a live production ARMOR deployment yet, only
local/CI tests. This project has a long history (see the repo's own ADR-005
through ADR-011 saga) of closed beads and passing tests that didn't survive
first contact with production data. The runbook itself documents sharp
footguns that only a real rotation would surface: the rotate-request MEK
must be byte-identical to the OpenBao value or every rotated object becomes
unreadable, and replicas serve mixed MEKs until every pod restarts. Before
trusting rotation in an emergency, dry-run it once against a low-stakes
bucket/prefix.

## The actually-open design question

The current design is **single active MEK with a synchronous cutover**: all
four runbook steps (OpenBao write → ESO sync → rotate call → pod restart)
must complete before you're cleanly on the new key, and the rotate call
itself walks the entire bucket before returning. There's no support for
multiple concurrently-valid decrypt keys (e.g., "new writes use MEK v3, but
MEK v1/v2 remain valid for decrypting data not yet migrated") — that would be
a key-versioning/keyring architecture, not what's implemented.

Whether that's worth building is genuinely undecided. It would matter most
if the bucket ever gets large enough that a full `CopyObject` sweep becomes
slow/expensive relative to how urgently the MEK needs to roll (e.g. a
suspected-compromise rotation where you want the new key active *now* and
willing to background-migrate old objects over hours/days, rather than
gating "safely rotated" on the sweep finishing). For ARMOR's current bucket
sizes this hasn't been a problem. If it becomes one, this should get its own
ADR and beads — not a reopen of `bf-3hwoly`, which was scoped to proving the
existing single-key rotation works, not to a multi-key keyring.
