# Build stage
# Builder pin must equal go.mod's `toolchain` directive — with the pin
# matching, the image's go is exactly the declared toolchain and nothing is
# downloaded or substituted at build time. go.mod is the single source of
# this version.
FROM golang:1.25.14-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Staging directory copied into the scratch runtime stages as /tmp. The demo
# subcommand (README "Local demo (Docker only)") builds its temporary
# filesystem backend under os.TempDir(), so the published image must ship a
# writable /tmp or the demo container dies at startup with "failed to create
# temp directory: stat /tmp: no such file or directory".
RUN mkdir -p /image-tmp && chmod 1777 /image-tmp

# Test gate shared with CI. It covers the v3 cryptographic primitives,
# multipart state, canary, concurrent HTTP round trips, and integration-suite
# compilation without requiring live B2 credentials.
RUN CGO_ENABLED=0 ./scripts/release-gate.sh

# Build the main armor binary
ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X github.com/jedarden/armor/internal/version.Version=${VERSION}" -o /armor ./cmd/armor

# Build the restore-verifier binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X github.com/jedarden/armor/internal/version.Version=${VERSION}" -o /restore-verifier ./cmd/restore-verifier

# Build the armor-fleet binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X github.com/jedarden/armor/internal/version.Version=${VERSION}" -o /armor-fleet ./cmd/armor-fleet

# Canonical bead-rs CLI for the restore-verifier image (ADR-004 §5 escalation).
# Pinned release binary from the public jedarden/bead-rs mirror, checksum-
# verified at build time — a mismatch fails the build. Kept in its own stage so
# the checksum/download layer caches independently of code changes. The release
# asset is x86_64 (the only architecture kaniko builds for this repo).
FROM golang:1.25.14-alpine AS bead-cli
ARG BEAD_VERSION=v0.2.6
ARG BEAD_CLI_SHA256=15324894af38a8ffce8ad54da47ac067c7fd198779a7744f967bcf56a9aa3aee
RUN wget -q -O /usr/local/bin/bead \
      "https://github.com/jedarden/bead-rs/releases/download/${BEAD_VERSION}/bead-x86_64-unknown-linux-gnu" \
    && echo "${BEAD_CLI_SHA256}  /usr/local/bin/bead" | sha256sum -c - \
    && chmod 0755 /usr/local/bin/bead

# Runtime stage for restore-verifier.
# Built only with an explicit --target restore-verifier-runtime; it must NOT
# be the last stage — an untargeted build produces the final stage, and the
# published ronaldraygun/armor image must be the armor server. (Images
# 0.1.1833–0.1.1870 shipped /restore-verifier as the entrypoint because this
# stage sat last; deployed pods crash-looped with restore-verifier's
# credential error.)
#
# Base is debian:bookworm-slim, not scratch: escalation (VERIFIER_ESCALATION)
# shells out to the canonical bead-rs CLI copied in above, and that release
# binary is glibc-dynamic (libgcc_s/libm/libc) — it cannot run on scratch, and
# alpine's musl would need gcompat. A real base also leaves a shell for
# operator triage (`bead list` inside the pod's escalation workspace).
FROM debian:bookworm-slim AS restore-verifier-runtime

# Copy CA certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Canonical bead-rs CLI: escalation filings (--unique-ref deduped creates)
# and startup validation (`bead --version` / `bead init` / `bead list`).
COPY --from=bead-cli /usr/local/bin/bead /usr/local/bin/bead

# Copy the binary
COPY --from=builder /restore-verifier /restore-verifier

# Expose ports
EXPOSE 9002

# Set entrypoint
ENTRYPOINT ["/restore-verifier"]

# Runtime stage for armor-fleet
FROM scratch AS armor-fleet-runtime

# Copy CA certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /armor-fleet /armor-fleet

# Expose ports
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/armor-fleet"]

# Runtime stage for armor — final stage = default build target
FROM scratch AS armor-runtime

# Copy CA certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# /tmp for the demo subcommand's temporary filesystem backend (see the
# /image-tmp staging note in the builder stage).
COPY --from=builder /image-tmp /tmp

# Copy the binary
COPY --from=builder /armor /armor

# Expose ports
EXPOSE 9000 9001

# Set entrypoint
ENTRYPOINT ["/armor"]
