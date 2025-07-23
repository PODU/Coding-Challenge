// Command dhcpclient leases an IPv4 address with the DHCP DORA handshake.
//
// Without --live it only prints the DISCOVER it would send. --live and
// --apply touch the network and need admin/root.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"time"

	"dhcpclient/internal/client"
	"dhcpclient/internal/dhcp"
	"dhcpclient/internal/netcfg"
)

const (
	exitOK        = 0
	exitUsage     = 1
	exitTimeout   = 2
	exitNak       = 3
	exitPrivilege = 4
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	live      bool
	apply     bool
	release   bool
	iface     string
	mac       string
	xid       uint
	timeout   time.Duration
	retries   int
	hostname  string
	requestIP string
	jsonOut   bool
	verbose   bool
	hexdump   bool
}

func run(args []string, stdout, stderr io.Writer) int {
	var o options
	fs := flag.NewFlagSet("dhcpclient", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&o.live, "live", false, "perform real UDP broadcast DORA (needs admin/root)")
	fs.BoolVar(&o.apply, "apply", false, "after ACK, apply the IP to the interface (implies --live)")
	fs.BoolVar(&o.release, "release", false, "send DHCPRELEASE for the bound lease and exit")
	fs.StringVar(&o.iface, "iface", "", "interface name to use (default: first non-loopback up)")
	fs.StringVar(&o.mac, "mac", "", "override client MAC (hex, colon-separated)")
	fs.UintVar(&o.xid, "xid", 0, "fixed transaction id (default: random)")
	fs.DurationVar(&o.timeout, "timeout", 4*time.Second, "per-message wait before retransmit")
	fs.IntVar(&o.retries, "retries", 4, "max retransmissions per phase")
	fs.StringVar(&o.hostname, "hostname", "", "send Option 12 (host name)")
	fs.StringVar(&o.requestIP, "request-ip", "", "preferred address (Option 50) in DISCOVER")
	fs.BoolVar(&o.jsonOut, "json", false, "emit the resulting lease as JSON")
	fs.BoolVar(&o.verbose, "verbose", false, "log each DORA step and decoded options")
	fs.BoolVar(&o.verbose, "v", false, "shorthand for --verbose")
	fs.BoolVar(&o.hexdump, "hexdump", false, "print hex dump of every sent/received packet")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	if o.apply {
		o.live = true
	}

	mac, err := resolveMAC(o)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}

	xid := uint32(o.xid)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	if xid == 0 {
		xid = rng.Uint32()
	}

	var reqIP net.IP
	if o.requestIP != "" {
		reqIP = net.ParseIP(o.requestIP).To4()
		if reqIP == nil {
			fmt.Fprintf(stderr, "error: invalid --request-ip %q\n", o.requestIP)
			return exitUsage
		}
	}

	logf := func(string, ...any) {}
	if o.verbose {
		logf = func(format string, a ...any) { fmt.Fprintf(stderr, format+"\n", a...) }
	}

	if !o.live {
		return dryRun(stdout, stderr, o, mac, xid, reqIP)
	}

	transport, err := client.NewUDPTransport()
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot run --live: %v\n", err)
		return exitPrivilege
	}
	defer transport.Close()

	c := &client.Client{
		Transport: hexTransport(transport, o.hexdump, stderr),
		MAC:       mac,
		XID:       xid,
		Hostname:  o.hostname,
		RequestIP: reqIP,
		Timeout:   o.timeout,
		Retries:   o.retries,
		Now:       time.Now,
		Rand:      rng,
		Logf:      logf,
	}

	lease, err := c.Run()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		if errors.Is(err, client.ErrRejected) {
			return exitNak
		}
		return exitTimeout
	}

	printLease(stdout, lease, o.jsonOut)

	if o.release {
		if err := c.Release(lease); err != nil {
			fmt.Fprintf(stderr, "error: release: %v\n", err)
			return exitTimeout
		}
		fmt.Fprintf(stdout, "released %s\n", lease.IP)
		return exitOK
	}

	if o.apply {
		if err := netcfg.Apply(o.iface, lease.IP, lease.SubnetMask, firstIP(lease.Routers)); err != nil {
			fmt.Fprintf(stderr, "error: apply: %v\n", err)
			return exitPrivilege
		}
		fmt.Fprintf(stdout, "applied %s to %s\n", lease.IP, o.iface)
	}

	return exitOK
}

