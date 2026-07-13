//! Unit tests for push_scope method
//!
//! These tests provide focused unit test coverage for the push_scope method,
//! verifying correct behavior for:
//! - Adding scope information to the scope stack
//! - Proper stack growth on multiple push operations
//! - Handling different scope types (Root, Block, FlowMapping, BlockSequence)
//! - Correct scope depth tracking
//! - Stack state isolation between test cases
//!
//! Bead: bf-4pkivk

use armor::parsers::yaml::parser::BasicParser;
use armor::parsers::yaml::scope::{ScopeInfo, ScopeType};

// =============================================================================
// push_scope::new() Tests
// =============================================================================

#[test]
fn test_push_scope_adds_to_stack() {
    let mut parser = BasicParser::new();

    // Initially scope_info_stack should be empty
    assert_eq!(parser.scope_info_stack().len(), 0, "Initial scope info stack should be empty");

    // Push a scope info
    let scope_info = ScopeInfo::block(1);
    parser.push_scope(scope_info);

    // Verify it was added
    assert_eq!(parser.scope_info_stack().len(), 1, "Scope info stack should have 1 item after push");
}

#[test]
fn test_push_scope_preserves_scope_info() {
    let mut parser = BasicParser::new();

    // Create and push a scope info
    let scope_info = ScopeInfo::block(1);
    parser.push_scope(scope_info);

    // Verify the pushed scope info matches
    let pushed_info = parser.scope_info_stack().last().unwrap();
    assert_eq!(pushed_info.scope_type(), ScopeType::Block, "Pushed scope should be Block type");
    assert_eq!(pushed_info.scope_depth(), 1, "Pushed scope should have depth 1");
}

#[test]
fn test_push_scope_multiple_times() {
    let mut parser = BasicParser::new();

    // Push multiple scopes
    parser.push_scope(ScopeInfo::block(1));
    parser.push_scope(ScopeInfo::block(2));
    parser.push_scope(ScopeInfo::block(3));

    // Verify all were added
    assert_eq!(parser.scope_info_stack().len(), 3, "Scope info stack should have 3 items");

    // Verify they're in order
    let scopes = parser.scope_info_stack();
    assert_eq!(scopes[0].scope_depth(), 1, "First scope should have depth 1");
    assert_eq!(scopes[1].scope_depth(), 2, "Second scope should have depth 2");
    assert_eq!(scopes[2].scope_depth(), 3, "Third scope should have depth 3");
}

#[test]
fn test_push_scope_different_types() {
    let mut parser = BasicParser::new();

    // Push different scope types
    parser.push_scope(ScopeInfo::root());
    parser.push_scope(ScopeInfo::block(1));
    parser.push_scope(ScopeInfo::new(ScopeType::BlockSequence, 2));
    parser.push_scope(ScopeInfo::new(ScopeType::FlowMapping, 3));

    // Verify all were added
    assert_eq!(parser.scope_info_stack().len(), 4, "Scope info stack should have 4 items");

    // Verify types
    let scopes = parser.scope_info_stack();
    assert_eq!(scopes[0].scope_type(), ScopeType::Root, "First scope should be Root");
    assert_eq!(scopes[1].scope_type(), ScopeType::Block, "Second scope should be Block");
    assert_eq!(scopes[2].scope_type(), ScopeType::BlockSequence, "Third scope should be BlockSequence");
    assert_eq!(scopes[3].scope_type(), ScopeType::FlowMapping, "Fourth scope should be FlowMapping");
}

#[test]
fn test_push_scope_root_type() {
    let mut parser = BasicParser::new();

    // Push a root scope
    parser.push_scope(ScopeInfo::root());

    // Verify it was added with correct type
    let pushed_info = parser.scope_info_stack().last().unwrap();
    assert_eq!(pushed_info.scope_type(), ScopeType::Root, "Pushed scope should be Root type");
    assert!(pushed_info.is_root(), "Pushed scope should identify as root");
}

#[test]
fn test_push_scope_block_sequence_type() {
    let mut parser = BasicParser::new();

    // Push a block sequence scope
    parser.push_scope(ScopeInfo::block_sequence(2));

    // Verify it was added with correct type
    let pushed_info = parser.scope_info_stack().last().unwrap();
    assert_eq!(pushed_info.scope_type(), ScopeType::BlockSequence, "Pushed scope should be BlockSequence type");
    assert!(pushed_info.is_sequence(), "Pushed scope should identify as sequence");
}

#[test]
fn test_push_scope_flow_mapping_type() {
    let mut parser = BasicParser::new();

    // Push a flow mapping scope
    parser.push_scope(ScopeInfo::flow_mapping(3));

    // Verify it was added with correct type
    let pushed_info = parser.scope_info_stack().last().unwrap();
    assert_eq!(pushed_info.scope_type(), ScopeType::FlowMapping, "Pushed scope should be FlowMapping type");
    assert!(pushed_info.is_flow(), "Pushed scope should identify as flow");
}

#[test]
fn test_push_scope_stack_isolation() {
    let mut parser1 = BasicParser::new();
    let mut parser2 = BasicParser::new();

    // Push to first parser
    parser1.push_scope(ScopeInfo::block(1));

    // Push to second parser
    parser2.push_scope(ScopeInfo::block(2));

    // Verify isolation - each parser has its own stack
    assert_eq!(parser1.scope_info_stack().len(), 1, "First parser should have 1 item");
    assert_eq!(parser2.scope_info_stack().len(), 1, "Second parser should have 1 item");
    assert_eq!(parser1.scope_info_stack()[0].scope_depth(), 1, "First parser scope should have depth 1");
    assert_eq!(parser2.scope_info_stack()[0].scope_depth(), 2, "Second parser scope should have depth 2");
}
