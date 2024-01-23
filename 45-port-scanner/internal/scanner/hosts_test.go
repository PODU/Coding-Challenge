package scanner

import (
	"reflect"
	"testing"
)

func TestParseHostsBasic(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want []string
	}{
		{"single", "localhost", []string{"localhost"}},
		{"list", "localhost,127.0.0.1", []string{"localhost", "127.0.0.1"}},
		{"dedup exact", "127.0.0.1,127.0.0.1", []string{"127.0.0.1"}},
		{"order preserved", "10.0.0.2,10.0.0.1", []string{"10.0.0.2", "10.0.0.1"}},
		{"whitespace", " localhost , 127.0.0.1 ", []string{"localhost", "127.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHosts(tt.spec, false)
			if err != nil {
				t.Fatalf("ParseHosts(%q) unexpected error: %v", tt.spec, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseHosts(%q) = %v, want %v", tt.spec, got, tt.want)
			}
		})
	}
}

func TestParseHostsWildcard(t *testing.T) {
	got, err := ParseHosts("192.168.1.*", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 254 {
		t.Fatalf("wildcard count = %d, want 254", len(got))
	}
	if got[0] != "192.168.1.1" {
		t.Errorf("first = %q, want 192.168.1.1", got[0])
	}
	if got[len(got)-1] != "192.168.1.254" {
		t.Errorf("last = %q, want 192.168.1.254", got[len(got)-1])
	}
}

func TestParseHostsCIDR(t *testing.T) {
	// /30 => 4 addresses, minus network + broadcast => 2 usable hosts.
	got, err := ParseHosts("192.168.1.0/30", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"192.168.1.1", "192.168.1.2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("/30 = %v, want %v", got, want)
	}
}

func TestParseHostsCIDR29(t *testing.T) {
	// /29 => 8 addresses, minus network + broadcast => 6 usable hosts.
	got, err := ParseHosts("10.0.0.0/29", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"10.0.0.1", "10.0.0.2", "10.0.0.3",
		"10.0.0.4", "10.0.0.5", "10.0.0.6",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("/29 = %v, want %v", got, want)
	}
}

func TestParseHostsLargeCIDRGuard(t *testing.T) {
	if _, err := ParseHosts("10.0.0.0/16", false); err == nil {
		t.Errorf("expected guard error for /16 without allowLarge")
	}
	if _, err := ParseHosts("10.0.0.0/23", true); err != nil {
		t.Errorf("allowLarge should permit /23: %v", err)
	}
}

func TestParseHostsErrors(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{"empty", ""},
		{"empty token", "localhost,,127.0.0.1"},
		{"bad cidr", "192.168.1.0/33"},
		{"ipv6 cidr unsupported", "::1/64"},
		{"bad wildcard middle octet", "192.*.1.0"},
		{"bad wildcard non-numeric", "abc.def.ghi.*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseHosts(tt.spec, false); err == nil {
				t.Errorf("ParseHosts(%q) expected error, got nil", tt.spec)
			}
		})
	}
}
