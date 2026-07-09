#!/usr/bin/env python3
"""
Tests for the Debug File Inventory Reader
Verifies that the inventory reader meets all acceptance criteria.
"""

import sys
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory

# Add scripts to path
scripts_dir = Path(__file__).parent.parent / 'scripts'
sys.path.insert(0, str(scripts_dir))

# Import inventory module directly
import importlib.util
spec = importlib.util.spec_from_file_location(
    "inventory",
    scripts_dir / "debug-config-parser" / "inventory.py"
)
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)

DebugFileInventoryReader = inventory.DebugFileInventoryReader
DebugFileInventory = inventory.DebugFileInventory
FileEntry = inventory.FileEntry
FileType = inventory.FileType
InventorySummary = inventory.InventorySummary
create_inventory = inventory.create_inventory


class TestDebugFileInventoryReader(unittest.TestCase):
    """Test suite for DebugFileInventoryReader."""

    def setUp(self):
        """Set up test fixtures with temporary directory."""
        self.temp_dir = TemporaryDirectory()
        self.workspace = Path(self.temp_dir.name)

        # Create some test config files
        (self.workspace / "config.yaml").write_text("key: value\n")
        (self.workspace / "settings.yml").write_text("setting: true\n")
        (self.workspace / "data.json").write_text('{"key": "value"}\n')
        (self.workspace / "config.toml").write_text("[section]\nkey = 'value'\n")

        # Create subdirectory with config
        subdir = self.workspace / "subdir"
        subdir.mkdir()
        (subdir / "nested.yaml").write_text("nested: data\n")

        # Create directories that should be excluded
        target_dir = self.workspace / "target"
        target_dir.mkdir()
        (target_dir / "excluded.yaml").write_text("excluded: true\n")

        node_modules = self.workspace / "node_modules"
        node_modules.mkdir()
        (node_modules / "config.json").write_text('{"excluded": true}\n')

        git_dir = self.workspace / ".git"
        git_dir.mkdir()
        (git_dir / "config.yaml").write_text("excluded: true\n")

    def tearDown(self):
        """Clean up temporary directory."""
        self.temp_dir.cleanup()

    def test_finds_all_config_files(self):
        """Test that inventory reader finds all config files in workspace."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # Should find 5 config files (not counting excluded directories)
        self.assertEqual(inventory.summary.total_files, 5)

    def test_file_type_detection_yaml(self):
        """Test that .yaml and .yml files are properly detected as YAML."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # Should have 3 YAML files (config.yaml, settings.yml, nested.yaml)
        self.assertEqual(inventory.summary.yaml_files, 3)

        yaml_entries = inventory.get_by_type(FileType.YAML)
        file_names = [e.relative_path.name for e in yaml_entries]
        self.assertIn("config.yaml", file_names)
        self.assertIn("settings.yml", file_names)
        self.assertIn("nested.yaml", file_names)

    def test_file_type_detection_json(self):
        """Test that .json files are properly detected as JSON."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        self.assertEqual(inventory.summary.json_files, 1)

        json_entries = inventory.get_by_type(FileType.JSON)
        self.assertEqual(len(json_entries), 1)
        self.assertEqual(json_entries[0].relative_path.name, "data.json")

    def test_file_type_detection_toml(self):
        """Test that .toml files are properly detected as TOML."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        self.assertEqual(inventory.summary.toml_files, 1)

        toml_entries = inventory.get_by_type(FileType.TOML)
        self.assertEqual(len(toml_entries), 1)
        self.assertEqual(toml_entries[0].relative_path.name, "config.toml")

    def test_excludes_target_directory(self):
        """Test that target/ directory is excluded from inventory."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # target/excluded.yaml should not be in inventory
        for entry in inventory.entries:
            self.assertNotIn("target", str(entry.relative_path))

    def test_excludes_node_modules_directory(self):
        """Test that node_modules/ directory is excluded from inventory."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # node_modules/config.json should not be in inventory
        for entry in inventory.entries:
            self.assertNotIn("node_modules", str(entry.relative_path))

    def test_excludes_git_directory(self):
        """Test that .git/ directory is excluded from inventory."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # .git/config.yaml should not be in inventory
        for entry in inventory.entries:
            self.assertNotIn(".git", str(entry.relative_path))

    def test_returns_structured_inventory(self):
        """Test that inventory returns structured data with file paths and types."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # Check inventory structure
        self.assertIsInstance(inventory.workspace, Path)
        self.assertIsInstance(inventory.entries, list)
        self.assertIsInstance(inventory.summary, InventorySummary)

        # Check entry structure
        for entry in inventory.entries:
            self.assertIsInstance(entry, FileEntry)
            self.assertIsInstance(entry.path, Path)
            self.assertIsInstance(entry.relative_path, Path)
            self.assertIsInstance(entry.file_type, FileType)
            self.assertIsInstance(entry.size, int)
            self.assertIsInstance(entry.is_empty, bool)

    def test_inventory_to_dict_conversion(self):
        """Test that inventory can be converted to dictionary for JSON serialization."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        inventory_dict = inventory.to_dict()

        # Check dictionary structure
        self.assertIn('workspace', inventory_dict)
        self.assertIn('summary', inventory_dict)
        self.assertIn('entries', inventory_dict)

        # Check summary structure
        summary = inventory_dict['summary']
        self.assertIn('total_files', summary)
        self.assertIn('yaml_files', summary)
        self.assertIn('json_files', summary)
        self.assertIn('toml_files', summary)

    def test_get_file_list(self):
        """Test that get_file_list returns list of file paths."""
        reader = DebugFileInventoryReader(str(self.workspace))
        files = reader.get_file_list()

        self.assertIsInstance(files, list)
        self.assertEqual(len(files), 5)
        self.assertTrue(all(isinstance(f, str) for f in files))
        self.assertTrue(all(Path(f).is_absolute() for f in files))

    def test_get_relative_file_list(self):
        """Test that get_relative_file_list returns relative paths."""
        reader = DebugFileInventoryReader(str(self.workspace))
        files = reader.get_relative_file_list()

        self.assertIsInstance(files, list)
        self.assertEqual(len(files), 5)
        self.assertTrue(all(isinstance(f, str) for f in files))
        # Should not contain absolute paths
        self.assertFalse(any(Path(f).is_absolute() for f in files))

    def test_custom_exclude_dirs(self):
        """Test that custom exclude directories work correctly."""
        # Create a custom reader that also excludes 'subdir'
        exclude_dirs = {'target', 'node_modules', '.git', 'subdir'}
        reader = DebugFileInventoryReader(
            str(self.workspace),
            exclude_dirs=exclude_dirs
        )
        inventory = reader.create_inventory()

        # Should not include nested.yaml in subdir
        for entry in inventory.entries:
            self.assertNotIn("subdir", str(entry.relative_path))

        self.assertEqual(inventory.summary.total_files, 4)  # Excludes subdir/nested.yaml

    def test_custom_patterns(self):
        """Test that custom file patterns work correctly."""
        # Only look for YAML files
        reader = DebugFileInventoryReader(
            str(self.workspace),
            patterns=['*.yaml', '*.yml']
        )
        inventory = reader.create_inventory()

        # Should only find YAML files, no JSON or TOML
        self.assertEqual(inventory.summary.yaml_files, 3)
        self.assertEqual(inventory.summary.json_files, 0)
        self.assertEqual(inventory.summary.toml_files, 0)
        self.assertEqual(inventory.summary.total_files, 3)

    def test_empty_file_detection(self):
        """Test that empty files are properly detected."""
        # Create an empty file
        (self.workspace / "empty.yaml").write_text("")

        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # Should have one empty file
        self.assertEqual(inventory.summary.empty_files, 1)
        empty_entries = inventory.get_empty_files()
        self.assertEqual(len(empty_entries), 1)
        self.assertEqual(empty_entries[0].relative_path.name, "empty.yaml")

    def test_filter_by_path(self):
        """Test filtering entries by path pattern."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # Filter for 'config' pattern
        filtered = inventory.filter_by_path('config')
        # Should find config.yaml and config.toml
        self.assertEqual(len(filtered), 2)

    def test_convenience_function(self):
        """Test the convenience function for creating inventory."""
        inventory = create_inventory(str(self.workspace))

        self.assertIsInstance(inventory, DebugFileInventory)
        self.assertEqual(inventory.summary.total_files, 5)

    def test_file_entry_to_dict(self):
        """Test that FileEntry can be converted to dictionary."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        entry_dict = inventory.entries[0].to_dict()

        self.assertIn('path', entry_dict)
        self.assertIn('relative_path', entry_dict)
        self.assertIn('file_type', entry_dict)
        self.assertIn('size', entry_dict)
        self.assertIn('is_empty', entry_dict)

    def test_batch_validation_ready(self):
        """Test that inventory is ready for batch validation integration."""
        reader = DebugFileInventoryReader(str(self.workspace))
        inventory = reader.create_inventory()

        # Get file paths for batch processing
        file_paths = reader.get_file_list()

        # All paths should be valid files that can be validated
        for filepath in file_paths:
            path = Path(filepath)
            self.assertTrue(path.is_file())
            self.assertTrue(path.exists())


class TestInventoryIntegration(unittest.TestCase):
    """Integration tests for inventory reader with real ARMOR workspace."""

    def test_real_workspace_inventory(self):
        """Test inventory creation on actual ARMOR workspace."""
        reader = DebugFileInventoryReader("/home/coding/ARMOR")
        inventory = reader.create_inventory()

        # Basic sanity checks
        self.assertGreater(inventory.summary.total_files, 0)
        self.assertIsInstance(inventory.entries, list)

        # All entries should have valid paths
        for entry in inventory.entries:
            self.assertTrue(entry.path.exists())
            self.assertTrue(entry.path.is_file())


def run_tests():
    """Run all tests and report results."""
    print("=" * 70)
    print("Running Debug File Inventory Reader Tests")
    print("=" * 70)
    print()

    # Run tests
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()

    suite.addTests(loader.loadTestsFromTestCase(TestDebugFileInventoryReader))
    suite.addTests(loader.loadTestsFromTestCase(TestInventoryIntegration))

    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    print()
    print("=" * 70)
    print(f"Tests run: {result.testsRun}")
    print(f"Successes: {result.testsRun - len(result.failures) - len(result.errors)}")
    print(f"Failures: {len(result.failures)}")
    print(f"Errors: {len(result.errors)}")
    print("=" * 70)

    return result.wasSuccessful()


if __name__ == "__main__":
    success = run_tests()
    sys.exit(0 if success else 1)
