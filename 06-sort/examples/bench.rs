//! Benchmark comparing the five sort algorithms, as the challenge asks.
//!
//! Generates synthetic inputs and times every algorithm on each, printing a
//! small table. Dependency-free (uses `std::time::Instant`), so it runs on
//! stable Rust with no extra crates.
//!
//! Run with:  `cargo run --release --example bench`

use ccsort::algo::sorter_for;
use ccsort::cmp::Comparator;
use ccsort::config::{Algorithm, Config};
use ccsort::key::build_key;
use ccsort::record::Record;
use std::time::Instant;

struct Rng(u64);
impl Rng {
    fn new(seed: u64) -> Self {
        Rng(seed | 1)
    }
    fn next_u64(&mut self) -> u64 {
        let mut x = self.0;
        x ^= x >> 12;
        x ^= x << 25;
        x ^= x >> 27;
        self.0 = x;
        x.wrapping_mul(0x2545_F491_4F6C_DD1D)
    }
}

/// Many short keys with heavy duplication (where radix should shine).
fn short_dup_keys(n: usize) -> Vec<Vec<u8>> {
    let mut rng = Rng::new(1);
    (0..n)
        .map(|_| format!("{:05}", rng.next_u64() % 10_000).into_bytes())
        .collect()
}

/// Fewer duplicates, longer random lines.
fn long_random_lines(n: usize) -> Vec<Vec<u8>> {
    let mut rng = Rng::new(2);
    (0..n)
        .map(|_| {
            let len = 16 + (rng.next_u64() % 48) as usize;
            (0..len).map(|_| b'a' + (rng.next_u64() % 26) as u8).collect()
        })
        .collect()
}

fn records(lines: &[Vec<u8>], cfg: &Config) -> Vec<Record> {
    lines
        .iter()
        .map(|l| Record::new(l.clone(), build_key(l, cfg, 0)))
        .collect()
}

fn bench_dataset(name: &str, lines: &[Vec<u8>]) {
    let cfg = Config::default();
    let cmp = Comparator::new(false);
    println!("\n{name}  ({} lines)", lines.len());
    println!("  {:<10} {:>12}", "algorithm", "time");
    for algo in [
        Algorithm::Default,
        Algorithm::Radix,
        Algorithm::Merge,
        Algorithm::Quick,
        Algorithm::Heap,
    ] {
        // Exclude record/key construction from the measured region.
        let mut data = records(lines, &cfg);
        let start = Instant::now();
        sorter_for(algo).sort(&mut data, &cmp);
        let elapsed = start.elapsed();
        // Sanity: confirm the output is actually ordered.
        debug_assert!(data.windows(2).all(|w| w[0].key <= w[1].key));
        println!("  {:<10} {:>12?}", algo.to_string(), elapsed);
    }
}

fn main() {
    let n: usize = std::env::args()
        .nth(1)
        .and_then(|s| s.parse().ok())
        .unwrap_or(500_000);

    println!("ccsort algorithm comparison (n = {n})");
    bench_dataset("short keys, many duplicates", &short_dup_keys(n));
    bench_dataset("long random lines", &long_random_lines(n));
    println!("\n(Build with --release for meaningful numbers.)");
}
