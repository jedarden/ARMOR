#!/usr/bin/env python3
"""
Fetch ARMOR releases from GitHub API and distinguish correctness-labeled releases
from routine version bumps.

Returns structured JSON: [{tag, published_at, is_correctness, url}]

Features:
- Fetches latest N releases from GitHub API
- Identifies correctness-labeled releases via keyword detection
- Local caching with TTL to avoid API spam
- Graceful rate limit handling
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timedelta
from pathlib import Path


# Configuration
REPO_OWNER = "jedarden"
REPO_NAME = "ARMOR"
GITHUB_API_URL = f"https://api.github.com/repos/{REPO_OWNER}/{REPO_NAME}/releases"

# Cache configuration
CACHE_DIR = Path.home() / '.cache' / 'armor-release-fetcher'
CACHE_FILE = CACHE_DIR / 'releases.json'
CACHE_TTL_SECONDS = 3600  # 1 hour default TTL

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


def is_cache_valid(ttl_seconds: int = None) -> bool:
    """
    Check if cached data exists and is within TTL.

    Args:
        ttl_seconds: Override TTL in seconds (uses CACHE_TTL_SECONDS if None)

    Returns:
        True if cache is valid, False otherwise
    """
    if ttl_seconds is None:
        ttl_seconds = CACHE_TTL_SECONDS

    if not CACHE_FILE.exists():
        return False

    try:
        # Check cache age
        cache_mtime = datetime.fromtimestamp(CACHE_FILE.stat().st_mtime)
        age = datetime.now() - cache_mtime

        return age.total_seconds() < ttl_seconds
    except Exception:
        return False


def load_cached_releases(ttl_seconds: int = None) -> list[dict] | None:
    """
    Load releases from cache if valid.

    Args:
        ttl_seconds: Override TTL in seconds (uses CACHE_TTL_SECONDS if None)

    Returns:
        List of cached releases or None if cache is invalid/missing
    """
    if ttl_seconds is None:
        ttl_seconds = CACHE_TTL_SECONDS

    if not is_cache_valid(ttl_seconds):
        return None

    try:
        with open(CACHE_FILE, 'r') as f:
            cache_data = json.load(f)

        # Validate cache structure
        if not isinstance(cache_data, dict):
            return None

        if 'releases' not in cache_data or 'cached_at' not in cache_data:
            return None

        cached_at = datetime.fromisoformat(cache_data['cached_at'])
        age = datetime.now() - cached_at

        if age.total_seconds() >= ttl_seconds:
            return None

        print(f"Using cached data from {cached_at.strftime('%Y-%m-%d %H:%M:%S')} (age: {int(age.total_seconds())}s)",
              file=sys.stderr)
        return cache_data['releases']

    except (json.JSONDecodeError, KeyError, ValueError) as e:
        print(f"Warning: Invalid cache file: {e}", file=sys.stderr)
        return None


def save_releases_to_cache(releases: list[dict]) -> None:
    """
    Save releases to cache with timestamp.

    Args:
        releases: List of parsed release dictionaries
    """
    try:
        # Create cache directory if it doesn't exist
        CACHE_DIR.mkdir(parents=True, exist_ok=True)

        cache_data = {
            'cached_at': datetime.now().isoformat(),
            'releases': releases,
            'repo': f"{REPO_OWNER}/{REPO_NAME}",
            'ttl_seconds': CACHE_TTL_SECONDS
        }

        with open(CACHE_FILE, 'w') as f:
            json.dump(cache_data, f, indent=2)

        print(f"Cached {len(releases)} release(s) to {CACHE_FILE}", file=sys.stderr)

    except Exception as e:
        print(f"Warning: Failed to write cache: {e}", file=sys.stderr)


def clear_cache() -> bool:
    """
    Clear the release cache.

    Returns:
        True if cache was cleared, False if it didn't exist
    """
    if CACHE_FILE.exists():
        try:
            CACHE_FILE.unlink()
            print(f"Cleared cache at {CACHE_FILE}", file=sys.stderr)
            return True
        except Exception as e:
            print(f"Error clearing cache: {e}", file=sys.stderr)
            return False
    return False


def fetch_releases(per_page: int = 100, use_cache: bool = True, force_refresh: bool = False, limit: int | None = None, ttl_seconds: int = None) -> list[dict]:
    """
    Fetch releases from GitHub API with optional caching.

    Args:
        per_page: Number of releases per page (max 100)
        use_cache: Whether to use cached data if available
        force_refresh: Force refresh from GitHub API even if cache is valid
        limit: Maximum number of releases to fetch (None for all)
        ttl_seconds: Override cache TTL in seconds (uses CACHE_TTL_SECONDS if None)

    Returns:
        List of release dictionaries from GitHub API
    """
    # Check cache first (unless force refresh is requested)
    if use_cache and not force_refresh:
        cached = load_cached_releases(ttl_seconds)
        if cached is not None:
            # Apply limit if specified
            if limit is not None and len(cached) > limit:
                return cached[:limit]
            return cached

    print("Fetching releases from GitHub API...", file=sys.stderr)
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

                # Check if we've reached the limit
                if limit is not None and len(releases) >= limit:
                    releases = releases[:limit]
                    break

                # Check if we've fetched all releases
                if len(data) < per_page:
                    break

                page += 1
                # Rate limit avoidance: small delay between pages
                time.sleep(0.1)

        except urllib.error.HTTPError as e:
            if e.code == 403:
                # Rate limit exceeded
                print(f"Error: GitHub API rate limit exceeded. "
                      f"Use --cache to avoid repeated API calls.", file=sys.stderr)
                sys.exit(1)
            print(f"Error fetching releases: {e}", file=sys.stderr)
            sys.exit(1)
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
    import argparse

    parser = argparse.ArgumentParser(
        description='Fetch ARMOR releases from GitHub API',
        epilog='Examples:\n'
               '  %(prog)s                 # Fetch all releases (uses cache if fresh)\n'
               '  %(prog)s --limit 10      # Fetch latest 10 releases\n'
               '  %(prog)s --refresh       # Force refresh from GitHub API\n'
               '  %(prog)s --no-cache       # Bypass cache entirely\n'
               '  %(prog)s --clear-cache    # Clear cached data and exit',
        formatter_class=argparse.RawDescriptionHelpFormatter
    )

    parser.add_argument(
        '--limit', '-n',
        type=int,
        help='Maximum number of releases to fetch (default: all)'
    )
    parser.add_argument(
        '--refresh',
        action='store_true',
        help='Force refresh from GitHub API (ignore cache)'
    )
    parser.add_argument(
        '--no-cache',
        action='store_true',
        help='Disable cache reading/writing'
    )
    parser.add_argument(
        '--clear-cache',
        action='store_true',
        help='Clear cached data and exit'
    )
    parser.add_argument(
        '--cache-ttl',
        type=int,
        help=f'Override cache TTL in seconds (default: {CACHE_TTL_SECONDS})'
    )

    args = parser.parse_args()

    # Determine cache TTL
    cache_ttl = args.cache_ttl if args.cache_ttl else CACHE_TTL_SECONDS

    # Handle cache clearing
    if args.clear_cache:
        if clear_cache():
            print("Cache cleared successfully", file=sys.stderr)
            return 0
        else:
            print("No cache to clear", file=sys.stderr)
            return 1

    # Determine cache settings
    use_cache = not args.no_cache
    force_refresh = args.refresh

    # Fetch releases
    raw_releases = fetch_releases(
        limit=args.limit,
        use_cache=use_cache,
        force_refresh=force_refresh,
        ttl_seconds=cache_ttl
    )

    if not raw_releases:
        print("No releases found", file=sys.stderr)
        print(json.dumps([], indent=2))
        return 0

    # Parse releases
    parsed_releases = [parse_release(r) for r in raw_releases]

    # Save to cache if we fetched from API and cache is enabled
    if use_cache and not args.no_cache:
        # Only cache if we actually fetched from API (not from existing cache)
        if force_refresh or not is_cache_valid(cache_ttl):
            save_releases_to_cache(parsed_releases)

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
