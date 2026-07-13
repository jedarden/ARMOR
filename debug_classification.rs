//! Quick test to verify line classification

use armor::parsers::yaml::scope::classify_line_type;

fn main() {
    println!("=== Test 1: Empty lines ===");
    let test1 = r#"


key: value
"#;

    for (i, line) in test1.lines().enumerate() {
        let classification = classify_line_type(line);
        println!("Line {}: '{}' => {:?}", i, line, classification);
    }

    println!("\n=== Test 2: Complex YAML ===");
    let test2 = r#"
# Configuration file
application:
  name: MyApp
  version: 1.0
  # Server configuration
  server:
    host: localhost
    port: 8080

  # Database settings
  database:
    host: db.example.com
    port: 5432

# Logging configuration
logging:
  level: info
  outputs:
    - type: stdout
      format: json
    - type: file
      format: text
        path: /var/log/app.log
"#;

    let mut key_bearing_count = 0;
    let mut indent_only_count = 0;
    let mut empty_count = 0;

    for (i, line) in test2.lines().enumerate() {
        let classification = classify_line_type(line);
        match classification {
            armor::parsers::yaml::scope::LineClassification::KeyBearing => key_bearing_count += 1,
            armor::parsers::yaml::scope::LineClassification::IndentOnly => indent_only_count += 1,
            armor::parsers::yaml::scope::LineClassification::Empty => empty_count += 1,
        }
        if i < 20 {
            println!("Line {}: '{}' => {:?}", i, line, classification);
        }
    }

    println!("\nTotals: key-bearing={}, indent-only={}, empty={}", key_bearing_count, indent_only_count, empty_count);

    println!("\n=== Test 3: Sequence items ===");
    let test3 = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
  - value_only
"#;

    for (i, line) in test3.lines().enumerate() {
        let classification = classify_line_type(line);
        println!("Line {}: '{}' => {:?}", i, line, classification);
    }
}
