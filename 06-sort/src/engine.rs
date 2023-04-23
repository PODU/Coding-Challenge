//! The orchestrator: wires the reader, key extraction, the selected algorithm,
//! uniqueness, the external-sort fallback and the output writer together.
//!
//! Lines are read lazily into [`Record`]s while tracking roughly how many bytes
//! they use. If everything fits in the budget it is sorted in memory. Once the
//! buffer would go over, it is sorted and spilled to a temp run and the rest of
//! the input goes through the external merge, so memory stays bounded even for
//! endless stdin.

use crate::algo::sorter_for;
use crate::check::run_check;
use crate::cmp::Comparator;
use crate::config::Config;
use crate::external::TempRuns;
use crate::key::build_key;
use crate::output::OutputWriter;
use crate::reader::LineReader;
use crate::record::Record;
use crate::unique::dedup_adjacent;
use crate::{exit, random};
use std::io;

/// Execute `cfg`, returning the process exit code.
pub fn run(cfg: &Config) -> io::Result<i32> {
    cfg.validate().map_err(invalid_input)?;

    // -c / -C never sort; they only verify order.
    if cfg.check.is_some() {
        return run_check(cfg);
    }

    let cmp = Comparator::new(cfg.reverse);
    let seed = if cfg.random {
        random::resolve_seed(cfg.random_source.as_deref())
    } else {
        0
    };
    let sorter = sorter_for(cfg.algorithm);

    let mut buffer: Vec<Record> = Vec::new();
    let mut buffered_bytes: usize = 0;
    let mut runs: Option<TempRuns> = None;

    for item in LineReader::new(&cfg.files) {
        let line = item?;
        let key = build_key(&line, cfg, seed);
        buffered_bytes += record_size(&line, &key);
        buffer.push(Record::new(line, key));

        if buffered_bytes >= cfg.buffer_size {
            if runs.is_none() {
                runs = Some(TempRuns::new(cfg.temp_dir.as_deref())?);
            }
            sorter.sort(&mut buffer, &cmp);
            runs.as_mut().expect("just set").spill(&buffer)?;
            buffer.clear();
            buffered_bytes = 0;
        }
    }

    match runs {
        // Fast path: everything fit in memory.
        None => {
            sorter.sort(&mut buffer, &cmp);
            if cfg.unique {
                dedup_adjacent(&mut buffer);
            }
            let mut out = OutputWriter::open(cfg.output.as_deref())?;
            for r in &buffer {
                out.write_line(&r.line)?;
            }
            out.finish()?;
        }
        // External path: spill the tail, then k-way merge all runs.
        Some(mut runs) => {
            if !buffer.is_empty() {
                sorter.sort(&mut buffer, &cmp);
                runs.spill(&buffer)?;
                buffer.clear();
            }
            let mut out = OutputWriter::open(cfg.output.as_deref())?;
            runs.merge(cfg, seed, &cmp, &mut out)?;
            out.finish()?;
        }
    }

    Ok(exit::OK)
}

/// Approximate in-memory footprint of a record: payload plus per-`Vec` overhead.
#[inline]
fn record_size(line: &[u8], key: &[u8]) -> usize {
    line.len() + key.len() + 2 * std::mem::size_of::<Vec<u8>>()
}

fn invalid_input(msg: String) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidInput, msg)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::io::Write;

    fn write_tmp(name: &str, contents: &[u8]) -> std::path::PathBuf {
        let mut p = std::env::temp_dir();
        p.push(format!("ccsort_engine_{}_{}", std::process::id(), name));
        let mut f = fs::File::create(&p).unwrap();
        f.write_all(contents).unwrap();
        p
    }

    fn run_to_file(cfg: &mut Config, out_name: &str) -> Vec<u8> {
        let out = std::env::temp_dir().join(format!(
            "ccsort_engine_out_{}_{}",
            std::process::id(),
            out_name
        ));
        cfg.output = Some(out.to_string_lossy().into_owned());
        let code = run(cfg).unwrap();
        assert_eq!(code, exit::OK);
        let data = fs::read(&out).unwrap();
        fs::remove_file(out).ok();
        data
    }

    #[test]
    fn in_memory_and_external_agree() {
        // 2000 lines of pseudo-random short keys with duplicates.
        let mut content = Vec::new();
        for i in 0..2000u32 {
            content.extend_from_slice(format!("{:04}\n", (i * 7919) % 1000).as_bytes());
        }
        let input = write_tmp("big.txt", &content);

        // In-memory (huge budget).
        let mut c1 = Config::default();
        c1.files = vec![input.to_string_lossy().into_owned()];
        c1.buffer_size = 64 * 1024 * 1024;
        let mem = run_to_file(&mut c1, "mem.txt");

        // External (tiny budget forces many runs).
        let mut c2 = Config::default();
        c2.files = vec![input.to_string_lossy().into_owned()];
        c2.buffer_size = 1024; // ~1 KiB budget -> dozens of runs
        let ext = run_to_file(&mut c2, "ext.txt");

        assert_eq!(mem, ext, "external sort must match the in-memory result");

        // And both must equal a straightforward reference sort.
        let mut lines: Vec<&[u8]> = content.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
        lines.sort();
        let mut expected = Vec::new();
        for l in lines {
            expected.extend_from_slice(l);
            expected.push(b'\n');
        }
        assert_eq!(mem, expected);

        fs::remove_file(input).ok();
    }

    #[test]
    fn external_path_honours_unique_and_reverse() {
        let mut content = Vec::new();
        for i in 0..500u32 {
            content.extend_from_slice(format!("{}\n", i % 50).as_bytes());
        }
        let input = write_tmp("dups.txt", &content);

        let mut c = Config::default();
        c.files = vec![input.to_string_lossy().into_owned()];
        c.unique = true;
        c.reverse = true;
        c.numeric = true;
        c.buffer_size = 256; // force external
        let got = run_to_file(&mut c, "ur.txt");

        // Expect 50..=0 each once (numeric, reverse, unique).
        let mut expected = Vec::new();
        for i in (0..50u32).rev() {
            expected.extend_from_slice(format!("{i}\n").as_bytes());
        }
        assert_eq!(got, expected);

        fs::remove_file(input).ok();
    }

    #[test]
    fn in_place_output_does_not_truncate_input() {
        let path = write_tmp("inplace.txt", b"c\na\nb\n");
        let mut c = Config::default();
        c.files = vec![path.to_string_lossy().into_owned()];
        c.output = Some(path.to_string_lossy().into_owned());
        let code = run(&c).unwrap();
        assert_eq!(code, exit::OK);
        let got = fs::read(&path).unwrap();
        assert_eq!(got, b"a\nb\nc\n");
        fs::remove_file(path).ok();
    }
}
