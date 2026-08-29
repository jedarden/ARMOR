#!/usr/bin/env python3
"""
Scan jedarden/declarative-config for all armor-deployment.yml files and extract
deployed ARMOR image tags per cluster.
"""

import os
import json
import re
import sys
from pathlib import Path


def extract_image_info(yaml_content: str) -> tuple[str | None, str | None]:
    """Extract image name and tag from a Kubernetes deployment YAML.

    Returns:
        tuple: (image_type, tag) where image_type is like 'armor', 'armor-restore-verifier', etc.
               Returns (None, None) if no armor* image found.
    """
    # Match any ronaldraygun/armor* image (armor, armor-restore-verifier, armor-fleet, etc.)
    match = re.search(r'image:\s+ronaldraygun/(armor[^\s:]*):([^\s]+)', yaml_content)
    if match:
        image_type = match.group(1)
        tag = match.group(2)
        return (image_type, tag)
    return (None, None)


def extract_cluster_from_path(filepath: str) -> str:
    """Extract cluster name from file path."""
    # Path pattern: .../k8s/<cluster>/...
    parts = Path(filepath).parts
    try:
        k8s_idx = parts.index('k8s')
        if k8s_idx + 1 < len(parts):
            return parts[k8s_idx + 1]
    except ValueError:
        pass
    return "unknown"


def find_armor_deployments(declarative_config_path: str) -> list[dict]:
    """
    Scan declarative-config for all ARMOR deployments and extract
    deployment information.
    """
    deployments = []
    k8s_path = os.path.join(declarative_config_path, 'k8s')

    if not os.path.exists(k8s_path):
        print(f"Warning: k8s directory not found at {k8s_path}", file=sys.stderr)
        return deployments

    # Walk the k8s directory recursively
    for root, dirs, files in os.walk(k8s_path):
        for filename in files:
            if not filename.endswith('.yml') and not filename.endswith('.yaml'):
                continue

            filepath = os.path.join(root, filename)
            try:
                with open(filepath, 'r') as f:
                    content = f.read()

                # Check if this is an ARMOR deployment
                if 'ronaldraygun/armor' not in content:
                    continue

                image_type, image_tag = extract_image_info(content)
                if image_type and image_tag:
                    cluster = extract_cluster_from_path(filepath)
                    deployments.append({
                        'cluster': cluster,
                        'image_type': image_type,
                        'image_tag': image_tag,
                        'filepath': filepath
                    })
                else:
                    print(f"Warning: No image tag found in {filepath}", file=sys.stderr)
            except Exception as e:
                # Silently skip files that can't be read
                pass

    return deployments


def main():
    # Default path to declarative-config
    declarative_config_path = os.path.expanduser('~/declarative-config')

    # Allow override via command line argument
    if len(sys.argv) > 1:
        declarative_config_path = sys.argv[1]

    if not os.path.exists(declarative_config_path):
        print(f"Error: declarative-config not found at {declarative_config_path}", file=sys.stderr)
        sys.exit(1)

    deployments = find_armor_deployments(declarative_config_path)

    # Sort by cluster name for consistent output
    deployments.sort(key=lambda d: d['cluster'])

    # Output as JSON
    print(json.dumps(deployments, indent=2))

    # Print summary
    print(f"\nFound {len(deployments)} ARMOR deployment(s)", file=sys.stderr)

    return 0


if __name__ == '__main__':
    sys.exit(main())
