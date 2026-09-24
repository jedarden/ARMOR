# ARMOR CLI Command Reference

Operator-facing reference for the `armor` binary's subcommands: what each
command reads, what it writes, which credentials and keys it needs, which exit
code it returns, and where the safe-use boundaries are. The one-paragraph
summary in the [README](../README.md#subcommands) stays short; this document is
the per-command contract. Environment variables are referenced by name only —
the full values reference is [Configuration](configuration.md).

Every claim here is enforced by the smoke tests in
[`cmd/armor/cli_reference_test.go`](../cmd/armor/cli_reference_test.go): the
command list, every flag name, and the documented help output are checked
against the binary's own registry, so this page cannot silently drift from the
code.

## Global behavior

- `armor` with no subcommand runs `serve` — the container entry point relies on
  this default.
- `armor --help` (also `-h`, `-help`) prints the top-level command table and
  exits 0. `armor help` does the same.
- `armor <cmd> --help` prints that command's summary and its flags and exits 0
  without running the command.
- `armor --version` (also `-v`) prints the version line and exits 0.
- An unknown subcommand prints the command table to stderr and exits 2.
- An undefined or malformed flag prints the subcommand's usage to stderr and
  exits 2 without running the command.
- Every command that takes arguments rejects unexpected positional arguments
  with exit 2; `help` deliberately ignores them (asked-for help is never an
  error).

Exit-code conventions used by every command:

| Code | Meaning |
|------|---------|
| 0 | Success (for `check`, warnings still exit 0) |
| 1 | The requested work failed: missing credentials or keys, unreadable input, network or HTTP failure, corrupted objects |
| 2 | Usage error: unknown subcommand, undefined flag, missing required flag, unexpected positional argument |

Two credential conventions recur across the offline commands:

- **MEK sources**, tried in this order: `-mek` (hex, 64 chars), `-mek-file`
  (path to a file holding the hex), `-escrow` (see below), then the `ARMOR_MEK`
  environment variable where the command supports the fallback.
- **Escrow file** (`-escrow`): a self-contained JSON recovery package with
  `mek`, a `mek_ring` array of `{mek, fingerprint}` entries, and a `b2` object
  (`region`, `endpoint`, `access_key`, `secret_key`, `bucket`, `cf_domain`).
  Loading it exports the B2 fields into the process environment, so one file
  arms both the keys and the backend. It is the only multi-key input the CLI
  has. Format and provisioning: [Disaster Recovery](disaster-recovery.md).

## `armor serve`

Run the ARMOR S3-compatible server. This is the default command.

- **Inputs:** no flags; the entire configuration is environmental
  (`ARMOR_LISTEN` for the S3 API, `ARMOR_ADMIN_LISTEN` for the admin API, plus
  backend, encryption, and authentication variables —
  [Configuration](configuration.md)).
- **Outputs:** structured JSON logs on stdout; the S3 API on `ARMOR_LISTEN`;
  the admin API on `ARMOR_ADMIN_LISTEN`.
- **Exit codes:** 0 after a clean shutdown on SIGINT/SIGTERM (in-flight
  requests are drained with a 60-second cap); 1 when configuration loading,
  server construction, or listener binding fails; 2 on unexpected positional
  arguments.
- **Credentials/keys:** the full deployment set — `ARMOR_MEK`, backend
  credentials, client credentials, and `ARMOR_ADMIN_TOKEN` (without it the
  key-management and migration admin endpoints stay disabled, fail-closed).
- **Safe use:** the S3 listener deliberately sets no read/write deadlines so
  multi-gigabyte multipart transfers survive; front it with something that
  tolerates long requests. Keep the admin listener off public interfaces
  (loopback or tailnet) — it controls keys and migration. Restarting with the
  same `ARMOR_MEK` is what keeps existing objects readable.

## `armor demo`

Run a disposable local instance for trying ARMOR: filesystem backend, fixed
demo credentials, a fresh random in-memory MEK.

| Flag | Meaning |
|------|---------|
| `-dir` | Directory for the filesystem backend (default: a temp directory removed on exit) |
| `-listen` | S3 API listen address (default `127.0.0.1:9000`) |
| `-admin-listen` | Admin API listen address (default `127.0.0.1:9001`) |

- **Inputs:** the flags above; everything else is fixed by the command
  (backend `filesystem`, bucket `demo-bucket`, access key `armor`, secret key
  `armor-demo-secret`).
- **Outputs:** a startup banner with the listen addresses, the demo
  credentials, and a ready-to-paste AWS CLI configuration, all through the
  JSON logger on stdout.
- **Exit codes:** 0 on shutdown; 1 on setup failure (temp directory, config);
  2 on unexpected positional arguments.
- **Credentials/keys:** none required — the command generates a random MEK and
  prints the fixed demo credentials.
- **Safe use:** never point real clients at a demo. The MEK lives only in the
  process: with the default temp directory everything disappears on exit, and
  with `-dir` the files persist but become permanently undecryptable once the
  process is gone.

## `armor check`

Verify a deployment's live configuration and connectivity. Read-only.

- **Inputs:** no flags. Loads the same configuration the server would
  (`config.Load`), so run it inside the container or with the deployment's
  environment.
- **Probes:** `config` (required fields, credentials present), `backend`
  (bucket reachable), `cloudflare` (ranged GET of the newest
  `<prefix>.armor/canary/` object through the Cloudflare domain, reporting
  `CF-Cache-Status` and latency), `mek` (unwraps that canary's wrapped DEK
  with the configured MEK), `fingerprint` (scans objects under `.armor/` and
  fails when any object names a MEK fingerprint absent from the active key and
  ring — the retire gate).
- **Outputs:** a redacted configuration dump to stderr (bucket and key
  identifiers render as fingerprints); one `[PASS|WARN|FAIL] probe: message`
  line per probe on stdout; a final verdict line on stderr.
- **Exit codes:** 0 all probes pass (warnings still pass); 2 any probe FAIL;
  1 when the configuration itself fails to load.
- **Credentials/keys:** the deployment's own environment: `ARMOR_MEK`, backend
  credentials, and the rest of [Configuration](configuration.md). The
  `cloudflare` and `mek` probes need canary objects (the server writes them
  continuously) and only WARN when none exist.
- **Safe use:** read-only — safe against production at any time. A FAIL on
  `fingerprint` means objects exist that a future rotation would orphan; add
  the named fingerprints to the ring before retiring keys.

## `armor decrypt`

Decrypt one ARMOR-encrypted object offline — break-glass recovery with no
running server. Reads single-PUT envelopes (any envelope version) and
multipart objects (headerless ciphertext plus HMAC sidecar) identically to the
server's read path.

| Flag | Meaning |
|------|---------|
| `-mek` | Master encryption key as hex (32 bytes / 64 chars) |
| `-mek-file` | File holding the MEK hex |
| `-mek-ring` | Comma-separated MEK ring fingerprints (16 hex chars each); validates format only — it carries no key material, so ring unwrapping requires an escrow file |
| `-mek-env` | Allow the `ARMOR_MEK` environment fallback when no MEK flag is set (default true) |
| `-escrow` | Escrow JSON file: MEK, ring, and B2 credentials in one package |
| `-input` | The object: a `b2://bucket/key` URL or a local file path |
| `-b2-bucket` | Bucket when `-input` names only a key |
| `-b2-region` | B2 region (overrides `ARMOR_B2_REGION`) |
| `-b2-endpoint` | B2 S3 endpoint (overrides `ARMOR_B2_ENDPOINT`) |
| `-b2-prefix` | ADR-001 prefix the bucket stores objects under (default `ARMOR_PREFIX`); locates the multipart HMAC sidecar |
| `-key-id` | Expected `x-amz-meta-armor-key-id` for multi-key buckets |
| `-wrapped-dek` | Base64 wrapped DEK — required for local single-PUT files (the `x-amz-meta-armor-wrapped-dek` metadata value) |
| `-sidecar` | JSON HMAC sidecar file for a local multipart object |
| `-iv` | Object IV as hex (16 bytes) — required for a local multipart object (it has no header to read one from) |
| `-version` | Envelope version (1 or 2) for local multipart sidecars lacking a version field (default 1) |
| `-output` | Write plaintext here instead of stdout |
| `-v` | Verbose progress on stderr |
| `-read-concurrency` | Maximum concurrent ranged reads (default `ARMOR_READ_CONCURRENCY` or 16) |

- **Inputs:** one object, from B2 (`-input b2://bucket/key`, or `-input` +
  `-b2-bucket`) or the local filesystem. B2 reads need the backend
  environment: `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`,
  `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY` (optional
  `ARMOR_CF_DOMAIN`) — or an `-escrow` file, which exports them. Access and
  secret keys have no command-line flags on purpose.
- **Outputs:** the plaintext on stdout, or to `-output` (created mode 0644).
  Verbose diagnostics go to stderr.
- **Exit codes:** 0 decrypted and integrity-checked; 1 on any failure — MEK or
  escrow load, missing/unparseable input, backend error, wrong MEK, HMAC or
  plaintext-SHA verification failure, output write error; 2 on unexpected
  positional arguments.
- **Credentials/keys:** the object's MEK (or the ring entry matching its
  fingerprint, from escrow). A fingerprinted wrapped DEK
  (`v2:<fingerprint>:<base64>`) selects its key by fingerprint; legacy values
  trial-unwrap against the active key and then the ring.
