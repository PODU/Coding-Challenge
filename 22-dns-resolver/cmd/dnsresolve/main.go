// Command dnsresolve looks up a hostname by walking down from the root
// nameservers, or with -server, by asking one recursive resolver.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"dnsresolve/internal/message"
	"dnsresolve/internal/resolver"
	"dnsresolve/internal/transport"
)

const (
	exitOK       = 0
	exitUsage    = 1
	exitNoAnswer = 2 // NXDOMAIN or no records
	exitNetwork  = 3 // timeouts, server failures and anything else
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	rtype   string
	server  string
	trace   bool
	timeout time.Duration
	retries int
	tcp     bool
	json    bool
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("dnsresolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opt options
	fs.StringVar(&opt.rtype, "type", "A", "record type: A, NS, CNAME, AAAA")
	fs.StringVar(&opt.server, "server", "", "query this resolver directly (RD=1) instead of recursing from root")
	fs.BoolVar(&opt.trace, "trace", false, "print every server consulted and the records returned")
	fs.DurationVar(&opt.timeout, "timeout", 3*time.Second, "per-query UDP timeout")
	fs.IntVar(&opt.retries, "retries", 2, "UDP attempts per server")
	fs.BoolVar(&opt.tcp, "tcp", false, "force TCP; otherwise TCP is used only on a truncated reply")
	fs.BoolVar(&opt.json, "json", false, "machine-readable output")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: dnsresolve [flags] <hostname>")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	host := fs.Arg(0)

	qtype, err := message.TypeFromName(opt.rtype)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}

	xport := transport.UDP{
		Timeout:  opt.timeout,
		Retries:  opt.retries,
		ForceTCP: opt.tcp,
	}

	if opt.server != "" {
		return runDirect(opt, xport, host, qtype, stdout, stderr)
	}
	return runRecursive(opt, xport, host, qtype, stdout, stderr)
}

func runRecursive(opt options, xport transport.Exchanger, host string, qtype uint16, stdout, stderr *os.File) int {
	res, err := resolver.New(xport).Resolve(host, qtype)
	if opt.trace && res != nil {
		printTrace(stderr, res.Trace)
	}
	if err != nil {
		return reportErr(stderr, err)
	}
	return printResult(opt, stdout, res)
}

// runDirect sends one query with recursion desired to opt.server.
func runDirect(opt options, xport transport.UDP, host string, qtype uint16, stdout, stderr *os.File) int {
	addrPort, err := parseServer(opt.server)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}

	q := message.NewQuery(uint16(time.Now().UnixNano()), host, qtype, message.ClassIN, true)
	query, err := q.Pack()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}
	raw, err := xport.Exchange(addrPort, query)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitNetwork
	}
	msg, err := message.Unpack(raw)
	if err != nil {
		fmt.Fprintf(stderr, "error: decode response: %v\n", err)
		return exitNetwork
	}
	if msg.Header.ID != q.Header.ID {
		fmt.Fprintf(stderr, "error: response ID mismatch (want %d got %d)\n", q.Header.ID, msg.Header.ID)
		return exitNetwork
	}
	if !msg.Header.QR() {
		fmt.Fprintln(stderr, "error: response is not a reply (QR=0)")
		return exitNetwork
	}

	res := buildResult(host, qtype, msg, raw)
	if opt.trace {
		fmt.Fprintf(stderr, ";; server %s  %s  %d answer(s)\n",
			addrPort, message.RCodeName(msg.Header.RCode()), len(res.Answers))
	}
	if msg.Header.RCode() == message.RCodeNXDomain {
		fmt.Fprintln(stderr, "NXDOMAIN")
		return exitNoAnswer
	}
	return printResult(opt, stdout, res)
}

