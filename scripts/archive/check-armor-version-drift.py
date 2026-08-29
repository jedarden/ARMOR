#!/usr/bin/env python3
"""
ARMOR Version Drift Check

Checks deployed ARMOR versions across clusters against the latest release.
Flags deployments that are N releases or M days behind.
Distinguishes correctness-labeled releases from routine version bumps.
"""

import os
import re
import sys
import subprocess
import json
from datetime import datetime, timedelta
from pathlib import Path
from typing import Dict, List, Tuple, Optional

# Configuration
DECLARATIVE_CONFIG_PATH = Path.home() / "declarative-config" / "k8s"
ARMOR_REPO_PATH = Path.cwd()
VERSION_WARNING_THRESHOLD = 50  # Flag if behind by this many versions
DAYS_WARNING_THRESHOLD = 30  # Flag if behind by this many days

# Deployments to check (cluster, path to deployment file)
DEPLOYMENTS = [
    ("iad-ci", "iad-ci/armor/armor-deployment.yaml"),
    ("iad-kalshi", "iad-kalshi/armor/armor-deployment.yml"),
    ("rs-manager", "rs-manager/armor/armor-deployment.yml"),
    ("ord-devimprint", "ord-devimprint/devimprint/armor-deployment.yml"),
    ("iad-native-ads", "iad-native-ads/armor/armor-deployment.yml"),
    ("iad-acb", "iad-acb/ai-code-battle/acb-armor-deployment.yml"),
]


def get_current_version() -> Tuple[str, int]:
    """Get current ARMOR version from VERSION file."""
    version_file = ARMOR_REPO_PATH / "VERSION"
    if not version_file.exists():
        print(f"ERROR: VERSION file not found at {version_file}")
        sys.exit(1)

    version_str = version_file.read_text().strip()
    # Parse version number (e.g., "0.1.1804" -> 1804)
    match = re.match(r'0\.1\.(\d+)', version_str)
    if not match:
        print(f"ERROR: Invalid version format: {version_str}")
        sys.exit(1)

    version_num = int(match.group(1))
    return f"0.1.{version_num}", version_num


def extract_image_tag(deployment_file: Path) -> Optional[str]:
    """Extract ARMOR image tag from deployment file."""
    if not deployment_file.exists():
        return None

    content = deployment_file.read_text()
    # Look for ronaldraygun/armor:TAG pattern
    match = re.search(r'ronaldraygun/armor:([^\s"\']+)', content)
    if match:
        return match.group(1)
    return None


def parse_version_tag(tag: str) -> Optional[int]:
    """Parse version tag to number (e.g., '0.1.42' -> 42, 'fcbf6d3' -> None)."""
    match = re.match(r'0\.1\.(\d+)', tag)
    if match:
        return int(match.group(1))
    return None


def get_commit_date(version_num: int) -> Optional[datetime]:
    """Get the commit date for a given version number."""
    try:
        # Find commit that bumps to this version
        result = subprocess.run(
            ["git", "log", "--all", "--grep", f"auto-bump version to 0.1.{version_num}",
             "--format=%H", "-n", "1"],
            cwd=ARMOR_REPO_PATH,
            capture_output=True,
            text=True,
            timeout=10
        )

        if result.returncode == 0 and result.stdout.strip():
            commit_hash = result.stdout.strip()
            # Get commit date
            date_result = subprocess.run(
                ["git", "show", "-s", "--format=%ci", commit_hash],
                cwd=ARMOR_REPO_PATH,
                capture_output=True,
                text=True,
                timeout=10
            )

            if date_result.returncode == 0:
                date_str = date_result.stdout.strip()
                # Parse git date format (e.g., "2026-07-14 12:34:56 +0000")
                return datetime.fromisoformat(date_str.replace(" +", "+"))

        return None
    except (subprocess.TimeoutExpired, FileNotFoundError, ValueError) as e:
        print(f"WARNING: Could not get commit date for version {version_num}: {e}")
        return None


def check_correctness_releases(since_version: int, current_version: int) -> List[str]:
    """Check for correctness/security releases between two versions."""
    try:
        # Get commit messages between the two versions (excluding auto-bump commits)
        result = subprocess.run(
            ["git", "log", "--all", "--oneline",
             f"~1..HEAD"],  # Get recent commits, will filter in Python
            cwd=ARMOR_REPO_PATH,
            capture_output=True,
            text=True,
            timeout=10
        )

        if result.returncode != 0:
            return []

        commits = result.stdout.strip().split('\n')
        correctness_keywords = ['fix', 'bug', 'security', 'correct', 'vulnerability', 'patch']
        correctness_releases = []
        versions_to_check = range(since_version + 1, current_version + 1)

        # For each version in range, get the actual feature commit (not the auto-bump)
        for version_num in versions_to_check:
            try:
                # Find the auto-bump commit for this version
                bump_result = subprocess.run(
                    ["git", "log", "--all", "--grep", f"auto-bump version to 0.1.{version_num}",
                     "--format=%H", "-n", "1"],
                    cwd=ARMOR_REPO_PATH,
                    capture_output=True,
                    text=True,
                    timeout=5
                )

                if bump_result.returncode == 0 and bump_result.stdout.strip():
                    bump_hash = bump_result.stdout.strip()
                    # Get the previous commit (the actual feature commit)
                    feature_result = subprocess.run(
                        ["git", "log", "--format=%H %s", f"{bump_hash}~1..{bump_hash}~1"],
                        cwd=ARMOR_REPO_PATH,
                        capture_output=True,
                        text=True,
                        timeout=5
                    )

                    if feature_result.returncode == 0 and feature_result.stdout.strip():
                        feature_line = feature_result.stdout.strip()
                        parts = feature_line.split(' ', 1)
                        if len(parts) == 2:
                            commit_msg = parts[1]
                            commit_lower = commit_msg.lower()
                            # Check if it's a correctness release
                            if any(kw in commit_lower for kw in correctness_keywords):
                                correctness_releases.append(f"0.1.{version_num}: {commit_msg}")
            except (subprocess.TimeoutExpired, FileNotFoundError):
                continue

        return correctness_releases
    except (subprocess.TimeoutExpired, FileNotFoundError) as e:
        print(f"WARNING: Could not check correctness releases: {e}")
        return []


