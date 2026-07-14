#!/usr/bin/env python3
"""
Verify JSON Structure and Required Fields

This script verifies that the parser outputs valid JSON with all expected
fields present, matching the documented schema.

Bead: bf-2nj8vx
Created: 2026-07-13
"""

import json
import sys
from pathlib import Path
from typing import Dict, List, Any, Set
from dataclasses import dataclass

sys.path.insert(0, str(Path(__file__).parent))
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


class JSONStructureVerifier:
    """Verify JSON output structure and required fields."""

    def __init__(self, samples_dir: str):
        self.samples_dir = Path(samples_dir)
        self.parser = PytestOutputParser()

    def detect_format(self, filename: str) -> str:
        """Detect pytest format from filename."""
        if 'line' in filename.lower() or 'tb_line' in filename.lower():
            return 'tb_line'
        elif 'long' in filename.lower() or 'v_long' in filename.lower():
            return 'tb_long'
        elif 'short' in filename.lower() or 'vv_short' in filename.lower() or 'detailed' in filename.lower():
            return 'tb_short'
        return 'tb_short'  # Default

    def verify_json_parseable(self, json_text: str) -> bool:
        """Verify that JSON text can be parsed by Python json module."""
        try:
            json.loads(json_text)
            return True
        except json.JSONDecodeError as e:
            print(f"  ❌ JSON parsing failed: {e}")
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

    def verify_no_extra_fields(self, failure_dict: Dict[str, Any],
                               format_spec: FormatSpec) -> List[str]:
        """Verify that there are no truly unexpected fields (outside all valid fields)."""
        unexpected = []
        # Use the union of all valid fields across all formats
        # since TestFailure.to_dict() always outputs all fields
        all_possible_fields = set()
        for spec in FORMAT_SCHEMAS.values():
            all_possible_fields.update(spec.all_valid_fields)

        for field in failure_dict.keys():
            if field not in all_possible_fields:
                unexpected.append(f"Unexpected field: {field}")
        return unexpected

    def verify_json_structure(self, failures: List[Dict[str, Any]],
                              format_spec: FormatSpec) -> Dict[str, Any]:
        """Comprehensive JSON structure verification for a format."""
        results = {
            'total_failures': len(failures),
            'valid_json': True,
            'required_fields_ok': True,
            'field_types_ok': True,
            'no_extra_fields': True,
            'issues': []
        }

        for i, failure in enumerate(failures):
            failure_issues = []

            # Check required fields
            missing = self.verify_required_fields(failure, format_spec)
            if missing:
                results['required_fields_ok'] = False
                failure_issues.extend(missing)

            # Check field types
            type_issues = self.verify_field_types(failure)
            if type_issues:
                results['field_types_ok'] = False
                failure_issues.extend(type_issues)

            # Check for extra fields
            extra = self.verify_no_extra_fields(failure, format_spec)
            if extra:
                results['no_extra_fields'] = False
                failure_issues.extend(extra)

            if failure_issues:
                results['issues'].append(f"Failure #{i+1}: " + "; ".join(failure_issues))

        return results

    def verify_all_formats(self) -> Dict[str, Any]:
        """Verify JSON structure for all sample formats."""
        # Group samples by format
        format_samples = {
            'tb_short': ['sample1_full_detailed.txt', 'sample_format1_vv_short.txt'],
            'tb_long': ['sample2_long_format.txt', 'sample_format2_v_long.txt'],
            'tb_line': ['sample3_line_format.txt', 'sample_format3_tb_line.txt']
        }

        all_results = {}

        for format_key, sample_files in format_samples.items():
            format_spec = FORMAT_SCHEMAS[format_key]
            format_results = {
                'format_name': format_spec.name,
                'samples': [],
                'overall': {
                    'valid_json': True,
                    'required_fields_ok': True,
                    'field_types_ok': True,
                    'no_extra_fields': True
                }
            }

            print(f"\n{'='*70}")
            print(f"Testing Format: {format_spec.name}")
            print(f"Description: {format_spec.description}")
            print(f"Required Fields: {format_spec.required_fields}")
            print(f"{'='*70}\n")

            for sample_file in sample_files:
                try:
                    # Read sample
                    content = (self.samples_dir / sample_file).read_text()

                    # Parse with parser
                    failures = self.parser.parse(content)

                    # Convert to JSON-compatible dictionaries
                    json_data = extract_json_compatible_data(failures)

                    # Verify JSON is parseable
                    json_text = json.dumps(json_data, indent=2)
                    if not self.verify_json_parseable(json_text):
                        format_results['overall']['valid_json'] = False
                        format_results['samples'].append({
                            'file': sample_file,
                            'status': 'FAILED',
                            'issues': ['JSON is not parseable']
                        })
                        print(f"❌ {sample_file}: JSON is not parseable\n")
                        continue

                    # Verify structure
                    structure_result = self.verify_json_structure(json_data, format_spec)

                    # Update overall results
                    if not structure_result['required_fields_ok']:
                        format_results['overall']['required_fields_ok'] = False
                    if not structure_result['field_types_ok']:
                        format_results['overall']['field_types_ok'] = False
                    if not structure_result['no_extra_fields']:
                        format_results['overall']['no_extra_fields'] = False

                    # Record results
                    sample_result = {
                        'file': sample_file,
                        'status': 'PASS' if all([
                            structure_result['required_fields_ok'],
                            structure_result['field_types_ok'],
                            structure_result['no_extra_fields']
                        ]) else 'FAILED',
                        'failures_tested': structure_result['total_failures'],
                        'issues': structure_result['issues']
                    }
                    format_results['samples'].append(sample_result)

                    # Print results
                    status_symbol = "✅" if sample_result['status'] == 'PASS' else "❌"
                    print(f"{status_symbol} {sample_file}")
                    print(f"   Failures tested: {structure_result['total_failures']}")
                    print(f"   Required fields: {'✅' if structure_result['required_fields_ok'] else '❌'}")
                    print(f"   Field types: {'✅' if structure_result['field_types_ok'] else '❌'}")
                    print(f"   No extra fields: {'✅' if structure_result['no_extra_fields'] else '❌'}")
                    if structure_result['issues']:
                        for issue in structure_result['issues'][:3]:  # Show first 3 issues
                            print(f"   Issue: {issue}")
                        if len(structure_result['issues']) > 3:
                            print(f"   ... and {len(structure_result['issues']) - 3} more issues")
                    print()

                except Exception as e:
                    format_results['samples'].append({
                        'file': sample_file,
                        'status': 'ERROR',
                        'issues': [str(e)]
                    })
                    print(f"❌ {sample_file}: Error - {e}\n")

            all_results[format_key] = format_results

        return all_results

    def generate_overall_assessment(self, results: Dict[str, Any]) -> Dict[str, Any]:
        """Generate overall assessment across all formats."""
        total_formats = len(results)
        formats_passed = sum(1 for r in results.values()
                            if all(r['overall'].values()))

        # Count total samples and samples passed
        total_samples = 0
        samples_passed = 0
        for format_result in results.values():
            for sample in format_result['samples']:
                total_samples += 1
                if sample['status'] == 'PASS':
                    samples_passed += 1

        return {
            'total_formats': total_formats,
            'formats_passed': formats_passed,
            'total_samples': total_samples,
            'samples_passed': samples_passed,
            'pass_rate': (samples_passed / total_samples * 100) if total_samples > 0 else 0
        }


