package main

import (
	"reflect"
	"testing"
)

func argvs(jobs []Job) [][]string {
	out := make([][]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Argv
	}
	return out
}

func TestBuildAppendNoLimit(t *testing.T) {
	jobs := Build(BuildConfig{Prefix: []string{"cat"}, RunIfEmpty: true},
		[]string{"f1", "f2", "f3"})
	want := [][]string{{"cat", "f1", "f2", "f3"}}
	if !reflect.DeepEqual(argvs(jobs), want) {
		t.Fatalf("got %#v, want %#v", argvs(jobs), want)
	}
}

func TestBuildMaxArgs(t *testing.T) {
	items := []string{"1", "2", "3", "4", "5", "6", "7"}
	tests := []struct {
		n    int
		want [][]string
	}{
		{1, [][]string{{"e", "1"}, {"e", "2"}, {"e", "3"}, {"e", "4"}, {"e", "5"}, {"e", "6"}, {"e", "7"}}},
		{2, [][]string{{"e", "1", "2"}, {"e", "3", "4"}, {"e", "5", "6"}, {"e", "7"}}},
		{3, [][]string{{"e", "1", "2", "3"}, {"e", "4", "5", "6"}, {"e", "7"}}},
	}
	for _, tt := range tests {
		jobs := Build(BuildConfig{Prefix: []string{"e"}, MaxArgs: tt.n, RunIfEmpty: true}, items)
		if !reflect.DeepEqual(argvs(jobs), tt.want) {
			t.Fatalf("-n %d: got %#v, want %#v", tt.n, argvs(jobs), tt.want)
		}
	}
}

func TestBuildDefaultCommand(t *testing.T) {
	prefix := defaultCommand()
	jobs := Build(BuildConfig{Prefix: prefix, RunIfEmpty: true}, []string{"hi"})
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	first := jobs[0].Argv[0]
	if first != "echo" && first != "cmd" {
		t.Fatalf("default command should start with echo or cmd, got %q", first)
	}
}

func TestBuildMaxChars(t *testing.T) {
	// prefix "e" -> base len 2 ("e" + sep). Each "aa" adds 3.
	jobs := Build(BuildConfig{Prefix: []string{"e"}, MaxChars: 8, RunIfEmpty: true},
		[]string{"aa", "aa", "aa", "aa"})
	// base 2; +3 ->5 (aa), +3 ->8 (aa) ok, next +3 ->11 >8 flush. So 2 per batch.
	want := [][]string{{"e", "aa", "aa"}, {"e", "aa", "aa"}}
	if !reflect.DeepEqual(argvs(jobs), want) {
		t.Fatalf("got %#v, want %#v", argvs(jobs), want)
	}
}

func TestBuildOversizedItem(t *testing.T) {
	// A single item larger than -s is still emitted alone.
	jobs := Build(BuildConfig{Prefix: []string{"e"}, MaxChars: 4, RunIfEmpty: true},
		[]string{"hugeitem", "x"})
	want := [][]string{{"e", "hugeitem"}, {"e", "x"}}
	if !reflect.DeepEqual(argvs(jobs), want) {
		t.Fatalf("got %#v, want %#v", argvs(jobs), want)
	}
}

func TestBuildReplace(t *testing.T) {
	jobs := Build(BuildConfig{
		Prefix:     []string{"mv", "{}", "backup/{}"},
		Replace:    true,
		ReplaceStr: "{}",
	}, []string{"f1", "f2"})
	want := [][]string{
		{"mv", "f1", "backup/f1"},
		{"mv", "f2", "backup/f2"},
	}
	if !reflect.DeepEqual(argvs(jobs), want) {
		t.Fatalf("got %#v, want %#v", argvs(jobs), want)
	}
}

func TestBuildReplaceMultipleOccurrences(t *testing.T) {
	jobs := Build(BuildConfig{
		Prefix:     []string{"cp", "{}", "{}.bak", "and-{}"},
		Replace:    true,
		ReplaceStr: "{}",
	}, []string{"x"})
	want := [][]string{{"cp", "x", "x.bak", "and-x"}}
	if !reflect.DeepEqual(argvs(jobs), want) {
		t.Fatalf("got %#v, want %#v", argvs(jobs), want)
	}
}

func TestBuildReplaceEmptyInput(t *testing.T) {
	jobs := Build(BuildConfig{
		Prefix:     []string{"echo", "{}"},
		Replace:    true,
		ReplaceStr: "{}",
		RunIfEmpty: true,
	}, nil)
	if len(jobs) != 0 {
		t.Fatalf("replace mode with empty input should yield no jobs, got %#v", argvs(jobs))
	}
}

func TestBuildEmptyInput(t *testing.T) {
	// Default (POSIX): run once with no extra args.
	jobs := Build(BuildConfig{Prefix: []string{"echo"}, RunIfEmpty: true}, nil)
	want := [][]string{{"echo"}}
	if !reflect.DeepEqual(argvs(jobs), want) {
		t.Fatalf("default empty: got %#v, want %#v", argvs(jobs), want)
	}

	// With -r: run nothing.
	jobs = Build(BuildConfig{Prefix: []string{"echo"}, RunIfEmpty: false}, nil)
	if len(jobs) != 0 {
		t.Fatalf("-r empty: expected 0 jobs, got %#v", argvs(jobs))
	}
}
