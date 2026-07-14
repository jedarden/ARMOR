#!/usr/bin/env python3
"""Debug edge case parsing for verification."""

import sys
sys.path.insert(0, '/home/coding/ARMOR')
from tools.parse_pytest_output import PytestOutputParser

# Test floating point - proper format
float_test = '''
_____________________________ test _____________________________
tools/test.py:1: in test
    assert 0.1 + 0.2 == 0.3
E   AssertionError: assert 0.30000000000000004 != 0.3
E     - 0.3
E     + 0.30000000000000004

tools/test.py:1: AssertionError
'''

parser = PytestOutputParser()
failures = parser.parse(float_test)
print('Floating point test:')
print(f'  Failures found: {len(failures)}')
if failures:
    f = failures[0]
    print(f'  Expected: {f.expected}')
    print(f'  Actual: {f.actual}')
    print(f'  Diff lines: {f.diff_lines}')

# Test index diffs - proper format
index_test = '''
_____________________________ test _____________________________
tools/test.py:1: in test
    assert list1 == list2
E   AssertionError: At index 2 diff: 3 != 10
E
E     Full diff:
E       [
E         1,
E         2,
E     -     10,
E     ?     ^^
E     +     3,
E     ?     ^
E         4,
E         5,
E       ]

tools/test.py:1: AssertionError
'''

parser = PytestOutputParser()
failures = parser.parse(index_test)
print('\nIndex diff test:')
print(f'  Failures found: {len(failures)}')
if failures:
    f = failures[0]
    print(f'  Index diff: {f.index_diff}')
    print(f'  Expected: {f.expected}')
    print(f'  Actual: {f.actual}')
    print(f'  Diff lines: {f.diff_lines}')
