//! Parser modules for ARMOR
//!
//! This module defines generic parsing interfaces and implementations
//! for different data formats.

pub mod yaml;

mod traits;
pub use traits::{Parser, ParseOptions, ParseMetadata, StreamingParser, IncrementalParser};
