# ARMOR Deployment Discovery Script (bf-woipic)

## Summary

Created a Python script to scan jedarden/declarative-config for all ARMOR deployments and extract their image tags per cluster.

## Script Location

`/home/coding/ARMOR/scripts/find-armor-deployments.py`

## Usage

```bash
# Run with default path (~/declarative-config)
./scripts/find-armor-deployments.py

# Run with custom path
./scripts/find-armor-deployments.py /path/to/declarative-config
```

## Output

The script outputs JSON to stdout:

```json
[
  {
    "cluster": "iad-acb",
    "image_tag": "fcbf6d3",
    "filepath": "/home/coding/declarative-config/k8s/iad-acb/ai-code-battle/acb-armor-deployment.yml"
  },
  {
    "cluster": "iad-ci",
    "image_tag": "0.1.24",
    "filepath": "/home/coding/declarative-config/k8s/iad-ci/armor/armor-deployment.yaml"
  },
  ...
]
```

## Discoveries

Found 6 ARMOR deployments across 5 clusters:

| Cluster | Image Tag | Purpose |
|---------|-----------|---------|
| iad-acb | fcbf6d3 | AI Code Battle PostgreSQL backup proxy |
| iad-ci | 0.1.24 | CI cluster ARMOR instance |
| iad-kalshi | 0.1.13 | Kalshi weather workloads |
| iad-native-ads | 0.1.42 | Native ads pipeline |
| ord-devimprint | 0.1.42 | Devimprint B2 bucket proxy |
| rs-manager | 0.1.13 | Rackspace Spot manager cluster |

## Notes

- apexalgo-iad has no ARMOR deployment (only ExternalSecret for acb-armor-credentials)
- Script searches for any YAML file with `kind: Deployment` and `ronaldraygun/armor:` image
- Gracefully handles missing/unreadable files
- Output is sorted by cluster name for consistency
