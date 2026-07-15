# ARMOR Connection Guide

This guide explains how to connect to the ARMOR S3-compatible endpoint for use in scripts and applications.

## Quick Reference

| Property | Value |
|----------|-------|
| **Protocol** | HTTP (S3 API) / HTTP (Admin API) |
| **S3 Port** | 9000 (default) |
| **Admin Port** | 9001 (default) |
| **Authentication** | AWS Signature V4 |
| **API Compatibility** | S3-compatible |

## Endpoints

### S3 API Endpoint (Primary)
- **Port**: 9000 (default)
- **URL**: `http://<host>:9000`
- **Purpose**: S3-compatible operations (PutObject, GetObject, ListBuckets, etc.)
- **Authentication**: AWS SigV4 required

### Admin API Endpoint
- **Port**: 9001 (default)
- **URL**: `http://<host>:9001`
- **Purpose**: Key management, metrics, health checks
- **Authentication**: Varies by endpoint (see below)

### Health Check Endpoint
- **Path**: `/healthz`
- **Method**: GET
- **Authentication**: None (public)
- **Response**: `OK` on success

## Authentication Methods

### 1. AWS Signature V4 (S3 API)

ARMOR uses AWS Signature Version 4 for authentication, compatible with standard S3 clients.

#### Environment Variables

```bash
export ARMOR_ENDPOINT=http://localhost:9000
export ARMOR_ACCESS_KEY=your-access-key
export ARMOR_SECRET_KEY=your-secret-key
export ARMOR_REGION=us-west-002  # Or your configured region
```

#### Using curl with AWS SigV4

For quick testing without a full S3 client, you can use pre-signed requests or raw HTTP:

```bash
# Health check (no auth required)
curl http://localhost:9000/healthz

# List buckets (requires auth - use AWS CLI or boto3 for proper SigV4 signing)
```

### 2. AWS CLI

Configure AWS CLI to point to ARMOR:

```bash
# Create a named profile
aws configure --profile armor

# When prompted:
# AWS Access Key ID: <your-access-key>
# AWS Secret Access Key: <your-secret-key>
# Default region name: us-west-002
# Default output format: json

# Set the endpoint URL per command
aws --profile armor --endpoint-url http://localhost:9000 s3 ls

# Or set it in environment
export AWS_ENDPOINT_URL=http://localhost:9000
aws --profile armor s3 ls
```

### 3. Python boto3

```python
import boto3
import os

# Create S3 client pointing to ARMOR
s3 = boto3.client(
    's3',
    endpoint_url=os.getenv('ARMOR_ENDPOINT', 'http://localhost:9000'),
    aws_access_key_id=os.getenv('ARMOR_ACCESS_KEY'),
    aws_secret_access_key=os.getenv('ARMOR_SECRET_KEY'),
    region_name=os.getenv('ARMOR_REGION', 'us-west-002')
)

# List buckets
response = s3.list_buckets()
for bucket in response['Buckets']:
    print(f"Bucket: {bucket['Name']}")

# Upload a file
s3.upload_file('local.txt', 'my-bucket', 'remote-key.txt')

# Download a file
s3.download_file('my-bucket', 'remote-key.txt', 'local-downloaded.txt')
```

## Connection Examples

### Example 1: Simple Health Check

```bash
#!/bin/bash
# Simple health check script

ENDPOINT="http://localhost:9000"

if curl -sf "${ENDPOINT}/healthz" | grep -q "OK"; then
    echo "ARMOR is healthy"
    exit 0
else
    echo "ARMOR health check failed"
    exit 1
fi
```

### Example 2: AWS CLI Operations

```bash
#!/bin/bash
# ARMOR operations using AWS CLI

set -e

ENDPOINT="http://localhost:9000"
BUCKET="my-bucket"
FILE="test.txt"

# Create bucket
aws --endpoint-url "$ENDPOINT" s3 mb "s3://$BUCKET"

# List buckets
aws --endpoint-url "$ENDPOINT" s3 ls

# Upload file
echo "Hello ARMOR" > "$FILE"
aws --endpoint-url "$ENDPOINT" s3 cp "$FILE" "s3://$BUCKET/$FILE"

# List objects
aws --endpoint-url "$ENDPOINT" s3 ls "s3://$BUCKET/"

# Download file
aws --endpoint-url "$ENDPOINT" s3 cp "s3://$BUCKET/$FILE" "downloaded-$FILE"

# Clean up
aws --endpoint-url "$ENDPOINT" s3 rm "s3://$BUCKET/$FILE"
```

### Example 3: Python with boto3

