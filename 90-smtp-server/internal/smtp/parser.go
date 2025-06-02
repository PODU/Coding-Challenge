package smtp

import (
	"errors"
	"strings"
)

const (
	VerbHELO = "HELO"
	VerbEHLO = "EHLO"
	VerbMAIL = "MAIL"
	VerbRCPT = "RCPT"
	VerbDATA = "DATA"
	VerbRSET = "RSET"
	VerbNOOP = "NOOP"
	VerbVRFY = "VRFY"
	VerbQUIT = "QUIT"
)

// Command is a parsed command line. Verb is upper-cased; Arg is the rest of
// the line after the verb.
type Command struct {
	Verb string
	Arg  string
}

// ErrEmptyLine is returned by ParseLine when the input is blank.
var ErrEmptyLine = errors.New("empty command line")

// ParseLine splits a command line into verb and argument. Verbs are
// case-insensitive in SMTP, so the verb is upper-cased.
func ParseLine(line string) (Command, error) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return Command{}, ErrEmptyLine
	}
	verb := line
	arg := ""
	if i := strings.IndexAny(line, " \t"); i >= 0 {
		verb = line[:i]
		arg = strings.TrimLeft(line[i+1:], " \t")
	}
	return Command{Verb: strings.ToUpper(verb), Arg: arg}, nil
}

// ErrBadAddress is returned for a MAIL FROM or RCPT TO argument without a
// usable address.
var ErrBadAddress = errors.New("malformed address")

// parsePath extracts the address from the argument of MAIL or RCPT:
//
//	"FROM:<a@b.com>"            -> "a@b.com"
//	"TO: <c@d.com>"            -> "c@d.com"
//	"FROM:<a@b> SIZE=1000"     -> "a@b" (ESMTP params ignored)
//	"FROM:<>"                  -> ""    (null reverse-path, valid for MAIL)
//
// allowEmpty permits "<>", which only MAIL FROM accepts.
func parsePath(arg, keyword string, allowEmpty bool) (string, error) {
	upper := strings.ToUpper(strings.TrimLeft(arg, " \t"))
	prefix := keyword + ":"
	if !strings.HasPrefix(upper, prefix) {
		return "", ErrBadAddress
	}
	// Slice the original, not upper, to keep the address's case.
	rest := strings.TrimLeft(arg, " \t")[len(prefix):]
	rest = strings.TrimLeft(rest, " \t")

	if !strings.HasPrefix(rest, "<") {
		return "", ErrBadAddress
	}
	end := strings.IndexByte(rest, '>')
	if end < 0 {
		return "", ErrBadAddress
	}
	addr := strings.TrimSpace(rest[1:end])
	if addr == "" && !allowEmpty {
		return "", ErrBadAddress
	}
	return addr, nil
}

// ParseMailFrom returns the sender address from a MAIL argument.
func ParseMailFrom(arg string) (string, error) {
	return parsePath(arg, "FROM", true)
}

// ParseRcptTo returns the recipient address from an RCPT argument.
func ParseRcptTo(arg string) (string, error) {
	return parsePath(arg, "TO", false)
}
