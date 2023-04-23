//! Random-sort (`-R`) support.
//!
//! GNU `sort -R` is *not* a shuffle: it sorts by a hash of each line so that
//! identical lines stay adjacent ("equal keys group"). We reproduce that by
//! deriving a per-line 64-bit key = `hash(seed, line)` and encoding it
//! big-endian as the record key, so the normal byte-wise comparator orders the
//! groups deterministically for a given seed.

/// A seeded FNV-1a 64-bit hash with a final avalanche mix.
///
/// FNV-1a is small and gives the same result on every platform, so
/// `--random-source` runs are reproducible. The final mix (from MurmurHash3)
/// spreads similar inputs across all 64 bits so different seeds give visibly
/// different orders.
pub fn hash64(seed: u64, data: &[u8]) -> u64 {
    const OFFSET: u64 = 0xcbf2_9ce4_8422_2325;
    const PRIME: u64 = 0x0000_0100_0000_01b3;

    let mut h = OFFSET ^ seed;
    for &b in data {
        h ^= b as u64;
        h = h.wrapping_mul(PRIME);
    }
    // MurmurHash3 fmix64 finalizer.
    h ^= h >> 33;
    h = h.wrapping_mul(0xff51_afd7_ed55_8ccd);
    h ^= h >> 33;
    h = h.wrapping_mul(0xc4ce_b9fe_1a85_ec53);
    h ^= h >> 33;
    h
}

/// Resolve the seed for `-R` from the optional `--random-source` argument.
///
/// * A value that parses as an unsigned integer is used verbatim (e.g. `42`).
/// * Any other string is itself hashed into a seed (so `--random-source foo`
///   is reproducible too).
/// * `None` seeds from the clock, so every run differs, as in GNU sort.
pub fn resolve_seed(source: Option<&str>) -> u64 {
    match source {
        Some(s) => match s.trim().parse::<u64>() {
            Ok(n) => n,
            Err(_) => hash64(0, s.as_bytes()),
        },
        None => nondeterministic_seed(),
    }
}

fn nondeterministic_seed() -> u64 {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_nanos() as u64)
        .unwrap_or(0);
    // Mix in the process id for a little extra entropy within the same nanosecond.
    hash64(nanos, &std::process::id().to_le_bytes())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn deterministic_for_fixed_seed() {
        assert_eq!(hash64(42, b"hello"), hash64(42, b"hello"));
    }

    #[test]
    fn equal_lines_hash_equal() {
        assert_eq!(hash64(7, b"banana"), hash64(7, b"banana"));
    }

    #[test]
    fn different_seeds_differ() {
        assert_ne!(hash64(1, b"hello"), hash64(2, b"hello"));
    }

    #[test]
    fn numeric_source_is_used_verbatim() {
        assert_eq!(resolve_seed(Some("42")), 42);
    }

    #[test]
    fn textual_source_is_reproducible() {
        assert_eq!(resolve_seed(Some("foo")), resolve_seed(Some("foo")));
    }
}
