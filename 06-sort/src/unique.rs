//! Uniqueness (`-u`) strategies.
//!
//! The challenge asks us to weigh two approaches:
//!
//! 1. **Sort then dedup adjacent** ([`dedup_adjacent`]). After sorting, equal
//!    keys are neighbours, so a single linear pass drops duplicates. This is the
//!    path `ccsort` uses in production: `O(n)` extra time, `O(1)` extra space,
//!    and it composes with *every* algorithm and with the external merge.
//!
//! 2. **Dedup while inserting with a hash set** ([`dedup_hashset`]). Removing
//!    duplicates *before* sorting shrinks the working set (great when there are
//!    many duplicates) but costs `O(n)` memory for the set, hashes every key,
//!    and can't work inside a memory-bounded external sort. It is here mostly so
//!    the trade-off can be measured and tested.
//!
//! "Equal" is by **key**, matching GNU semantics: under `-n -u`, two lines with
//! the same numeric value but different text are duplicates (their keys match).

use crate::record::Record;
use std::collections::HashSet;

/// In-place adjacent dedup. Assumes `records` is already sorted by key; keeps
/// the first record of each equal-key run.
pub fn dedup_adjacent(records: &mut Vec<Record>) {
    records.dedup_by(|a, b| a.key == b.key);
}

/// Pre-sort dedup using a hash set of keys. Returns the records whose key was
/// seen for the first time, in input order.
pub fn dedup_hashset(records: Vec<Record>) -> Vec<Record> {
    let mut seen: HashSet<Vec<u8>> = HashSet::with_capacity(records.len());
    let mut out = Vec::with_capacity(records.len());
    for r in records {
        if seen.insert(r.key.clone()) {
            out.push(r);
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cmp::Comparator;
    use crate::config::Config;
    use crate::key::build_key;

    fn recs(lines: &[&[u8]], cfg: &Config) -> Vec<Record> {
        lines
            .iter()
            .map(|l| Record::new(l.to_vec(), build_key(l, cfg, 0)))
            .collect()
    }

    #[test]
    fn adjacent_dedup_after_sort() {
        let cfg = Config::default();
        let mut d = recs(&[b"a", b"b", b"a", b"c", b"b", b"a"], &cfg);
        d.sort_by(|x, y| Comparator::new(false).cmp_records(x, y));
        dedup_adjacent(&mut d);
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        assert_eq!(got, vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec()]);
    }

    #[test]
    fn both_strategies_agree_on_set() {
        let cfg = Config::default();
        let lines: Vec<&[u8]> = vec![b"x", b"y", b"x", b"z", b"y", b"y", b"w"];

        // hash-set then sort
        let hs = dedup_hashset(recs(&lines, &cfg));
        let mut hs_keys: Vec<_> = hs.iter().map(|r| r.key.clone()).collect();
        hs_keys.sort();

        // sort then adjacent
        let mut sd = recs(&lines, &cfg);
        sd.sort_by(|x, y| Comparator::new(false).cmp_records(x, y));
        dedup_adjacent(&mut sd);
        let mut sd_keys: Vec<_> = sd.iter().map(|r| r.key.clone()).collect();
        sd_keys.sort();

        assert_eq!(hs_keys, sd_keys);
    }

    #[test]
    fn unique_is_by_numeric_key() {
        // "1.0" and "1" differ textually but share the numeric key.
        let mut cfg = Config::default();
        cfg.numeric = true;
        let mut d = recs(&[b"1", b"1.0", b"2", b"02"], &cfg);
        d.sort_by(|x, y| Comparator::new(false).cmp_records(x, y));
        dedup_adjacent(&mut d);
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        // one representative for value 1, one for value 2
        assert_eq!(got.len(), 2);
        assert_eq!(got[0], b"1".to_vec());
    }
}
