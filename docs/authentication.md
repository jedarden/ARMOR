# ARMOR Authentication and ACLs

ARMOR uses its own credential system for client authentication. **These ARMOR
credentials are separate from your B2 credentials**: ARMOR validates clients
locally, then uses its own B2 credentials to talk to the backend. B2
credentials never leave the ARMOR server, multiple clients can share one ARMOR
with different keys and permissions, and access keys can be scoped per bucket
or per prefix.

## Default credential

The simplest deployment uses a single static key pair:

```bash
ARMOR_AUTH_ACCESS_KEY=my-access-key
ARMOR_AUTH_SECRET_KEY=my-secret-key
```

## Named credentials with ACLs

For multi-user deployments, define any number of named credentials via
environment triplets: one `ACCESS_KEY`, one `SECRET_KEY`, and one optional `ACL`:

```bash
# Credential named "READONLY" (the name is for your bookkeeping)
ARMOR_AUTH_READONLY_ACCESS_KEY=reader-key
ARMOR_AUTH_READONLY_SECRET_KEY=reader-secret
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*"

# Credential named "WRITER"
ARMOR_AUTH_WRITER_ACCESS_KEY=writer-key
ARMOR_AUTH_WRITER_SECRET_KEY=writer-secret
ARMOR_AUTH_WRITER_ACL="mybucket:*,otherbucket:uploads/*"
```

**ACL format**

- **Syntax:** `bucket:prefix[:actions]`
- **Multiple rules:** comma-separated (`bucket1:prefix1,bucket2:prefix2`)
- **Wildcard bucket:** `*` matches all buckets
- **Wildcard prefix:** `*` or an empty string matches all keys

**Action verbs** ([ADR-012](adr/012-authorization-action-verbs-and-consumer-separation.md)).
If no actions are specified, all verbs are permitted.

| Verb | S3 operations covered |
|------|----------------------|
| `get` | GetObject, HeadObject |
| `put` | PutObject, CreateMultipartUpload, UploadPart, CompleteMultipartUpload, CopyObject (destination) |
| `delete` | DeleteObject, DeleteObjects, DeleteBucket, DeleteBucketLifecycleConfiguration |
| `list` | ListObjectsV2, ListMultipartUploads, ListObjectVersions, ListParts, ListBuckets |
| `abort` | AbortMultipartUpload |

An entry granting `delete` continues to grant `abort` (abort was part of delete
before it became its own verb); the reverse does not hold.

```bash
# All verbs on logs/ (no action segment = all permitted)
ARMOR_AUTH_LOGS_ACL="mybucket:logs/*"

# Only GET and LIST on readonly/
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*:get+list"

# Append-only backup writer: can create new keys and list, never read or delete
ARMOR_AUTH_BACKUP_ACL="mybucket:backups/*:put+list"

# Append-only writer that also needs to read its own objects
ARMOR_AUTH_BACKUP_READER_ACL="mybucket:backups/*:get+put+list"

# Multipart writer that can clean up its own aborted uploads but never delete objects
ARMOR_AUTH_RAW_ACL="mybucket:raw/*:put+list+abort"

# Full access to one bucket, read-only to another
ARMOR_AUTH_CROSSBUCKET_ACL="bucket-primary:*:get+put+delete+list,bucket-audit:logs/*:get+list"
```

**Append-only PUT semantics:** an explicit action-scoped ACL that grants `put`
but not `delete` is create-only. A PUT of a new key succeeds; DELETE is always
rejected. A PUT to an existing key splits by the request's own condition
(ADR-017): a conflict on an explicit `If-None-Match: *` — the standard
create-then-retry idiom — returns `412 PreconditionFailed`, so a retrying
client can distinguish the conflict from an authorization fault and compare
the stored object with its local bytes before treating a stale retry as
resolved, while an unconditional overwrite attempt is rejected with
`403 AccessDenied` because the credential never asked for a conditional check
and has no overwrite right to exercise. The backend enforces the create-only
write atomically in both cases. Credentials with no action segment retain full
legacy access, and a credential granted both `put` and `delete` may overwrite
existing objects.

