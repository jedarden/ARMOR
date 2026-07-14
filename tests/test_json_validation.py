#!/usr/bin/env python3
"""
JSON Output Validation Tests for ARMOR Pytest Parser

Tests validate that the parser produces valid, parseable JSON with all required
fields present and structure matching the documented schema.

Acceptance Criteria:
- Tests verify JSON is parseable
- Tests check for all required fields
- Tests validate structure matches schema
- All tests pass for the 3 sample formats
- Tests are runnable via pytest

Bead: bf-2uvxls
Created: 2026-07-13
"""

import json
import sys
from pathlib import Path
from typing import Dict, List, Any, Set
from dataclasses import dataclass

sys.path.insert(0, str(Path(__file__).parent.parent / "tools"))
from parse_pytest_output import PytestOutputParser, extract_json_compatible_data


@dataclass
class FormatSpec:
    """Schema specification for each pytest format."""
    name: str
    description: str
    # Core fields that must be present in the JSON output
    required_fields: Set[str]
    # Optional fields that may or may not be present
    optional_fields: Set[str]
    # All valid fields that should be in the JSON
    all_valid_fields: Set[str]


# Define the schema for each format based on the parser documentation
FORMAT_SCHEMAS = {
    'tb_short': FormatSpec(
        name='--tb=short',
        description='Default format with detailed diffs',
        required_fields={'test_file', 'line_number', 'error_type'},
        optional_fields={
            'test_name', 'error_message', 'assertion_line', 'assertion_type',
            'expected', 'actual', 'diff_lines', 'index_diff', 'differing_items',
            'where_clause'
        },
        all_valid_fields={
            'test_name', 'test_file', 'line_number', 'error_type', 'error_message',
            'assertion_line', 'assertion_type', 'expected', 'actual', 'diff_lines',
            'index_diff', 'differing_items', 'where_clause'
        }
    ),
    'tb_long': FormatSpec(
        name='--tb=long',
        description='Verbose format with full context',
        required_fields={'test_file', 'line_number', 'error_type'},
        optional_fields={
            'test_name', 'error_message', 'assertion_line', 'assertion_type',
            'expected', 'actual', 'diff_lines', 'index_diff', 'differing_items',
            'where_clause'
        },
        all_valid_fields={
            'test_name', 'test_file', 'line_number', 'error_type', 'error_message',
            'assertion_line', 'assertion_type', 'expected', 'actual', 'diff_lines',
            'index_diff', 'differing_items', 'where_clause'
        }
    ),
    'tb_line': FormatSpec(
        name='--tb=line',
        description='Compact format with one line per failure',
        required_fields={'test_file', 'line_number', 'error_type'},
        optional_fields={'test_name', 'error_message', 'expected', 'actual'},
        all_valid_fields={
            'test_name', 'test_file', 'line_number', 'error_type', 'error_message',
            'expected', 'actual'
        }
    )
}


class JSONStructureValidator:
    """Validate JSON output structure and required fields."""

    def __init__(self, samples_dir: str):
        self.samples_dir = Path(samples_dir)
        self.parser = PytestOutputParser()

    def load_sample(self, filename: str) -> str:
        """Load a sample pytest output file."""
        return (self.samples_dir / filename).read_text()

    def parse_sample(self, content: str) -> List[Dict[str, Any]]:
        """Parse sample and convert to JSON-compatible dictionaries."""
        failures = self.parser.parse(content)
        return extract_json_compatible_data(failures)

    def verify_json_parseable(self, json_text: str) -> bool:
        """Verify that JSON text can be parsed by Python json module."""
        try:
            json.loads(json_text)
            return True
        except json.JSONDecodeError:
            return False

    def verify_required_fields(self, failure_dict: Dict[str, Any],
                               format_spec: FormatSpec) -> List[str]:
        """Verify that all required fields are present and non-null."""
        missing = []
        for field in format_spec.required_fields:
            if field not in failure_dict:
                missing.append(f"Missing field: {field}")
            elif failure_dict[field] is None:
                missing.append(f"Null required field: {field}")
        return missing

    def verify_field_types(self, failure_dict: Dict[str, Any]) -> List[str]:
        """Verify that field types match expected types."""
        issues = []

        # String fields
        string_fields = ['test_name', 'test_file', 'error_type', 'error_message',
                        'assertion_line', 'assertion_type', 'expected', 'actual',
                        'where_clause']
        for field in string_fields:
            if field in failure_dict and failure_dict[field] is not None:
                if not isinstance(failure_dict[field], str):
                    issues.append(f"Field {field} should be str, got {type(failure_dict[field]).__name__}")

        # Integer fields
        if 'line_number' in failure_dict and failure_dict['line_number'] is not None:
            if not isinstance(failure_dict['line_number'], int):
                issues.append(f"Field line_number should be int, got {type(failure_dict['line_number']).__name__}")

        if 'index_diff' in failure_dict and failure_dict['index_diff'] is not None:
            if not isinstance(failure_dict['index_diff'], int):
                issues.append(f"Field index_diff should be int, got {type(failure_dict['index_diff']).__name__}")

        # List fields
        if 'diff_lines' in failure_dict and failure_dict['diff_lines'] is not None:
            if not isinstance(failure_dict['diff_lines'], list):
                issues.append(f"Field diff_lines should be list, got {type(failure_dict['diff_lines']).__name__}")
            elif failure_dict['diff_lines']:
                if not all(isinstance(line, str) for line in failure_dict['diff_lines']):
                    issues.append(f"Field diff_lines should contain only strings")

        if 'differing_items' in failure_dict and failure_dict['differing_items'] is not None:
            if not isinstance(failure_dict['differing_items'], list):
                issues.append(f"Field differing_items should be list, got {type(failure_dict['differing_items']).__name__}")
            elif failure_dict['differing_items']:
                if not all(isinstance(item, dict) for item in failure_dict['differing_items']):
                    issues.append(f"Field differing_items should contain only dicts")

        return issues

    def verify_no_extra_fields(self, failure_dict: Dict[str, Any]) -> bool:
        """Verify that there are no unexpected fields."""
        # Use the union of all valid fields across all formats
        all_possible_fields = set()
        for spec in FORMAT_SCHEMAS.values():
            all_possible_fields.update(spec.all_valid_fields)

        for field in failure_dict.keys():
            if field not in all_possible_fields:
                return False
        return True


