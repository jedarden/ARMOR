// Package crypto provides decompression correctness verification helpers.
//
// **VerificationError Mapping Documentation:**
// The VerificationError type in this package implements the error message format
// specification defined in the following documentation:
//
// 1. docs/error-message-structure.md
//    - Core error message format with required fields (offset, expected, actual)
//    - Optional context fields for detailed diagnostics
//    - JSON serialization structure
//
// 2. docs/verification-error-to-message-mapping.md
//    - Field-by-field mapping from VerificationError struct to message format
//    - Transformation logic (byte arrays → hex strings or descriptions)
//    - Special offset value semantics (-1, -2, < -2)
//    - Integration with VerifyResult wrapper type
//
// 3. docs/verification-error-message-examples.md
//    - Concrete error message examples for common failure scenarios
//    - Human-readable format vs. JSON format
//    - Go code to generate each error type
//
// **Mapping Summary:**
// VerificationError fields → Error message format → Human-readable output
//   Offset               → "offset" field           → "byte mismatch at offset X"
//   Expected[]           → "expected" field        → "expected 0x03" or "expected N-byte context"
//   Actual[]             → "actual" field          → "got 0x00" or "got N-byte context"
//   ContextBytes         → Optional context        → "[C bytes context: B before, A after]"
//   ContextBefore/After  → Optional context        → Included in context description
//   ExpectedLength       → Optional context        → "(got X bytes, expected Y bytes)"
//   ActualLength         → Optional context        → "(got X bytes, expected Y bytes)"
//
// **Key Implementation Points:**
// - Error() method: Implements human-readable message construction
// - MismatchSeverity(): Classifies error severity (critical/high/medium/low)
// - IsLengthMismatch()/IsByteMismatch()/IsOutOfRange(): Type check helpers
//
// See individual type and method documentation for detailed mapping examples.
package crypto

// Verification Function Signatures - Quick Reference
//
// This package provides decompression verification functions for ARMOR's restore
// verification pipeline (ADR-004). All functions return structured results that
// include pass/fail status, diagnostic information, and precise error location.
//
// **Core Verification Functions:**
//
// 1. Full Object Verification:
//    func VerifyDecompression(decompressed, expected []byte) *VerificationResult
//    - Verifies byte-for-byte correctness of full decompressed objects
//    - Used in restore verifier dual-path agreement checks
//    - Short-circuits on first mismatch for performance
//
// 2. Range Verification:
//    func VerifyRangeDecompression(decompressed, expected []byte, rangeStart int64) *VerificationResult
//    - Verifies byte ranges (e.g., HTTP Range requests)
//    - Reports mismatches at absolute offsets in the full object
//    - Supports partial object verification without full download
//
// 3. Context-Aware Verification:
//    func VerifyDecompressionWithContext(decompressed, expected []byte, context string) *VerificationResult
//    - Wraps VerifyDecompression with structured logging context
//    - Used for escalation bead correlation and multi-tenant debugging
//    - Prefixes all diagnostic messages with object/version/tenant context
//
// 4. Forensic Analysis:
//    func AnalyzeByteDifferences(decompressed, expected []byte) *ByteStats
//    - Full-scan analysis of all byte differences (does not short-circuit)
//    - Builds frequency maps of corruption patterns
//    - Used for root cause analysis and pattern detection
//
// **Return Types:**
//
// - VerificationResult: Pass/fail status, diagnostic message, error details
// - ByteStats: Comprehensive statistics about byte differences
// - VerificationError: Structured error details with offset and context
//
// **Integration Patterns:**
//
// All verification functions follow these patterns:
// - Never panic on invalid input (graceful degradation)
// - Return structured results for programmatic handling
// - Include human-readable diagnostics for debugging
// - Support both fast-failure and full-forensic workflows
//
// **Usage in Restore Verifier:**
//
// The restoreverifier package integrates these functions as:
// - Dual-path agreement checks (ARMOR vs direct decrypt)
// - DR drill validation (direct-only restore proof)
// - Escalation bead correlation with context strings
// - Forensic analysis for persistent failures
//
// See individual function documentation for detailed signature examples,
// parameter constraints, error handling patterns, and integration points.

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

// VerifyResult represents the outcome of a decompression verification operation.
//
// This type is designed to be returned by verification helpers to clearly indicate
// whether decompressed content matches expected data, and to provide diagnostic
// information when verification fails.
//
// Use Pass (not Passed) to check verification success:
//
//	result := VerifyDecompression(decompressed, expected)
//	if result.Pass {
//	    // Content is verified as correct
//	} else {
//	    // Content failed verification - check Diagnostic for details
//	    fmt.Printf("Verification failed: %s\n", result.Diagnostic)
//	}
//
// The Diagnostic field contains human-readable details about the failure:
// - Length mismatches
// - Byte offset where corruption begins
// - Hexdump context showing expected vs actual bytes
// - Statistical information about corruption patterns
//
// Example usage in HTTP handlers:
//
//	func handleRestoreRequest(w http.ResponseWriter, r *http.Request) {
//	    decompressed := decompressPayload(requestBody)
//	    expected := fetchExpectedChecksum(objectID)
//
//	    result := VerifyDecompression(decompressed, expected)
//	    if !result.Pass {
//	        http.Error(w, result.Diagnostic, http.StatusUnprocessableEntity)
//	        return
//	    }
//
//	    // Proceed with verified content
//	    processVerifiedContent(decompressed)
//	}
type VerifyResult struct {
	// Pass indicates whether the decompression verification succeeded.
	// When true, the decompressed content exactly matches the expected data.
	// When false, check Diagnostic for failure details.
	Pass bool

	// Diagnostic contains human-readable information about the verification result.
	// For successful verifications, this may include confirmation details like the
	// number of bytes verified.
	// For failed verifications, this provides actionable diagnostic information:
	//   - Length mismatch details
	//   - Byte offset of first corruption
	//   - Hexdump context around the corruption point
	//   - Statistical analysis of corruption patterns
	Diagnostic string

	// Error contains structured error details when verification fails.
	// When Pass is true, this field is nil. Use this field for programmatic
	// error handling and analysis rather than parsing Diagnostic strings.
	Error *VerificationError
}

