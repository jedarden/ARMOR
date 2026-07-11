//! Parser configuration module
//!
//! This module provides comprehensive configuration options for controlling
//! parsing behavior across different parser implementations. It defines:
//!
//! - [`ParserConfig`] - Main configuration struct for all parsing options
//! - [`ParserMode`] - Strict vs lenient parsing modes
//! - [`TypeConstructor`] - Custom hooks for type construction
//! - Builder pattern for fluent configuration

use std::collections::HashMap;
use std::fmt;

/// Parser execution mode
///
/// Defines the strictness level for parsing operations. Each mode has
/// specific behaviors for handling malformed input, unknown fields, and
/// type mismatches.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ParserMode {
    /// Strict mode - reject any malformed or unexpected input
    ///
    /// In strict mode:
    /// - Unknown fields cause parsing to fail
    /// - Type mismatches are errors (not coerced)
    /// - Duplicate keys are rejected
    /// - All syntax rules are enforced
    /// - Missing required fields cause errors
    Strict,

    /// Lenient mode - attempt to recover from errors
    ///
    /// In lenient mode:
    /// - Unknown fields are silently ignored
    /// - Type mismatches are coerced when possible (e.g., string → number)
    /// - Last duplicate key wins (with warning if configured)
    /// - Some syntax variations are accepted
    /// - Missing optional fields use defaults
    Lenient,
}

impl ParserMode {
    /// Check if this mode is strict
    pub fn is_strict(&self) -> bool {
        matches!(self, Self::Strict)
    }

    /// Check if this mode is lenient
    pub fn is_lenient(&self) -> bool {
        matches!(self, Self::Lenient)
    }
}

impl fmt::Display for ParserMode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Strict => write!(f, "strict"),
            Self::Lenient => write!(f, "lenient"),
        }
    }
}

impl Default for ParserMode {
    fn default() -> Self {
        // Default to lenient mode for better user experience
        Self::Lenient
    }
}

/// Type constructor function signature
///
/// This function type defines the signature for custom type constructors.
/// It takes a field name and raw value, and attempts to construct the
/// target type from those inputs.
pub type TypeConstructorFn = fn(&str, &serde_yaml::Value) -> Result<serde_yaml::Value, String>;

/// Custom type constructor hook
///
/// Allows users to register custom logic for constructing specific types
/// during parsing. This is useful for:
///
/// - Custom enum parsing (e.g., "warn" → LogLevel::Warning)
/// - Validation-rich construction (e.g., ensure ports are in range)
/// - Complex type assembly (e.g., Duration from "5s" string)
/// - Default value injection for optional fields
///
/// # Examples
///
/// ```
/// use armor::parsers::config::TypeConstructor;
///
/// // Custom constructor for log levels
/// fn log_level_constructor(
///     field: &str,
///     value: &serde_yaml::Value
/// ) -> Result<serde_yaml::Value, String> {
///     let s = value.as_str()
///         .ok_or("expected string")?
///         .to_lowercase();
///
///     let level = match s.as_str() {
///         "debug" => 0,
///         "info" => 1,
///         "warn" | "warning" => 2,
///         "error" => 3,
///         _ => return Err(format!("invalid log level: {}", s)),
///     };
///
///     Ok(serde_yaml::Value::Number(level.into()))
/// }
///
/// let constructor = TypeConstructor::new("LogLevel", log_level_constructor);
/// ```
#[derive(Clone)]
pub struct TypeConstructor {
    /// Name of the type this constructor handles (for debugging)
    pub type_name: String,
    /// The constructor function
    pub constructor: TypeConstructorFn,
}

impl TypeConstructor {
    /// Create a new type constructor
    ///
    /// # Arguments
    ///
    /// * `type_name` - Human-readable name for the type being constructed
    /// * `constructor` - Function that implements the construction logic
    pub fn new(type_name: impl Into<String>, constructor: TypeConstructorFn) -> Self {
        Self {
            type_name: type_name.into(),
            constructor,
        }
    }

    /// Invoke the constructor
    ///
    /// Calls the underlying constructor function with the provided inputs.
    ///
    /// # Arguments
    ///
    /// * `field` - Field name being constructed
    /// * `value` - Raw value from the parser
    ///
    /// # Returns
    ///
    /// * `Ok(serde_yaml::Value)` - Successfully constructed value
    /// * `Err(String)` - Construction error message
    pub fn construct(&self, field: &str, value: &serde_yaml::Value) -> Result<serde_yaml::Value, String> {
        (self.constructor)(field, value)
    }
}

impl fmt::Debug for TypeConstructor {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("TypeConstructor")
            .field("type_name", &self.type_name)
            .field("constructor", &"<function>")
            .finish()
    }
}

