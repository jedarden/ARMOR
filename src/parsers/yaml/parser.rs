//! Core parser trait and implementations for YAML parsing
//!
//! This module defines the Parser trait and provides functionality for
//! parsing YAML content from various sources.

use crate::parsers::yaml::{
    types::{ParseResult, ValidationResult, ValidationError},
    error::ParseError,
    ParserConfig,
    syntax_validator::SyntaxValidator,
    syntax_detector::SyntaxDetector,
    scope::ScopeStack,
    line_parser::{calculate_indentation},
    scope::extract_key_context,
};

#[cfg(debug_assertions)]
use log::debug as log_debug;
#[cfg(debug_assertions)]
use log::warn as log_warn;

/// Trait for YAML parsers
///
/// This trait defines the core interface for parsing YAML content.
/// Implementations can parse from strings, files, or other sources.
pub trait Parser {
    /// Parse YAML content from a string
    ///
    /// # Arguments
    /// * `content` - The YAML content as a string
    ///
    /// # Returns
    /// A ParseResult containing the parsed data or an error
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a byte slice
    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value>;

    /// Parse YAML content from a file
    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value>;

    /// Validate YAML content without fully parsing it
    fn validate_str(&self, content: &str) -> ValidationResult;

    /// Validate a YAML file without fully parsing it
    fn validate_file(&self, path: &std::path::Path) -> ValidationResult;

    /// Get the parser configuration
    ///
    /// # Returns
    /// A reference to the parser's configuration
    fn config(&self) -> &ParserConfig;

    /// Set the parser configuration
    ///
    /// # Arguments
    /// * `config` - The new configuration
    ///
    /// # Returns
    /// The parser with the new configuration
    fn with_config(self, config: ParserConfig) -> Self
    where
        Self: Sized;
}

/// Basic YAML parser implementation
///
/// This is a minimal implementation of the Parser trait that
/// provides basic YAML parsing functionality with scope-aware key tracking.
#[derive(Debug, Clone)]
pub struct BasicParser {
    config: ParserConfig,
    /// Scope stack for tracking keys within their proper scope contexts
    scope_stack: ScopeStack,
}

impl BasicParser {
    /// Create a new BasicParser with default configuration
    pub fn new() -> Self {
        Self {
            config: ParserConfig::default(),
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
        }
    }

    /// Create a new BasicParser with the specified configuration
    pub fn with_config(config: ParserConfig) -> Self {
        Self {
            config,
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
        }
    }

    /// Create a new strict parser
    ///
    /// A strict parser enables strict mode and disallows duplicate keys.
    /// Uses the comprehensive strict configuration from ParserConfig.
    pub fn strict() -> Self {
        Self {
            config: ParserConfig::strict(),
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
        }
    }

    /// Detect duplicate keys using scope-aware tracking
    ///
    /// This method processes YAML content line-by-line, tracking keys within
    /// their proper scope contexts. It returns a list of validation errors
    /// for any duplicate keys found within the same scope.
    fn detect_duplicate_keys_with_scope(&self, content: &str, scope_stack: &mut ScopeStack) -> Vec<ValidationError> {
        let mut duplicate_errors = Vec::new();

        for (line_num, line) in content.lines().enumerate() {
            let line_num_1index = line_num + 1;
            let trimmed = line.trim();

            // Handle document markers - reset scope tracking
            if trimmed == "---" || trimmed == "..." {
                #[cfg(debug_assertions)]
                {
                    log_debug!("[detect_duplicate] Document marker '{}' at line={}, resetting scope stack", trimmed, line_num_1index);
                }
                scope_stack.reset();
                continue;
            }

            // Skip YAML directives
            if trimmed.starts_with('%') {
                continue;
            }

            let indent = calculate_indentation(line);

            // Handle blank lines with indentation changes
            if trimmed.is_empty() || trimmed.starts_with('#') {
                // Check if blank line has a different indentation than current scope
                if indent != scope_stack.current_indent() && !trimmed.starts_with('#') {
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[detect_duplicate] Blank line with indent change: line={}, indent={}, current_indent={}, skipping",
                            line_num_1index, indent, scope_stack.current_indent());
                    }
                    // Blank lines with indent changes are skipped but don't affect scope
                    continue;
                }
                // Regular blank lines at same indent are skipped
                continue;
            }

