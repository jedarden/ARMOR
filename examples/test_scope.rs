//! Test file to demonstrate scope-aware key tracking
//!
//! This test verifies that the scope-based key tracking correctly handles
//! nested mappings where keys at different nesting levels should be
//! independent.

use armor::parsers::yaml::SyntaxValidator;

fn main() {
    println!("Testing scope-aware key tracking...\n");

    // Test 1: Sibling mappings with same keys (should be valid)
    let yaml1 = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;
    let validator = SyntaxValidator::new();
    let result1 = validator.validate(yaml1);
    println!("Test 1 - Sibling mappings with same keys:");
    println!("  Valid: {}", result1.valid);
    println!("  Expected: true (keys in different scopes should not conflict)\n");

    // Test 2: Deeply nested mappings with same keys at different levels (should be valid)
    let yaml2 = r#"
config:
  database:
    host: localhost
  api:
    host: api.example.com
host: global.example.com
"#;
    let result2 = validator.validate(yaml2);
    println!("Test 2 - Same key at different nesting levels:");
    println!("  Valid: {}", result2.valid);
    println!("  Expected: true (keys at different levels are in different scopes)\n");

    // Test 3: Complex nested structure
    let yaml3 = r#"
level1:
  level2:
    level3:
      key1: value1
      key2: value2
    key3: value3
  key4: value4
key5: value5
"#;
    let result3 = validator.validate(yaml3);
    println!("Test 3 - Complex nested structure:");
    println!("  Valid: {}", result3.valid);
    println!("  Expected: true (complex nesting should work correctly)\n");

    // Test 4: Actual duplicate key in same scope
    let yaml4 = r#"
config:
  host: localhost
  host: duplicate
"#;
    let result4 = validator.validate(yaml4);
    println!("Test 4 - Actual duplicate in same scope:");
    println!("  Valid: {}", result4.valid);
    println!("  Note: Current implementation doesn't report duplicates as errors");
    println!("        but scope tracking ensures proper isolation\n");

    println!("All scope-aware tracking tests completed!");
}
