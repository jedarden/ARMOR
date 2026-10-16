# ARMOR

**Authenticated Range-readable Managed Object Repository**

ARMOR is an S3-compatible proxy. It encrypts objects before storing them in
Backblaze B2 and serves reads through a Cloudflare domain. Clients use ARMOR
credentials and an S3 endpoint; ARMOR holds the B2 credentials and master
encryption key (MEK). Its source tests cover a scoped set of S3 operations,
including single-PUT writes, reads, listing, and documented multipart cases.

## Install

Install Docker with the Compose plugin, then clone this repository for its
[`compose.yaml`](compose.yaml) and [`.env.example`](.env.example):

```bash
git clone https://github.com/jedarden/ARMOR.git
cd ARMOR
```

Select an image tag that has been published. The [release-status
register](docs/release-status.md) records `0.1.1975` as the latest published
tag on its 2026-10-15 status date; check the releases before selecting a newer
one. The checkout's [`VERSION`](VERSION) and Compose default can be ahead of
published images. For the commands below, set the tag once:

```bash
export ARMOR_VERSION=0.1.1975
docker pull ghcr.io/jedarden/armor:"$ARMOR_VERSION"
```

The published image targets linux/amd64. On Apple Silicon, set
`DOCKER_DEFAULT_PLATFORM=linux/amd64` before pulling or starting Compose.
To build from source instead, use Go as specified by `go.mod` and `make build`;
[AGENTS.md](AGENTS.md) has the build and test commands.

## 60-second demo

This profile stores data in a temporary filesystem and uses fixed demo
credentials. It needs no B2 or Cloudflare account.

```bash
docker compose --profile demo up -d armor-demo
docker compose --profile demo ps
docker run --rm --network container:$(docker compose --profile demo ps -q armor-demo) \
  -e AWS_ACCESS_KEY_ID=armor -e AWS_SECRET_ACCESS_KEY=armor-demo-secret \
  -e AWS_DEFAULT_REGION=us-east-1 amazon/aws-cli:2.29.0 \
  --endpoint-url http://127.0.0.1:9000 s3 ls
docker compose --profile demo down
```

The demo exposes port 9000 and sets a fixed demo admin token. Its temporary
MEK is lost when the process stops, so use the production steps for durable
objects.

## Production

These steps run one B2-backed instance with its S3 API on port 9000 and its
admin API bound to host loopback on port 9001. Keep the chosen image tag in
your shell or set `ARMOR_VERSION` in `.env`.

1. **Prepare B2.** Create a bucket and an application key with access to it.
   Record the bucket name, region, application key ID, and application key.
   ARMOR uses the B2 S3 API for writes. For Cloudflare reads, configure the
   bucket for public ciphertext downloads as described in the
   [Cloudflare setup guide](docs/cloudflare-setup.md).
2. **Prepare Cloudflare.** Create a proxied CNAME to the bucket's B2 download
   hostname and set up the read path in the [Cloudflare setup
   guide](docs/cloudflare-setup.md). Record the Cloudflare hostname without
   `https://`. Set it as `ARMOR_CF_DOMAIN`. If you leave that variable empty,
   ARMOR reads directly from B2 and B2 egress charges apply.
3. **Create the environment file and keys.** The following generates a random
   32-byte MEK and a separate admin bearer token directly into the ignored
   `.env` file; it prints neither value. Back up the MEK securely before
   writing objects. Losing it makes existing objects unreadable.

   ```bash
   umask 077
   cp -n .env.example .env
   python3 - <<'PY'
   from pathlib import Path
   import secrets

   path = Path('.env')
   values = path.read_text()
   for name in ('ARMOR_MEK', 'ARMOR_ADMIN_TOKEN'):
       values = values.replace(f'{name}=\n', f'{name}={secrets.token_hex(32)}\n', 1)
   path.write_text(values)
   PY
   ```

