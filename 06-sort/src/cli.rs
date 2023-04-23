//! Hand-rolled, `getopt`-style command-line parser.
//!
//! Dependency-free by design (see `Cargo.toml`). It supports clustered short
//! flags (`-ur`), attached (`-ofile`, `-S16M`) and separate (`-o file`) option
//! values, long options with `=` or a following argument, the `--` terminator,
//! and `-` for stdin.

use crate::config::{Algorithm, Config};

/// The result of parsing argv.
pub enum ParseOutcome {
    /// Run with this configuration.
    Run(Config),
    /// `-h`/`--help` was given: print help and exit 0.
    Help,
    /// `-V`/`--version` was given: print version and exit 0.
    Version,
}

/// Parse an argument list (excluding argv[0]).
pub fn parse<I>(args: I) -> Result<ParseOutcome, String>
where
    I: IntoIterator<Item = String>,
{
    let args: Vec<String> = args.into_iter().collect();
    let mut cfg = Config::default();
    let mut files: Vec<String> = Vec::new();
    let mut only_files = false;
    let mut i = 0;

    while i < args.len() {
        let arg = args[i].clone();

        if only_files {
            files.push(arg);
            i += 1;
            continue;
        }
        if arg == "--" {
            only_files = true;
            i += 1;
            continue;
        }
        if arg == "-" {
            files.push("-".to_string());
            i += 1;
            continue;
        }

        if let Some(long) = arg.strip_prefix("--") {
            let (name, inline_val) = match long.split_once('=') {
                Some((n, v)) => (n, Some(v.to_string())),
                None => (long, None),
            };
            match name {
                "help" => return Ok(ParseOutcome::Help),
                "version" => return Ok(ParseOutcome::Version),
                "reverse" => cfg.reverse = true,
                "numeric-sort" => cfg.numeric = true,
                "ignore-case" => cfg.ignore_case = true,
                "ignore-leading-blanks" => cfg.ignore_blanks = true,
                "unique" => cfg.unique = true,
                "random-sort" => cfg.random = true,
                "check" => {
                    cfg.check = Some(match inline_val.as_deref() {
                        None | Some("diagnose-first") => false,
                        Some("quiet") | Some("silent") => true,
                        Some(other) => {
                            return Err(format!("invalid argument {other:?} for '--check'"))
                        }
                    });
                }
                "algorithm" => {
                    let v = value_for(name, inline_val, &args, &mut i)?;
                    cfg.algorithm = Algorithm::parse(&v)?;
                }
                "sort" => {
                    let v = value_for(name, inline_val, &args, &mut i)?;
                    apply_sort_word(&mut cfg, &v)?;
                }
                "random-source" => {
                    cfg.random_source = Some(value_for(name, inline_val, &args, &mut i)?);
                }
                "output" => {
                    cfg.output = Some(value_for(name, inline_val, &args, &mut i)?);
                }
                "buffer-size" => {
                    let v = value_for(name, inline_val, &args, &mut i)?;
                    cfg.buffer_size = parse_size(&v)?;
                }
                "temp-dir" => {
                    cfg.temp_dir = Some(value_for(name, inline_val, &args, &mut i)?);
                }
                "key" | "field-separator" => {
                    return Err(format!(
                        "--{name}: key fields (-k/-t) are not implemented in this build"
                    ))
                }
                other => return Err(format!("unrecognized option '--{other}'")),
            }
            i += 1;
            continue;
        }

        if arg.starts_with('-') && arg.len() > 1 {
            let bytes = arg.as_bytes();
            let mut j = 1;
            'cluster: while j < bytes.len() {
                let c = bytes[j] as char;
                match c {
                    'r' => cfg.reverse = true,
                    'n' => cfg.numeric = true,
                    'f' => cfg.ignore_case = true,
                    'b' => cfg.ignore_blanks = true,
                    'u' => cfg.unique = true,
                    'R' => cfg.random = true,
                    'c' => cfg.check = Some(false),
                    'C' => cfg.check = Some(true),
                    'h' => return Ok(ParseOutcome::Help),
                    'V' => return Ok(ParseOutcome::Version),
                    'o' | 'S' | 'k' | 't' => {
                        // Value option: the rest of the cluster, or the next arg.
                        let rest = &arg[j + 1..];
                        let val = if !rest.is_empty() {
                            rest.to_string()
                        } else {
                            i += 1;
                            if i >= args.len() {
                                return Err(format!("option requires an argument -- '{c}'"));
                            }
                            args[i].clone()
                        };
                        match c {
                            'o' => cfg.output = Some(val),
                            'S' => cfg.buffer_size = parse_size(&val)?,
                            'k' | 't' => {
                                return Err(format!(
                                    "-{c}: key fields (-k/-t) are not implemented in this build"
                                ))
                            }
                            _ => unreachable!(),
                        }
                        break 'cluster;
                    }
                    other => return Err(format!("invalid option -- '{other}'")),
                }
                j += 1;
            }
            i += 1;
            continue;
        }

        // A plain operand: an input file.
        files.push(arg);
        i += 1;
    }

    cfg.files = files;
    Ok(ParseOutcome::Run(cfg))
}

