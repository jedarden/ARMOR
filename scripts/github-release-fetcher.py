#!/usr/bin/env python3
"""
Fetch ARMOR releases from GitHub API and distinguish correctness-labeled releases
from routine version bumps.

Returns structured JSON: [{tag, published_at, is_correctness, url}]
"""

import json
import sys
import time
import urllib.error
import urllib.request


# Configuration
REPO_OWNER = "jedarden"
REPO_NAME = "ARMOR"
GITHUB_API_URL = f"https://api.github.com/repos/{REPO_OWNER}/{REPO_NAME}/releases"

# Keywords that indicate correctness/security releases
CORRECTNESS_KEYWORDS = [
    'correctness',
    'fix',
    'critical',
    'security',
    'bug',
    'patch',
    'hotfix',
    'urgent',
    'vulnerability',
    'cve',
    'issue',
    'regression',
]


def fetch_releases(per_page: int = 100) -> list[dict]:
    """
    Fetch all releases from GitHub API.

    Args:
        per_page: Number of releases per page (max 100)

    Returns:
        List of release dictionaries from GitHub API
    """
    releases = []
    page = 1

    while True:
        try:
            url = f"{GITHUB_API_URL}?per_page={per_page}&page={page}"

            # Set user agent for GitHub API requirements
            headers = {
                'User-Agent': f'{REPO_OWNER}-{REPO_NAME}-release-fetcher',
                'Accept': 'application/vnd.github.v3+json'
            }

            req = urllib.request.Request(url, headers=headers)

            with urllib.request.urlopen(req, timeout=10) as response:
                if response.status != 200:
                    print(f"Error: GitHub API returned status {response.status}", file=sys.stderr)
                    sys.exit(1)

                data = json.loads(response.read().decode('utf-8'))

                if not data:
                    break

                releases.extend(data)

                # Check if we've fetched all releases
                if len(data) < per_page:
                    break

                page += 1
                # Rate limit avoidance: small delay between pages
                time.sleep(0.1)

        except urllib.error.URLError as e:
            print(f"Error fetching releases: {e}", file=sys.stderr)
            sys.exit(1)
        except json.JSONDecodeError as e:
            print(f"Error decoding JSON response: {e}", file=sys.stderr)
            sys.exit(1)
        except Exception as e:
            print(f"Unexpected error: {e}", file=sys.stderr)
            sys.exit(1)

    return releases


def is_correctness_release(release: dict) -> bool:
    """
    Determine if a release is correctness-labeled based on tag name and release notes.

    Args:
        release: GitHub release dictionary

    Returns:
        True if release appears to be correctness/security-related
    """
    tag_name = release.get('tag_name', '').lower()
    name = release.get('name', '').lower()
    body = release.get('body', '').lower()

    # Check both tag name, release name, and body for correctness keywords
    combined_text = f"{tag_name} {name} {body}"

    for keyword in CORRECTNESS_KEYWORDS:
        if keyword in combined_text:
            return True

    return False


def parse_release(release: dict) -> dict:
    """
    Parse a GitHub release into structured output.

    Args:
        release: GitHub release dictionary

    Returns:
        Dictionary with {tag, published_at, is_correctness, url}
    """
    return {
        'tag': release.get('tag_name', ''),
        'published_at': release.get('published_at', ''),
        'is_correctness': is_correctness_release(release),
        'url': release.get('html_url', ''),
    }


def main():
    """Main entry point."""
    # Fetch all releases
    raw_releases = fetch_releases()

    if not raw_releases:
        print("No releases found", file=sys.stderr)
        print(json.dumps([], indent=2))
        return 0

    # Parse releases
    parsed_releases = [parse_release(r) for r in raw_releases]

    # Output as JSON (already sorted by GitHub API - newest first)
    print(json.dumps(parsed_releases, indent=2))

    # Print summary to stderr
    correctness_count = sum(1 for r in parsed_releases if r['is_correctness'])
    print(f"Fetched {len(parsed_releases)} release(s)", file=sys.stderr)
    print(f"Correctness-labeled: {correctness_count}", file=sys.stderr)
    print(f"Routine version bumps: {len(parsed_releases) - correctness_count}", file=sys.stderr)

    return 0


if __name__ == '__main__':
    sys.exit(main())