            // Handle scope transitions based on indentation changes
            use std::cmp::Ordering;
            match indent.cmp(&scope_stack.current_indent()) {
                Ordering::Greater => {
                    // Indent increased - entering deeper scope
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // Add the parent key to current scope
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            // Enter new scope for the parent key's nested content
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else {
                            // Indent increased but not a parent key - this is an inline scalar at deeper indent
                            // Just add the key to current scope, don't create a new scope
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Inline scalar: key='{}', indent={}, line={}, in_sequence={}",
                                    ctx.key_name(), indent, line_num_1index, scope_stack.in_sequence_context());
                            }
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    } else {
                        // Indent increased but no key context found - this is likely:
                        // - A sequence item continuation (already handled by sequence scope logic)
                        // - A scalar value continuation
                        // - Don't enter a scope for non-key lines
                        #[cfg(debug_assertions)]
                        {
                            log_debug!("[detect_duplicate] No key context: line={}, indent={}, in_sequence={}, skipping scope entry",
                                line_num_1index, indent, scope_stack.in_sequence_context());
                        }
                    }
                }
                Ordering::Less => {
                    // Indent decreased - exit to parent scope
                    #[cfg(debug_assertions)]
                    {
                        let current_path = scope_stack.get_scope_path();
                        log_debug!("[detect_duplicate] Scope exit: from_indent={}, to_indent={}, line={}, current_scope='{}'",
                            scope_stack.current_indent(), indent, line_num_1index, current_path);
                    }
                    scope_stack.exit_to_scope(indent);

                    // After exiting, check if this line has a key
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a new parent key at this level
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                    // Edge case: indent decreased but no key - we've already exited to the right scope
                }
                Ordering::Equal => {
                    // Same scope - check for keys
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a sibling parent key at same indent level
                            // Exit and re-enter scope for the sibling
                            #[cfg(debug_assertions)]
                            {
                                let current_path = scope_stack.get_scope_path();
                                log_debug!("[detect_duplicate] Scope exit for sibling: key='{}', indent={}, line={}, current_scope='{}'",
                                    ctx.key_name(), indent, line_num_1index, current_path);
                            }
                            scope_stack.exit_to_scope(indent);
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[detect_duplicate] Re-entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                duplicate_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                    // Edge case: same indent but no key - just continue, no scope change needed
                }
            }

            // Handle sequence items with their own scopes
            if trimmed.starts_with("- ") {
                if let Some(ctx) = extract_key_context(line) {
                    // Enter a sequence scope for this item
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[detect_duplicate] Entering scope: type=Sequence, key='{}', indent={}, line={}",
                            ctx.key_name(), indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                    // Add the key to the sequence scope
                    if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                        duplicate_errors.push(ValidationError::new(
                            format!("line_{}", line_num_1index),
                            dup_err.message()
                        ).with_line(line_num_1index));
                    }
                } else {
                    // Sequence item without a key context
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[detect_duplicate] Entering scope: type=Sequence (no key), indent={}, line={}",
                            indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                }
            }
        }

        duplicate_errors
    }
}

impl Default for BasicParser {
    fn default() -> Self {
        Self::new()
    }
}

impl Parser for BasicParser {
    fn parse_str(&self, content: &str) -> ParseResult<serde_yaml::Value> {
        // Create a new scope stack for this parsing operation
        let mut scope_stack = ScopeStack::new(2);
        let mut parse_errors = Vec::new();

        // Parse line by line with scope-aware key tracking
        for (line_num, line) in content.lines().enumerate() {
            let line_num_1index = line_num + 1;
            let trimmed = line.trim();

            // Handle document markers - reset scope tracking
            if trimmed == "---" || trimmed == "..." {
                #[cfg(debug_assertions)]
                {
                    log_debug!("[parse_str] Document marker '{}' at line={}, resetting scope stack", trimmed, line_num_1index);
                }
                scope_stack.reset();
                continue;
            }

            // Skip YAML directives
            if trimmed.starts_with('%') {
                continue;
            }

            let indent = calculate_indentation(line);

            // Handle blank lines with indentation changes
            if trimmed.is_empty() || trimmed.starts_with('#') {
                // Check if blank line has a different indentation than current scope
                if indent != scope_stack.current_indent() && !trimmed.starts_with('#') {
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[parse_str] Blank line with indent change: line={}, indent={}, current_indent={}",
                            line_num_1index, indent, scope_stack.current_indent());
                    }
                    // Update scope to match blank line indentation
                    // This handles the case where blank lines appear at different indentation levels
                    if indent < scope_stack.current_indent() {
                        scope_stack.exit_to_scope(indent);
                    }
                    // Note: we don't enter scopes on blank lines with increased indent,
                    // only exit when indent decreases
                    continue;
                }
                // Regular blank lines at same indent are skipped
                continue;
            }