// VerificationError describes where and how decompression verification failed.
//
// This type provides structured, machine-readable details about byte-level
// mismatches between decompressed and expected content. It is used to precisely
// identify corruption points and provide context for debugging.
//
// **Documentation Reference:**
// This type maps to the core error message structure defined in:
// - docs/error-message-structure.md (core format with required fields)
// - docs/verification-error-to-message-mapping.md (field-by-field mapping)
// - docs/verification-error-message-examples.md (concrete scenarios)
//
// The Error() method below implements the conversion to human-readable format
// following the documented message format specification.
//
// Error Type Classification:
// - Length mismatch: Decompressed content has different total size than expected
// - Byte mismatch: Content matches in length but differs at one or more byte positions
// - Offset out of range: Requested verification offset exceeds data bounds
//
// Example error cases:
//
//	1. Length mismatch (decompressed too short):
//	   expected: 1024 bytes
//	   got:      997 bytes
//	   offset:   -1 (special sentinel for length error)
//
//	2. Single byte corruption:
//	   offset:   512
//	   expected: 0xDE (at byte 512)
//	   actual:   0x00 (null byte overwrite)
//
//	3. Bit-flip corruption:
//	   offset:   1024
//	   expected: 0x55 (01010101 binary)
//	   actual:   0x54 (01010100 binary - single bit flip)
//
//	4. Burst corruption (sequential bytes corrupted):
//	   offset:   2048
//	   expected: 0x41 0x42 0x43 ("ABC")
//	   actual:   0xFF 0xFF 0xFF (burst overwrite)
//
//	5. Offset out of range:
//	   offset:   5000
//	   expected: <nil> (offset exceeds expected data length)
//	   actual:   <nil> (offset exceeds decompressed data length)
type VerificationError struct {
	// Offset is the byte position where the first difference occurs.
	//
	// **Message Format Mapping:**
	// This field maps directly to the "offset" field in the error message format
	// (see docs/error-message-structure.md). The Error() method uses this field
	// to construct messages like:
	// - "byte mismatch at offset 512..." (when Offset >= 0)
	// - "length mismatch..." (when Offset == -2)
	//
	// How to calculate the offset:
	// 1. Start byte-by-byte comparison from position 0
	// 2. Compare decompressed[i] with expected[i] for each i
	// 3. The first i where decompressed[i] != expected[i] is the offset
	// 4. If all bytes match up to min(len(decompressed), len(expected)),
	//    the offset is min(len(decompressed), len(expected))
	//
	// Special offset values:
	// - -1: Content is identical (no error, used for consistency)
	// - -2: Length mismatch error (total sizes differ)
	// - < -2: Reserved for future error types
	//
	// Examples:
	//   decompressed: [0x01, 0x02, 0x00, 0x04] (4 bytes)
	//   expected:      [0x01, 0x02, 0x03, 0x04] (4 bytes)
	//   offset:       2 (third byte differs: 0x00 vs 0x03)
	Offset int64

	// Expected is the byte or byte sequence that was expected at the error offset.
	//
	// **Message Format Mapping:**
	// This field maps to the "expected" field in the error message format.
	// In Error() method:
	// - Single byte → hex format: "expected 0x03"
	// - Multi-byte → description: "expected N-byte context"
	//
	// For single-byte errors, this contains a single byte.
	// For multi-byte context (when ContextBytes > 0), this contains a slice
	// of [ContextBefore + 1 + ContextAfter] bytes centered on the error offset.
	//
	// If offset is -2 (length mismatch), Expected contains the full expected data
	// for size comparison. If offset is out of range, Expected is nil.
	//
	// Examples:
	//   Single-byte error:    []byte{0x03}
	//   With context (8 bytes): []byte{0x01, 0x02, 0x03, 0x04, 0x03, 0x06, 0x07, 0x08}
	//                                             ^^^^ center 4 bytes are around error
	Expected []byte

	// Actual is the byte or byte sequence that was found in the decompressed content
	// at the error offset.
	//
	// **Message Format Mapping:**
	// This field maps to the "actual" field in the error message format.
	// In Error() method:
	// - Single byte → hex format: "got 0x00"
	// - Multi-byte → description: "got N-byte context"
	//
	// Mirrors the Expected field structure:
	// - Single-byte errors: one byte
	// - With context: [ContextBefore + 1 + ContextAfter] bytes
	// - Length mismatch: full decompressed data
	// - Out of range: nil
	//
	// Examples:
	//   Single-byte error:    []byte{0x00} (corruption: null byte)
	//   Bit-flip error:      []byte{0x54} (0x55 expected, 1 bit differs)
	//   With context:         []byte{0x01, 0x02, 0x00, 0x04, 0x05, 0x06, 0x07, 0x08}
	Actual []byte

	// ContextBytes specifies the number of surrounding bytes included in Expected
	// and Actual on each side of the error offset.
	//
	// A value of 0 means only the exact byte at the error offset is included.
	// A value of 16 means 16 bytes before + the error byte + 16 bytes after = 33 total bytes.
	//
	// Context helps identify corruption patterns:
	// - Sequential 0x00 bytes suggests null overwrite burst
	// - Repeated 0xFF suggests uninitialized memory or erase pattern
	// - Incrementing values suggests pointer/index corruption
	// - ASCII-printable differences suggests text encoding issues
	//
	// Example context values:
	//   0:  Minimal context, only the error byte
	//   8:  Short context for compact errors
	//   16: Standard context for hexdump-style display
	//   32: Extended context for pattern analysis
	//   64: Full context for detailed forensics
	//
	// Context calculation:
	//   totalBytes = ContextBefore + 1 + ContextAfter
	//   where ContextBefore = ContextBytes (or fewer if near start)
	//   and   ContextAfter  = ContextBytes (or fewer if near end)
	ContextBytes int

	// ContextBefore is the actual number of bytes before the error offset included
	// in Expected and Actual. This may be less than ContextBytes if the error occurs
	// near the start of the data (offset < ContextBytes).
	//
	// Example:
	//   ContextBytes: 16
	//   Offset: 5
	//   ContextBefore: 5 (can't show 16 bytes before position 5)
	//   Total included: 5 + 1 + 16 = 12 bytes (not 33)
	ContextBefore int

	// ContextAfter is the actual number of bytes after the error offset included
	// in Expected and Actual. This may be less than ContextBytes if the error occurs
	// near the end of the data (offset + ContextBytes >= len(data)).
	//
	// Example:
	//   ContextBytes: 16
	//   Data length: 20
	//   Offset: 18
	//   ContextAfter: 1 (only 1 byte exists after position 18)
	//   Total included: 16 + 1 + 1 = 18 bytes (not 33)
	ContextAfter int

	// ExpectedLength is the total length of the expected (correct) data.
	// This is useful for diagnosing length mismatch errors where Offset == -2.
	//
	// Example:
	//   ExpectedLength: 1024
	//   ActualLength: 997
	//   Interpretation: Decompressed output is 27 bytes too short
	ExpectedLength int

	// ActualLength is the total length of the decompressed data that was verified.
	// This is useful for diagnosing length mismatch errors where Offset == -2.
	//
	// Example:
	//   ExpectedLength: 1024
	//   ActualLength: 1056
	//   Interpretation: Decompressed output is 32 bytes too long (trailing data)
	ActualLength int
}

