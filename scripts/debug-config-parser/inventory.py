"""
Debug File Inventory Reader
Locates and categorizes all debug configuration files in the ARMOR workspace.

This module provides comprehensive file discovery and inventory management for
debug configuration files, supporting YAML, JSON, and TOML formats.
"""

from pathlib import Path
from typing import Dict, List, Optional, Set, Any
from enum import Enum
from dataclasses import dataclass, field


class FileType(Enum):
    """Configuration file types."""
    YAML = 'yaml'
    JSON = 'json'
    TOML = 'toml'
    UNKNOWN = 'unknown'


@dataclass
class FileEntry:
    """Represents a single configuration file entry in the inventory."""
    path: Path                      # Absolute path to the file
    relative_path: Path             # Path relative to workspace root
    file_type: FileType             # Detected file type
    size: int = 0                   # File size in bytes
    is_empty: bool = False          # Whether file is empty

    def to_dict(self) -> Dict[str, Any]:
        """Convert entry to dictionary representation."""
        return {
            'path': str(self.path),
            'relative_path': str(self.relative_path),
            'file_type': self.file_type.value,
            'size': self.size,
            'is_empty': self.is_empty
        }


@dataclass
class InventorySummary:
    """Summary statistics for the file inventory."""
    total_files: int = 0
    yaml_files: int = 0
    json_files: int = 0
    toml_files: int = 0
    empty_files: int = 0
    total_size: int = 0
    excluded_dirs: Set[str] = field(default_factory=set)

    def to_dict(self) -> Dict[str, Any]:
        """Convert summary to dictionary representation."""
        return {
            'total_files': self.total_files,
            'yaml_files': self.yaml_files,
            'json_files': self.json_files,
            'toml_files': self.toml_files,
            'empty_files': self.empty_files,
            'total_size': self.total_size,
            'excluded_dirs': sorted(self.excluded_dirs)
        }


@dataclass
class DebugFileInventory:
    """Complete inventory of debug configuration files."""
    workspace: Path
    entries: List[FileEntry] = field(default_factory=list)
    summary: InventorySummary = field(default_factory=InventorySummary)

    def add_entry(self, entry: FileEntry) -> None:
        """Add a file entry to the inventory."""
        self.entries.append(entry)

        # Update summary
        self.summary.total_files += 1
        self.summary.total_size += entry.size

        if entry.file_type == FileType.YAML:
            self.summary.yaml_files += 1
        elif entry.file_type == FileType.JSON:
            self.summary.json_files += 1
        elif entry.file_type == FileType.TOML:
            self.summary.toml_files += 1

        if entry.is_empty:
            self.summary.empty_files += 1

    def get_by_type(self, file_type: FileType) -> List[FileEntry]:
        """Get all entries of a specific file type."""
        return [entry for entry in self.entries if entry.file_type == file_type]

    def get_empty_files(self) -> List[FileEntry]:
        """Get all empty files in the inventory."""
        return [entry for entry in self.entries if entry.is_empty]

    def filter_by_path(self, pattern: str) -> List[FileEntry]:
        """Filter entries by path pattern."""
        pattern_lower = pattern.lower()
        return [
            entry for entry in self.entries
            if pattern_lower in str(entry.relative_path).lower()
        ]

    def to_dict(self) -> Dict[str, Any]:
        """Convert inventory to dictionary representation."""
        return {
            'workspace': str(self.workspace),
            'summary': self.summary.to_dict(),
            'entries': [entry.to_dict() for entry in self.entries]
        }


