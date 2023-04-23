//! The active comparator.
//!
//! All ordering in `ccsort` is expressed as a byte-wise comparison of the
//! precomputed, order-preserving [`Record::key`](crate::record::Record::key).
//! The only twist this layer adds is `-r` (reverse): it flips the result of the
//! underlying `[u8]` comparison. Because the keys already encode numeric and
//! random ordering, this single comparator drives every algorithm uniformly.

use crate::record::Record;
use std::cmp::Ordering;

/// A cheap, `Copy` comparator over record keys.
#[derive(Clone, Copy, Debug)]
pub struct Comparator {
    /// When `true`, ascending becomes descending.
    pub reverse: bool,
}

impl Comparator {
    /// Construct a comparator. `reverse` corresponds to the `-r` flag.
    pub fn new(reverse: bool) -> Self {
        Comparator { reverse }
    }

    /// Compare two raw keys.
    #[inline]
    pub fn cmp(&self, a: &[u8], b: &[u8]) -> Ordering {
        let o = a.cmp(b);
        if self.reverse {
            o.reverse()
        } else {
            o
        }
    }

    /// Compare two records by their keys.
    #[inline]
    pub fn cmp_records(&self, a: &Record, b: &Record) -> Ordering {
        self.cmp(&a.key, &b.key)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ascending_is_byte_order() {
        let c = Comparator::new(false);
        assert_eq!(c.cmp(b"abc", b"abd"), Ordering::Less);
        assert_eq!(c.cmp(b"ab", b"abc"), Ordering::Less); // shorter prefix first
        assert_eq!(c.cmp(b"abc", b"abc"), Ordering::Equal);
    }

    #[test]
    fn reverse_flips() {
        let c = Comparator::new(true);
        assert_eq!(c.cmp(b"abc", b"abd"), Ordering::Greater);
        assert_eq!(c.cmp(b"abc", b"abc"), Ordering::Equal);
    }
}
