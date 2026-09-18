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

# Append-only backup writer: can write and list, never read, overwrite-protect or delete
ARMOR_AUTH_BACKUP_ACL="mybucket:backups/*:put+list"

# Multipart writer that can clean up its own aborted uploads but never delete objects
ARMOR_AUTH_RAW_ACL="mybucket:raw/*:put+list+abort"

# Full access to one bucket, read-only to another
ARMOR_AUTH_CROSSBUCKET_ACL="bucket-primary:*:get+put+delete+list,bucket-audit:logs/*:get+list"
```

**Overwrite-as-destruction risk:** without bucket versioning, a compromised
`put`-only credential can still overwrite existing objects. Append-only writers
mitigate but do not eliminate this; it is accepted residual risk.

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
