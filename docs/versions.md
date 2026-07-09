# ARMOR Version Inventory

This document tracks all dependency versions, minimum requirements, and development tool versions for the ARMOR project.

**Last Updated**: 2026-07-09

---

## Go Runtime

| Component | Version | Requirement |
|-----------|---------|-------------|
| Go | 1.25.0 | Minimum: 1.25.0 |

**Installation Check**:
```bash
go version
# Output: go version go1.25.0 linux/amd64
```

---

## Runtime Dependencies (Direct)

| Package | Version | Purpose | Minimum Version |
|---------|---------|---------|-----------------|
| `github.com/aws/aws-sdk-go-v2` | v1.41.4 | AWS S3 API compatibility | v1.41.4 |
| `github.com/aws/aws-sdk-go-v2/config` | v1.32.12 | AWS configuration loading | v1.32.12 |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.19.12 | AWS credential management | v1.19.12 |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.97.2 | S3 service implementation | v1.97.2 |
| `github.com/kurin/blazer` | v0.5.3 | Backblaze B2 client | v0.5.3 |
| `golang.org/x/crypto` | v0.49.0 | Cryptographic primitives (HMAC, SHA256) | v0.49.0 |
| `golang.org/x/sync` | v0.12.0 | Concurrency utilities | v0.12.0 |

---

## Runtime Dependencies (Indirect)

AWS SDK v2 internal modules (all indirect dependencies):

| Package | Version |
|---------|---------|
| `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` | v1.7.8 |
| `github.com/aws/aws-sdk-go-v2/feature/ec2/imds` | v1.18.20 |
| `github.com/aws/aws-sdk-go-v2/internal/configsources` | v1.4.20 |
| `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2` | v2.7.20 |
| `github.com/aws/aws-sdk-go-v2/internal/ini` | v1.8.6 |
| `github.com/aws/aws-sdk-go-v2/internal/v4a` | v1.4.21 |
| `github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding` | v1.13.7 |
| `github.com/aws/aws-sdk-go-v2/service/internal/checksum` | v1.9.12 |
| `github.com/aws/aws-sdk-go-v2/service/internal/presigned-url` | v1.13.20 |
| `github.com/aws/aws-sdk-go-v2/service/internal/s3shared` | v1.19.20 |
| `github.com/aws/aws-sdk-go-v2/service/signin` | v1.0.8 |
| `github.com/aws/aws-sdk-go-v2/service/sso` | v1.30.13 |
| `github.com/aws/aws-sdk-go-v2/service/ssooidc` | v1.35.17 |
| `github.com/aws/aws-sdk-go-v2/service/sts` | v1.41.9 |
| `github.com/aws/smithy-go` | v1.24.2 |

Other indirect dependencies:

| Package | Version |
|---------|---------|
| `golang.org/x/net` | v0.51.0 |
| `golang.org/x/sys` | v0.42.0 |
| `golang.org/x/term` | v0.41.0 |
| `golang.org/x/text` | v0.35.0 |
| `gopkg.in/check.v1` | v0.0.0-20161208181325-20d25e280405 |
| `gopkg.in/yaml.v3` | v3.0.1 |

---

## Development Tools

### Build & Container Tools

| Tool | Version | Purpose | Requirement |
|------|---------|---------|-------------|
| Docker | 27.5.1 | Container image building and deployment | >= 20.0 |
| Git | 2.50.1 | Version control | >= 2.0 |
| Go | 1.25.0 | Build toolchain | >= 1.25.0 |

### Kubernetes Deployment Tools

| Tool | Version | Purpose | Requirement |
|------|---------|---------|-------------|
| kubectl | v1.33.3 | Kubernetes cluster management | >= 1.20 |
| Kustomize | v5.6.0 | Kubernetes resource customization | v5.x |

**Note**: kubectl includes integrated Kustomize support.

### Testing Tools

| Tool | Version | Purpose |
|------|---------|---------|
| `go test` | Built-in | Unit and integration testing |
| `gopkg.in/check.v1` | v0.0.0-20161208181325-20d25e280405 | Test helpers (indirect dependency) |

### Linting & Code Quality

