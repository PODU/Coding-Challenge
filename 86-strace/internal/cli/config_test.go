package cli

import "testing"

func TestParseCommand(t *testing.T) {
	cfg, err := Parse([]string{"-s", "64", "/bin/echo", "hi", "-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StrSize != 64 {
		t.Errorf("StrSize = %d, want 64", cfg.StrSize)
	}
	want := []string{"/bin/echo", "hi", "-x"}
	if len(cfg.Command) != len(want) {
		t.Fatalf("Command = %v, want %v", cfg.Command, want)
	}
	for i := range want {
		if cfg.Command[i] != want[i] {
			t.Errorf("Command[%d] = %q, want %q", i, cfg.Command[i], want[i])
		}
	}
	if cfg.HasPID {
		t.Error("HasPID should be false")
	}
}

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse([]string{"/bin/ls"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StrSize != DefaultStrSize {
		t.Errorf("StrSize = %d, want %d", cfg.StrSize, DefaultStrSize)
	}
}

func TestParseAttachAndFlags(t *testing.T) {
	cfg, err := Parse([]string{"-c", "-e", "trace=open,read", "-e", "write", "-p", "1234"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Count {
		t.Error("Count should be true")
	}
	if !cfg.HasPID || cfg.PID != 1234 {
		t.Errorf("PID = %d hasPID=%v, want 1234/true", cfg.PID, cfg.HasPID)
	}
	if len(cfg.Exprs) != 2 || cfg.Exprs[0] != "trace=open,read" || cfg.Exprs[1] != "write" {
		t.Errorf("Exprs = %v", cfg.Exprs)
	}
}

func TestParseAttachedValueForms(t *testing.T) {
	cfg, err := Parse([]string{"-p99", "-s8"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PID != 99 || cfg.StrSize != 8 {
		t.Errorf("PID=%d StrSize=%d, want 99/8", cfg.PID, cfg.StrSize)
	}
}

func TestParseHelp(t *testing.T) {
	cfg, err := Parse([]string{"-h"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Help {
		t.Error("Help should be true")
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string][]string{
		"both pid and command": {"-p", "5", "/bin/ls"},
		"neither":              {"-c"},
		"bad pid":              {"-p", "abc"},
		"bad size":             {"-s", "xyz"},
		"unknown option":       {"--bogus", "/bin/ls"},
		"missing value":        {"-p"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(args); err == nil {
				t.Errorf("expected error for %v", args)
			}
		})
	}
}
