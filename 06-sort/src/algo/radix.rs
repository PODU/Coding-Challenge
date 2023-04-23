//! MSD (most-significant-digit) byte-wise radix sort.
//!
//! Radix sorts on the raw key bytes. Because keys are *order-preserving* byte
//! strings (see [`crate::key`]), ascending byte-radix order equals the desired
//! ascending order for lexical, numeric and random modes alike. Reverse (`-r`)
//! is applied by reversing the computed permutation at the end.
//!
//! MSD recursion handles variable-length keys naturally: at each depth a 257-way
//! counting sort buckets records by the byte at that position, with bucket 0
//! reserved for "key ended here" (shorter keys sort first, exactly like lexical
//! order where `"ab" < "abc"`). Distribution is stable, so equal keys keep their
//! input order.

use crate::cmp::Comparator;
use crate::record::Record;

/// Sort `data` with MSD radix sort.
pub fn radix_sort(data: &mut [Record], cmp: &Comparator) {
    let n = data.len();
    if n <= 1 {
        return;
    }

    let mut idx: Vec<u32> = (0..n as u32).collect();
    msd(data, &mut idx, 0);

    let mut order: Vec<usize> = idx.into_iter().map(|x| x as usize).collect();
    if cmp.reverse {
        order.reverse();
    }
    super::reorder_by_indices(data, &order);
}

/// Switch to a simple insertion sort (on the key suffix) below this size to
/// avoid counting-sort overhead on tiny buckets.
const INSERTION_CUTOFF: usize = 24;

fn msd(data: &[Record], idx: &mut [u32], depth: usize) {
    if idx.len() <= 1 {
        return;
    }
    if idx.len() <= INSERTION_CUTOFF {
        insertion_by_suffix(data, idx, depth);
        return;
    }

    // Bucket 0 == "key ended"; buckets 1..=256 == byte value 0..=255.
    let mut count = [0usize; 257];
    for &i in idx.iter() {
        count[bucket(&data[i as usize].key, depth)] += 1;
    }

    // Exclusive prefix sums -> starting offset of each bucket.
    let mut offset = [0usize; 257];
    let mut sum = 0usize;
    for b in 0..257 {
        offset[b] = sum;
        sum += count[b];
    }

    // Stable distribution into a scratch buffer, then copy back.
    let mut tmp = vec![0u32; idx.len()];
    let mut pos = offset;
    for &i in idx.iter() {
        let b = bucket(&data[i as usize].key, depth);
        tmp[pos[b]] = i;
        pos[b] += 1;
    }
    idx.copy_from_slice(&tmp);

    // Recurse into each value bucket (bucket 0 is fully ordered: keys ended).
    let mut start = count[0];
    for b in 1..257 {
        let len = count[b];
        if len > 1 {
            msd(data, &mut idx[start..start + len], depth + 1);
        }
        start += len;
    }
}

#[inline]
fn bucket(key: &[u8], depth: usize) -> usize {
    if depth < key.len() {
        key[depth] as usize + 1
    } else {
        0
    }
}

/// Insertion sort a small bucket comparing key suffixes from `depth` onward.
fn insertion_by_suffix(data: &[Record], idx: &mut [u32], depth: usize) {
    for i in 1..idx.len() {
        let mut j = i;
        while j > 0 {
            let a = &data[idx[j - 1] as usize].key;
            let b = &data[idx[j] as usize].key;
            let da = if depth < a.len() { &a[depth..] } else { &[][..] };
            let db = if depth < b.len() { &b[depth..] } else { &[][..] };
            if da > db {
                idx.swap(j - 1, j);
                j -= 1;
            } else {
                break;
            }
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
    fn handles_variable_length_keys() {
        let mut d = recs(&[b"abc", b"ab", b"abcd", b"a", b"", b"b"]);
        radix_sort(&mut d, &Comparator::new(false));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        assert_eq!(
            got,
            vec![
                b"".to_vec(),
                b"a".to_vec(),
                b"ab".to_vec(),
                b"abc".to_vec(),
                b"abcd".to_vec(),
                b"b".to_vec()
            ]
        );
    }

    #[test]
    fn large_bucket_uses_counting_path() {
        // > INSERTION_CUTOFF distinct keys to exercise the counting-sort branch.
        let lines: Vec<Vec<u8>> = (0..200u32)
            .rev()
            .map(|n| format!("{n:04}").into_bytes())
            .collect();
        let refs: Vec<&[u8]> = lines.iter().map(|v| v.as_slice()).collect();
        let mut d = recs(&refs);
        radix_sort(&mut d, &Comparator::new(false));
        let got: Vec<_> = d.iter().map(|r| r.line.clone()).collect();
        let mut expected = lines.clone();
        expected.sort();
        assert_eq!(got, expected);
    }
}