| Tool | Status | Notes |
|------|--------|-------|
| golangci-lint | Not installed locally | Can be added for CI/CD pipeline |
| go fmt | Built-in | Standard Go formatting |
| go vet | Built-in | Static analysis |

---

## Deployment Configuration

### Docker Images

| Image | Registry | Current Version |
|-------|----------|-----------------|
| `ronaldraygun/armor` | Docker Hub | `:0.1.43` |

**Image Tagging Strategy**:
- Auto-bumped on CI/CD build
- Production deployments should pin to specific version tags
- Latest tags available at: https://hub.docker.com/r/ronaldraygun/armor/tags

### Kubernetes Resources

ARMOR deploys to Kubernetes with the following resource types:
- `Deployment` (Stateless)
- `Service` (ClusterIP and LoadBalancer)
- `Secret` (Environment variables and auth)
- `Ingress` (External access)

Kubernetes manifests are located in `deploy/kubernetes/` and managed via Kustomize.

---

## CI/CD Integration

ARMOR is built and deployed via Argo Workflows in the `iad-ci` cluster.

### Workflow Template

| Template | Repository | Output |
|----------|------------|--------|
| `armor-build` | `jedarden/armor` | Docker image → `ronaldraygun/armor:<version>` |

### Build Process

1. Checkout source code
2. Run Go tests
3. Build Docker image
4. Push to Docker Hub
5. Update `VERSION` file
6. Create GitHub release (if applicable)

---

## Environment-Specific Requirements

### Development Environment

- **Go 1.25.0+** - Required for building and testing
- **Docker 27.5+** - For local container builds
- **Git 2.50+** - For version control operations
- **Backblaze B2 account** - For integration testing (optional)

### Production Environment

- **Kubernetes 1.20+** - Minimum cluster version
- **Backblaze B2** - Object storage backend
- **Cloudflare account** - Zero-egress routing via Bandwidth Alliance
- **Valid TLS certificates** - For secure client connections

---

## Version Compatibility Matrix

| Go Version | ARMOR Version | Status |
|------------|---------------|--------|
| 1.25.0 | 0.1.375 | ✅ Current |
| 1.24.x | 0.1.375 | ✅ Compatible |
| < 1.24.0 | 0.1.375 | ❌ Unsupported |

---

## Dependency Security

### Cryptographic Dependencies

ARMOR relies on the following cryptographic primitives:

- **AES-256-CTR** - Via `golang.org/x/crypto` for file encryption
- **HMAC-SHA256** - Per-block integrity verification
- **SHA-256** - Plaintext integrity checksums
- **Envelope encryption** - Key wrapping via standard Go crypto packages

All cryptographic operations use standard library and `golang.org/x/crypto` implementations.

### Security Notes

- DEKs (Data Encryption Keys) are randomly generated per file using `crypto/rand`
- MEKs (Master Encryption Keys) are user-provided 32-byte hex strings
- Wrapped DEKs are stored in B2 object metadata (never with the file itself)
- HMACs are computed independently for each 64KB block

---

## Minimum Required Versions

For ARMOR to function correctly, the following minimum versions must be met:

| Component | Minimum Version | Rationale |
|-----------|-----------------|-----------|
| Go | 1.25.0 | Language features and standard library APIs |
| Docker | 20.0 | Multi-stage builds and layer caching |
| kubectl | 1.20 | Kustomize integration and API compatibility |
| Kubernetes | 1.20 | Custom Resource Definitions and APIs |

---

## Generating This Document

To regenerate this document with updated versions:

```bash
# Update Go dependencies
go mod tidy

# List all dependencies
go list -m all > /tmp/deps.txt

# Check tool versions
go version
docker --version
git --version
kubectl version --client

# Update this document with the new versions
```

---

## References

- [ARMOR README](../README.md) - Project overview and architecture
- [Go Module Documentation](https://golang.org/ref/mod) - Go dependency management
- [AWS SDK Go v2](https://aws.github.io/aws-sdk-go-v2/docs/) - AWS SDK documentation
- [Kurin/blazer](https://github.com/kurin/blazer) - Backblaze B2 Go client
- [Cloudflare Bandwidth Alliance](https://www.cloudflare.com/bandwidth-alliance/) - Zero-egress partners

---

*This document is maintained as part of the ARMOR project. For questions or updates, please refer to the project repository.*