class DebugFileInventoryReader:
    """Reader for discovering and cataloging debug configuration files."""

    # Default file patterns to match
    DEFAULT_PATTERNS = ['*.yaml', '*.yml', '*.json', '*.toml']

    # Default directories to exclude
    DEFAULT_EXCLUDE_DIRS = {
        '.git',
        '.beads',
        'target',
        'node_modules',
        'logs',
        '.cache',
        '__pycache__',
        '.pytest_cache',
        'dist',
        'build'
    }

    def __init__(
        self,
        workspace: str = "/home/coding/ARMOR",
        patterns: Optional[List[str]] = None,
        exclude_dirs: Optional[Set[str]] = None
    ):
        """
        Initialize the inventory reader.

        Args:
            workspace: Path to the workspace root
            patterns: File patterns to match (default: *.yaml, *.yml, *.json, *.toml)
            exclude_dirs: Directory names to exclude from scanning
        """
        self.workspace = Path(workspace)
        self.patterns = patterns or self.DEFAULT_PATTERNS
        self.exclude_dirs = exclude_dirs or self.DEFAULT_EXCLUDE_DIRS.copy()

        # Extension to FileType mapping
        self._extension_map = {
            '.yaml': FileType.YAML,
            '.yml': FileType.YAML,
            '.json': FileType.JSON,
            '.toml': FileType.TOML,
        }

    def detect_file_type(self, filepath: Path) -> FileType:
        """
        Detect file type based on extension.

        Args:
            filepath: Path to the file

        Returns:
            FileType enum value
        """
        suffix = filepath.suffix.lower()
        return self._extension_map.get(suffix, FileType.UNKNOWN)

    def is_excluded(self, filepath: Path) -> bool:
        """
        Check if a file path should be excluded from inventory.

        Args:
            filepath: Path to check

        Returns:
            True if path should be excluded
        """
        # Check if any excluded directory is in the path
        for part in filepath.parts:
            if part in self.exclude_dirs:
                return True
        return False

    def scan_file(self, filepath: Path) -> Optional[FileEntry]:
        """
        Scan a single file and create an inventory entry.

        Args:
            filepath: Path to the file

        Returns:
            FileEntry if file is valid config file, None otherwise
        """
        # Skip excluded paths
        if self.is_excluded(filepath):
            return None

        # Detect file type
        file_type = self.detect_file_type(filepath)
        if file_type == FileType.UNKNOWN:
            return None

        # Get file info
        try:
            stat = filepath.stat()
            size = stat.st_size
            is_empty = size == 0
        except (OSError, IOError):
            size = 0
            is_empty = True

        # Create entry
        relative_path = filepath.relative_to(self.workspace)

        return FileEntry(
            path=filepath,
            relative_path=relative_path,
            file_type=file_type,
            size=size,
            is_empty=is_empty
        )

    def create_inventory(self) -> DebugFileInventory:
        """
        Scan workspace and create complete file inventory.

        Returns:
            DebugFileInventory containing all discovered config files
        """
        inventory = DebugFileInventory(
            workspace=self.workspace,
            summary=InventorySummary(excluded_dirs=self.exclude_dirs.copy())
        )

        # Scan for all matching patterns
        for pattern in self.patterns:
            for filepath in self.workspace.rglob(pattern):
                entry = self.scan_file(filepath)
                if entry:
                    inventory.add_entry(entry)

        # Sort entries by relative path
        inventory.entries.sort(key=lambda e: str(e.relative_path))

        return inventory

    def get_inventory_dict(self) -> Dict[str, Any]:
        """
        Get inventory as a dictionary for JSON serialization.

        Returns:
            Dict representation of the inventory
        """
        inventory = self.create_inventory()
        return inventory.to_dict()

    def print_inventory(self, inventory: Optional[DebugFileInventory] = None) -> None:
        """
        Print inventory summary to console.

        Args:
            inventory: Inventory to print (creates new if None)
        """
        if inventory is None:
            inventory = self.create_inventory()

        print("Debug Configuration File Inventory")
        print("=" * 70)
        print(f"Workspace: {self.workspace}")
        print(f"File patterns: {', '.join(self.patterns)}")
        print(f"Excluded directories: {', '.join(sorted(self.exclude_dirs))}")
        print()

        summary = inventory.summary
        print(f"Total files discovered: {summary.total_files}")
        print(f"  YAML files:  {summary.yaml_files}")
        print(f"  JSON files:  {summary.json_files}")
        print(f"  TOML files:  {summary.toml_files}")
        print(f"  Empty files: {summary.empty_files}")
        print(f"  Total size:  {summary.total_size:,} bytes")
        print()

        # Show breakdown by file type
        if summary.yaml_files > 0:
            yaml_entries = inventory.get_by_type(FileType.YAML)
            print(f"YAML files ({len(yaml_entries)}):")
            for entry in yaml_entries[:10]:  # Show first 10
                status = " (empty)" if entry.is_empty else ""
                print(f"  - {entry.relative_path}{status}")
            if len(yaml_entries) > 10:
                print(f"  ... and {len(yaml_entries) - 10} more")
            print()

        if summary.json_files > 0:
            json_entries = inventory.get_by_type(FileType.JSON)
            print(f"JSON files ({len(json_entries)}):")
            for entry in json_entries[:10]:  # Show first 10
                status = " (empty)" if entry.is_empty else ""
                print(f"  - {entry.relative_path}{status}")
            if len(json_entries) > 10:
                print(f"  ... and {len(json_entries) - 10} more")
            print()

        if summary.toml_files > 0:
            toml_entries = inventory.get_by_type(FileType.TOML)
            print(f"TOML files ({len(toml_entries)}):")
            for entry in toml_entries[:10]:  # Show first 10
                status = " (empty)" if entry.is_empty else ""
                print(f"  - {entry.relative_path}{status}")
            if len(toml_entries) > 10:
                print(f"  ... and {len(toml_entries) - 10} more")
            print()

    def get_file_list(self) -> List[str]:
        """
        Get a simple list of all discovered configuration file paths.

        Returns:
            List of absolute file paths
        """
        inventory = self.create_inventory()
        return [str(entry.path) for entry in inventory.entries]

    def get_relative_file_list(self) -> List[str]:
        """
        Get a list of relative file paths.

        Returns:
            List of relative file paths (strings)
        """
        inventory = self.create_inventory()
        return [str(entry.relative_path) for entry in inventory.entries]


def create_inventory(
    workspace: str = "/home/coding/ARMOR",
    patterns: Optional[List[str]] = None,
    exclude_dirs: Optional[Set[str]] = None
) -> DebugFileInventory:
    """
    Convenience function to create a file inventory.

    Args:
        workspace: Path to workspace root
        patterns: File patterns to match
        exclude_dirs: Directories to exclude

    Returns:
        DebugFileInventory containing all discovered config files
    """
    reader = DebugFileInventoryReader(workspace, patterns, exclude_dirs)
    return reader.create_inventory()


def main():
    """Main entry point for inventory command-line usage."""
    import argparse
    import json

    parser = argparse.ArgumentParser(
        description='Create inventory of debug configuration files in ARMOR workspace'
    )
    parser.add_argument(
        '--workspace',
        default='/home/coding/ARMOR',
        help='Path to ARMOR workspace (default: /home/coding/ARMOR)'
    )
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output inventory as JSON'
    )
    parser.add_argument(
        '--files',
        action='store_true',
        help='Output just list of file paths (one per line)'
    )
    parser.add_argument(
        '--relative',
        action='store_true',
        help='Use relative paths instead of absolute'
    )

    args = parser.parse_args()

    reader = DebugFileInventoryReader(args.workspace)

    if args.json:
        inventory_dict = reader.get_inventory_dict()
        print(json.dumps(inventory_dict, indent=2))
    elif args.files:
        if args.relative:
            files = reader.get_relative_file_list()
        else:
            files = reader.get_file_list()
        for filepath in files:
            print(filepath)
    else:
        reader.print_inventory()


if __name__ == "__main__":
    main()
