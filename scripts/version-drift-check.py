#!/usr/bin/env python3
"""
ARMOR Version Drift Check - Unified Script

Orchestrates the complete version drift check pipeline:
1. Fetch releases from GitHub
2. Find all ARMOR deployments in declarative-config
3. Compare deployments against releases
4. Generate report (console + file output)

Usage:
    python3 scripts/version-drift-check.py [--config CONFIG_FILE] [--json] [--output OUTPUT_FILE]
"""

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Any


# Default configuration
DEFAULT_CONFIG = {
    "releases_threshold": 50,
    "days_threshold": 30,
    "declarative_config_path": "~/declarative-config",
    "github_repo": "jedarden/ARMOR",
    "output_file": None,
    "sort_by": "correctness"
}


def load_config(config_path: str = None) -> Dict[str, Any]:
    """Load configuration from file or use defaults."""
    config = DEFAULT_CONFIG.copy()

    if config_path and Path(config_path).exists():
        with open(config_path, 'r') as f:
            user_config = json.load(f)
            config.update(user_config)

    # Expand tilde in paths
    if 'declarative_config_path' in config:
        config['declarative_config_path'] = os.path.expanduser(config['declarative_config_path'])

    return config


def run_script(script_path: str, args: List[str] = None) -> str:
    """Run a Python script and return its stdout."""
    script_full_path = Path(__file__).parent / script_path
    if not script_full_path.exists():
        raise FileNotFoundError(f"Script not found: {script_full_path}")

    cmd = [sys.executable, str(script_full_path)]
    if args:
        cmd.extend(args)

    result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)

    if result.returncode != 0:
        raise RuntimeError(f"Script {script_path} failed: {result.stderr}")

    return result.stdout


def fetch_releases(config: Dict[str, Any]) -> List[Dict]:
    """Fetch releases from GitHub."""
    output = run_script("github-release-fetcher.py")
    releases = json.loads(output)

    return releases


def find_deployments(config: Dict[str, Any]) -> List[Dict]:
    """Find ARMOR deployments in declarative-config."""
    print("Finding ARMOR deployments...", file=sys.stderr)

    declarative_path = config['declarative_config_path']
    if not os.path.exists(declarative_path):
        raise FileNotFoundError(f"declarative-config not found at {declarative_path}")

    output = run_script("find-armor-deployments.py", [declarative_path])
    deployments = json.loads(output)

    print(f"Found {len(deployments)} deployment(s)", file=sys.stderr)

    return deployments


def compare_drift(
    deployments: List[Dict],
    releases: List[Dict],
    config: Dict[str, Any]
) -> List[Dict]:
    """Compare deployments against releases and generate drift reports."""
    print("Comparing drift...", file=sys.stderr)

    # Write temporary files for compare script
    import tempfile

    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        json.dump(deployments, f)
        deployments_file = f.name

    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        json.dump(releases, f)
        releases_file = f.name

    try:
        args = [
            '--deployments', deployments_file,
            '--releases', releases_file,
            '--releases-threshold', str(config['releases_threshold']),
            '--days-threshold', str(config['days_threshold']),
            '--json'
        ]

        output = run_script("compare-version-drift.py", args)
        result = json.loads(output)

        # Sort by specified field
        sort_by = config.get('sort_by', 'correctness')
        deployments_list = result.get('deployments', [])

        if sort_by == 'correctness':
            deployments_list.sort(key=lambda d: d.get('is_correctness_drift', False), reverse=True)
        elif sort_by == 'releases':
            deployments_list.sort(key=lambda d: d.get('releases_behind') or 0, reverse=True)
        elif sort_by == 'days':
            deployments_list.sort(key=lambda d: d.get('days_behind') or 0, reverse=True)
        elif sort_by == 'cluster':
            deployments_list.sort(key=lambda d: d.get('cluster', ''))

        result['deployments'] = deployments_list
        return result

    finally:
        # Clean up temp files
        os.unlink(deployments_file)
        os.unlink(releases_file)


def format_report(drift_result: Dict, config: Dict) -> str:
    """Format human-readable report."""
    lines = []
    lines.append("=" * 80)
    lines.append("ARMOR Version Drift Report")
    lines.append("=" * 80)
    lines.append(f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S UTC')}")
    lines.append(f"Thresholds: > {config['releases_threshold']} releases, > {config['days_threshold']} days")
    lines.append("")

    summary = drift_result.get('summary', {})
    lines.append(f"Total deployments: {summary.get('total_deployments', 0)}")
    lines.append(f"With drift: {summary.get('with_drift', 0)}")
    lines.append(f"With correctness drift: {summary.get('with_correctness_drift', 0)}")
    lines.append("")
    lines.append("=" * 80)
    lines.append("")

    for deployment in drift_result.get('deployments', []):
        if deployment.get('is_correctness_drift'):
            icon = "🔴"
        elif deployment.get('is_drift'):
            icon = "🟡"
        else:
            icon = "✅"

        lines.append(f"{icon} {deployment.get('cluster', 'unknown')}")
        lines.append(f"   Deployed: {deployment.get('deployed_tag', 'N/A')}")
        lines.append(f"   Latest:   {deployment.get('latest_tag', 'N/A')}")

        if deployment.get('releases_behind') is not None:
            lines.append(f"   Releases behind: {deployment['releases_behind']}")

        if deployment.get('days_behind') is not None:
            lines.append(f"   Days behind: {deployment['days_behind']}")

        if deployment.get('is_correctness_drift'):
            lines.append(f"   🚨 CORRECTNESS DRIFT: Missing correctness releases!")

        lines.append("")

    return "\n".join(lines)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='ARMOR Version Drift Check - Unified Script'
    )
    parser.add_argument(
        '--config',
        help='Path to configuration file (JSON)'
    )
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output machine-readable JSON instead of human-readable format'
    )
    parser.add_argument(
        '--output',
        help='Write report to file (in addition to stdout)'
    )
    parser.add_argument(
        '--releases-threshold',
        type=int,
        help='Override releases threshold from config'
    )
    parser.add_argument(
        '--days-threshold',
        type=int,
        help='Override days threshold from config'
    )
    parser.add_argument(
        '--sort-by',
        choices=['cluster', 'releases', 'days', 'correctness'],
        help='Sort output by field'
    )

    args = parser.parse_args()

    try:
        # Load configuration
        config = load_config(args.config)

        # Override config with command-line arguments
        if args.releases_threshold:
            config['releases_threshold'] = args.releases_threshold
        if args.days_threshold:
            config['days_threshold'] = args.days_threshold
        if args.sort_by:
            config['sort_by'] = args.sort_by

        # Run the pipeline
        releases = fetch_releases(config)
        deployments = find_deployments(config)
        drift_result = compare_drift(deployments, releases, config)

        # Add metadata
        drift_result['generated_at'] = datetime.now().isoformat()
        drift_result['config'] = {
            'releases_threshold': config['releases_threshold'],
            'days_threshold': config['days_threshold']
        }

        # Output
        if args.json:
            output = json.dumps(drift_result, indent=2)
        else:
            output = format_report(drift_result, config)

        print(output)

        # Write to file if specified
        if args.output:
            output_path = Path(args.output)
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_text(output)
            print(f"Report written to {args.output}", file=sys.stderr)

        # Exit with error code if there's correctness drift
        if drift_result.get('summary', {}).get('with_correctness_drift', 0) > 0:
            sys.exit(1)

        sys.exit(0)

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