// ========================================================================
// VerificationError to Message Mapping - Documentation Integration
// ========================================================================
// The following sections demonstrate how VerificationError instances map
// to both human-readable messages and JSON output for common failure scenarios.
//
// **Documentation Mapping:**
// Each scenario below corresponds to examples in the documentation:
// - Scenario 1 (Single byte mismatch) → docs/verification-error-message-examples.md Example 1
// - Scenario 2 (Multi-byte mismatch) → docs/verification-error-message-examples.md Example 2
// - Scenarios 3-7 → Additional failure patterns documented in mapping spec
//
// **Construction Pattern:**
// For each scenario, the pattern is:
// 1. Create VerificationError instance with appropriate fields
// 2. Call Error() method to construct human-readable message
// 3. Reference JSON serialization format (for structured output)
// 4. Show interpretation and diagnostic value
//
// See docs/verification-error-to-message-mapping.md for complete field-by-field
// mapping and transformation logic.
//
// Scenario 1: Single byte mismatch at a specific offset
// -------------------------------------------------------
// A single byte at position 512 is corrupted (null byte overwrite).
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:        512,
//       Expected:      []byte{0x03},
//       Actual:        []byte{0x00},
//       ContextBytes:  0,
//       ExpectedLength: 1024,
//       ActualLength:  1024,
//   }
//
// **Human-readable message:**
//   "verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)"
//
// **JSON output (required fields only):**
//   {
//     "offset": 512,
//     "expected": "0x03",
//     "actual": "0x00"
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": 512,
//     "expected": "0x03",
//     "actual": "0x00",
//     "error_type": "byte_mismatch",
//     "severity": "high"
//   }
//
// **Interpretation:** Single byte corruption at offset 512 - null byte (0x00) overwrote expected data (0x03)
//
// Scenario 2: Multi-byte mismatch (corrupted chunk)
// ------------------------------------------------
// A sequential burst of corrupted bytes starting at offset 1024.
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:        1024,
//       Expected:      []byte{0x41, 0x42, 0x43, 0x44, 0x45}, // "ABCDE"
//       Actual:        []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, // burst overwrite
//       ContextBytes:  16,
//       ContextBefore: 16,
//       ContextAfter:  16,
//       ExpectedLength: 2048,
//       ActualLength:  2048,
//   }
//
// **Human-readable message:**
//   "verification failed: byte mismatch at offset 1024 (expected 5-byte context, got 5-byte context) [16 bytes context: 16 before, 16 after]"
//
// **JSON output (required fields only):**
//   {
//     "offset": 1024,
//     "expected": "0x41 0x42 0x43 0x44 0x45",
//     "actual": "0xFF 0xFF 0xFF 0xFF 0xFF"
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": 1024,
//     "expected": "0x41 0x42 0x43 0x44 0x45",
//     "actual": "0xFF 0xFF 0xFF 0xFF 0xFF",
//     "context_bytes": 16,
//     "context_before": 16,
//     "context_after": 16,
//     "error_type": "byte_mismatch",
//     "severity": "medium"
//   }
//
// **Interpretation:** Burst corruption starting at offset 1024 - 5 sequential bytes overwritten with 0xFF pattern
//
// Scenario 3: Offset mismatch in range request
// --------------------------------------------
// Verification of an HTTP range request fails at a specific absolute offset.
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:        1536,  // Absolute offset in full object
//       Expected:      []byte{0x00, 0x01, 0x02, 0x03},
//       Actual:        []byte{0x00, 0x01, 0xFF, 0x03},  // Byte 2 corrupted in range
//       ContextBytes:  8,
//       ContextBefore: 2,
//       ContextAfter:  2,
//       ExpectedLength: 1024,  // Range length
//       ActualLength:  1024,
//   }
//
// **Human-readable message (from VerifyRangeDecompression):**
//   "range byte mismatch at absolute offset 1536 (relative offset 512 within range 1024-2047): expected 0x02 (2), got 0xFF (255)"
//
// **JSON output (required fields only):**
//   {
//     "offset": 1536,
//     "expected": "0x00 0x01 0x02 0x03",
//     "actual": "0x00 0x01 0xFF 0x03"
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": 1536,
//     "expected": "0x00 0x01 0x02 0x03",
//     "actual": "0x00 0x01 0xFF 0x03",
//     "context_bytes": 8,
//     "context_before": 2,
//     "context_after": 2,
//     "error_type": "range_mismatch",
//     "severity": "medium",
//     "range_start": 1024,
//     "range_end": 2047
//   }
//
// **Interpretation:** Range request verification failed - corruption at relative offset 512 within the requested range
//
// Scenario 4: Entire object mismatch (wrong data entirely)
// -------------------------------------------------------
// The decompressed output is completely different from expected (wrong object, wrong decryption key, etc.).
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:        0,  // First byte already differs
//       Expected:      []byte{0x89, 0x50, 0x4E, 0x47}, // PNG signature
//       Actual:        []byte{0x50, 0x4B, 0x03, 0x04}, // ZIP signature
//       ContextBytes:  0,
//       ExpectedLength: 1048576,
//       ActualLength:  1048576,
//   }
//
// **Human-readable message:**
//   "verification failed: byte mismatch at offset 0 (expected 0x89, got 0x50)"
//
// **JSON output (required fields only):**
//   {
//     "offset": 0,
//     "expected": "0x89",
//     "actual": "0x50"
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": 0,
//     "expected": "0x89",
//     "actual": "0x50",
//     "error_type": "byte_mismatch",
//     "severity": "critical"
//   }
//
// **Interpretation:** Completely wrong data - expected PNG file signature, got ZIP file signature (wrong object retrieved or decryption error)
//
// Scenario 5: Missing or truncated data
// ------------------------------------
// Decompressed output is shorter than expected (incomplete decompression, network truncation, etc.).
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:         -2,  // Special code for length mismatch
//       Expected:       nil,  // Not applicable for length mismatch
//       Actual:         nil,
//       ContextBytes:   0,
//       ExpectedLength: 1024,
//       ActualLength:   997,  // 27 bytes missing
//   }
//
// **Human-readable message:**
//   "verification failed: length mismatch (got 997 bytes, expected 1024 bytes)"
//
// **JSON output (required fields only):**
//   {
//     "offset": -2,
//     "expected_length": 1024,
//     "actual_length": 997
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": -2,
//     "expected_length": 1024,
//     "actual_length": 997,
//     "error_type": "length_mismatch",
//     "severity": "critical",
//     "missing_bytes": 27
//   }
//
// **Interpretation:** Data truncation - decompressed output is 27 bytes too short (incomplete transfer or decompression error)
//
// Scenario 6: Bit-flip corruption
// -------------------------------
// A single bit differs at a specific offset (often indicates transmission error or memory corruption).
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:        2048,
//       Expected:      []byte{0x55}, // 01010101 binary
//       Actual:        []byte{0x54}, // 01010100 binary - LSB flipped
//       ContextBytes:  0,
//       ExpectedLength: 4096,
//       ActualLength:  4096,
//   }
//
// **Human-readable message:**
//   "verification failed: byte mismatch at offset 2048 (expected 0x55, got 0x54)"
//
// **JSON output (required fields only):**
//   {
//     "offset": 2048,
//     "expected": "0x55",
//     "actual": "0x54"
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": 2048,
//     "expected": "0x55",
//     "actual": "0x54",
//     "error_type": "bit_flip",
//     "severity": "low"
//   }
//
// **Interpretation:** Single-bit difference - suggests transmission error or memory corruption rather than data replacement
//
// Scenario 7: Offset out of range
// ------------------------------
// The requested offset exceeds the bounds of the data (programming error or corrupted metadata).
//
// **VerificationError instance:**
//   verr := &VerificationError{
//       Offset:        5000,  // Exceeds data length
//       Expected:      nil,  // Out of range
//       Actual:        nil,
//       ContextBytes:  0,
//       ExpectedLength: 1024,
//       ActualLength:  1024,
//   }
//
// **Human-readable message:**
//   "verification failed: invalid offset 5000"
//
// **JSON output (required fields only):**
//   {
//     "offset": 5000,
//     "expected": null,
//     "actual": null
//   }
//
// **JSON output (with optional context fields):**
//   {
//     "offset": 5000,
//     "expected": null,
//     "actual": null,
//     "error_type": "out_of_range",
//     "severity": "unknown",
//     "data_length": 1024
//   }
//
// **Interpretation:** Invalid offset request - offset 5000 exceeds data length of 1024 bytes (metadata corruption or programming error)
//
// ========================================================================
// VerificationError to Message Mapping Summary
// ========================================================================
// The Error() method converts VerificationError instances to human-readable
// messages following this mapping logic:
//
// 1. nil error → "verification failed: nil error"
// 2. Offset == -2 → "verification failed: length mismatch (got X bytes, expected Y bytes)"
// 3. Offset < -2 → "verification failed: invalid offset X"
// 4. Offset >= 0 → "verification failed: byte mismatch at offset X (expected Y, got Z)"
//    - Appends "[C bytes context: B before, A after]" if ContextBytes > 0
//
// Special offset values:
// - -1: No error (content matches) - typically represented by Pass=true instead
// - -2: Length mismatch (total sizes differ)
// - < -2: Reserved for future error types
//

