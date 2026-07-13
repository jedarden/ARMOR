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
fn test_push_scope_all_five_types() {
    let mut parser = BasicParser::new();

    // Push ALL five scope types
    parser.push_scope(ScopeInfo::root());                          // Root (depth 0)
    parser.push_scope(ScopeInfo::block(1));                       // Block (depth 1)
    parser.push_scope(ScopeInfo::new(ScopeType::BlockSequence, 2));  // BlockSequence (depth 2)
    parser.push_scope(ScopeInfo::new(ScopeType::FlowMapping, 3));     // FlowMapping (depth 3)
    parser.push_scope(ScopeInfo::new(ScopeType::FlowSequence, 4));     // FlowSequence (depth 4)

    // Verify all were added
    assert_eq!(parser.scope_info_stack().len(), 5, "Scope info stack should have 5 items");

    // Verify types in order
    let scopes = parser.scope_info_stack();
    assert_eq!(scopes[0].scope_type(), ScopeType::Root, "First scope should be Root");
    assert_eq!(scopes[1].scope_type(), ScopeType::Block, "Second scope should be Block");
    assert_eq!(scopes[2].scope_type(), ScopeType::BlockSequence, "Third scope should be BlockSequence");
    assert_eq!(scopes[3].scope_type(), ScopeType::FlowMapping, "Fourth scope should be FlowMapping");
    assert_eq!(scopes[4].scope_type(), ScopeType::FlowSequence, "Fifth scope should be FlowSequence");

    // Verify depths
    assert_eq!(scopes[0].scope_depth(), 0, "Root should have depth 0");
    assert_eq!(scopes[1].scope_depth(), 1, "Block should have depth 1");
    assert_eq!(scopes[2].scope_depth(), 2, "BlockSequence should have depth 2");
    assert_eq!(scopes[3].scope_depth(), 3, "FlowMapping should have depth 3");
    assert_eq!(scopes[4].scope_depth(), 4, "FlowSequence should have depth 4");

    // Verify scope type helper methods
    assert!(scopes[0].is_root(), "Root should identify as root");
    assert!(scopes[1].is_block(), "Block should identify as block");
    assert!(scopes[1].is_mapping(), "Block should identify as mapping");
    assert!(scopes[2].is_sequence(), "BlockSequence should identify as sequence");
    assert!(scopes[2].is_block(), "BlockSequence should identify as block");
    assert!(scopes[3].is_flow(), "FlowMapping should identify as flow");
    assert!(scopes[3].is_mapping(), "FlowMapping should identify as mapping");
    assert!(scopes[4].is_flow(), "FlowSequence should identify as flow");
    assert!(scopes[4].is_sequence(), "FlowSequence should identify as sequence");
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

#[test]
fn test_push_scope_consecutive_incremental_growth() {
    let mut parser = BasicParser::new();

    // Initially empty
    assert_eq!(parser.scope_info_stack().len(), 0, "Stack should be empty initially");

    // First push
    parser.push_scope(ScopeInfo::block(1));
    assert_eq!(parser.scope_info_stack().len(), 1, "Stack should have 1 item after first push");

    // Second push
    parser.push_scope(ScopeInfo::block(2));
    assert_eq!(parser.scope_info_stack().len(), 2, "Stack should have 2 items after second push");

    // Third push
    parser.push_scope(ScopeInfo::block(3));
    assert_eq!(parser.scope_info_stack().len(), 3, "Stack should have 3 items after third push");

    // Verify stack growth was consistent
    let scopes = parser.scope_info_stack();
    assert_eq!(scopes.len(), 3, "Final stack size should be 3");
}

#[test]
fn test_push_scope_consecutive_lifo_order() {
    let mut parser = BasicParser::new();

    // Push scopes in sequence with different depths to track order
    parser.push_scope(ScopeInfo::block(10));
    parser.push_scope(ScopeInfo::block(20));
    parser.push_scope(ScopeInfo::block(30));

    // Verify LIFO order: last pushed is last in stack (top of stack)
    let scopes = parser.scope_info_stack();

    // First element should be the first pushed (bottom of stack)
    assert_eq!(scopes[0].scope_depth(), 10, "Bottom of stack should be first pushed (depth 10)");

    // Middle element should be the second pushed
    assert_eq!(scopes[1].scope_depth(), 20, "Middle of stack should be second pushed (depth 20)");

    // Last element should be the last pushed (top of stack)
    assert_eq!(scopes[2].scope_depth(), 30, "Top of stack should be last pushed (depth 30)");

    // Verify that .last() returns the most recently pushed item
    assert_eq!(
        scopes.last().unwrap().scope_depth(),
        30,
        "last() should return the most recently pushed scope"
    );
}

#[test]
fn test_push_scope_mixed_type_identification() {
    let mut parser = BasicParser::new();

    // Push a variety of scope types in mixed order
    parser.push_scope(ScopeInfo::root());                          // Root
    parser.push_scope(ScopeInfo::block(1));                       // Block mapping
    parser.push_scope(ScopeInfo::new(ScopeType::FlowSequence, 2)); // Flow sequence
    parser.push_scope(ScopeInfo::block_sequence(3));              // Block sequence
    parser.push_scope(ScopeInfo::flow_mapping(4));                // Flow mapping

    let scopes = parser.scope_info_stack();
    assert_eq!(scopes.len(), 5, "Should have 5 scopes");

    // Verify each scope type is correctly identified using multiple methods
    // Root scope
    assert_eq!(scopes[0].scope_type(), ScopeType::Root, "First should be Root");
    assert!(scopes[0].is_root(), "Root should identify as root");
    assert!(!scopes[0].is_block(), "Root should not identify as block");
    assert!(!scopes[0].is_flow(), "Root should not identify as flow");
    assert!(!scopes[0].is_sequence(), "Root should not identify as sequence");
    assert!(!scopes[0].is_mapping(), "Root should not identify as mapping");

    // Block mapping scope
    assert_eq!(scopes[1].scope_type(), ScopeType::Block, "Second should be Block");
    assert!(scopes[1].is_block(), "Block should identify as block");
    assert!(scopes[1].is_mapping(), "Block should identify as mapping");
    assert!(!scopes[1].is_flow(), "Block should not identify as flow");
    assert!(!scopes[1].is_sequence(), "Block should not identify as sequence");
    assert!(!scopes[1].is_root(), "Block should not identify as root");

    // Flow sequence scope
    assert_eq!(scopes[2].scope_type(), ScopeType::FlowSequence, "Third should be FlowSequence");
    assert!(scopes[2].is_flow(), "FlowSequence should identify as flow");
    assert!(scopes[2].is_sequence(), "FlowSequence should identify as sequence");
    assert!(!scopes[2].is_block(), "FlowSequence should not identify as block");
    assert!(!scopes[2].is_mapping(), "FlowSequence should not identify as mapping");
    assert!(!scopes[2].is_root(), "FlowSequence should not identify as root");

    // Block sequence scope
    assert_eq!(scopes[3].scope_type(), ScopeType::BlockSequence, "Fourth should be BlockSequence");
    assert!(scopes[3].is_block(), "BlockSequence should identify as block");
    assert!(scopes[3].is_sequence(), "BlockSequence should identify as sequence");
    assert!(!scopes[3].is_flow(), "BlockSequence should not identify as flow");
    assert!(!scopes[3].is_mapping(), "BlockSequence should not identify as mapping");
    assert!(!scopes[3].is_root(), "BlockSequence should not identify as root");

    // Flow mapping scope
    assert_eq!(scopes[4].scope_type(), ScopeType::FlowMapping, "Fifth should be FlowMapping");
    assert!(scopes[4].is_flow(), "FlowMapping should identify as flow");
    assert!(scopes[4].is_mapping(), "FlowMapping should identify as mapping");
    assert!(!scopes[4].is_block(), "FlowMapping should not identify as block");
    assert!(!scopes[4].is_sequence(), "FlowMapping should not identify as sequence");
    assert!(!scopes[4].is_root(), "FlowMapping should not identify as root");
}

#[test]
fn test_push_scope_flow_sequence_type() {
    let mut parser = BasicParser::new();

    // Push a flow sequence scope
    parser.push_scope(ScopeInfo::new(ScopeType::FlowSequence, 2));

    // Verify it was added with correct type
    let pushed_info = parser.scope_info_stack().last().unwrap();
    assert_eq!(pushed_info.scope_type(), ScopeType::FlowSequence, "Pushed scope should be FlowSequence type");
    assert!(pushed_info.is_sequence(), "Pushed scope should identify as sequence");
    assert!(pushed_info.is_flow(), "Pushed scope should identify as flow");
    assert!(!pushed_info.is_block(), "Pushed scope should not identify as block");
}
