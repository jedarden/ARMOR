package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/logging"
)

// fatalConfigFixture builds a joined configuration failure shaped like the
// real one a bare `armor` with a missing environment produces: several
// problems in one errors.Join, at least one of them containing a character
// the JSON form HTML-escapes.
func fatalConfigFixture() error {
	return errors.Join(
		fmt.Errorf("ARMOR_MEK is required"),
		fmt.Errorf("ARMOR_AUTH_SECRET_KEY must be at least 16 characters <see docs/configuration.md>"),
	)
}

// TestFatalConfigErrorPlainTextForTerminal pins the terminal rendering: a
// header, then every problem on its own indented line, with nothing escaped.
// This is the form fatalConfigError prints when stderr is a TTY, where the
// previous single JSON line showed the same problems as backslash-n and
// backslash-u003c escapes.
func TestFatalConfigErrorPlainTextForTerminal(t *testing.T) {
	var buf bytes.Buffer
	writeConfigErrorPlainText(&buf, joinedErrorList(fatalConfigFixture()))
	got := buf.String()

	if !strings.HasPrefix(got, "armor: failed to load configuration:") {
		t.Errorf("terminal form must open with the failure header, got:\n%s", got)
	}
	if !strings.Contains(got, "  - ARMOR_MEK is required\n") {
		t.Errorf("terminal form must list each problem on its own line, got:\n%s", got)
	}
	if !strings.Contains(got, "  - ARMOR_AUTH_SECRET_KEY must be at least 16 characters <see docs/configuration.md>\n") {
		t.Errorf("terminal form must carry the full problem text, got:\n%s", got)
	}
	if strings.Contains(got, "\\u003c") || strings.Contains(got, "\\n") {
		t.Errorf("terminal form must not carry JSON escapes, got:\n%s", got)
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Errorf("terminal form must not be JSON, got:\n%s", got)
	}
	if n := strings.Count(got, "\n  - "); n != 2 {
		t.Errorf("terminal form listed %d problems, want 2, got:\n%s", n, got)
	}
}

// TestFatalConfigErrorJSONForPipelines pins the non-terminal rendering: the
// failure stays one structured JSON log line whose msg carries every problem
// with the two-space indentation and the HTML escapes it always had, so log
// pipelines matching the old output keep matching. Errorf exercises the same
// rendering path as the Fatalf fatalConfigError calls (which exits after
// logging, and therefore cannot run inside a test).
func TestFatalConfigErrorJSONForPipelines(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New("armor")
	logger.SetOutput(&buf)
	logger.Errorf("failed to load configuration:\n%s", configErrorLogMessage(joinedErrorList(fatalConfigFixture())))

	body := buf.String()
	if body == "" || !strings.HasSuffix(body, "\n") {
		t.Fatalf("pipeline form must be one complete log line, got %q", body)
	}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("pipeline form must stay a single JSON line, got %d lines:\n%s", len(lines), body)
	}
	var entry logging.Entry
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("pipeline form is not valid JSON: %v\n%s", err, lines[0])
	}
	if !strings.Contains(entry.Msg, "ARMOR_MEK is required") || !strings.Contains(entry.Msg, "ARMOR_AUTH_SECRET_KEY") {
		t.Errorf("pipeline msg lost a problem: %q", entry.Msg)
	}
	if !strings.Contains(lines[0], "\\u003c") {
		t.Errorf("pipeline form must keep the machine-facing escapes it always had, got:\n%s", lines[0])
	}
}

// TestStderrIsTerminalDrawsTheRenderingLine pins the predicate that chooses
// between the two renderings: pipes and regular files — everything a
// non-interactive caller (init, a log pipeline, `armor 2>file`) sees — are
// not terminals, while a character device is. /dev/null is the reachable
// character device in a test; nothing reads the error written there either
// way, so the terminal form is acceptable, and the check must not classify
// it as a pipe.
func TestStderrIsTerminalDrawsTheRenderingLine(t *testing.T) {
	old := os.Stderr
	t.Cleanup(func() { os.Stderr = old })

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	defer pipeR.Close()
	defer pipeW.Close()
	os.Stderr = pipeW
	if stderrIsTerminal() {
		t.Error("stderrIsTerminal = true for a pipe, want false")
	}

	null, err := os.Open("/dev/null")
	if err != nil {
		t.Skipf("open /dev/null: %v", err)
	}
	defer null.Close()
	os.Stderr = null
	if !stderrIsTerminal() {
		t.Error("stderrIsTerminal = false for a character device, want true")
	}
	os.Stderr = old
}

// TestConfigErrorLogMessageMatchesLegacyFormat holds the joined message to
// the exact bytes the pre-terminal code produced inside its JSON line, so
// the non-TTY output does not drift.
func TestConfigErrorLogMessageMatchesLegacyFormat(t *testing.T) {
	got := configErrorLogMessage([]string{"first problem", "second problem"})
	want := "  first problem\n  second problem"
	if got != want {
		t.Errorf("configErrorLogMessage = %q, want %q", got, want)
	}
}
