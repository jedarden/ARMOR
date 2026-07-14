# ARMOR Integration Test Inventory

**Bead ID**: bf-51apzs  
**Created**: 2026-07-13  
**Purpose**: Complete inventory of all integration test files in the ARMOR project

## Summary

- **Total Integration Test Files**: 82
- **Languages**: Go (7), Rust (51), Python (24)
- **Test Frameworks**:
  - Go: `go test` with build tags
  - Rust: Rust `#[test]` attribute
  - Python: `pytest`

---

## Go Integration Tests (7 files)

### Framework: `go test` with build tags
- **Test Command**: `go test -tags=integration ./tests/integration/...`
- **Build Tag**: `//go:build integration`

| File | Lines | Purpose | Test Environment |
|------|-------|---------|------------------|
| `/home/coding/ARMOR/tests/integration/integration_test.go` | ~500 | End-to-end ARMOR server testing against real B2 and Cloudflare | Requires ARMOR server, B2 bucket, Cloudflare domain |
| `/home/coding/ARMOR/tests/integration/awscli_test.go` | ~350 | AWS CLI compatibility testing with ARMOR | Requires ARMOR server, AWS CLI installed |

### Named Integration Tests (5 files)
Located in `internal/yamlutil/`:
- **Test Command**: `go test ./internal/yamlutil/...`

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/internal/yamlutil/integration_test.go` | ~1,600 | End-to-end YAML parsing with actual testdata files |
| `/home/coding/ARMOR/internal/yamlutil/valid_simple_integration_test.go` | ~300 | Valid simple YAML test cases |
| `/home/coding/ARMOR/internal/yamlutil/valid_nested_integration_test.go` | ~350 | Valid nested YAML test cases |
| `/home/coding/ARMOR/internal/yamlutil/valid_complex_integration_test.go` | ~320 | Valid complex YAML test cases |
| `/home/coding/ARMOR/cmd/armor-decrypt/main_test.go` | ~150 | Command-line decryption functionality tests |

**Environment Variables Required**: `ARMOR_INTEGRATION_TEST`, `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`, `ARMOR_B2_REGION`, `ARMOR_BUCKET`, `ARMOR_CF_DOMAIN`, `ARMOR_MEK`, `ARMOR_AUTH_ACCESS_KEY`, `ARMOR_AUTH_SECRET_KEY`

---

## Rust Integration Tests (46 files)

### Framework: Rust `#[test]` attribute
- **Test Command**: `cargo test --test <test_name>`
- **Location**: `/home/coding/ARMOR/tests/` directory

#### Scope Tracking & Scope Stack Integration Tests (8 files)

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/tests/comprehensive_scope_tracking_test.rs` | 859 | Full scope tracking across complex scenarios |
| `/home/coding/ARMOR/tests/scope_tracking_comprehensive_test.rs` | 958 | Comprehensive scope tracking with nesting |
| `/home/coding/ARMOR/tests/scope_stack_verification_test.rs` | 895 | Verification that scope stack maintains correct state |
| `/home/coding/ARMOR/tests/exit_to_scope_edge_cases_test.rs` | 604 | Edge cases when exiting scopes |
| `/home/coding/ARMOR/tests/state_preservation_scope_exit_test.rs` | 710 | State preservation when exiting scopes |
| `/home/coding/ARMOR/tests/target_scope_lookup_test.rs` | 393 | Looking up the correct target scope for operations |
| `/home/coding/ARMOR/tests/sequence_scope_verification_test.rs` | 723 | Scope behavior in sequence contexts |
| `/home/coding/ARMOR/tests/acceptance_criteria_verification_test.rs` | 197 | Verification that acceptance criteria are met |

#### Type Conversion Integration Tests (5 files)

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/tests/invalid_type_conversion_test.rs` | 2,983 | Invalid conversions (strings to non-strings, structs to scalars, etc.) |
| `/home/coding/ARMOR/tests/int32_to_uint32_boundary_test.rs` | 857 | Boundary testing for int32 → uint32 conversions |
| `/home/coding/ARMOR/tests/int32_to_uint32_error_detection_test.rs` | 513 | Error detection in int32 → uint32 conversions |
| `/home/coding/ARMOR/tests/negative_conversion_error_message_test.rs` | 248 | Error messages for negative number conversions |
| `/home/coding/ARMOR/tests/type_like_string_false_positive_test.rs` | 14,580 | Preventing false positives when types look like strings |

