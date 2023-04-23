//! `-c` / `-C` check mode: verify the input is already sorted without sorting.
//!
//! Streams the input, builds each key, and compares neighbours. Reports the
//! first line that is out of order (unless quiet) and returns the appropriate
//! exit code. With `-u`, equal adjacent keys also count as disorder (the input
//! must be *strictly* sorted).

use crate::cmp::Comparator;
use crate::config::Config;
use crate::key::build_key;
use crate::reader::LineReader;
use crate::{exit, random};
use std::cmp::Ordering;
use std::io::{self, Write};

/// Run check mode. Returns the process exit code.
pub fn run_check(cfg: &Config) -> io::Result<i32> {
    let quiet = cfg.check.unwrap_or(false);
    let cmp = Comparator::new(cfg.reverse);
    let seed = if cfg.random {
        random::resolve_seed(cfg.random_source.as_deref())
    } else {
        0
    };

    let label = display_source(cfg);
    let mut prev: Option<Vec<u8>> = None;
    let mut line_no: u64 = 0;

    for item in LineReader::new(&cfg.files) {
        let line = item?;
        line_no += 1;
        let key = build_key(&line, cfg, seed);

        if let Some(prev_key) = &prev {
            let ord = cmp.cmp(prev_key, &key);
            let disordered = match ord {
                Ordering::Greater => true,
                Ordering::Equal => cfg.unique, // -u demands strictly increasing
                Ordering::Less => false,
            };
            if disordered {
                if !quiet {
                    let mut stderr = io::stderr();
                    write!(stderr, "ccsort: {label}:{line_no}: disorder: ")?;
                    stderr.write_all(&line)?;
                    stderr.write_all(b"\n")?;
                }
                return Ok(exit::DISORDER);
            }
        }
        prev = Some(key);
    }

    Ok(exit::OK)
}

fn display_source(cfg: &Config) -> String {
    match cfg.files.first() {
        Some(f) if f != "-" && cfg.files.len() == 1 => f.clone(),
        _ => "-".to_string(),
    }
}
