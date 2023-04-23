//! Top-down stable merge sort.
//!
//! Merge sort is the stable, predictable `O(n log n)` workhorse and the
//! conceptual basis of the external k-way merge. To avoid moving (and thus
//! cloning) `Record` payloads during the recursive merges, it sorts an index
//! permutation and applies it to the slice once at the end.

use crate::cmp::Comparator;
use crate::record::Record;
use std::cmp::Ordering;

/// Sort `data` with a stable merge sort.
pub fn merge_sort(data: &mut [Record], cmp: &Comparator) {
    let n = data.len();
    if n <= 1 {
        return;
    }
    let mut idx: Vec<usize> = (0..n).collect();
    let mut scratch: Vec<usize> = vec![0; n];
    msort(&mut idx, &mut scratch, data, cmp);
    super::reorder_by_indices(data, &idx);
}

fn msort(idx: &mut [usize], scratch: &mut [usize], data: &[Record], cmp: &Comparator) {
    let n = idx.len();
    if n <= 1 {
        return;
    }
    let mid = n / 2;
    {
        let (left, right) = idx.split_at_mut(mid);
        let (sl, sr) = scratch.split_at_mut(mid);
        msort(left, sl, data, cmp);
        msort(right, sr, data, cmp);
    }

    // Merge the two sorted halves of `idx` into `scratch`, then copy back.
    let mut i = 0;
    let mut j = mid;
    let mut k = 0;
    while i < mid && j < n {
        // `!= Greater` (i.e. take the left element on ties) preserves stability.
        if cmp.cmp_records(&data[idx[i]], &data[idx[j]]) != Ordering::Greater {
            scratch[k] = idx[i];
            i += 1;
        } else {
            scratch[k] = idx[j];
            j += 1;
        }
        k += 1;
    }
    while i < mid {
        scratch[k] = idx[i];
        i += 1;
        k += 1;
    }
    while j < n {
        scratch[k] = idx[j];
        j += 1;
        k += 1;
    }
    idx.copy_from_slice(&scratch[..n]);
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::key::build_key;

    fn recs(lines: &[&[u8]]) -> Vec<Record> {
        let cfg = Config::default();
        lines
            .iter()
            .map(|l| Record::new(l.to_vec(), build_key(l, &cfg, 0)))
            .collect()
    }

    #[test]
    fn sorts_and_is_stable() {
        let mut d = recs(&[b"b1", b"a1", b"b2", b"a2", b"b3"]);
        // Reduce keys to first byte so b*/a* tie within groups.
        for r in &mut d {
            r.key = vec![r.line[0]];
        }
        merge_sort(&mut d, &Comparator::new(false));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        assert_eq!(
            got,
            vec![
                b"a1".to_vec(),
                b"a2".to_vec(),
                b"b1".to_vec(),
                b"b2".to_vec(),
                b"b3".to_vec()
            ]
        );
    }
}
