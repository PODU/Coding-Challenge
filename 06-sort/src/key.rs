//! Comparison-key extraction (the Schwartzian transform).
//!
//! Given a raw line and the active [`Config`], [`build_key`] returns an
//! **order-preserving** byte string: lexical `[u8]` comparison of two keys
//! yields the desired *ascending* order. Encoding every mode (lexical, numeric,
//! random) as byte strings is what lets all five algorithms, including the
//! byte-wise radix sort, share one comparator.
//!
//! ## Last-resort comparison
//! GNU `sort`, when two keys compare equal, falls back to comparing the whole
//! raw line. `ccsort` deliberately does **not** do this: equal-key records keep
//! their input order under the stable algorithms (radix/merge), or an arbitrary
//! order under the unstable ones. This only affects the *order among lines that
//! share a key* (possible with `-b`/`-f`/`-n`), never which lines appear, and
//! it keeps all five algorithms mutually consistent. See README "Deviations".

use crate::config::Config;
use crate::random::hash64;

/// Build the order-preserving comparison key for `line`.
///
/// `seed` is only used when `cfg.random` is set.
pub fn build_key(line: &[u8], cfg: &Config, seed: u64) -> Vec<u8> {
    if cfg.random {
        // Group identical lines: key = big-endian hash of the raw line.
        return hash64(seed, line).to_be_bytes().to_vec();
    }

    if cfg.numeric {
        let value = parse_leading_number(line);
        return encode_f64_order_preserving(value).to_vec();
    }

    // Lexical key: optionally strip leading blanks and/or fold case.
    let start = if cfg.ignore_blanks {
        line.iter()
            .position(|&b| !is_blank(b))
            .unwrap_or(line.len())
    } else {
        0
    };

    if cfg.ignore_case {
        line[start..].iter().map(|&b| b.to_ascii_uppercase()).collect()
    } else {
        line[start..].to_vec()
    }
}

/// ASCII blank per the `C` locale: space or horizontal tab.
#[inline]
fn is_blank(b: u8) -> bool {
    b == b' ' || b == b'\t'
}

/// Parse the leading numeric value of a line as GNU `sort -n` does.
///
/// Recognised grammar: optional leading blanks, optional sign, decimal digits,
/// optional fractional part. There is **no** exponent support (`1e3` parses as
/// `1`, matching GNU). Anything non-numeric (or empty) yields `0.0`.
pub fn parse_leading_number(line: &[u8]) -> f64 {
    let mut i = 0;
    let n = line.len();
    while i < n && is_blank(line[i]) {
        i += 1;
    }

    let start = i;
    let mut buf: Vec<u8> = Vec::new();

    // sign
    if i < n && (line[i] == b'+' || line[i] == b'-') {
        if line[i] == b'-' {
            buf.push(b'-');
        }
        i += 1;
    }

    let mut saw_digit = false;
    while i < n && line[i].is_ascii_digit() {
        buf.push(line[i]);
        saw_digit = true;
        i += 1;
    }

    if i < n && line[i] == b'.' {
        buf.push(b'.');
        i += 1;
        while i < n && line[i].is_ascii_digit() {
            buf.push(line[i]);
            saw_digit = true;
            i += 1;
        }
    }

    let _ = start;
    if !saw_digit {
        return 0.0;
    }

    // `buf` is ASCII and matches Rust's float grammar (sans exponent).
    std::str::from_utf8(&buf)
        .ok()
        .and_then(|s| s.parse::<f64>().ok())
        .filter(|f| f.is_finite())
        .unwrap_or(0.0)
}

/// Encode an `f64` as 8 bytes such that unsigned lexical byte order equals
/// numeric order.
///
/// Trick: flip the sign bit for non-negatives, flip *all* bits for negatives.
/// `-0.0` is normalised to `+0.0` first so the two compare equal.
pub fn encode_f64_order_preserving(f: f64) -> [u8; 8] {
    let f = if f == 0.0 { 0.0 } else { f }; // normalise -0.0 -> +0.0
    let bits = f.to_bits();
    let ordered = if bits & 0x8000_0000_0000_0000 != 0 {
        !bits
    } else {
        bits ^ 0x8000_0000_0000_0000
    };
    ordered.to_be_bytes()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn cfg() -> Config {
        Config::default()
    }

    #[test]
    fn lexical_key_is_identity_by_default() {
        let c = cfg();
        assert_eq!(build_key(b"Hello", &c, 0), b"Hello".to_vec());
    }

    #[test]
    fn ignore_blanks_strips_leading_whitespace() {
        let mut c = cfg();
        c.ignore_blanks = true;
        assert_eq!(build_key(b"   abc", &c, 0), b"abc".to_vec());
        assert_eq!(build_key(b"\t\t x", &c, 0), b"x".to_vec());
        assert_eq!(build_key(b"     ", &c, 0), b"".to_vec());
    }

    #[test]
    fn ignore_case_folds_to_upper() {
        let mut c = cfg();
        c.ignore_case = true;
        assert_eq!(build_key(b"Apple", &c, 0), b"APPLE".to_vec());
        assert_eq!(build_key(b"apple", &c, 0), b"APPLE".to_vec());
    }

    #[test]
    fn numeric_parsing_edge_cases() {
        assert_eq!(parse_leading_number(b"  42"), 42.0);
        assert_eq!(parse_leading_number(b"-3.5"), -3.5);
        assert_eq!(parse_leading_number(b"+7"), 7.0);
        assert_eq!(parse_leading_number(b"1e3"), 1.0); // no exponent (GNU)
        assert_eq!(parse_leading_number(b"abc"), 0.0);
        assert_eq!(parse_leading_number(b""), 0.0);
        assert_eq!(parse_leading_number(b"12abc"), 12.0);
        assert_eq!(parse_leading_number(b"."), 0.0);
        assert_eq!(parse_leading_number(b"3.14xyz"), 3.14);
    }

    #[test]
    fn numeric_encoding_preserves_order() {
        let cases = [-1e308, -100.0, -1.5, -0.0, 0.0, 1.5, 100.0, 1e308];
        for w in cases.windows(2) {
            let a = encode_f64_order_preserving(w[0]);
            let b = encode_f64_order_preserving(w[1]);
            assert!(a <= b, "{:?} should encode <= {:?}", w[0], w[1]);
        }
    }

    #[test]
    fn numeric_zero_signs_equal() {
        assert_eq!(
            encode_f64_order_preserving(-0.0),
            encode_f64_order_preserving(0.0)
        );
    }

    #[test]
    fn random_key_groups_equal_lines() {
        let mut c = cfg();
        c.random = true;
        assert_eq!(build_key(b"banana", &c, 9), build_key(b"banana", &c, 9));
        assert_ne!(build_key(b"banana", &c, 9), build_key(b"apple", &c, 9));
    }
}