/// Validation hook function signature
///
/// This function type defines the signature for custom validation hooks.
/// It takes a field name and parsed value, and validates that the value
/// meets application-specific constraints.
pub type ValidationFn = fn(&str, &serde_yaml::Value) -> Result<(), String>;

/// Custom validation hook
///
/// Allows users to register custom validation logic for specific fields
/// or types. Validation runs after parsing but before the final result
/// is returned.
///
/// # Examples
///
/// ```
/// use armor::parsers::config::ValidationHook;
///
/// // Validate port numbers are in valid range
/// fn validate_port(
///     field: &str,
///     value: &serde_yaml::Value
/// ) -> Result<(), String> {
///     let port = value.as_i64()
///         .ok_or("port must be an integer")?;
///
///     if !(1..=65535).contains(&port) {
///         return Err(format!("port {} out of valid range (1-65535)", port));
///     }
///
///     Ok(())
/// }
///
/// let hook = ValidationHook::new("port", validate_port);
/// ```
#[derive(Clone)]
pub struct ValidationHook {
    /// Pattern for matching field names (supports simple "*"" wildcard)
    pub field_pattern: String,
    /// The validation function
    pub validator: ValidationFn,
}

impl ValidationHook {
    /// Create a new validation hook
    ///
    /// # Arguments
    ///
    /// * `field_pattern` - Pattern to match field names (supports "*"" wildcard)
    /// * `validator` - Function that implements the validation logic
    pub fn new(field_pattern: impl Into<String>, validator: ValidationFn) -> Self {
        Self {
            field_pattern: field_pattern.into(),
            validator,
        }
    }

    /// Check if this hook applies to the given field
    ///
    /// # Arguments
    ///
    /// * `field` - Field name to check
    ///
    /// # Returns
    ///
    /// `true` if the field pattern matches, `false` otherwise
    pub fn applies_to(&self, field: &str) -> bool {
        if self.field_pattern == "*" {
            return true;
        }

        if self.field_pattern.ends_with('*') {
            let prefix = &self.field_pattern[..self.field_pattern.len() - 1];
            return field.starts_with(prefix);
        }

        self.field_pattern == field
    }

    /// Invoke the validator
    ///
    /// Calls the underlying validation function with the provided inputs.
    ///
    /// # Arguments
    ///
    /// * `field` - Field name being validated
    /// * `value` - Parsed value to validate
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Validation passed
    /// * `Err(String)` - Validation error message
    pub fn validate(&self, field: &str, value: &serde_yaml::Value) -> Result<(), String> {
        (self.validator)(field, value)
    }
}

impl fmt::Debug for ValidationHook {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ValidationHook")
            .field("field_pattern", &self.field_pattern)
            .field("validator", &"<function>")
            .finish()
    }
}

/// Comprehensive parser configuration
///
/// This structure consolidates all configuration options for parsing behavior.
/// It provides fine-grained control over parsing, validation, and type construction.
///
/// # Default Configuration
///
/// The default configuration is:
///
/// ```ignore
/// ParserConfig {
///     mode: Lenient,
///     allow_duplicates: true,
///     preserve_comments: false,
///     preserve_quotes: false,
///     max_depth: 0,  // unlimited
///     strict_types: false,
///     .. }
/// ```
///
/// # Examples
///
/// ## Using Defaults
///
/// ```
/// use armor::parsers::config::ParserConfig;
///
/// let config = ParserConfig::default();
/// ```
///
/// ## Strict Mode
///
/// ```
/// use armor::parsers::config::ParserConfig;
///
/// let config = ParserConfig::strict();
/// ```
///
/// ## Builder Pattern
///
/// ```ignore
/// use armor::parsers::config::{ParserConfig, ParserMode};
///
/// let config = ParserConfig::builder()
///     .mode(ParserMode::Strict)
///     .allow_duplicates(false)
///     .max_depth(10)
///     .build();
/// ```
#[derive(Debug, Clone)]
pub struct ParserConfig {
    /// Parsing mode (strict vs lenient)
    pub mode: ParserMode,
    /// Allow duplicate keys in mappings
    pub allow_duplicates: bool,
    /// Preserve comments in the output (if format supports it)
    pub preserve_comments: bool,
    /// Preserve quote information in parsed strings
    pub preserve_quotes: bool,
    /// Preserve order of mapping keys (use index-based map instead of hash map)
    pub preserve_order: bool,
    /// Maximum nesting depth (0 = unlimited)
    pub max_depth: usize,
    /// Enforce strict type checking (no implicit coercion)
    pub strict_types: bool,
    /// Custom type constructors registered by field name
    pub type_constructors: HashMap<String, TypeConstructor>,
    /// Custom validation hooks
    pub validation_hooks: Vec<ValidationHook>,
    /// Emit warnings for recoverable errors
    pub emit_warnings: bool,
    /// Treat warnings as errors (fail on warnings)
    pub warnings_as_errors: bool,
}

