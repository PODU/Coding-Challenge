// Package client sends a request, reads the response and, if asked, follows
// redirects. With -v it traces both directions like curl does.
package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/codingchallenges/cc-curl/internal/httpmsg"
	"github.com/codingchallenges/cc-curl/internal/trace"
	"github.com/codingchallenges/cc-curl/internal/urlparse"
)

// Dialer opens a connection for a URL. Tests point it at an httptest server.
type Dialer func(u urlparse.URL) (net.Conn, error)

// Config controls a Client.
type Config struct {
	Method      string // "" means GET, or POST when there is a body
	Headers     []httpmsg.Header
	Body        []byte // nil means no body
	FollowRedir bool
	MaxRedirs   int
	ReadTimeout time.Duration // 0 means no deadline
	Tracer      *trace.Tracer
	Dial        Dialer
}

// Result is the last response and the URL it came from after redirects.
type Result struct {
	Response *httpmsg.Response
	FinalURL urlparse.URL
}

// Client sends requests according to its Config.
type Client struct {
	cfg Config
}

// New returns a Client, defaulting MaxRedirs to 20.
func New(cfg Config) *Client {
	if cfg.MaxRedirs == 0 {
		cfg.MaxRedirs = 20
	}
	return &Client{cfg: cfg}
}

// Do sends the request to startURL, following redirects if configured.
func (c *Client) Do(startURL urlparse.URL) (*Result, error) {
	method := c.effectiveMethod()
	body := c.cfg.Body
	headers := c.cfg.Headers

	current := startURL
	redirs := 0
	for {
		resp, err := c.roundTrip(current, method, headers, body)
		if err != nil {
			return nil, err
		}

		if c.cfg.FollowRedir && isRedirect(resp.StatusCode) {
			loc, ok := resp.Headers.Get("Location")
			if ok && loc != "" {
				if redirs >= c.cfg.MaxRedirs {
					return nil, fmt.Errorf("maximum (%d) redirects followed", c.cfg.MaxRedirs)
				}
				redirs++
				next, err := urlparse.Resolve(current, loc)
				if err != nil {
					return nil, fmt.Errorf("resolving redirect %q: %w", loc, err)
				}
				c.cfg.Tracer.Info(fmt.Sprintf("Issue another request to this URL: '%s'", next.String()))

				// Like curl, 301/302/303 switch to GET and drop the body, while
				// 307/308 resend the same method and body.
				if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 {
					if method != "HEAD" {
						method = "GET"
					}
					body = nil
					headers = stripBodyHeaders(headers)
				}
				current = next
				continue
			}
		}

		return &Result{Response: resp, FinalURL: current}, nil
	}
}

func (c *Client) effectiveMethod() string {
	if c.cfg.Method != "" {
		return c.cfg.Method
	}
	if c.cfg.Body != nil {
		return "POST"
	}
	return "GET"
}

func (c *Client) roundTrip(u urlparse.URL, method string, headers []httpmsg.Header, body []byte) (*httpmsg.Response, error) {
	reqBytes := httpmsg.BuildRequest(httpmsg.Request{
		Method:  method,
		URL:     u,
		Headers: headers,
		Body:    body,
	})

	if c.cfg.Tracer.Enabled() {
		c.cfg.Tracer.SentBlock(string(requestHead(reqBytes)))
		c.cfg.Tracer.Sent("")
		if len(body) > 0 {
			c.cfg.Tracer.Info(fmt.Sprintf("[%d bytes request body]", len(body)))
		}
	}

	conn, err := c.cfg.Dial(u)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if c.cfg.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
	}

	if _, err := conn.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	br := bufio.NewReader(conn)
	resp, err := httpmsg.ParseResponse(br, method == "HEAD")
	if err != nil {
		return nil, err
	}

	if c.cfg.Tracer.Enabled() {
		c.cfg.Tracer.Recv(resp.RawStatusLine)
		for _, h := range resp.RawHeaderLines {
			c.cfg.Tracer.Recv(h)
		}
		c.cfg.Tracer.Recv("")
	}

	return resp, nil
}

func isRedirect(code int) bool {
	switch code {
	case 301, 302, 303, 307, 308:
		return true
	}
	return false
}

// stripBodyHeaders is for redirects that drop the body.
func stripBodyHeaders(headers []httpmsg.Header) []httpmsg.Header {
	out := headers[:0:0]
	for _, h := range headers {
		n := strings.ToLower(h.Name)
		if n == "content-type" || n == "content-length" {
			continue
		}
		out = append(out, h)
	}
	return out
}

// requestHead returns the request up to the blank line ending the headers.
func requestHead(raw []byte) []byte {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return raw[:i]
	}
	return raw
}
