package interp_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goawk/interp"
	"goawk/parser"
)

type config struct {
	fs   string
	vars [][2]string
	args []string
}

func run(t *testing.T, prog, input string, cfg *config) (string, string, int) {
	t.Helper()
	p, err := parser.Parse(prog)
	if err != nil {
		return "", err.Error(), 2
	}
	in := interp.New(p)
	if cfg != nil {
		if cfg.fs != "" {
			in.SetVarString("FS", cfg.fs)
		}
		for _, kv := range cfg.vars {
			in.SetVarString(kv[0], kv[1])
		}
	}
	var out, errb bytes.Buffer
	var args []string
	if cfg != nil {
		args = cfg.args
	}
	code := in.Run(args, strings.NewReader(input), &out, &errb)
	return out.String(), errb.String(), code
}

func TestGolden(t *testing.T) {
	cases := []struct {
		name  string
		prog  string
		input string
		cfg   *config
		want  string
	}{
		{"fields1", `{print $1}`, "a b c\nd e f\n", nil, "a\nd\n"},
		{"fields_order", `{print $2, $1}`, "a b\nc d\n", nil, "b a\nd c\n"},
		{"nf_dollar0", `{print NF": "$0}`, "a b c\n", nil, "3: a b c\n"},
		{"fs_colon", `{print $1}`, "root:x:0\nbob:y:1\n", &config{fs: ":"}, "root\nbob\n"},
		{"fs_regex", `{print NF}`, "a,b;c\n", &config{fs: "[,;]"}, "3\n"},
		{"fs_tab", `{print $2}`, "a\tb\tc\n", &config{fs: "\t"}, "b\n"},
		{"begin_end", `BEGIN{print "h"} END{print NR}`, "a\nb\n", nil, "h\n2\n"},
		{"only_begin", `BEGIN{print "x"}`, "ignored\n", nil, "x\n"},
		{"compare_num", `$3>30`, "a b 10\nc d 40\n", nil, "c d 40\n"},
		{"compare_str", `$1=="bob"`, "alice 1\nbob 2\n", nil, "bob 2\n"},
		{"regex_pat", `/^a/`, "apple\nbanana\navocado\n", nil, "apple\navocado\n"},
		{"field_match", `$2 ~ /x/`, "a yx\nb yy\nc xz\n", nil, "a yx\nc xz\n"},
		{"field_notmatch", `$2 !~ /x/`, "a yx\nb yy\n", nil, "b yy\n"},
		{"logical_and", `$3>20 && $1!="x"`, "x a 30\ny b 30\nz c 10\n", nil, "y b 30\n"},
		{"logical_or", `$1=="a" || $1=="c"`, "a\nb\nc\n", nil, "a\nc\n"},
		{"not", `!/a/`, "cat\ndog\n", nil, "dog\n"},
		{"range", `/START/,/END/`, "x\nSTART\nm\nEND\ny\n", nil, "START\nm\nEND\n"},
		{"arith_sum", `{s+=$1} END{print s}`, "1\n2\n3\n", nil, "6\n"},
		{"arith_prec", `BEGIN{print 2+3*4}`, "", nil, "14\n"},
		{"pow_right", `BEGIN{print 2^3^2}`, "", nil, "512\n"},
		{"neg_pow", `BEGIN{print -2^2}`, "", nil, "-4\n"},
		{"mod", `BEGIN{print 17%5}`, "", nil, "2\n"},
		{"compound_assign", `{$1*=2; print}`, "3 4\n", nil, "6 4\n"},
		{"concat", `BEGIN{print "a" 1}`, "", nil, "a1\n"},
		{"concat_var", `{print $1 $2}`, "foo bar\n", nil, "foobar\n"},
		{"num_fmt", `BEGIN{print 0.1+0.2}`, "", nil, "0.3\n"},
		{"div_fmt", `BEGIN{print 1/3}`, "", nil, "0.333333\n"},
		{"int_print", `BEGIN{print 6/2}`, "", nil, "3\n"},
		{"if_else", `{if($1>0)print "p"; else print "n"}`, "5\n-2\n", nil, "p\nn\n"},
		{"for_loop", `{for(i=1;i<=NF;i++)print $i}`, "a b\n", nil, "a\nb\n"},
		{"while_loop", `BEGIN{i=0;while(i<3){printf "%d",i;i++};print ""}`, "", nil, "012\n"},
		{"do_while", `BEGIN{i=0;do{printf "%d",i;i++}while(i<3);print ""}`, "", nil, "012\n"},
		{"next", `NR==1{next} {print}`, "a\nb\nc\n", nil, "b\nc\n"},
		{"ternary", `{print ($1>0?"+":"-")}`, "3\n-1\n", nil, "+\n-\n"},
		{"array_count", `{c[$1]++} END{print c["a"], c["b"]}`, "a\nb\na\n", nil, "2 1\n"},
		{"in_op", `BEGIN{a["x"]=1; print ("x" in a), ("y" in a)}`, "", nil, "1 0\n"},
		{"delete", `BEGIN{a["x"]=1; delete a["x"]; print ("x" in a)}`, "", nil, "0\n"},
		{"multi_sub", `BEGIN{a[1,2]=5; print a[1,2]; print (1,2) in a}`, "", nil, "5\n1\n"},
		{"printf_width", `{printf "%-5s%3d\n",$1,$2}`, "ab 7\n", nil, "ab     7\n"},
		{"printf_star", `BEGIN{printf "%*d\n",4,7}`, "", nil, "   7\n"},
		{"printf_c_num", `BEGIN{printf "%c\n",65}`, "", nil, "A\n"},
		{"printf_pct", `BEGIN{printf "%d%%\n",50}`, "", nil, "50%\n"},
		{"length", `{print length}`, "hello\n", nil, "5\n"},
		{"length_arg", `BEGIN{print length("abcd")}`, "", nil, "4\n"},
		{"substr3", `BEGIN{print substr("hello",2,3)}`, "", nil, "ell\n"},
		{"substr2", `BEGIN{print substr("hello",3)}`, "", nil, "llo\n"},
		{"substr_oob", `BEGIN{print substr("hello",0,3)}`, "", nil, "hel\n"},
		{"substr_neg", `BEGIN{print substr("hello",-1,3)}`, "", nil, "hel\n"},
		{"index", `BEGIN{print index("hello","ll")}`, "", nil, "3\n"},
		{"split", `BEGIN{n=split("a:b:c",x,":");print n,x[1],x[3]}`, "", nil, "3 a c\n"},
		{"split_empty", `BEGIN{n=split("abc",x,"");print n,x[1],x[3]}`, "", nil, "3 a c\n"},
		{"gsub", `{n=gsub(/o/,"0");print n, $0}`, "foo boo\n", nil, "4 f00 b00\n"},
		{"sub", `{sub(/o/,"0");print}`, "foo\n", nil, "f0o\n"},
		{"gsub_amp", `{gsub(/o/,"[&]");print}`, "foo\n", nil, "f[o][o]\n"},
		{"match_rstart", `{if(match($0,/[0-9]+/))print RSTART,RLENGTH}`, "abc123de\n", nil, "4 3\n"},
		{"match_none", `{print match($0,/z/), RSTART, RLENGTH}`, "abc\n", nil, "0 0 -1\n"},
		{"toupper", `BEGIN{print toupper("aB"), tolower("aB")}`, "", nil, "AB ab\n"},
		{"sprintf", `BEGIN{print sprintf("[%05.2f]",3.14159)}`, "", nil, "[03.14]\n"},
		{"user_fn", `function max(a,b){return a>b?a:b} BEGIN{print max(3,7)}`, "", nil, "7\n"},
		{"recursion", `function f(n){return n<=1?1:n*f(n-1)} BEGIN{print f(5)}`, "", nil, "120\n"},
		{"fn_locals", `function inc(x,  t){t=x+1; return t} BEGIN{print inc(4)}`, "", nil, "5\n"},
		{"fn_array_ref", `function fill(a){a[1]="z"} BEGIN{fill(arr); print arr[1]}`, "", nil, "z\n"},
		{"math", `BEGIN{print int(3.9), sqrt(16), 2^10}`, "", nil, "3 4 1024\n"},
		{"atan2", `BEGIN{printf "%.4f\n", atan2(1,1)}`, "", nil, "0.7854\n"},
		{"incdec", `BEGIN{i=5; print i++, i, ++i}`, "", nil, "5 6 7\n"},
		{"uninit_num", `BEGIN{print x+0}`, "", nil, "0\n"},
		{"uninit_str", `BEGIN{print "["x"]"}`, "", nil, "[]\n"},
		{"dollar_expr", `{print $(NF-1)}`, "a b c\n", nil, "b\n"},
		{"assign_dollar0", `{$0="x y z"; print NF, $2}`, "a b\n", nil, "3 y\n"},
		{"set_nf", `{NF=2; print}`, "a b c d\n", nil, "a b\n"},
		{"strnum_cmp", `{print ($1<9)}`, "10\n", nil, "0\n"},
		{"str_cmp", `BEGIN{print ("10"<"9")}`, "", nil, "1\n"},
		{"printf_g", `BEGIN{printf "%g\n", 100000}`, "", nil, "100000\n"},
		{"getline_var_main", `NR==1{getline x; print x; print $0}`, "a\nb\nc\n", nil, "b\na\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errs, code := run(t, tc.prog, tc.input, tc.cfg)
			if out != tc.want {
				t.Errorf("output mismatch\nprog:  %s\ninput: %q\ngot:   %q\nwant:  %q\nstderr:%s code:%d",
					tc.prog, tc.input, out, tc.want, errs, code)
			}
		})
	}
}