// Error implements the error interface for VerificationError.
//
// Returns a human-readable string describing the verification failure.
// The format is intentionally detailed to aid debugging without requiring
// field inspection.
//
// **Message Construction Process:**
// This method implements the error message format specification defined in
// docs/error-message-structure.md. The mapping from VerificationError fields
// to message format is:
//
// 1. nil error → "verification failed: nil error"
// 2. Offset == -2 → "verification failed: length mismatch (got X bytes, expected Y bytes)"
//    - Maps from: ActualLength, ExpectedLength fields
// 3. Offset < -2 → "verification failed: invalid offset X"
//    - Reserved for future error types
// 4. Offset >= 0 → "verification failed: byte mismatch at offset X (expected Y, got Z)"
//    - Maps from: Offset, Expected[0], Actual[0] (or full context arrays)
//    - Appends context information if ContextBytes > 0
//
// **Transformations Applied:**
// - Single-byte arrays → hex format: "0x03"
// - Multi-byte arrays → description: "N-byte context"
// - ContextBytes > 0 → append: "[C bytes context: B before, A after]"
//
// See docs/verification-error-to-message-mapping.md for complete field-by-field
// mapping documentation and transformation examples.
//
// Example outputs:
//
//	"verification failed: length mismatch (got 997 bytes, expected 1024 bytes)"
//	"verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)"
//	"verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00) [16 bytes context: 16 before, 16 after]"
//	"verification failed: invalid offset 5000"
func (ve *VerificationError) Error() string {
	if ve == nil {
		return "verification failed: nil error"
	}

	// Length mismatch (special offset code)
	// Maps: ActualLength → "got X bytes", ExpectedLength → "expected Y bytes"
	if ve.Offset == -2 {
		return fmt.Sprintf("verification failed: length mismatch (got %d bytes, expected %d bytes)",
			ve.ActualLength, ve.ExpectedLength)
	}

	// Out of range offset
	// Reserved for future error types (see docs/error-message-structure.md §Offset)
	if ve.Offset < 0 {
		return fmt.Sprintf("verification failed: invalid offset %d", ve.Offset)
	}

	// Build expected/actual representation
	// Transformation: byte arrays → formatted strings
	// - Single byte: hex format "0x03" (from docs/error-message-structure.md)
	// - Multi-byte: description "N-byte context"
	expectedStr := "<nil>"
	actualStr := "<nil>"

	if len(ve.Expected) > 0 {
		if len(ve.Expected) == 1 {
			// Single-byte format: "0x03" (hexadecimal representation)
			expectedStr = fmt.Sprintf("0x%02X", ve.Expected[0])
		} else {
			// Multi-byte format: "N-byte context" (size description)
			expectedStr = fmt.Sprintf("%d-byte context", len(ve.Expected))
		}
	}

	if len(ve.Actual) > 0 {
		if len(ve.Actual) == 1 {
			// Single-byte format: "0x00" (hexadecimal representation)
			actualStr = fmt.Sprintf("0x%02X", ve.Actual[0])
		} else {
			// Multi-byte format: "N-byte context" (size description)
			actualStr = fmt.Sprintf("%d-byte context", len(ve.Actual))
		}
	}

	// Byte mismatch error
	msg := fmt.Sprintf("verification failed: byte mismatch at offset %d (expected %s, got %s)",
		ve.Offset, expectedStr, actualStr)

	// Add context information if available
	if ve.ContextBytes > 0 {
		msg += fmt.Sprintf(" [%d bytes context: %d before, %d after]",
			ve.ContextBytes, ve.ContextBefore, ve.ContextAfter)
	}

	return msg
}

// IsLengthMismatch returns true if this error represents a length mismatch
// (decompressed and expected have different total sizes).
func (ve *VerificationError) IsLengthMismatch() bool {
	return ve != nil && ve.Offset == -2
}

// IsByteMismatch returns true if this error represents a byte-level mismatch
// (content differs at a specific offset).
func (ve *VerificationError) IsByteMismatch() bool {
	return ve != nil && ve.Offset >= 0
}

// IsOutOfRange returns true if this error represents an out-of-range offset.
func (ve *VerificationError) IsOutOfRange() bool {
	return ve != nil && ve.Offset < -2
}

// MismatchSeverity classifies the severity of the verification error.
//
// Returns:
// - "critical": Length mismatch (data truncation or extension)
// - "high": Single-byte mismatch at critical position (header, metadata)
// - "medium": Multi-byte corruption or data-area corruption
// - "low": Recoverable corruption (can be corrected with redundancy)
// - "unknown": Error type not recognized
func (ve *VerificationError) MismatchSeverity() string {
	if ve == nil {
		return "unknown"
	}

	switch {
	case ve.Offset == -2:
		return "critical" // Length mismatch is always critical
	case ve.Offset < -2:
		return "unknown" // Reserved error codes
	case ve.Offset < 256:
		return "high" // Header/metadata corruption
	default:
		// Check if it's a burst error (many sequential bytes corrupted)
		// This is a heuristic - in practice you'd analyze the full byte stream
		return "medium"
	}
}

// String returns a formatted string representation of the verification result.
// This implements the fmt.Stringer interface for easy logging and debugging.
//
// Example:
//
//	result := VerifyDecompression(data, expected)
//	log.Printf("Verification result: %s", result)  // Logs: "PASS: 1024 bytes verified" or "FAIL: ..."
func (vr VerifyResult) String() string {
	if vr.Pass {
		return fmt.Sprintf("PASS: %s", vr.Diagnostic)
	}
	return fmt.Sprintf("FAIL: %s", vr.Diagnostic)
}

// VerificationResult is an alias for VerifyResult for backward compatibility.
// Deprecated: Use VerifyResult directly.
type VerificationResult = VerifyResult

// BytesMismatch provides detailed information about byte-level mismatches.
type BytesMismatch struct {
	Offset        int64  // byte offset where mismatch occurs
	ExpectedByte  byte   // expected byte value at offset
	ActualByte    byte   // actual byte value at offset
	ExpectedHex   string // hexadecimal representation of expected byte
	ActualHex     string // hexadecimal representation of actual byte
	ExpectedCtx   string // context around expected mismatch (hexdump)
	ActualCtx     string // context around actual mismatch (hexdump)
	ContextBefore int    // number of bytes before mismatch in context
	ContextAfter  int    // number of bytes after mismatch in context
}

