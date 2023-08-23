package trace

import (
	"net"
	"testing"
	"time"
)

// numericResolve mimics -n output (host == bare IP).
func numericResolve(ip net.IP) string { return ip.String() }

func us(n int) time.Duration { return time.Duration(n) * time.Microsecond }

func TestFormatHop(t *testing.T) {
	ip1 := net.ParseIP("192.168.1.1")
	a1 := net.ParseIP("10.0.0.1")
	a2 := net.ParseIP("10.0.0.2")

	named := func(ip net.IP) string {
		if ip.Equal(ip1) {
			return "router.lan"
		}
		return ip.String()
	}

	tests := []struct {
		name    string
		hop     Hop
		resolve func(net.IP) string
		want    string
	}{
		{
			name: "normal three rtts with hostname",
			hop: Hop{TTL: 1, Queries: []Query{
				{Addr: ip1, RTT: us(1204)},
				{Addr: ip1, RTT: us(998)},
				{Addr: ip1, RTT: us(1011)},
			}},
			resolve: named,
			want:    " 1  router.lan (192.168.1.1)  1.204 ms  0.998 ms  1.011 ms",
		},
		{
			name: "full timeout",
			hop: Hop{TTL: 2, Queries: []Query{
				{TimedOut: true}, {TimedOut: true}, {TimedOut: true},
			}},
			resolve: numericResolve,
			want:    " 2  * * *",
		},
		{
			name: "partial timeout numeric",
			hop: Hop{TTL: 3, Queries: []Query{
				{TimedOut: true},
				{Addr: a1, RTT: us(9331)},
				{TimedOut: true},
			}},
			resolve: numericResolve,
			want:    " 3  * 10.0.0.1 (10.0.0.1)  9.331 ms *",
		},
		{
			name: "mixed source addresses",
			hop: Hop{TTL: 4, Queries: []Query{
				{Addr: a1, RTT: us(1000)},
				{Addr: a2, RTT: us(2000)},
				{Addr: a1, RTT: us(3000)},
			}},
			resolve: numericResolve,
			want:    " 4  10.0.0.1 (10.0.0.1)  1.000 ms 10.0.0.2 (10.0.0.2)  2.000 ms 10.0.0.1 (10.0.0.1)  3.000 ms",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatHop(tc.hop, tc.resolve); got != tc.want {
				t.Errorf("FormatHop mismatch:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}
