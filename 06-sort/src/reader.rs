//! A streaming line reader over multiple input sources.
//!
//! Lines are yielded as raw `Vec<u8>` (newline stripped) so the pipeline never
//! assumes valid UTF-8 and never has to hold every source open at once. Sources
//! are read lazily, one after another; `"-"` (or an empty file list) means
//! stdin. Reading lazily is what lets the engine switch to the external path
//! partway through.

use std::fs::File;
use std::io::{self, BufRead, BufReader};

/// Iterator over the concatenated lines of all configured sources.
pub struct LineReader {
    files: Vec<String>,
    next_file: usize,
    current: Option<Box<dyn BufRead>>,
}

impl LineReader {
    /// Create a reader over `files`. An empty list reads stdin.
    pub fn new(files: &[String]) -> Self {
        let files = if files.is_empty() {
            vec!["-".to_string()]
        } else {
            files.to_vec()
        };
        LineReader {
            files,
            next_file: 0,
            current: None,
        }
    }

    /// Open the next source, attaching its name to any open error.
    fn open_next(&mut self) -> Option<io::Result<Box<dyn BufRead>>> {
        if self.next_file >= self.files.len() {
            return None;
        }
        let name = self.files[self.next_file].clone();
        self.next_file += 1;

        if name == "-" {
            // `Stdin::lock` yields a `'static` guard, so it is fine to box.
            Some(Ok(Box::new(io::stdin().lock())))
        } else {
            match File::open(&name) {
                Ok(f) => Some(Ok(Box::new(BufReader::new(f)))),
                Err(e) => Some(Err(io::Error::new(
                    e.kind(),
                    format!("{name}: {e}"),
                ))),
            }
        }
    }
}

impl Iterator for LineReader {
    type Item = io::Result<Vec<u8>>;

    fn next(&mut self) -> Option<Self::Item> {
        loop {
            if self.current.is_none() {
                match self.open_next() {
                    None => return None,
                    Some(Err(e)) => return Some(Err(e)),
                    Some(Ok(reader)) => self.current = Some(reader),
                }
            }

            let reader = self.current.as_mut().expect("current set above");
            let mut buf = Vec::new();
            match reader.read_until(b'\n', &mut buf) {
                Ok(0) => {
                    // EOF on this source; advance to the next one.
                    self.current = None;
                    continue;
                }
                Ok(_) => {
                    if buf.last() == Some(&b'\n') {
                        buf.pop();
                    }
                    return Some(Ok(buf));
                }
                Err(e) => return Some(Err(e)),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    fn tmp_with(contents: &[u8]) -> std::path::PathBuf {
        let mut p = std::env::temp_dir();
        let unique = format!(
            "ccsort_reader_{}_{}.txt",
            std::process::id(),
            COUNTER.fetch_add(1, std::sync::atomic::Ordering::Relaxed)
        );
        p.push(unique);
        let mut f = File::create(&p).unwrap();
        f.write_all(contents).unwrap();
        p
    }

    static COUNTER: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(0);

    fn collect(path: &std::path::Path) -> Vec<Vec<u8>> {
        LineReader::new(&[path.to_string_lossy().into_owned()])
            .map(|r| r.unwrap())
            .collect()
    }

    #[test]
    fn strips_trailing_newline() {
        let p = tmp_with(b"a\nb\nc\n");
        assert_eq!(collect(&p), vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
        std::fs::remove_file(p).ok();
    }

    #[test]
    fn handles_missing_final_newline() {
        let p = tmp_with(b"a\nb\nc");
        assert_eq!(collect(&p), vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
        std::fs::remove_file(p).ok();
    }

    #[test]
    fn empty_file_yields_nothing() {
        let p = tmp_with(b"");
        assert!(collect(&p).is_empty());
        std::fs::remove_file(p).ok();
    }

    #[test]
    fn keeps_carriage_return_in_c_locale() {
        let p = tmp_with(b"a\r\nb\n");
        assert_eq!(collect(&p), vec![b"a\r".to_vec(), b"b".to_vec()]);
        std::fs::remove_file(p).ok();
    }

    #[test]
    fn missing_file_reports_name() {
        let mut r = LineReader::new(&["does_not_exist_ccsort.txt".to_string()]);
        let first = r.next().unwrap();
        let err = first.unwrap_err();
        assert!(err.to_string().contains("does_not_exist_ccsort.txt"));
    }
}
