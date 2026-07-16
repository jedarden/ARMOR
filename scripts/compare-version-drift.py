#!/usr/bin/env python3
"""
ARMOR Version Drift Comparison

Compares deployed ARMOR versions against GitHub releases to detect drift.
Uses outputs from github-release-fetcher.py and find-armor-deployments.py.

Acceptance Criteria:
- Configurable thresholds (N releases, M days)
- Returns structured report per cluster: {cluster, deployed_tag, latest_tag, releases_behind, days_behind, is_drift, is_correctness_drift}
- Correctness drift gets highest priority flag
- Machine-readable JSON output
"""

import argparse
import json
import sys
from datetime import datetime
from typing import Dict, List, Optional
from dataclasses import dataclass


@dataclass
class Release:
    """GitHub release information."""
    tag: str
    published_at: str
    is_correctness: bool
    url: str


@dataclass
class Deployment:
    """ARMOR deployment information."""
    cluster: str
    image_tag: str
    filepath: str


@dataclass
class DriftReport:
    """Drift comparison report for a single deployment."""
    cluster: str
    deployed_tag: str
    latest_tag: str
    releases_behind: Optional[int]
    days_behind: Optional[int]
    is_drift: bool
    is_correctness_drift: bool
    deployed_date: Optional[str]
    latest_date: Optional[str]
    filepath: str


def parse_version(tag: str) -> Optional[int]:
    """Parse version tag to integer (e.g., 'v0.1.42' -> 42, 'fcbf6d3' -> None)."""
    import re
    match = re.match(r'v?0\.1\.(\d+)', tag)
    if match:
        return int(match.group(1))
    return None


def calculate_releases_behind(deployed_version: Optional[int], releases: List[Release]) -> Optional[int]:
    """Calculate how many releases a deployment is behind."""
    if deployed_version is None:
        return None

    # Count releases with version number greater than deployed
    releases_behind = 0
    for release in releases:
        release_version = parse_version(release.tag)
        if release_version and release_version > deployed_version:
            releases_behind += 1

    return releases_behind if releases_behind > 0 else 0


def calculate_days_behind(deployed_tag: str, releases: List[Release]) -> Optional[int]:
    """Calculate days behind based on release dates."""
    deployed_version = parse_version(deployed_tag)
    if deployed_version is None:
        return None

    # Find the release that matches the deployed version
    deployed_release = None
    for release in releases:
        release_version = parse_version(release.tag)
        if release_version == deployed_version:
            deployed_release = release
            break

    if not deployed_release:
        return None

    # Find the latest release
    latest_release = max(releases, key=lambda r: datetime.fromisoformat(r.published_at.replace('Z', '+00:00')))

    # Calculate days difference
    deployed_date = datetime.fromisoformat(deployed_release.published_at.replace('Z', '+00:00'))
    latest_date = datetime.fromisoformat(latest_release.published_at.replace('Z', '+00:00'))

    days_diff = (latest_date - deployed_date).days
    return days_diff if days_diff > 0 else 0


def check_correctness_drift(deployed_tag: str, releases: List[Release]) -> bool:
    """Check if deployment is behind a correctness release."""
    deployed_version = parse_version(deployed_tag)
    if deployed_version is None:
        return False

    # Check if any correctness release exists with version > deployed_version
    for release in releases:
        release_version = parse_version(release.tag)
        if release_version and release_version > deployed_version and release.is_correctness:
            return True

    return False


