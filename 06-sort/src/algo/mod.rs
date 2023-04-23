//! The pluggable sorting algorithms.
//!
//! Each algorithm is implemented from scratch over `&mut [Record]` and sorts by
//! the precomputed [`Record::key`] using the shared [`Comparator`]. They are
//! selected at runtime via [`sorter_for`].
//!
//! | Algorithm | Stable | In-place | Notes                                   |
//! |-----------|--------|----------|-----------------------------------------|
//! | radix     | yes    | no       | MSD byte-wise; natural fit for strings  |
//! | merge     | yes    | no       | O(n) aux; basis of the external sort    |
//! | quick     | no     | yes      | median-of-three + insertion cutoff      |
//! | heap      | no     | yes      | O(n log n) worst case, bounded memory   |
//! | default   | no     | yes      | `slice::sort_unstable_by` baseline      |

pub mod default;
pub mod heap;
pub mod merge;
pub mod quick;
pub mod radix;

use crate::cmp::Comparator;
use crate::config::Algorithm;
use crate::record::Record;

/// A sorting strategy over a slice of records.
pub trait Sorter {
    /// Sort `data` in place into ascending key order (descending if the
    /// comparator is reversed).
    fn sort(&self, data: &mut [Record], cmp: &Comparator);
}

/// Marker types for each algorithm.
pub struct RadixSort;
pub struct MergeSort;
pub struct QuickSort;
pub struct HeapSort;
pub struct DefaultSort;

impl Sorter for RadixSort {
    fn sort(&self, data: &mut [Record], cmp: &Comparator) {
        radix::radix_sort(data, cmp);
    }
}
impl Sorter for MergeSort {
    fn sort(&self, data: &mut [Record], cmp: &Comparator) {
        merge::merge_sort(data, cmp);
    }
}
impl Sorter for QuickSort {
    fn sort(&self, data: &mut [Record], cmp: &Comparator) {
        quick::quick_sort(data, cmp);
    }
}
impl Sorter for HeapSort {
    fn sort(&self, data: &mut [Record], cmp: &Comparator) {
        heap::heap_sort(data, cmp);
    }
}
impl Sorter for DefaultSort {
    fn sort(&self, data: &mut [Record], cmp: &Comparator) {
        default::default_sort(data, cmp);
    }
}

/// Construct the boxed [`Sorter`] for the requested algorithm.
pub fn sorter_for(algo: Algorithm) -> Box<dyn Sorter> {
    match algo {
        Algorithm::Radix => Box::new(RadixSort),
        Algorithm::Merge => Box::new(MergeSort),
        Algorithm::Quick => Box::new(QuickSort),
        Algorithm::Heap => Box::new(HeapSort),
        Algorithm::Default => Box::new(DefaultSort),
    }
}

