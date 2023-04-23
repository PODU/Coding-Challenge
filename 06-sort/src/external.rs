//! Memory-bounded external merge sort.
//!
//! When the input exceeds the configured memory budget (`-S`), the engine sorts
//! bounded chunks in RAM, spills each *sorted* chunk to a temp run file, and
//! then performs a k-way merge of the runs using a binary min-heap. Memory stays
//! bounded throughout: the merge holds only one line per run plus the heap.
//!
//! The temp files and their directory are removed when [`TempRuns`] is dropped,
//! including on an early error or panic.

use crate::cmp::Comparator;
use crate::config::Config;
use crate::key::build_key;
use crate::output::OutputWriter;
use crate::record::Record;
use std::cmp::{Ordering, Reverse};
use std::collections::BinaryHeap;
use std::fs::{self, File};
use std::io::{self, BufRead, BufReader, BufWriter, Write};
use std::path::PathBuf;

/// Owns the on-disk sorted runs and guarantees their cleanup.
pub struct TempRuns {
    dir: PathBuf,
    paths: Vec<PathBuf>,
}

impl TempRuns {
    /// Create a fresh, unique run directory under `temp_dir` (or the system
    /// temp dir if `None`).
    pub fn new(temp_dir: Option<&str>) -> io::Result<TempRuns> {
        let base = match temp_dir {
            Some(d) => PathBuf::from(d),
            None => std::env::temp_dir(),
        };
        let dir = base.join(format!(
            "ccsort.{}.{}",
            std::process::id(),
            unique_counter()
        ));
        fs::create_dir_all(&dir)
            .map_err(|e| io::Error::new(e.kind(), format!("{}: {e}", dir.display())))?;
        Ok(TempRuns {
            dir,
            paths: Vec::new(),
        })
    }

    /// Number of runs spilled so far.
    pub fn len(&self) -> usize {
        self.paths.len()
    }

    /// Whether any run has been spilled.
    pub fn is_empty(&self) -> bool {
        self.paths.is_empty()
    }

    /// Write one already-sorted chunk to a new run file.
    pub fn spill(&mut self, records: &[Record]) -> io::Result<()> {
        let path = self.dir.join(format!("run_{:06}.tmp", self.paths.len()));
        let file = File::create(&path)
            .map_err(|e| io::Error::new(e.kind(), format!("{}: {e}", path.display())))?;
        let mut writer = BufWriter::new(file);
        for r in records {
            writer.write_all(&r.line)?;
            writer.write_all(b"\n")?;
        }
        writer.flush()?;
        self.paths.push(path);
        Ok(())
    }

    /// k-way merge every run into `out`, applying `-u` (and `-R`/`-n` ordering,
    /// which are already baked into the keys) during the merge.
    pub fn merge(
        &self,
        cfg: &Config,
        seed: u64,
        cmp: &Comparator,
        out: &mut OutputWriter,
    ) -> io::Result<()> {
        let mut readers: Vec<BufReader<File>> = Vec::with_capacity(self.paths.len());
        for p in &self.paths {
            let f = File::open(p)
                .map_err(|e| io::Error::new(e.kind(), format!("{}: {e}", p.display())))?;
            readers.push(BufReader::new(f));
        }

        let mut heap: BinaryHeap<Reverse<HeapEntry>> = BinaryHeap::with_capacity(readers.len());
        for (run, reader) in readers.iter_mut().enumerate() {
            if let Some(line) = read_run_line(reader)? {
                let key = build_key(&line, cfg, seed);
                heap.push(Reverse(HeapEntry {
                    key,
                    line,
                    run,
                    reverse: cmp.reverse,
                }));
            }
        }

        let mut last_key: Option<Vec<u8>> = None;
        while let Some(Reverse(entry)) = heap.pop() {
            let emit = if cfg.unique {
                match &last_key {
                    Some(k) if *k == entry.key => false,
                    _ => true,
                }
            } else {
                true
            };
            if emit {
                out.write_line(&entry.line)?;
                if cfg.unique {
                    last_key = Some(entry.key.clone());
                }
            }

            // Pull the next line from the run we just consumed.
            if let Some(line) = read_run_line(&mut readers[entry.run])? {
                let key = build_key(&line, cfg, seed);
                heap.push(Reverse(HeapEntry {
                    key,
                    line,
                    run: entry.run,
                    reverse: cmp.reverse,
                }));
            }
        }
        Ok(())
    }
}