impl Default for ParserConfig {
    fn default() -> Self {
        Self {
            mode: ParserMode::default(),
            allow_duplicates: true,
            preserve_comments: false,
            preserve_quotes: false,
            preserve_order: false,
            max_depth: 0,
            strict_types: false,
            type_constructors: HashMap::new(),
            validation_hooks: Vec::new(),
            emit_warnings: false,
            warnings_as_errors: false,
        }
    }
}

impl ParserConfig {
    /// Create a builder for constructing ParserConfig
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::{ParserConfig, ParserMode};
    ///
    /// let config = ParserConfig::builder()
    ///     .mode(ParserMode::Strict)
    ///     .build();
    /// ```
    pub fn builder() -> ParserConfigBuilder {
        ParserConfigBuilder::new()
    }

    /// Create a strict-mode configuration
    ///
    /// This is a convenience method that creates a configuration optimized
    /// for strict validation. Equivalent to:
    ///
    /// ```ignore
    /// ParserConfig::builder()
    ///     .mode(ParserMode::Strict)
    ///     .allow_duplicates(false)
    ///     .strict_types(true)
    ///     .build()
    /// ```
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::ParserConfig;
    ///
    /// let config = ParserConfig::strict();
    /// assert!(config.mode.is_strict());
    /// assert!(!config.allow_duplicates);
    /// assert!(config.strict_types);
    /// ```
    pub fn strict() -> Self {
        Self {
            mode: ParserMode::Strict,
            allow_duplicates: false,
            strict_types: true,
            ..Default::default()
        }
    }

    /// Create a lenient-mode configuration
    ///
    /// This is a convenience method that creates a configuration optimized
    /// for forgiving parsing. Equivalent to:
    ///
    /// ```ignore
    /// ParserConfig::builder()
    ///     .mode(ParserMode::Lenient)
    ///     .allow_duplicates(true)
    ///     .strict_types(false)
    ///     .build()
    /// ```
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::ParserConfig;
    ///
    /// let config = ParserConfig::lenient();
    /// assert!(config.mode.is_lenient());
    /// assert!(config.allow_duplicates);
    /// assert!(!config.strict_types);
    /// ```
    pub fn lenient() -> Self {
        Self {
            mode: ParserMode::Lenient,
            allow_duplicates: true,
            strict_types: false,
            ..Default::default()
        }
    }

    /// Check if strict mode is enabled
    pub fn is_strict(&self) -> bool {
        self.mode.is_strict()
    }

    /// Check if lenient mode is enabled
    pub fn is_lenient(&self) -> bool {
        self.mode.is_lenient()
    }

    /// Register a custom type constructor
    ///
    /// # Arguments
    ///
    /// * `field` - Field name this constructor handles
    /// * `constructor` - The type constructor to register
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::{ParserConfig, TypeConstructor};
    ///
    /// fn make_duration(_: &str, v: &serde_yaml::Value) -> Result<serde_yaml::Value, String> {
    ///     // Implementation...
    ///     Ok(v.clone())
    /// }
    ///
    /// let mut config = ParserConfig::default();
    /// config.register_constructor("timeout", TypeConstructor::new("Duration", make_duration));
    /// ```
    pub fn register_constructor(&mut self, field: impl Into<String>, constructor: TypeConstructor) {
        self.type_constructors.insert(field.into(), constructor);
    }

    /// Register a custom validation hook
    ///
    /// # Arguments
    ///
    /// * `hook` - The validation hook to register
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::{ParserConfig, ValidationHook};
    ///
    /// fn validate_port(field: &str, v: &serde_yaml::Value) -> Result<(), String> {
    ///     // Implementation...
    ///     Ok(())
    /// }
    ///
    /// let mut config = ParserConfig::default();
    /// config.register_validation(ValidationHook::new("port", validate_port));
    /// ```
    pub fn register_validation(&mut self, hook: ValidationHook) {
        self.validation_hooks.push(hook);
    }

    /// Get type constructor for a field (if registered)
    ///
    /// # Arguments
    ///
    /// * `field` - Field name to look up
    ///
    /// # Returns
    ///
    /// `Some(&TypeConstructor)` if registered, `None` otherwise
    pub fn get_constructor(&self, field: &str) -> Option<&TypeConstructor> {
        self.type_constructors.get(field)
    }

