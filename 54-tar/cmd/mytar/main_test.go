package main

import "testing"

func TestParseArgsBundled(t *testing.T) {
	o, err := parseArgs([]string{"-cvf", "out.tar", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.create || !o.verbose || o.file != "out.tar" {
		t.Errorf("bundled parse wrong: %+v", o)
	}
	if len(o.args) != 2 || o.args[0] != "a" || o.args[1] != "b" {
		t.Errorf("positionals wrong: %v", o.args)
	}
}

func TestParseArgsSeparate(t *testing.T) {
	o, err := parseArgs([]string{"-x", "-v", "-f", "in.tar"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.extract || !o.verbose || o.file != "in.tar" {
		t.Errorf("separate parse wrong: %+v", o)
	}
}

func TestParseArgsFileGlued(t *testing.T) {
	o, err := parseArgs([]string{"-tfarchive.tar"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.list || o.file != "archive.tar" {
		t.Errorf("glued -f parse wrong: %+v", o)
	}
}

func TestParseArgsModeExclusive(t *testing.T) {
	if _, err := parseArgs([]string{"-cx"}); err == nil {
		t.Error("expected error for two modes")
	}
	if _, err := parseArgs([]string{"-v"}); err == nil {
		t.Error("expected error for no mode")
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	if _, err := parseArgs([]string{"-cz"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestParseArgsMissingFileArg(t *testing.T) {
	if _, err := parseArgs([]string{"-cf"}); err == nil {
		t.Error("expected error when -f has no argument")
	}
}