def compare_drift(
    deployments: List[Deployment],
    releases: List[Release],
    releases_threshold: int,
    days_threshold: int
) -> List[DriftReport]:
    """Compare deployments against releases and generate drift reports."""
    if not releases:
        print("Warning: No releases found for comparison", file=sys.stderr)
        return []

    # Find the latest release by version number
    versioned_releases = [r for r in releases if parse_version(r.tag)]
    if not versioned_releases:
        print("Warning: No versioned releases found for comparison", file=sys.stderr)
        # Use the most recent release by date instead
        latest_release = max(releases, key=lambda r: datetime.fromisoformat(r.published_at.replace('Z', '+00:00')))
    else:
        latest_release = max(versioned_releases, key=lambda r: parse_version(r.tag) or 0)
    latest_tag = latest_release.tag

    reports = []
    for deployment in deployments:
        deployed_version = parse_version(deployment.image_tag)

        # Calculate releases behind
        releases_behind = calculate_releases_behind(deployed_version, releases)

        # Calculate days behind
        days_behind = calculate_days_behind(deployment.image_tag, releases)

        # Check for correctness drift
        correctness_drift = check_correctness_drift(deployment.image_tag, releases)

        # Determine if drift exceeds thresholds
        is_drift = (
            (releases_behind is not None and releases_behind >= releases_threshold) or
            (days_behind is not None and days_behind >= days_threshold) or
            correctness_drift
        )

        # Get deployed date
        deployed_date = None
        if deployed_version is not None:
            for release in releases:
                release_version = parse_version(release.tag)
                if release_version == deployed_version:
                    deployed_date = release.published_at
                    break

        # Get latest date
        latest_date = latest_release.published_at

        report = DriftReport(
            cluster=deployment.cluster,
            deployed_tag=deployment.image_tag,
            latest_tag=latest_tag,
            releases_behind=releases_behind,
            days_behind=days_behind,
            is_drift=is_drift,
            is_correctness_drift=correctness_drift,
            deployed_date=deployed_date,
            latest_date=latest_date,
            filepath=deployment.filepath
        )

        reports.append(report)

    return reports


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Compare ARMOR deployments against GitHub releases to detect version drift'
    )
    parser.add_argument(
        '--deployments',
        required=True,
        help='Path to JSON file containing deployments output from find-armor-deployments.py'
    )
    parser.add_argument(
        '--releases',
        required=True,
        help='Path to JSON file containing releases output from github-release-fetcher.py'
    )
    parser.add_argument(
        '--releases-threshold',
        type=int,
        default=50,
        help='Flag deployments behind by N or more releases (default: 50)'
    )
    parser.add_argument(
        '--days-threshold',
        type=int,
        default=30,
        help='Flag deployments behind by M or more days (default: 30)'
    )
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output machine-readable JSON instead of human-readable format'
    )
    parser.add_argument(
        '--sort-by',
        choices=['cluster', 'releases', 'days', 'correctness'],
        default='cluster',
        help='Sort output by field (default: cluster)'
    )

    args = parser.parse_args()

    # Load deployments
    try:
        with open(args.deployments, 'r') as f:
            deployments_data = json.load(f)
        deployments = [Deployment(**d) for d in deployments_data]
    except Exception as e:
        print(f"Error loading deployments: {e}", file=sys.stderr)
        sys.exit(1)

    # Load releases
    try:
        with open(args.releases, 'r') as f:
            releases_data = json.load(f)
        releases = [Release(**r) for r in releases_data]
    except Exception as e:
        print(f"Error loading releases: {e}", file=sys.stderr)
        sys.exit(1)

    # Compare drift
    reports = compare_drift(
        deployments,
        releases,
        args.releases_threshold,
        args.days_threshold
    )

    # Sort reports
    if args.sort_by == 'cluster':
        reports.sort(key=lambda r: r.cluster)
    elif args.sort_by == 'releases':
        reports.sort(key=lambda r: r.releases_behind or 0, reverse=True)
    elif args.sort_by == 'days':
        reports.sort(key=lambda r: r.days_behind or 0, reverse=True)
    elif args.sort_by == 'correctness':
        reports.sort(key=lambda r: r.is_correctness_drift, reverse=True)

    # Output
    if args.json:
        output_json(reports, args.releases_threshold, args.days_threshold)
    else:
        output_human_readable(reports, args.releases_threshold, args.days_threshold)


def output_json(reports: List[DriftReport], releases_threshold: int, days_threshold: int):
    """Output machine-readable JSON."""
    output = {
        "thresholds": {
            "releases": releases_threshold,
            "days": days_threshold
        },
        "summary": {
            "total_deployments": len(reports),
            "with_drift": sum(1 for r in reports if r.is_drift),
            "with_correctness_drift": sum(1 for r in reports if r.is_correctness_drift)
        },
        "deployments": []
    }

    for report in reports:
        deployment_dict = {
            "cluster": report.cluster,
            "deployed_tag": report.deployed_tag,
            "latest_tag": report.latest_tag,
            "releases_behind": report.releases_behind,
            "days_behind": report.days_behind,
            "is_drift": report.is_drift,
            "is_correctness_drift": report.is_correctness_drift,
            "deployed_date": report.deployed_date,
            "latest_date": report.latest_date,
            "filepath": report.filepath
        }
        output["deployments"].append(deployment_dict)

    print(json.dumps(output, indent=2))


def output_human_readable(reports: List[DriftReport], releases_threshold: int, days_threshold: int):
    """Output human-readable summary."""
    print("=" * 80)
    print("ARMOR Version Drift Comparison")
    print("=" * 80)
    print(f"Thresholds: > {releases_threshold} releases, > {days_threshold} days")
    print()

    # Print individual reports
    for report in reports:
        status_icon = "🔴" if report.is_correctness_drift else ("🟡" if report.is_drift else "✅")

        print(f"{status_icon} {report.cluster}")
        print(f"   Deployed: {report.deployed_tag}")
        print(f"   Latest:   {report.latest_tag}")

        if report.releases_behind is not None:
            print(f"   Releases behind: {report.releases_behind}")

        if report.days_behind is not None:
            print(f"   Days behind: {report.days_behind}")

        if report.is_correctness_drift:
            print(f"   🚨 CORRECTNESS DRIFT: Missing correctness releases!")

        print()

    # Print summary
    print("=" * 80)
    print("SUMMARY")
    print(f"Total deployments: {len(reports)}")
    print(f"With drift: {sum(1 for r in reports if r.is_drift)}")
    print(f"With correctness drift: {sum(1 for r in reports if r.is_correctness_drift)}")


if __name__ == "__main__":
    main()
