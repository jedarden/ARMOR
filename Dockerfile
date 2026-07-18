# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Test gate: run go vet and unit tests before building
# Integration tests (tests/integration/) require build tags and credentials,
# and are automatically skipped by this gate.
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short

# Build the main armor binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /armor ./cmd/armor

# Build the restore-verifier binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /restore-verifier ./cmd/restore-verifier

# Runtime stage for restore-verifier.
# Built only with an explicit --target restore-verifier-runtime; it must NOT
# be the last stage — an untargeted build produces the final stage, and the
# published ronaldraygun/armor image must be the armor server. (Images
# 0.1.1833–0.1.1870 shipped /restore-verifier as the entrypoint because this
# stage sat last; deployed pods crash-looped with restore-verifier's
# credential error.)
FROM scratch AS restore-verifier-runtime

# Copy CA certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /restore-verifier /restore-verifier

# Expose ports
EXPOSE 9002

# Set entrypoint
ENTRYPOINT ["/restore-verifier"]

# Runtime stage for armor — final stage = default build target
FROM scratch AS armor-runtime

# Copy CA certificates and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /armor /armor

# Expose ports
EXPOSE 9000 9001

# Set entrypoint
ENTRYPOINT ["/armor"]
