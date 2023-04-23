//! End-to-end CLI tests: drive the compiled `ccsort` binary like a user would.
//!
//! They cover basic ordering,
//! `-u`/`-r`/`-n` and their combinations across every `--algorithm`, `-c` exit
//! codes, `-o` (including in-place), stdin handling, `-R` reproducibility, and
//! the external-sort path forced by a tiny `-S` budget.

use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Stdio};

fn bin() -> &'static str {
    env!("CARGO_BIN_EXE_ccsort")
}

struct Output {
    code: i32,
    stdout: Vec<u8>,
    stderr: Vec<u8>,
}

/// Run the binary with `args`, feeding `stdin` to it.
fn run(args: &[&str], stdin: &[u8]) -> Output {
    let mut child = Command::new(bin())
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("spawn ccsort");
    child
        .stdin
        .take()
        .unwrap()
        .write_all(stdin)
        .expect("write stdin");
    let out = child.wait_with_output().expect("wait ccsort");
    Output {
        code: out.status.code().unwrap_or(-1),
        stdout: out.stdout,
        stderr: out.stderr,
    }
}

fn tmp_path(name: &str) -> PathBuf {
    let mut p = std::env::temp_dir();
    p.push(format!(
        "ccsort_it_{}_{}_{}",
        std::process::id(),
        next_id(),
        name
    ));
    p
}

fn next_id() -> u64 {
    use std::sync::atomic::{AtomicU64, Ordering};
    static C: AtomicU64 = AtomicU64::new(0);
    C.fetch_add(1, Ordering::Relaxed)
}

fn write_file(name: &str, contents: &[u8]) -> PathBuf {
    let p = tmp_path(name);
    std::fs::write(&p, contents).unwrap();
    p
}

const ALGOS: &[&str] = &["default", "radix", "merge", "quick", "heap"];

#[test]
fn basic_lexicographic_sort_all_algorithms() {
    let input = b"banana\napple\ncherry\napple\n";
    for &algo in ALGOS {
        let o = run(&["--algorithm", algo], input);
        assert_eq!(o.code, 0, "{algo}: exit");
        assert_eq!(
            o.stdout, b"apple\napple\nbanana\ncherry\n",
            "{algo}: output"
        );
    }
}

#[test]
fn unique_flag_all_algorithms() {
    let input = b"banana\napple\ncherry\napple\nbanana\n";
    for &algo in ALGOS {
        let o = run(&["-u", "--algorithm", algo], input);
        assert_eq!(o.code, 0);
        assert_eq!(o.stdout, b"apple\nbanana\ncherry\n", "{algo}");
    }
}

#[test]
fn reverse_flag() {
    let o = run(&["-r"], b"a\nc\nb\n");
    assert_eq!(o.stdout, b"c\nb\na\n");
}

#[test]
fn numeric_sort_and_reverse() {
    let input = b"10\n2\n1\n20\n3\n";
    let o = run(&["-n"], input);
    assert_eq!(o.stdout, b"1\n2\n3\n10\n20\n");

    let o = run(&["-n", "-r"], input);
    assert_eq!(o.stdout, b"20\n10\n3\n2\n1\n");
}

#[test]
fn numeric_unique_dedups_by_value() {
    // "1", "1.0" and "01" share a numeric key; "2","2" share another.
    // Which textual representative survives -u is algorithm-dependent (the
    // default sort is unstable), so assert by value, not by exact text.
    let o = run(&["-n", "-u"], b"1\n1.0\n01\n2\n2\n");
    let lines: Vec<&[u8]> = o.stdout.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
    assert_eq!(lines.len(), 2, "two distinct numeric values remain");
    let val = |s: &[u8]| std::str::from_utf8(s).unwrap().trim().parse::<f64>().unwrap();
    assert_eq!(val(lines[0]), 1.0);
    assert_eq!(val(lines[1]), 2.0);
}

#[test]
fn ignore_case_and_leading_blanks() {
    // Use the stable merge sort so equal-key (case-folded) lines keep input order.
    let o = run(&["-f", "--algorithm", "merge"], b"banana\nApple\napple\nBanana\n");
    assert_eq!(o.stdout, b"Apple\napple\nbanana\nBanana\n");

    let o = run(&["-b"], b"   zebra\n  apple\n");
    assert_eq!(o.stdout, b"  apple\n   zebra\n");
}

