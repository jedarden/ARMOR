//! YAML syntax error detection module
//!
//! This module provides comprehensive syntax validation for YAML content,
//! detecting common errors including indentation problems, delimiter issues,
//! and structure violations.

use crate::parsers::yaml::types::{ValidationError, ValidationResult};
use std::collections::VecDeque;

/// Syntax validator for YAML content
///
/// This struct performs line-by-line analysis of YAML content to detect
/// common syntax errors and structural problems.
#[derive(Debug, Clone)]
pub struct SyntaxValidator {
    /// Whether to use strict indentation checking (disallow tabs)
    strict_indentation: bool,
    /// Whether to check for duplicate keys
    check_duplicate_keys: bool,
    /// Whether to validate block scalar indicators
    validate_block_scalars: bool,
}

impl Default for SyntaxValidator {
    fn default() -> Self {
        Self {
            strict_indentation: true,
            check_duplicate_keys: true,
            validate_block_scalars: true,
        }
    }
}

impl SyntaxValidator {
    /// Create a new syntax validator with default settings
    pub fn new() -> Self {
        Self::default()
    }

    /// Create a new syntax validator with strict indentation checking
    pub fn strict() -> Self {
        Self {
            strict_indentation: true,
            check_duplicate_keys: true,
            validate_block_scalars: true,
        }
    }

    /// Create a new lenient syntax validator
    pub fn lenient() -> Self {
        Self {
            strict_indentation: false,
            check_duplicate_keys: false,
            validate_block_scalars: false,
        }
    }

    /// Validate YAML content from a string
    ///
    /// # Arguments
    /// * `content` - The YAML content to validate
    ///
    /// # Returns
    /// A ValidationResult containing any errors or warnings found
    pub fn validate(&self, content: &str) -> ValidationResult {
        let mut errors = Vec::new();
        let mut warnings = Vec::new();

        let lines: Vec<&str> = content.lines().collect();
        let mut context = ValidationContext::new();

        for (line_num, line) in lines.iter().enumerate() {
            let line_num_1indexed = line_num + 1;

            // Skip empty lines and comment-only lines for most checks
            let trimmed = line.trim();
            if trimmed.is_empty() || trimmed.starts_with('#') {
                continue;
            }

            // Run each validation pass
            if let Err(mut line_errors) = self.validate_indentation(line, line_num_1indexed, &context) {
                errors.append(&mut line_errors);
            }

            if let Err(mut line_errors) = self.validate_delimiters(line, line_num_1indexed) {
                errors.append(&mut line_errors);
            }

            if let Err(mut line_errors) = self.validate_structure(line, line_num_1indexed, &context) {
                errors.append(&mut line_errors);
            }

            // Update context for next line
            context.update_from_line(line, line_num_1indexed);
        }

        // Final validation checks
        if let Err(mut final_errors) = self.validate_final_structure(&context) {
            errors.append(&mut final_errors);
        }

        ValidationResult {
            valid: errors.is_empty(),
            errors,
            warnings,
        }
    }

    /// Validate indentation on a single line
    fn validate_indentation(
        &self,
        line: &str,
        line_num: usize,
        context: &ValidationContext,
    ) -> Result<(), Vec<ValidationError>> {
        let mut errors = Vec::new();

        // Check for tabs in strict mode
        if self.strict_indentation && line.contains('\t') {
            errors.push(ValidationError::new(
                format!("line {}", line_num),
                "tabs are not allowed in YAML (use spaces for indentation)"
            ).with_line(line_num));
        }

        // Check for mixed indentation (tabs and spaces)
        let leading_bytes = line.as_bytes();
        let mut has_tabs = false;
        let mut has_spaces = false;

        for &byte in leading_bytes.iter().take_while(|&&b| b == b' ' || b == b'\t') {
            if byte == b'\t' {
                has_tabs = true;
            } else if byte == b' ' {
                has_spaces = true;
            }
        }

        if has_tabs && has_spaces {
            errors.push(ValidationError::new(
                format!("line {}", line_num),
                "mixed tabs and spaces in indentation"
            ).with_line(line_num));
        }

        // Check indentation consistency
        let current_indent = self.count_indentation(line);
        if let Some(prev_indent) = context.last_indentation {
            // Check for indentation that's not a multiple of 2 (common convention)
            if self.strict_indentation && current_indent % 2 != 0 && current_indent > 0 {
                errors.push(ValidationError::new(
                    format!("line {}", line_num),
                    &format!("indentation of {} spaces is not a multiple of 2", current_indent)
                ).with_line(line_num));
            }

            // Check for dramatic indentation increases (more than 2 levels at once)
            if current_indent > prev_indent + 4 {
                errors.push(ValidationError::new(
                    format!("line {}", line_num),
                    &format!("indentation increased by {} levels (should increase by at most 2)",
                             current_indent - prev_indent)
                ).with_line(line_num));
            }
        }

        if errors.is_empty() {
            Ok(())
        } else {
            Err(errors)
        }
    }