    /// Get all applicable validation hooks for a field
    ///
    /// # Arguments
    ///
    /// * `field` - Field name to check
    ///
    /// # Returns
    ///
    /// Iterator over all validation hooks that apply to this field
    pub fn get_validations<'a>(&'a self, field: &'a str) -> impl Iterator<Item = &'a ValidationHook> {
        self.validation_hooks
            .iter()
            .filter(move |hook| hook.applies_to(field))
    }

    /// Validate configuration for mutually exclusive or inconsistent options
    ///
    /// This method checks if any mutually exclusive options are set together
    /// or if the configuration has inconsistent settings. Returns an error if
    /// validation fails.
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Configuration is valid
    /// * `Err(String)` - Configuration has conflicting or inconsistent options
    pub fn validate(&self) -> Result<(), String> {
        // Check for mutually exclusive or inconsistent options
        if self.warnings_as_errors && !self.emit_warnings {
            return Err("warnings_as_errors requires emit_warnings to be true".to_string());
        }

        // Strict mode should not allow duplicates
        if self.mode.is_strict() && self.allow_duplicates {
            return Err("Strict mode with allow_duplicates=true is inconsistent".to_string());
        }

        // Strict types should align with strict mode
        if self.strict_types && self.mode.is_lenient() {
            return Err("strict_types=true with lenient mode is inconsistent".to_string());
        }

        Ok(())
    }
}

/// Builder for constructing ParserConfig instances
///
/// Provides a fluent interface for creating parser configurations.
/// Use [`ParserConfig::builder()`] to create an instance.
///
/// # Examples
///
/// ```
/// use armor::parsers::config::{ParserConfig, ParserMode};
///
/// let config = ParserConfig::builder()
///     .mode(ParserMode::Strict)
///     .allow_duplicates(false)
///     .max_depth(10)
///     .preserve_comments(true)
///     .build();
/// ```
#[derive(Debug, Clone)]
pub struct ParserConfigBuilder {
    config: ParserConfig,
}

impl ParserConfigBuilder {
    /// Create a new builder with default configuration
    fn new() -> Self {
        Self {
            config: ParserConfig::default(),
        }
    }

    /// Set the parsing mode
    pub fn mode(mut self, mode: ParserMode) -> Self {
        self.config.mode = mode;
        self
    }

    /// Set whether to allow duplicate keys
    pub fn allow_duplicates(mut self, allow: bool) -> Self {
        self.config.allow_duplicates = allow;
        self
    }

    /// Set whether to preserve comments
    pub fn preserve_comments(mut self, preserve: bool) -> Self {
        self.config.preserve_comments = preserve;
        self
    }

    /// Set whether to preserve quote information
    pub fn preserve_quotes(mut self, preserve: bool) -> Self {
        self.config.preserve_quotes = preserve;
        self
    }

    /// Set whether to preserve mapping key order
    pub fn preserve_order(mut self, preserve: bool) -> Self {
        self.config.preserve_order = preserve;
        self
    }

    /// Set maximum nesting depth
    pub fn max_depth(mut self, depth: usize) -> Self {
        self.config.max_depth = depth;
        self
    }

    /// Set strict type checking
    pub fn strict_types(mut self, strict: bool) -> Self {
        self.config.strict_types = strict;
        self
    }

    /// Add a type constructor
    pub fn with_constructor(mut self, field: impl Into<String>, constructor: TypeConstructor) -> Self {
        self.config.register_constructor(field, constructor);
        self
    }

    /// Add a validation hook
    pub fn with_validation(mut self, hook: ValidationHook) -> Self {
        self.config.register_validation(hook);
        self
    }

    /// Set whether to emit warnings
    pub fn emit_warnings(mut self, emit: bool) -> Self {
        self.config.emit_warnings = emit;
        self
    }

    /// Set whether to treat warnings as errors
    pub fn warnings_as_errors(mut self, treat_as_errors: bool) -> Self {
        self.config.warnings_as_errors = treat_as_errors;
        self
    }

    /// Build the final ParserConfig
    ///
    /// This method validates the configuration before returning it.
    ///
    /// # Returns
    ///
    /// * `Ok(ParserConfig)` - Valid configuration
    /// * `Err(String)` - Configuration validation failed
    pub fn build(self) -> Result<ParserConfig, String> {
        self.config.validate()?;
        Ok(self.config)
    }

    /// Build without validation (internal use only)
    #[doc(hidden)]
    pub fn build_unchecked(self) -> ParserConfig {
        self.config
    }
}