#[test]
fn lexical_vs_numeric_difference() {
    // Lexical: "10" < "2"; numeric: 2 < 10.
    let input = b"2\n10\n1\n";
    assert_eq!(run(&[], input).stdout, b"1\n10\n2\n");
    assert_eq!(run(&["-n"], input).stdout, b"1\n2\n10\n");
}

#[test]
fn check_reports_exit_codes() {
    let sorted = write_file("sorted.txt", b"a\nb\nc\n");
    let o = run(&["-c", sorted.to_str().unwrap()], b"");
    assert_eq!(o.code, 0);
    std::fs::remove_file(sorted).ok();

    let unsorted = write_file("unsorted.txt", b"a\nc\nb\n");
    let o = run(&["-c", unsorted.to_str().unwrap()], b"");
    assert_eq!(o.code, 1);
    assert!(String::from_utf8_lossy(&o.stderr).contains("disorder"));
    std::fs::remove_file(unsorted).ok();

    // -C is quiet: status only, no diagnostic.
    let unsorted = write_file("unsorted2.txt", b"b\na\n");
    let o = run(&["-C", unsorted.to_str().unwrap()], b"");
    assert_eq!(o.code, 1);
    assert!(o.stderr.is_empty());
    std::fs::remove_file(unsorted).ok();
}

#[test]
fn output_to_file_and_in_place() {
    let f = write_file("data.txt", b"c\na\nb\n");
    let out = tmp_path("out.txt");
    let o = run(&["-o", out.to_str().unwrap(), f.to_str().unwrap()], b"");
    assert_eq!(o.code, 0);
    assert!(o.stdout.is_empty());
    assert_eq!(std::fs::read(&out).unwrap(), b"a\nb\nc\n");
    std::fs::remove_file(&out).ok();

    // In-place: -o f f must not lose data.
    let o = run(&["-o", f.to_str().unwrap(), f.to_str().unwrap()], b"");
    assert_eq!(o.code, 0);
    assert_eq!(std::fs::read(&f).unwrap(), b"a\nb\nc\n");
    std::fs::remove_file(f).ok();
}

#[test]
fn stdin_and_multiple_files_concatenated() {
    let a = write_file("a.txt", b"banana\napple\n");
    let b = write_file("b.txt", b"cherry\napple\n");
    let o = run(&[a.to_str().unwrap(), b.to_str().unwrap()], b"");
    assert_eq!(o.stdout, b"apple\napple\nbanana\ncherry\n");
    std::fs::remove_file(a).ok();
    std::fs::remove_file(b).ok();

    // "-" mixes stdin with a file.
    let c = write_file("c.txt", b"mango\n");
    let o = run(&["-", c.to_str().unwrap()], b"apple\nzebra\n");
    assert_eq!(o.stdout, b"apple\nmango\nzebra\n");
    std::fs::remove_file(c).ok();
}

#[test]
fn empty_input_is_empty_output() {
    let o = run(&[], b"");
    assert_eq!(o.code, 0);
    assert!(o.stdout.is_empty());
}

#[test]
fn missing_final_newline_is_normalised() {
    let o = run(&[], b"b\na"); // no trailing newline
    assert_eq!(o.stdout, b"a\nb\n");
}

#[test]
fn blank_lines_sort_first() {
    let o = run(&[], b"b\n\na\n");
    assert_eq!(o.stdout, b"\na\nb\n");
}

#[test]
fn random_sort_is_reproducible_and_groups_equal_lines() {
    let input = b"banana\napple\nbanana\ncherry\napple\nbanana\n";
    let o1 = run(&["-R", "--random-source", "42"], input);
    let o2 = run(&["-R", "--random-source", "42"], input);
    assert_eq!(o1.code, 0);
    assert_eq!(o1.stdout, o2.stdout, "same seed must reproduce order");

    // Same multiset of lines.
    let mut got: Vec<&[u8]> = o1.stdout.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
    got.sort();
    assert_eq!(got, vec![&b"apple"[..], b"apple", b"banana", b"banana", b"banana", b"cherry"]);

    // Equal lines must be adjacent (grouped).
    let lines: Vec<&[u8]> = o1.stdout.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
    for w in lines.windows(2) {
        if w[0] == w[1] {
            continue;
        }
        // Once a value changes, it must never reappear later.
        let first = w[0];
        let later = lines.iter().skip_while(|l| **l != w[1]).any(|l| *l == first);
        assert!(!later, "equal lines not grouped for {:?}", String::from_utf8_lossy(first));
    }
}

