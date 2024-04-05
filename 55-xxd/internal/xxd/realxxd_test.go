package xxd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAgainstRealXxd diffs our forward output against the system `xxd` when it
// is available on PATH. On machines without `xxd` (e.g. bare Windows) the test
// is skipped and the checked-in golden strings in TestDumpGolden are the oracle.
func TestAgainstRealXxd(t *testing.T) {
	xxdPath, err := exec.LookPath("xxd")
	if err != nil {
		t.Skip("real xxd not on PATH; relying on golden strings")
	}

	inputs := [][]byte{
		f1,
		allBytes(),
		{},
		{'A'},
		[]byte("0123456789abcdefX"),
	}
	flagSets := [][]string{
		{},
		{"-p"},
		{"-e"},
		{"-e", "-g", "2"},
		{"-c", "8"},
		{"-c", "32"},
		{"-g", "1"},
		{"-g", "8"},
		{"-g", "0"},
		{"-l", "10"},
		{"-s", "4"},
		{"-s", "4", "-l", "6"},
		{"-c", "8", "-g", "4"},
		{"-e", "-g", "8", "-c", "16"},
	}

	dir := t.TempDir()
	for ii, in := range inputs {
		// Real xxd cannot seek a pipe, so feed it a real file argument.
		path := filepath.Join(dir, "in"+string(rune('a'+ii)))
		if err := os.WriteFile(path, in, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, fs := range flagSets {
			cfg := configFromFlags(t, fs)

			var ours bytes.Buffer
			if err := Dump(bytes.NewReader(in), &ours, cfg); err != nil {
				t.Fatalf("Dump %v: %v", fs, err)
			}

			args := append(append([]string{}, fs...), path)
			want, err := exec.Command(xxdPath, args...).Output()
			if err != nil {
				t.Fatalf("real xxd %v: %v", fs, err)
			}

			if !bytes.Equal(ours.Bytes(), want) {
				t.Errorf("flags %v len=%d mismatch\n ours: %q\n xxd:  %q", fs, len(in), ours.String(), string(want))
			}
		}
	}
}

// configFromFlags builds a Config from a small subset of CLI flags, mirroring
// the defaults applied in cmd/xxd.
func configFromFlags(t *testing.T, flags []string) Config {
	t.Helper()
	cfg := NewConfig()
	groupSet := false
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case "-e":
			cfg.LittleEndian = true
		case "-p":
			cfg.Plain = true
		case "-c":
			i++
			cfg.Cols = atoi(t, flags[i])
		case "-g":
			i++
			cfg.Group = atoi(t, flags[i])
			groupSet = true
		case "-l":
			i++
			cfg.Length = int64(atoi(t, flags[i]))
		case "-s":
			i++
			cfg.Seek = int64(atoi(t, flags[i]))
		default:
			t.Fatalf("unsupported flag in test harness: %s", flags[i])
		}
	}
	if cfg.LittleEndian && !groupSet {
		cfg.Group = 4
	}
	if cfg.Plain {
		// Real xxd uses 30 bytes/line in plain mode unless -c overrides.
		seen := false
		for _, f := range flags {
			if f == "-c" {
				seen = true
			}
		}
		if !seen {
			cfg.Cols = DefaultPlainCols
		}
	}
	return cfg
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("bad number %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