fn value_for(
    name: &str,
    inline: Option<String>,
    args: &[String],
    i: &mut usize,
) -> Result<String, String> {
    match inline {
        Some(v) => Ok(v),
        None => {
            *i += 1;
            if *i >= args.len() {
                Err(format!("option '--{name}' requires an argument"))
            } else {
                Ok(args[*i].clone())
            }
        }
    }
}

/// Map a `--sort=WORD` value onto the config (GNU compatibility shim).
fn apply_sort_word(cfg: &mut Config, word: &str) -> Result<(), String> {
    match word {
        "random" => cfg.random = true,
        "numeric" | "general-numeric" | "n" => cfg.numeric = true,
        other => cfg.algorithm = Algorithm::parse(other)?,
    }
    Ok(())
}

/// Parse a memory size like `64M`, `16m`, `1G`, `512`, `2048b`.
///
/// Suffixes (case-insensitive, IEC/1024-based): `b`, `k`, `m`, `g`, `t`.
pub fn parse_size(s: &str) -> Result<usize, String> {
    let s = s.trim();
    if s.is_empty() {
        return Err("empty buffer size".to_string());
    }
    let digit_len = s.chars().take_while(|c| c.is_ascii_digit()).count();
    if digit_len == 0 {
        return Err(format!("invalid buffer size {s:?}"));
    }
    let (num, suffix) = s.split_at(digit_len);
    let base: usize = num
        .parse()
        .map_err(|_| format!("invalid buffer size {s:?}"))?;
    let mult: usize = match suffix {
        "" | "b" | "B" => 1,
        "k" | "K" => 1024,
        "m" | "M" => 1024 * 1024,
        "g" | "G" => 1024 * 1024 * 1024,
        "t" | "T" => 1024usize.pow(4),
        other => return Err(format!("invalid buffer-size suffix {other:?}")),
    };
    base.checked_mul(mult)
        .filter(|&v| v > 0)
        .ok_or_else(|| "buffer size out of range".to_string())
}

/// The program name as shown in help/usage.
pub const PROGRAM: &str = "ccsort";

