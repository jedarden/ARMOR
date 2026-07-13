use armor::parsers::yaml::{BasicParser, Parser};

fn main() {
    let parser = BasicParser::strict();

    // Test 1: YAML WITH blank line at start
    let yaml_with_blank = r#"
items:
  - name: First
    value: 1
  - name: Second
    value: 2
  - name: Third
    value: 3
"#;

    println!("=== Test 1: YAML WITH blank line at start ===");
    let validation = parser.validate_str(yaml_with_blank);
    if !validation.is_valid() {
        for error in &validation.errors {
            println!("Validation error: {}", error.message);
        }
    } else {
        println!("No errors!");
    }

    // Test 2: YAML WITHOUT blank line at start
    let yaml_no_blank = r#"items:
  - name: First
    value: 1
  - name: Second
    value: 2
  - name: Third
    value: 3
"#;

    println!("\n=== Test 2: YAML WITHOUT blank line at start ===");
    let validation = parser.validate_str(yaml_no_blank);
    if !validation.is_valid() {
        for error in &validation.errors {
            println!("Validation error: {}", error.message);
        }
    } else {
        println!("No errors!");
    }
}