// VerifyDecompression verifies that decompressed content matches the original byte-for-byte.
//
// This is the core full-object verification function that validates that a decompressed
// byte stream exactly matches its expected plaintext. It is used throughout ARMOR's
// restore verification pipeline to prove that the decrypt-then-decompress path
// produces correct output.
//
// **Function Signature:**
//
//	func VerifyDecompression(decompressed, expected []byte) *VerifyResult
//
// **Conceptual Integration Signature (for restore verifier):**
//
//	// In the context of restoreverifier.VerifyObject, this would be called as:
//	result := crypto.VerifyDecompression(
//	    decompressedPlaintext,  // from crypto.Decompress(decryptedData)
//	    expectedPlaintext,      // from original source or metadata
//	)
//
// **Parameters:**
//   - decompressed: The output from the decompression pipeline (e.g., from crypto.Decompress
//     after decrypting ciphertext). This is the data under test.
//   - expected: The original plaintext data that was compressed before encryption. This is the
//     reference data for comparison. Can be sourced from:
//     * Original unencrypted backup (golden copy)
//     * Previous verification run output
//     * Known-good test fixtures
//
// **Return Type:**
//   - *VerifyResult: Contains pass/fail status, diagnostic message, and error details.
//     The result.Pass field is the primary success indicator; result.Diagnostic provides
//     human-readable details; result.Error contains structured VerificationError.
//
// **Constraints:**
//   - Both parameters must be non-nil byte slices
//   - Empty slices are valid (will result in length mismatch if only one is empty)
//   - No maximum size limit (constrained only by available memory)
//   - Comparison stops at first mismatch for performance (use AnalyzeByteDifferences for full scan)
//
// **Validation:**
//   - Length mismatch is detected first (fast path)
//   - Byte-for-byte comparison stops at first mismatch (optimized for performance)
//   - Context of 16 bytes surrounding the mismatch is captured for diagnostics
//
// **Error Handling:**
//   - Never panics, even with nil or malformed input
//   - Returns VerifyResult with Pass=false and appropriate VerificationError
//   - VerificationError.Offset indicates failure type:
//     - -1: Verification passed (no error)
//     - -2: Length mismatch
//     - >= 0: Byte offset of first difference
//
// **Usage Example:**
//
//	// After retrieving and decrypting an object
//	decompressed, err := crypto.Decompress(decryptedData)
//	if err != nil {
//	    return fmt.Errorf("decompression failed: %w", err)
//	}
//
//	// Verify against the original plaintext (e.g., from B2 metadata or known-good copy)
//	result := crypto.VerifyDecompression(decompressed, originalPlaintext)
//	if !result.Pass {
//	    return fmt.Errorf("decompression verification failed: %s", result.Diagnostic)
//	}
//
// **Integration with Backend:**
//
//	// The restore verifier integrates this as:
//	plaintext, err := v.backend.Get(ctx, bucket, key)  // decrypts via ARMOR read path
//	if err != nil {
//	    return fmt.Errorf("restore error: %w", err)
//	}
//
//	// Read and verify
//	decompressed, _ := io.ReadAll(plaintext)
//	defer plaintext.Close()
//
//	expectedPlaintext := getExpectedPlaintext(objectID)
//	result := crypto.VerifyDecompression(decompressed, expectedPlaintext)
//	if !result.Pass {
//	    return fmt.Errorf("checksum error: %s", result.Diagnostic)
//	}
//
// **Performance:**
//   - O(n) time complexity where n is the length of the shorter slice
//   - O(1) additional memory (only stores mismatch offset and context)
//   - Short-circuits on first mismatch for fast failure detection
func VerifyDecompression(decompressed, expected []byte) *VerifyResult {
	const contextBytes = 16 // show 16 bytes before and after mismatch

	// Handle nil inputs gracefully
	if decompressed == nil && expected == nil {
		return &VerifyResult{
			Pass:       true,
			Diagnostic: "verified: both inputs are nil",
		}
	}

	// Check if lengths match
	if len(decompressed) != len(expected) {
		return &VerifyResult{
			Pass: false,
			Diagnostic: fmt.Sprintf("length mismatch: got %d bytes, expected %d bytes",
				len(decompressed), len(expected)),
			Error: &VerificationError{
				Offset:         -2, // Special code for length mismatch
				Expected:       nil,
				Actual:         nil,
				ContextBytes:   0,
				ContextBefore:  0,
				ContextAfter:   0,
				ExpectedLength: len(expected),
				ActualLength:   len(decompressed),
			},
		}
	}

	// Handle empty slices (both are empty and have same length)
	if len(decompressed) == 0 {
		return &VerifyResult{
			Pass:       true,
			Diagnostic: "verified: 0 bytes (empty)",
		}
	}

	// Perform byte-for-byte comparison with early exit on first mismatch
	for i := 0; i < len(decompressed); i++ {
		if decompressed[i] != expected[i] {
			// Found the first mismatch - capture context and return
			return createMismatchResult(decompressed, expected, i, contextBytes)
		}
	}

	// All bytes match
	return &VerifyResult{
		Pass:       true,
		Diagnostic: fmt.Sprintf("verified: %d bytes match exactly", len(decompressed)),
	}
}

// createMismatchResult creates a VerifyResult for a byte mismatch at the given offset.
func createMismatchResult(decompressed, expected []byte, offset int, contextBytes int) *VerifyResult {
	// Calculate context boundaries
	contextBefore := contextBytes
	if offset-contextBefore < 0 {
		contextBefore = offset
	}

	contextAfter := contextBytes
	if offset+contextAfter >= len(decompressed) {
		contextAfter = len(decompressed) - offset - 1
	}

	// Extract context byte ranges
	expectedStart := offset - contextBefore
	expectedEnd := offset + contextAfter + 1
	actualStart := offset - contextBefore
	actualEnd := offset + contextAfter + 1

	// Ensure bounds are valid
	if expectedEnd > len(expected) {
		expectedEnd = len(expected)
	}
	if actualEnd > len(decompressed) {
		actualEnd = len(decompressed)
	}
	if expectedStart < 0 {
		expectedStart = 0
	}
	if actualStart < 0 {
		actualStart = 0
	}

	// Build context slices for diagnostic value
	expectedCtx := expected[expectedStart:expectedEnd]
	actualCtx := decompressed[actualStart:actualEnd]

	// Create the error
	verr := &VerificationError{
		Offset:         int64(offset),
		Expected:       expectedCtx,
		Actual:         actualCtx,
		ContextBytes:   contextBytes,
		ContextBefore:  contextBefore,
		ContextAfter:   contextAfter,
		ExpectedLength: len(expected),
		ActualLength:   len(decompressed),
	}

	return &VerifyResult{
		Pass:       false,
		Diagnostic: verr.Error(),
		Error:      verr,
	}
}

