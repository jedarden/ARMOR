//! Parser modules for ARMOR
//!
//! This module defines generic parsing interfaces and implementations
//! for different data formats.

pub mod config;
pub mod yaml;

mod traits;
pub use config::{
    ParserConfig, ParserMode, ParserConfigBuilder, TypeConstructor, ValidationHook,
    ValidatorConfig, ValidationMode, ValidatorConfigBuilder,
    default_parser_config, default_validator_config,
};
pub use traits::{Parser, ParseOptions, ParseMetadata, StreamingParser, IncrementalParser};
