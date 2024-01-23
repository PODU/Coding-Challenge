// Command ccscan is a small, nmap-style TCP connect port scanner.
//
// Only scan machines you own or have permission to test; scanning anything
// else may be illegal and usually breaks acceptable-use policies. That's why
// the default target is localhost.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"ccscan/internal/scanner"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("ccscan", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		host       = fs.String("host", "localhost", "Target host(s): single, comma-list, wildcard (192.168.1.*), or CIDR (192.168.1.0/24)")
		port       = fs.String("port", "", "Port spec: single (80), list (22,80,443), range (1-1024), combo, or empty for all 1-65535")
		conc       = fs.Int("c", 512, "Concurrency: number of worker goroutines")
		timeout    = fs.Duration("t", 500*time.Millisecond, "Per-connection timeout (e.g. 500ms, 1s)")
		format     = fs.String("format", "text", `Output format: "text" or "json"`)
		verbose    = fs.Bool("v", false, "Verbose: also list closed/filtered ports")
		allowLarge = fs.Bool("allow-large", false, "Allow CIDR blocks larger than /24 (guards against accidental mass scans)")
	)

	fs.Usage = func() {
		fmt.Fprintf(stderr, "ccscan — authorized TCP connect port scanner\n\n")
		fmt.Fprintf(stderr, "Usage: ccscan [flags]\n\n")
		fs.PrintDefaults()
		fmt.Fprintf(stderr, "\nLEGAL: Only scan hosts you own or are explicitly authorized to test.\n")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "error: -format must be \"text\" or \"json\", got %q\n", *format)
		return 2
	}
	if *conc < 1 {
		fmt.Fprintf(stderr, "error: -c (concurrency) must be >= 1\n")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintf(stderr, "error: -t (timeout) must be > 0\n")
		return 2
	}

	hosts, err := scanner.ParseHosts(*host, *allowLarge)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	ports, err := scanner.ParsePorts(*port)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *format == "text" {
		fmt.Fprintf(stdout, "Scanning %d host(s) — %d port(s) each, concurrency %d, timeout %s\n\n",
			len(hosts), len(ports), *conc, *timeout)
	}

	start := time.Now()
	results := scanner.Scan(ctx, hosts, ports, *conc, *timeout)
	elapsed := time.Since(start)

	rep := scanner.BuildReport(results, *conc, *timeout, elapsed, len(ports), *verbose)

	switch *format {
	case "json":
		if err := scanner.RenderJSON(stdout, rep); err != nil {
			fmt.Fprintf(stderr, "error writing json: %v\n", err)
			return 1
		}
	default:
		if err := scanner.RenderText(stdout, rep, *verbose); err != nil {
			fmt.Fprintf(stderr, "error writing report: %v\n", err)
			return 1
		}
	}

	// 130 = 128 + SIGINT, the usual shell convention.
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "\nscan interrupted — partial results shown\n")
		return 130
	}
	return 0
}
