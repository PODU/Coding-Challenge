//! In-place binary heapsort.
//!
//! Guaranteed `O(n log n)` with `O(1)` extra memory, whatever the input. It
//! builds a max-heap over the slice and repeatedly swaps the root to the end.
//! Not stable.

use crate::cmp::Comparator;
use crate::record::Record;
use std::cmp::Ordering;

/// Sort `data` with heapsort.
pub fn heap_sort(data: &mut [Record], cmp: &Comparator) {
    let n = data.len();
    if n <= 1 {
        return;
    }

    // Build a max-heap: sift down every internal node, bottom-up.
    for start in (0..n / 2).rev() {
        sift_down(data, start, n, cmp);
    }

    // Repeatedly move the current max to the end and restore the heap.
    for end in (1..n).rev() {
        data.swap(0, end);
        sift_down(data, 0, end, cmp);
    }
}

/// Restore the max-heap property for the subtree rooted at `root`, considering
/// only indices `< end`.
fn sift_down(data: &mut [Record], mut root: usize, end: usize, cmp: &Comparator) {
    loop {
        let left = 2 * root + 1;
        let right = 2 * root + 2;
        let mut largest = root;

        if left < end && cmp.cmp_records(&data[left], &data[largest]) == Ordering::Greater {
            largest = left;
        }
        if right < end && cmp.cmp_records(&data[right], &data[largest]) == Ordering::Greater {
            largest = right;
        }
        if largest == root {
            break;
        }
        data.swap(root, largest);
        root = largest;
    }
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
    fn sorts_with_duplicates() {
        let lines: Vec<Vec<u8>> = (0..64u32)
            .map(|n| format!("{:02}", (n * 13) % 17).into_bytes())
            .collect();
        let refs: Vec<&[u8]> = lines.iter().map(|v| v.as_slice()).collect();
        let mut d = recs(&refs);
        heap_sort(&mut d, &Comparator::new(false));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        let mut expected = lines.clone();
        expected.sort();
        assert_eq!(got, expected);
    }

    #[test]
    fn reverse_descends() {
        let mut d = recs(&[b"a", b"b", b"c", b"d"]);
        heap_sort(&mut d, &Comparator::new(true));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        assert_eq!(got, vec![b"d".to_vec(), b"c".to_vec(), b"b".to_vec(), b"a".to_vec()]);
    }
}
