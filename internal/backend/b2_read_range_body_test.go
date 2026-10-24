package backend

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// readRangeBody must read exactly the requested span: no fewer (short read),
// no more (over-long body is a mismatch, not a silent truncation), a
// transport error surfaces as an error, and a negative length fails instead
// of panicking on a negative allocation.

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadRangeBody(t *testing.T) {
	body := "0123456789abcdef"

	t.Run("exact length", func(t *testing.T) {
		got, err := readRangeBody(strings.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatalf("readRangeBody: %v", err)
		}
		if string(got) != body {
			t.Fatalf("got %q, want %q", got, body)
		}
	})

	t.Run("short body", func(t *testing.T) {
		got, err := readRangeBody(strings.NewReader("0123"), 16)
		if err == nil {
			t.Fatalf("short body accepted: %q", got)
		}
		var sre *shortReadError
		if !errors.As(err, &sre) {
			t.Fatalf("got %T (%v), want *shortReadError", err, err)
		}
		if sre.got != 4 || sre.want != 16 {
			t.Fatalf("shortReadError fields: got=%d want=%d", sre.got, sre.want)
		}
	})

	t.Run("over-long body", func(t *testing.T) {
		long := body + "EXTRA"
		got, err := readRangeBody(strings.NewReader(long), 16)
		if err == nil {
			t.Fatalf("over-long body accepted: %q", got)
		}
		var sre *shortReadError
		if !errors.As(err, &sre) {
			t.Fatalf("got %T (%v), want *shortReadError", err, err)
		}
		if sre.got != 17 {
			t.Fatalf("shortReadError.got = %d, want 17 (over-long must be detected)", sre.got)
		}
	})

	t.Run("empty body, zero length", func(t *testing.T) {
		got, err := readRangeBody(strings.NewReader(""), 0)
		if err != nil {
			t.Fatalf("readRangeBody: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d bytes, want 0", len(got))
		}
	})

	t.Run("transport error surfaces", func(t *testing.T) {
		want := errors.New("connection reset")
		_, err := readRangeBody(errReader{err: want}, 16)
		if !errors.Is(err, want) {
			t.Fatalf("got %v, want wrapped %v", err, want)
		}
	})

	t.Run("negative length fails closed", func(t *testing.T) {
		_, err := readRangeBody(strings.NewReader("x"), -1)
		if err == nil {
			t.Fatal("negative expectedLength accepted")
		}
		var sre *shortReadError
		if !errors.As(err, &sre) {
			t.Fatalf("got %T (%v), want *shortReadError", err, err)
		}
	})

	t.Run("1 MiB fetch allocates once", func(t *testing.T) {
		// The regrowth fix: a 1 MiB read through the pre-sized buffer must
		// not grow the result slice (~20 io.ReadAll regrowths historically).
		payload := make([]byte, 1<<20)
		for i := range payload {
			payload[i] = byte(i)
		}
		allocs := testing.AllocsPerRun(100, func() {
			got, err := readRangeBody(bytes.NewReader(payload), 1<<20)
			if err != nil {
				t.Fatalf("readRangeBody: %v", err)
			}
			if len(got) != 1<<20 {
				t.Fatalf("got %d bytes", len(got))
			}
		})
		// Two allocations are inherent: the pre-sized body buffer and this
		// test's own reader escaping into io.ReadFull. The regrowth the
		// pre-sizing removed would show up as ~20 more, one per append
		// doubling of io.ReadAll's result slice.
		if allocs > 2 {
			t.Fatalf("readRangeBody allocated %.0f times for a 1 MiB read, want <= 2", allocs)
		}
	})
}
