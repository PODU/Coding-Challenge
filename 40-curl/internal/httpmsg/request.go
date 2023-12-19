// Package httpmsg writes HTTP/1.1 requests and parses responses by hand,
// without net/http.
package httpmsg

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/codingchallenges/cc-curl/internal/urlparse"
)

// Request is a request to be written with BuildRequest.
type Request struct {
	Method  string
	URL     urlparse.URL
	Headers []Header // from -H, in order
	Body    []byte   // nil if no body
}

// Header is one header field.
type Header struct {
	Name  string
	Value string
}

const crlf = "\r\n"

// BuildRequest returns the bytes to send. It adds Host, Accept and Connection
// headers, plus Content-Length and Content-Type when there is a body. A user
// header with the same name (case-insensitive) replaces the default, even if
// its value is empty.
func BuildRequest(req Request) []byte {
	method := req.Method
	if method == "" {
		method = "GET"
	}

	userSet := make(map[string]bool, len(req.Headers))
	for _, h := range req.Headers {
		userSet[strings.ToLower(strings.TrimSpace(h.Name))] = true
	}

	var b bytes.Buffer
	b.WriteString(method)
	b.WriteByte(' ')
	b.WriteString(req.URL.RequestTarget())
	b.WriteByte(' ')
	b.WriteString("HTTP/1.1")
	b.WriteString(crlf)

	if !userSet["host"] {
		writeHeader(&b, "Host", req.URL.HostHeader())
	}
	if !userSet["accept"] {
		writeHeader(&b, "Accept", "*/*")
	}
	if !userSet["connection"] {
		writeHeader(&b, "Connection", "close")
	}

	if req.Body != nil {
		if !userSet["content-length"] {
			writeHeader(&b, "Content-Length", strconv.Itoa(len(req.Body)))
		}
		if !userSet["content-type"] {
			writeHeader(&b, "Content-Type", "application/x-www-form-urlencoded")
		}
	}

	for _, h := range req.Headers {
		writeHeader(&b, h.Name, h.Value)
	}

	b.WriteString(crlf)
	if req.Body != nil {
		b.Write(req.Body)
	}
	return b.Bytes()
}

func writeHeader(b *bytes.Buffer, name, value string) {
	b.WriteString(name)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString(crlf)
}

// ParseHeaderArg splits a -H argument ("Name: value"). Without a colon the whole
// argument is the name and the value is empty.
func ParseHeaderArg(raw string) Header {
	if i := strings.IndexByte(raw, ':'); i >= 0 {
		return Header{
			Name:  strings.TrimSpace(raw[:i]),
			Value: strings.TrimSpace(raw[i+1:]),
		}
	}
	return Header{Name: strings.TrimSpace(raw)}
}
