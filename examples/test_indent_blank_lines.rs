use armor::parsers::yaml::parser::{Parser, BasicParser};

fn main() {
    let parser = BasicParser::new();
    
    // Test 1: Blank lines with different indentation
    let yaml1 = r#"
key1: value1
  
  key2: value2
key3: value3
"#;
    println!("Test 1: Blank line with indent");
    let result = parser.parse_str(yaml1);
    println!("Success: {}, Result: {:?}", result.is_success(), result);
    
    // Test 2: Multi-line scalar value continuation
    let yaml2 = r#"
key1:
  some value
  that spans
  multiple lines
key2: value2
"#;
    println!("\nTest 2: Multi-line scalar value");
    let result = parser.parse_str(yaml2);
    println!("Success: {}, Result: {:?}", result.is_success(), result);
    
    // Test 3: Empty lines at various indents
    let yaml3 = r#"
root:
    indent4: value
    
  indent2: value2
"#;
    println!("\nTest 3: Empty line then indent decrease");
    let result = parser.parse_str(yaml3);
    println!("Success: {}, Result: {:?}", result.is_success(), result);
}
