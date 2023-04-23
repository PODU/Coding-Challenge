//! The unit of work that flows through the sort pipeline.

/// A single input line together with its precomputed comparison key.
///
/// Caching the key alongside the line is a *Schwartzian transform*: the key is
/// derived once (per `-b`/`-f`/`-n`/`-R` flags) and then every comparison and
/// every radix pass reuses it, instead of re-deriving it `O(n log n)` times.
///
/// The `key` is an **order-preserving** byte string: comparing two keys with
/// plain `[u8]` lexical ordering yields the desired *ascending* order for the
/// active flags. That is what lets all five algorithms share one code path and
/// what makes byte-wise radix sort "just work" for numeric and random keys too
/// (see [`crate::key`] for how the encodings are built).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Record {
    /// The raw line bytes, without the trailing `\n`.
    pub line: Vec<u8>,
    /// The order-preserving comparison key for this line.
    pub key: Vec<u8>,
}

impl Record {
    /// Build a record from a line and its precomputed key.
    pub fn new(line: Vec<u8>, key: Vec<u8>) -> Self {
        Record { line, key }
    }

    /// An empty placeholder, used as a temporary "hole" when permuting a slice
    /// of records in place without cloning their payloads.
    pub fn placeholder() -> Self {
        Record {
            line: Vec::new(),
            key: Vec::new(),
        }
    }
}