# Pytest test fixtures and tests
validator = JSONStructureValidator(str(Path(__file__).parent.parent / "tools" / "test_samples"))


def test_format1_sample1_json_parseable():
    """Test that Format 1 Sample 1 produces parseable JSON."""
    content = validator.load_sample('sample1_full_detailed.txt')
    json_data = validator.parse_sample(content)
    json_text = json.dumps(json_data, indent=2)
    assert validator.verify_json_parseable(json_text), "JSON should be parseable"


def test_format1_sample1_required_fields():
    """Test that Format 1 Sample 1 has all required fields."""
    content = validator.load_sample('sample1_full_detailed.txt')
    json_data = validator.parse_sample(content)
    spec = FORMAT_SCHEMAS['tb_short']

    for failure in json_data:
        missing = validator.verify_required_fields(failure, spec)
        assert not missing, f"Missing required fields: {missing}"


def test_format1_sample1_field_types():
    """Test that Format 1 Sample 1 has correct field types."""
    content = validator.load_sample('sample1_full_detailed.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        issues = validator.verify_field_types(failure)
        assert not issues, f"Field type issues: {issues}"


def test_format1_sample1_no_extra_fields():
    """Test that Format 1 Sample 1 has no unexpected fields."""
    content = validator.load_sample('sample1_full_detailed.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        assert validator.verify_no_extra_fields(failure), "Should have no extra fields"


def test_format1_sample2_json_parseable():
    """Test that Format 1 Sample 2 produces parseable JSON."""
    content = validator.load_sample('sample_format1_vv_short.txt')
    json_data = validator.parse_sample(content)
    json_text = json.dumps(json_data, indent=2)
    assert validator.verify_json_parseable(json_text), "JSON should be parseable"


def test_format1_sample2_required_fields():
    """Test that Format 1 Sample 2 has all required fields."""
    content = validator.load_sample('sample_format1_vv_short.txt')
    json_data = validator.parse_sample(content)
    spec = FORMAT_SCHEMAS['tb_short']

    for failure in json_data:
        missing = validator.verify_required_fields(failure, spec)
        assert not missing, f"Missing required fields: {missing}"


def test_format1_sample2_field_types():
    """Test that Format 1 Sample 2 has correct field types."""
    content = validator.load_sample('sample_format1_vv_short.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        issues = validator.verify_field_types(failure)
        assert not issues, f"Field type issues: {issues}"


def test_format1_sample2_no_extra_fields():
    """Test that Format 1 Sample 2 has no unexpected fields."""
    content = validator.load_sample('sample_format1_vv_short.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        assert validator.verify_no_extra_fields(failure), "Should have no extra fields"


def test_format2_sample1_json_parseable():
    """Test that Format 2 Sample 1 produces parseable JSON."""
    content = validator.load_sample('sample2_long_format.txt')
    json_data = validator.parse_sample(content)
    json_text = json.dumps(json_data, indent=2)
    assert validator.verify_json_parseable(json_text), "JSON should be parseable"


def test_format2_sample1_required_fields():
    """Test that Format 2 Sample 1 has all required fields."""
    content = validator.load_sample('sample2_long_format.txt')
    json_data = validator.parse_sample(content)
    spec = FORMAT_SCHEMAS['tb_long']

    for failure in json_data:
        missing = validator.verify_required_fields(failure, spec)
        assert not missing, f"Missing required fields: {missing}"


def test_format2_sample1_field_types():
    """Test that Format 2 Sample 1 has correct field types."""
    content = validator.load_sample('sample2_long_format.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        issues = validator.verify_field_types(failure)
        assert not issues, f"Field type issues: {issues}"


def test_format2_sample1_no_extra_fields():
    """Test that Format 2 Sample 1 has no unexpected fields."""
    content = validator.load_sample('sample2_long_format.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        assert validator.verify_no_extra_fields(failure), "Should have no extra fields"


def test_format2_sample2_json_parseable():
    """Test that Format 2 Sample 2 produces parseable JSON."""
    content = validator.load_sample('sample_format2_v_long.txt')
    json_data = validator.parse_sample(content)
    json_text = json.dumps(json_data, indent=2)
    assert validator.verify_json_parseable(json_text), "JSON should be parseable"


def test_format2_sample2_required_fields():
    """Test that Format 2 Sample 2 has all required fields."""
    content = validator.load_sample('sample_format2_v_long.txt')
    json_data = validator.parse_sample(content)
    spec = FORMAT_SCHEMAS['tb_long']

    for failure in json_data:
        missing = validator.verify_required_fields(failure, spec)
        assert not missing, f"Missing required fields: {missing}"


def test_format2_sample2_field_types():
    """Test that Format 2 Sample 2 has correct field types."""
    content = validator.load_sample('sample_format2_v_long.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        issues = validator.verify_field_types(failure)
        assert not issues, f"Field type issues: {issues}"


def test_format2_sample2_no_extra_fields():
    """Test that Format 2 Sample 2 has no unexpected fields."""
    content = validator.load_sample('sample_format2_v_long.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        assert validator.verify_no_extra_fields(failure), "Should have no extra fields"


def test_format3_sample1_json_parseable():
    """Test that Format 3 Sample 1 produces parseable JSON."""
    content = validator.load_sample('sample3_line_format.txt')
    json_data = validator.parse_sample(content)
    json_text = json.dumps(json_data, indent=2)
    assert validator.verify_json_parseable(json_text), "JSON should be parseable"


def test_format3_sample1_required_fields():
    """Test that Format 3 Sample 1 has all required fields."""
    content = validator.load_sample('sample3_line_format.txt')
    json_data = validator.parse_sample(content)
    spec = FORMAT_SCHEMAS['tb_line']

    for failure in json_data:
        missing = validator.verify_required_fields(failure, spec)
        assert not missing, f"Missing required fields: {missing}"


def test_format3_sample1_field_types():
    """Test that Format 3 Sample 1 has correct field types."""
    content = validator.load_sample('sample3_line_format.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        issues = validator.verify_field_types(failure)
        assert not issues, f"Field type issues: {issues}"


def test_format3_sample1_no_extra_fields():
    """Test that Format 3 Sample 1 has no unexpected fields."""
    content = validator.load_sample('sample3_line_format.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        assert validator.verify_no_extra_fields(failure), "Should have no extra fields"


def test_format3_sample2_json_parseable():
    """Test that Format 3 Sample 2 produces parseable JSON."""
    content = validator.load_sample('sample_format3_tb_line.txt')
    json_data = validator.parse_sample(content)
    json_text = json.dumps(json_data, indent=2)
    assert validator.verify_json_parseable(json_text), "JSON should be parseable"


def test_format3_sample2_required_fields():
    """Test that Format 3 Sample 2 has all required fields."""
    content = validator.load_sample('sample_format3_tb_line.txt')
    json_data = validator.parse_sample(content)
    spec = FORMAT_SCHEMAS['tb_line']

    for failure in json_data:
        missing = validator.verify_required_fields(failure, spec)
        assert not missing, f"Missing required fields: {missing}"


def test_format3_sample2_field_types():
    """Test that Format 3 Sample 2 has correct field types."""
    content = validator.load_sample('sample_format3_tb_line.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        issues = validator.verify_field_types(failure)
        assert not issues, f"Field type issues: {issues}"


def test_format3_sample2_no_extra_fields():
    """Test that Format 3 Sample 2 has no unexpected fields."""
    content = validator.load_sample('sample_format3_tb_line.txt')
    json_data = validator.parse_sample(content)

    for failure in json_data:
        assert validator.verify_no_extra_fields(failure), "Should have no extra fields"


def test_all_samples_produce_valid_json():
    """Test that all 6 sample files produce valid JSON."""
    samples = [
        'sample1_full_detailed.txt',
        'sample_format1_vv_short.txt',
        'sample2_long_format.txt',
        'sample_format2_v_long.txt',
        'sample3_line_format.txt',
        'sample_format3_tb_line.txt'
    ]

    for sample_file in samples:
        content = validator.load_sample(sample_file)
        json_data = validator.parse_sample(content)
        json_text = json.dumps(json_data, indent=2)
        assert validator.verify_json_parseable(json_text), \
            f"{sample_file}: JSON should be parseable"


def test_all_samples_have_required_fields():
    """Test that all 6 sample files have required fields for their format."""
    format_mapping = {
        'sample1_full_detailed.txt': 'tb_short',
        'sample_format1_vv_short.txt': 'tb_short',
        'sample2_long_format.txt': 'tb_long',
        'sample_format2_v_long.txt': 'tb_long',
        'sample3_line_format.txt': 'tb_line',
        'sample_format3_tb_line.txt': 'tb_line'
    }

    for sample_file, format_key in format_mapping.items():
        content = validator.load_sample(sample_file)
        json_data = validator.parse_sample(content)
        spec = FORMAT_SCHEMAS[format_key]

        for failure in json_data:
            missing = validator.verify_required_fields(failure, spec)
            assert not missing, \
                f"{sample_file}: Missing required fields: {missing}"


def test_all_samples_have_correct_field_types():
    """Test that all 6 sample files have correct field types."""
    samples = [
        'sample1_full_detailed.txt',
        'sample_format1_vv_short.txt',
        'sample2_long_format.txt',
        'sample_format2_v_long.txt',
        'sample3_line_format.txt',
        'sample_format3_tb_line.txt'
    ]

    for sample_file in samples:
        content = validator.load_sample(sample_file)
        json_data = validator.parse_sample(content)

        for failure in json_data:
            issues = validator.verify_field_types(failure)
            assert not issues, \
                f"{sample_file}: Field type issues: {issues}"


def test_all_samples_have_no_extra_fields():
    """Test that all 6 sample files have no unexpected fields."""
    samples = [
        'sample1_full_detailed.txt',
        'sample_format1_vv_short.txt',
        'sample2_long_format.txt',
        'sample_format2_v_long.txt',
        'sample3_line_format.txt',
        'sample_format3_tb_line.txt'
    ]

    for sample_file in samples:
        content = validator.load_sample(sample_file)
        json_data = validator.parse_sample(content)

        for failure in json_data:
            assert validator.verify_no_extra_fields(failure), \
                f"{sample_file}: Should have no extra fields"


def test_format_schemas_complete():
    """Test that format schemas are properly defined."""
    # All formats should have required fields
    for format_key, spec in FORMAT_SCHEMAS.items():
        assert spec.required_fields, \
            f"{format_key}: Should have required fields defined"
        assert 'test_file' in spec.required_fields, \
            f"{format_key}: test_file should be required"
        assert 'line_number' in spec.required_fields, \
            f"{format_key}: line_number should be required"
        assert 'error_type' in spec.required_fields, \
            f"{format_key}: error_type should be required"


if __name__ == '__main__':
    # Run tests with unittest if pytest is not available
    import unittest

    # Convert pytest functions to unittest test case
    class JSONValidationTests(unittest.TestCase):
        pass

    # Add all test functions to the test case
    test_functions = [
        test_format1_sample1_json_parseable,
        test_format1_sample1_required_fields,
        test_format1_sample1_field_types,
        test_format1_sample1_no_extra_fields,
        test_format1_sample2_json_parseable,
        test_format1_sample2_required_fields,
        test_format1_sample2_field_types,
        test_format1_sample2_no_extra_fields,
        test_format2_sample1_json_parseable,
        test_format2_sample1_required_fields,
        test_format2_sample1_field_types,
        test_format2_sample1_no_extra_fields,
        test_format2_sample2_json_parseable,
        test_format2_sample2_required_fields,
        test_format2_sample2_field_types,
        test_format2_sample2_no_extra_fields,
        test_format3_sample1_json_parseable,
        test_format3_sample1_required_fields,
        test_format3_sample1_field_types,
        test_format3_sample1_no_extra_fields,
        test_format3_sample2_json_parseable,
        test_format3_sample2_required_fields,
        test_format3_sample2_field_types,
        test_format3_sample2_no_extra_fields,
        test_all_samples_produce_valid_json,
        test_all_samples_have_required_fields,
        test_all_samples_have_correct_field_types,
        test_all_samples_have_no_extra_fields,
        test_format_schemas_complete
    ]

    for func in test_functions:
        def make_test(f):
            def test_func(self):
                f()
            return test_func

        setattr(JSONValidationTests, func.__name__, make_test(func))

    # Run tests
    loader = unittest.TestLoader()
    suite = loader.loadTestsFromTestCase(JSONValidationTests)
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    sys.exit(0 if result.wasSuccessful() else 1)
