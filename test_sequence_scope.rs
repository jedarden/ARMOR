use armor::parsers::yaml::scope::ScopeStack;

fn main() {
    let mut stack = ScopeStack::new(2);

    // Enter items mapping
    stack.enter_scope(2, 1, Some("items".to_string()));
    println!("After items: depth={}, indent={}, path='{}'", stack.depth(), stack.current_indent(), stack.get_scope_path());

    // Enter first sequence item
    stack.enter_sequence_scope(2, 2);
    println!("After first sequence item: depth={}, indent={}, path='{}'", stack.depth(), stack.current_indent(), stack.get_scope_path());

    // Add name to sequence scope
    stack.add_key("name", 3).unwrap();
    println!("After adding 'name': contains_key('name')={}, path='{}'", stack.contains_key("name"), stack.get_scope_path());

    // Add value to sequence scope
    stack.add_key("value", 4).unwrap();
    println!("After adding 'value': contains_key('value')={}, path='{}'", stack.contains_key("value"), stack.get_scope_path());

    // Enter second sequence item
    stack.enter_sequence_scope(2, 5);
    println!("After second sequence item: depth={}, indent={}, path='{}'", stack.depth(), stack.current_indent(), stack.get_scope_path());

    // Try to add name again (should succeed since it's a new sequence scope)
    let result = stack.add_key("name", 6);
    println!("After adding 'name' again: result={:?}, contains_key('name')={}, path='{}'", result, stack.contains_key("name"), stack.get_scope_path());
}
