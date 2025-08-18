// Command whois is a WHOIS client (RFC 3912) for domains, IP addresses and AS
// numbers. It starts at whois.iana.org and follows referrals until it reaches
// the server that holds the record.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/codingchallenges/whois/internal/whois"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr *os.File) int {
	var (
		host       string
		port       int
		raw        bool
		jsonOut    bool
		noReferral bool
		timeout    time.Duration
		verbose    bool
	)

	fs := flag.NewFlagSet("whois", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&host, "h", "", "query this WHOIS server directly; skip IANA discovery")
	fs.StringVar(&host, "host", "", "query this WHOIS server directly; skip IANA discovery")
	fs.IntVar(&port, "p", whois.DefaultPort, "TCP port")
	fs.IntVar(&port, "port", whois.DefaultPort, "TCP port")
	fs.BoolVar(&raw, "r", false, "print raw concatenated server responses")
	fs.BoolVar(&raw, "raw", false, "print raw concatenated server responses")
	fs.BoolVar(&jsonOut, "j", false, "emit the parsed merged record as JSON")
	fs.BoolVar(&jsonOut, "json", false, "emit the parsed merged record as JSON")
	fs.BoolVar(&noReferral, "no-referral", false, "do not follow referrals (single hop)")
	fs.DurationVar(&timeout, "timeout", whois.DefaultTimeout, "per-hop dial+read timeout")
	fs.BoolVar(&verbose, "v", false, "log each hop to stderr")
	fs.BoolVar(&verbose, "verbose", false, "log each hop to stderr")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: whois [flags] <query>")
		fmt.Fprintln(stderr, "\n  <query>  Domain (example.com), IPv4/IPv6 (8.8.8.8), or ASN (AS15169).")
		fmt.Fprintln(stderr, "\nFlags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(argv); err != nil {
		return 1
	}
	args := fs.Args()
	if len(args) != 1 {
		fs.Usage()
		return 1
	}
	query := args[0]

	client := whois.NewClient()
	client.Port = port
	client.Timeout = timeout

	lookup := whois.NewLookup(client)
	lookup.Follow = !noReferral
	lookup.Verbose = verbose
	lookup.Log = func(format string, a ...any) {
		fmt.Fprintf(stderr, format+"\n", a...)
	}
	if host != "" {
		// Ask only this server: no IANA, no referrals.
		lookup.Seed = host
		lookup.Follow = false
	}

	res, err := lookup.Do(context.Background(), query)
	if err != nil {
		fmt.Fprintf(stderr, "whois: %v\n", err)
		return 2
	}
	if res.NoMatch {
		fmt.Fprintf(stderr, "whois: no match for %q\n", query)
		return 3
	}

	switch {
	case raw:
		whois.PrintRaw(stdout, res)
	case jsonOut:
		if err := whois.PrintJSON(stdout, res); err != nil {
			fmt.Fprintf(stderr, "whois: %v\n", err)
			return 2
		}
	default:
		whois.PrintRecord(stdout, res)
	}
	return 0
}
