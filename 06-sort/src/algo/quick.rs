//! In-place quicksort with median-of-three pivots and an insertion-sort cutoff.
//!
//! Uses `O(log n)` stack. It is not stable, which is fine here: equal keys may
//! swap places, and `-u` dedups by key anyway. Median-of-three pivots and the
//! insertion-sort cutoff avoid the quadratic cases on sorted and organ-pipe
//! inputs.

use crate::cmp::Comparator;
use crate::record::Record;
use std::cmp::Ordering;

/// Slices at or below this length are finished with insertion sort.
const INSERTION_CUTOFF: usize = 16;

/// Sort `data` with quicksort.
pub fn quick_sort(data: &mut [Record], cmp: &Comparator) {
    quicksort(data, cmp);
}

fn quicksort(mut data: &mut [Record], cmp: &Comparator) {
    // Recurse into the smaller partition and loop on the larger one. This caps
    // recursion depth at O(log n) even for adversarial inputs (e.g. all-equal),
    // so the stack can never overflow.
    loop {
        if data.len() <= INSERTION_CUTOFF {
            insertion_sort(data, cmp);
            return;
        }
        let p = partition(data, cmp);
        let (left, right) = data.split_at_mut(p);
        // `right[0]` is the pivot, now in its final position; exclude it.
        let right = &mut right[1..];
        if left.len() < right.len() {
            quicksort(left, cmp);
            data = right;
        } else {
            quicksort(right, cmp);
            data = left;
        }
    }
}

/// Lomuto partition around a median-of-three pivot. Returns the pivot's final
/// index.
fn partition(data: &mut [Record], cmp: &Comparator) -> usize {
    let len = data.len();
    let mid = len / 2;
    let last = len - 1;

    // Order data[0] <= data[mid] <= data[last] so the median lands at `mid`.
    if cmp.cmp_records(&data[mid], &data[0]) == Ordering::Less {
        data.swap(0, mid);
    }
    if cmp.cmp_records(&data[last], &data[0]) == Ordering::Less {
        data.swap(0, last);
    }
    if cmp.cmp_records(&data[last], &data[mid]) == Ordering::Less {
        data.swap(mid, last);
    }

    // Park the pivot at the end, partition, then swap it into place.
    data.swap(mid, last);
    let pivot = last;
    let mut store = 0;
    for i in 0..last {
        if cmp.cmp_records(&data[i], &data[pivot]) == Ordering::Less {
            data.swap(i, store);
            store += 1;
        }
    }
    data.swap(store, pivot);
    store
}

fn insertion_sort(data: &mut [Record], cmp: &Comparator) {
    for i in 1..data.len() {
        let mut j = i;
        while j > 0 && cmp.cmp_records(&data[j - 1], &data[j]) == Ordering::Greater {
            data.swap(j - 1, j);
            j -= 1;
        }
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
    fn sorts_various_shapes() {
        // Larger than the cutoff, includes duplicates and adversarial order.
        let lines: Vec<Vec<u8>> = (0..100u32)
            .map(|n| format!("{:03}", (n * 37) % 100).into_bytes())
            .collect();
        let refs: Vec<&[u8]> = lines.iter().map(|v| v.as_slice()).collect();
        let mut d = recs(&refs);
        quick_sort(&mut d, &Comparator::new(false));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        let mut expected = lines.clone();
        expected.sort();
        assert_eq!(got, expected);
    }

    #[test]
    fn already_sorted_and_reverse() {
        let mut a = recs(&[b"a", b"b", b"c", b"d", b"e"]);
        quick_sort(&mut a, &Comparator::new(false));
        assert_eq!(a.iter().map(|r| r.line.clone()).collect::<Vec<_>>(),
                   vec![b"a".to_vec(), b"b".to_vec(), b"c".to_vec(), b"d".to_vec(), b"e".to_vec()]);
    }
}
