# ARMOR Integration Test Inventory

**Bead ID**: bf-51apzs  
**Created**: 2026-07-13  
**Purpose**: Complete inventory of all integration test files in the ARMOR project

## Summary

- **Total Integration Test Files**: 50
- **Languages**: Go (2), Rust (46), Python (2)
- **Test Frameworks**: 
  - Go: `go test` with build tags
  - Rust: Rust `#[test]` attribute
  - Python: `pytest`

---

## Go Integration Tests (2 files)

### Framework: `go test` with build tags
- **Test Command**: `go test -tags=integration ./tests/integration/...`
- **Build Tag**: `//go:build integration`

| File | Lines | Purpose | Test Environment |
|------|-------|---------|------------------|
| `/home/coding/ARMOR/tests/integration/integration_test.go` | ~500 | End-to-end ARMOR server testing against real B2 and Cloudflare | Requires ARMOR server, B2 bucket, Cloudflare domain |
| `/home/coding/ARMOR/tests/integration/awscli_test.go` | ~350 | AWS CLI compatibility testing with ARMOR | Requires ARMOR server, AWS CLI installed |

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

## Python Integration Tests (2 files)

### Framework: `pytest`
- **Test Command**: `pytest <test_file>.py`
- **Location**: Various directories

| File | Lines | Purpose | Module |
|------|-------|---------|--------|
| `/home/coding/ARMOR/tests/test_inventory_reader.py` | ~50 | Test inventory verification and catalog testing | Test infrastructure |
| `/home/coding/ARMOR/tools/parse_module/verify_integration.py` | ~100 | Integration verification for parse module | Parse module testing |

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
# Using pytest
pytest tests/test_inventory_reader.py -v
pytest tools/parse_module/verify_integration.py -v
```

---

## Test File Size Distribution

### Very Large Tests (1000+ lines)
- `type_like_string_false_positive_test.rs` - 14,580 lines
- `invalid_type_conversion_test.rs` - 2,983 lines
- `yaml_indentation_and_mixed_scenarios_test.rs` - 1,539 lines

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

3. **Test Coverage**: The 50 integration test files represent the primary test suite for ARMOR's YAML parsing, error handling, and scope tracking functionality.

4. **Real Integration**: Only the Go tests in `tests/integration/` are true integration tests that require external services (B2, Cloudflare). Most Rust and Python tests are better described as "comprehensive tests" that test complete workflows but don't require external services.

5. **Existing Catalog**: A more detailed catalog of the Rust tests exists at `/home/coding/ARMOR/docs/integration-test-catalog.md` (created for bead bf-4538tw).

---

**End of Integration Test Inventory**