- **Safe use:** this is the command that turns ciphertext into plaintext —
  redirect stdout to a file rather than letting it scroll into a terminal or a
  captured session log, and tighten `-output`'s permissions if the content is
  sensitive. Prefer `-mek-file` or `ARMOR_MEK` over `-mek`: a command-line key
  lands in shell history and `ps`. An escrow file is every secret at once —
  keep it mode 600 and delete it after the recovery. Integrity: single-PUT
  objects are verified against the header's whole-object SHA-256; multipart
  objects declare no whole-object digest (ADR-003 gap), so per-block HMACs are
  the guarantee there.

## `armor verify`

Audit stored objects for corruption: unwrap each object's DEK, decrypt, and
check per-block HMACs and digests — the same checks the server's read path
performs, without a running server.

| Flag | Meaning |
|------|---------|
| `-bucket` | Bucket to verify (required) |
| `-mek` | Master encryption key as hex |
| `-mek-file` | File holding the MEK hex |
| `-escrow` | Escrow JSON file — the only way to verify objects written under ring keys other than the active one |
| `-output` | Write the JSON report here (default stdout) |
| `-v` | Verbose output on stderr |
| `-prefix` | Only verify keys under this prefix |
| `-keys-file` | Verify exactly these keys: a JSON array or one key per line (`#` comments allowed) |
| `-since` | Skip objects modified before this time (RFC3339 or YYYY-MM-DD) |
| `-quick` | Verify envelope header and DEK unwrap only — skip downloading and HMAC-checking object bodies |
| `-concurrency` | Concurrent verifications (default 10) |
| `-b2-prefix` | ADR-001 prefix the bucket stores objects under (default `ARMOR_PREFIX`) |

