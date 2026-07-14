# ARMOR Endpoint URL Documentation

## Task: bf-65daai

Identify and document ARMOR endpoint URLs from configuration files and code.

## Findings

### 1. Default Listen Addresses

**Source:** `internal/config/config.go` (lines 109-110)

```go
Listen:      getEnv("ARMOR_LISTEN", "0.0.0.0:9000"),
AdminListen: getEnv("ARMOR_ADMIN_LISTEN", "127.0.0.0:9001"),
```

**Environment Variables:**
- `ARMOR_LISTEN` - Main S3 API endpoint (default: `0.0.0.0:9000`)
- `ARMOR_ADMIN_LISTEN` - Admin/Dashboard endpoint (default: `127.0.0.1:9001`)

### 2. Kubernetes Service Configuration

**Source:** `deploy/kubernetes/service.yaml`

ARMOR exposes two ports via Kubernetes Service:
- **S3 API:** Port 9000 (ClusterIP service `armor` and `armor-headless`)
- **Admin API:** Port 9001 (ClusterIP service `armor` and `armor-headless`)

### 3. Production Endpoints

#### Cluster-Internal (Kubernetes DNS)

**Source:** declarative-config/k8s/

- **Fully qualified:** `armor.armor.svc.cluster.local:9000`
- **Namespace-local:** `armor:9000` (when accessed from same namespace)

**Examples found in production configs:**
- iad-ci forgejo: `http://armor.armor.svc.cluster.local:9000`
- iad-ci queue-db: `http://armor.armor.svc.cluster.local:9000`
- iad-native-ads: `http://armor.armor.svc.cluster.local:9000`

#### External (Tailscale Ingress)

**Source:** notes/armor-s8k.3.2.2-blocker-summary.md, notes/bf-5m70-completion.md

- **devimprint cluster:** `https://devimprint-armor-tailscale-ingress.tail1b1987.ts.net`

### 4. B2 Backend Endpoint

**Source:** `internal/config/config.go` (lines 119-122)

```go
cfg.B2Endpoint = os.Getenv("ARMOR_B2_ENDPOINT")
if cfg.B2Endpoint == "" {
    cfg.B2Endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", cfg.B2Region)
}
```

**Environment Variable:** `ARMOR_B2_ENDPOINT`
- If not set, defaults to: `https://s3.<region>.backblazeb2.com`

### 5. Dashboard Ingress (External)

**Source:** `deploy/kubernetes/ingress-dashboard.yaml`

- **Example host:** `armor-dashboard.example.com` (placeholder)
- **Port:** 9001 (backend service port)
- **Authentication:** Basic auth optional (commented out in template)

## Summary Table

| Endpoint Type | URL Format | Port | Source |
|---|---|---|---|
| S3 API (default) | `0.0.0.0:9000` | 9000 | config.go:109 |
| Admin API (default) | `127.0.0.1:9001` | 9001 | config.go:110 |
| Cluster-internal (FQDN) | `armor.armor.svc.cluster.local:9000` | 9000 | service.yaml |
| Cluster-internal (short) | `armor:9000` | 9000 | service.yaml |
| External (Tailscale) | `https://devimprint-armor-tailscale-ingress.tail1b1987.ts.net` | 443 | declarative-config |
| B2 Backend | `https://s3.<region>.backblazeb2.com` | 443 | config.go:121 |

## Acceptance Criteria Met

- ✅ ARMOR endpoint URL is located in config and code
- ✅ URL is documented
- ✅ Source of URL (config file, env var, etc.) is identified