**Empty ACL:** a credential with no `ACL` has full access to `ARMOR_BUCKET`.

## Credentials from a YAML file

For deployments managed by Kubernetes or an external secret system, load
credentials from a file:

```bash
ARMOR_AUTH_FILE=/etc/armor/credentials.yaml
```

```yaml
credentials:
  - name: FORGEJO_BACKUP
    access_key: "forgejo-backup-key"
    secret_key: "forgejo-backup-secret"
    acl: "iad-ci:forgejo-backup/*:put+list"

  - name: READONLY_USER
    access_key: "readonly-key"
    secret_key: "readonly-secret"
    acl: "mybucket:readonly/*:get+list"

  - name: FULL_ACCESS
    access_key: "full-key"
    secret_key: "full-secret"
    # No ACL means full access to the configured bucket
```

File credentials are merged with environment credentials; environment wins on
an access-key collision (logged at WARN); duplicate access keys within the file
are skipped (first wins); the file is watched and reloaded without a restart;
validation errors name the entry index and field, never the values.

## S3 presigned URL authentication

SigV4 query-authenticated S3 requests must include `X-Amz-Expires` as an
integer from 1 through 604800 seconds (seven days). ARMOR rejects a missing,
empty, malformed, zero, negative, or larger value with HTTP 400 and the S3
error code `AuthorizationQueryParametersError`. A valid signature is still
required, and a URL past its signed expiry returns `RequestExpired`.

These S3 presigned URLs are separate from the `/admin/presign` and `/share/`
token flow, which uses its own `expires_in` duration.

## Payload integrity (`x-amz-content-sha256`)

`x-amz-content-sha256` is part of the signed SigV4 canonical request, so it is
also the client's statement about which body the signature covers. ARMOR
accepts exactly three values for it:

- **A hex-encoded SHA-256 digest** of the request body — the body is hashed as
  the handler reads it, and the request fails with HTTP 400 and the S3 error
  code `XAmzContentSHA256Mismatch` if the digest disagrees. The failure
  happens during the body read, before the object is finalised: a mismatched
  upload never becomes visible, never overwrites an existing object, and
  never occupies a multipart part.
- **`UNSIGNED-PAYLOAD`** — the body is not bound to the signature. This is the
  common shape for TLS-terminated endpoints and is accepted unchanged. An
  absent `x-amz-content-sha256` header behaves the same way.
- **`STREAMING-AWS4-HMAC-SHA256-PAYLOAD`** — the aws-chunked framing used by
  litestream, barman-cloud, and the AWS SDKs for streamed uploads is stripped
  before the handler sees the body, and the chained per-chunk signatures are
  verified: each chunk's `chunk-signature` must match the SigV4 streaming
  computation seeded from the Authorization header's signature (chunk N signs
  chunk N's data hash together with chunk N−1's signature), so every chunk of
  the body is bound to the authenticated request. The first bad chunk fails
  the read with HTTP 403 and `SignatureDoesNotMatch`, before the object is
  finalised — the same never-visible, never-overwrites guarantee as the
  digest mode above. A chunk header without a verifiable signature fails the
  same way. Presigned requests carry no Authorization header to seed the
  chain, so their streaming bodies are still accepted with the framing
  stripped and no per-chunk verification.

Any other value — for example `STREAMING-UNSIGNED-PAYLOAD-TRAILER`, which some
AWS SDKs send for checksum-trailer uploads — is rejected with HTTP 400 and
`InvalidArgument` before the handler runs; the message names the three
supported modes. Such a request was never readable by ARMOR before (its
framing would have been stored as object data), so the rejection turns a
silent-corruption case into a loud one.

The mismatch check applies to every request whose header is a hex digest,
including multipart part uploads and XML-body operations such as
DeleteObjects. Clients inside the ±15-minute signature skew window therefore
cannot replay a captured PUT with different bytes.