- **Inputs:** B2 credentials and the keys to check. Backend environment:
  `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`, `ARMOR_B2_ACCESS_KEY_ID`,
  `ARMOR_B2_SECRET_ACCESS_KEY` — or an `-escrow` file, which supplies both the
  credentials and any ring keys. Key selection: the whole bucket by default,
  narrowed by `-prefix`, `-keys-file`, or `-since`. The internal `.armor/`
  namespace and `*.armor-manifest` sidecars are skipped; multipart objects
  whose B2 head carries no metadata resolve it from their ADR-016 manifest.
- **Outputs:** a JSON report (bucket, prefix, totals, and one row per object:
  status `OK`/`CORRUPTED`/`ERROR`, error, details, size, modification time,
  duration), on stdout or `-output`. Progress and every failure line with its
  reason go to stderr. Rows are sorted by key so two runs diff cleanly.
- **Exit codes:** 0 every object verified OK; 1 when any object is CORRUPTED
  or ERROR, when zero keys were selected, or when the MEK, keys file,
  `-since` value, backend, or report write fails; 2 when `-bucket` is missing
  or positional arguments were passed.
- **Credentials/keys:** at minimum the active MEK. Fingerprinted wrapped DEKs
  select their key by fingerprint (active key first, then the escrow ring);
  an object naming a fingerprint this run did not bring is an ERROR row
  (keying gap — the object may be fine), not a corruption verdict.
- **Safe use:** read-only against the bucket. Full mode downloads every
  selected object in its entirety — on a large bucket that is real egress and
  hours; scope with `-prefix`/`-keys-file`/`-since`, or sweep first with
  `-quick` (envelope + DEK only). SIGINT stops scheduling new work and still
  writes the report for what completed. CORRUPTED means assessed-and-damaged
  (HMAC mismatch, broken envelope, unparseable wrapped DEK); ERROR means the
  object could not be assessed at all. Continuous verification — the same
  checks on a schedule — is the restore verifier
  ([deployment guide](restore-verifier-deployment-guide.md)).

## `armor migrate`

Drive the server's format migration: re-encrypt objects written in legacy
envelope versions to the server's configured write format, under fresh
per-object keys. The CLI is a thin client of the admin API
(`POST`/`GET /admin/format/migrate`); the server performs the migration.

