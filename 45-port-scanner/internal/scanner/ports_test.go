package scanner

import (
	"reflect"
	"testing"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want []int
	}{
		{"single", "80", []int{80}},
		{"list", "22,80,443", []int{22, 80, 443}},
		{"range", "1-5", []int{1, 2, 3, 4, 5}},
		{"combo", "22,80,8000-8002", []int{22, 80, 8000, 8001, 8002}},
		{"dedup", "22,22,22", []int{22}},
		{"overlap dedup", "80,79-81", []int{79, 80, 81}},
		{"unsorted input sorted output", "443,22,80", []int{22, 80, 443}},
		{"whitespace tolerated", " 22 , 80 ", []int{22, 80}},
		{"single boundary low", "1", []int{1}},
		{"single boundary high", "65535", []int{65535}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePorts(tt.spec)
			if err != nil {
				t.Fatalf("ParsePorts(%q) unexpected error: %v", tt.spec, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePorts(%q) = %v, want %v", tt.spec, got, tt.want)
			}
		})
	}
}

func TestParsePortsFullRange(t *testing.T) {
	got, err := ParsePorts("")
	if err != nil {
		t.Fatalf("ParsePorts(\"\") unexpected error: %v", err)
	}
	if len(got) != 65535 {
		t.Fatalf("full range length = %d, want 65535", len(got))
	}
	if got[0] != 1 {
		t.Errorf("first port = %d, want 1", got[0])
	}
	if got[len(got)-1] != 65535 {
		t.Errorf("last port = %d, want 65535", got[len(got)-1])
	}
}

func TestParsePortsErrors(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{"zero", "0"},
		{"too high", "70000"},
		{"negative", "-1"},
		{"inverted range", "1000-10"},
		{"empty middle token", "22,,80"},
		{"non-numeric", "abc"},
		{"non-numeric in list", "22,abc,80"},
		{"range too high", "1-70000"},
		{"range zero low", "0-10"},
		{"triple range", "1-2-3"},
		{"empty range bound", "1-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParsePorts(tt.spec); err == nil {
				t.Errorf("ParsePorts(%q) expected error, got nil", tt.spec)
			}
		})
	}
}
