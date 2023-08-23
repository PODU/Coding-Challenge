// Command traceroute maps the network path to a destination host by sending
// probes with an incrementing IP TTL and recording the ICMP responses.
//
// Raw ICMP sockets need Administrator on Windows or root on Unix. On Windows,
// ICMP mode (-I) works better.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"traceroute/internal/trace"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "traceroute:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr *os.File) error {
	fs := flag.NewFlagSet("traceroute", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		maxHops = fs.Int("m", trace.DefaultMaxHops, "maximum number of hops (TTL)")
		queries = fs.Int("q", trace.DefaultQueries, "number of probe queries per hop")
		timeout = fs.Duration("w", trace.DefaultTimeout, "per-query wait for a reply (e.g. 2s)")
		port    = fs.Int("p", trace.DefaultBasePort, "base destination UDP port")
		size    = fs.Int("s", trace.DefaultPacketSize, "probe packet size in bytes")
		numeric = fs.Bool("n", false, "numeric output; skip reverse DNS")
		useICMP = fs.Bool("I", false, "use ICMP Echo probes instead of UDP (recommended on Windows)")
	)
	fs.IntVar(maxHops, "max-hops", *maxHops, "alias for -m")
	fs.IntVar(queries, "queries", *queries, "alias for -q")
	fs.DurationVar(timeout, "timeout", *timeout, "alias for -w")
	fs.IntVar(port, "port", *port, "alias for -p")
	fs.IntVar(size, "size", *size, "alias for -s")
	fs.BoolVar(useICMP, "icmp", *useICMP, "alias for -I")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: traceroute [options] <host>\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("exactly one destination host is required")
	}
	if *maxHops < 1 {
		return errors.New("max hops must be >= 1")
	}
	if *queries < 1 {
		return errors.New("queries must be >= 1")
	}
	if *size < 0 {
		return errors.New("packet size must be >= 0")
	}

	host := fs.Arg(0)
	ipAddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return fmt.Errorf("unknown host %q: %w", host, err)
	}

	mode := trace.ModeUDP
	if *useICMP {
		mode = trace.ModeICMP
	}

	cfg := trace.Config{
		Dest:       ipAddr.IP,
		DestName:   host,
		MaxHops:    *maxHops,
		Queries:    *queries,
		Timeout:    normalizeTimeout(*timeout),
		BasePort:   *port,
		PacketSize: *size,
		Numeric:    *numeric,
		Mode:       mode,
	}

	prober, err := trace.NewProber(cfg)
	if err != nil {
		return err
	}
	defer prober.Close()

	resolver := trace.NewHostResolver(trace.DefaultResolver(), cfg.Numeric)
	tracer := trace.NewTracer(cfg, prober, resolver.Host)

	if err := tracer.Run(stdout); err != nil {
		if errors.Is(err, trace.ErrNotReached) {
			fmt.Fprintln(stderr, "traceroute:", err)
			os.Exit(1)
		}
		return err
	}
	return nil
}

func normalizeTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return trace.DefaultTimeout
	}
	return d
}
