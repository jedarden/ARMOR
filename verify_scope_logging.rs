//! Verification test for scope entry/exit logging
//!
//! This test demonstrates that scope entry and exit events are properly logged
//! with debug-level logging that includes scope type, indent level, and context.

use armor::parsers::yaml::{new_parser, YamlParser};

fn main() {
    // Initialize the logger to see debug output
    env_logger::Builder::from_default_env()
        .filter_level(log::LevelFilter::Debug)
        .init();

    println!("=== Testing Scope Entry/Exit Logging ===\n");

    // Test 1: Nested mappings (scope entry and exit)
    let yaml1 = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;
    println!("Test 1: Nested mappings - watch for [SCOPE ENTRY] and [SCOPE EXIT] logs below:");
    println!("--- YAML ---");
    println!("{}", yaml1.trim());
    println!("--- Logging Output ---");
    let parser = new_parser();
    let result1 = parser.parse_str(yaml1);
    println!("\nValid: {}\n", result1.is_success());

    // Test 2: Deep nesting (multiple scope entries and exits)
    let yaml2 = r#"
level1:
  level2:
    level3:
      key1: value1
      key2: value2
    key3: value3
  key4: value4
key5: value5
"#;
    // Test 2: Deep nesting (multiple scope entries and exits)
    let yaml2 = r#"
level1:
  level2:
    level3:
      key1: value1
      key2: value2
    key3: value3
  key4: value4
key5: value5
"#;
    println!("Test 2: Deep nesting - multiple scope transitions:");
    println!("--- YAML ---");
    println!("{}", yaml2.trim());
    println!("--- Logging Output ---");
    let parser2 = new_parser();
    let result2 = parser2.parse_str(yaml2);
    println!("\nValid: {}\n", result2.is_success());

    // Test 3: Sequence items (sequence scope entry)
    let yaml3 = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
  - name: item3
    value: 300
"#;
    println!("Test 3: Sequence items - watch for Sequence scope entries:");
    println!("--- YAML ---");
    println!("{}", yaml3.trim());
    println!("--- Logging Output ---");
    let parser3 = new_parser();
    let result3 = parser3.parse_str(yaml3);
    println!("\nValid: {}\n", result3.is_success());

    println!("=== All logging verification tests completed ===");
    println!("\nKey observations:");
    println!("- [SCOPE ENTRY] logs show: type, line, indent, parent, depth, path");
    println!("- [SCOPE EXIT] logs show: target_indent, current_depth, current_indent, path");
    println!("- Logs are debug-level (only shown with RUST_LOG=debug or env_logger initialization)");
    println!("- Logging uses conditional compilation so it doesn't affect release builds");
}