func dryRun(stdout, stderr io.Writer, o options, mac net.HardwareAddr, xid uint32, reqIP net.IP) int {
	disc := dhcp.BuildDiscover(mac, xid, reqIP, o.hostname)
	raw, err := disc.Marshal()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}
	fmt.Fprintf(stdout, "[INIT]      built DISCOVER xid=0x%08x mac=%s\n", xid, mac)
	if o.verbose {
		fmt.Fprintf(stdout, "            options: message-type=DISCOVER, client-id, param-request-list%s\n",
			optSuffix(reqIP, o.hostname))
	}
	if o.hexdump {
		fmt.Fprintf(stdout, "%s", hex.Dump(raw))
	}
	fmt.Fprintln(stdout, "[note] not running with --live; no packets sent (use --live as admin)")
	return exitOK
}

func optSuffix(reqIP net.IP, hostname string) string {
	s := ""
	if reqIP != nil {
		s += ", requested-ip=" + reqIP.String()
	}
	if hostname != "" {
		s += ", hostname=" + hostname
	}
	return s
}

func printLease(stdout io.Writer, lease *client.Lease, asJSON bool) {
	if asJSON {
		out := map[string]any{
			"ip":            ipString(lease.IP),
			"server_id":     ipString(lease.ServerID),
			"subnet_mask":   ipString(lease.SubnetMask),
			"routers":       ipsString(lease.Routers),
			"dns":           ipsString(lease.DNS),
			"lease_seconds": lease.LeaseSeconds,
			"xid":           fmt.Sprintf("0x%08x", lease.XID),
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}
	fmt.Fprintf(stdout, "BOUND\n")
	fmt.Fprintf(stdout, "  ip:           %s\n", ipString(lease.IP))
	fmt.Fprintf(stdout, "  server id:    %s\n", ipString(lease.ServerID))
	fmt.Fprintf(stdout, "  subnet mask:  %s\n", ipString(lease.SubnetMask))
	fmt.Fprintf(stdout, "  routers:      %v\n", ipsString(lease.Routers))
	fmt.Fprintf(stdout, "  dns:          %v\n", ipsString(lease.DNS))
	fmt.Fprintf(stdout, "  lease (secs): %d\n", lease.LeaseSeconds)
}

// resolveMAC uses --mac, then the interface's MAC, then a fixed
// locally-administered address so the dry run works on any machine.
func resolveMAC(o options) (net.HardwareAddr, error) {
	if o.mac != "" {
		mac, err := net.ParseMAC(o.mac)
		if err != nil {
			return nil, fmt.Errorf("invalid --mac: %w", err)
		}
		return mac, nil
	}
	if iface := pickInterface(o.iface); iface != nil && len(iface.HardwareAddr) == 6 {
		return iface.HardwareAddr, nil
	}
	return net.HardwareAddr{0x02, 0x00, 0x5e, 0x00, 0x53, 0x01}, nil
}

func pickInterface(name string) *net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for i := range ifaces {
		ifc := &ifaces[i]
		if name != "" {
			if ifc.Name == name {
				return ifc
			}
			continue
		}
		if ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagUp == 0 {
			continue
		}
		if len(ifc.HardwareAddr) == 6 {
			return ifc
		}
	}
	return nil
}

func firstIP(ips []net.IP) net.IP {
	if len(ips) > 0 {
		return ips[0]
	}
	return nil
}

func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

func ipsString(ips []net.IP) []string {
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

// hexConn hex-dumps every packet that passes through the wrapped Transport.
type hexConn struct {
	inner client.Transport
	w     io.Writer
}

func hexTransport(inner client.Transport, enabled bool, w io.Writer) client.Transport {
	if !enabled {
		return inner
	}
	return &hexConn{inner: inner, w: w}
}

func (h *hexConn) Send(b []byte) error {
	fmt.Fprintf(h.w, "--> send %d bytes\n%s", len(b), hex.Dump(b))
	return h.inner.Send(b)
}

func (h *hexConn) Receive(deadline time.Time) ([]byte, error) {
	b, err := h.inner.Receive(deadline)
	if err == nil {
		fmt.Fprintf(h.w, "<-- recv %d bytes\n%s", len(b), hex.Dump(b))
	}
	return b, err
}

func (h *hexConn) Close() error { return h.inner.Close() }
