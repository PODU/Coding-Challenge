package cli

import (
	"errors"
	"testing"

	"github.com/codingchallenges/cut/internal/cut"
)

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantMode  cut.Mode
		wantDelim byte
		wantFiles []string
	}{
		{"fields attached", []string{"-f2", "sample.tsv"}, cut.ModeFields, '\t', []string{"sample.tsv"}},
		{"fields separate", []string{"-f", "2", "sample.tsv"}, cut.ModeFields, '\t', []string{"sample.tsv"}},
		{"delim attached", []string{"-d,", "-f1"}, cut.ModeFields, ',', []string{"-"}},
		{"long fields", []string{"--fields=1,2", "f"}, cut.ModeFields, '\t', []string{"f"}},
		{"long delim eq", []string{"--delimiter=:", "-f1"}, cut.ModeFields, ':', []string{"-"}},
		{"bytes", []string{"-b1-5"}, cut.ModeBytes, '\t', []string{"-"}},
		{"chars", []string{"-c1-5"}, cut.ModeChars, '\t', []string{"-"}},
		{"clustered s", []string{"-sf1", "-d,"}, cut.ModeFields, ',', []string{"-"}},
		{"stdin dash", []string{"-f1", "-"}, cut.ModeFields, '\t', []string{"-"}},
		{"no file means stdin", []string{"-f1"}, cut.ModeFields, '\t', []string{"-"}},
		{"double dash files", []string{"-f1", "--", "-weird"}, cut.ModeFields, '\t', []string{"-weird"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", cfg.Mode, tt.wantMode)
			}
			if cfg.Delim != tt.wantDelim {
				t.Errorf("delim = %q, want %q", cfg.Delim, tt.wantDelim)
			}
			if len(cfg.Files) != len(tt.wantFiles) {
				t.Fatalf("files = %v, want %v", cfg.Files, tt.wantFiles)
			}
			for i := range cfg.Files {
				if cfg.Files[i] != tt.wantFiles[i] {
					t.Errorf("files = %v, want %v", cfg.Files, tt.wantFiles)
				}
			}
		})
	}
}

func TestParse_OutputDelimiter(t *testing.T) {
	cfg, err := Parse([]string{"-d-", "-f2", "--output-delimiter=:"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OutDelimSet || cfg.OutDelim != ":" {
		t.Errorf("OutDelim = %q set=%v, want \":\" set=true", cfg.OutDelim, cfg.OutDelimSet)
	}
}

func TestParse_Complement(t *testing.T) {
	cfg, err := Parse([]string{"-f1", "--complement"})
	if err != nil {
		t.Fatal(err)
	}
	// Complement of field 1: position 1 not selected, 2 selected.
	if cfg.Sel.Selected(1) {
		t.Error("complement: position 1 should not be selected")
	}
	if !cfg.Sel.Selected(2) {
		t.Error("complement: position 2 should be selected")
	}
}

func TestParse_UsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no mode", []string{"file"}},
		{"two modes", []string{"-f1", "-b1"}},
		{"d with bytes", []string{"-b1", "-d,"}},
		{"s with chars", []string{"-c1", "-s"}},
		{"multichar delim", []string{"-d,,", "-f1"}},
		{"empty delim", []string{"-d", "", "-f1"}},
		{"field zero", []string{"-f0"}},
		{"decreasing range", []string{"-f5-2"}},
		{"non-numeric", []string{"-f", "a"}},
		{"unknown short", []string{"-z"}},
		{"unknown long", []string{"--bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args)
			if err == nil {
				t.Fatalf("expected usage error, got nil")
			}
			var ue *UsageError
			if !errors.As(err, &ue) {
				t.Errorf("expected *UsageError, got %T: %v", err, err)
			}
		})
	}
}

func TestParse_HelpVersion(t *testing.T) {
	if _, err := Parse([]string{"--help"}); !errors.Is(err, ErrHelp) {
		t.Errorf("--help: got %v, want ErrHelp", err)
	}
	if _, err := Parse([]string{"--version"}); !errors.Is(err, ErrVersion) {
		t.Errorf("--version: got %v, want ErrVersion", err)
	}
}