/// Validation mode for field checking
///
/// Defines the strictness level for validation operations. Each mode has
/// specific behaviors for field presence and type checking.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ValidationMode {
    /// Strict mode - enforce all validation rules
    ///
    /// In strict mode:
    /// - All required fields must be present
    /// - Unknown fields cause validation to fail
    /// - Type mismatches are errors
    Strict,

    /// Permissive mode - allow some flexibility
    ///
    /// In permissive mode:
    /// - Required fields use defaults if missing
    /// - Unknown fields are ignored with warnings
    /// - Type mismatches are coerced when possible
    Permissive,
}

impl ValidationMode {
    /// Check if this mode is strict
    pub fn is_strict(&self) -> bool {
        matches!(self, Self::Strict)
    }

    /// Check if this mode is permissive
    pub fn is_permissive(&self) -> bool {
        matches!(self, Self::Permissive)
    }
}

impl fmt::Display for ValidationMode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Strict => write!(f, "strict"),
            Self::Permissive => write!(f, "permissive"),
        }
    }
}

impl Default for ValidationMode {
    fn default() -> Self {
        Self::Permissive
    }
}

/// Comprehensive validator configuration
///
/// This structure consolidates all configuration options for validation behavior.
/// It provides fine-grained control over field checking, type validation, and
/// constraint enforcement.
///
/// # Default Configuration
///
/// The default configuration is:
///
/// ```ignore
/// ValidatorConfig {
///     mode: Permissive,
///     require_all_fields: false,
///     disallow_unknown_fields: false,
///     type_checking: true,
///     .. }
/// ```
///
/// # Examples
///
/// ## Using Defaults
///
/// ```
/// use armor::parsers::config::ValidatorConfig;
///
/// let config = ValidatorConfig::default();
/// ```
///
/// ## Strict Mode
///
/// ```
/// use armor::parsers::config::ValidatorConfig;
///
/// let config = ValidatorConfig::strict();
/// ```
///
/// ## Builder Pattern
///
/// ```ignore
/// use armor::parsers::config::{ValidatorConfig, ValidationMode};
///
/// let config = ValidatorConfig::builder()
///     .mode(ValidationMode::Strict)
///     .require_all_fields(true)
///     .disallow_unknown_fields(true)
///     .build();
/// ```
#[derive(Debug, Clone)]
pub struct ValidatorConfig {
    /// Validation mode (strict vs permissive)
    pub mode: ValidationMode,
    /// Require all fields to be present (no missing fields)
    pub require_all_fields: bool,
    /// Disallow unknown fields (fail on unexpected fields)
    pub disallow_unknown_fields: bool,
    /// Enable type checking (validate field types match schema)
    pub type_checking: bool,
    /// Perform deep validation on nested structures
    pub deep_validation: bool,
    /// Emit warnings for non-critical issues
    pub emit_warnings: bool,
    /// Treat warnings as errors (fail on warnings)
    pub warnings_as_errors: bool,
}

impl Default for ValidatorConfig {
    fn default() -> Self {
        Self {
            mode: ValidationMode::default(),
            require_all_fields: false,
            disallow_unknown_fields: false,
            type_checking: true,
            deep_validation: true,
            emit_warnings: true,
            warnings_as_errors: false,
        }
    }
}

impl ValidatorConfig {
    /// Create a builder for constructing ValidatorConfig
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::{ValidatorConfig, ValidationMode};
    ///
    /// let config = ValidatorConfig::builder()
    ///     .mode(ValidationMode::Strict)
    ///     .build();
    /// ```
    pub fn builder() -> ValidatorConfigBuilder {
        ValidatorConfigBuilder::new()
    }

    /// Create a strict-mode configuration
    ///
    /// This is a convenience method that creates a configuration optimized
    /// for strict validation. Equivalent to:
    ///
    /// ```ignore
    /// ValidatorConfig::builder()
    ///     .mode(ValidationMode::Strict)
    ///     .require_all_fields(true)
    ///     .disallow_unknown_fields(true)
    ///     .type_checking(true)
    ///     .build()
    /// ```
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::ValidatorConfig;
    ///
    /// let config = ValidatorConfig::strict();
    /// assert!(config.mode.is_strict());
    /// assert!(config.require_all_fields);
    /// assert!(config.disallow_unknown_fields);
    /// assert!(config.type_checking);
    /// ```
    pub fn strict() -> Self {
        Self {
            mode: ValidationMode::Strict,
            require_all_fields: true,
            disallow_unknown_fields: true,
            type_checking: true,
            ..Default::default()
        }
    }

