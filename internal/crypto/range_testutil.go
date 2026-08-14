// Package crypto provides range request simulation helpers for testing.
package crypto

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// RangeSpec represents a parsed HTTP Range request specification.
type RangeSpec struct {
	Start int64 // Start byte offset (inclusive)
	End   int64 // End byte offset (inclusive), or -1 for open-ended ranges
}

// ParseRangeSpec parses a Range header value like "bytes=0-1023" or "bytes=512-".
func ParseRangeSpec(header string, totalSize int64) (*RangeSpec, error) {
	if !strings.HasPrefix(header, "bytes=") {
		return nil, fmt.Errorf("invalid range format: must start with 'bytes='")
	}

	rangeSpec := strings.TrimPrefix(header, "bytes=")

	if strings.Contains(rangeSpec, ",") {
		return nil, fmt.Errorf("multiple ranges not supported")
	}

	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format: expected 'start-end' or 'start-' or '-suffix'")
	}

	spec := &RangeSpec{}

	if parts[0] == "" {
		// Suffix range: -500 means last 500 bytes
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid suffix range: %w", err)
		}
		if suffix < 0 {
			return nil, fmt.Errorf("suffix cannot be negative")
		}
		if suffix > totalSize {
			return nil, fmt.Errorf("suffix %d exceeds file size %d", suffix, totalSize)
		}
		spec.Start = totalSize - suffix
		spec.End = totalSize - 1
	} else {
		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid start offset: %w", err)
		}
		if start < 0 {
			return nil, fmt.Errorf("start cannot be negative")
		}
		spec.Start = start

		if parts[1] == "" {
			// Open-ended range: bytes=512- means from 512 to end
			spec.End = -1
		} else {
			end, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid end offset: %w", err)
			}
			if end < 0 {
				return nil, fmt.Errorf("end cannot be negative")
			}
			spec.End = end
		}
	}

	// Validate range bounds
	if spec.Start >= totalSize {
		return nil, fmt.Errorf("start offset %d exceeds file size %d", spec.Start, totalSize)
	}

	if spec.End != -1 && spec.End >= totalSize {
		// Clamp end per RFC 7233: ranges ending beyond file size are satisfiable
		// as long as start is within bounds
		spec.End = totalSize - 1
	}

	if spec.End != -1 && spec.End < spec.Start {
		return nil, fmt.Errorf("end %d is before start %d", spec.End, spec.Start)
	}

	return spec, nil
}

// ResolveRange resolves an open-ended range (where End == -1) to the actual end position.
func (r *RangeSpec) ResolveRange(totalSize int64) (start, end int64) {
	start = r.Start
	if r.End == -1 {
		end = totalSize - 1
	} else {
		end = r.End
	}
	return
}

// String returns the canonical string representation of the range spec.
func (r *RangeSpec) String() string {
	if r.End == -1 {
		return fmt.Sprintf("bytes=%d-", r.Start)
	}
	return fmt.Sprintf("bytes=%d-%d", r.Start, r.End)
}

// ContentRange returns the Content-Range header value for this range.
func (r *RangeSpec) ContentRange(totalSize int64) string {
	start, end := r.ResolveRange(totalSize)
	return fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize)
}

// Length returns the length of the range in bytes.
func (r *RangeSpec) Length(totalSize int64) int64 {
	start, end := r.ResolveRange(totalSize)
	return end - start + 1
}

// ExtractRange extracts the specified byte range from data.
func ExtractRange(data []byte, spec *RangeSpec) ([]byte, error) {
	totalSize := int64(len(data))
	if spec.Start >= totalSize {
		return nil, fmt.Errorf("start offset %d exceeds data size %d", spec.Start, totalSize)
	}

	start, end := spec.ResolveRange(totalSize)
	if end >= totalSize {
		return nil, fmt.Errorf("end offset %d exceeds data size %d", end, totalSize-1)
	}

	return data[start : end+1], nil
}

// RangeSimulator provides utilities for simulating range requests on test data.
type RangeSimulator struct {
	data       []byte
	totalSize  int64
	compressed bool
	blockSize  int
}