            // Handle scope transitions based on indentation changes
            use std::cmp::Ordering;
            match indent.cmp(&scope_stack.current_indent()) {
                Ordering::Greater => {
                    // Indent increased - entering deeper scope
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // Add the parent key to current scope
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            // Enter new scope for the parent key's nested content
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else {
                            // Indent increased but not a parent key - this is an inline scalar at deeper indent
                            // Just add the key to current scope, don't create a new scope
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Inline scalar: key='{}', indent={}, line={}, in_sequence={}",
                                    ctx.key_name(), indent, line_num_1index, scope_stack.in_sequence_context());
                            }
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    } else {
                        // Indent increased but no key context found - this is likely:
                        // - A sequence item continuation (already handled by sequence scope logic)
                        // - A scalar value continuation
                        // - Don't enter a scope for non-key lines
                        #[cfg(debug_assertions)]
                        {
                            log_debug!("[parse_str] No key context: line={}, indent={}, in_sequence={}, skipping scope entry",
                                line_num_1index, indent, scope_stack.in_sequence_context());
                        }
                    }
                }
                Ordering::Less => {
                    // Indent decreased - exit to parent scope
                    #[cfg(debug_assertions)]
                    {
                        let current_path = scope_stack.get_scope_path();
                        log_debug!("[parse_str] Scope exit: from_indent={}, to_indent={}, line={}, current_scope='{}'",
                            scope_stack.current_indent(), indent, line_num_1index, current_path);
                    }
                    scope_stack.exit_to_scope(indent);

                    // After exiting, check if this line has a key
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a new parent key at this level
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                }
                Ordering::Equal => {
                    // Same scope - check for keys
                    if let Some(ctx) = extract_key_context(line) {
                        if ctx.is_parent_key() {
                            // This is a sibling parent key at same indent level
                            // Exit and re-enter scope for the sibling
                            #[cfg(debug_assertions)]
                            {
                                let current_path = scope_stack.get_scope_path();
                                log_debug!("[parse_str] Scope exit for sibling: key='{}', indent={}, line={}, current_scope='{}'",
                                    ctx.key_name(), indent, line_num_1index, current_path);
                            }
                            scope_stack.exit_to_scope(indent);
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                            #[cfg(debug_assertions)]
                            {
                                log_debug!("[parse_str] Re-entering scope: type=Mapping, key='{}', indent={}, line={}",
                                    ctx.key_name(), indent, line_num_1index);
                            }
                            scope_stack.enter_scope(
                                indent + scope_stack.base_indent(),
                                line_num_1index,
                                Some(ctx.key_name().to_string())
                            );
                        } else if ctx.is_inline_scalar() {
                            if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                                parse_errors.push(ValidationError::new(
                                    format!("line_{}", line_num_1index),
                                    dup_err.message()
                                ).with_line(line_num_1index));
                            }
                        }
                    }
                }
            }

            // Handle sequence items with their own scopes
            if trimmed.starts_with("- ") {
                if let Some(ctx) = extract_key_context(line) {
                    // Enter a sequence scope for this item
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[parse_str] Entering scope: type=Sequence, key='{}', indent={}, line={}",
                            ctx.key_name(), indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                    // Add the key to the sequence scope
                    if let Err(dup_err) = scope_stack.add_key(ctx.key_name(), line_num_1index) {
                        parse_errors.push(ValidationError::new(
                            format!("line_{}", line_num_1index),
                            dup_err.message()
                        ).with_line(line_num_1index));
                    }
                } else {
                    // Sequence item without a key context
                    #[cfg(debug_assertions)]
                    {
                        log_debug!("[parse_str] Entering scope: type=Sequence (no key), indent={}, line={}",
                            indent, line_num_1index);
                    }
                    scope_stack.enter_sequence_scope(indent, line_num_1index);
                }
            }
        }

        // Return parse result using serde_yaml for the actual value
        match serde_yaml::from_str::<serde_yaml::Value>(content) {
            Ok(value) => {
                // Note: parse_errors from scope tracking are currently informational
                // They could be added as warnings in a future enhancement
                ParseResult::success(value)
            }
            Err(err) => {
                // Add any duplicate key errors we found to the parse error
                let error_msg = if parse_errors.is_empty() {
                    err.to_string()
                } else {
                    format!("{}; duplicate key errors: {}", err, parse_errors.iter()
                        .map(|e| e.message.clone()).collect::<Vec<_>>().join("; "))
                };
                ParseResult::failure(ParseError::syntax(error_msg))
            }
        }
    }

    fn parse_bytes(&self, content: &[u8]) -> ParseResult<serde_yaml::Value> {
        // Convert bytes to string and parse
        match std::str::from_utf8(content) {
            Ok(utf8_content) => self.parse_str(utf8_content),
            Err(err) => ParseResult::failure(ParseError::io(format!("Invalid UTF-8: {}", err))),
        }
    }

    fn parse_file(&self, path: &std::path::Path) -> ParseResult<serde_yaml::Value> {
        // Read file content
        match std::fs::read_to_string(path) {
            Ok(content) => self.parse_str(&content),
            Err(err) => ParseResult::failure(ParseError::io(format!("Failed to read file {}: {}", path.display(), err))),
        }
    }

    fn validate_str(&self, content: &str) -> ValidationResult {
        // Create syntax validator based on parser mode
        let validator = if self.config.is_strict() {
            SyntaxValidator::strict()
        } else {
            SyntaxValidator::lenient()
        };

        // Run syntax validation
        let mut result = validator.validate(content);

        // Run scope-aware duplicate key detection (when duplicates are not allowed)
        if !self.config.allow_duplicates {
            let mut scope_stack = self.scope_stack.clone();
            let duplicate_errors = self.detect_duplicate_keys_with_scope(content, &mut scope_stack);
            if !duplicate_errors.is_empty() {
                result.valid = false;
                result.errors.extend(duplicate_errors);
            }
        }

        // If no errors from basic validation, run enhanced detection
        if result.is_valid() {
            let mut detector = SyntaxDetector::new();
            let detector_result = detector.detect_to_validation_result(content);

            // Merge errors from detector
            if !detector_result.is_valid() {
                result.valid = false;
                result.errors.extend(detector_result.errors);
            }
        }

        result
    }

    fn validate_file(&self, path: &std::path::Path) -> ValidationResult {
        // Read file content
        let content = match std::fs::read_to_string(path) {
            Ok(content) => content,
            Err(err) => {
                return ValidationResult {
                    valid: false,
                    errors: vec![ValidationError::new(
                        path.display().to_string(),
                        format!("failed to read file: {}", err)
                    )],
                    warnings: Vec::new(),
                };
            }
        };

        // Validate the content
        self.validate_str(&content)
    }

    fn config(&self) -> &ParserConfig {
        &self.config
    }

    fn with_config(self, config: ParserConfig) -> Self
    where
        Self: Sized,
    {
        Self {
            config,
            scope_stack: ScopeStack::new(2), // Standard 2-space YAML indentation
        }
    }
}

