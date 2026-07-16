#!/usr/bin/env python3
"""
Discover and catalog ARMOR deployments across all clusters.

Scans jedarden/declarative-config for armor-deployment.yml/yaml files
and extracts the current image tags from each deployment.
"""

import json
import re
import sys
from pathlib import Path
from typing import Dict, List, Optional


def find_armor_deployment_files(declarative_config_path: Path) -> List[Path]:
    """Find all armor-deployment.yml/yaml files in the declarative-config."""
    deployment_files = []

    # Look for files matching armor-deployment.yml or armor-deployment.yaml
    # Also handle variations like acb-armor-deployment.yml
    for pattern in ['**/armor-deployment.yml', '**/armor-deployment.yaml', '**/*armor-deployment.yml', '**/*armor-deployment.yaml']:
        deployment_files.extend(declarative_config_path.glob(pattern))

    # Filter to k8s directory only
    k8s_path = declarative_config_path / 'k8s'
    deployment_files = [f for f in deployment_files if k8s_path in f.parents or f.parent == k8s_path]

    return sorted(set(deployment_files))


def extract_cluster_from_path(file_path: Path, k8s_path: Path) -> str:
    """Extract cluster name from file path."""
    try:
        # Get the relative path from k8s/ directory
        rel_path = file_path.relative_to(k8s_path)
        # First component should be the cluster name
        return rel_path.parts[0]
    except (ValueError, IndexError):
        return "unknown"


def extract_image_tag_from_yaml(yaml_content: str) -> Optional[str]:
    """Extract the image tag from a Kubernetes deployment YAML."""
    # Look for the image line in the deployment
    # Pattern: image: ronaldraygun/armor:TAG
    match = re.search(r'image:\s*ronaldraygun/armor:([^\s]+)', yaml_content)
    if match:
        return match.group(1)
    return None


def parse_deployment_file(file_path: Path) -> Optional[Dict]:
    """Parse an armor-deployment file and extract metadata."""
    try:
        with open(file_path, 'r') as f:
            content = f.read()

        image_tag = extract_image_tag_from_yaml(content)
        if not image_tag:
            print(f"Warning: Could not extract image tag from {file_path}", file=sys.stderr)
            return None

        return {
            'file_path': str(file_path),
            'image_tag': image_tag,
            'full_image': f'ronaldraygun/armor:{image_tag}'
        }
    except Exception as e:
        print(f"Error parsing {file_path}: {e}", file=sys.stderr)
        return None


def main():
    """Main entry point."""
    # Path to declarative-config
    declarative_config_path = Path.home() / 'declarative-config'
    k8s_path = declarative_config_path / 'k8s'

    if not k8s_path.exists():
        print(f"Error: {k8s_path} does not exist", file=sys.stderr)
        sys.exit(1)

    # Find all armor-deployment files
    deployment_files = find_armor_deployment_files(declarative_config_path)

    if not deployment_files:
        print("No armor-deployment files found", file=sys.stderr)
        sys.exit(1)

    # Parse each file
    deployments = {}
    for file_path in deployment_files:
        cluster = extract_cluster_from_path(file_path, k8s_path)
        deployment_info = parse_deployment_file(file_path)

        if deployment_info:
            # Use cluster as key, but handle duplicates (multiple files in same cluster)
            if cluster not in deployments:
                deployments[cluster] = []
            deployments[cluster].append(deployment_info)

    # Create output structure
    output = {
        'scan_date': '2026-07-16',
        'declarative_config_path': str(declarative_config_path),
        'clusters_found': list(deployments.keys()),
        'deployments_by_cluster': deployments
    }

    # Also create a simplified mapping
    simplified_mapping = {}
    for cluster, dep_list in deployments.items():
        if len(dep_list) == 1:
            simplified_mapping[cluster] = dep_list[0]['image_tag']
        else:
            # Multiple deployments in same cluster
            for i, dep in enumerate(dep_list):
                key = f"{cluster}_{i}" if i > 0 else cluster
                simplified_mapping[key] = dep['image_tag']

    output['simplified_mapping'] = simplified_mapping

    # Output JSON
    print(json.dumps(output, indent=2))


if __name__ == '__main__':
    main()
