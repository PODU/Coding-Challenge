package cli

import (
	"reflect"
	"strings"
	"testing"
)

func mustParse(t *testing.T, args ...string) *Options {
	t.Helper()
	o, err := Parse(args)
	if err != nil {
		t.Fatalf("Parse(%v) failed: %v", args, err)
	}
	return o
}

func TestCommandsArePositional(t *testing.T) {
	o := mustParse(t, "sleep 0.1", "sleep 0.2")
	want := []string{"sleep 0.1", "sleep 0.2"}
	if !reflect.DeepEqual(o.Commands, want) {
		t.Errorf("commands = %v, want %v", o.Commands, want)
	}
}

func TestDoubleDashStopsOptionParsing(t *testing.T) {
	o := mustParse(t, "-r", "3", "--", "--not-a-flag", "-N")
	if o.Runs != 3 {
		t.Errorf("runs = %d, want 3", o.Runs)
	}
	want := []string{"--not-a-flag", "-N"}
	if !reflect.DeepEqual(o.Commands, want) {
		t.Errorf("commands = %v, want %v", o.Commands, want)
	}
}

func TestValueSyntaxes(t *testing.T) {
	for _, args := range [][]string{
		{"-r", "7", "cmd"},
		{"-r7", "cmd"},
		{"-r=7", "cmd"},
		{"--runs", "7", "cmd"},
		{"--runs=7", "cmd"},
	} {
		o := mustParse(t, args...)
		if o.Runs != 7 {
			t.Errorf("%v: runs = %d, want 7", args, o.Runs)
		}
	}
}

func TestShortBooleanCluster(t *testing.T) {
	o := mustParse(t, "-Ni", "cmd")
	if o.Shell.Enabled() {
		t.Error("-N did not disable the shell")
	}
	if !o.IgnoreFail {
		t.Error("-i did not set IgnoreFail")
	}
	if len(o.Commands) != 1 || o.Commands[0] != "cmd" {
		t.Errorf("commands = %v, want [cmd]", o.Commands)
	}
}

func TestParameterScanTakesThreeOperands(t *testing.T) {
	o := mustParse(t, "-P", "n", "1", "8", "-D", "2", "cmd {n}")
	if len(o.Scans) != 1 {
		t.Fatalf("scans = %v, want one", o.Scans)
	}
	got := o.Scans[0]
	if got.Var != "n" || got.Min != "1" || got.Max != "8" || got.Step != "2" {
		t.Errorf("scan = %+v", got)
	}
}

func TestParameterListSplitsAndUnescapes(t *testing.T) {
	o := mustParse(t, "-L", "x", `a,b\,c,d`, "cmd {x}")
	want := []string{"a", "b,c", "d"}
	if !reflect.DeepEqual(o.Lists[0].Values, want) {
		t.Errorf("values = %v, want %v", o.Lists[0].Values, want)
	}
}

func TestRepeatableHooksPairWithCommands(t *testing.T) {
	o := mustParse(t, "-p", "one", "-p", "two", "a", "b")
	if o.PrepareFor(0) != "one" || o.PrepareFor(1) != "two" {
		t.Errorf("prepare pairing wrong: %v", o.Prepare)
	}
}

func TestSinglePrepareAppliesToEveryCommand(t *testing.T) {
	o := mustParse(t, "-p", "sync", "a", "b", "c")
	for i := range 3 {
		if o.PrepareFor(i) != "sync" {
			t.Errorf("PrepareFor(%d) = %q, want sync", i, o.PrepareFor(i))
		}
	}
}

func TestShowOutputImpliesInherit(t *testing.T) {
	o := mustParse(t, "--show-output", "cmd")
	if o.OutputMode != OutputInherit {
		t.Errorf("output mode = %v, want inherit", o.OutputMode)
	}
}

func TestOutputFilePath(t *testing.T) {
	o := mustParse(t, "--output", "log.txt", "cmd")
	if o.OutputMode != OutputFile || o.OutputFile != "log.txt" {
		t.Errorf("output = %v %q", o.OutputMode, o.OutputFile)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no commands", nil, "no commands given"},
		{"unknown option", []string{"--nope", "cmd"}, "unknown option"},
		{"missing value", []string{"cmd", "-r"}, "expected 1 value"},
		{"bad number", []string{"-r", "x", "cmd"}, "not a non-negative integer"},
		{"bad time unit", []string{"-u", "fortnight", "cmd"}, "unknown unit"},
		{"bad sort", []string{"--sort", "sideways", "cmd"}, "unknown method"},
		{"bad style", []string{"--style", "fancy", "cmd"}, "unknown style"},
		{"max below min", []string{"-m", "20", "-M", "5", "cmd"}, "must not be smaller"},
		{"too many names", []string{"-n", "a", "-n", "b", "cmd"}, "--command-name values"},
		{"prepare count mismatch", []string{"-p", "a", "-p", "b", "-p", "c", "x", "y"}, "once per command"},
		{"step without scan", []string{"-D", "2", "cmd"}, "requires --parameter-scan"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.args)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestHelpAndVersionSkipValidation(t *testing.T) {
	if o := mustParse(t, "--help"); !o.ShowHelp {
		t.Error("--help not recognised")
	}
	if o := mustParse(t, "-V"); !o.ShowVersion {
		t.Error("-V not recognised")
	}
}

func TestShellSelection(t *testing.T) {
	o := mustParse(t, "--shell", "none", "cmd")
	if o.Shell.Enabled() {
		t.Error("--shell=none should disable the shell")
	}
	o = mustParse(t, "--shell", "bash -f", "cmd")
	want := []string{"bash", "-f", "-c"}
	if !reflect.DeepEqual(o.Shell.Argv, want) {
		t.Errorf("shell argv = %v, want %v", o.Shell.Argv, want)
	}
}