func TestOFSORS(t *testing.T) {
	out, _, _ := run(t, `{$1=$1; print}`, "a b c\n", &config{vars: [][2]string{{"OFS", ","}, {"ORS", ";"}}})
	if out != "a,b,c;" {
		t.Errorf("got %q", out)
	}
}

func TestParagraphRS(t *testing.T) {
	out, _, _ := run(t, `{print NR": "$1"/"$2}`, "a b\nc d\n\ne f\ng h\n", &config{vars: [][2]string{{"RS", ""}}})
	want := "1: a/b\n2: e/f\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestCustomRS(t *testing.T) {
	out, _, _ := run(t, `{print NR, $0}`, "a;b;c", &config{vars: [][2]string{{"RS", ";"}}})
	want := "1 a\n2 b\n3 c\n"
	if out != want {
		t.Errorf("got %q want %q", out, want)
	}
}

func TestExitCode(t *testing.T) {
	out, _, code := run(t, `BEGIN{exit 4} END{print "end"}`, "", nil)
	if code != 4 {
		t.Errorf("exit code = %d, want 4", code)
	}
	if out != "end\n" {
		t.Errorf("END did not run, out=%q", out)
	}
}

func TestArgvVars(t *testing.T) {
	out, _, _ := run(t, `BEGIN{print x, ARGC}`, "", &config{vars: [][2]string{{"x", "5"}}, args: []string{"file1"}})
	if out != "5 2\n" {
		t.Errorf("got %q want %q", out, "5 2\n")
	}
}

