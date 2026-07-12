# ValidationError Instantiations - Complete Catalog

**Task ID:** bf-5gcia  
**Date:** 2026-07-12  
**Status:** COMPLETE ✓

## Summary

All ValidationError instantiations in the ARMOR codebase have been documented. **Total: 0 production code instantiations, 27 test instantiations**. Every instantiation includes the `path` field.

---

## ValidationError Structure

**File:** `src/parsers/yaml/types.rs:554`

```rust
pub struct ValidationError {
    /// Path to the invalid element (e.g., "server.port")
    pub path: String,
    /// Error message
    pub message: String,
    /// Line number where the error occurred (1-indexed)
    pub line: Option<usize>,
}
```

**Constructor:** `ValidationError::new(path, message) -> Self` (line 565)

---

## Production Code Instantiations

**Count: 0**

There are **no** ValidationError instantiations in production code (`src/`). The struct is only defined and exported from `src/parsers/yaml/types.rs` and `src/parsers/yaml/mod.rs`.

---

## Test Instantiations

### By File

#### 1. `tests/error_message_format_examples_test.rs` (7 instantiations)

| Line | Type | Path Field | Context |
|------|------|------------|---------|
| 288 | `ValidationError { }` | ✓ `"server.port"` | Port range validation example |
| 308 | `ValidationError { }` | ✓ `"name"` | Required field validation example |
| 322 | `ValidationError { }` | ✓ `"database.host"` | Nested field validation example |
| 336 | `ValidationError { }` | ✓ `"servers[0].config.port"` | Array nested validation example |
| 350 | `ValidationError { }` | ✓ `"database.name"` | Validation without line number |
| 368 | `ValidationError { }` | ✓ `"server.port"` | Multiple errors test (first) |
| 373 | `ValidationError { }` | ✓ `"server.host"` | Multiple errors test (second) |

**Status:** All have path field ✓

---

#### 2. `tests/acceptance_criteria_verification_test.rs` (4 instantiations)

| Line | Type | Path Field | Context |
|------|------|------------|---------|
| 48 | `ValidationError::new()` | ✓ `"spec.replicas"` | K8s field validation with line |
| 64 | `ValidationError::new()` | ✓ `"server.host"` | Host validation without line |
| 116 | `ValidationError::new()` | ✓ `"field.name"` | Generic field validation |
| 167 | `ValidationError::new()` | ✓ `field_path` (dynamic) | Parameterized test |

**Status:** All have path field ✓

---

#### 3. `tests/validation_error_format_test.rs` (16 instantiations)

| Line | Type | Path Field | Context |
|------|------|------------|---------|
| 25 | `ValidationError::new()` | ✓ `"server.port"` | Basic display format test |
| 42 | `ValidationError::new()` | ✓ `"server.port"` | Display with line number |
| 68 | `ValidationError::new()` | ✓ `field_path` (dynamic) | Parameterized path test |
| 92 | `ValidationError::new()` | ✓ `"spec.template.spec.containers[0].image"` | K8s deep path test |
| 115 | `ValidationError::new()` | ✓ `"database.port"` | Type mismatch error |
| 130 | `ValidationError::new()` | ✓ `"server.timeout"` | Positive value constraint |
| 147 | `ValidationError::new()` | ✓ `"service.name"` | Required field test |
| 169 | `ValidationError::new()` | ✓ `"field1"` | Multiple errors array (1) |
| 170 | `ValidationError::new()` | ✓ `"field2"` | Multiple errors array (2) |
| 171 | `ValidationError::new()` | ✓ `"field3"` | Multiple errors array (3) |
| 192 | `ValidationError::new()` | ✓ `"test.field"` | Field access test |
| 217 | `ValidationError::new()` | ✓ `"port"` | Structured error test (1) |
| 218 | `ValidationError::new()` | ✓ `"timeout"` | Structured error test (2) |
| 219 | `ValidationError::new()` | ✓ `"email"` | Structured error test (3) |
| 220 | `ValidationError::new()` | ✓ `"url"` | Structured error test (4) |
| 255 | `ValidationError::new()` | ✓ `"services[0].port"` | Array service path test |

**Status:** All have path field ✓

---

## Path Field Status Summary

**Total ValidationError instantiations: 27**
- ✅ **With Path field: 27 (100%)**
- ❌ **Missing Path field: 0**

### By Construction Method

| Method | Count | Path Status |
|--------|-------|-------------|
| `ValidationError { path, message, line }` | 7 | 100% ✓ |
| `ValidationError::new(path, message)` | 20 | 100% ✓ |

---

## Path Value Examples

The following path patterns are used across tests:

- **Simple fields:** `"name"`, `"port"`, `"timeout"`, `"email"`, `"url"`, `"service.name"`
- **Nested fields:** `"server.port"`, `"database.host"`, `"database.name"`, `"database.port"`
- **Array-indexed:** `"servers[0].config.port"`, `"services[0].port"`
- **K8s-style:** `"spec.replicas"`, `"spec.template.spec.containers[0].image"`
- **Dynamic:** `field_path` (test variable)

---

## Key Findings

1. **No production code usage** - ValidationError is only instantiated in tests
2. **100% Path field coverage** - All test instantiations include the path field
3. **Consistent API usage** - Tests use both struct literal and constructor pattern correctly
4. **Rich path variety** - Tests cover simple, nested, array-indexed, and K8s-style paths

---

## Notes for Next Bead

- All ValidationError instantiations already have the Path field
- No fixes needed for missing Path fields
- Struct definition at `src/parsers/yaml/types.rs:554` is the production implementation
- Constructor `ValidationError::new()` at line 565 properly initializes path field
- Consider whether production validation code needs to be added (currently no ValidationError creation in src/)
