// Command cc-curl is a small HTTP/1.1 client that writes requests straight to
// a TCP socket (or TLS for https).
package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/codingchallenges/cc-curl/internal/cli"
	"github.com/codingchallenges/cc-curl/internal/client"
	"github.com/codingchallenges/cc-curl/internal/httpmsg"
	"github.com/codingchallenges/cc-curl/internal/trace"
	"github.com/codingchallenges/cc-curl/internal/transport"
	"github.com/codingchallenges/cc-curl/internal/urlparse"
)

const (
	exitOK        = 0
	exitUsage     = 1
	exitTransport = 2
	exitResponse  = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr *os.File) int {
	opts, err := cli.Parse(argv)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Fprint(stdout, cli.Usage)
			return exitOK
		}
		fmt.Fprintf(stderr, "cc-curl: %v\n\n%s", err, cli.Usage)
		return exitUsage
	}

	u, err := urlparse.ParseURL(opts.URL)
	if err != nil {
		fmt.Fprintf(stderr, "cc-curl: bad URL %q: %v\n", opts.URL, err)
		return exitUsage
	}

	var tracer *trace.Tracer
	if opts.Verbose {
		tracer = trace.New(stderr)
	}

	headers := make([]httpmsg.Header, 0, len(opts.Headers))
	for _, raw := range opts.Headers {
		headers = append(headers, httpmsg.ParseHeaderArg(raw))
	}

	var body []byte
	if opts.HasData {
		body = []byte(opts.Data)
	}

	method := opts.Method
	if method == "" && body != nil {
		method = "POST"
	}

	dial := func(target urlparse.URL) (net.Conn, error) {
		return transport.Dial(target, transport.Config{
			Insecure: opts.Insecure,
			Tracer:   tracer,
		})
	}

	c := client.New(client.Config{
		Method:      method,
		Headers:     headers,
		Body:        body,
		FollowRedir: opts.Location,
		MaxRedirs:   opts.MaxRedirs,
		ReadTimeout: 60 * time.Second,
		Tracer:      tracer,
		Dial:        dial,
	})

	result, err := c.Do(u)
	if err != nil {
		fmt.Fprintf(stderr, "cc-curl: %v\n", err)
		if errors.Is(err, httpmsg.ErrBadStatusLine) || errors.Is(err, httpmsg.ErrBadChunk) {
			return exitResponse
		}
		return exitTransport
	}

	return render(result.Response, opts, stdout, stderr)
}

func render(resp *httpmsg.Response, opts cli.Options, stdout, stderr *os.File) int {
	var out []byte
	if opts.Include {
		out = append(out, resp.RawStatusLine...)
		out = append(out, '\r', '\n')
		for _, h := range resp.RawHeaderLines {
			out = append(out, h...)
			out = append(out, '\r', '\n')
		}
		out = append(out, '\r', '\n')
	}
	out = append(out, resp.Body...)

	if opts.Output != "" {
		if err := os.WriteFile(opts.Output, out, 0o644); err != nil {
			fmt.Fprintf(stderr, "cc-curl: writing %s: %v\n", opts.Output, err)
			return exitTransport
		}
		return exitOK
	}

	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "cc-curl: writing output: %v\n", err)
		return exitTransport
	}
	return exitOK
}