/// Reorder `data` in place so that `data[i]` becomes the element that was at
/// `order[i]`. `order` must be a permutation of `0..data.len()`.
///
/// Each record is moved exactly once (through an `Option` hole), so line and
/// key buffers are never cloned.
pub(crate) fn reorder_by_indices(data: &mut [Record], order: &[usize]) {
    debug_assert_eq!(data.len(), order.len());
    let mut slots: Vec<Option<Record>> = data
        .iter_mut()
        .map(|r| Some(std::mem::replace(r, Record::placeholder())))
        .collect();
    for (dst, &src) in order.iter().enumerate() {
        data[dst] = slots[src].take().expect("order must be a permutation");
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::key::build_key;

    /// Build records from raw lines using the given config (seed 0).
    fn records(lines: &[&[u8]], cfg: &Config) -> Vec<Record> {
        lines
            .iter()
            .map(|l| Record::new(l.to_vec(), build_key(l, cfg, 0)))
            .collect()
    }

    fn lines_of(recs: &[Record]) -> Vec<Vec<u8>> {
        recs.iter().map(|r| r.line.clone()).collect()
    }

    /// The canonical fixtures from the design's test plan.
    fn fixtures() -> Vec<Vec<&'static [u8]>> {
        let data: Vec<Vec<&'static [u8]>> = vec![
            vec![],                                              // empty
            vec![b"only"],                                       // single
            vec![b"a", b"b", b"c"],                              // already sorted
            vec![b"c", b"b", b"a"],                              // reverse sorted
            vec![b"x", b"x", b"x", b"x"],                        // all equal
            vec![b"banana", b"apple", b"cherry", b"apple", b"date"], // dups
            vec![b"", b"zzz", b"", b"a", b"Ab", b"ab"],          // blanks + case bytes
        ];
        data
    }

    fn all_sorters() -> Vec<(Algorithm, Box<dyn Sorter>)> {
        [
            Algorithm::Radix,
            Algorithm::Merge,
            Algorithm::Quick,
            Algorithm::Heap,
            Algorithm::Default,
        ]
        .into_iter()
        .map(|a| (a, sorter_for(a)))
        .collect()
    }

    #[test]
    fn every_algorithm_matches_reference_ascending() {
        let cfg = Config::default();
        let cmp = Comparator::new(false);
        for fx in fixtures() {
            // Reference: std stable sort by key.
            let mut reference = records(&fx, &cfg);
            reference.sort_by(|a, b| cmp.cmp_records(a, b));
            let expected = lines_of(&reference);

            for (algo, sorter) in all_sorters() {
                let mut data = records(&fx, &cfg);
                sorter.sort(&mut data, &cmp);
                // Output must be non-decreasing under the comparator...
                for w in data.windows(2) {
                    assert_ne!(
                        cmp.cmp_records(&w[0], &w[1]),
                        std::cmp::Ordering::Greater,
                        "{algo} produced out-of-order output on {fx:?}"
                    );
                }
                // ...and a permutation of the reference (same multiset of keys).
                let mut got_keys: Vec<_> = data.iter().map(|r| r.key.clone()).collect();
                let mut exp_keys: Vec<_> = reference.iter().map(|r| r.key.clone()).collect();
                got_keys.sort();
                exp_keys.sort();
                assert_eq!(got_keys, exp_keys, "{algo} lost/added elements on {fx:?}");
                let _ = &expected;
            }
        }
    }

    #[test]
    fn stable_algorithms_preserve_input_order_of_equal_keys() {
        // Distinct lines that share a case-folded key.
        let mut cfg = Config::default();
        cfg.ignore_case = true;
        let cmp = Comparator::new(false);
        let fx: Vec<&[u8]> = vec![b"Bob", b"bob", b"BOB", b"amy", b"Amy"];

        for algo in [Algorithm::Radix, Algorithm::Merge] {
            let mut data = records(&fx, &cfg);
            sorter_for(algo).sort(&mut data, &cmp);
            let got = lines_of(&data);
            // amy-group keeps input order (amy, Amy); bob-group keeps (Bob,bob,BOB)
            assert_eq!(
                got,
                vec![
                    b"amy".to_vec(),
                    b"Amy".to_vec(),
                    b"Bob".to_vec(),
                    b"bob".to_vec(),
                    b"BOB".to_vec()
                ],
                "{algo} is not stable"
            );
        }
    }

    #[test]
    fn reverse_orders_descending() {
        let cfg = Config::default();
        let cmp = Comparator::new(true);
        let fx: Vec<&[u8]> = vec![b"a", b"c", b"b"];
        for algo in [
            Algorithm::Radix,
            Algorithm::Merge,
            Algorithm::Quick,
            Algorithm::Heap,
            Algorithm::Default,
        ] {
            let mut data = records(&fx, &cfg);
            sorter_for(algo).sort(&mut data, &cmp);
            assert_eq!(
                lines_of(&data),
                vec![b"c".to_vec(), b"b".to_vec(), b"a".to_vec()],
                "{algo} reverse failed"
            );
        }
    }
}
