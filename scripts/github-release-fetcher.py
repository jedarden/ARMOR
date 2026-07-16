#!/usr/bin/env python3
"""
Fetch ARMOR versions from GitHub API and distinguish correctness-labeled releases
from routine version bumps.

Returns structured JSON: [{tag, published_at, is_correctness, url}]

Features:
- Fetches git tags from GitHub API (since ARMOR uses auto-bump commits)
- Falls back to GitHub Releases if tags are not available
- Identifies correctness-labeled releases via keyword detection in commit messages
- Local caching with TTL to avoid API spam
- Graceful rate limit handling
"""

import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timedelta
from pathlib import Path
from typing import Dict, List, Optional


# Configuration
REPO_OWNER = "jedarden"
REPO_NAME = "ARMOR"
GITHUB_TAGS_API = f"https://api.github.com/repos/{REPO_OWNER}/{REPO_NAME}/tags"
GITHUB_API_URL = f"https://api.github.com/repos/{REPO_OWNER}/{REPO_NAME}/releases"
REPO_URL = f"https://github.com/{REPO_OWNER}/{REPO_NAME}"

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


def fetch_git_tags(per_page: int = 100, use_cache: bool = True, force_refresh: bool = False, limit: int | None = None, ttl_seconds: int = None) -> list[dict]:
    """
    Fetch git tags from GitHub API (since ARMOR uses auto-bump commits, not GitHub Releases).

    Args:
        per_page: Number of tags per page (max 100)
        use_cache: Whether to use cached data if available
        force_refresh: Force refresh from GitHub API even if cache is valid
        limit: Maximum number of tags to fetch (None for all)
        ttl_seconds: Override cache TTL in seconds (uses CACHE_TTL_SECONDS if None)

    Returns:
        List of tag dictionaries with commit information
    """
    # Check cache first (unless force refresh is requested)
    if use_cache and not force_refresh:
        cached = load_cached_releases(ttl_seconds)
        if cached is not None:
            # Apply limit if specified
            if limit is not None and len(cached) > limit:
                return cached[:limit]
            return cached

    print("Fetching tags from GitHub API...", file=sys.stderr)
    tags = []
    page = 1

    while True:
        try:
            url = f"{GITHUB_TAGS_API}?per_page={per_page}&page={page}"

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

                tags.extend(data)

                # Check if we've reached the limit
                if limit is not None and len(tags) >= limit:
                    tags = tags[:limit]
                    break

                # Check if we've fetched all tags
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
            print(f"Error fetching tags: {e}", file=sys.stderr)
            sys.exit(1)
        except urllib.error.URLError as e:
            print(f"Error fetching tags: {e}", file=sys.stderr)
            sys.exit(1)
        except json.JSONDecodeError as e:
            print(f"Error decoding JSON response: {e}", file=sys.stderr)
            sys.exit(1)
        except Exception as e:
            print(f"Unexpected error: {e}", file=sys.stderr)
            sys.exit(1)

    return tags


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


def get_commit_message_for_tag(tag_name: str, repo_path: Optional[Path] = None) -> str:
    """
    Get the commit message for a specific tag using local git repository.

    Args:
        tag_name: Tag name to look up (e.g., 'v0.1.42')
        repo_path: Path to local git repo (uses current directory if None)

    Returns:
        Commit message for the tag, or empty string if not found
    """
    if repo_path is None:
        repo_path = Path.cwd()

    try:
        # Get the commit for the tag
        result = subprocess.run(
            ['git', 'rev-list', '-n', '1', tag_name],
            cwd=repo_path,
            capture_output=True,
            text=True,
            timeout=5
        )

        if result.returncode != 0 or not result.stdout.strip():
            return ""

        commit_hash = result.stdout.strip()

        # Get the commit message
        msg_result = subprocess.run(
            ['git', 'log', '-1', '--format=%B', commit_hash],
            cwd=repo_path,
            capture_output=True,
            text=True,
            timeout=5
        )

        if msg_result.returncode == 0:
            return msg_result.stdout.strip()

    except (subprocess.TimeoutExpired, FileNotFoundError, Exception):
        pass

    return ""


def get_tag_commit_date(tag_info: dict, repo_path: Optional[Path] = None) -> str:
    """
    Get the commit date for a tag using local git repository.
    Falls back to GitHub API commit info if local git fails.

    Args:
        tag_info: Tag info from GitHub API
        repo_path: Path to local git repo (uses current directory if None)

    Returns:
        ISO 8601 timestamp string
    """
    if repo_path is None:
        repo_path = Path.cwd()

    try:
        # Try local git first (more reliable)
        tag_name = tag_info.get('name', '').replace('refs/tags/', '')
        result = subprocess.run(
            ['git', 'log', '-1', '--format=%ci', tag_name],
            cwd=repo_path,
            capture_output=True,
            text=True,
            timeout=5
        )

        if result.returncode == 0 and result.stdout.strip():
            date_str = result.stdout.strip()
            # Parse git date format and convert to ISO
            dt = datetime.fromisoformat(date_str.replace(' +', '+'))
            return dt.isoformat()

    except (subprocess.TimeoutExpired, FileNotFoundError, Exception):
        pass

    # Fallback to GitHub API
    commit_sha = tag_info.get('commit', {}).get('sha')
    if commit_sha:
        try:
            url = f"https://api.github.com/repos/{REPO_OWNER}/{REPO_NAME}/commits/{commit_sha}"
            headers = {
                'User-Agent': f'{REPO_OWNER}-{REPO_NAME}-release-fetcher',
                'Accept': 'application/vnd.github.v3+json'
            }
            req = urllib.request.Request(url, headers=headers)

            with urllib.request.urlopen(req, timeout=5) as response:
                if response.status == 200:
                    commit_data = json.loads(response.read().decode('utf-8'))
                    return commit_data.get('commit', {}).get('committer', {}).get('date', '')
        except Exception:
            pass

    return datetime.now().isoformat()


