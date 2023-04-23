//! The fully-resolved run configuration produced by the CLI parser.

use std::fmt;

/// Which internal sorting algorithm to use.
///
/// Every variant must produce the same *set* of output lines in the same key
/// order; they differ only in stability among equal keys and in their
/// time/space characteristics (see `README.md` for the comparison table).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Algorithm {
    /// LSD/MSD byte-wise radix sort (stable, O(n·k)).
    Radix,
    /// Top-down merge sort (stable, O(n log n), basis of the external sort).
    Merge,
    /// In-place quicksort, median-of-three + insertion cutoff (unstable).
    Quick,
    /// In-place heapsort (unstable, O(n log n) worst case).
    Heap,
    /// The standard library's `sort_unstable_by` baseline.
    Default,
}

impl Algorithm {
    /// Parse the `--algorithm`/`--sort` value.
    pub fn parse(s: &str) -> Result<Algorithm, String> {
        match s.to_ascii_lowercase().as_str() {
            "radix" => Ok(Algorithm::Radix),
            "merge" => Ok(Algorithm::Merge),
            "quick" | "quicksort" => Ok(Algorithm::Quick),
            "heap" | "heapsort" => Ok(Algorithm::Heap),
            "default" | "std" => Ok(Algorithm::Default),
            other => Err(format!(
                "invalid algorithm {other:?} (expected radix|merge|quick|heap|default)"
            )),
        }
    }
}

impl fmt::Display for Algorithm {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let s = match self {
            Algorithm::Radix => "radix",
            Algorithm::Merge => "merge",
            Algorithm::Quick => "quick",
            Algorithm::Heap => "heap",
            Algorithm::Default => "default",
        };
        f.write_str(s)
    }
}

/// The resolved configuration for one invocation.
#[derive(Clone, Debug)]
pub struct Config {
    /// Input files in order; `"-"` means stdin. Empty means "stdin only".
    pub files: Vec<String>,
    /// Selected internal algorithm.
    pub algorithm: Algorithm,

    /// `-r`: reverse (descending) ordering.
    pub reverse: bool,
    /// `-n`: compare by leading numeric value.
    pub numeric: bool,
    /// `-f`: fold case before comparing.
    pub ignore_case: bool,
    /// `-b`: ignore leading blanks in the key.
    pub ignore_blanks: bool,
    /// `-R`: sort by a random hash of each line (equal lines group together).
    pub random: bool,
    /// `--random-source`: seed for `-R` (reproducible runs).
    pub random_source: Option<String>,

    /// `-u`: drop all but the first of each equal-key run.
    pub unique: bool,
    /// `-c`/`-C`: check-only mode. `Some(quiet)` where `quiet` suppresses the
    /// "disorder" diagnostic (`-C`). `None` means "sort normally".
    pub check: Option<bool>,

    /// `-o`: output file (supports in-place `-o f f`). `None` means stdout.
    pub output: Option<String>,
    /// `-S`: memory budget (bytes) before external merge sort kicks in.
    pub buffer_size: usize,
    /// `--temp-dir`: directory for external-sort run files.
    pub temp_dir: Option<String>,
}

/// Default memory budget before spilling to disk: 256 MiB.
pub const DEFAULT_BUFFER_SIZE: usize = 256 * 1024 * 1024;

impl Default for Config {
    fn default() -> Self {
        Config {
            files: Vec::new(),
            algorithm: Algorithm::Default,
            reverse: false,
            numeric: false,
            ignore_case: false,
            ignore_blanks: false,
            random: false,
            random_source: None,
            unique: false,
            check: None,
            output: None,
            buffer_size: DEFAULT_BUFFER_SIZE,
            temp_dir: None,
        }
    }
}

impl Config {
    /// Validate flag combinations after parsing. Returns a human-readable error
    /// for conflicting options.
    pub fn validate(&self) -> Result<(), String> {
        if self.numeric && self.random {
            return Err("options -n/--numeric-sort and -R/--random-sort are mutually exclusive".into());
        }
        if self.buffer_size == 0 {
            return Err("buffer size (-S) must be greater than zero".into());
        }
        Ok(())
    }
}