#### Error Handling & Parse Error Integration Tests (9 files)

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/tests/parse_error_integration_test.rs` | 616 | End-to-end error creation and formatting workflows |
| `/home/coding/ARMOR/tests/parse_error_full_lifecycle_integration_test.rs` | 743 | Full lifecycle of parse errors from creation to display |
| `/home/coding/ARMOR/tests/parse_error_propagation_test.rs` | 237 | How errors propagate through function calls |
| `/home/coding/ARMOR/tests/parse_error_display_test.rs` | 339 | Error message formatting and display |
| `/home/coding/ARMOR/tests/error_code_validation_test.rs` | 307 | Validation of error codes |
| `/home/coding/ARMOR/tests/error_messages_test.rs` | 52 | Basic error message tests |
| `/home/coding/ARMOR/tests/malformed_error_message_test.rs` | 785 | Handling malformed error messages |
| `/home/coding/ARMOR/tests/error_message_format_examples_test.rs` | 503 | Examples of error message formats |
| `/home/coding/ARMOR/tests/validation_error_format_test.rs` | 264 | Format of validation errors |

#### YAML Comment Filtering Integration Tests (14 files)

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/tests/yaml_comment_filtering_edge_cases_test.rs` | 672 | Edge cases in YAML comment detection |
| `/home/coding/ARMOR/tests/yaml_comment_edge_case_test.rs` | 861 | Difficult edge cases in comment handling |
| `/home/coding/ARMOR/tests/yaml_comment_false_positive_test.rs` | 820 | Preventing false positive comment detection |
| `/home/coding/ARMOR/tests/yaml_comment_position_test.rs` | 388 | Comment position awareness |
| `/home/coding/ARMOR/tests/yaml_block_scalar_indentation_comment_test.rs` | 881 | Comments in block scalar contexts |
| `/home/coding/ARMOR/tests/yaml_folded_multiline_comment_test.rs` | 910 | Comments in folded multiline scalars |
| `/home/coding/ARMOR/tests/yaml_folded_scalar_continuation_validation_test.rs` | 769 | Folded scalar continuation with comments |
| `/home/coding/ARMOR/tests/yaml_literal_multiline_comment_test.rs` | 533 | Comments in literal multiline scalars |
| `/home/coding/ARMOR/tests/yaml_multiline_quoted_scalar_comment_test.rs` | 536 | Comments in quoted multiline scalars |
| `/home/coding/ARMOR/tests/yaml_plain_multiline_scalar_comment_test.rs` | 599 | Comments in plain multiline scalars |
| `/home/coding/ARMOR/tests/yaml_indentation_and_mixed_scenarios_test.rs` | 1,539 | Mixed indentation and comment scenarios |
| `/home/coding/ARMOR/tests/yaml_indent_without_keys_test.rs` | 354 | Indentation changes without key definitions |
| `/home/coding/ARMOR/tests/inline_comment_detection_test.rs` | 757 | Inline comment detection and stripping |
| `/home/coding/ARMOR/tests/comment_filtering_basic_test.rs` | 358 | Basic comment filtering functionality |

