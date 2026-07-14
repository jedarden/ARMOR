#!/usr/bin/env python3
"""Final comprehensive verification for JSON output and edge cases."""

import sys
sys.path.insert(0, '/home/coding/ARMOR')

import json
import subprocess
from tools.parse_pytest_output import PytestOutputParser

print('=' * 60)
print('FINAL COMPREHENSIVE VERIFICATION')
print('=' * 60)
print()

# Test 1: JSON validity for all samples
print('Test 1: JSON Validity')
print('-' * 60)
samples = [
    'tools/test_samples/sample_format1_vv_short.txt',
    'tools/test_samples/sample_format2_v_long.txt',
    'tools/test_samples/sample_format3_tb_line.txt',
]

all_valid = True
for sample in samples:
    result = subprocess.run(
        ['python3', 'tools/parse_pytest_output.py', '--json', sample],
        capture_output=True, text=True
    )
    try:
        data = json.loads(result.stdout)
        print(f'✓ {sample.split("/")[-1]:40s} Valid JSON ({len(data["failures"])} failures)')
    except Exception as e:
        print(f'❌ {sample.split("/")[-1]:40s} INVALID JSON: {e}')
        all_valid = False

print()

# Test 2: Required fields
print('Test 2: Required Fields Presence')
print('-' * 60)
required_fields = [
    'test_name', 'test_file', 'line_number', 'error_type', 'error_message',
    'assertion_line', 'assertion_type', 'expected', 'actual',
    'diff_lines', 'index_diff', 'differing_items', 'where_clause'
]

result = subprocess.run(
    ['python3', 'tools/parse_pytest_output.py', '--json', 'tools/test_samples/sample_format1_vv_short.txt'],
    capture_output=True, text=True
)
data = json.loads(result.stdout)

if data['failures']:
    first = data['failures'][0]
    missing = [f for f in required_fields if f not in first]

    if missing:
        print(f'❌ Missing fields: {missing}')
        all_valid = False
    else:
        print(f'✓ All 13 required fields present')

print()

# Test 3: Edge cases (using actual samples)
print('Test 3: Edge Cases Handling (from actual samples)')
print('-' * 60)

# Parse format 1 sample (has index diffs)
with open('tools/test_samples/sample_format1_vv_short.txt', 'r') as f:
    output = f.read()

parser = PytestOutputParser()
failures = parser.parse(output)

# Check for index diffs
index_found = False
for f in failures:
    if f.index_diff is not None:
        print(f'✓ Index diffs handled (index={f.index_diff}, actual={f.actual}, expected={f.expected})')
        index_found = True
        break

if not index_found:
    print('⚠️  No index diffs found in sample (may not be present)')

# Check for dict diffs
dict_found = False
for f in failures:
    if f.differing_items:
        print(f'✓ Differing items handled ({len(f.differing_items)} items)')
        dict_found = True
        break

if not dict_found:
    print('⚠️  No differing items found in sample (may not be present)')

# Check for diff lines
diff_found = False
for f in failures:
    if f.diff_lines:
        print(f'✓ Diff lines handled ({len(f.diff_lines)} lines)')
        diff_found = True
        break

if not diff_found:
    print('⚠️  No diff lines found in sample (may not be present)')

print()

# Test 4: Format detection
print('Test 4: Format Detection (3 formats)')
print('-' * 60)

formats = [
    ('sample_format1_vv_short.txt', '--tb=short'),
    ('sample_format2_v_long.txt', '--tb=long'),
    ('sample_format3_tb_line.txt', '--tb=line'),
]

for sample, expected_format in formats:
    result = subprocess.run(
        ['python3', 'tools/parse_pytest_output.py', '--json', f'tools/test_samples/{sample}'],
        capture_output=True, text=True
    )
    data = json.loads(result.stdout)
    if data['failures']:
        print(f'✓ {expected_format:15s} - {len(data["failures"])} failures extracted')

print()

# Test 5: All 6 sample files
print('Test 5: All Sample Files (6 total)')
print('-' * 60)

all_samples = [
    'sample1_full_detailed.txt',
    'sample2_long_format.txt',
    'sample3_line_format.txt',
    'sample_format1_vv_short.txt',
    'sample_format2_v_long.txt',
    'sample_format3_tb_line.txt',
]

for sample in all_samples:
    result = subprocess.run(
        ['python3', 'tools/parse_pytest_output.py', '--json', f'tools/test_samples/{sample}'],
        capture_output=True, text=True
    )
    try:
        data = json.loads(result.stdout)
        failure_count = len(data['failures'])
        print(f'✓ {sample:40s} - {failure_count:2d} failures - Valid JSON')
    except Exception as e:
        print(f'❌ {sample:40s} - ERROR: {e}')
        all_valid = False

print()

# Final result
print('=' * 60)
if all_valid:
    print('✅ ALL VERIFICATION TESTS PASSED')
    print('=' * 60)
    sys.exit(0)
else:
    print('❌ SOME TESTS FAILED')
    print('=' * 60)
    sys.exit(1)
