//! `ccsort` binary entry point.
//!
//! Parses argv, runs the engine, and maps results to GNU-compatible exit codes:
//! `0` success, `1` `-c`/`-C` disorder, `2` any error.

use ccsort::cli::{self, ParseOutcome};
use ccsort::{engine, exit};
use std::process::ExitCode;

fn main() -> ExitCode {
    let args = std::env::args().skip(1);

    let cfg = match cli::parse(args) {
        Ok(ParseOutcome::Run(cfg)) => cfg,
        Ok(ParseOutcome::Help) => {
            print!("{}", cli::help_text());
            return ExitCode::from(exit::OK as u8);
        }
        Ok(ParseOutcome::Version) => {
            println!("{}", cli::version_text());
            return ExitCode::from(exit::OK as u8);
        }
        Err(msg) => {
            eprintln!("{}: {msg}", cli::PROGRAM);
            eprintln!("Try '{} --help' for more information.", cli::PROGRAM);
            return ExitCode::from(exit::ERROR as u8);
        }
    };

    match engine::run(&cfg) {
        Ok(code) => ExitCode::from(code as u8),
        Err(e) => {
            eprintln!("{}: {e}", cli::PROGRAM);
            ExitCode::from(exit::ERROR as u8)
        }
    }
}
