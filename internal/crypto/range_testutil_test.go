package crypto

import (
	"bytes"
	"testing"
)

// TestParseRangeSpec tests parsing various HTTP Range header specifications.
func TestParseRangeSpec(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		totalSize   int64
		wantStart   int64
		wantEnd     int64
		expectError bool
	}{
		// Valid ranges
		{
			name:      "standard range",
			header:    "bytes=0-1023",
			totalSize: 2048,
			wantStart: 0,
			wantEnd:   1023,
		},
		{
			name:      "open-ended range",
			header:    "bytes=512-",
			totalSize: 2048,
			wantStart: 512,
			wantEnd:   -1, // -1 indicates open-ended
		},
		{
			name:      "suffix range (last 500 bytes)",
			header:    "bytes=-500",
			totalSize: 1000,
			wantStart: 500,
			wantEnd:   999,
		},
		{
			name:      "middle range",
			header:    "bytes=256-767",
			totalSize: 1024,
			wantStart: 256,
			wantEnd:   767,
		},
		{
			name:      "single byte at start",
			header:    "bytes=0-0",
			totalSize: 100,
			wantStart: 0,
			wantEnd:   0,
		},
		{
			name:      "single byte at end",
			header:    "bytes=99-99",
			totalSize: 100,
			wantStart: 99,
			wantEnd:   99,
		},
		{
			name:      "range clamping (end beyond file size)",
			header:    "bytes=900-9999",
			totalSize: 1000,
			wantStart: 900,
			wantEnd:   999, // Clamped to file size - 1
		},
		{
			name:      "exact file size",
			header:    "bytes=0-999",
			totalSize: 1000,
			wantStart: 0,
			wantEnd:   999,
		},

		// Invalid ranges
		{
			name:        "missing bytes prefix",
			header:      "0-1023",
			totalSize:   2048,
			expectError: true,
		},
		{
			name:        "multiple ranges",
			header:      "bytes=0-1023,2048-3071",
			totalSize:   4096,
			expectError: true,
		},
		{
			name:        "start beyond file size",
			header:      "bytes=2000-2999",
			totalSize:   1000,
			expectError: true,
		},
		{
			name:        "end before start",
			header:      "bytes=500-400",
			totalSize:   1000,
			expectError: true,
		},
		{
			name:        "negative start",
			header:      "bytes=-100-500",
			totalSize:   1000,
			expectError: true,
		},
		{
			name:        "empty range spec",
			header:      "bytes=",
			totalSize:   1000,
			expectError: true,
		},
		{
			name:        "invalid start characters",
			header:      "bytes=abc-500",
			totalSize:   1000,
			expectError: true,
		},
		{
			name:        "suffix larger than file",
			header:      "bytes=-2000",
			totalSize:   1000,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := ParseRangeSpec(tt.header, tt.totalSize)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseRangeSpec() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseRangeSpec() unexpected error: %v", err)
			}

			if spec.Start != tt.wantStart {
				t.Errorf("Start = %d, want %d", spec.Start, tt.wantStart)
			}
			if spec.End != tt.wantEnd {
				t.Errorf("End = %d, want %d", spec.End, tt.wantEnd)
			}
		})
	}
}