// VerifyRangeDecompression verifies that a decompressed range matches the expected range.
//
// This function validates HTTP range requests and partial object reads. It verifies that
// a byte range extracted from an object (e.g., bytes=1024-2047) decompresses correctly
// and matches the corresponding slice of the original plaintext.
//
// **Function Signature:**
//
//	func VerifyRangeDecompression(decompressed, expected []byte, rangeStart int64) *VerificationResult
//
// **Conceptual Integration Signature (for backend range requests):**
//
//	// In the context of backend.GetRange, this would be called as:
//	ciphertext, err := backend.GetRange(ctx, bucket, key, offset, length)
//	decryptedRange := decryptor.DecryptRange(ciphertext, offset, length)
//	decompressedRange := crypto.Decompress(decryptedRange)
//	expectedRange := originalPlaintext[offset : offset+length]
//
//	result := crypto.VerifyRangeDecompression(decompressedRange, expectedRange, offset)
//
// **Parameters:**
//   - decompressed: The decompressed range data (output from range-specific decompression)
//   - expected: Expected range data (original plaintext slice for the range)
//   - rangeStart: The absolute byte offset where the range starts in the full object. This is
//     critical for reporting absolute mismatch positions in error messages.
//
// **Return Type:**
//   - *VerificationResult: Contains pass/fail status, diagnostic message, and error details
//     - ByteOffset is relative to the full object (absolute), not the range
//     - Diagnostic messages include both absolute and relative offset information
//     - Example error: "range byte mismatch at absolute offset 1536 (relative offset 512 within range 1024-2047)"
//
// **Constraints:**
//   - rangeStart must be >= 0 (negative offsets are invalid)
//   - decompressed and expected must have the same length for verification to pass
//   - Empty range (length 0) is valid and will pass if both slices are empty
//   - Range must be within bounds of the original object (caller's responsibility)
//
// **Validation:**
//   - Range length mismatch is detected first
//   - Byte-for-byte comparison operates on the range data only
//   - Mismatch offsets are reported as absolute positions in the full object
//   - Context windows are adjusted near range boundaries
//
// **Error Handling:**
//   - Never panics, even with nil or malformed input
//   - Returns VerificationResult with Passed=false and appropriate diagnostic message
//   - ByteOffset field indicates absolute position in the full object:
//     - rangeStart + relative_offset: Where the mismatch occurs in the full object
//     - -2: Length mismatch between decompressed range and expected range
//     - -1: Verification passed
//
// **Usage Example:**
//
//	// For HTTP range requests (e.g., "Range: bytes=1024-2047")
//	rangeStart := int64(1024)
//	rangeEnd := int64(2047)
//	rangeLength := rangeEnd - rangeStart + 1
//
//	// Retrieve and decrypt the range
//	decryptedRange, err := retrieveAndDecryptRange(objectID, rangeStart, rangeLength)
//	if err != nil {
//	    return err
//	}
//
//	decompressedRange, err := crypto.Decompress(decryptedRange)
//	if err != nil {
//	    return fmt.Errorf("range decompression failed: %w", err)
//	}
//
//	// Extract the expected range from the original
//	expectedRange := originalPlaintext[rangeStart : rangeEnd+1]
//
//	// Verify
//	result := crypto.VerifyRangeDecompression(decompressedRange, expectedRange, rangeStart)
//	if !result.Passed() {
//	    return fmt.Errorf("range verification failed at offset %d: %s",
//	        result.ByteOffset, result)
//	}
//
// **Integration with Backend.GetRange:**
//
//	// The backend integrates this for range request verification:
//	func (b *Backend) VerifyRange(ctx context.Context, bucket, key string, offset, length int64) (*VerificationResult, error) {
//	    // Get the encrypted range
//	    ciphertext, err := b.GetRange(ctx, bucket, key, offset, length)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Decrypt the range
//	    decrypted, err := crypto.DecryptRange(ciphertext, dek, iv, offset, blockSize)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Decompress
//	    decompressed, err := crypto.Decompress(decrypted)
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Get expected range from metadata or reference copy
//	    expected := getExpectedRange(bucket, key, offset, length)
//
//	    // Verify
//	    return crypto.VerifyRangeDecompression(decompressed, expected, offset), nil
//	}
//
// **Performance:**
//   - O(m) time complexity where m is the range length
//   - O(1) additional memory
//   - Short-circuits on first mismatch
//
// **Integration Point:**
//   - Used in HTTP range request handlers (e.g., GET with Range header)
//   - Can be integrated with backend.GetRange() for direct range verification
//   - Supports partial object verification without full download
func VerifyRangeDecompression(decompressed, expected []byte, rangeStart int64) *VerifyResult {
	const contextBytes = 16

	// Check if decompressed length matches expected range length
	if len(decompressed) != len(expected) {
		return &VerifyResult{
			Pass: false,
			Diagnostic: fmt.Sprintf("range length mismatch: got %d bytes, expected %d bytes (range starts at offset %d)",
				len(decompressed), len(expected), rangeStart),
			Error: &VerificationError{
				Offset:         -2,
				Expected:       nil,
				Actual:         nil,
				ContextBytes:   0,
				ContextBefore:  0,
				ContextAfter:   0,
				ExpectedLength: len(expected),
				ActualLength:   len(decompressed),
			},
		}
	}

	// Perform byte-for-byte comparison
	if bytes.Equal(decompressed, expected) {
		return &VerifyResult{
			Pass:       true,
			Diagnostic: fmt.Sprintf("range verified: %d bytes match exactly (range starts at offset %d)",
				len(decompressed), rangeStart),
		}
	}

	// Find the first mismatching byte within the range
	offset := findFirstMismatch(decompressed, expected)
	absoluteOffset := rangeStart + int64(offset)

	// Create mismatch result with absolute offset
	result := createMismatchResult(decompressed, expected, offset, contextBytes)
	// Update the offset to be absolute
	if result.Error != nil {
		result.Error.Offset = absoluteOffset
		result.Diagnostic = fmt.Sprintf("range byte mismatch at absolute offset %d (relative offset %d within range): %s",
			absoluteOffset, offset, result.Diagnostic)
	}

	return result
}

