// Integration tests for valid_nested.yaml
package yamlutil

import (
	"os"
	"testing"
)

// TestValidNestedYAML_Integration tests parsing valid_nested.yaml from internal/yamlutil/testdata/
// and verifies nested structures and lists are correctly extracted with actual values.
func TestValidNestedYAML_Integration(t *testing.T) {
	testFile := "testdata/valid_nested.yaml"

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("test file %s does not exist", testFile)
	}

	// Read file content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("test file is empty")
	}

	// Parse YAML content
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify top-level sections exist
	requiredSections := []string{"server", "database", "logging"}
	for _, section := range requiredSections {
		if _, exists := data[section]; !exists {
			t.Errorf("expected section '%s' to exist in parsed data", section)
		}
	}

	// Verify server section structure
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'server' to be a map")
	}

	// Verify server.host
	if host, ok := server["host"].(string); !ok {
		t.Error("expected 'server.host' to be a string")
	} else if host != "localhost" {
		t.Errorf("expected server.host='localhost', got %q", host)
	}

	// Verify server.port
	port := server["port"]
	switch p := port.(type) {
	case int:
		if p != 8080 {
			t.Errorf("expected server.port=8080, got %d", p)
		}
	case int64:
		if p != 8080 {
			t.Errorf("expected server.port=8080, got %d", p)
		}
	default:
		t.Errorf("expected server.port to be int or int64, got %T with value %v", p, p)
	}

	// Verify server.ssl nested structure
	ssl, ok := server["ssl"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'server.ssl' to be a map")
	}

	// Verify server.ssl.enabled
	if enabled, ok := ssl["enabled"].(bool); !ok {
		t.Error("expected 'server.ssl.enabled' to be a boolean")
	} else if !enabled {
		t.Errorf("expected server.ssl.enabled=true, got %v", enabled)
	}

	// Verify server.ssl.certificate
	if cert, ok := ssl["certificate"].(string); !ok {
		t.Error("expected 'server.ssl.certificate' to be a string")
	} else if cert != "/path/to/cert.pem" {
		t.Errorf("expected server.ssl.certificate='/path/to/cert.pem', got %q", cert)
	}

	// Verify server.ssl.key
	if key, ok := ssl["key"].(string); !ok {
		t.Error("expected 'server.ssl.key' to be a string")
	} else if key != "/path/to/key.pem" {
		t.Errorf("expected server.ssl.key='/path/to/key.pem', got %q", key)
	}

	// Verify database section structure
	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'database' to be a map")
	}

	// Verify database.primary nested structure
	primary, ok := database["primary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'database.primary' to be a map")
	}

	// Verify database.primary.host
	if host, ok := primary["host"].(string); !ok {
		t.Error("expected 'database.primary.host' to be a string")
	} else if host != "db1.example.com" {
		t.Errorf("expected database.primary.host='db1.example.com', got %q", host)
	}

	// Verify database.primary.port
	port = primary["port"]
	switch p := port.(type) {
	case int:
		if p != 5432 {
			t.Errorf("expected database.primary.port=5432, got %d", p)
		}
	case int64:
		if p != 5432 {
			t.Errorf("expected database.primary.port=5432, got %d", p)
		}
	default:
		t.Errorf("expected database.primary.port to be int or int64, got %T with value %v", p, p)
	}

	// Verify database.primary.name
	if name, ok := primary["name"].(string); !ok {
		t.Error("expected 'database.primary.name' to be a string")
	} else if name != "production" {
		t.Errorf("expected database.primary.name='production', got %q", name)
	}

	// Verify database.replica nested structure
	replica, ok := database["replica"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'database.replica' to be a map")
	}

	// Verify database.replica.host
	if host, ok := replica["host"].(string); !ok {
		t.Error("expected 'database.replica.host' to be a string")
	} else if host != "db2.example.com" {
		t.Errorf("expected database.replica.host='db2.example.com', got %q", host)
	}

	// Verify database.replica.port
	port = replica["port"]
	switch p := port.(type) {
	case int:
		if p != 5432 {
			t.Errorf("expected database.replica.port=5432, got %d", p)
		}
	case int64:
		if p != 5432 {
			t.Errorf("expected database.replica.port=5432, got %d", p)
		}
	default:
		t.Errorf("expected database.replica.port to be int or int64, got %T with value %v", p, p)
	}

	// Verify database.replica.name
	if name, ok := replica["name"].(string); !ok {
		t.Error("expected 'database.replica.name' to be a string")
	} else if name != "production_replica" {
		t.Errorf("expected database.replica.name='production_replica', got %q", name)
	}

	// Verify logging section structure
	logging, ok := data["logging"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'logging' to be a map")
	}

	// Verify logging.level
	if level, ok := logging["level"].(string); !ok {
		t.Error("expected 'logging.level' to be a string")
	} else if level != "debug" {
		t.Errorf("expected logging.level='debug', got %q", level)
	}

	// Verify logging.format
	if format, ok := logging["format"].(string); !ok {
		t.Error("expected 'logging.format' to be a string")
	} else if format != "json" {
		t.Errorf("expected logging.format='json', got %q", format)
	}

	// Verify logging.output is a list
	output, ok := logging["output"].([]interface{})
	if !ok {
		t.Fatal("expected 'logging.output' to be a list")
	}

	// Verify logging.output has exactly 2 items
	if len(output) != 2 {
		t.Errorf("expected logging.output to have 2 items, got %d", len(output))
	}

	// Verify first output item
	if firstOutput, ok := output[0].(string); !ok {
		t.Error("expected first logging.output item to be a string")
	} else if firstOutput != "stdout" {
		t.Errorf("expected first logging.output='stdout', got %q", firstOutput)
	}

	// Verify second output item
	if secondOutput, ok := output[1].(string); !ok {
		t.Error("expected second logging.output item to be a string")
	} else if secondOutput != "/var/log/app.log" {
		t.Errorf("expected second logging.output='/var/log/app.log', got %q", secondOutput)
	}

	// Verify no unexpected top-level keys
	expectedTopLevelKeys := map[string]bool{
		"server":   true,
		"database": true,
		"logging":  true,
	}

	for key := range data {
		if !expectedTopLevelKeys[key] {
			t.Errorf("unexpected top-level key '%s' in parsed data", key)
		}
	}

	// Verify all expected top-level keys are present
	for key := range expectedTopLevelKeys {
		if _, exists := data[key]; !exists {
			t.Errorf("expected top-level key '%s' missing from parsed data", key)
		}
	}
}

