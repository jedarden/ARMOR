package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		level   Level
		expect  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expect {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expect)
		}
	}
}

func TestLoggerBasicLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test-service")
	logger.SetOutput(&buf)
	logger.SetLevel(LevelDebug)

	logger.Info("test message")

	if buf.Len() == 0 {
		t.Error("expected output, got empty buffer")
	}

	// Verify JSON format
	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if entry["msg"] != "test message" {
		t.Errorf("expected msg='test message', got %v", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level='INFO', got %v", entry["level"])
	}
	if entry["service"] != "test-service" {
		t.Errorf("expected service='test-service', got %v", entry["service"])
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test")
	logger.SetOutput(&buf)
	logger.SetLevel(LevelWarn) // Only Warn and Error should be logged

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %s", len(lines), buf.String())
	}
}

func TestLoggerWithField(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test")
	logger.SetOutput(&buf)
	logger.SetLevel(LevelDebug)

	logger.WithField("key", "value").Info("test")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	fields, ok := entry["Fields"].(map[string]interface{})
	if !ok {
		t.Fatal("expected Fields to be a map")
	}
	if fields["key"] != "value" {
		t.Errorf("expected Fields.key='value', got %v", fields["key"])
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test")
	logger.SetOutput(&buf)
	logger.SetLevel(LevelDebug)

	logger.WithFields(map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}).Info("test")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	fields, ok := entry["Fields"].(map[string]interface{})
	if !ok {
		t.Fatal("expected Fields to be a map")
	}
	if fields["key1"] != "value1" {
		t.Errorf("expected Fields.key1='value1', got %v", fields["key1"])
	}
	if fields["key2"] != 123.0 { // JSON numbers are float64
		t.Errorf("expected Fields.key2=123, got %v", fields["key2"])
	}
}

func TestLoggerFormattedMessages(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test")
	logger.SetOutput(&buf)
	logger.SetLevel(LevelDebug)

	logger.Infof("test %s: %d", "value", 42)

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if entry["msg"] != "test value: 42" {
		t.Errorf("expected formatted message, got %v", entry["msg"])
	}
}

func TestLoggerChainedFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New("test")
	logger.SetOutput(&buf)
	logger.SetLevel(LevelDebug)

	logger.WithField("first", 1).WithField("second", 2).Info("test")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	fields, _ := entry["Fields"].(map[string]interface{})
	if fields["first"] != 1.0 || fields["second"] != 2.0 {
		t.Errorf("expected chained fields, got %v", fields)
	}
}

func TestDefaultLogger(t *testing.T) {
	// Test that the default logger is functional
	def := Default()
	if def == nil {
		t.Fatal("Default() returned nil")
	}

	// Create a new logger and set as default
	newLogger := New("new-default")
	SetDefault(newLogger)

	if Default() != newLogger {
		t.Error("SetDefault did not update default logger")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	// Test that package-level functions don't panic
	// (they may not produce output if no output is set)
	Info("test")
	Infof("test %s", "value")
	Warn("test")
	Warnf("test %s", "value")
	Error("test")
	Errorf("test %s", "value")
	Debug("test")
	Debugf("test %s", "value")

	// Test WithField and WithFields
	l := WithField("key", "value")
	if l == nil {
		t.Error("WithField returned nil")
	}

	l = WithFields(map[string]interface{}{"key": "value"})
	if l == nil {
		t.Error("WithFields returned nil")
	}
}