def main():
    """Main verification entry point."""
    print("="*70)
    print("JSON STRUCTURE AND REQUIRED FIELDS VERIFICATION")
    print("="*70)
    print("\nVerifying parser output matches documented schema...")

    samples_dir = Path(__file__).parent / "test_samples"
    verifier = JSONStructureVerifier(str(samples_dir))

    results = verifier.verify_all_formats()
    assessment = verifier.generate_overall_assessment(results)

    print("\n" + "="*70)
    print("OVERALL ASSESSMENT")
    print("="*70)
    print(f"Total formats tested: {assessment['total_formats']}")
    print(f"Formats passed: {assessment['formats_passed']}")
    print(f"Total samples tested: {assessment['total_samples']}")
    print(f"Samples passed: {assessment['samples_passed']}")
    print(f"Pass rate: {assessment['pass_rate']:.1f}%")
    print()

    # Check acceptance criteria
    criteria_met = True
    print("ACCEPTANCE CRITERIA:")
    print("-" * 70)

    # Check 1: All formats produce valid JSON
    all_valid_json = all(r['overall']['valid_json'] for r in results.values())
    print(f"{'✅' if all_valid_json else '❌'} Parser output is valid JSON (parseable by json module)")
    if not all_valid_json:
        criteria_met = False

    # Check 2: All required fields present
    all_required_fields = all(r['overall']['required_fields_ok'] for r in results.values())
    print(f"{'✅' if all_required_fields else '❌'} All required fields present in JSON output")
    if not all_required_fields:
        criteria_met = False

    # Check 3: JSON structure matches schema
    all_structure_ok = all(r['overall']['field_types_ok'] and r['overall']['no_extra_fields']
                         for r in results.values())
    print(f"{'✅' if all_structure_ok else '❌'} JSON structure matches documented schema")
    if not all_structure_ok:
        criteria_met = False

    # Check 4: All 3 sample formats produce structurally valid JSON
    all_formats_valid = assessment['formats_passed'] == 3
    print(f"{'✅' if all_formats_valid else '❌'} Each of the 3 sample formats produces structurally valid JSON")
    if not all_formats_valid:
        criteria_met = False

    print()
    if criteria_met and assessment['pass_rate'] == 100:
        print("="*70)
        print("✅ VERIFICATION PASSED: All acceptance criteria met")
        print("="*70)
        return 0
    elif assessment['pass_rate'] >= 80:
        print("="*70)
        print("⚠️ VERIFICATION WARNING: Some criteria not fully met")
        print("="*70)
        return 1
    else:
        print("="*70)
        print("❌ VERIFICATION FAILED: Critical issues detected")
        print("="*70)
        return 2


if __name__ == '__main__':
    sys.exit(main())
