package interp_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"goawk/interp"
	"goawk/parser"
)

// realAwk runs the system awk, returning its stdout. ok is false if awk is
// unavailable.
func realAwk(prog, input string, args ...string) (string, bool) {
	path, err := exec.LookPath("awk")
	if err != nil {
		return "", false
	}
	full := append(args, prog)
	c := exec.Command(path, full...)
	c.Stdin = strings.NewReader(input)
	var out bytes.Buffer
	c.Stdout = &out
	if err := c.Run(); err != nil {
		return "", false
	}
	return out.String(), true
}

// TestCompatWithRealAwk cross-checks a batch of deterministic programs against
// the system awk where available.
func TestCompatWithRealAwk(t *testing.T) {
	if _, ok := realAwk(`BEGIN{print 1}`, ""); !ok {
		t.Skip("system awk not available")
	}
	cases := []struct {
		prog  string
		input string
	}{
		{`{print $1}`, "a b c\nd e f\n"},
		{`{print $2, $1}`, "one two\nthree four\n"},
		{`{print NF, $0}`, "a b c\n\nx\n"},
		{`{s+=$1} END{print s}`, "1\n2\n3\n4\n"},
		{`$3 > 30`, "a b 10\nc d 40\ne f 35\n"},
		{`/^a/`, "apple\nbob\navocado\n"},
		{`$2 ~ /x/ {print $1}`, "a yx\nb yy\nc xz\n"},
		{`NR==1,NR==3`, "1\n2\n3\n4\n5\n"},
		{`BEGIN{print 2+3*4, 2^10, 17%5, -2^2}`, ""},
		{`BEGIN{printf "%-5s|%3d|%05.2f\n","ab",7,3.14159}`, ""},
		{`BEGIN{print substr("hello",2,3), index("hello","ll"), length("hello")}`, ""},
		{`BEGIN{print substr("hello",-1,3), substr("hello",0,2), substr("hello",2,100)}`, ""},
		{`{gsub(/o/,"0"); print}`, "foo boo\nzoo\n"},
		{`{n=split($0,a,":"); print n, a[1], a[n]}`, "x:y:z\n"},
		{`{if(match($0,/[0-9]+/))print RSTART,RLENGTH; else print "none"}`, "ab12cd\nxyz\n"},
		{`function max(a,b){return a>b?a:b} {print max($1,$2)}`, "3 7\n9 2\n"},
		{`BEGIN{print toupper("aBc"), tolower("aBc")}`, ""},
		{`{for(i=NF;i>=1;i--)printf "%s ",$i; print ""}`, "a b c\n"},
		{`{print ($1>0?"pos":"nonpos")}`, "5\n-3\n0\n"},
		{`BEGIN{x="10"; y="9"; print (x<y)}`, ""},
		{`{print ($1<9)}`, "10\n5\n"},
		{`{$2="X"; print}`, "a b c\n"},
		{`{NF=2; print}`, "a b c d\n"},
		{`BEGIN{print 1/3, 100000, 0.1+0.2}`, ""},
	}
	for _, tc := range cases {
		want, ok := realAwk(tc.prog, tc.input)
		if !ok {
			t.Fatalf("awk failed on %q", tc.prog)
		}
		p, err := parser.Parse(tc.prog)
		if err != nil {
			t.Errorf("parse error for %q: %v", tc.prog, err)
			continue
		}
		in := interp.New(p)
		var out, errb bytes.Buffer
		in.Run(nil, strings.NewReader(tc.input), &out, &errb)
		if out.String() != want {
			t.Errorf("mismatch for %q\ninput: %q\ngoawk: %q\nawk:   %q", tc.prog, tc.input, out.String(), want)
		}
	}
}

func TestCompatFS(t *testing.T) {
	prog := `{print $1, $3}`
	input := "a:b:c\nd:e:f\n"
	want, ok := realAwk(prog, input, "-F:")
	if !ok {
		t.Skip("system awk not available")
	}
	p, _ := parser.Parse(prog)
	in := interp.New(p)
	in.SetVarString("FS", ":")
	var out bytes.Buffer
	in.Run(nil, strings.NewReader(input), &out, &out)
	if out.String() != want {
		t.Errorf("FS mismatch: goawk=%q awk=%q", out.String(), want)
	}
}

func TestPipeOutputAndGetline(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	// "cmd" | getline
	out, _, _ := run(t, `BEGIN{ "printf '1\n2\n3\n'" | getline a; print "first="a }`, "", nil)
	if out != "first=1\n" {
		t.Errorf("cmd|getline got %q", out)
	}

	// print | "cmd" (pipe to cat reverses nothing but verifies plumbing)
	out2, _, _ := run(t, `{ print $1 | "cat" }`, "hello\nworld\n", nil)
	if out2 != "hello\nworld\n" {
		t.Errorf("print|cmd got %q", out2)
	}
}