4. **Fill in the B2 and client credentials** in `.env`. Set
   `ARMOR_B2_REGION`, `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`,
   `ARMOR_BUCKET`, `ARMOR_CF_DOMAIN`, `ARMOR_AUTH_ACCESS_KEY`, and
   `ARMOR_AUTH_SECRET_KEY`. Use distinct ARMOR client credentials; they are
   separate from the B2 application key. [Required configuration](#required-configuration)
   lists the minimum, and the [configuration reference](docs/configuration.md)
   explains every setting. Set `ARMOR_VERSION` there if the shell export above
   will not persist.
5. **Start and verify.** Compose passes the MEK and admin token from `.env` to
   the server. `armor check` loads that same environment and probes the bucket,
   Cloudflare path, and MEK/canary state. An empty new bucket can produce
   warnings until the canary exists; investigate any `FAIL` result.

   ```bash
   docker compose --profile production up -d armor-production
   docker compose --profile production exec -T armor-production /armor check
   curl --fail http://127.0.0.1:9001/version
   ```

Keep `.env` out of version control. The admin token enables key management and
format migration; follow the linked runbooks before using either operation.

### Required configuration

| Variable | What to provide |
|----------|-----------------|
| `ARMOR_B2_REGION` | The bucket's B2 region, for example `us-east-005` |
| `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY` | The bucket-scoped B2 application key |
| `ARMOR_BUCKET` | The B2 bucket name |
| `ARMOR_CF_DOMAIN` | The Cloudflare read hostname; leave empty only if direct B2 reads are intended |
| `ARMOR_MEK` | A stable 32-byte key encoded as 64 hex characters |
| `ARMOR_AUTH_ACCESS_KEY`, `ARMOR_AUTH_SECRET_KEY` | Credentials that S3 clients use with ARMOR |
| `ARMOR_ADMIN_TOKEN` | A separate bearer token for admin operations |

## Clients

Point an S3 client at `http://<armor-host>:9000` and use the ARMOR client
credentials from `.env`. Use path-style addressing. The [connection
guide](docs/connection-guide.md) gives AWS CLI, rclone, boto3, DuckDB,
litestream, and barman examples. `armor client-config --for aws-cli --endpoint
http://<armor-host>:9000 --bucket <bucket>` prints the client settings for a
specific endpoint; see the [CLI reference](docs/cli-reference.md).

## Operations

The [documentation index](docs/README.md) links the operator guides. Start
with [key rotation](docs/key-rotation-runbook.md), [format
migration](docs/runbooks/format-migration.md), [disaster
recovery](docs/disaster-recovery.md), and the [release
process](docs/release-process.md). Keep the MEK and B2 credentials available
to recovery tooling, and verify restores with the intended image and object
population.

Legacy v1 objects have the counter defect recorded in [ADR-005](docs/adr/005-ctr-counter-stride-fix.md);
the v2/v3 confidentiality guarantee applies to them only after migration.
Follow the [migration reference](docs/research/migration/V3_Migration_Reference.md)
before running `armor migrate --target v3`, then audit with `armor verify` or
the [continuous restore verifier](docs/restore-verifier-deployment-guide.md).

### Subcommands

`armor help` lists `serve`, `demo`, `check`, `client-config`, `decrypt`,
`verify`, `migrate`, and `version`. The [CLI reference](docs/cli-reference.md)
documents flags, exit codes, credentials, and safe use.

### Admin API

The admin listener exposes health, version, and metrics endpoints. Token-gated
routes cover key management, migration, and other operations. See the [HTTP API
reference](docs/http-api-reference.md) for the route and authorization contract.

## Status and limitations

The repository tests a defined S3 operation and client matrix. Publication
proof, deployment verification, active regressions, and limits on range
performance, multipart/DR checks, and named-key reads are tracked in the
[release-status register](docs/release-status.md). Use that register for the
exact image and deployment boundary before a production rollout.