// buildResult wraps a direct response in a resolver.Result so it can be
// printed the same way as a recursive one.
func buildResult(host string, qtype uint16, msg *message.Message, raw []byte) *resolver.Result {
	res := &resolver.Result{Name: host, QType: qtype, RCode: msg.Header.RCode()}
	for _, rec := range msg.Answers {
		if rec.Type != qtype {
			continue
		}
		res.Answers = append(res.Answers, rec)
		if a, ok := rec.AString(); ok {
			res.Addrs = append(res.Addrs, a)
		} else if a, ok := rec.AAAAString(); ok {
			res.Addrs = append(res.Addrs, a)
		}
		if ns, ok := rec.NSString(raw); ok {
			res.Names = append(res.Names, ns)
		} else if cn, ok := rec.CNAMEString(raw); ok {
			res.Names = append(res.Names, cn)
		}
	}
	return res
}

func printResult(opt options, stdout *os.File, res *resolver.Result) int {
	if opt.json {
		return printJSON(stdout, res)
	}
	lines := answerLines(res)
	if len(lines) == 0 {
		fmt.Fprintln(stdout, "(no records)")
		return exitNoAnswer
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	return exitOK
}

func answerLines(res *resolver.Result) []string {
	var out []string
	switch res.QType {
	case message.TypeA, message.TypeAAAA:
		for _, a := range res.Addrs {
			out = append(out, a.String())
		}
	case message.TypeNS, message.TypeCNAME:
		out = append(out, res.Names...)
	}
	return out
}

type jsonResult struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	RCode   string   `json:"rcode"`
	CNAME   string   `json:"cname,omitempty"`
	Addrs   []string `json:"addresses,omitempty"`
	Answers []string `json:"answers,omitempty"`
}

func printJSON(stdout *os.File, res *resolver.Result) int {
	jr := jsonResult{
		Name:  res.Name,
		Type:  message.TypeName(res.QType),
		RCode: message.RCodeName(res.RCode),
		CNAME: res.CNAME,
	}
	for _, a := range res.Addrs {
		jr.Addrs = append(jr.Addrs, a.String())
	}
	jr.Answers = answerLines(res)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(jr)
	if len(jr.Addrs) == 0 && len(jr.Answers) == 0 {
		return exitNoAnswer
	}
	return exitOK
}

func printTrace(stderr *os.File, trace []resolver.Trace) {
	for _, t := range trace {
		switch t.Kind {
		case "error":
			fmt.Fprintf(stderr, ";; querying %s (%s) for %s %s -> error: %s\n",
				t.ServerName, t.ServerAddr, t.QName, message.TypeName(t.QType), strings.Join(t.Detail, "; "))
		case "referral":
			fmt.Fprintf(stderr, ";; querying %s (%s) for %s %s -> referral\n",
				t.ServerName, t.ServerAddr, t.QName, message.TypeName(t.QType))
			for _, d := range t.Detail {
				fmt.Fprintf(stderr, ";;   -> %s\n", d)
			}
		case "cname":
			fmt.Fprintf(stderr, ";; querying %s (%s) for %s %s -> CNAME %s\n",
				t.ServerName, t.ServerAddr, t.QName, message.TypeName(t.QType), strings.Join(t.Detail, " "))
		default:
			fmt.Fprintf(stderr, ";; querying %s (%s) for %s %s -> %s\n",
				t.ServerName, t.ServerAddr, t.QName, message.TypeName(t.QType), message.RCodeName(t.RCode))
			for _, d := range t.Detail {
				fmt.Fprintf(stderr, ";;   %s\n", d)
			}
		}
	}
}

func reportErr(stderr *os.File, err error) int {
	fmt.Fprintf(stderr, "error: %v\n", err)
	if errors.Is(err, resolver.ErrNXDomain) || errors.Is(err, resolver.ErrNoAnswer) {
		return exitNoAnswer
	}
	return exitNetwork
}

// parseServer accepts an IP address, with or without a port. The port
// defaults to 53.
func parseServer(s string) (netip.AddrPort, error) {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap, nil
	}
	if addr, err := netip.ParseAddr(s); err == nil {
		return netip.AddrPortFrom(addr, 53), nil
	}
	return netip.AddrPort{}, fmt.Errorf("invalid -server %q (want IP or IP:port)", s)
}
