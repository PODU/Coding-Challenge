//! `ccsort`, a clone of the Unix `sort` utility (Coding Challenge #6).
//!
//! Parse a [`Config`] with [`cli::parse`] and run it with [`engine::run`]. The
//! algorithms and helpers are public too so the tests can exercise them
//! directly.

pub mod algo;
pub mod check;
pub mod cli;
pub mod cmp;
pub mod config;
pub mod engine;
pub mod external;
pub mod key;
pub mod output;
pub mod random;
pub mod reader;
pub mod record;
pub mod unique;

pub use config::{Algorithm, Config};
pub use record::Record;

/// Process-level exit codes, matching GNU `sort` semantics.
pub mod exit {
    /// Success (or, with `-c`, the input was already sorted).
    pub const OK: i32 = 0;
    /// With `-c`/`-C`: the input was NOT sorted.
    pub const DISORDER: i32 = 1;
    /// An error occurred (bad option, I/O failure, ...).
    pub const ERROR: i32 = 2;
}