    /// Create a permissive-mode configuration
    ///
    /// This is a convenience method that creates a configuration optimized
    /// for flexible validation. Equivalent to:
    ///
    /// ```ignore
    /// ValidatorConfig::builder()
    ///     .mode(ValidationMode::Permissive)
    ///     .require_all_fields(false)
    ///     .disallow_unknown_fields(false)
    ///     .type_checking(true)
    ///     .build()
    /// ```
    ///
    /// # Examples
    ///
    /// ```
    /// use armor::parsers::config::ValidatorConfig;
    ///
    /// let config = ValidatorConfig::permissive();
    /// assert!(config.mode.is_permissive());
    /// assert!(!config.require_all_fields);
    /// assert!(!config.disallow_unknown_fields);
    /// assert!(config.type_checking);
    /// ```
    pub fn permissive() -> Self {
        Self {
            mode: ValidationMode::Permissive,
            require_all_fields: false,
            disallow_unknown_fields: false,
            type_checking: true,
            ..Default::default()
        }
    }

    /// Check if strict mode is enabled
    pub fn is_strict(&self) -> bool {
        self.mode.is_strict()
    }

    /// Check if permissive mode is enabled
    pub fn is_permissive(&self) -> bool {
        self.mode.is_permissive()
    }

    /// Validate configuration for mutually exclusive options
    ///
    /// This method checks if any mutually exclusive options are set together.
    /// Returns an error if validation fails.
    ///
    /// # Returns
    ///
    /// * `Ok(())` - Configuration is valid
    /// * `Err(String)` - Configuration has conflicting options
    pub fn validate(&self) -> Result<(), String> {
        // Check for mutually exclusive options
        if self.mode.is_strict() && !self.require_all_fields {
            return Err("Strict mode requires require_all_fields to be true".to_string());
        }

        if self.mode.is_strict() && !self.disallow_unknown_fields {
            return Err("Strict mode requires disallow_unknown_fields to be true".to_string());
        }

        if self.warnings_as_errors && !self.emit_warnings {
            return Err("warnings_as_errors requires emit_warnings to be true".to_string());
        }

        Ok(())
    }
}

/// Builder for constructing ValidatorConfig instances
///
/// Provides a fluent interface for creating validator configurations.
/// Use [`ValidatorConfig::builder()`] to create an instance.
///
/// # Examples
///
/// ```
/// use armor::parsers::config::{ValidatorConfig, ValidationMode};
///
/// let config = ValidatorConfig::builder()
///     .mode(ValidationMode::Strict)
///     .require_all_fields(true)
///     .disallow_unknown_fields(true)
///     .type_checking(true)
///     .build();
/// ```
#[derive(Debug, Clone)]
pub struct ValidatorConfigBuilder {
    config: ValidatorConfig,
}

impl ValidatorConfigBuilder {
    /// Create a new builder with default configuration
    fn new() -> Self {
        Self {
            config: ValidatorConfig::default(),
        }
    }

    /// Set the validation mode
    pub fn mode(mut self, mode: ValidationMode) -> Self {
        self.config.mode = mode;
        self
    }

    /// Set whether to require all fields
    pub fn require_all_fields(mut self, require: bool) -> Self {
        self.config.require_all_fields = require;
        self
    }

    /// Set whether to disallow unknown fields
    pub fn disallow_unknown_fields(mut self, disallow: bool) -> Self {
        self.config.disallow_unknown_fields = disallow;
        self
    }

    /// Set type checking
    pub fn type_checking(mut self, enabled: bool) -> Self {
        self.config.type_checking = enabled;
        self
    }

    /// Set deep validation on nested structures
    pub fn deep_validation(mut self, enabled: bool) -> Self {
        self.config.deep_validation = enabled;
        self
    }

    /// Set whether to emit warnings
    pub fn emit_warnings(mut self, emit: bool) -> Self {
        self.config.emit_warnings = emit;
        self
    }

    /// Set whether to treat warnings as errors
    pub fn warnings_as_errors(mut self, treat_as_errors: bool) -> Self {
        self.config.warnings_as_errors = treat_as_errors;
        self
    }

    /// Build the final ValidatorConfig
    ///
    /// This method validates the configuration before returning it.
    ///
    /// # Returns
    ///
    /// * `Ok(ValidatorConfig)` - Valid configuration
    /// * `Err(String)` - Configuration validation failed
    pub fn build(self) -> Result<ValidatorConfig, String> {
        self.config.validate()?;
        Ok(self.config)
    }

    /// Build without validation (internal use only)
    #[doc(hidden)]
    pub fn build_unchecked(self) -> ValidatorConfig {
        self.config
    }
}

/// Create default parser configuration
///
/// This is a convenience function that creates a `ParserConfig` with default values.
/// Equivalent to `ParserConfig::default()`.
///
/// # Examples
///
/// ```
/// use armor::parsers::config::default_parser_config;
///
/// let config = default_parser_config();
/// assert!(config.is_lenient());
/// ```
pub fn default_parser_config() -> ParserConfig {
    ParserConfig::default()
}