// ============================================================================
// Integration Tests for Scope Tracking
// ============================================================================

#[cfg(test)]
mod integration_tests {
    use super::*;

    /// Test parse_str with nested YAML structures
    #[test]
    fn test_parse_str_with_nested_yaml() {
        let parser = BasicParser::new();
        let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should parse valid nested YAML successfully");

        // Verify we can access the parsed structure
        let value = result.unwrap();
        assert!(value.is_mapping());
        assert!(value.get("services").is_some());
    }

    /// Test parse_str with deeply nested YAML
    #[test]
    fn test_parse_str_with_deeply_nested_yaml() {
        let parser = BasicParser::new();
        let yaml = r#"
application:
  server:
    config:
      timeouts:
        connect: 30
        read: 60
    logging:
      level: info
  database:
    connection:
      pool:
        max: 10
        min: 2
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Should parse deeply nested YAML successfully");
    }

    /// Test that same key in different scopes passes
    #[test]
    fn test_same_key_different_scopes_passes() {
        let parser = BasicParser::new();

        // This YAML has 'host' and 'port' in different scopes
        let yaml = r#"
services:
  web:
    host: localhost
    port: 8080
  database:
    host: db.example.com
    port: 5432
  cache:
    host: redis.example.com
    port: 6379
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Same key in different scopes should be allowed");

        let value = result.unwrap();
        let services = &value["services"];
        assert_eq!(services["web"]["host"], "localhost");
        assert_eq!(services["database"]["host"], "db.example.com");
        assert_eq!(services["cache"]["host"], "redis.example.com");
    }

