package smtp

import (
	"fmt"
	"strings"
)

// SMTP reply codes (RFC 5321 section 4.2.3).
const (
	CodeServiceReady    = 220
	CodeServiceClosing  = 221
	CodeOK              = 250
	CodeCannotVRFY      = 252
	CodeStartMailInput  = 354
	CodeServiceNotAvail = 421
	CodeSyntaxError     = 500
	CodeParamError      = 501
	CodeNotImplemented  = 502
	CodeBadSequence     = 503
	CodeMessageTooBig   = 552
	CodeTransactionFail = 554
)

func reply(code int, text string) string {
	return fmt.Sprintf("%d %s\r\n", code, text)
}

// multiReply formats a multi-line reply: every line but the last uses
// "<code>-" and the last uses "<code> ".
func multiReply(code int, lines []string) string {
	if len(lines) == 0 {
		return reply(code, "")
	}
	var b strings.Builder
	for i, line := range lines {
		sep := "-"
		if i == len(lines)-1 {
			sep = " "
		}
		fmt.Fprintf(&b, "%d%s%s\r\n", code, sep, line)
	}
	return b.String()
}
