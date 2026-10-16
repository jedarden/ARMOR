# Manifest repair and quarantine — operator guide

How to unstick an object that fails the ADR-016 ciphertext freshness gate,
without editing the backend store by hand. Added after the ord-devimprint
incident (2026-08) where one stale manifest 500'd on every GetObject for three
days because litestream compaction retried the retryable error forever.

## The condition

Multipart objects are read through a manifest object (`<key>.armor-manifest`)
that names the ciphertext object and carries a `completed-at` timestamp. Before
serving, GetObject checks the ciphertext's LastModified against that timestamp
(ADR-016, `verifyCiphertextFreshness`):

- ciphertext **older** than completion → normal multipart ordering, served
- ciphertext **newer** than completion → the manifest no longer describes the
  object at its ref; the read is rejected with a **retryable 500**
  (`InternalError`, message starts with `Stale manifest`)

A stale manifest does not heal on its own unless an in-progress overwrite is
still landing, and every 5xx-retrying client (litestream, restore-verifier)
will loop on it indefinitely. The 500 message names the two endpoints below so
the remediation is discoverable from the log line during the outage.

## Inspect first

```bash
curl -s -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  'http://<armor-admin>/admin/manifest?key=<object-key>' | jq .
```

Returns the manifest's ciphertext ref, its `completed_at`, whether the gate
currently passes (`fresh`), and quarantine state. `verify_error` carries the
reason a verdict could not be reached (dangling ciphertext ref, backend Head
failure) — the same condition behind the retryable 500.

All four endpoints accept `&bucket=<b>`; without it the server's configured
bucket is used. `key` is the logical object key as clients read it, not the
manifest key.

## Decide: repair or quarantine

| Situation | Action |
|---|---|
| The ciphertext at the ref is the right data (e.g. an identical re-assembly, or the loss is accepted) | **repair** |
| The manifest must not be served and the ciphertext must not be declared canonical; a human needs to look first | **quarantine** |
| The ciphertext object at the ref is gone | **quarantine** — there is no timestamp left to re-stamp to, so repair refuses |

### Repair — re-stamp completedAt

```bash
curl -s -X POST -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  'http://<armor-admin>/admin/manifest/repair?key=<object-key>' | jq .
```

Sets the manifest's completion timestamp to the ciphertext's LastModified,
declaring the ciphertext canonical. Provenance is preserved on the manifest:
`original-completed-at` records what the manifest used to say (first repair
only — repeat repairs never overwrite it) and `manifest-repaired-at` records
when. Repair also lifts any quarantine, since a repaired-but-still-blocked
object could never be the intent; re-quarantine afterwards if the repair was a
mistake. The response reports `fresh: true` after re-running the same gate
GetObject applies, so a 200 with `fresh: true` means reads have resumed.

### Quarantine — definitive non-retryable error

```bash
curl -s -X POST -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  'http://<armor-admin>/admin/manifest/quarantine?key=<object-key>&reason=ciphertext+overwritten+pending+review' | jq .
```

Marks the manifest quarantined. Reads then fail with **403 `AccessDenied`**
carrying the reason — a status S3 clients do not retry, so litestream and the
restore-verifier fail fast and surface the problem instead of looping. The
reason is metadata, so it must be printable ASCII and at most 256 characters;
an empty reason is recorded as `no reason recorded`. Quarantine works even
when the ciphertext ref dangles, and it wins over the freshness gate (a
quarantined-and-stale manifest gets the 403, not the 500).

### Release — lift a quarantine

```bash
curl -s -X POST -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  'http://<armor-admin>/admin/manifest/release?key=<object-key>' | jq .
```

Removes the quarantine markers and changes nothing else. Idempotent, so a run
release-after-repair script can call it unconditionally.

## Notes

- These operations rewrite only the manifest object; the ciphertext is never
  modified. Repair is auditable from the manifest itself via the
  `original-completed-at` / `manifest-repaired-at` metadata.
- Repair refuses to invent state: no manifest (404), no ciphertext ref (409),
  no completion timestamp (409 — such a manifest was never gated, and stamping
  one would silently switch it onto the gate), or a ciphertext with no
  LastModified (409).
- Quarantined objects still appear in listings and HEADs; only GetObject is
  gated. HEAD never ran the freshness check, so its behaviour is unchanged.
- All four routes sit behind the admin token gate like the rest of
  `/admin/*`; a missing bearer token is a 401.
- Semantics live in `internal/server/handlers/manifest_repair.go`, the HTTP
  surface in `internal/server/manifest_admin.go`.