def is_correctness_release(release: dict, commit_msg: str = "") -> bool:
    """
    Determine if a release is correctness-labeled based on tag name and release notes.

    Args:
        release: GitHub release dictionary (raw API format or parsed format)
        commit_msg: Commit message to check (if available)

    Returns:
        True if release appears to be correctness/security-related
    """
    # Handle both raw API format (tag_name) and parsed format (tag)
    tag_name = release.get('tag_name', release.get('tag', release.get('name', ''))).lower()
    name = release.get('name', '').lower()
    body = release.get('body', '').lower()

    # Check both tag name, release name, body, and commit message for correctness keywords
    combined_text = f"{tag_name} {name} {body} {commit_msg}"

    for keyword in CORRECTNESS_KEYWORDS:
        if keyword in combined_text:
            return True

    return False


def parse_release(release: dict, commit_msg: str = "") -> dict:
    """
    Parse a GitHub release or tag into structured output.
    Handles both raw API format and already-parsed format.

    Args:
        release: GitHub release/tag dictionary (raw API format or parsed format)
        commit_msg: Commit message for the tag/release

    Returns:
        Dictionary with {tag, published_at, is_correctness, url}
    """
    # Check if already parsed (has 'tag' field from our format)
    if 'tag' in release and 'html_url' not in release and 'tarball_url' not in release:
        # Already in our parsed format - return as-is
        return release

    # Handle GitHub tag format
    if 'tarball_url' in release and 'name' in release:
        tag_name = release.get('name', '')
        # Get commit date from local git (more reliable for tags)
        commit_date = get_tag_commit_date(release)

        return {
            'tag': tag_name,
            'published_at': commit_date,
            'is_correctness': is_correctness_release(release, commit_msg),
            'url': f"{REPO_URL}/releases/tag/{tag_name}",
        }

    # Parse from raw GitHub release format
    return {
        'tag': release.get('tag_name', ''),
        'published_at': release.get('published_at', ''),
        'is_correctness': is_correctness_release(release, commit_msg),
        'url': release.get('html_url', ''),
    }


def main():
    """Main entry point."""
    import argparse

    parser = argparse.ArgumentParser(
        description='Fetch ARMOR versions from GitHub API (git tags)',
        epilog='Examples:\n'
               '  %(prog)s                 # Fetch all tags (uses cache if fresh)\n'
               '  %(prog)s --limit 10      # Fetch latest 10 tags\n'
               '  %(prog)s --refresh       # Force refresh from GitHub API\n'
               '  %(prog)s --no-cache       # Bypass cache entirely\n'
               '  %(prog)s --clear-cache    # Clear cached data and exit',
        formatter_class=argparse.RawDescriptionHelpFormatter
    )

    parser.add_argument(
        '--limit', '-n',
        type=int,
        help='Maximum number of tags to fetch (default: all)'
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
    parser.add_argument(
        '--use-releases',
        action='store_true',
        help='Use GitHub Releases API instead of git tags'
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

    # Fetch tags or releases based on flag
    if args.use_releases:
        raw_items = fetch_releases(
            limit=args.limit,
            use_cache=use_cache,
            force_refresh=force_refresh,
            ttl_seconds=cache_ttl
        )
        item_type = "release(s)"
    else:
        raw_items = fetch_git_tags(
            limit=args.limit,
            use_cache=use_cache,
            force_refresh=force_refresh,
            ttl_seconds=cache_ttl
        )
        item_type = "tag(s)"

    if not raw_items:
        print(f"No {item_type} found", file=sys.stderr)
        print(json.dumps([], indent=2))
        return 0

    # Parse items with commit messages
    parsed_items = []
    repo_path = Path.cwd()
    for item in raw_items:
        tag_name = item.get('name', item.get('tag_name', ''))
        commit_msg = get_commit_message_for_tag(tag_name, repo_path) if not args.use_releases else ""
        parsed_item = parse_release(item, commit_msg)
        parsed_items.append(parsed_item)

    # Sort by version number (descending) for git tags
    if not args.use_releases:
        parsed_items.sort(key=lambda x: parse_version_number(x.get('tag', '')), reverse=True)

    # Save to cache if we fetched from API and cache is enabled
    if use_cache and not args.no_cache:
        # Only cache if we actually fetched from API (not from existing cache)
        if force_refresh or not is_cache_valid(cache_ttl):
            save_releases_to_cache(parsed_items)

    # Output as JSON
    print(json.dumps(parsed_items, indent=2))

    # Print summary to stderr
    correctness_count = sum(1 for r in parsed_items if r['is_correctness'])
    print(f"Fetched {len(parsed_items)} {item_type}", file=sys.stderr)
    print(f"Correctness-labeled: {correctness_count}", file=sys.stderr)
    print(f"Routine version bumps: {len(parsed_items) - correctness_count}", file=sys.stderr)

    return 0


def parse_version_number(tag: str) -> int:
    """Parse version tag to integer for sorting (e.g., 'v0.1.42' -> 42)."""
    import re
    match = re.match(r'v?0\.1\.(\d+)', tag)
    if match:
        return int(match.group(1))
    return 0


if __name__ == '__main__':
    sys.exit(main())