// NewRangeSimulator creates a new RangeSimulator for the given data.
func NewRangeSimulator(data []byte, compressed bool, blockSize int) *RangeSimulator {
	return &RangeSimulator{
		data:       data,
		totalSize:  int64(len(data)),
		compressed: compressed,
		blockSize:  blockSize,
	}
}

// SimulateRangeRequest simulates an HTTP range request and returns the partial content.
func (rs *RangeSimulator) SimulateRangeRequest(rangeHeader string) (*RangeResult, error) {
	spec, err := ParseRangeSpec(rangeHeader, rs.totalSize)
	if err != nil {
		return nil, err
	}

	partialData, err := ExtractRange(rs.data, spec)
	if err != nil {
		return nil, err
	}

	return &RangeResult{
		Spec:         spec,
		Data:         partialData,
		ContentRange: spec.ContentRange(rs.totalSize),
		ContentLength: spec.Length(rs.totalSize),
		TotalSize:    rs.totalSize,
	}, nil
}

// RangeResult represents the result of a range request simulation.
type RangeResult struct {
	Spec          *RangeSpec
	Data          []byte
	ContentRange  string
	ContentLength int64
	TotalSize     int64
}

// Verify verifies that the range result matches expectations.
func (rr *RangeResult) Verify(expectedData []byte) error {
	if !bytes.Equal(rr.Data, expectedData) {
		return fmt.Errorf("range data mismatch: got %d bytes, expected %d bytes", len(rr.Data), len(expectedData))
	}
	if rr.ContentLength != int64(len(expectedData)) {
		return fmt.Errorf("content-length mismatch: got %d, expected %d", rr.ContentLength, len(expectedData))
	}
	return nil
}

// ParseContentRange parses a Content-Range header value like "bytes 0-1023/2048".
func ParseContentRange(header string) (start, end, total int64, err error) {
	// Format: "bytes start-end/total"
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "bytes" {
		return 0, 0, 0, fmt.Errorf("invalid content-range format: missing 'bytes' prefix")
	}

	rangeParts := strings.Split(parts[1], "-")
	if len(rangeParts) != 2 {
		return 0, 0, 0, fmt.Errorf("invalid content-range format: missing dash in range")
	}

	start, err = strconv.ParseInt(rangeParts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid start offset: %w", err)
	}

	totalParts := strings.Split(rangeParts[1], "/")
	if len(totalParts) != 2 {
		return 0, 0, 0, fmt.Errorf("invalid content-range format: missing total size")
	}

	end, err = strconv.ParseInt(totalParts[0], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid end offset: %w", err)
	}

	total, err = strconv.ParseInt(totalParts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid total size: %w", err)
	}

	return start, end, total, nil
}

// ReadAllFromRange reads all data from a ReadCloser, handling potential range reads.
func ReadAllFromRange(r io.ReadCloser) ([]byte, error) {
	defer r.Close()
	return io.ReadAll(r)
}

// CommonRangeSpecs returns a list of common range specifications for testing.
// The returned specs are guaranteed to be valid for the given data size.
func CommonRangeSpecs(dataSize int64) []string {
	if dataSize == 0 {
		return []string{}
	}

	suffixSize := int64(500)
	if suffixSize > dataSize {
		suffixSize = dataSize
	}

	midPoint := dataSize / 2
	if midPoint == 0 {
		midPoint = 1
	}

	// Generate specs that are valid for this data size
	specs := []string{
		// Single byte at start
		"bytes=0-0",
		// Single byte at end
		fmt.Sprintf("bytes=%d-%d", dataSize-1, dataSize-1),
		// Last N bytes (suffix range)
		fmt.Sprintf("bytes=-%d", suffixSize),
		// Middle range
		fmt.Sprintf("bytes=%d-%d", midPoint/2, midPoint),
	}

	// Add more specs if data size is large enough
	if dataSize > 100 {
		specs = append(specs, fmt.Sprintf("bytes=%d-%d", dataSize-100, dataSize-1))
	}

	if dataSize > 1024 {
		specs = append(specs, "bytes=0-1023")
	}

	if dataSize > 512 {
		specs = append(specs, "bytes=512-")
	}

	return specs
}
