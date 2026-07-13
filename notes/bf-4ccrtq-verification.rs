//! Verification script for indent change detection without key tokens
//!
//! Run with: cargo run --example bf-4ccrtq-verification

use armor::parsers::yaml::{parser::BasicParser, YamlParser as Parser};
use armor::parsers::yaml::scope::ScopeStack;

fn main() {
    println!("=== Indent Change Detection Without Key Tokens ===\n");

    // Test YAML with blank lines that change indent
    let yaml = r#"
level1:
  level2:
    key1: value1

key2: value2
"#;

    println!("Test YAML:");
    println!("{}", yaml);
    println!("---");

    // Parse with basic parser
    let parser = BasicParser::new();
    let result = parser.parse_str(yaml);

    println!("\nParse result: {}", if result.is_success() { "SUCCESS" } else { "FAILED" });

    if result.is_success() {
        let value = result.unwrap();
        println!("Parsed structure:");
        println!("  level1.level2.key1 = {}", value["level1"]["level2"]["key1"]);
        println!("  key2 = {}", value["key2"]);
    }

    println!("\n---");

    // Now test with scope stack to see indent transitions
    let mut scope_stack = ScopeStack::new(2);

    // Simulate parsing the YAML line by line
    for (line_num, line) in yaml.lines().enumerate() {
        let line_num_1index = line_num + 1;
        let trimmed = line.trim();
        let indent = line.chars().take_while(|c| c.is_whitespace()).count();

        println!("Line {}: indent={}, content='{}'", line_num_1index, indent, trimmed);

        // Skip empty lines for scope demo
        if trimmed.is_empty() {
            let last_indent = scope_stack.get_last_indent();
            if indent != last_indent {
                println!("  → Indent change detected on blank line: {} → {}", last_indent, indent);
                let processed = scope_stack.process_indent_transition_without_key(line_num_1index, indent);
                println!("  → Scope transition processed: {}", processed);
                println!("  → Current scope path: '{}'", scope_stack.get_scope_path());
            }
            continue;
        }

        // Track indent changes
        if indent != scope_stack.get_last_indent() {
            scope_stack.record_indent_transition(line_num_1index, indent, false, line);
        }
    }

    println!("\n---");
    println!("All recorded indent transitions:");
    for transition in scope_stack.get_indent_transitions() {
        println!("  {}", transition);
    }

    println!("\nTransitions without keys:");
    for transition in scope_stack.get_transitions_without_keys() {
        println!("  {}", transition);
    }

    println!("\n✅ Verification complete!");
}