#[test]
fn external_sort_matches_in_memory() {
    // Build an input far larger than the forced -S budget.
    let mut content = Vec::new();
    for i in 0..5000u32 {
        content.extend_from_slice(format!("{:05}\n", (i * 2654435761u32) % 9000).as_bytes());
    }
    let f = write_file("large.txt", &content);

    let in_mem = run(&["-S", "64M", f.to_str().unwrap()], b"");
    let external = run(&["-S", "4K", f.to_str().unwrap()], b"");
    assert_eq!(in_mem.code, 0);
    assert_eq!(external.code, 0);
    assert_eq!(in_mem.stdout, external.stdout, "external must match in-memory");

    // Reference.
    let mut lines: Vec<&[u8]> = content.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
    lines.sort();
    let mut expected = Vec::new();
    for l in lines {
        expected.extend_from_slice(l);
        expected.push(b'\n');
    }
    assert_eq!(in_mem.stdout, expected);
    std::fs::remove_file(f).ok();
}

#[test]
fn external_sort_with_unique() {
    let mut content = Vec::new();
    for i in 0..3000u32 {
        content.extend_from_slice(format!("{}\n", i % 100).as_bytes());
    }
    let f = write_file("dups.txt", &content);
    let o = run(&["-n", "-u", "-S", "1K", f.to_str().unwrap()], b"");
    assert_eq!(o.code, 0);
    let lines: Vec<&[u8]> = o.stdout.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
    assert_eq!(lines.len(), 100, "0..=99 each once");
    assert_eq!(lines[0], &b"0"[..]);
    assert_eq!(lines[99], &b"99"[..]);
    std::fs::remove_file(f).ok();
}

#[test]
fn all_algorithms_produce_identical_sets() {
    let mut content = Vec::new();
    for i in 0..1000u32 {
        content.extend_from_slice(format!("{}\n", (i * 48271) % 137).as_bytes());
    }
    let f = write_file("algos.txt", &content);
    let reference = run(&["--algorithm", "default", f.to_str().unwrap()], b"").stdout;
    for &algo in ALGOS {
        let o = run(&["--algorithm", algo, f.to_str().unwrap()], b"");
        assert_eq!(o.stdout, reference, "{algo} differs from default");
    }
    std::fs::remove_file(f).ok();
}

#[test]
fn errors_exit_2() {
    // Missing file.
    let o = run(&["does_not_exist_ccsort_xyz.txt"], b"");
    assert_eq!(o.code, 2);
    assert!(!o.stderr.is_empty());

    // Bad option.
    let o = run(&["--definitely-not-an-option"], b"");
    assert_eq!(o.code, 2);

    // Conflicting -n and -R.
    let o = run(&["-n", "-R"], b"1\n2\n");
    assert_eq!(o.code, 2);
}

#[test]
fn binary_bytes_do_not_panic() {
    let input: Vec<u8> = vec![0xff, 0x00, b'\n', 0x80, 0x01, b'\n', b'a', b'\n'];
    let o = run(&[], &input);
    assert_eq!(o.code, 0);
    // 0x00.. sorts before 'a' before 0x80 before 0xff (byte order).
    let lines: Vec<&[u8]> = o.stdout.split(|&b| b == b'\n').filter(|l| !l.is_empty()).collect();
    assert_eq!(lines[0], &[0x00][..]);
    assert_eq!(lines.last().unwrap(), &&[0xff][..]);
}

#[test]
fn help_and_version_exit_zero() {
    let o = run(&["--help"], b"");
    assert_eq!(o.code, 0);
    assert!(String::from_utf8_lossy(&o.stdout).contains("USAGE"));

    let o = run(&["--version"], b"");
    assert_eq!(o.code, 0);
    assert!(String::from_utf8_lossy(&o.stdout).contains("ccsort"));
}
