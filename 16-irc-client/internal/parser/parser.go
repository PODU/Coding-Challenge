// Package parser parses and builds IRC protocol lines (RFC 1459/2812, plus
// IRCv3 message tags).
package parser

import (
	"errors"
	"strings"
)

// ErrEmpty is returned by Parse when the input contains no command.
var ErrEmpty = errors.New("parser: empty message")

// Message is a decoded IRC protocol message.
//
// The grammar handled is:
//
//	["@" tags SPACE] [":" prefix SPACE] command [params] CRLF
//
// where the final parameter may be a "trailing" parameter introduced by " :",
// which is the only parameter permitted to contain spaces.
type Message struct {
	Raw     string            // the original line, with CRLF stripped
	Tags    map[string]string // IRCv3 tags (nil if none present)
	Prefix  string            // full "nick!user@host" or server name (no leading ':')
	Nick    string            // nick portion of the prefix, if any
	User    string            // user portion of the prefix, if any
	Host    string            // host portion of the prefix, if any
	Command string            // "PRIVMSG", "PING", "JOIN", "001", ...
	Params  []string          // all parameters, trailing captured whole
}

// Parse decodes one line, with or without its CRLF. A line with no command
// gives ErrEmpty.
func Parse(line string) (Message, error) {
	raw := strings.TrimRight(line, "\r\n")
	m := Message{Raw: raw}

	rest := raw
	// "@key=value;key2=value2 "
	if strings.HasPrefix(rest, "@") {
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			return m, errors.New("parser: tags without command")
		}
		m.Tags = parseTags(rest[1:sp])
		rest = strings.TrimLeft(rest[sp+1:], " ")
	}

	// ":nick!user@host "
	if strings.HasPrefix(rest, ":") {
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			return m, errors.New("parser: prefix without command")
		}
		m.Prefix = rest[1:sp]
		m.Nick, m.User, m.Host = splitPrefix(m.Prefix)
		rest = strings.TrimLeft(rest[sp+1:], " ")
	}

	rest = strings.TrimLeft(rest, " ")
	if rest == "" {
		return m, ErrEmpty
	}

	if sp := strings.IndexByte(rest, ' '); sp >= 0 {
		m.Command = rest[:sp]
		rest = strings.TrimLeft(rest[sp+1:], " ")
	} else {
		m.Command = rest
		rest = ""
	}
	m.Command = strings.ToUpper(m.Command)

	// The trailing parameter starts at the first " :" and may contain spaces.
	for rest != "" {
		if strings.HasPrefix(rest, ":") {
			m.Params = append(m.Params, rest[1:])
			break
		}
		sp := strings.IndexByte(rest, ' ')
		if sp < 0 {
			m.Params = append(m.Params, rest)
			break
		}
		m.Params = append(m.Params, rest[:sp])
		rest = strings.TrimLeft(rest[sp+1:], " ")
	}

	return m, nil
}

func parseTags(s string) map[string]string {
	tags := make(map[string]string)
	for _, kv := range strings.Split(s, ";") {
		if kv == "" {
			continue
		}
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			tags[kv[:eq]] = unescapeTag(kv[eq+1:])
		} else {
			tags[kv] = ""
		}
	}
	return tags
}

// unescapeTag reverses IRCv3 tag value escaping.
func unescapeTag(v string) string {
	if !strings.ContainsRune(v, '\\') {
		return v
	}
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' || i+1 >= len(v) {
			b.WriteByte(v[i])
			continue
		}
		i++
		switch v[i] {
		case ':':
			b.WriteByte(';')
		case 's':
			b.WriteByte(' ')
		case 'r':
			b.WriteByte('\r')
		case 'n':
			b.WriteByte('\n')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

// splitPrefix splits "nick!user@host". A server name comes back as the nick.
func splitPrefix(prefix string) (nick, user, host string) {
	rest := prefix
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		host = rest[at+1:]
		rest = rest[:at]
	}
	if bang := strings.IndexByte(rest, '!'); bang >= 0 {
		nick = rest[:bang]
		user = rest[bang+1:]
	} else {
		nick = rest
	}
	return nick, user, host
}

// IsNumeric reports whether the command is a 3-digit numeric reply.
func (m Message) IsNumeric() bool {
	if len(m.Command) != 3 {
		return false
	}
	for _, c := range m.Command {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Target returns the first parameter (usually a channel or nick), or "".
func (m Message) Target() string {
	if len(m.Params) == 0 {
		return ""
	}
	return m.Params[0]
}

// Trailing returns the last parameter, usually the message text, or "".
func (m Message) Trailing() string {
	if len(m.Params) == 0 {
		return ""
	}
	return m.Params[len(m.Params)-1]
}

// String re-encodes the message, prefix included, without CRLF. Meant for
// logging.
func (m Message) String() string {
	var b strings.Builder
	if m.Prefix != "" {
		b.WriteByte(':')
		b.WriteString(m.Prefix)
		b.WriteByte(' ')
	}
	b.WriteString(Encode(m.Command, m.Params...))
	return b.String()
}

// trailingCommands always get ':' before their last parameter, even a single
// word, since that parameter is free text.
var trailingCommands = map[string]bool{
	"PRIVMSG": true,
	"NOTICE":  true,
	"QUIT":    true,
	"PART":    true,
	"PING":    true,
	"PONG":    true,
	"ERROR":   true,
	"TOPIC":   true,
	"AWAY":    true,
	"KICK":    true,
	"USER":    true,
}

// Encode builds a line (without CRLF). The last parameter gets a ':' when the
// protocol needs one: it's empty, has a space, starts with ':', or belongs to
// one of trailingCommands.
func Encode(command string, params ...string) string {
	var b strings.Builder
	b.WriteString(command)
	for i, p := range params {
		b.WriteByte(' ')
		last := i == len(params)-1
		if last && needsTrailing(command, p) {
			b.WriteByte(':')
		}
		b.WriteString(p)
	}
	return b.String()
}

func needsTrailing(command, p string) bool {
	if trailingCommands[strings.ToUpper(command)] {
		return true
	}
	return p == "" || strings.ContainsRune(p, ' ') || strings.HasPrefix(p, ":")
}