def check_drift() -> Dict:
    """Main function to check version drift across deployments."""
    current_version, current_version_num = get_current_version()

    results = {
        "current_version": current_version,
        "current_version_num": current_version_num,
        "current_date": datetime.now().isoformat(),
        "deployments": [],
        "summary": {
            "total_deployments": 0,
            "needs_update": 0,
            "using_non_version_tag": 0
        }
    }

    current_date = datetime.now()
    current_commit_date = get_commit_date(current_version_num)

    print(f"ARMOR Version Drift Check")
    print(f"Current version: {current_version}")
    if current_commit_date:
        print(f"Current version date: {current_commit_date.strftime('%Y-%m-%d')}")
    print(f"Warning thresholds: > {VERSION_WARNING_THRESHOLD} versions, > {DAYS_WARNING_THRESHOLD} days")
    print("=" * 80)

    for cluster, deployment_path in DEPLOYMENTS:
        full_path = DECLARATIVE_CONFIG_PATH / deployment_path

        if not full_path.exists():
            print(f"\n⚠️  {cluster}: Deployment file not found at {deployment_path}")
            continue

        deployed_tag = extract_image_tag(full_path)
        if not deployed_tag:
            print(f"\n⚠️  {cluster}: Could not extract image tag")
            continue

        deployed_version = parse_version_tag(deployed_tag)

        deployment_info = {
            "cluster": cluster,
            "deployment_file": str(deployment_path),
            "deployed_tag": deployed_tag,
            "deployed_version_num": deployed_version,
            "drift_versions": None,
            "drift_days": None,
            "deployed_date": None,
            "needs_update": False,
            "using_non_version_tag": False,
            "correctness_releases_missed": []
        }

        if deployed_version is None:
            # Using git SHA or other tag
            deployment_info["using_non_version_tag"] = True
            deployment_info["needs_update"] = True
            results["summary"]["using_non_version_tag"] += 1
            print(f"\n🔶 {cluster}: Using non-version tag '{deployed_tag}' (consider updating)")
        else:
            versions_behind = current_version_num - deployed_version
            deployment_info["drift_versions"] = versions_behind

            deployed_date = get_commit_date(deployed_version)
            if deployed_date and current_commit_date:
                days_behind = (current_commit_date - deployed_date).days
                deployment_info["drift_days"] = days_behind
                deployment_info["deployed_date"] = deployed_date.isoformat()

            # Check for correctness releases
            correctness = check_correctness_releases(deployed_version, current_version_num)
            deployment_info["correctness_releases_missed"] = correctness

            # Determine if update needed
            version_drift_critical = versions_behind > VERSION_WARNING_THRESHOLD
            days_drift_critical = deployment_info.get("drift_days", 0) > DAYS_WARNING_THRESHOLD
            has_correctness_releases = len(correctness) > 0

            deployment_info["needs_update"] = (
                version_drift_critical or
                days_drift_critical or
                has_correctness_releases
            )

            if deployment_info["needs_update"]:
                results["summary"]["needs_update"] += 1

            # Print status
            status_icon = "✅" if not deployment_info["needs_update"] else "🔴"
            print(f"\n{status_icon} {cluster}: {deployed_tag}")

            if deployed_date:
                print(f"   Deployed: {deployed_date.strftime('%Y-%m-%d')}")

            print(f"   Versions behind: {versions_behind}")
            if deployment_info.get("drift_days"):
                print(f"   Days behind: {deployment_info['drift_days']}")

            if correctness:
                print(f"   ⚠️  MISSED CORRECTNESS RELEASES ({len(correctness)}):")
                for cr in correctness[:5]:  # Show first 5
                    print(f"      - {cr}")
                if len(correctness) > 5:
                    print(f"      ... and {len(correctness) - 5} more")

            if deployment_info["needs_update"]:
                if has_correctness_releases:
                    print(f"   🚨 ACTION NEEDED: Correctness releases missed!")
                elif version_drift_critical:
                    print(f"   ⚠️  UPDATE RECOMMENDED: {versions_behind} versions behind")
                elif days_drift_critical:
                    print(f"   ⚠️  UPDATE RECOMMENDED: {deployment_info['drift_days']} days behind")

        results["deployments"].append(deployment_info)
        results["summary"]["total_deployments"] += 1

    # Print summary
    print("\n" + "=" * 80)
    print("SUMMARY")
    print(f"Total deployments checked: {results['summary']['total_deployments']}")
    print(f"Deployments needing update: {results['summary']['needs_update']}")
    print(f"Using non-version tags: {results['summary']['using_non_version_tag']}")

    return results


def main():
    """Main entry point."""
    if len(sys.argv) > 1 and sys.argv[1] == "--json":
        results = check_drift()
        print(json.dumps(results, indent=2))
    else:
        check_drift()


if __name__ == "__main__":
    main()
