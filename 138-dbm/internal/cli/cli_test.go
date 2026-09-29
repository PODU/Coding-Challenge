package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type result struct {
	code        int
	out, errOut string
}

func run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errOut)
	return result{code, out.String(), errOut.String()}
}

func ok(t *testing.T, args ...string) string {
	t.Helper()
	r := run(t, "", args...)
	if r.code != 0 {
		t.Fatalf("%v: exit %d, stderr %q", args, r.code, r.errOut)
	}
	return r.out
}

func TestWorkflow(t *testing.T) {
	db := filepath.Join(t.TempDir(), "test.db")
	out := ok(t, "create", db)
	if !strings.HasPrefix(out, "created") {
		t.Fatalf("create output %q", out)
	}
	ok(t, "-nosync", "put", db, "greeting", "Hello, world!")
	ok(t, "put", "-nosync", db, "language", "Go")
	ok(t, "-nosync", "put", db, "greeting", "Hi again")

	if out := ok(t, "get", db, "greeting"); out != "Hi again\n" {
		t.Fatalf("get = %q", out)
	}
	r := run(t, "", "get", db, "missing")
	if r.code != ExitNotFound || !strings.Contains(r.errOut, "key not found") {
		t.Fatalf("get missing: %+v", r)
	}

	keys := strings.Fields(ok(t, "list", db))
	if len(keys) != 2 || !contains(keys, "greeting") || !contains(keys, "language") {
		t.Fatalf("list = %v", keys)
	}
	kv := ok(t, "-values", "list", db)
	if !strings.Contains(kv, "language\tGo\n") {
		t.Fatalf("list -values = %q", kv)
	}

	st := ok(t, "stats", db)
	if !strings.Contains(st, "keys:                2") || !strings.Contains(st, "hash:                fnv1a") {
		t.Fatalf("stats = %s", st)
	}

	ok(t, "-nosync", "delete", db, "language")
	r = run(t, "", "delete", db, "language")
	if r.code != ExitNotFound {
		t.Fatalf("delete missing: %+v", r)
	}
	if keys := strings.Fields(ok(t, "list", db)); !reflect.DeepEqual(keys, []string{"greeting"}) {
		t.Fatalf("after delete list = %v", keys)
	}

	out = ok(t, "reorganise", db)
	if !strings.HasPrefix(out, "reorganised") || !strings.Contains(out, "1 keys kept") {
		t.Fatalf("reorganise = %q", out)
	}
	ok(t, "reorganize", db) // American spelling alias
	if out := ok(t, "get", db, "greeting"); out != "Hi again\n" {
		t.Fatalf("after reorganise get = %q", out)
	}
}

func TestCreateOptionsAndErrors(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "m.db")
	out := ok(t, "-page-size", "512", "-hash", "murmur3", "create", db)
	if !strings.Contains(out, "page size 512") || !strings.Contains(out, "murmur3") {
		t.Fatalf("create = %q", out)
	}
	ok(t, "-nosync", "put", db, "a", "1")
	if st := ok(t, "stats", db); !strings.Contains(st, "murmur3") || !strings.Contains(st, "page size:           512") {
		t.Fatalf("stats = %s", st)
	}
	if r := run(t, "", "create", db); r.code != ExitError {
		t.Fatalf("create over existing: %+v", r)
	}
	if r := run(t, "", "-hash", "sha1", "create", filepath.Join(dir, "x.db")); r.code != ExitError || !strings.Contains(r.errOut, "unknown hash") {
		t.Fatalf("bad hash: %+v", r)
	}
	if r := run(t, "", "-page-size", "1000", "create", filepath.Join(dir, "y.db")); r.code != ExitError {
		t.Fatalf("bad page size: %+v", r)
	}
	if r := run(t, "", "get", filepath.Join(dir, "nope.db"), "k"); r.code != ExitError {
		t.Fatalf("missing db: %+v", r)
	}
	junk := filepath.Join(dir, "junk.db")
	os.WriteFile(junk, bytes.Repeat([]byte("x"), 8192), 0o644)
	if r := run(t, "", "get", junk, "k"); r.code != ExitError || !strings.Contains(r.errOut, "corrupt") {
		t.Fatalf("junk db: %+v", r)
	}
}

func TestUsageErrors(t *testing.T) {
	if r := run(t, ""); r.code != ExitError || !strings.Contains(r.errOut, "Usage") {
		t.Fatalf("no args: %+v", r)
	}
	if r := run(t, "", "help"); r.code != 0 || !strings.Contains(r.out, "Usage") {
		t.Fatalf("help: %+v", r)
	}
	if r := run(t, "", "version"); r.code != 0 || !strings.Contains(r.out, Version) {
		t.Fatalf("version: %+v", r)
	}
	if r := run(t, "", "frobnicate", "x.db"); r.code != ExitError || !strings.Contains(r.errOut, "unknown command") {
		t.Fatalf("unknown: %+v", r)
	}
	if r := run(t, "", "put"); r.code != ExitError || !strings.Contains(r.errOut, "missing database file") {
		t.Fatalf("no file: %+v", r)
	}
	if r := run(t, "", "put", "x.db", "onlykey"); r.code != ExitError || !strings.Contains(r.errOut, "expected 2") {
		t.Fatalf("arity: %+v", r)
	}
	if r := run(t, "", "-bogus", "list", "x.db"); r.code != ExitError {
		t.Fatalf("bad flag: %+v", r)
	}
}

