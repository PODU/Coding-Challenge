package trace

import (
	"fmt"
	"net"
	"strings"
)

// FormatHop renders one hop in the usual traceroute layout:
//
//	" 1  router.lan (192.168.1.1)  1.204 ms  0.998 ms  1.011 ms"
//
// The address is only printed again if a later query in the hop was answered
// by a different router. Timeouts print as "*".
func FormatHop(hop Hop, resolve func(net.IP) string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%2d ", hop.TTL)

	lastAddr := ""
	for _, q := range hop.Queries {
		if q.TimedOut || q.Addr == nil {
			b.WriteString(" *")
			continue
		}
		addr := q.Addr.String()
		if addr != lastAddr {
			host := addr
			if resolve != nil {
				host = resolve(q.Addr)
			}
			fmt.Fprintf(&b, " %s (%s)", host, addr)
			lastAddr = addr
		}
		fmt.Fprintf(&b, "  %.3f ms", float64(q.RTT.Nanoseconds())/1e6)
	}
	return b.String()
}