func TestNegativeFieldError(t *testing.T) {
	_, _, code := run(t, `BEGIN{$0="a"; print $(-1)}`, "", nil)
	if code != 2 {
		t.Errorf("expected exit 2 for $(-1), got %d", code)
	}
}

func TestDivZeroError(t *testing.T) {
	_, errs, code := run(t, `BEGIN{print 1/0}`, "", nil)
	if code != 2 {
		t.Errorf("expected exit 2 for div by zero, got %d", code)
	}
	if !strings.Contains(errs, "division by zero") {
		t.Errorf("stderr missing message: %q", errs)
	}
}

func TestParseError(t *testing.T) {
	_, _, code := run(t, `BEGIN{print `, "", nil)
	if code != 2 {
		t.Errorf("expected parse error exit 2, got %d", code)
	}
}

func TestMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("x\ny\n"), 0644)
	os.WriteFile(f2, []byte("z\n"), 0644)

	p, err := parser.Parse(`{print FNR, NR, $0}`)
	if err != nil {
		t.Fatal(err)
	}
	in := interp.New(p)
	var out bytes.Buffer
	in.Run([]string{f1, f2}, strings.NewReader(""), &out, &out)
	want := "1 1 x\n2 2 y\n1 3 z\n"
	if out.String() != want {
		t.Errorf("got %q want %q", out.String(), want)
	}
}

func TestProgFile(t *testing.T) {
	dir := t.TempDir()
	pf := filepath.Join(dir, "prog.awk")
	os.WriteFile(pf, []byte("BEGIN{print \"from file\"}\n"), 0644)
	data, _ := os.ReadFile(pf)
	out, _, _ := run(t, string(data), "", nil)
	if out != "from file\n" {
		t.Errorf("got %q", out)
	}
}

func TestRedirectAndGetlineFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	prog := `{print $1 > "` + filepath.ToSlash(outFile) + `"}`
	_, _, _ = run(t, prog, "one two\nthree four\n", nil)
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file not written: %v", err)
	}
	if string(data) != "one\nthree\n" {
		t.Errorf("got %q", string(data))
	}

	// now read it back with getline < file
	readProg := `BEGIN{ while ((getline line < "` + filepath.ToSlash(outFile) + `") > 0) print "got " line }`
	out, _, _ := run(t, readProg, "", nil)
	if out != "got one\ngot three\n" {
		t.Errorf("getline<file got %q", out)
	}
}

func TestForInSum(t *testing.T) {
	out, _, _ := run(t, `{c[$1]++} END{n=0; for(k in c) n+=c[k]; print n}`, "a\nb\na\nc\na\n", nil)
	if out != "5\n" {
		t.Errorf("got %q", out)
	}
}
