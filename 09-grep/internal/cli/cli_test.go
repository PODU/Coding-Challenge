package cli

import (
	"reflect"
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Config
		wantErr bool
	}{
		{
			name: "short flags separate",
			args: []string{"-i", "-v", "-n", "foo", "a.txt"},
			want: Config{Pattern: "foo", Paths: []string{"a.txt"}, IgnoreCase: true, Invert: true, LineNumbers: true},
		},
		{
			name: "clustered short flags",
			args: []string{"-ivn", "foo"},
			want: Config{Pattern: "foo", Paths: []string{}, IgnoreCase: true, Invert: true, LineNumbers: true},
		},
		{
			name: "long flags",
			args: []string{"--recursive", "--count", "--word-regexp", "foo", "dir"},
			want: Config{Pattern: "foo", Paths: []string{"dir"}, Recursive: true, Count: true, WordMatch: true},
		},
		{
			name: "double dash separator lets pattern look like a flag",
			args: []string{"--", "-v", "file.txt"},
			want: Config{Pattern: "-v", Paths: []string{"file.txt"}},
		},
		{
			name: "bare dash is a stdin path",
			args: []string{"foo", "-"},
			want: Config{Pattern: "foo", Paths: []string{"-"}},
		},
		{
			name: "no paths means stdin",
			args: []string{"foo"},
			want: Config{Pattern: "foo", Paths: []string{}},
		},
		{
			name: "empty pattern is valid",
			args: []string{""},
			want: Config{Pattern: "", Paths: []string{}},
		},
		{
			name:    "missing pattern",
			args:    []string{"-i"},
			wantErr: true,
		},
		{
			name:    "unknown short flag",
			args:    []string{"-z", "foo"},
			wantErr: true,
		},
		{
			name:    "unknown long flag",
			args:    []string{"--nope", "foo"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Errorf("Parse() = %+v, want %+v", *got, tc.want)
			}
		})
	}
}

func TestParseHelp(t *testing.T) {
	for _, a := range [][]string{{"-h"}, {"--help"}} {
		got, err := Parse(a)
		if err != nil {
			t.Fatalf("Parse(%v) error: %v", a, err)
		}
		if !got.Help {
			t.Errorf("Parse(%v).Help = false, want true", a)
		}
	}
}
