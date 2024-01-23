package scanner

import "testing"

func TestServiceName(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{22, "ssh"},
		{80, "http"},
		{443, "https"},
		{6379, "redis"},
		{3306, "mysql"},
		{1, ""},     // unknown
		{65000, ""}, // unknown
	}
	for _, tt := range tests {
		if got := ServiceName(tt.port); got != tt.want {
			t.Errorf("ServiceName(%d) = %q, want %q", tt.port, got, tt.want)
		}
	}
}
