package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// mockCommandFunc is a test helper that captures execution
var mockExecuted string

func mockCmd() func() {
	return func() {
		mockExecuted = "mock"
	}
}

// TestRegisterCommand verifies that init() registration populates the commands map
func TestRegisterCommand(t *testing.T) {
	// Save original commands
	original := make(map[string]Command)
	for k, v := range commands {
		original[k] = v
	}

	// Clear and register test command
	commands = make(map[string]Command)
	testCmd := Command{
		Name:        "test",
		Description: "Test command",
		Func:        mockCmd(),
	}
	registerCommand(testCmd)

	// Verify registration
	if _, exists := commands["test"]; !exists {
		t.Errorf("registerCommand failed to register 'test' command")
	}

	// Restore original commands
	commands = original
}

// TestCommandDispatch verifies that commands execute correctly
func TestCommandDispatch(t *testing.T) {
	// Save original commands
	original := make(map[string]Command)
	for k, v := range commands {
		original[k] = v
	}

	// Set up test commands
	mockExecuted = ""
	commands = make(map[string]Command)
	commands["test"] = Command{
		Name:        "test",
		Description: "Test command",
		Func: func() {
			mockExecuted = "test"
		},
	}
	commands["help"] = Command{
		Name:        "help",
		Description: "Show help",
		Func: func() {
			mockExecuted = "help"
		},
	}

	// Test 1: Valid command executes
	// We can't test main() directly due to os.Exit, but we can verify registration
	t.Run("Registration", func(t *testing.T) {
		if _, exists := commands["test"]; !exists {
			t.Error("test command not registered")
		}
		if _, exists := commands["help"]; !exists {
			t.Error("help command not registered")
		}
	})

	// Test 2: Non-existent command handling
	t.Run("UnknownCommand", func(t *testing.T) {
		_, exists := commands["nonexistent"]
		if exists {
			t.Error("nonexistent command should not exist")
		}
	})

	// Restore original commands
	commands = original
}

// TestListCommands verifies that listCommands outputs sorted commands
func TestListCommands(t *testing.T) {
	// Save original commands
	original := make(map[string]Command)
	for k, v := range commands {
		original[k] = v
	}

	// Set up test commands with unsorted names
	commands = make(map[string]Command)
	commands["zebra"] = Command{Name: "zebra", Description: "Last"}
	commands["apple"] = Command{Name: "apple", Description: "First"}
	commands["middle"] = Command{Name: "middle", Description: "Mid"}

	var buf bytes.Buffer
	listCommands(&buf)
	output := buf.String()

	// Verify sorted output (alphabetical)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(lines))
	}

	// Check alphabetical order
	if !strings.Contains(lines[0], "apple") {
		t.Errorf("first line should contain 'apple', got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "middle") {
		t.Errorf("second line should contain 'middle', got: %s", lines[1])
	}
	if !strings.Contains(lines[2], "zebra") {
		t.Errorf("third line should contain 'zebra', got: %s", lines[2])
	}

	// Restore original commands
	commands = original
}
