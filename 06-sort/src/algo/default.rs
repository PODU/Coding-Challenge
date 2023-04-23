//! The baseline: the standard library's pattern-defeating quicksort.
//!
//! `slice::sort_unstable_by` is the reference point the challenge asks us to
//! compare the hand-written algorithms against. It is unstable but extremely
//! fast and allocation-free.

use crate::cmp::Comparator;
use crate::record::Record;

/// Sort `data` with the standard library's unstable sort.
pub fn default_sort(data: &mut [Record], cmp: &Comparator) {
    data.sort_unstable_by(|a, b| cmp.cmp_records(a, b));
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::key::build_key;

    #[test]
    fn matches_std_sort() {
        let cfg = Config::default();
        let lines: Vec<&[u8]> = vec![b"delta", b"alpha", b"charlie", b"bravo", b"alpha"];
        let mut d: Vec<Record> = lines
            .iter()
            .map(|l| Record::new(l.to_vec(), build_key(l, &cfg, 0)))
            .collect();
        default_sort(&mut d, &Comparator::new(false));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        assert_eq!(
            got,
            vec![
                b"alpha".to_vec(),
                b"alpha".to_vec(),
                b"bravo".to_vec(),
                b"charlie".to_vec(),
                b"delta".to_vec()
            ]
        );
    }
}