/// Create default validator configuration
///
/// This is a convenience function that creates a `ValidatorConfig` with default values.
/// Equivalent to `ValidatorConfig::default()`.
///
/// # Examples
///
/// ```
/// use armor::parsers::config::default_validator_config;
///
/// let config = default_validator_config();
/// assert!(config.is_permissive());
/// ```
pub fn default_validator_config() -> ValidatorConfig {
    ValidatorConfig::default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_yaml::Value;

    #[test]
    fn test_parser_mode_display() {
        assert_eq!(ParserMode::Strict.to_string(), "strict");
        assert_eq!(ParserMode::Lenient.to_string(), "lenient");
    }

    #[test]
    fn test_parser_mode_checks() {
        assert!(ParserMode::Strict.is_strict());
        assert!(!ParserMode::Strict.is_lenient());
        assert!(ParserMode::Lenient.is_lenient());
        assert!(!ParserMode::Lenient.is_strict());
    }

    #[test]
    fn test_parser_mode_default() {
        assert_eq!(ParserMode::default(), ParserMode::Lenient);
    }

    #[test]
    fn test_type_constructor() {
        fn simple_constructor(_: &str, value: &Value) -> Result<Value, String> {
            Ok(value.clone())
        }

        let constructor = TypeConstructor::new("TestType", simple_constructor);
        assert_eq!(constructor.type_name, "TestType");

        let input = Value::Number(42.into());
        let result = constructor.construct("field", &input).unwrap();
        assert_eq!(result, input);
    }

    #[test]
    fn test_validation_hook_pattern_matching() {
        fn noop(_: &str, _: &Value) -> Result<(), String> {
            Ok(())
        }

        let hook = ValidationHook::new("port_*", noop);
        assert!(hook.applies_to("port_http"));
        assert!(hook.applies_to("port_https"));
        assert!(!hook.applies_to("timeout"));

        let universal_hook = ValidationHook::new("*", noop);
        assert!(universal_hook.applies_to("any_field"));
    }

    #[test]
    fn test_parser_config_default() {
        let config = ParserConfig::default();
        assert!(config.is_lenient());
        assert!(config.allow_duplicates);
        assert!(!config.preserve_comments);
        assert!(!config.preserve_quotes);
        assert!(!config.preserve_order);
        assert_eq!(config.max_depth, 0);
        assert!(!config.strict_types);
    }

    #[test]
    fn test_parser_config_strict() {
        let config = ParserConfig::strict();
        assert!(config.is_strict());
        assert!(!config.allow_duplicates);
        assert!(config.strict_types);
    }

    #[test]
    fn test_parser_config_lenient() {
        let config = ParserConfig::lenient();
        assert!(config.is_lenient());
        assert!(config.allow_duplicates);
        assert!(!config.strict_types);
    }

    #[test]
    fn test_parser_config_register_constructor() {
        fn dummy_constructor(_: &str, _: &Value) -> Result<Value, String> {
            Ok(Value::Null)
        }

        let mut config = ParserConfig::default();
        config.register_constructor("timeout", TypeConstructor::new("Duration", dummy_constructor));

        assert!(config.get_constructor("timeout").is_some());
        assert!(config.get_constructor("unknown").is_none());
    }

    #[test]
    fn test_parser_config_register_validation() {
        fn dummy_validation(_: &str, _: &Value) -> Result<(), String> {
            Ok(())
        }

        let mut config = ParserConfig::default();
        config.register_validation(ValidationHook::new("port_*", dummy_validation));

        let hooks: Vec<_> = config.get_validations("port_http").collect();
        assert_eq!(hooks.len(), 1);
    }

    #[test]
    fn test_parser_config_builder() {
        let config = ParserConfig::builder()
            .mode(ParserMode::Strict)
            .allow_duplicates(false)
            .max_depth(10)
            .preserve_comments(true)
            .preserve_order(true)
            .strict_types(true)
            .build()
            .unwrap();

        assert!(config.is_strict());
        assert!(!config.allow_duplicates);
        assert_eq!(config.max_depth, 10);
        assert!(config.preserve_comments);
        assert!(config.preserve_order);
        assert!(config.strict_types);
    }

    #[test]
    fn test_parser_config_builder_with_hooks() {
        fn constructor(_: &str, v: &Value) -> Result<Value, String> {
            Ok(v.clone())
        }

        fn validation(_: &str, _: &Value) -> Result<(), String> {
            Ok(())
        }

        let config = ParserConfig::builder()
            .with_constructor("timeout", TypeConstructor::new("Duration", constructor))
            .with_validation(ValidationHook::new("port", validation))
            .build()
            .unwrap();

        assert!(config.get_constructor("timeout").is_some());
        assert_eq!(config.get_validations("port").count(), 1);
    }

    #[test]
    fn test_parser_config_validate() {
        let config = ParserConfig::default();
        assert!(config.validate().is_ok());

        let strict_config = ParserConfig::strict();
        assert!(strict_config.validate().is_ok());
    }

    #[test]
    fn test_parser_config_validate_errors() {
        // warnings_as_errors without emit_warnings should fail
        let config = ParserConfig {
            emit_warnings: false,
            warnings_as_errors: true,
            ..Default::default()
        };
        assert!(config.validate().is_err());

        // Strict mode with allow_duplicates should fail
        let config2 = ParserConfig {
            mode: ParserMode::Strict,
            allow_duplicates: true,
            ..Default::default()
        };
        assert!(config2.validate().is_err());

        // strict_types with lenient mode should fail
        let config3 = ParserConfig {
            mode: ParserMode::Lenient,
            strict_types: true,
            ..Default::default()
        };
        assert!(config3.validate().is_err());
    }

    #[test]
    fn test_parser_config_builder_validation_error() {
        let result = ParserConfig::builder()
            .mode(ParserMode::Strict)
            .allow_duplicates(true)  // Invalid for strict mode
            .build();

        assert!(result.is_err());
    }

    #[test]
    fn test_validation_mode_display() {
        assert_eq!(ValidationMode::Strict.to_string(), "strict");
        assert_eq!(ValidationMode::Permissive.to_string(), "permissive");
    }

    #[test]
    fn test_validation_mode_checks() {
        assert!(ValidationMode::Strict.is_strict());
        assert!(!ValidationMode::Strict.is_permissive());
        assert!(ValidationMode::Permissive.is_permissive());
        assert!(!ValidationMode::Permissive.is_strict());
    }

    #[test]
    fn test_validation_mode_default() {
        assert_eq!(ValidationMode::default(), ValidationMode::Permissive);
    }

    #[test]
    fn test_validator_config_default() {
        let config = ValidatorConfig::default();
        assert!(config.is_permissive());
        assert!(!config.require_all_fields);
        assert!(!config.disallow_unknown_fields);
        assert!(config.type_checking);
        assert!(config.deep_validation);
        assert!(config.emit_warnings);
        assert!(!config.warnings_as_errors);
    }

    #[test]
    fn test_validator_config_strict() {
        let config = ValidatorConfig::strict();
        assert!(config.is_strict());
        assert!(config.require_all_fields);
        assert!(config.disallow_unknown_fields);
        assert!(config.type_checking);
    }

    #[test]
    fn test_validator_config_permissive() {
        let config = ValidatorConfig::permissive();
        assert!(config.is_permissive());
        assert!(!config.require_all_fields);
        assert!(!config.disallow_unknown_fields);
        assert!(config.type_checking);
    }

    #[test]
    fn test_validator_config_validate() {
        let config = ValidatorConfig::default();
        assert!(config.validate().is_ok());

        let strict_config = ValidatorConfig::strict();
        assert!(strict_config.validate().is_ok());
    }

    #[test]
    fn test_validator_config_validate_errors() {
        // Strict mode without require_all_fields should fail
        let config = ValidatorConfig {
            mode: ValidationMode::Strict,
            require_all_fields: false,
            disallow_unknown_fields: true,
            ..Default::default()
        };
        assert!(config.validate().is_err());

        // warnings_as_errors without emit_warnings should fail
        let config2 = ValidatorConfig {
            emit_warnings: false,
            warnings_as_errors: true,
            ..Default::default()
        };
        assert!(config2.validate().is_err());
    }

    #[test]
    fn test_validator_config_builder() {
        let config = ValidatorConfig::builder()
            .mode(ValidationMode::Strict)
            .require_all_fields(true)
            .disallow_unknown_fields(true)
            .type_checking(true)
            .build()
            .unwrap();

        assert!(config.is_strict());
        assert!(config.require_all_fields);
        assert!(config.disallow_unknown_fields);
        assert!(config.type_checking);
    }

    #[test]
    fn test_validator_config_builder_validation_error() {
        let result = ValidatorConfig::builder()
            .mode(ValidationMode::Strict)
            .require_all_fields(false)  // Invalid for strict mode
            .build();

        assert!(result.is_err());
    }

    #[test]
    fn test_default_parser_config() {
        let config = default_parser_config();
        assert!(config.is_lenient());
        assert!(config.allow_duplicates);
    }

    #[test]
    fn test_default_validator_config() {
        let config = default_validator_config();
        assert!(config.is_permissive());
        assert!(config.type_checking);
    }
}
