#!/usr/bin/env python3
"""
JSON Output Validation Tests for ARMOR Pytest Parser

This test suite validates JSON output structure, field presence, and schema compliance
for all three pytest output formats.

Can be run as:
- pytest: pytest test_json_validation.py -v
- Standalone: python3 test_json_validation.py

Author: ARMOR Project (bf-2uvxls)
Created: 2026-07-13
"""

import sys
import json
from pathlib import Path
from typing import Dict, Any, List, Tuple, Optional

# Try importing pytest for test discovery, but work without it
try:
    import pytest
    HAS_PYTEST = True
except ImportError:
    HAS_PYTEST = False

# Add tools directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from parse_pytest_output import PytestOutputParser, extract_json_compatible_data, TestFailure


class JSONValidationTests:
    """Comprehensive JSON validation tests for pytest parser output."""

    # Test data paths
    TEST_SAMPLES_DIR = Path(__file__).parent / "test_samples"

    # Required fields that must be present and non-null
    REQUIRED_FIELDS = {
        'test_name': str,
        'test_file': str,
        'line_number': (int, type(None)),  # Can be None in some formats
        'error_type': str,
        'error_message': str,
        'assertion_line': (str, type(None)),
        'assertion_type': (str, type(None)),
        'expected': (str, type(None)),
        'actual': (str, type(None)),
        'diff_lines': list,
        'index_diff': (int, type(None)),
        'differing_items': list,
        'where_clause': (str, type(None))
    }

    # Fields that must be non-empty strings (if not None)
    # Note: test_name is optional for format 3
    NON_EMPTY_STRING_FIELDS = [
        'test_file',
        'error_type'
    ]

    def __init__(self):
        """Initialize test instance."""
        self.test_results = []
        self.pass_count = 0
        self.fail_count = 0

    def parse_to_json(self, output: str) -> List[Dict[str, Any]]:
        """Parse pytest output and convert to JSON-compatible format."""
        parser = PytestOutputParser()
        failures = parser.parse(output)
        return extract_json_compatible_data(failures)

    def load_samples(self) -> Tuple[str, str, str]:
        """Load all three sample formats."""
        samples = {}
        for i in range(1, 4):
            sample_file = self.TEST_SAMPLES_DIR / f"sample_format{i}_vv_short.txt" if i == 1 else \
                         (self.TEST_SAMPLES_DIR / f"sample_format{i}_v_long.txt" if i == 2 else \
                          self.TEST_SAMPLES_DIR / f"sample_format{i}_tb_line.txt")

            if not sample_file.exists():
                # Try alternate naming
                sample_file = self.TEST_SAMPLES_DIR / f"sample_format{i}.txt"

            with open(sample_file, 'r') as f:
                samples[f'format{i}'] = f.read()

        return samples['format1'], samples['format2'], samples['format3']

    def record_result(self, test_name: str, passed: bool, message: str = ""):
        """Record a test result."""
        self.test_results.append({
            'name': test_name,
            'passed': passed,
            'message': message
        })
        if passed:
            self.pass_count += 1
        else:
            self.fail_count += 1

    def test_json_is_parseable(self) -> bool:
        """Test that all sample formats produce parseable JSON."""
        print("\n" + "=" * 70)
        print("TEST: JSON is Parseable")
        print("=" * 70)

        format1, format2, format3 = self.load_samples()
        samples = [('format1', format1), ('format2', format2), ('format3', format3)]
        all_passed = True

        for i, (format_name, sample) in enumerate(samples, 1):
            try:
                json_data = self.parse_to_json(sample)

                # Should be able to serialize to JSON string
                json_str = json.dumps(json_data)

                # Should be able to deserialize back
                parsed_back = json.loads(json_str)

                # Should equal original
                if parsed_back == json_data:
                    print(f"  ✓ Format {i}: JSON parseable")
                    self.record_result(f"json_parseable_format{i}", True)
                else:
                    print(f"  ✗ Format {i}: JSON round-trip failed")
                    self.record_result(f"json_parseable_format{i}", False, "Round-trip failed")
                    all_passed = False
            except Exception as e:
                print(f"  ✗ Format {i}: Exception - {e}")
                self.record_result(f"json_parseable_format{i}", False, str(e))
                all_passed = False

        return all_passed

    def test_all_required_fields_present(self) -> bool:
        """Test that all required fields are present in JSON output."""
        print("\n" + "=" * 70)
        print("TEST: All Required Fields Present")
        print("=" * 70)

        format1, format2, format3 = self.load_samples()
        samples = [('format1', format1), ('format2', format2), ('format3', format3)]
        all_passed = True

        for i, (format_name, sample) in enumerate(samples, 1):
            json_data = self.parse_to_json(sample)

            if not json_data:
                print(f"  ⚠ Format {i}: No failures to validate (skipped)")
                self.record_result(f"required_fields_format{i}", True, "No failures")
                continue

            format_passed = True
            for field, expected_type in self.REQUIRED_FIELDS.items():
                for failure in json_data:
                    if field not in failure:
                        print(f"  ✗ Format {i}: Missing required field '{field}'")
                        self.record_result(f"required_field_{field}_format{i}", False, "Missing")
                        format_passed = False
                        all_passed = False
                    else:
                        # Check type compatibility
                        value = failure[field]
                        if isinstance(expected_type, tuple):
                            if not isinstance(value, expected_type):
                                print(f"  ✗ Format {i}: Field '{field}' has wrong type {type(value)}")
                                self.record_result(f"required_field_{field}_format{i}", False, f"Wrong type: {type(value)}")
                                format_passed = False
                                all_passed = False
                        else:
                            if not isinstance(value, expected_type):
                                print(f"  ✗ Format {i}: Field '{field}' has wrong type {type(value)}")
                                self.record_result(f"required_field_{field}_format{i}", False, f"Wrong type: {type(value)}")
                                format_passed = False
                                all_passed = False

            if format_passed:
                print(f"  ✓ Format {i}: All required fields present and correct types")

        return all_passed

    def test_non_empty_string_fields(self) -> bool:
        """Test that critical string fields are non-empty."""
        print("\n" + "=" * 70)
        print("TEST: Non-Empty String Fields")
        print("=" * 70)

        format1, format2, format3 = self.load_samples()
        samples = [('format1', format1), ('format2', format2), ('format3', format3)]
        all_passed = True

        for i, (format_name, sample) in enumerate(samples, 1):
            json_data = self.parse_to_json(sample)

            if not json_data:
                print(f"  ⚠ Format {i}: No failures to validate (skipped)")
                continue

            format_passed = True
            for failure in json_data:
                for field in self.NON_EMPTY_STRING_FIELDS:
                    value = failure[field]
                    if not value:
                        print(f"  ✗ Format {i}: Field '{field}' is empty or None")
                        self.record_result(f"nonempty_field_{field}_format{i}", False, "Empty")
                        format_passed = False
                        all_passed = False
                    elif not isinstance(value, str):
                        print(f"  ✗ Format {i}: Field '{field}' is not a string")
                        self.record_result(f"nonempty_field_{field}_format{i}", False, "Not string")
                        format_passed = False
                        all_passed = False
                    elif not value.strip():
                        print(f"  ✗ Format {i}: Field '{field}' is whitespace-only")
                        self.record_result(f"nonempty_field_{field}_format{i}", False, "Whitespace only")
                        format_passed = False
                        all_passed = False

            if format_passed:
                print(f"  ✓ Format {i}: All non-empty string fields valid")

        return all_passed

    def test_structure_matches_schema(self) -> bool:
        """Test that JSON structure matches expected schema."""
        print("\n" + "=" * 70)
        print("TEST: Structure Matches Schema")
        print("=" * 70)

        format1, format2, format3 = self.load_samples()
        samples = [('format1', format1), ('format2', format2), ('format3', format3)]
        all_passed = True

        for i, (format_name, sample) in enumerate(samples, 1):
            json_data = self.parse_to_json(sample)

            if not json_data:
                print(f"  ⚠ Format {i}: No failures to validate (skipped)")
                continue

            format_passed = True
            for failure in json_data:
                # Must be a dictionary
                if not isinstance(failure, dict):
                    print(f"  ✗ Format {i}: Failure is not a dict")
                    self.record_result(f"structure_dict_format{i}", False, "Not dict")
                    format_passed = False
                    all_passed = False
                    continue

                # diff_lines must be a list of strings (if present)
                if failure.get('diff_lines'):
                    if not isinstance(failure['diff_lines'], list):
                        print(f"  ✗ Format {i}: 'diff_lines' is not a list")
                        self.record_result(f"structure_diff_list_format{i}", False, "Not list")
                        format_passed = False
                        all_passed = False
                    else:
                        for line in failure['diff_lines']:
                            if not isinstance(line, str):
                                print(f"  ✗ Format {i}: diff line is not a string")
                                self.record_result(f"structure_diff_str_format{i}", False, "Not string")
                                format_passed = False
                                all_passed = False

                # differing_items must be a list of dicts with specific structure (if present)
                if failure.get('differing_items'):
                    if not isinstance(failure['differing_items'], list):
                        print(f"  ✗ Format {i}: 'differing_items' is not a list")
                        self.record_result(f"structure_diff_items_list_format{i}", False, "Not list")
                        format_passed = False
                        all_passed = False
                    else:
                        for item in failure['differing_items']:
                            if not isinstance(item, dict):
                                print(f"  ✗ Format {i}: differing item is not a dict")
                                self.record_result(f"structure_diff_items_dict_format{i}", False, "Not dict")
                                format_passed = False
                                all_passed = False
                            else:
                                for required_key in ['key', 'expected', 'actual']:
                                    if required_key not in item:
                                        print(f"  ✗ Format {i}: differing item missing '{required_key}'")
                                        self.record_result(f"structure_diff_items_{required_key}_format{i}", False, "Missing")
                                        format_passed = False
                                        all_passed = False

                # line_number must be positive integer or None
                line_number = failure.get('line_number')
                if line_number is not None:
                    if not isinstance(line_number, int):
                        print(f"  ✗ Format {i}: line_number is not an int")
                        self.record_result(f"structure_line_int_format{i}", False, "Not int")
                        format_passed = False
                        all_passed = False
                    elif line_number <= 0:
                        print(f"  ✗ Format {i}: line_number is not positive")
                        self.record_result(f"structure_line_positive_format{i}", False, "Not positive")
                        format_passed = False
                        all_passed = False

                # index_diff must be non-negative integer or None
                index_diff = failure.get('index_diff')
                if index_diff is not None:
                    if not isinstance(index_diff, int):
                        print(f"  ✗ Format {i}: index_diff is not an int")
                        self.record_result(f"structure_index_int_format{i}", False, "Not int")
                        format_passed = False
                        all_passed = False
                    elif index_diff < 0:
                        print(f"  ✗ Format {i}: index_diff is negative")
                        self.record_result(f"structure_index_nonneg_format{i}", False, "Negative")
                        format_passed = False
                        all_passed = False

            if format_passed:
                print(f"  ✓ Format {i}: Structure matches schema")

        return all_passed

    def test_json_serialization_roundtrip(self) -> bool:
        """Test that JSON can be serialized and deserialized without data loss."""
        print("\n" + "=" * 70)
        print("TEST: JSON Serialization Round-trip")
        print("=" * 70)

        format1, format2, format3 = self.load_samples()
        samples = [('format1', format1), ('format2', format2), ('format3', format3)]
        all_passed = True

        for i, (format_name, sample) in enumerate(samples, 1):
            json_data = self.parse_to_json(sample)

            try:
                # Serialize to JSON
                json_str = json.dumps(json_data, indent=2)

                # Deserialize
                parsed_data = json.loads(json_str)

                # Compare
                if parsed_data != json_data:
                    print(f"  ✗ Format {i}: Round-trip failed")
                    self.record_result(f"roundtrip_format{i}", False, "Data changed")
                    all_passed = False
                    continue

                # Verify all original fields are present
                for original, parsed in zip(json_data, parsed_data):
                    if set(original.keys()) != set(parsed.keys()):
                        print(f"  ✗ Format {i}: Keys changed after round-trip")
                        self.record_result(f"roundtrip_format{i}", False, "Keys changed")
                        all_passed = False
                        break
                else:
                    print(f"  ✓ Format {i}: Round-trip successful")
                    self.record_result(f"roundtrip_format{i}", True)
            except Exception as e:
                print(f"  ✗ Format {i}: Exception - {e}")
                self.record_result(f"roundtrip_format{i}", False, str(e))
                all_passed = False

        return all_passed

    def test_expected_failures_count(self) -> bool:
        """Test that each format produces expected number of failures."""
        print("\n" + "=" * 70)
        print("TEST: Expected Failures Count")
        print("=" * 70)

        format1, format2, format3 = self.load_samples()
        samples = [('format1', format1, 3), ('format2', format2, 3), ('format3', format3, 15)]
        all_passed = True

        for format_name, sample, expected in samples:
            format_num = format_name.replace('format', '')
            json_data = self.parse_to_json(sample)
            actual = len(json_data)

            if actual == expected:
                print(f"  ✓ Format {format_num}: {actual} failures (expected {expected})")
                self.record_result(f"failures_count_format{format_num}", True)
            else:
                print(f"  ✗ Format {format_num}: {actual} failures (expected {expected})")
                self.record_result(f"failures_count_format{format_num}", False, f"Got {actual}, expected {expected}")
                all_passed = False

        return all_passed

    def test_specific_field_values_format1(self) -> bool:
        """Test specific expected values for format 1."""
        print("\n" + "=" * 70)
        print("TEST: Specific Field Values - Format 1")
        print("=" * 70)

        format1, _, _ = self.load_samples()
        json_data = self.parse_to_json(format1)

        expected_tests = ['test_simple_equality', 'test_dict_equality', 'test_list_comparison']
        actual_tests = [f['test_name'] for f in json_data]

        if actual_tests == expected_tests:
            print(f"  ✓ Test names match: {actual_tests}")
            self.record_result("field_values_format1_names", True)
        else:
            print(f"  ✗ Test names mismatch: expected {expected_tests}, got {actual_tests}")
            self.record_result("field_values_format1_names", False, f"Names mismatch")
            return False

        # Test dict_equality has differing_items
        dict_failure = next((f for f in json_data if f['test_name'] == 'test_dict_equality'), None)
        if dict_failure and len(dict_failure['differing_items']) > 0:
            print(f"  ✓ test_dict_equality has differing_items")
            self.record_result("field_values_format1_dict", True)
        else:
            print(f"  ✗ test_dict_equality missing differing_items")
            self.record_result("field_values_format1_dict", False, "Missing differing_items")
            return False

        # Test list_comparison has index_diff
        list_failure = next((f for f in json_data if f['test_name'] == 'test_list_comparison'), None)
        if list_failure and list_failure['index_diff'] == 2:
            print(f"  ✓ test_list_comparison has index_diff=2")
            self.record_result("field_values_format1_list", True)
        else:
            print(f"  ✗ test_list_comparison missing or wrong index_diff")
            self.record_result("field_values_format1_list", False, "Missing/wrong index_diff")
            return False

        return True

    def test_all_samples_accessible(self) -> bool:
        """Test that all sample files are accessible."""
        print("\n" + "=" * 70)
        print("TEST: Sample Files Accessible")
        print("=" * 70)

        all_passed = True

        if not self.TEST_SAMPLES_DIR.exists():
            print(f"  ✗ Test samples directory not found: {self.TEST_SAMPLES_DIR}")
            return False

        print(f"  ✓ Test samples directory exists")

        required_files = [
            "sample_format1_vv_short.txt",
            "sample_format2_v_long.txt",
            "sample_format3_tb_line.txt"
        ]

        for filename in required_files:
            file_path = self.TEST_SAMPLES_DIR / filename
            if not file_path.exists():
                # Try alternate naming
                alt_path = self.TEST_SAMPLES_DIR / filename.replace("_vv_short", "").replace("_v_long", "").replace("_tb_line", "")
                if alt_path.exists():
                    print(f"  ✓ Sample file exists (alternate): {alt_path.name}")
                    self.record_result(f"sample_file_{filename}", True)
                else:
                    print(f"  ✗ Sample file missing: {filename}")
                    self.record_result(f"sample_file_{filename}", False, "Not found")
                    all_passed = False
            elif file_path.stat().st_size == 0:
                print(f"  ✗ Sample file is empty: {filename}")
                self.record_result(f"sample_file_{filename}", False, "Empty")
                all_passed = False
            else:
                size = file_path.stat().st_size
                print(f"  ✓ Sample file exists: {filename} ({size} bytes)")
                self.record_result(f"sample_file_{filename}", True)

        return all_passed

    def run_all_tests(self) -> bool:
        """Run all tests and return overall success status."""
        print("=" * 70)
        print("ARMOR JSON OUTPUT VALIDATION TESTS")
        print("=" * 70)
        print(f"Test samples directory: {self.TEST_SAMPLES_DIR}")

        # Run all test methods
        test_methods = [
            ('Sample Files Accessible', self.test_all_samples_accessible),
            ('JSON Parseable', self.test_json_is_parseable),
            ('Required Fields Present', self.test_all_required_fields_present),
            ('Non-Empty String Fields', self.test_non_empty_string_fields),
            ('Structure Matches Schema', self.test_structure_matches_schema),
            ('JSON Serialization Round-trip', self.test_json_serialization_roundtrip),
            ('Expected Failures Count', self.test_expected_failures_count),
            ('Specific Field Values Format 1', self.test_specific_field_values_format1),
        ]

        for test_name, test_method in test_methods:
            try:
                test_method()
            except Exception as e:
                print(f"\n  ✗ EXCEPTION in {test_name}: {e}")
                self.record_result(f"exception_{test_name}", False, str(e))

        # Print summary
        print("\n" + "=" * 70)
        print("TEST SUMMARY")
        print("=" * 70)
        print(f"Total tests: {self.pass_count + self.fail_count}")
        print(f"Passed: {self.pass_count}")
        print(f"Failed: {self.fail_count}")

        if self.fail_count > 0:
            print("\nFailed tests:")
            for result in self.test_results:
                if not result['passed']:
                    print(f"  - {result['name']}: {result['message']}")
            print("\n" + "=" * 70)
            print("✗ SOME TESTS FAILED")
            return False
        else:
            print("\n" + "=" * 70)
            print("✓ ALL TESTS PASSED")
            return True


