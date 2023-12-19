package cli

import (
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want Options
	}{
		{
			name: "bare url",
			argv: []string{"http://example.com"},
			want: Options{URL: "http://example.com", MaxRedirs: 20},
		},
		{
			name: "short method separate value",
			argv: []string{"-X", "DELETE", "http://h"},
			want: Options{URL: "http://h", Method: "DELETE", MaxRedirs: 20},
		},
		{
			name: "short method attached value",
			argv: []string{"-XPUT", "http://h"},
			want: Options{URL: "http://h", Method: "PUT", MaxRedirs: 20},
		},
		{
			name: "long method equals",
			argv: []string{"--request=PATCH", "http://h"},
			want: Options{URL: "http://h", Method: "PATCH", MaxRedirs: 20},
		},
		{
			name: "repeated headers",
			argv: []string{"-H", "A: 1", "-H", "B: 2", "http://h"},
			want: Options{URL: "http://h", Headers: []string{"A: 1", "B: 2"}, MaxRedirs: 20},
		},
		{
			name: "header attached",
			argv: []string{"-HX-Foo:bar", "http://h"},
			want: Options{URL: "http://h", Headers: []string{"X-Foo:bar"}, MaxRedirs: 20},
		},
		{
			name: "data implies nothing here but sets flag",
			argv: []string{"-d", "k=v", "http://h"},
			want: Options{URL: "http://h", Data: "k=v", HasData: true, MaxRedirs: 20},
		},
		{
			name: "combined boolean cluster",
			argv: []string{"-vL", "http://h"},
			want: Options{URL: "http://h", Verbose: true, Location: true, MaxRedirs: 20},
		},
		{
			name: "combined cluster ending in value flag",
			argv: []string{"-viH", "A: 1", "http://h"},
			want: Options{URL: "http://h", Verbose: true, Include: true, Headers: []string{"A: 1"}, MaxRedirs: 20},
		},
		{
			name: "long flags",
			argv: []string{"--verbose", "--location", "--insecure", "--include", "http://h"},
			want: Options{URL: "http://h", Verbose: true, Location: true, Insecure: true, Include: true, MaxRedirs: 20},
		},
		{
			name: "output and max-redirs",
			argv: []string{"-o", "out.txt", "--max-redirs", "5", "http://h"},
			want: Options{URL: "http://h", Output: "out.txt", MaxRedirs: 5},
		},
		{
			name: "double dash positional",
			argv: []string{"-v", "--", "http://h"},
			want: Options{URL: "http://h", Verbose: true, MaxRedirs: 20},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.argv)
			if err != nil {
				t.Fatalf("Parse(%v) error: %v", tc.argv, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%v)\n got = %+v\nwant = %+v", tc.argv, got, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]string{"-v"}); err == nil {
		t.Error("expected error for missing URL")
	}
	if _, err := Parse([]string{"--bogus", "http://h"}); err == nil {
		t.Error("expected error for unknown long flag")
	}
	if _, err := Parse([]string{"-Z", "http://h"}); err == nil {
		t.Error("expected error for unknown short flag")
	}
	if _, err := Parse([]string{"-X"}); err == nil {
		t.Error("expected error for -X with no value")
	}
	if _, err := Parse([]string{"http://a", "http://b"}); err == nil {
		t.Error("expected error for multiple URLs")
	}
}

func TestParseHelp(t *testing.T) {
	_, err := Parse([]string{"-h"})
	if !errors.Is(err, ErrHelp) {
		t.Errorf("expected ErrHelp, got %v", err)
	}
	_, err = Parse([]string{"--help"})
	if !errors.Is(err, ErrHelp) {
		t.Errorf("expected ErrHelp, got %v", err)
	}
}