// VerifyDecompressionWithContext performs verification with additional context information.
//
// This function wraps VerifyDecompression with structured context for distributed systems.
// It prefixes all diagnostic messages with a context string (e.g., object ID, version, tenant)
// to make logs and error messages actionable in multi-tenant, high-scale environments.
//
// **Function Signature:**
//
//	func VerifyDecompressionWithContext(decompressed, expected []byte, context string) *VerificationResult
//
// **Conceptual Integration Signature (for structured logging):**
//
//	// In the context of restoreverifier.verifyObjectDual, this would be called as:
//	context := fmt.Sprintf("object=%s/%s, version=%s, path=armor", bucket, key, metadata["version"])
//	result := crypto.VerifyDecompressionWithContext(
//	    armorPlaintext,
//	    directPlaintext,
//	    context,
//	)
//	// result.Message will be: "[object=my-bucket/key.db, version=3, path=armor] verified: 524288 bytes match exactly"
//
// **Parameters:**
//   - decompressed: The output from the decompression pipeline
//   - expected: The original plaintext data
//   - context: Human-readable context string for logging/debugging. Recommended formats:
//     * "object=bucket/key, version=X"
//     * "tenant=acme, object=backup.db, attempt=3/5"
//     * "path=armor, checksum=expected_vs_actual"
//     * "dr-drill=true, mode=direct-only"
//
// **Return Type:**
//   - *VerificationResult: Same as VerifyDecompression, but with Message prefixed by context
//     - Example success: "[object=my-bucket/key, version=123] verified: 1048576 bytes match exactly"
//     - Example failure: "[object=my-bucket/key, version=123] byte mismatch at offset 512..."
//
// **Constraints:**
//   - Inherits all constraints from VerifyDecompression
//   - context string can be empty (no prefix added)
//   - context is prepended to the Message field in format: "[context] original_message"
//   - Context should be relatively short (< 200 chars) for log readability
//
// **Validation:**
//   - Delegates to VerifyDecompression for all validation
//   - Context string is not validated (used as-is for diagnostic purposes)
//
// **Error Handling:**
//   - Same error handling as VerifyDecompression
//   - Context string is preserved even on verification failure
//   - Empty context produces identical output to VerifyDecompression
//
// **Usage Example:**
//
//	result := crypto.VerifyDecompressionWithContext(
//	    decompressed,
//	    originalPlaintext,
//	    fmt.Sprintf("object=%s, version=%d", objectID, version),
//	)
//	if !result.Passed() {
//	    log.Error("Verification failed", "detail", result.Message)
//	    // result.Message will be: "[object=my-bucket/key, version=123] byte mismatch at offset 512..."
//	    return result
//	}
//
// **Integration with Escalation:**
//
//	// The restore verifier uses context for escalation bead correlation:
//	context := fmt.Sprintf("bucket=%s, key=%s, path=%s", bucket, key, "dual")
//	result := crypto.VerifyDecompressionWithContext(armorResult, directResult, context)
//
//	if !result.Passed() {
//	    // Context is embedded in the escalation bead for correlation
//	    escalateResult(ctx, obj, result, context)
//	}
//
// **Performance:**
//   - Identical to VerifyDecompression (context string addition is O(1))
//
// **Use Cases:**
//   - Structured logging in distributed systems
//   - Correlating verification failures with specific objects/versions
//   - Debugging multi-tenant systems where context identifies the tenant
//   - Escalation bead correlation (ADR-004 §5)
//   - DR drill vs. normal path distinction in logs
func VerifyDecompressionWithContext(decompressed, expected []byte, context string) *VerifyResult {
	result := VerifyDecompression(decompressed, expected)
	result.Diagnostic = fmt.Sprintf("[%s] %s", context, result.Diagnostic)
	return result
}

// GetMismatchDetail extracts detailed information about byte mismatches.
// Returns nil if verification passed.
func (vr *VerifyResult) GetMismatchDetail(decompressed, expected []byte) *BytesMismatch {
	if vr.Pass || vr.Error == nil || vr.Error.Offset < 0 {
		return nil
	}

	// Convert absolute offset to relative if needed
	offset := int(vr.Error.Offset)
	if offset >= len(decompressed) {
		offset = 0
	}

	return createMismatchDetail(decompressed, expected, offset, vr.Error.ContextBytes)
}

// Passed returns true if verification passed.
func (vr *VerifyResult) Passed() bool {
	return vr.Pass
}

// findFirstMismatch finds the byte offset where two byte slices first differ.
// Returns -1 if slices are identical.
func findFirstMismatch(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}

	// If we get here, the common prefix matches but lengths differ
	if len(a) != len(b) {
		return minLen
	}

	return -1 // identical
}

// createMismatchDetail creates detailed mismatch information.
func createMismatchDetail(decompressed, expected []byte, offset int, contextSize int) *BytesMismatch {
	if offset < 0 || offset >= len(decompressed) || offset >= len(expected) {
		return nil
	}

	before := contextSize
	after := contextSize

	if offset-before < 0 {
		before = offset
	}
	if offset+after >= len(decompressed) {
		after = len(decompressed) - offset - 1
	}
	if offset+after >= len(expected) {
		after = len(expected) - offset - 1
	}

	expectedStart := offset - before
	expectedEnd := offset + after + 1
	actualStart := offset - before
	actualEnd := offset + after + 1

	// Ensure bounds are valid
	if expectedEnd > len(expected) {
		expectedEnd = len(expected)
	}
	if actualEnd > len(decompressed) {
		actualEnd = len(decompressed)
	}
	if expectedStart < 0 {
		expectedStart = 0
	}
	if actualStart < 0 {
		actualStart = 0
	}

	return &BytesMismatch{
		Offset:        int64(offset),
		ExpectedByte:  expected[offset],
		ActualByte:    decompressed[offset],
		ExpectedHex:   hex.EncodeToString([]byte{expected[offset]}),
		ActualHex:     hex.EncodeToString([]byte{decompressed[offset]}),
		ExpectedCtx:   hex.EncodeToString(expected[expectedStart:expectedEnd]),
		ActualCtx:     hex.EncodeToString(decompressed[actualStart:actualEnd]),
		ContextBefore: before,
		ContextAfter:  after,
	}
}

// formatMismatchMessage creates a detailed error message for mismatches.
func formatMismatchMessage(mismatch *BytesMismatch, decompLen, expLen int) string {
	if mismatch == nil {
		return "verification failed: unable to determine mismatch details"
	}

	msg := fmt.Sprintf("byte mismatch at offset %d: expected 0x%s (%d), got 0x%s (%d)",
		mismatch.Offset,
		mismatch.ExpectedHex, mismatch.ExpectedByte,
		mismatch.ActualHex, mismatch.ActualByte)

	// Add context if available
	if mismatch.ContextBefore > 0 || mismatch.ContextAfter > 0 {
		msg += fmt.Sprintf("\n  expected context: %s", mismatch.ExpectedCtx)
		msg += fmt.Sprintf("\n  actual context:   %s", mismatch.ActualCtx)

		// Show position marker
		marker := bytes.Repeat([]byte("  "), mismatch.ContextBefore)
		marker = append(marker, []byte("^^")...)
		msg += fmt.Sprintf("\n                    %s", string(marker))
	}

	return msg
}

// formatRangeMismatchMessage creates a detailed error message for range mismatches.
func formatRangeMismatchMessage(mismatch *BytesMismatch, rangeStart int64, rangeLen int) string {
	if mismatch == nil {
		return fmt.Sprintf("range verification failed (range: %d-%d, length: %d)",
			rangeStart, rangeStart+int64(rangeLen)-1, rangeLen)
	}

	absoluteOffset := rangeStart + mismatch.Offset
	msg := fmt.Sprintf("range byte mismatch at absolute offset %d (relative offset %d within range %d-%d): expected 0x%s (%d), got 0x%s (%d)",
		absoluteOffset, mismatch.Offset,
		rangeStart, rangeStart+int64(rangeLen)-1,
		mismatch.ExpectedHex, mismatch.ExpectedByte,
		mismatch.ActualHex, mismatch.ActualByte)

	// Add context if available
	if mismatch.ContextBefore > 0 || mismatch.ContextAfter > 0 {
		msg += fmt.Sprintf("\n  expected context: %s", mismatch.ExpectedCtx)
		msg += fmt.Sprintf("\n  actual context:   %s", mismatch.ActualCtx)

		// Show position marker
		marker := bytes.Repeat([]byte("  "), mismatch.ContextBefore)
		marker = append(marker, []byte("^^")...)
		msg += fmt.Sprintf("\n                    %s", string(marker))
	}

	return msg
}

// ByteStats provides statistics about byte differences.
type ByteStats struct {
	TotalBytes      int      // total bytes compared
	MismatchCount   int      // number of mismatching bytes
	MismatchOffsets []int64  // offsets of all mismatches
	MismatchMap     map[byte]int // frequency distribution of mismatching byte values
}