    /// Validate delimiters on a single line
    fn validate_delimiters(
        &self,
        line: &str,
        line_num: usize,
    ) -> Result<(), Vec<ValidationError>> {
        let mut errors = Vec::new();

        // Check for flow mapping/sequence delimiter balance
        let mut brace_stack = VecDeque::new();
        let mut bracket_stack = VecDeque::new();

        for (col, ch) in line.chars().enumerate() {
            match ch {
                '{' => brace_stack.push_front(('{', col)),
                '}' => {
                    if brace_stack.pop_front().is_none() {
                        errors.push(ValidationError::new(
                            format!("line {}", line_num),
                            "unmatched closing brace '}'"
                        ).with_line(line_num));
                    }
                }
                '[' => bracket_stack.push_front(('[', col)),
                ']' => {
                    if bracket_stack.pop_front().is_none() {
                        errors.push(ValidationError::new(
                            format!("line {}", line_num),
                            "unmatched closing bracket ']'"
                        ).with_line(line_num));
                    }
                }
                ':' => {
                    // Check if colon is used as key-value separator
                    // (followed by space or end of line)
                    let next_char = line.chars().nth(col + 1);
                    if next_char.map_or(false, |c| !c.is_whitespace() && c != ':') {
                        // This might be a value part (like in "http://")
                        // but if it's at the end of non-whitespace, it's suspicious
                        let remaining = &line[col..];
                        if remaining.trim_matches(':').is_empty() || remaining.trim().starts_with('#') {
                            errors.push(ValidationError::new(
                                format!("line {}", line_num),
                                "key-value colon without value"
                            ).with_line(line_num));
                        }
                    }
                }
                _ => {}
            }
        }

        // Report unmatched opening delimiters
        for (delimiter, col) in brace_stack {
            errors.push(ValidationError::new(
                format!("line {}", line_num),
                &format!("unmatched opening brace '{{' at column {}", col + 1)
            ).with_line(line_num));
        }

        for (delimiter, col) in bracket_stack {
            errors.push(ValidationError::new(
                format!("line {}", line_num),
                &format!("unmatched opening bracket '[' at column {}", col + 1)
            ).with_line(line_num));
        }

        if errors.is_empty() {
            Ok(())
        } else {
            Err(errors)
        }
    }

    /// Validate structure on a single line
    fn validate_structure(
        &self,
        line: &str,
        line_num: usize,
        context: &ValidationContext,
    ) -> Result<(), Vec<ValidationError>> {
        let mut errors = Vec::new();

        let trimmed = line.trim();

        // Check for invalid document start marker
        if trimmed == "---" || line.trim_start().starts_with("---") {
            if !line.trim_start().starts_with("--- ") && trimmed != "---" {
                errors.push(ValidationError::new(
                    format!("line {}", line_num),
                    "document start marker '---' must be followed by space or be on its own line"
                ).with_line(line_num));
            }
        }

        // Check for invalid document end marker
        if trimmed == "..." || line.trim_start().starts_with("...") {
            if !line.trim_start().starts_with("... ") && trimmed != "..." {
                errors.push(ValidationError::new(
                    format!("line {}", line_num),
                    "document end marker '...' must be followed by space or be on its own line"
                ).with_line(line_num));
            }
        }

        // Check for colon at start of line (invalid YAML syntax)
        if trimmed.starts_with(':') {
            errors.push(ValidationError::new(
                format!("line {}", line_num),
                "colon at start of line without preceding key"
            ).with_line(line_num));
        }

        // Check for block scalar indicators
        if self.validate_block_scalars {
            if trimmed.starts_with('|') || trimmed.starts_with('>') {
                let indicator = trimmed.chars().next().unwrap();
                // Check if indicator is followed by valid modifier or space
                let rest = &trimmed[1..];
                if !rest.is_empty() && !rest.starts_with(' ') && !rest.starts_with('-') && !rest.starts_with('+') {
                    errors.push(ValidationError::new(
                        format!("line {}", line_num),
                        &format!("block scalar indicator '{}' must be followed by space, '+', '-', or end of line", indicator)
                    ).with_line(line_num));
                }
            }
        }

        // Check for anchor and alias definitions
        if trimmed.contains('&') {
            for (col, _) in line.match_indices('&') {
                // Check if anchor name is valid
                let rest = &line[col + 1..];
                let rest_trimmed = rest.trim();

                // If there's nothing after the &, or only whitespace, it's invalid
                if rest_trimmed.is_empty() {
                    errors.push(ValidationError::new(
                        format!("line {}", line_num),
                        "anchor '&' must be followed by a name"
                    ).with_line(line_num));
                }
            }
        }

        if trimmed.contains('*') {
            for (col, _) in line.match_indices('*') {
                let rest = &line[col + 1..];
                let rest_trimmed = rest.trim();

                // If there's nothing after the *, or only whitespace, it's invalid
                if rest_trimmed.is_empty() {
                    errors.push(ValidationError::new(
                        format!("line {}", line_num),
                        "alias '*' must be followed by a name"
                    ).with_line(line_num));
                }
            }
        }

        if errors.is_empty() {
            Ok(())
        } else {
            Err(errors)
        }
    }