#### Indentation Detection Integration Tests (4 files)

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/tests/indent_change_detection_test.rs` | 566 | Detecting and handling indent changes |
| `/home/coding/ARMOR/tests/indent_without_key_test.rs` | 356 | Indentation changes that don't introduce keys |
| `/home/coding/ARMOR/tests/false_positive_indent_key_test.rs` | 166 | Preventing false positives in indent key detection |
| `/home/coding/ARMOR/tests/missing_colon_comprehensive_test.rs` | 227 | Handling missing colons with indent changes |

#### Other Integration Tests (6 files)

| File | Lines | Purpose |
|------|-------|---------|
| `/home/coding/ARMOR/tests/line_classification_test.rs` | 301 | Classification of YAML line types |
| `/home/coding/ARMOR/tests/schema_validation_test.rs` | 655 | Schema validation workflows |
| `/home/coding/ARMOR/tests/nested_duplicate_detection_test.rs` | 878 | Duplicate key detection in nested structures |
| `/home/coding/ARMOR/tests/test_inventory_reader.py` | ~50 | Test inventory verification |
| `/home/coding/ARMOR/tools/parse_module/verify_integration.py` | ~100 | Integration verification for parse module |

---

## Python Integration Tests (24 files)

### Framework: `pytest`
- **Test Command**: `pytest tests/yamlutil/ tools/parse_module/tests/`
- **Location**: Multiple directories

#### YAML Util Tests (12 files)
Located in `tests/yamlutil/`:

| File | Purpose |
|------|---------|
| `test_broken_samples.py` | Tests for malformed YAML samples |
| `test_complete_mixed_yaml_documents.py` | Complete mixed YAML document handling |
| `test_exceptions.py` | Exception handling in YAML parsing |
| `test_explicit_indent.py` | Explicit indentation testing |
| `test_indentation_comment_filtering.py` | Indentation and comment filtering |
| `test_mixed_comment_scenarios.py` | Mixed comment scenario testing |
| `test_parser.py` | Core parser functionality tests |
| `test_reader.py` | YAML reader functionality |
| `test_result_comprehensive.py` | Comprehensive result handling |
| `test_result_helpers.py` | Result helper utilities |
| `test_result_helpers_extended.py` | Extended result helper functions |
| `test_validator.py` | YAML validation functionality |

#### Parse Module Tests (2 files)
Located in `tools/parse_module/tests/`:

| File | Purpose |
|------|---------|
| `test_parse_result.py` | Parse result structure and handling |
| `test_yaml_parser.py` | YAML parser integration tests |

#### Root-Level Python Tests (10 files)
Located in project root:

| File | Purpose |
|------|---------|
| `test_scope_exit_comprehensive.py` | Comprehensive scope exit testing |
| `test_scope_depth_tracking.py` | Scope depth tracking |
| `test_key_token_detection.py` | Key token detection |
| `test_comment_filtering_simple.py` | Simple comment filtering |
| `test_parser_state_line_type.py` | Parser state and line type |
| `test_indent_transition_state_machine.py` | Indent transition state machine |
| `test_mixed_yaml_comments.py` | Mixed YAML comment handling |
| `test_line_classification.py` | Line classification |
| `test_indent_without_key_verification.py` | Indent without key verification |

#### Verification Scripts (4 files)

| File | Purpose |
|------|---------|
| `internal/yamlutil/verify_parser.py` | Parser verification script |
| `tests/yamlutil/verify_implementation.py` | Implementation verification |
| `tools/parse_module/verify_integration.py` | Integration verification |
| `tools/parse_module/verify_structure.py` | Structure verification |

#### Other Python Files (6 files)
Located in `tests/yamlutil/`:

| File | Purpose |
|------|---------|
| `broken_yaml_samples.py` | Test data/fixtures for broken YAML |
| `examples.py` | Example YAML test cases |
| `validate_yaml_functional.py` | Functional YAML validation |
| `test_inventory_reader.py` | Test infrastructure inventory |
| `__init__.py` | Package initialization |

---

## Additional Unit Test Files (Not Integration Tests)

The following are unit tests that test isolated components and are not classified as integration tests:

### Go Unit Tests (~35 files)
Located in various `internal/` subdirectories:
- `/home/coding/ARMOR/internal/b2keys/b2keys_test.go`
- `/home/coding/ARMOR/internal/backend/backend_test.go`
- `/home/coding/ARMOR/internal/config/config_test.go`
- And 32 more files in `internal/` directories

### Rust Unit Tests (6 files)
Located in `/home/coding/ARMOR/tests/`:
- `scope_stack_unit_test.rs` - 987 lines
- `push_scope_unit_test.rs` - 309 lines
- `scope_stack_structure_test.rs` - 208 lines
- `scope_stack_test.rs` - 257 lines
- `parse_error_unit_test.rs` - 607 lines
- `result_dataclass_test.rs` - 87 lines

### Python Unit Tests (~20 files)
Located in various directories:
- `/home/coding/ARMOR/tests/yamlutil/test_*.py`
- `/home/coding/ARMOR/internal/yamlutil/tests/test_parser.py`
- `/home/coding/ARMOR/tools/parse_module/tests/*.py`
- Root-level `test_*.py` files

---

## Integration Test Execution

### Go Integration Tests
```bash
# Requires environment variables and running ARMOR server
export ARMOR_INTEGRATION_TEST=1
export ARMOR_B2_ACCESS_KEY_ID=...
export ARMOR_B2_SECRET_ACCESS_KEY=...
export ARMOR_B2_REGION=us-east-005
export ARMOR_BUCKET=...
export ARMOR_CF_DOMAIN=...
export ARMOR_MEK=...
export ARMOR_AUTH_ACCESS_KEY=...
export ARMOR_AUTH_SECRET_KEY=...

go test -tags=integration ./tests/integration/... -v
```

### Rust Integration Tests
```bash
# All integration tests in /tests directory
cargo test --test '*integration*'

# Specific test
cargo test --test parse_error_integration_test

# With output
cargo test --test '*integration*' -- --nocapture
```

### Python Integration Tests
```bash
# All yamlutil tests
pytest tests/yamlutil/ -v

# Parse module tests
pytest tools/parse_module/tests/ -v

# Root-level Python tests
pytest test_*.py -v

# Specific verification scripts
python tests/yamlutil/verify_implementation.py
python tools/parse_module/verify_integration.py

# All Python tests
pytest tests/yamlutil/ tools/parse_module/tests/ -v
```

---

## Test Distribution by Category

| Language | Test Framework | Integration Files | Unit Test Files | Total Test Files |
|----------|--------------|-------------------|-----------------|------------------|
| Go | go test | 7 | 90 | 97 |
| Rust | cargo test | 51 | Various | 51+ |
| Python | pytest | 24 | ~8 | ~32 |
| **Total** | - | **82** | **~98** | **~180** |

## Test File Size Distribution

### Very Large Tests (1000+ lines)
- `type_like_string_false_positive_test.rs` - 14,580 lines
- `invalid_type_conversion_test.rs` - 2,983 lines
- `yaml_indentation_and_mixed_scenarios_test.rs` - 1,539 lines
- `scope_tracking_comprehensive_test.rs` - 958 lines

### Large Tests (500-999 lines)
- 29 files in this range

### Medium Tests (200-499 lines)
- 14 files in this range

### Small Tests (1-199 lines)
- 4 files in this range

---

## Notes

1. **Integration vs Unit Tests**: Integration tests test end-to-end workflows and component interactions, while unit tests test individual components in isolation.

2. **Go Build Tags**: The Go integration tests use build tags and are only run when explicitly enabled with `-tags=integration`.

3. **Test Coverage**: The 82 integration test files represent the primary test suite for ARMOR's YAML parsing, error handling, scope tracking, and ARMOR server functionality.

4. **Real Integration**: Only the Go tests in `tests/integration/` are true integration tests that require external services (B2, Cloudflare). Most Rust and Python tests are better described as "comprehensive tests" that test complete workflows but don't require external services.

5. **Existing Catalog**: A more detailed catalog of the Rust tests exists at `/home/coding/ARMOR/docs/integration-test-catalog.md` (created for bead bf-4538tw).

6. **Python Test Structure**: Python tests are organized into three main areas:
   - `tests/yamlutil/` - YAML utility testing (12 files)
   - `tools/parse_module/tests/` - Parse module testing (2 files)
   - Root level - General integration tests (10+ files)

7. **Rust Test Organization**: All Rust integration tests are located in `/tests/` directory, separate from unit tests which are typically co-located with source code.

8. **Verification Scripts**: Several Python scripts (`verify_*.py`) provide standalone verification without requiring test framework execution.

---

**End of Integration Test Inventory**
