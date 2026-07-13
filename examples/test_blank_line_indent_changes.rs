use armor::parsers::yaml::parser::{Parser, BasicParser};

fn main() {
    let mut parser = BasicParser::new();

    println!("=== Testing blank line indent change handling ===\n");

    // Test 1: Blank line at different indent (should NOT create new scope)
    let yaml1 = r#"
root:
  key1: value1

  key2: value2
  key3: value3
"#;
    println!("Test 1: Blank line at same indent as keys");
    println!("YAML:\n{}", yaml1);
    let result = parser.parse_str(yaml1);
    println!("Success: {}", result.is_success());
    if !result.is_success() {
        println!("Error: {:?}", result.error());
    }
    println!();

    // Test 2: Blank line with LESS indent than current scope (should exit scope)
    let yaml2 = r#"
root:
  nested:
    key1: value1

key2: value2
"#;
    println!("Test 2: Blank line with less indent than current scope");
    println!("YAML:\n{}", yaml2);
    let result = parser.parse_str(yaml2);
    println!("Success: {}", result.is_success());
    if !result.is_success() {
        println!("Error: {:?}", result.error());
    }
    println!();

    // Test 3: Blank line with MORE indent than current scope (edge case)
    let yaml3 = r#"
root:
  key1: value1

    weird_indent: value2
  key2: value3
"#;
    println!("Test 3: Blank line with more indent than current scope");
    println!("YAML:\n{}", yaml3);
    let result = parser.parse_str(yaml3);
    println!("Success: {}", result.is_success());
    if !result.is_success() {
        println!("Error: {:?}", result.error());
    }
    println!();

    // Test 4: Multiple blank lines at various indents
    let yaml4 = r#"
level1:
  level2:
    key1: value1


  key2: value2

level3: value3
"#;
    println!("Test 4: Multiple blank lines at various indents");
    println!("YAML:\n{}", yaml4);
    let result = parser.parse_str(yaml4);
    println!("Success: {}", result.is_success());
    if !result.is_success() {
        println!("Error: {:?}", result.error());
    }
    println!();

    // Test 5: Valid YAML with blank lines (should work)
    let yaml5 = r#"
services:
  web:
    host: localhost
    port: 8080

  database:
    host: db.example.com
    port: 5432

cache:
  enabled: true
"#;
    println!("Test 5: Valid YAML with blank lines (control test)");
    println!("YAML:\n{}", yaml5);
    let result = parser.parse_str(yaml5);
    println!("Success: {}", result.is_success());
    if result.is_success() {
        let value = result.unwrap();
        println!("Parsed: {} keys at root", value.as_mapping().unwrap().len());
    } else {
        println!("Error: {:?}", result.error());
    }
}