| Flag | Meaning |
|------|---------|
| `-admin-url` | Admin API endpoint (required), e.g. `http://127.0.0.1:9001` |
| `-dry-run` | Verify objects could be migrated without changing anything |
| `-target` | Target format version (`v3` or `3`); must match the server's configured write version |
| `-include` | Comma-separated source versions to migrate (default: `v2` for a v3 target, `v1` for a v2 target) |
| `-concurrency` | Concurrent workers (default: the server's own default) |
| `-watch` | Poll progress every 2 seconds until the migration completes |
| `-json` | Print the completion report as JSON on stdout |

- **Inputs:** the admin URL and a bearer token: `ARMOR_ADMIN_TOKEN` is
  required — the server disables its migration endpoints without it.
- **Outputs:** progress and the human-readable summary (including the
  per-dimension classification report: source layout, size bucket, key
  fingerprint, outcome) on stderr. With `-json`, the server's final state
  document goes to stdout and progress lines move to stderr, so stdout carries
  nothing but JSON.
- **Exit codes:** 0 when the migration was accepted or completed without
  failures (a `-watch` poll that finds no migration in progress also exits 0);
  1 when `ARMOR_ADMIN_TOKEN` is unset, an HTTP request fails, the response
  cannot be parsed, the watch fails, or the migration completed with failed
  objects; 2 on a missing `-admin-url` or unexpected positional arguments.
- **Credentials/keys:** `ARMOR_ADMIN_TOKEN` only. The MEK never travels
  through this command — the server holds it.
- **Safe use:** this mutates objects (re-encryption in place). Always start
  with `-dry-run` and read its classification report; check
  `ARMOR_FORMAT_VERSION` on the server before choosing `-target`. Point
  `-admin-url` at the admin listener, never the public S3 listener. The full
  procedure, failure behavior, and per-format outcomes: [V3 Migration
  Reference](research/migration/V3_Migration_Reference.md); rotation context:
  [Key Rotation Runbook](key-rotation-runbook.md).

## `armor client-config`

Print known-good, copy-pasteable configuration for a common S3-compatible
tool: endpoint, path-style addressing, region placeholder, credential
variable names, and the multipart contract currently in force for the
deployment's write format.

| Flag | Meaning |
|------|---------|
| `-for` | Tool to generate for (required): `aws-cli`, `rclone`, `boto3`, `duckdb`, `litestream`, `barman` |
| `-endpoint` | ARMOR endpoint URL (required), e.g. `http://localhost:9000` |
| `-bucket` | Bucket name to include in the examples |
| `-credential` | Named credential to reference in the output comments |

- **Inputs:** the flags above, plus the deployment's `ARMOR_*` environment
  when available: the write format decides whether the emitted multipart
  block describes the v2 uniform-part-size contract or v3's
  no-contract rule. When the environment is missing, the command warns on
  stderr and assumes v3.
- **Outputs:** the configuration text on stdout; warnings on stderr.
- **Exit codes:** 0; 2 for a missing `-for`/`-endpoint`, an unknown tool, or
  unexpected positional arguments.
- **Credentials/keys:** none — the command never reads a credential and only
  ever prints variable names or `YOUR_ACCESS_KEY_ID`-style placeholders.
- **Safe use:** read-only. Fill the credential placeholders from your
  provisioning workflow — never paste real values into the generated file and
  commit it. Per-client behavior notes and the tested matrix:
  [Multipart Client-Compatibility](multipart-client-compatibility.md);
  walkthroughs: [Connection Guide](connection-guide.md).

## `armor version`

Print build and version information.

| Flag | Meaning |
|------|---------|
| `-json` | Print a single-line JSON object instead of the text line |

- **Inputs:** none. `armor --version` and `armor -v` (handled before
  subcommand dispatch) print the same text line. With `-json`, the reported
  `format_write_version` honors `ARMOR_FORMAT_VERSION` (2 or 3) exactly as the
  server's configuration does.
- **Outputs:** `armor <version> (go<goversion>, <os>/<arch>)` on stdout, or
  the JSON object (`app`, `version`, `go`, `os`, `arch`,
  `format_write_version`, plus VCS `commit`/`dirty` when stamped).
- **Exit codes:** 0; 2 on unexpected positional arguments or an invalid
  `ARMOR_FORMAT_VERSION`; 1 if JSON rendering fails (not expected in
  practice).
- **Credentials/keys:** none.
- **Safe use:** none needed — no network, no filesystem, no secrets.

## `armor help`

Print the top-level help: the command table with one-line summaries and the
per-command help hint.

- **Inputs:** none; extra arguments are ignored — asked-for help is never an
  error.
- **Outputs:** the help text on stdout.
- **Exit codes:** always 0.
- **Credentials/keys:** none.
- **Safe use:** none needed. For a specific command's flags, run
  `armor <cmd> --help`.
