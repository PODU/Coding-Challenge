//! Property-style tests over the library API (no external crates).
//!
//! For many pseudo-random inputs we assert these invariants:
//! every algorithm yields a permutation of the input that is non-decreasing
//! under the comparator; `-u` output has no adjacent equal keys; and `-R` with a
//! fixed seed is a deterministic permutation.

use ccsort::algo::sorter_for;
use ccsort::cmp::Comparator;
use ccsort::config::{Algorithm, Config};
use ccsort::key::build_key;
use ccsort::record::Record;
use ccsort::unique::dedup_adjacent;
use std::cmp::Ordering;

/// A tiny deterministic xorshift64* PRNG so tests need no `rand` crate.
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
    fn below(&mut self, n: u64) -> u64 {
        self.next_u64() % n
    }
}

fn random_lines(rng: &mut Rng, count: usize, max_len: usize, alphabet: u64) -> Vec<Vec<u8>> {
    (0..count)
        .map(|_| {
            let len = rng.below(max_len as u64 + 1) as usize;
            (0..len)
                .map(|_| b'a' + (rng.below(alphabet) as u8))
                .collect()
        })
        .collect()
}

fn records(lines: &[Vec<u8>], cfg: &Config, seed: u64) -> Vec<Record> {
    lines
        .iter()
        .map(|l| Record::new(l.clone(), build_key(l, cfg, seed)))
        .collect()
}

fn sorted_multiset(lines: &[Vec<u8>]) -> Vec<Vec<u8>> {
    let mut v = lines.to_vec();
    v.sort();
    v
}

const ALGOS: &[Algorithm] = &[
    Algorithm::Radix,
    Algorithm::Merge,
    Algorithm::Quick,
    Algorithm::Heap,
    Algorithm::Default,
];

#[test]
fn every_algorithm_is_a_sorted_permutation() {
    let cfg = Config::default();
    for trial in 0..200u64 {
        let mut rng = Rng::new(0xABCD_0000 + trial);
        // Vary shape: small alphabet -> many duplicates; longer -> few.
        let count = (rng.below(64) + 1) as usize;
        let max_len = (rng.below(6) + 1) as usize;
        let alphabet = rng.below(4) + 2; // 2..=5 distinct letters
        let lines = random_lines(&mut rng, count, max_len, alphabet);

        let expected = sorted_multiset(&lines);
        let cmp = Comparator::new(false);

        for &algo in ALGOS {
            let mut data = records(&lines, &cfg, 0);
            sorter_for(algo).sort(&mut data, &cmp);

            // Non-decreasing under the comparator.
            for w in data.windows(2) {
                assert_ne!(
                    cmp.cmp_records(&w[0], &w[1]),
                    Ordering::Greater,
                    "{algo} out of order (trial {trial})"
                );
            }
            // Permutation of input (multiset equality).
            let got: Vec<Vec<u8>> = data.iter().map(|r| r.line.clone()).collect();
            let mut got_sorted = got.clone();
            got_sorted.sort();
            assert_eq!(got_sorted, expected, "{algo} not a permutation (trial {trial})");
        }
    }
}

#[test]
fn unique_has_no_adjacent_equal_keys() {
    let mut cfg = Config::default();
    cfg.unique = true;
    for trial in 0..100u64 {
        let mut rng = Rng::new(0x5151_0000 + trial);
        let lines = random_lines(&mut rng, 50, 3, 3); // lots of duplicates
        let cmp = Comparator::new(false);
        for &algo in ALGOS {
            let mut data = records(&lines, &cfg, 0);
            sorter_for(algo).sort(&mut data, &cmp);
            dedup_adjacent(&mut data);
            for w in data.windows(2) {
                assert_ne!(w[0].key, w[1].key, "{algo} left adjacent duplicate");
            }
            // Same set of distinct keys as a reference dedup.
            let mut distinct: Vec<Vec<u8>> = sorted_multiset(&lines);
            distinct.dedup();
            let mut got: Vec<Vec<u8>> = data.iter().map(|r| r.key.clone()).collect();
            got.sort();
            assert_eq!(got, distinct, "{algo} wrong unique set");
        }
    }
}

#[test]
fn random_sort_fixed_seed_is_deterministic_permutation() {
    let mut cfg = Config::default();
    cfg.random = true;
    let mut rng = Rng::new(777);
    let lines = random_lines(&mut rng, 80, 4, 5);
    let cmp = Comparator::new(false);

    let run = |seed: u64| {
        let mut data = records(&lines, &cfg, seed);
        sorter_for(Algorithm::Default).sort(&mut data, &cmp);
        data.iter().map(|r| r.line.clone()).collect::<Vec<_>>()
    };

    let a = run(42);
    let b = run(42);
    assert_eq!(a, b, "fixed seed must be deterministic");

    // Permutation of input.
    assert_eq!(sorted_multiset(&a), sorted_multiset(&lines));

    // Different seed generally yields a different order.
    let c = run(43);
    assert_ne!(a, c, "different seeds should differ");

    // Equal lines remain grouped under -R.
    for w in a.windows(2) {
        if w[0] != w[1] {
            let first = &w[0];
            let reappears = a
                .iter()
                .skip_while(|l| *l != &w[1])
                .any(|l| l == first);
            assert!(!reappears, "equal lines not grouped under -R");
        }
    }
}

#[test]
fn numeric_ordering_matches_value_order() {
    let mut cfg = Config::default();
    cfg.numeric = true;
    let raw = [
        "10", "2", "-5", "3.14", "0", "-0", "100", "-100", "  7", "1.5", "abc", "",
    ];
    let lines: Vec<Vec<u8>> = raw.iter().map(|s| s.as_bytes().to_vec()).collect();
    let cmp = Comparator::new(false);
    let mut data = records(&lines, &cfg, 0);
    sorter_for(Algorithm::Merge).sort(&mut data, &cmp);

    // Decode the numeric value of each output line and assert non-decreasing.
    let vals: Vec<f64> = data
        .iter()
        .map(|r| ccsort::key::parse_leading_number(&r.line))
        .collect();
    for w in vals.windows(2) {
        assert!(w[0] <= w[1], "numeric order violated: {:?}", vals);
    }
}