def main():
    """Main entry point."""
    tests = JSONValidationTests()
    success = tests.run_all_tests()
    return 0 if success else 1


if __name__ == '__main__':
    sys.exit(main())


# If pytest is available, also expose as pytest-compatible tests
if HAS_PYTEST:
    # Re-export the test class for pytest discovery
    class TestJSONValidation(JSONValidationTests):
        """Pytest-compatible wrapper."""

        @pytest.fixture
        def sample_format1(self):
            """Load sample format 1 (--tb=short)."""
            return self.load_samples()[0]

        @pytest.fixture
        def sample_format2(self):
            """Load sample format 2 (--tb=long)."""
            return self.load_samples()[1]

        @pytest.fixture
        def sample_format3(self):
            """Load sample format 3 (--tb=line)."""
            return self.load_samples()[2]

        def test_json_is_parseable(self, sample_format1, sample_format2, sample_format3):
            """Test that all sample formats produce parseable JSON."""
            for i, sample in enumerate([sample_format1, sample_format2, sample_format3], 1):
                json_data = self.parse_to_json(sample)
                json_str = json.dumps(json_data)
                parsed_back = json.loads(json_str)
                assert parsed_back == json_data, f"Format {i}: JSON round-trip failed"

        def test_all_required_fields_present(self, sample_format1, sample_format2, sample_format3):
            """Test that all required fields are present in JSON output."""
            for i, sample in enumerate([sample_format1, sample_format2, sample_format3], 1):
                json_data = self.parse_to_json(sample)
                if not json_data:
                    pytest.skip(f"Format {i} produced no failures")
                for failure in json_data:
                    for field, expected_type in self.REQUIRED_FIELDS.items():
                        assert field in failure, f"Format {i}: Missing field '{field}'"
                        value = failure[field]
                        if isinstance(expected_type, tuple):
                            assert isinstance(value, expected_type), \
                                f"Format {i}: Field '{field}' wrong type"
                        else:
                            assert isinstance(value, expected_type), \
                                f"Format {i}: Field '{field}' wrong type"

        def test_structure_matches_schema(self, sample_format1, sample_format2, sample_format3):
            """Test that JSON structure matches expected schema."""
            for i, sample in enumerate([sample_format1, sample_format2, sample_format3], 1):
                json_data = self.parse_to_json(sample)
                if not json_data:
                    pytest.skip(f"Format {i} produced no failures")
                for failure in json_data:
                    assert isinstance(failure, dict), f"Format {i}: Not a dict"
                    if failure.get('diff_lines'):
                        assert isinstance(failure['diff_lines'], list)
                        for line in failure['diff_lines']:
                            assert isinstance(line, str)
                    if failure.get('differing_items'):
                        assert isinstance(failure['differing_items'], list)
                        for item in failure['differing_items']:
                            assert isinstance(item, dict)
                            assert 'key' in item
                            assert 'expected' in item
                            assert 'actual' in item

        def test_expected_failures_count(self, sample_format1, sample_format2, sample_format3):
            """Test that each format produces expected number of failures."""
            json_data1 = self.parse_to_json(sample_format1)
            json_data2 = self.parse_to_json(sample_format2)
            json_data3 = self.parse_to_json(sample_format3)
            assert len(json_data1) == 3, "Format 1: Expected 3 failures"
            assert len(json_data2) == 3, "Format 2: Expected 3 failures"
            assert len(json_data3) == 3, "Format 3: Expected 3 failures"

        def test_specific_field_values_format1(self, sample_format1):
            """Test specific expected values for format 1."""
            json_data = self.parse_to_json(sample_format1)
            expected_tests = ['test_simple_equality', 'test_dict_equality', 'test_list_comparison']
            actual_tests = [f['test_name'] for f in json_data]
            assert actual_tests == expected_tests

            dict_failure = next((f for f in json_data if f['test_name'] == 'test_dict_equality'), None)
            assert dict_failure is not None
            assert len(dict_failure['differing_items']) > 0

            list_failure = next((f for f in json_data if f['test_name'] == 'test_list_comparison'), None)
            assert list_failure is not None
            assert list_failure['index_diff'] == 2
