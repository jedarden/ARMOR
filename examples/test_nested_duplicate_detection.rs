//! Test nested duplicate detection - same key names in different scopes should be OK
use armor::parsers::yaml::SyntaxDetector;

fn main() {
    println!("=== Testing Nested Duplicate Detection ===\n");

    // Test 1: Same key names in sibling nested mappings (should be OK)
    println!("--- Test 1: Sibling Mappings ---");
    let yaml1 = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

    let mut detector = SyntaxDetector::new();
    let errors = detector.detect_errors(yaml1);

    if errors.is_empty() {
        println!("✓ PASS: No duplicate key errors (expected)");
    } else {
        println!("✗ FAIL: Unexpected errors:");
        for error in &errors {
            println!("  Line {}: {}", error.line.unwrap_or(0), error.message);
        }
    }

    println!();

    // Test 2: Actual duplicate in same scope (should fail)
    println!("--- Test 2: Actual Duplicate in Same Scope ---");
    let yaml2 = r#"
config:
  host: localhost
  host: duplicate
"#;

    let mut detector2 = SyntaxDetector::new();
    let errors2 = detector2.detect_errors(yaml2);

    if !errors2.is_empty() && errors2.iter().any(|e| e.message.contains("duplicate key")) {
        println!("✓ PASS: Correctly detected duplicate key");
        for error in &errors2 {
            if error.message.contains("duplicate key") {
                println!("  Line {}: {}", error.line.unwrap_or(0), error.message);
            }
        }
    } else {
        println!("✗ FAIL: Should have detected duplicate key");
    }

    println!();

    // Test 3: Deeply nested same-key names (should be OK)
    println!("--- Test 3: Deeply Nested Same Key Names ---");
    let yaml3 = r#"
level1:
  level2:
    level3:
      key: value1
    key: value2
  key: value3
key: value4
"#;

    let mut detector3 = SyntaxDetector::new();
    let errors3 = detector3.detect_errors(yaml3);

    if errors3.is_empty() {
        println!("✓ PASS: No duplicate key errors (expected)");
    } else {
        println!("✗ FAIL: Unexpected errors:");
        for error in &errors3 {
            println!("  Line {}: {}", error.line.unwrap_or(0), error.message);
        }
    }

    println!();

    // Test 4: Complex real-world scenario
    println!("--- Test 4: Complex Real-World Scenario ---");
    let yaml4 = r#"
# Server configuration
server:
  host: localhost
  port: 8080
  ssl:
    enabled: true
    cert: /path/to/cert.pem

# Database configuration
database:
  host: db.example.com
  port: 5432
  credentials:
    username: admin
    password: secret

# Cache configuration
cache:
  host: redis.example.com
  port: 6379
"#;

    let mut detector4 = SyntaxDetector::new();
    let errors4 = detector4.detect_errors(yaml4);

    if errors4.is_empty() {
        println!("✓ PASS: No duplicate key errors (expected)");
    } else {
        println!("✗ FAIL: Unexpected errors:");
        for error in &errors4 {
            println!("  Line {}: {}", error.line.unwrap_or(0), error.message);
        }
    }

    println!();

    // Test 5: Multiple levels with same keys
    println!("--- Test 5: Multiple Levels with Same Keys ---");
    let yaml5 = r#"
app:
  name: myapp
  version: 1.0
  config:
    name: inner_config
    version: 2.0
    nested:
      name: deepest
      version: 3.0
"#;

    let mut detector5 = SyntaxDetector::new();
    let errors5 = detector5.detect_errors(yaml5);

    if errors5.is_empty() {
        println!("✓ PASS: No duplicate key errors (expected)");
    } else {
        println!("✗ FAIL: Unexpected errors:");
        for error in &errors5 {
            println!("  Line {}: {}", error.line.unwrap_or(0), error.message);
        }
    }

    println!();
    println!("=== Test Complete ===");
}