// AnalyzeByteDifferences performs a detailed analysis of byte-level differences.
//
// This function provides forensic-level insight into corruption patterns. Unlike
// VerifyDecompression which stops at the first mismatch, this function scans the entire
// byte stream to build a complete picture of all differences, enabling pattern detection
// and root cause analysis.
//
// **Function Signature:**
//
//	func AnalyzeByteDifferences(decompressed, expected []byte) *ByteStats
//
// **Conceptual Integration Signature (for forensic analysis):**
//
//	// In the context of restoreverifier failure analysis:
//	result := crypto.VerifyDecompression(decompressed, expected)
//	if !result.Passed() {
//	    // Perform full forensic scan
//	    stats := crypto.AnalyzeByteDifferences(decompressed, expected)
//
//	    // Analyze corruption pattern for escalation context
//	    corruptionType := classifyCorruption(stats)
//	    context := fmt.Sprintf("object=%s, corruption=%s, coverage=%.2f%%",
//	        objectID, corruptionType, coverage(stats))
//
//	    escalateResult(ctx, obj, result, stats, context)
//	}
//
// **Parameters:**
//   - decompressed: The decompressed data to analyze (typically the failed output)
//   - expected: The reference data to compare against (the known-good copy)
//
// **Return Type:**
//   - *ByteStats: Contains statistics about byte differences:
//     - TotalBytes: Total bytes in expected data
//     - MismatchCount: Number of mismatching bytes
//     - MismatchOffsets: Offsets of all mismatches (for pattern analysis)
//     - MismatchMap: Frequency distribution of mismatching byte values
//
// **Constraints:**
//   - Both parameters must be non-nil byte slices
//   - No maximum size limit (constrained only by available memory)
//   - Full scan is performed (does not short-circuit)
//   - Can be expensive for large objects - use after VerifyDecompression failure only
//
// **Validation:**
//   - Compares byte-by-byte across the full length of both slices
//   - Tracks every mismatching offset (not just the first one)
//   - Builds frequency map of incorrect byte values
//   - Accounts for length differences in mismatch count
//
// **Error Handling:**
//   - Never panics
//   - Returns empty ByteStats (with zero counts) for nil input
//   - Safe to use even when verification has already failed
//   - Does not return error - incomplete stats indicate input issues
//
// **Usage Example:**
//
//	result := crypto.VerifyDecompression(decompressed, expected)
//	if !result.Passed() {
//	    // Analyze the corruption pattern
//	    stats := crypto.AnalyzeByteDifferences(decompressed, expected)
//
//	    log.Error("Decompression corruption detected",
//	        "mismatches", stats.MismatchCount,
//	        "percentage", float64(stats.MismatchCount)/float64(stats.TotalBytes)*100.0,
//	        "first_mismatch", result.ByteOffset,
//	    )
//
//	    // Check for patterns (e.g., all mismatches are the same byte value)
//	    topMismatches := stats.TopMismatches(5)
//	    for _, m := range topMismatches {
//	        log.Info("Common mismatching byte",
//	            "byte", m.Byte,
//	            "hex", fmt.Sprintf("0x%02X", m.Byte),
//	            "count", m.Count)
//	    }
//	}
//
// **Pattern Detection Examples:**
//
//	// Burst error detection (sequential corrupted bytes)
//	stats := crypto.AnalyzeByteDifferences(decompressed, expected)
//	if isBurstError(stats.MismatchOffsets) {
//	    log.Info("Burst error detected", "start", stats.MismatchOffsets[0],
//	        "end", stats.MismatchOffsets[len(stats.MismatchOffsets)-1])
//	}
//
//	// Zero-fill corruption (all mismatches are 0x00)
//	if len(stats.MismatchMap) == 1 && stats.MismatchMap[0x00] == stats.MismatchCount {
//	    log.Info("Zero-fill corruption detected")
//	}
//
//	// Random corruption (high entropy in mismatched bytes)
//	if highEntropyMismatch(stats.MismatchMap) {
//	    log.Info("Random corruption pattern detected")
//	}
//
// **Integration with VerificationResult:**
//
//	// Combine first-mismatch fast failure with full forensic analysis
//	result := crypto.VerifyDecompression(decompressed, expected)
//	if !result.Passed() {
//	    // Fast path: first mismatch is already in result.ByteOffset
//	    log.Error("First mismatch", "offset", result.ByteOffset)
//
//	    // Deep dive: full forensic scan for pattern analysis
//	    stats := crypto.AnalyzeByteDifferences(decompressed, expected)
//	    analyzeCorruptionPattern(stats)
//	}
//
// **Performance:**
//   - O(n) time complexity where n is the max length of both slices
//   - O(k) additional memory where k is the number of mismatching bytes
//   - Does not short-circuit (always performs full scan)
//   - Recommended: call only after VerifyDecompression failure
//
// **Use Cases:**
//   - Forensic analysis of corruption patterns
//   - Identifying burst errors vs. random corruption
//   - Detecting systematic issues (e.g., certain byte values always corrupted)
//   - Monitoring data integrity over time
//   - Root cause analysis for escalation beads
//   - DR drill vs. normal path comparison anomaly detection
func AnalyzeByteDifferences(decompressed, expected []byte) *ByteStats {
	stats := &ByteStats{
		TotalBytes:    len(expected),
		MismatchMap:   make(map[byte]int),
		MismatchOffsets: make([]int64, 0),
	}

	maxLen := len(decompressed)
	if len(expected) < maxLen {
		maxLen = len(expected)
	}

	for i := 0; i < maxLen; i++ {
		if decompressed[i] != expected[i] {
			stats.MismatchCount++
			stats.MismatchOffsets = append(stats.MismatchOffsets, int64(i))

			// Track which byte values are appearing in the wrong data
			stats.MismatchMap[decompressed[i]]++
		}
	}

	// Account for length differences
	if len(decompressed) != len(expected) {
		stats.MismatchCount += abs(len(decompressed) - len(expected))
	}

	return stats
}

// Summary returns a summary string of the byte statistics.
func (bs *ByteStats) Summary() string {
	if bs.MismatchCount == 0 {
		return fmt.Sprintf("all %d bytes match", bs.TotalBytes)
	}

	percentage := float64(bs.MismatchCount) / float64(bs.TotalBytes) * 100.0
	return fmt.Sprintf("%d/%d bytes mismatch (%.2f%%)",
		bs.MismatchCount, bs.TotalBytes, percentage)
}

// TopMismatches returns the most common mismatching byte values.
func (bs *ByteStats) TopMismatches(n int) []struct {
	Byte  byte
	Count int
} {
	type mismatch struct {
		Byte  byte
		Count int
	}

	mismatches := make([]mismatch, 0, len(bs.MismatchMap))
	for b, count := range bs.MismatchMap {
		mismatches = append(mismatches, mismatch{Byte: b, Count: count})
	}

	// Sort by count (descending)
	for i := 0; i < len(mismatches)-1; i++ {
		for j := i + 1; j < len(mismatches); j++ {
			if mismatches[j].Count > mismatches[i].Count {
				mismatches[i], mismatches[j] = mismatches[j], mismatches[i]
			}
		}
	}

	if n > len(mismatches) {
		n = len(mismatches)
	}

	result := make([]struct {
		Byte  byte
		Count int
	}, n)
	for i := 0; i < n; i++ {
		result[i].Byte = mismatches[i].Byte
		result[i].Count = mismatches[i].Count
	}

	return result
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}