    /// Test that duplicate key in same scope is detected by strict parser
    #[test]
    fn test_duplicate_key_same_scope_detected() {
        let parser = BasicParser::strict(); // Strict parser doesn't allow duplicates

        let yaml = r#"
config:
  server:
    host: localhost
    host: duplicate-value
    port: 8080
"#;

        // The parse_str itself may succeed (serde_yaml's behavior), but validation should detect the duplicate
        let validation_result = parser.validate_str(yaml);
        assert!(!validation_result.is_valid(), "Validation should detect duplicate keys");

        // Check that error message mentions duplicate key
        let error_messages: Vec<String> = validation_result.errors
            .iter()
            .map(|e| e.message.clone())
            .collect();
        assert!(error_messages.iter().any(|msg| msg.contains("duplicate") || msg.contains("host")));
    }

    /// Test parse_str with sequence items having same keys
    #[test]
    fn test_parse_str_sequence_items_same_keys() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
  - name: item1
    value: 100
  - name: item2
    value: 200
  - name: item3
    value: 300
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Sequence items with same keys should be allowed");

        let value = result.unwrap();
        let items = value["items"].as_sequence().unwrap();
        assert_eq!(items.len(), 3);
    }

    /// Test parse_str with mixed mapping and sequence scopes
    #[test]
    fn test_parse_str_mixed_mapping_sequence() {
        let parser = BasicParser::new();

        let yaml = r#"
services:
  web:
    endpoints:
      - path: /api
        method: GET
      - path: /health
        method: GET
    config:
      timeout: 30
  database:
    endpoints:
      - path: /query
        method: POST
    config:
      pool_size: 10
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Mixed mapping and sequence structures should parse successfully");
    }

    /// Test parse_str with multiple indent transitions
    #[test]
    fn test_parse_str_multiple_indent_transitions() {
        let parser = BasicParser::new();

        let yaml = r#"
level1_a:
  level2_a:
    level3:
      value: 1
  level2_b:
    level3:
      value: 2
level1_b:
  level2:
    level3:
      value: 3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Multiple indent transitions should be handled correctly");
    }

    /// Test parse_str with inline scalars (no scope creation)
    #[test]
    fn test_parse_str_inline_scalars() {
        let parser = BasicParser::new();

        let yaml = r#"
name: test
version: "1.0"
author: example
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Inline scalars should parse successfully");

        let value = result.unwrap();
        assert_eq!(value["name"], "test");
        assert_eq!(value["version"], "1.0");
    }

    /// Test parse_str with empty lines and comments
    #[test]
    fn test_parse_str_with_comments_and_empty_lines() {
        let parser = BasicParser::new();

        let yaml = r#"
# Configuration file
name: test

# Server settings
server:
  host: localhost
  # Port number
  port: 8080

# Database settings
database:
  host: db.example.com
  port: 5432
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Comments and empty lines should be handled correctly");
    }

    /// Test validate_str detects duplicate keys
    #[test]
    fn test_validate_str_duplicate_detection() {
        let parser = BasicParser::strict();

        let yaml = r#"
config:
  setting1: value1
  setting1: value2
  setting2: value3
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid(), "Should detect duplicate keys in same scope");
        assert!(!result.errors.is_empty(), "Should have validation errors");
    }

    /// Test validate_str allows same key in different scopes
    #[test]
    fn test_validate_str_same_key_different_scopes() {
        let parser = BasicParser::strict();

        let yaml = r#"
section1:
  key: value1
section2:
  key: value2
section3:
  key: value3
"#;

        let result = parser.validate_str(yaml);
        assert!(result.is_valid(), "Same key in different scopes should be valid");
    }

    /// Test real-world config scenario
    #[test]
    fn test_real_world_config_scenario() {
        let parser = BasicParser::new();

        let yaml = r#"
application:
  name: myapp
  version: 1.0.0

server:
  host: 0.0.0.0
  port: 8080
  ssl:
    enabled: true
    cert_path: /path/to/cert.pem

database:
  primary:
    host: db1.example.com
    port: 5432
  replica:
    host: db2.example.com
    port: 5432

logging:
  level: info
  outputs:
    - type: stdout
      format: json
    - type: file
      format: text
      path: /var/log/app.log
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Real-world config should parse successfully");

        let value = result.unwrap();
        assert_eq!(value["application"]["name"], "myapp");
        assert_eq!(value["server"]["port"], 8080);
        assert_eq!(value["logging"]["level"], "info");
    }

    /// Test parse_str with document markers
    #[test]
    fn test_parse_str_with_document_markers() {
        let parser = BasicParser::new();

        let yaml = r#"
---
name: test
value: 123
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Document markers should be handled correctly");

        let value = result.unwrap();
        assert_eq!(value["name"], "test");
        assert_eq!(value["value"], 123);
    }

    /// Test scope tracking with complex nested structure
    #[test]
    fn test_scope_tracking_complex_nesting() {
        let parser = BasicParser::new();

        let yaml = r#"
outer:
  inner1:
    deep1:
      value: 1
    deep2:
      value: 2
  inner2:
    deep1:
      deeper:
        value: 3
    deep2:
      value: 4
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Complex nesting should be handled correctly");
    }

    /// Test that parent key followed by nested content creates proper scope
    #[test]
    fn test_parent_key_creates_scope() {
        let parser = BasicParser::new();

        let yaml = r#"
parent:
  child1: value1
  child2: value2
  nested:
    deep: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Parent key with nested content should create proper scope");

        let value = result.unwrap();
        assert_eq!(value["parent"]["child1"], "value1");
        assert_eq!(value["parent"]["nested"]["deep"], "value3");
    }

    /// Test sequence scope isolation
    #[test]
    fn test_sequence_scope_isolation() {
        let parser = BasicParser::new();

        let yaml = r#"
items:
  - id: 1
    name: First
    config:
      enabled: true
  - id: 2
    name: Second
    config:
      enabled: false
  - id: 3
    name: Third
    config:
      enabled: true
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Sequence items should have isolated scopes");

        let value = result.unwrap();
        let items = value["items"].as_sequence().unwrap();
        assert_eq!(items.len(), 3);
    }

    /// Test validation with multiple duplicates in different scopes
    #[test]
    fn test_validate_multiple_scopes_with_duplicates() {
        let parser = BasicParser::strict();

        let yaml = r#"
scope1:
  key: value1
  key: duplicate1
scope2:
  key: value2
scope3:
  key: value3
  key: duplicate2
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid(), "Should detect duplicates in their respective scopes");

        // Should have errors for both duplicates
        assert!(result.errors.len() >= 2, "Should detect multiple duplicate errors");
    }

    /// Test parse_str with various indentation patterns
    #[test]
    fn test_parse_str_various_indentation_patterns() {
        let parser = BasicParser::new();

        let yaml = r#"
root:
  level1_a:
    level2_a:
      level3: value1
    level2_b: value2
  level1_b:
    level2:
      - item1
      - item2
  level1_c: value3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Various indentation patterns should be handled correctly");
    }

    /// Test that empty document parses successfully
    #[test]
    fn test_parse_empty_document() {
        let parser = BasicParser::new();

        let yaml = "";

        let result = parser.parse_str(yaml);
        // Empty document should parse as null
        assert!(result.is_success());
    }

    /// Test that document with only comments parses successfully
    #[test]
    fn test_parse_only_comments() {
        let parser = BasicParser::new();

        let yaml = r#"
# Comment 1
# Comment 2
# Comment 3
"#;

        let result = parser.parse_str(yaml);
        assert!(result.is_success(), "Document with only comments should parse successfully");
    }

    /// Test validation error message quality
    #[test]
    fn test_validation_error_message_quality() {
        let parser = BasicParser::strict();

        let yaml = r#"
config:
  server:
    host: localhost
    host: duplicate
    port: 8080
"#;

        let result = parser.validate_str(yaml);
        assert!(!result.is_valid());

        // Check that error messages are useful
        for error in &result.errors {
            assert!(!error.message.is_empty(), "Error message should not be empty");
            if error.line.is_some() {
                assert!(error.message.contains("Line"), "Error should mention line number");
            }
        }
    }
}