// TestRangeSpecString tests the string representation of range specs.
func TestRangeSpecString(t *testing.T) {
	tests := []struct {
		spec     *RangeSpec
		expected string
	}{
		{&RangeSpec{Start: 0, End: 1023}, "bytes=0-1023"},
		{&RangeSpec{Start: 512, End: -1}, "bytes=512-"},
		{&RangeSpec{Start: 100, End: 200}, "bytes=100-200"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.spec.String(); got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

// TestRangeSpecContentRange tests Content-Range header generation.
func TestRangeSpecContentRange(t *testing.T) {
	tests := []struct {
		spec       *RangeSpec
		totalSize  int64
		expected   string
	}{
		{&RangeSpec{Start: 0, End: 1023}, 2048, "bytes 0-1023/2048"},
		{&RangeSpec{Start: 512, End: -1}, 1024, "bytes 512-1023/1024"},
		{&RangeSpec{Start: 100, End: 200}, 500, "bytes 100-200/500"},
		{&RangeSpec{Start: 0, End: 999}, 1000, "bytes 0-999/1000"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.spec.ContentRange(tt.totalSize); got != tt.expected {
				t.Errorf("ContentRange() = %s, want %s", got, tt.expected)
			}
		})
	}
}

// TestRangeSpecLength tests range length calculation.
func TestRangeSpecLength(t *testing.T) {
	tests := []struct {
		spec       *RangeSpec
		totalSize  int64
		expected   int64
	}{
		{&RangeSpec{Start: 0, End: 1023}, 2048, 1024},
		{&RangeSpec{Start: 512, End: -1}, 1024, 512}, // 1024 - 512 = 512
		{&RangeSpec{Start: 100, End: 200}, 500, 101},  // 200 - 100 + 1 = 101
		{&RangeSpec{Start: 0, End: 0}, 100, 1},         // single byte
	}

	for _, tt := range tests {
		t.Run(tt.spec.String(), func(t *testing.T) {
			if got := tt.spec.Length(tt.totalSize); got != tt.expected {
				t.Errorf("Length() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestExtractRange tests extracting byte ranges from data.
func TestExtractRange(t *testing.T) {
	data := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	tests := []struct {
		name          string
		spec          *RangeSpec
		expectedData  []byte
		expectError   bool
	}{
		{
			name:         "first 10 bytes",
			spec:         &RangeSpec{Start: 0, End: 9},
			expectedData: []byte("0123456789"),
		},
		{
			name:         "middle range",
			spec:         &RangeSpec{Start: 10, End: 19},
			expectedData: []byte("ABCDEFGHIJ"),
		},
		{
			name:         "last 5 bytes",
			spec:         &RangeSpec{Start: 31, End: 35},
			expectedData: []byte("VWXYZ"),
		},
		{
			name:         "single byte at start",
			spec:         &RangeSpec{Start: 0, End: 0},
			expectedData: []byte("0"),
		},
		{
			name:         "single byte at end",
			spec:         &RangeSpec{Start: 35, End: 35},
			expectedData: []byte("Z"),
		},
		{
			name:         "single byte in middle",
			spec:         &RangeSpec{Start: 15, End: 15},
			expectedData: []byte("F"),
		},
		{
			name:        "start beyond data",
			spec:        &RangeSpec{Start: 100, End: 110},
			expectError: true,
		},
		{
			name:        "end beyond data",
			spec:        &RangeSpec{Start: 30, End: 100},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractRange(data, tt.spec)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExtractRange() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractRange() unexpected error: %v", err)
			}

			if !bytes.Equal(result, tt.expectedData) {
				t.Errorf("ExtractRange() = %s, want %s", string(result), string(tt.expectedData))
			}
		})
	}
}

// TestRangeSimulator tests the full range request simulation.
func TestRangeSimulator(t *testing.T) {
	// Create test data - "Hello, World!" repeated
	testData := []byte("Hello, World! Hello, World! Hello, World!")
	simulator := NewRangeSimulator(testData, false, 65536)

	tests := []struct {
		name         string
		rangeHeader  string
		expectedData []byte
		expectError  bool
	}{
		{
			name:         "first 13 bytes",
			rangeHeader:  "bytes=0-12",
			expectedData: []byte("Hello, World!"),
		},
		{
			name:         "middle range",
			rangeHeader:  "bytes=13-25",
			expectedData: []byte(" Hello, World"),
		},
		{
			name:         "open-ended range from position 14",
			rangeHeader:  "bytes=14-",
			expectedData: []byte("Hello, World! Hello, World!"),
		},
		{
			name:         "suffix range - last 13 bytes",
			rangeHeader:  "bytes=-13",
			expectedData: []byte("Hello, World!"),
		},
		{
			name:         "entire file",
			rangeHeader:  "bytes=0-41",
			expectedData: testData,
		},
		{
			name:        "invalid range format",
			rangeHeader: "invalid",
			expectError: true,
		},
		{
			name:        "range beyond file",
			rangeHeader: "bytes=100-200",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := simulator.SimulateRangeRequest(tt.rangeHeader)

			if tt.expectError {
				if err == nil {
					t.Errorf("SimulateRangeRequest() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("SimulateRangeRequest() unexpected error: %v", err)
			}

			// Verify the result matches expectations
			if !bytes.Equal(result.Data, tt.expectedData) {
				t.Errorf("Data = %s, want %s", string(result.Data), string(tt.expectedData))
			}

			// Verify Content-Length matches data length
			if result.ContentLength != int64(len(tt.expectedData)) {
				t.Errorf("ContentLength = %d, want %d", result.ContentLength, len(tt.expectedData))
			}

			// Verify Content-Range format
			if result.ContentRange == "" {
				t.Error("ContentRange should not be empty")
			}

			// Verify TotalSize
			if result.TotalSize != int64(len(testData)) {
				t.Errorf("TotalSize = %d, want %d", result.TotalSize, len(testData))
			}
		})
	}
}

// TestRangeResultVerify tests the verification method.
func TestRangeResultVerify(t *testing.T) {
	testData := []byte("Hello, World!")
	simulator := NewRangeSimulator(testData, false, 65536)

	result, err := simulator.SimulateRangeRequest("bytes=0-12")
	if err != nil {
		t.Fatalf("Failed to create range result: %v", err)
	}

	// Verify with correct data
	if err := result.Verify([]byte("Hello, World!")); err != nil {
		t.Errorf("Verify() with correct data failed: %v", err)
	}

	// Verify with incorrect data
	if err := result.Verify([]byte("Wrong data")); err == nil {
		t.Error("Verify() with incorrect data should fail")
	}
}

// TestParseContentRange tests parsing Content-Range headers.
func TestParseContentRange(t *testing.T) {
	tests := []struct {
		header       string
		wantStart    int64
		wantEnd      int64
		wantTotal    int64
		expectError  bool
	}{
		{"bytes 0-1023/2048", 0, 1023, 2048, false},
		{"bytes 512-1023/2048", 512, 1023, 2048, false},
		{"bytes 0-0/100", 0, 0, 100, false},
		{"invalid format", 0, 0, 0, true},
		{"missing prefix 0-1023/2048", 0, 0, 0, true},
		{"bytes 0-1023", 0, 0, 0, true}, // missing total
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			start, end, total, err := ParseContentRange(tt.header)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseContentRange() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseContentRange() unexpected error: %v", err)
			}

			if start != tt.wantStart {
				t.Errorf("Start = %d, want %d", start, tt.wantStart)
			}
			if end != tt.wantEnd {
				t.Errorf("End = %d, want %d", end, tt.wantEnd)
			}
			if total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

// TestRangeSimulatorCompressedData tests range simulation with compressed data.
func TestRangeSimulatorCompressedData(t *testing.T) {
	// Create test data that looks like compressed data (has zstd magic bytes)
	compressedData := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x01, 0x00, 0x00, 0x00}
	compressedData = append(compressedData, []byte("additional compressed payload...")...)

	simulator := NewRangeSimulator(compressedData, true, 65536)

	// Test that we can extract ranges from compressed data
	result, err := simulator.SimulateRangeRequest("bytes=0-7")
	if err != nil {
		t.Fatalf("SimulateRangeRequest() on compressed data failed: %v", err)
	}

	if !bytes.Equal(result.Data, compressedData[0:8]) {
		t.Errorf("Compressed data range extraction failed")
	}

	if !simulator.compressed {
		t.Error("Simulator should be marked as compressed")
	}
}

// TestCommonRangeSpecs tests the helper function that generates common range specs.
func TestCommonRangeSpecs(t *testing.T) {
	dataSize := int64(2048)
	specs := CommonRangeSpecs(dataSize)

	if len(specs) == 0 {
		t.Error("CommonRangeSpecs() returned empty list")
	}

	// Verify each spec is valid
	for _, spec := range specs {
		_, err := ParseRangeSpec(spec, dataSize)
		if err != nil {
			t.Errorf("Invalid common range spec '%s': %v", spec, err)
		}
	}

	// Test with very small data size
	smallSize := int64(100)
	smallSpecs := CommonRangeSpecs(smallSize)

	for _, spec := range smallSpecs {
		_, err := ParseRangeSpec(spec, smallSize)
		if err != nil {
			t.Errorf("Invalid small data range spec '%s': %v", spec, err)
		}
	}
}

// TestRangeSpecEdgeCases tests edge cases and boundary conditions.
func TestRangeSpecEdgeCases(t *testing.T) {
	t.Run("resolve range with -1 end", func(t *testing.T) {
		spec := &RangeSpec{Start: 100, End: -1}
		start, end := spec.ResolveRange(1000)

		if start != 100 {
			t.Errorf("Start = %d, want 100", start)
		}
		if end != 999 {
			t.Errorf("End = %d, want 999", end)
		}
	})

	t.Run("empty data range", func(t *testing.T) {
		data := []byte("test")
		spec := &RangeSpec{Start: 0, End: 3}
		result, err := ExtractRange(data, spec)

		if err != nil {
			t.Fatalf("ExtractRange() failed: %v", err)
		}
		if !bytes.Equal(result, data) {
			t.Errorf("ExtractRange() = %v, want %v", result, data)
		}
	})

	t.Run("large range spec validation", func(t *testing.T) {
		largeSize := int64(1 << 30) // 1GB
		spec := &RangeSpec{Start: 0, End: largeSize - 1}

		if spec.Start != 0 {
			t.Errorf("Large range start failed")
		}
		if spec.End != largeSize-1 {
			t.Errorf("Large range end failed")
		}
	})
}

// TestRangeSimulatorErrors tests error conditions in range simulation.
func TestRangeSimulatorErrors(t *testing.T) {
	testData := []byte("test data")
	simulator := NewRangeSimulator(testData, false, 65536)

	errorTests := []struct {
		name        string
		rangeHeader string
	}{
		{"missing equals", "bytes0-10"},
		{"multiple ranges", "bytes=0-10,20-30"},
		{"invalid characters", "bytes=abc-def"},
		{"negative suffix", "bytes=--100"},
		{"empty spec", "bytes=-"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := simulator.SimulateRangeRequest(tt.rangeHeader)
			if err == nil {
				t.Errorf("SimulateRangeRequest('%s') should return error", tt.rangeHeader)
			}
		})
	}
}

// TestRangeRoundTrip tests that range specs round-trip correctly.
func TestRangeRoundTrip(t *testing.T) {
	originalHeaders := []string{
		"bytes=0-1023",
		"bytes=512-",
		"bytes=-500",
		"bytes=100-200",
	}

	for _, header := range originalHeaders {
		spec, err := ParseRangeSpec(header, 2048)
		if err != nil {
			t.Fatalf("ParseRangeSpec(%s) failed: %v", header, err)
		}

		// Convert back to string
		reconstructed := spec.String()

		// For open-ended ranges, the reconstructed string might differ slightly
		// from the original, but should still be valid
		spec2, err := ParseRangeSpec(reconstructed, 2048)
		if err != nil {
			t.Fatalf("ParseRangeSpec(%s) failed on reconstructed: %v", reconstructed, err)
		}

		if spec.Start != spec2.Start {
			t.Errorf("Start mismatch: %d vs %d", spec.Start, spec2.Start)
		}
		if spec.End != spec2.End {
			t.Errorf("End mismatch: %d vs %d", spec.End, spec2.End)
		}
	}
}
