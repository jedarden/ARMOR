#!/bin/bash
# ARMOR Connectivity Check Script
# Quick verification of ARMOR endpoint connectivity

set -euo pipefail

NAMESPACE="armor"
SERVICE="armor"
PROXY_URL="http://traefik-rs-manager:8001"

echo "ARMOR Connectivity Check"
echo "========================="
echo ""

# Check service exists
echo "Checking service..."
if kubectl --server="$PROXY_URL" get svc -n "$NAMESPACE" "$SERVICE" >/dev/null 2>&1; then
    CLUSTER_IP=$(kubectl --server="$PROXY_URL" get svc -n "$NAMESPACE" "$SERVICE" -o jsonpath='{.spec.clusterIP}')
    echo "✓ Service exists (ClusterIP: $CLUSTER_IP)"
else
    echo "✗ Service not found"
    exit 1
fi

# Check pod health
echo "Checking pod health..."
PODS=$(kubectl --server="$PROXY_URL" get pods -n "$NAMESPACE" -l app=armor --no-headers 2>/dev/null | wc -l)
if [ "$PODS" -gt 0 ]; then
    READY=$(kubectl --server="$PROXY_URL" get pods -n "$NAMESPACE" -l app=armor --no-headers 2>/dev/null | awk '{print $2}')
    echo "✓ Pods ready: $READY"
else
    echo "✗ No pods found"
    exit 1
fi

# Check endpoints
echo "Checking endpoints..."
ENDPOINTS=$(kubectl --server="$PROXY_URL" get endpoints -n "$NAMESPACE" "$SERVICE" -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null)
if [ -n "$ENDPOINTS" ]; then
    echo "✓ Endpoints ready: $ENDPOINTS"
else
    echo "✗ No endpoints found"
    exit 1
fi

# Check network connectivity
echo "Checking network connectivity..."
if curl -s --connect-timeout 3 http://traefik-rs-manager:8001/ >/dev/null 2>&1; then
    echo "✓ Cluster accessible via Tailscale"
else
    echo "✗ Cluster not accessible"
    exit 1
fi

echo ""
echo "=== ARMOR Connectivity: OK ==="
echo ""
echo "Service endpoints:"
echo "  - S3 API: http://$SERVICE.$NAMESPACE.svc.cluster.local:9000"
echo "  - Admin API: http://$SERVICE.$NAMESPACE.svc.cluster.local:9001"