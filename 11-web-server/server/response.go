package server

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"time"
)

const serverName = "cc-web-server/1.0"

var reasonPhrases = map[int]string{
	200: "OK",
	400: "Bad Request",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	500: "Internal Server Error",
}

func reason(code int) string {
	if r, ok := reasonPhrases[code]; ok {
		return r
	}
	return "Status " + strconv.Itoa(code)
}

// Response is a fully-formed HTTP response ready to be written to a connection.
type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
	// HeadOnly suppresses writing the body (for HEAD requests) while still
	// reporting the real Content-Length.
	HeadOnly bool
}

func newResponse(status int, contentType string, body []byte) *Response {
	return &Response{
		Status: status,
		Headers: map[string]string{
			"Content-Type":   contentType,
			"Content-Length": strconv.Itoa(len(body)),
		},
		Body: body,
	}
}

// Write serialises the response to w: status line, sorted headers, the blank
// line separator, then the body (unless HeadOnly).
func (resp *Response) Write(w *bufio.Writer) error {
	if _, err := fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", resp.Status, reason(resp.Status)); err != nil {
		return err
	}

	resp.Headers["Server"] = serverName
	resp.Headers["Date"] = time.Now().UTC().Format(http1Date)
	resp.Headers["Connection"] = "close"

	keys := make([]string, 0, len(resp.Headers))
	for k := range resp.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, resp.Headers[k]); err != nil {
			return err
		}
	}

	if _, err := w.WriteString("\r\n"); err != nil {
		return err
	}

	if !resp.HeadOnly && len(resp.Body) > 0 {
		if _, err := w.Write(resp.Body); err != nil {
			return err
		}
	}
	return w.Flush()
}

// http1Date is the RFC 7231 IMF-fixdate layout used for the Date header.
const http1Date = "Mon, 02 Jan 2006 15:04:05 GMT"