/// Render the help text.
pub fn help_text() -> String {
    format!(
        "\
{PROGRAM} {ver} — sort lines of text (Coding Challenge #6)

USAGE:
    {PROGRAM} [OPTIONS] [FILE...]

Reads FILE(s) (or stdin if none, or \"-\"), sorts the lines, and writes the
result to stdout (or to -o FILE). Ordering is byte-wise, matching LC_ALL=C.

ORDERING:
  -n, --numeric-sort            Compare by leading numeric value
  -r, --reverse                 Reverse the comparison (descending)
  -R, --random-sort             Sort by a random hash; equal lines group
  -f, --ignore-case             Fold case before comparing
  -b, --ignore-leading-blanks   Ignore leading blanks in the key

UNIQUENESS / CHECK:
  -u, --unique                  Output only the first of each equal-key run
  -c, --check[=quiet]           Check whether input is sorted; do not sort
  -C                            Like -c but report nothing (status only)

ALGORITHM:
      --algorithm <A>           radix | merge | quick | heap | default
                                [default: default]
      --sort <WORD>             random | numeric | <algorithm> (GNU shim)
      --random-source <SEED>    Seed for -R (reproducible runs)

I/O & RESOURCES:
  -o, --output <FILE>           Write to FILE (in-place safe: -o f f)
  -S, --buffer-size <SIZE>      Memory budget before external sort (e.g. 64M)
      --temp-dir <DIR>          Directory for external-sort run files

  -h, --help                    Print help
  -V, --version                 Print version

EXIT STATUS:
  0  success (or, with -c, input was sorted)
  1  with -c/-C: input was NOT sorted
  2  an error occurred (bad option, I/O failure)

NOTE: -k/-t (key fields) are recognised but not implemented in this build.
",
        ver = env!("CARGO_PKG_VERSION")
    )
}

/// Render the version line.
pub fn version_text() -> String {
    format!("{PROGRAM} {}", env!("CARGO_PKG_VERSION"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn run(args: &[&str]) -> Config {
        match parse(args.iter().map(|s| s.to_string())).unwrap() {
            ParseOutcome::Run(c) => c,
            _ => panic!("expected Run"),
        }
    }

    #[test]
    fn clustered_short_flags() {
        let c = run(&["-ur", "file.txt"]);
        assert!(c.unique && c.reverse);
        assert_eq!(c.files, vec!["file.txt"]);
    }

    #[test]
    fn attached_and_separate_option_values() {
        assert_eq!(run(&["-ofoo"]).output.as_deref(), Some("foo"));
        assert_eq!(run(&["-o", "bar"]).output.as_deref(), Some("bar"));
        assert_eq!(run(&["-S16M"]).buffer_size, 16 * 1024 * 1024);
        assert_eq!(run(&["-S", "1024"]).buffer_size, 1024);
    }

    #[test]
    fn long_options() {
        let c = run(&["--reverse", "--numeric-sort", "--output=out.txt"]);
        assert!(c.reverse && c.numeric);
        assert_eq!(c.output.as_deref(), Some("out.txt"));
        assert_eq!(run(&["--algorithm", "radix"]).algorithm, Algorithm::Radix);
        assert_eq!(run(&["--algorithm=heap"]).algorithm, Algorithm::Heap);
    }

    #[test]
    fn check_variants() {
        assert_eq!(run(&["-c"]).check, Some(false));
        assert_eq!(run(&["-C"]).check, Some(true));
        assert_eq!(run(&["--check"]).check, Some(false));
        assert_eq!(run(&["--check=quiet"]).check, Some(true));
    }

    #[test]
    fn double_dash_terminator_and_stdin() {
        let c = run(&["--", "-weird-name", "-"]);
        assert_eq!(c.files, vec!["-weird-name", "-"]);
        let c2 = run(&["-", "real.txt"]);
        assert_eq!(c2.files, vec!["-", "real.txt"]);
    }

    #[test]
    fn random_source_and_sort_word() {
        let c = run(&["-R", "--random-source", "42"]);
        assert!(c.random);
        assert_eq!(c.random_source.as_deref(), Some("42"));
        assert!(run(&["--sort=random"]).random);
        assert!(run(&["--sort=numeric"]).numeric);
    }

    #[test]
    fn size_parsing() {
        assert_eq!(parse_size("64M").unwrap(), 64 * 1024 * 1024);
        assert_eq!(parse_size("1G").unwrap(), 1024 * 1024 * 1024);
        assert_eq!(parse_size("512").unwrap(), 512);
        assert!(parse_size("abc").is_err());
        assert!(parse_size("10Q").is_err());
        assert!(parse_size("0").is_err());
    }

    #[test]
    fn unknown_option_errors() {
        assert!(parse(["--bogus".to_string()]).is_err());
        assert!(parse(["-z".to_string()]).is_err());
    }

    #[test]
    fn help_and_version() {
        assert!(matches!(
            parse(["--help".to_string()]).unwrap(),
            ParseOutcome::Help
        ));
        assert!(matches!(
            parse(["-V".to_string()]).unwrap(),
            ParseOutcome::Version
        ));
    }
}