// TestValidNestedYAML_ParseFile tests parsing valid_nested.yaml using ParseFile method.
func TestValidNestedYAML_ParseFile(t *testing.T) {
	testFile := "testdata/valid_nested.yaml"
	parser := NewParser()

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("ParseFile failed: %v", result.Error)
	}

	// Verify top-level sections exist
	requiredSections := []string{"server", "database", "logging"}
	for _, section := range requiredSections {
		if _, exists := data[section]; !exists {
			t.Errorf("expected section '%s' to exist in parsed data", section)
		}
	}

	// Verify server.ssl.enabled
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server to be a map")
	}
	ssl, ok := server["ssl"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server.ssl to be a map")
	}
	if ssl["enabled"] != true {
		t.Errorf("expected server.ssl.enabled=true, got %v", ssl["enabled"])
	}

	// Verify database.primary.host
	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database to be a map")
	}
	primary, ok := database["primary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database.primary to be a map")
	}
	if primary["host"] != "db1.example.com" {
		t.Errorf("expected database.primary.host='db1.example.com', got %v", primary["host"])
	}

	// Verify logging.output list
	logging, ok := data["logging"].(map[string]interface{})
	if !ok {
		t.Fatal("expected logging to be a map")
	}
	output, ok := logging["output"].([]interface{})
	if !ok {
		t.Fatal("expected logging.output to be a list")
	}
	if len(output) != 2 {
		t.Errorf("expected logging.output to have 2 items, got %d", len(output))
	}
}