impl Drop for TempRuns {
    fn drop(&mut self) {
        for p in &self.paths {
            fs::remove_file(p).ok();
        }
        fs::remove_dir(&self.dir).ok();
    }
}

/// One run's current head line, ordered for the merge heap.
struct HeapEntry {
    key: Vec<u8>,
    line: Vec<u8>,
    run: usize,
    reverse: bool,
}

impl Ord for HeapEntry {
    fn cmp(&self, other: &Self) -> Ordering {
        let base = self.key.cmp(&other.key);
        let base = if self.reverse { base.reverse() } else { base };
        // Tie-break by run index for deterministic, stable merge order.
        base.then(self.run.cmp(&other.run))
    }
}
impl PartialOrd for HeapEntry {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}
impl PartialEq for HeapEntry {
    fn eq(&self, other: &Self) -> bool {
        self.cmp(other) == Ordering::Equal
    }
}
impl Eq for HeapEntry {}

fn read_run_line(reader: &mut BufReader<File>) -> io::Result<Option<Vec<u8>>> {
    let mut buf = Vec::new();
    let n = reader.read_until(b'\n', &mut buf)?;
    if n == 0 {
        Ok(None)
    } else {
        if buf.last() == Some(&b'\n') {
            buf.pop();
        }
        Ok(Some(buf))
    }
}

fn unique_counter() -> u64 {
    use std::sync::atomic::{AtomicU64, Ordering as AtomicOrdering};
    static COUNTER: AtomicU64 = AtomicU64::new(0);
    let seed = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_nanos() as u64)
        .unwrap_or(0);
    seed ^ COUNTER.fetch_add(1, AtomicOrdering::Relaxed)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::key::build_key;

    fn rec(line: &[u8], cfg: &Config) -> Record {
        Record::new(line.to_vec(), build_key(line, cfg, 0))
    }

    #[test]
    fn merges_runs_in_order() {
        let cfg = Config::default();
        let cmp = Comparator::new(false);
        let mut runs = TempRuns::new(None).unwrap();
        // Two pre-sorted runs.
        runs.spill(&[rec(b"a", &cfg), rec(b"c", &cfg), rec(b"e", &cfg)])
            .unwrap();
        runs.spill(&[rec(b"b", &cfg), rec(b"d", &cfg), rec(b"f", &cfg)])
            .unwrap();

        let out_path = std::env::temp_dir().join(format!("ccsort_merge_{}.txt", std::process::id()));
        let mut out = OutputWriter::open(Some(out_path.to_str().unwrap())).unwrap();
        runs.merge(&cfg, 0, &cmp, &mut out).unwrap();
        out.finish().unwrap();

        let got = fs::read(&out_path).unwrap();
        assert_eq!(got, b"a\nb\nc\nd\ne\nf\n");
        fs::remove_file(out_path).ok();
    }

    #[test]
    fn merge_applies_unique() {
        let mut cfg = Config::default();
        cfg.unique = true;
        let cmp = Comparator::new(false);
        let mut runs = TempRuns::new(None).unwrap();
        runs.spill(&[rec(b"a", &cfg), rec(b"a", &cfg), rec(b"b", &cfg)])
            .unwrap();
        runs.spill(&[rec(b"b", &cfg), rec(b"c", &cfg)]).unwrap();

        let out_path =
            std::env::temp_dir().join(format!("ccsort_merge_u_{}.txt", std::process::id()));
        let mut out = OutputWriter::open(Some(out_path.to_str().unwrap())).unwrap();
        runs.merge(&cfg, 0, &cmp, &mut out).unwrap();
        out.finish().unwrap();

        let got = fs::read(&out_path).unwrap();
        assert_eq!(got, b"a\nb\nc\n");
        fs::remove_file(out_path).ok();
    }

    #[test]
    fn temp_files_cleaned_on_drop() {
        let dir;
        {
            let cfg = Config::default();
            let mut runs = TempRuns::new(None).unwrap();
            runs.spill(&[rec(b"x", &cfg)]).unwrap();
            dir = runs.dir.clone();
            assert!(dir.exists());
        }
        assert!(!dir.exists(), "temp run directory should be removed on drop");
    }
}