```python
#!/usr/bin/env python3
"""
ARMOR connection test with boto3
"""

import boto3
import os

def connect_to_armor():
    """Create a boto3 client for ARMOR"""
    return boto3.client(
        's3',
        endpoint_url=os.getenv('ARMOR_ENDPOINT', 'http://localhost:9000'),
        aws_access_key_id=os.getenv('ARMOR_ACCESS_KEY'),
        aws_secret_access_key=os.getenv('ARMOR_SECRET_KEY'),
        region_name=os.getenv('ARMOR_REGION', 'us-west-002')
    )

def main():
    s3 = connect_to_armor()

    # Health check
    try:
        # HTTP health check (no auth)
        import requests
        resp = requests.get(f"{os.getenv('ARMOR_ENDPOINT', 'http://localhost:9000')}/healthz", timeout=5)
        if resp.status_code == 200:
            print("✓ ARMOR health check passed")
    except Exception as e:
        print(f"✗ Health check failed: {e}")
        return

    # List buckets
    try:
        response = s3.list_buckets()
        print(f"✓ Found {len(response['Buckets'])} bucket(s)")
        for bucket in response['Buckets']:
            print(f"  - {bucket['Name']}")
    except Exception as e:
        print(f"✗ Failed to list buckets: {e}")

if __name__ == '__main__':
    main()
```

### Example 4: DuckDB Integration

ARMOR supports DuckDB for querying encrypted Parquet files:

```sql
-- Install and load httpfs extension
INSTALL httpfs;
LOAD httpfs;

-- Configure S3 endpoint
SET s3_endpoint='localhost:9000';
SET s3_access_key_id='your-access-key';
SET s3_secret_access_key='your-secret-key';
SET s3_region='us-west-002';
SET s3_use_ssl=false;

-- Query encrypted Parquet file
SELECT * FROM read_parquet('s3://my-bucket/data.parquet');

-- List objects (experimental)
SELECT * FROM glob('s3://my-bucket/*.parquet');
```

## Environment Setup

### Required Environment Variables

```bash
# S3 API Configuration
export ARMOR_ENDPOINT=http://localhost:9000
export ARMOR_ACCESS_KEY=your-access-key
export ARMOR_SECRET_KEY=your-secret-key
export ARMOR_REGION=us-west-002

# Optional: Default bucket
export ARMOR_BUCKET=my-bucket
```

### Docker Compose Example

```yaml
version: '3.8'
services:
  armor:
    image: ronaldraygun/armor:0.1.43
    ports:
      - "9000:9000"  # S3 API
      - "9001:9001"  # Admin API
    environment:
      - ARMOR_B2_REGION=us-east-005
      - ARMOR_B2_ACCESS_KEY_ID=your-b2-key-id
      - ARMOR_B2_SECRET_ACCESS_KEY=your-b2-secret
      - ARMOR_BUCKET=your-b2-bucket
      - ARMOR_CF_DOMAIN=your-cf-domain.example.com
      - ARMOR_MEK=your-master-encryption-key
      - ARMOR_AUTH_ACCESS_KEY=my-access-key
      - ARMOR_AUTH_SECRET_KEY=my-secret-key
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Kubernetes Service Example

ARMOR in Kubernetes is exposed via a ClusterIP service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: armor
spec:
  type: ClusterIP
  ports:
  - name: s3-api
    port: 9000
    targetPort: 9000
  selector:
    app: armor
```

Connect from within the cluster:

```bash
export ARMOR_ENDPOINT=http://armor:9000
aws --endpoint-url "$ARMOR_ENDPOINT" s3 ls
```

## Troubleshooting

### Connection Refused

```bash
# Check if ARMOR is running
curl http://localhost:9000/healthz

# If using Docker, check container status
docker ps | grep armor

# Check logs
docker logs <container-id>
```

### Authentication Failures

```bash
# Verify credentials
echo $ARMOR_ACCESS_KEY
echo $ARMOR_SECRET_KEY

# Test with AWS CLI verbose mode
aws --endpoint-url http://localhost:9000 --debug s3 ls
```

### Timeout Errors

```bash
# Check network connectivity
ping localhost
telnet localhost 9000

# Increase timeout for large operations
aws --endpoint-url http://localhost:9000 s3 cp file.txt s3://bucket/file.txt \
    --cli-connect-timeout 60 \
    --cli-read-timeout 300
```

## Advanced Configuration

### Multi-Credential Setup

ARMOR supports multiple credential sets with ACLs:

```bash
export ARMOR_AUTH_READONLY_ACCESS_KEY=reader-key
export ARMOR_AUTH_READONLY_SECRET_KEY=reader-secret
export ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*"

export ARMOR_AUTH_WRITER_ACCESS_KEY=writer-key
export ARMOR_AUTH_WRITER_SECRET_KEY=writer-secret
export ARMOR_AUTH_WRITER_ACL="mybucket:*"
```

### Admin API Authentication

The admin API (port 9001) may require additional authentication:

```bash
# Dashboard authentication (if configured)
export ARMOR_DASHBOARD_USER=admin
export ARMOR_DASHBOARD_PASS=secret

# Or bearer token authentication
export ARMOR_DASHBOARD_TOKEN=your-token
```

### TLS/SSL Configuration

For production deployments, configure TLS termination at a reverse proxy (nginx, Traefik, etc.):

```nginx
server {
    listen 443 ssl;
    server_name armor.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## See Also

- [README.md](../README.md) - ARMOR overview and architecture
- [Cloudflare Setup](./cloudflare-setup.md) - Zero-egress download configuration
- [Disaster Recovery](./disaster-recovery.md) - Backup and restore procedures
- [Integration Tests](../tests/integration/README.md) - Testing against real B2 + Cloudflare
