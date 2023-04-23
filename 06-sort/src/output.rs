//! The output sink: stdout or a file, with safe in-place support.
//!
//! For `-o FILE`, output is written to a sibling temporary file and atomically
//! renamed over `FILE` on success. That is what makes the in-place idiom
//! `sort -o f f` safe: the input `f` is fully consumed before the temp file
//! replaces it, so it can never be truncated mid-read.

use std::fs::{self, File};
use std::io::{self, BufWriter, Write};
use std::path::{Path, PathBuf};

/// A writer that targets stdout or a file (with deferred atomic rename).
pub struct OutputWriter {
    inner: Inner,
}

enum Inner {
    Stdout(BufWriter<io::Stdout>),
    /// Buffered writer over a temp file plus the (temp, final) paths.
    File {
        writer: BufWriter<File>,
        temp_path: PathBuf,
        final_path: PathBuf,
    },
}

impl OutputWriter {
    /// Open the output. `None` -> stdout; `Some(path)` -> file via temp+rename.
    pub fn open(output: Option<&str>) -> io::Result<OutputWriter> {
        match output {
            None => Ok(OutputWriter {
                inner: Inner::Stdout(BufWriter::new(io::stdout())),
            }),
            Some(path) => {
                let final_path = PathBuf::from(path);
                let temp_path = temp_sibling(&final_path);
                let file = File::create(&temp_path).map_err(|e| {
                    io::Error::new(e.kind(), format!("{}: {e}", temp_path.display()))
                })?;
                Ok(OutputWriter {
                    inner: Inner::File {
                        writer: BufWriter::new(file),
                        temp_path,
                        final_path,
                    },
                })
            }
        }
    }

    /// Write one line followed by `\n`.
    pub fn write_line(&mut self, line: &[u8]) -> io::Result<()> {
        let w: &mut dyn Write = match &mut self.inner {
            Inner::Stdout(w) => w,
            Inner::File { writer, .. } => writer,
        };
        w.write_all(line)?;
        w.write_all(b"\n")
    }

    /// Flush buffers and, for file output, atomically rename temp -> final.
    pub fn finish(self) -> io::Result<()> {
        match self.inner {
            Inner::Stdout(mut w) => w.flush(),
            Inner::File {
                mut writer,
                temp_path,
                final_path,
            } => {
                writer.flush()?;
                writer.get_ref().sync_all().ok();
                drop(writer);
                fs::rename(&temp_path, &final_path).map_err(|e| {
                    // Clean up the temp file on a failed rename.
                    fs::remove_file(&temp_path).ok();
                    io::Error::new(e.kind(), format!("{}: {e}", final_path.display()))
                })
            }
        }
    }
}

/// Pick a temp path next to `final_path` so the rename stays on one filesystem.
fn temp_sibling(final_path: &Path) -> PathBuf {
    let dir = final_path.parent().filter(|p| !p.as_os_str().is_empty());
    let name = final_path
        .file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| "out".to_string());
    let temp_name = format!(".{name}.ccsort.{}.tmp", std::process::id());
    match dir {
        Some(d) => d.join(temp_name),
        None => PathBuf::from(temp_name),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn writes_file_via_rename() {
        let mut p = std::env::temp_dir();
        p.push(format!("ccsort_out_{}.txt", std::process::id()));
        {
            let mut w = OutputWriter::open(Some(p.to_str().unwrap())).unwrap();
            w.write_line(b"alpha").unwrap();
            w.write_line(b"beta").unwrap();
            w.finish().unwrap();
        }
        let got = std::fs::read(&p).unwrap();
        assert_eq!(got, b"alpha\nbeta\n");
        std::fs::remove_file(p).ok();
    }
}
