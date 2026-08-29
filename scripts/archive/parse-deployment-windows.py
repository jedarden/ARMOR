#!/usr/bin/env python3
"""
Parse ARMOR deployment windows from drift inventory.

Extracts deployment timestamps for affected multipart-era ARMOR versions
and defines time windows for each deployment.
"""

import json
import re
import subprocess
from datetime import datetime, timedelta
from pathlib import Path
from typing import Dict, List, Any

# Bug window from corruption inventory
BUG_WINDOW_START = "2026-03-24T00:00:00Z"
BUG_WINDOW_END = "2026-07-16T23:59:59Z"


def get_deployment_history() -> List[Dict[str, Any]]:
    """Extract ARMOR deployment history from declarative-config git log."""
    try:
        result = subprocess.run(
            [
                "git", "log", "--all", "--oneline", "--date=iso-strict",
                "--format=%H|%ad|%s", "--",
                "k8s/*/armor/*.yml", "k8s/*/armor/*.yaml"
            ],
            cwd="/home/coding/declarative-config",
            capture_output=True,
            text=True,
            check=True
        )

        deployments = []
        for line in result.stdout.strip().split('\n'):
            if not line:
                continue

            parts = line.split('|')
            if len(parts) != 3:
                continue

            commit_hash, date_str, message = parts

            # Parse ISO timestamp to UTC
            try:
                # Convert from local timezone to UTC
                local_dt = datetime.fromisoformat(date_str)
                # Strip timezone info for comparison
                utc_dt = local_dt.strftime("%Y-%m-%dT%H:%M:%SZ")
            except ValueError:
                continue

            # Extract version from commit message
            version_match = re.search(r'0\.\d+\.\d+', message)
            if not version_match:
                continue

            version = version_match.group(0)

            deployments.append({
                "commit": commit_hash,
                "deployed_at": utc_dt,
                "version": version,
                "message": message.strip()
            })

        return deployments

    except subprocess.CalledProcessError as e:
        print(f"Error getting git history: {e}")
        return []


def is_in_bug_window(date_str: str) -> bool:
    """Check if a timestamp falls within the multipart bug window."""
    try:
        dt = datetime.fromisoformat(date_str.replace('Z', ''))
        start = datetime.fromisoformat(BUG_WINDOW_START.replace('Z', ''))
        end = datetime.fromisoformat(BUG_WINDOW_END.replace('Z', ''))
        return start <= dt <= end
    except ValueError:
        return False


def parse_armor_deployments() -> Dict[str, Any]:
    """Parse armor_deployments.json file."""
    deployments_file = Path("/home/coding/ARMOR/armor_deployments.json")

    if not deployments_file.exists():
        raise FileNotFoundError(f"armor_deployments.json not found at {deployments_file}")

    with open(deployments_file, 'r') as f:
        return json.load(f)


def create_deployment_windows():
    """Create deployment windows for affected multipart-era versions."""

    # Load current deployment inventory
    armor_deployments = parse_armor_deployments()

    # Get deployment history from git
    deployment_history = get_deployment_history()

    # Filter deployments within bug window
    affected_deployments = [
        d for d in deployment_history
        if is_in_bug_window(d['deployed_at'])
    ]

    # Sort by deployment date
    affected_deployments.sort(key=lambda x: x['deployed_at'])

    # Create deployment windows
    windows = []
    for i, deployment in enumerate(affected_deployments):
        deployed_at = deployment['deployed_at']

        # Window starts at deployment time
        window_start = deployed_at

        # Window ends at next deployment or bug window end
        if i + 1 < len(affected_deployments):
            window_end = affected_deployments[i + 1]['deployed_at']
        else:
            window_end = BUG_WINDOW_END

        windows.append({
            "version": deployment['version'],
            "deployed_at": deployed_at,
            "window_start": window_start,
            "window_end": window_end,
            "commit": deployment['commit'],
            "message": deployment['message']
        })

    return {
        "bug_window": {
            "start": BUG_WINDOW_START,
            "end": BUG_WINDOW_END,
            "description": "Multipart upload corruption bug window"
        },
        "affected_deployment_windows": windows,
        "summary": {
            "total_affected_deployments": len(windows),
            "versions_affected": list(set(w["version"] for w in windows)),
            "earliest_deployment": windows[0]["deployed_at"] if windows else None,
            "latest_deployment": windows[-1]["deployed_at"] if windows else None
        }
    }


def save_deployment_windows(windows_data: Dict[str, Any], output_file: str):
    """Save deployment windows to intermediate JSON file."""
    output_path = Path(output_file)
    output_path.parent.mkdir(parents=True, exist_ok=True)

    with open(output_path, 'w') as f:
        json.dump(windows_data, f, indent=2)

    print(f"Deployment windows saved to {output_path}")


def main():
    """Main function to parse deployment windows."""
    print("Parsing ARMOR deployment windows from drift inventory...")

    # Create deployment windows
    windows_data = create_deployment_windows()

    # Save to intermediate file
    output_file = "/home/coding/ARMOR/intermediate/deployment-windows.json"
    save_deployment_windows(windows_data, output_file)

    # Print summary
    print(f"\nSummary:")
    print(f"  Total affected deployments: {windows_data['summary']['total_affected_deployments']}")
    print(f"  Versions affected: {', '.join(windows_data['summary']['versions_affected'])}")
    print(f"  Earliest deployment: {windows_data['summary']['earliest_deployment']}")
    print(f"  Latest deployment: {windows_data['summary']['latest_deployment']}")

    print(f"\nDeployment windows:")
    for window in windows_data['affected_deployment_windows']:
        print(f"  {window['version']}: {window['window_start']} to {window['window_end']}")

    return windows_data


if __name__ == "__main__":
    main()