# Pytest Output Samples

This directory contains a comprehensive collection of pytest failure outputs demonstrating various failure scenarios. These samples are useful for:

- Understanding different pytest output formats
- Testing parser implementations
- Learning how pytest reports different types of failures
- Building tools that consume pytest output

## Sample Output Files

### 1. Simple Assertion Failures (`output_simple_assertion.txt`)
Demonstrates basic assertion failures with clear, concise output.

**Failure types covered:**
- Basic equality assertion (`assert x == y`)
- Boolean assertion (`assert result`)
- None check (`assert value is not None`)
- Type checking (`assert isinstance(value, int)`)

**Key characteristics:**
- Shows the failing line with `>` marker
- Displays the assertion expression that failed
- Shows both expected and actual values
- Includes custom assertion messages

### 2. Comparison Failures (`output_comparison_failures.txt`)
Shows failures when comparing complex data structures.

**Failure types covered:**
- List comparison with index highlighting
- Dictionary comparison with key-value diffs
- String comparison with character-level diffs
- Nested structure comparisons (dicts with lists)

**Key characteristics:**
- Highlights the specific index/element that differs
- Shows "At index X diff:" for lists
- Shows "Differing items:" for dicts
- Character-level string diffs with `?` markers
- Truncates long diffs (use `-vv` for full output)

### 3. Multiline Diffs (`output_multiline_diffs.py` → captured in various outputs)
Demonstrates how pytest displays differences in multi-line text.

**Failure types covered:**
- Multi-line string comparisons
- Code/text diffs with context lines
- Long list diffs with truncation
- JSON-like structure diffs

**Key characteristics:**
- Shows full context with `+` for additions, `-` for deletions
- Uses `?` lines to mark exact character differences
- Truncates long outputs and shows line counts

### 4. Exception Failures (`output_exception_failures.txt`)
Shows various exception and error scenarios.

**Failure types covered:**
- Wrong exception type raised
- Expected exception not raised (`DID NOT RAISE`)
- Unexpected exceptions (e.g., `ZeroDivisionError`)
- Exception message regex mismatch
- Setup/teardown failures

**Key characteristics:**
- Shows exception type and message
- Stack trace pointing to failure location
- For `pytest.raises()`: shows expected vs actual exception
- "DID NOT RAISE" message when no exception occurs

### 5. Parametrize & Fixture Failures (`output_parametrize_fixtures.txt`)
Demonstrates failures with pytest fixtures and parameterized tests.

**Failure types covered:**
- Parameterized test failures with specific parameter values
- Fixture setup errors (ERROR at setup)
- Fixture teardown errors (ERROR at teardown)
- Mixed pass/fail in parameterized tests

**Key characteristics:**
- Shows parameter values in test name: `test_name[param1-param2]`
- Distinguishes between FAILURES and ERRORS
- Setup errors prevent test from running
- Teardown errors occur after test completes

### 6. Edge Cases (`output_edge_cases.txt`)
Covers various assertion patterns and Python-specific failures.

**Failure types covered:**
- Floating-point precision issues
- Membership testing (`in` operator)
- KeyError on missing dict keys
- AttributeError on missing attributes
- Comparison operators (`>=`, `>`, etc.)
- Custom detailed assertion messages
- Empty container checks
- Regex pattern matching failures

**Key characteristics:**
- Shows Python's float representation (e.g., `0.30000000000000004`)
- Displays full expressions (e.g., `where 0 = len([])`)
- Shows function call chains (e.g., `where None = <function match>...`)

### 7. Verbose Output (`output_verbose_dict.txt`)
Demonstrates `-vv` (extra verbose) output format.

**Key characteristics:**
- Shows full diffs without truncation
- Separates "Common items" from "Differing items"
- More detailed structure representation
- Exact location of differences marked with `^`

### 8. All Tests Summary (`output_all_tests_short.txt`)
Comprehensive output showing all test runs with `--tb=short`.

**Key characteristics:**
- Progress percentage for each test
- Short traceback format (one line per failure)
- Summary statistics: `X failed, Y passed, Z errors`
- Complete overview of all failure types in one run

## Test Files

### `test_simple_assertion.py`
Basic assertion patterns for fundamental failure demonstrations.

### `test_comparison_failures.py`
Complex data structure comparisons (lists, dicts, nested objects).

### `test_multiline_diffs.py`
Multi-line text and code comparison scenarios.

### `test_exception_failures.py`
Exception handling and error condition scenarios.

### `test_parametrize_fixtures.py`
Pytest fixtures and parameterized test scenarios.

### `test_edge_cases.py`
Special cases and Python-specific assertion patterns.

### `conftest.py`
Shared fixtures used across multiple tests.

## Running the Samples

To regenerate these outputs, you'll need pytest installed. The samples were created using:

```bash
# Using nix-shell (if available)
nix-shell -p python312Packages.pytest --run 'python -m pytest <test_file> -v'

# Or with standard pytest
pytest <test_file> -v

# For extra verbose diffs
pytest <test_file> -vv

# For short traceback format
pytest --tb=short
```

## Output Format Notes

### Traceback Formats
- `--tb=long`: Full tracebacks (default)
- `--tb=short`: One line per failure (compact)
- `--tb=line`: Even more compact
- `--tb=no`: Suppress tracebacks entirely

### Verbosity Levels
- Default: Basic test names and progress
- `-v`: Verbose (individual test names)
- `-vv`: Extra verbose (full diffs, no truncation)
- `-vvv`: Very verbose (includes test execution details)

### Exit Codes
- `0`: All tests passed
- `1`: Tests failed
- `2`: Test execution interrupted (user interrupt)
- `3`: Internal error
- `4`: pytest command-line usage error
- `5`: No tests collected

## Usage in Parser Development

When developing a pytest output parser:

1. Start with `output_simple_assertion.txt` for basic patterns
2. Handle `output_comparison_failures.txt` for diff parsing
3. Add support for exceptions from `output_exception_failures.txt`
4. Parse parametrize/fixtures from `output_parametrize_fixtures.txt`
5. Handle edge cases from `output_edge_cases.txt`
6. Support both short and long traceback formats
7. Handle various verbosity levels

## Common Patterns to Recognize

1. **Test header**: `test_file.py::test_name[param values]`
2. **Failure marker**: Lines starting with `>`
3. **Assertion lines**: `E       AssertionError: ...`
4. **Diff markers**: `E         - expected` and `E         + actual`
5. **Position markers**: `E         ?       ^^^`
6. **Error location**: `file.py:line: ErrorType`
7. **Summary**: `==== X failed, Y passed, Z errors ====`

## License

These samples are provided as-is for educational and testing purposes.
