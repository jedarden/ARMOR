# ARMOR Installed Dependencies and Tools

## Generated
2026-07-09

## Go Environment
- **Go Version**: go1.25.0 linux/amd64
- **Go Module**: github.com/jedarden/armor

## Go Dependencies

### Direct Dependencies
| Package | Version |
|---------|---------|
| github.com/aws/aws-sdk-go-v2 | v1.41.4 |
| github.com/aws/aws-sdk-go-v2/config | v1.32.12 |
| github.com/aws/aws-sdk-go-v2/credentials | v1.19.12 |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.97.2 |
| github.com/kurin/blazer | v0.5.3 |
| golang.org/x/crypto | v0.49.0 |
| golang.org/x/sync | v0.12.0 |

### Transitive Dependencies (AWS SDK v2 Ecosystem)
| Package | Version |
|---------|---------|
| github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream | v1.7.8 |
| github.com/aws/aws-sdk-go-v2/feature/ec2/imds | v1.18.20 |
| github.com/aws/aws-sdk-go-v2/internal/configsources | v1.4.20 |
| github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 | v2.7.20 |
| github.com/aws/aws-sdk-go-v2/internal/ini | v1.8.6 |
| github.com/aws/aws-sdk-go-v2/internal/v4a | v1.4.21 |
| github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding | v1.13.7 |
| github.com/aws/aws-sdk-go-v2/service/internal/checksum | v1.9.12 |
| github.com/aws/aws-sdk-go-v2/service/internal/presigned-url | v1.13.20 |
| github.com/aws/aws-sdk-go-v2/service/internal/s3shared | v1.19.20 |
| github.com/aws/aws-sdk-go-v2/service/signin | v1.0.8 |
| github.com/aws/aws-sdk-go-v2/service/sso | v1.30.13 |
| github.com/aws/aws-sdk-go-v2/service/ssooidc | v1.35.17 |
| github.com/aws/aws-sdk-go-v2/service/sts | v1.41.9 |
| github.com/aws/smithy-go | v1.24.2 |

### Transitive Dependencies (golang.org/x)
| Package | Version |
|---------|---------|
| golang.org/x/net | v0.51.0 |
| golang.org/x/sys | v0.42.0 |
| golang.org/x/term | v0.41.0 |
| golang.org/x/text | v0.35.0 |

## Development Tools

### Build Tools
| Tool | Version |
|------|---------|
| Docker | 27.5.1 |
| Make | GNU Make 4.4.1 |
| Git | 2.50.1 |
| GCC | GCC 13.3.0 |

### Build Configuration
- **Docker Base Image**: golang:1.25-alpine
- **Build Target**: Linux (CGO_ENABLED=0)
- **Build Flags**: -ldflags="-s -w" (strip debug info)

### Container Build Dependencies
From Dockerfile (Alpine-based):
- git
- ca-certificates
- tzdata

## Cloud CLI Tools
- **kubectl**: Not found on host
- **gcloud**: Not found on host  
- **aws-cli**: Not found on host

## Summary

**Total Go Dependencies**: 24 packages
- 7 direct dependencies
- 17 transitive dependencies

**Primary Technology Stack**:
- Go 1.25.0
- AWS SDK v2 for S3 operations
- Google Cloud Storage (via kurin/blazer)
- Docker-based builds

**Security Notes**:
- Project uses CGO_ENABLED=0 for static binaries
- Alpine Linux base image for minimal attack surface
- All cloud operations handled via SDKs (no CLI tools in runtime)