// TestValidNestedYAML_ParseFileToMap tests parsing valid_nested.yaml using ParseFileToMap method.
func TestValidNestedYAML_ParseFileToMap(t *testing.T) {
	testFile := "testdata/valid_nested.yaml"
	parser := NewParser()

	result := parser.ParseFileToMap(testFile)

	if !result.Success {
		t.Fatalf("ParseFileToMap failed: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected result.Data to be map[string]interface{}")
	}

	// Verify nested structure values
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server to be a map")
	}

	if server["host"] != "localhost" {
		t.Errorf("expected server.host='localhost', got %v", server["host"])
	}

	// Handle int vs int64 for port
	if port, ok := server["port"].(int); ok {
		if port != 8080 {
			t.Errorf("expected server.port=8080, got %d", port)
		}
	} else if port, ok := server["port"].(int64); ok {
		if port != 8080 {
			t.Errorf("expected server.port=8080, got %d", port)
		}
	} else {
		t.Errorf("expected server.port to be int or int64, got %T", server["port"])
	}

	// Verify ssl nested structure
	ssl, ok := server["ssl"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server.ssl to be a map")
	}

	if ssl["enabled"] != true {
		t.Errorf("expected server.ssl.enabled=true, got %v", ssl["enabled"])
	}

	if ssl["certificate"] != "/path/to/cert.pem" {
		t.Errorf("expected server.ssl.certificate='/path/to/cert.pem', got %v", ssl["certificate"])
	}

	if ssl["key"] != "/path/to/key.pem" {
		t.Errorf("expected server.ssl.key='/path/to/key.pem', got %v", ssl["key"])
	}

	// Verify database structure
	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database to be a map")
	}

	primary, ok := database["primary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database.primary to be a map")
	}

	if primary["host"] != "db1.example.com" {
		t.Errorf("expected database.primary.host='db1.example.com', got %v", primary["host"])
	}

	// Handle int vs int64 for primary.port
	if port, ok := primary["port"].(int); ok {
		if port != 5432 {
			t.Errorf("expected database.primary.port=5432, got %d", port)
		}
	} else if port, ok := primary["port"].(int64); ok {
		if port != 5432 {
			t.Errorf("expected database.primary.port=5432, got %d", port)
		}
	} else {
		t.Errorf("expected database.primary.port to be int or int64, got %T", primary["port"])
	}

	if primary["name"] != "production" {
		t.Errorf("expected database.primary.name='production', got %v", primary["name"])
	}

	replica, ok := database["replica"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database.replica to be a map")
	}

	if replica["host"] != "db2.example.com" {
		t.Errorf("expected database.replica.host='db2.example.com', got %v", replica["host"])
	}

	if replica["name"] != "production_replica" {
		t.Errorf("expected database.replica.name='production_replica', got %v", replica["name"])
	}

	// Verify logging structure
	logging, ok := data["logging"].(map[string]interface{})
	if !ok {
		t.Fatal("expected logging to be a map")
	}

	if logging["level"] != "debug" {
		t.Errorf("expected logging.level='debug', got %v", logging["level"])
	}

	if logging["format"] != "json" {
		t.Errorf("expected logging.format='json', got %v", logging["format"])
	}

	output, ok := logging["output"].([]interface{})
	if !ok {
		t.Fatal("expected logging.output to be a list")
	}

	if len(output) != 2 {
		t.Errorf("expected logging.output to have 2 items, got %d", len(output))
	}

	if output[0] != "stdout" {
		t.Errorf("expected first logging.output='stdout', got %v", output[0])
	}

	if output[1] != "/var/log/app.log" {
		t.Errorf("expected second logging.output='/var/log/app.log', got %v", output[1])
	}
}