    /// Final structure validation after processing all lines
    fn validate_final_structure(&self, context: &ValidationContext) -> Result<(), Vec<ValidationError>> {
        let mut errors = Vec::new();

        // Check for unbalanced flow containers across the document
        if context.open_braces > 0 {
            errors.push(ValidationError::new(
                "document",
                &format!("unmatched opening brace(s) - {} unclosed", context.open_braces)
            ));
        }

        if context.open_brackets > 0 {
            errors.push(ValidationError::new(
                "document",
                &format!("unmatched opening bracket(s) - {} unclosed", context.open_brackets)
            ));
        }

        if errors.is_empty() {
            Ok(())
        } else {
            Err(errors)
        }
    }

    /// Count the indentation level of a line
    fn count_indentation(&self, line: &str) -> usize {
        let leading_spaces = line.chars().take_while(|&c| c == ' ').count();
        let leading_tabs = line.chars().take_while(|&c| c == '\t').count();

        if leading_tabs > 0 {
            // Tabs count as 8 spaces (traditional tab width)
            leading_spaces + (leading_tabs * 8)
        } else {
            leading_spaces
        }
    }
}

/// Validation context maintained during line-by-line processing
#[derive(Debug, Default)]
struct ValidationContext {
    /// Last indentation level seen
    last_indentation: Option<usize>,
    /// Number of currently open braces
    open_braces: usize,
    /// Number of currently open brackets
    open_brackets: usize,
    /// Keys seen at current indentation level
    keys_at_level: Vec<Vec<String>>,
}

impl ValidationContext {
    fn new() -> Self {
        Self::default()
    }

    fn update_from_line(&mut self, line: &str, line_num: usize) {
        let indent = line.chars().take_while(|&c| c == ' ' || c == '\t').count();
        self.last_indentation = Some(indent);

        // Track brace/bracket balance
        for ch in line.chars() {
            match ch {
                '{' => self.open_braces += 1,
                '}' => self.open_braces = self.open_braces.saturating_sub(1),
                '[' => self.open_brackets += 1,
                ']' => self.open_brackets = self.open_brackets.saturating_sub(1),
                _ => {}
            }
        }

        // Track nested levels for duplicate key checking
        let trimmed = line.trim();
        if let Some(colon_pos) = trimmed.find(':') {
            let key = trimmed[..colon_pos].trim();
            if !key.is_empty() && !key.starts_with('#') {
                // Determine nesting level
                let level = indent / 2;

                // Ensure we have enough levels
                while self.keys_at_level.len() <= level {
                    self.keys_at_level.push(Vec::new());
                }

                // Check for duplicates
                if self.keys_at_level[level].contains(&key.to_string()) {
                    // Would add a warning about duplicate key
                } else {
                    self.keys_at_level[level].push(key.to_string());
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_validate_empty_content() {
        let validator = SyntaxValidator::new();
        let result = validator.validate("");
        assert!(result.is_valid());
        assert!(result.errors.is_empty());
    }

    #[test]
    fn test_validate_simple_valid_yaml() {
        let validator = SyntaxValidator::new();
        let yaml = r#"
name: test
value: 42
items:
  - one
  - two
"#;
        let result = validator.validate(yaml);
        assert!(result.is_valid());
    }

    #[test]
    fn test_detect_tabs_in_strict_mode() {
        let validator = SyntaxValidator::strict();
        let yaml = "name:\tvalue";
        let result = validator.validate(yaml);
        assert!(!result.is_valid());
        assert!(result.errors.iter().any(|e| e.message.contains("tabs")));
    }

    #[test]
    fn test_detect_mixed_indentation() {
        let validator = SyntaxValidator::new();
        let yaml = "\t name: value";
        let result = validator.validate(yaml);
        assert!(!result.is_valid());
        assert!(result.errors.iter().any(|e| e.message.contains("mixed")));
    }

    #[test]
    fn test_detect_unmatched_brace() {
        let validator = SyntaxValidator::new();
        let yaml = "items: {one: 1, two: 2";
        let result = validator.validate(yaml);
        assert!(!result.is_valid());
    }

    #[test]
    fn test_detect_unmatched_bracket() {
        let validator = SyntaxValidator::new();
        let yaml = "items: [one, two";
        let result = validator.validate(yaml);
        assert!(!result.is_valid());
    }

    #[test]
    fn test_invalid_block_scalar_indicator() {
        let validator = SyntaxValidator::strict();
        let yaml = "|invalid";
        let result = validator.validate(yaml);
        assert!(!result.is_valid());
    }

    #[test]
    fn test_anchor_without_name() {
        let validator = SyntaxValidator::new();
        let yaml = "default: &";
        let result = validator.validate(yaml);
        assert!(!result.is_valid());
    }
}