func TestHexAndTTL(t *testing.T) {
	db := filepath.Join(t.TempDir(), "h.db")
	ok(t, "create", db)
	ok(t, "-nosync", "-hex", "put", db, "00ff", "0xdeadbeef")
	if out := ok(t, "-hex", "get", db, "00FF"); out != "deadbeef\n" {
		t.Fatalf("hex get = %q", out)
	}
	if out := ok(t, "-hex", "list", db); out != "00ff\n" {
		t.Fatalf("hex list = %q", out)
	}
	if r := run(t, "", "-hex", "get", db, "zz"); r.code != ExitError || !strings.Contains(r.errOut, "bad hex") {
		t.Fatalf("bad hex: %+v", r)
	}
	// raw get of a binary value goes out verbatim
	if out := ok(t, "get", db, "\x00\xff"); out != "\xde\xad\xbe\xef\n" {
		t.Fatalf("raw get = %q", out)
	}

	ok(t, "-nosync", "-ttl", "1h", "put", db, "session", "abc")
	if out := ok(t, "get", db, "session"); out != "abc\n" {
		t.Fatalf("ttl get = %q", out)
	}
	ok(t, "-nosync", "-ttl", "1ns", "put", db, "gone", "x")
	if r := run(t, "", "get", db, "gone"); r.code != ExitNotFound {
		t.Fatalf("expired key returned: %+v", r)
	}
	if r := run(t, "", "-ttl", "soon", "put", db, "k", "v"); r.code != ExitError {
		t.Fatalf("bad ttl accepted: %+v", r)
	}
}

func TestShell(t *testing.T) {
	db := filepath.Join(t.TempDir(), "s.db")
	ok(t, "create", db)
	script := `
# comment line
store greeting "Hello, world!"
store language Go
store 'two words' "tab\there"
fetch greeting
fetch missing
count
keys
list
delete language
delete language
first
next greeting
bogus 1 2
store onlykey
fetch "unterminated
reorganize
count
ttl greeting 1h
fetch greeting
quit
store after-quit x
`
	r := run(t, script, "-nosync", "shell", db)
	if r.code != 0 {
		t.Fatalf("shell exit %d: %s", r.code, r.errOut)
	}
	want := []string{
		"Hello, world!",
		"There are 3 items in the database.",
	}
	for _, w := range want {
		if !strings.Contains(r.out, w) {
			t.Errorf("stdout missing %q:\n%s", w, r.out)
		}
	}
	if !strings.Contains(r.out, "two words tab\there\n") {
		t.Errorf("quoted args not handled:\n%s", r.out)
	}
	if !strings.Contains(r.out, "reorganised:") || !strings.Contains(r.out, "There are 2 items") {
		t.Errorf("reorganize/count:\n%s", r.out)
	}
	for _, w := range []string{"No such item found.", "unknown command \"bogus\"", "expected 2 argument(s)", "unterminated"} {
		if !strings.Contains(r.errOut, w) {
			t.Errorf("stderr missing %q:\n%s", w, r.errOut)
		}
	}
	// "No such item found." must appear exactly twice (fetch missing, second delete)
	if n := strings.Count(r.errOut, "No such item found."); n != 2 {
		t.Errorf("No such item found. appeared %d times:\n%s", n, r.errOut)
	}
	if r := run(t, "", "get", db, "after-quit"); r.code != ExitNotFound {
		t.Errorf("command after quit was executed")
	}
	// first/next walk from the shell must visit all keys
	r = run(t, "first\n", "shell", db)
	first := strings.TrimSpace(r.out)
	r = run(t, "next "+quote(first)+"\n", "shell", db)
	second := strings.TrimSpace(r.out)
	if first == second || first == "" || second == "" {
		t.Errorf("first=%q next=%q", first, second)
	}
	if r := run(t, "help\n", "shell", db); !strings.Contains(r.out, "Commands") {
		t.Errorf("shell help: %+v", r)
	}
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`store a b`, []string{"store", "a", "b"}},
		{`store "a b" c`, []string{"store", "a b", "c"}},
		{`store 'x "y" z' "q\"r"`, []string{"store", `x "y" z`, `q"r`}},
		{`a\ b c\\d "e\n"`, []string{"a b", `c\d`, "e\n"}},
		{`  spaced   out  `, []string{"spaced", "out"}},
		{`""`, []string{""}},
		{``, nil},
	}
	for _, c := range cases {
		got, err := tokenize(c.in)
		if err != nil {
			t.Fatalf("tokenize(%q): %v", c.in, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("tokenize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := tokenize(`"open`); err == nil {
		t.Error("unterminated quote accepted")
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
