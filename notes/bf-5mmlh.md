# Bead bf-5mmlh: Tab vs Space Indentation Handling

## Status: Already Implemented

All acceptance criteria for this bead were already met in the existing codebase.

## Acceptance Criteria Verification

### ✅ 1. Document how tabs are handled (expanded or single character)
**Location:** `internal/yamlutil/line_parser.go` lines 7-28

The package-level documentation clearly states:
- Tabs are expanded to 8-space boundaries for indentation counting
- This follows YAML specification guidance
- Mixed indentation is detected and flagged via `HasMixedIndent`

### ✅ 2. Handle mixed tabs and spaces consistently
**Location:** `internal/yamlutil/line_parser.go` lines 233-289

The `calculateIndentation()` function implements tab expansion:
- Uses 8-space tab stop boundaries: `count = ((count + 8) / 8) * 8`
- Handles mixed indentation correctly
- Examples in docstring demonstrate behavior

### ✅ 3. Add documentation comment explaining the indentation strategy
**Location:** `internal/yamlutil/line_parser.go` lines 233-274

Comprehensive documentation includes:
- Tab expansion strategy explanation
- Rationale for choosing expansion over single-character counting
- Examples of how different patterns are calculated
- References to YAML specification

### ✅ 4. Choose either: expand tabs to spaces or count as single character
**Decision Made:** Expand tabs to 8-space boundaries

**Rationale documented in code:**
1. Matches traditional text editor behavior (tab stops every 8 columns)
2. Provides consistent alignment when mixed indentation occurs
3. YAML spec recommends spaces, so treating tabs as space-equivalents is safer
4. Most YAML generators use spaces (common case optimization)

## Test Coverage

All indentation tests pass:
- `TestCalculateIndentationSimple` - Basic indentation counting
- `TestCalculateIndentationTabsAsSingleCharacter` - Tab expansion verification
- `TestCalculateIndentationMultipleTabs` - Multiple tab handling
- `TestCalculateIndentationMixedTabSpace` - Mixed indentation patterns
- `TestIndentationTypeDetection` - Space/tab/mixed detection

## Implementation Summary

The code already implements a robust, YAML-spec-compliant indentation strategy:
- Tabs are expanded to 8-space boundaries for consistent handling
- Mixed indentation is detected and flagged
- The strategy is well-documented at package and function levels
- Comprehensive test coverage verifies the behavior

No code changes were required - the implementation was already complete.
