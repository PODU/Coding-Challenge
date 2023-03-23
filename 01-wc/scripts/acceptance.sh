#!/bin/sh
# acceptance.sh — quick manual gate for ccwc.
#
# Builds the binary and diffs its output against the system `wc` for each flag
# combination on a set of generated fixtures. Exits non-zero on any mismatch.
# (The Go differential tests in cmd/ccwc/main_test.go cover the same ground;
# this script is a fast human-facing smoke check.)

set -eu

here=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$here"

if ! command -v wc >/dev/null 2>&1; then
	echo "system wc not found; cannot run differential acceptance" >&2
	exit 1
fi

bin=$(mktemp -d)/ccwc
go build -o "$bin" ./cmd/ccwc

work=$(mktemp -d)
# Run inside the fixture dir and use bare filenames. This keeps output identical
# across tools even on MSYS/Git-Bash, whose runtime rewrites POSIX paths like
# /tmp/... into C:\... when launching a native (non-MSYS) binary such as ccwc.
cd "$work"
printf 'The quick brown fox\njumps over the lazy dog\n' >ascii.txt
printf 'héllo wörld\n日本語 テスト\n'                    >utf8.txt
printf 'trailing line without newline'                    >nonl.txt
printf ''                                                  >empty.txt

fail=0
check() {
	# $1 = description, remaining args = arguments to both tools
	desc=$1; shift
	got=$("$bin" "$@") || true
	want=$(wc "$@") || true
	if [ "$got" != "$want" ]; then
		printf 'FAIL %s\n  ccwc: %s\n  wc:   %s\n' "$desc" "$got" "$want" >&2
		fail=1
	else
		printf 'ok   %s\n' "$desc"
	fi
}

for f in ascii.txt utf8.txt nonl.txt empty.txt; do
	for flag in '' -c -l -w -m; do
		# shellcheck disable=SC2086
		check "$flag $f" $flag "$f"
	done
done

check "default multi" ascii.txt utf8.txt
check "-l multi" -l ascii.txt utf8.txt

if [ "$fail" -ne 0 ]; then
	echo "ACCEPTANCE FAILED" >&2
	exit 1
fi
echo "ALL ACCEPTANCE CHECKS PASSED"
