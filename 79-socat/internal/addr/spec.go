// Package addr parses socat-style address arguments.
package addr

import (
	"fmt"
	"strings"
)

// Kind is the address type, with aliases folded together.
type Kind int

const (
	KindInvalid Kind = iota
	KindSTDIO
	KindTCP
	KindTCPListen
	KindUDP
	KindUDPListen
	KindFile // FILE or OPEN
	KindExec // EXEC or SYSTEM
)

func (k Kind) String() string {
	switch k {
	case KindSTDIO:
		return "STDIO"
	case KindTCP:
		return "TCP"
	case KindTCPListen:
		return "TCP-LISTEN"
	case KindUDP:
		return "UDP"
	case KindUDPListen:
		return "UDP-LISTEN"
	case KindFile:
		return "FILE"
	case KindExec:
		return "EXEC"
	default:
		return "INVALID"
	}
}

// AddressSpec is one parsed address argument.
type AddressSpec struct {
	Kind    Kind
	Net     string            // "tcp", "udp6" etc. for net.Dial; empty for non-network kinds
	Raw     string            // the argument as given, for error messages
	Params  []string          // the colon-separated part: host, port, path, command
	Options map[string]string // the comma-separated part; bare flags map to ""
}

// HasOption reports whether an option is set. key is case-insensitive.
func (s *AddressSpec) HasOption(key string) bool {
	_, ok := s.Options[strings.ToLower(key)]
	return ok
}

// Host returns the host of a TCP or UDP client address.
func (s *AddressSpec) Host() string {
	if len(s.Params) > 0 {
		return s.Params[0]
	}
	return ""
}

// Port returns the port of any TCP or UDP address.
func (s *AddressSpec) Port() string {
	switch s.Kind {
	case KindTCP, KindUDP:
		if len(s.Params) > 1 {
			return s.Params[1]
		}
	case KindTCPListen, KindUDPListen:
		if len(s.Params) > 0 {
			return s.Params[0]
		}
	}
	return ""
}

// Path returns the path of a FILE address.
func (s *AddressSpec) Path() string {
	if s.Kind == KindFile && len(s.Params) > 0 {
		return s.Params[0]
	}
	return ""
}

// Command returns the command line of an EXEC address.
func (s *AddressSpec) Command() string {
	if s.Kind == KindExec && len(s.Params) > 0 {
		return s.Params[0]
	}
	return ""
}

// IsListener reports whether the address listens rather than connects.
func (s *AddressSpec) IsListener() bool {
	return s.Kind == KindTCPListen || s.Kind == KindUDPListen
}

// allowedOptions lists the options each kind accepts. Anything else is an
// error rather than being silently ignored.
var allowedOptions = map[Kind]map[string]bool{
	KindSTDIO:     {},
	KindTCP:       {},
	KindUDP:       {},
	KindTCPListen: {"reuseaddr": true, "fork": true},
	KindUDPListen: {"reuseaddr": true, "fork": true},
	KindFile: {
		"create": true, "creat": true, "append": true,
		"rdonly": true, "wronly": true, "rdwr": true, "trunc": true,
	},
	KindExec: {},
}

// Parse parses one address argument.
func Parse(raw string) (*AddressSpec, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty address spec")
	}

	upper := strings.ToUpper(raw)
	if raw == "-" || upper == "STDIO" || upper == "STDIN!!STDOUT" {
		return &AddressSpec{Kind: KindSTDIO, Raw: raw, Options: map[string]string{}}, nil
	}

	typeKw, rest, _ := strings.Cut(raw, ":")

	kind, netw, ok := normalizeType(typeKw)
	if !ok {
		return nil, fmt.Errorf("unknown address type %q in %q", typeKw, raw)
	}

	spec := &AddressSpec{Kind: kind, Net: netw, Raw: raw, Options: map[string]string{}}

	// The EXEC command is kept whole, since it can contain colons and commas.
	if kind == KindExec {
		cmd := stripQuotes(rest)
		if strings.TrimSpace(cmd) == "" {
			return nil, fmt.Errorf("EXEC requires a command in %q", raw)
		}
		spec.Params = []string{cmd}
		return spec, nil
	}

	paramsPart, optsPart, _ := strings.Cut(rest, ",")

	if paramsPart != "" {
		for _, p := range splitColons(paramsPart) {
			spec.Params = append(spec.Params, stripBrackets(p))
		}
	}

	if optsPart != "" {
		for _, o := range strings.Split(optsPart, ",") {
			if o == "" {
				continue
			}
			key, val, _ := strings.Cut(o, "=")
			key = strings.ToLower(key)
			if !allowedOptions[kind][key] {
				return nil, fmt.Errorf("unknown option %q for %s in %q", key, kind, raw)
			}
			spec.Options[key] = val
		}
	}

	if err := validateParams(spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func validateParams(s *AddressSpec) error {
	switch s.Kind {
	case KindSTDIO:
		if len(s.Params) != 0 {
			return fmt.Errorf("STDIO takes no parameters in %q", s.Raw)
		}
	case KindTCP, KindUDP:
		if len(s.Params) != 2 || s.Params[0] == "" || s.Params[1] == "" {
			return fmt.Errorf("%s requires host:port in %q", s.Kind, s.Raw)
		}
	case KindTCPListen, KindUDPListen:
		if len(s.Params) != 1 || s.Params[0] == "" {
			return fmt.Errorf("%s requires a port in %q", s.Kind, s.Raw)
		}
	case KindFile:
		if len(s.Params) != 1 || s.Params[0] == "" {
			return fmt.Errorf("FILE requires a path in %q", s.Raw)
		}
	}
	return nil
}

// normalizeType maps a type keyword, in any case, to its Kind and network.
func normalizeType(kw string) (Kind, string, bool) {
	switch strings.ToUpper(kw) {
	case "STDIO", "STDIN", "STDOUT", "STDIN!!STDOUT", "-":
		return KindSTDIO, "", true
	case "TCP", "TCP4", "TCP-CONNECT", "TCP4-CONNECT":
		return KindTCP, ipNet("tcp", kw), true
	case "TCP6", "TCP6-CONNECT":
		return KindTCP, "tcp6", true
	case "TCP-LISTEN", "TCP4-LISTEN", "TCP-L", "TCP4-L":
		return KindTCPListen, ipNet("tcp", kw), true
	case "TCP6-LISTEN", "TCP6-L":
		return KindTCPListen, "tcp6", true
	case "UDP", "UDP4", "UDP-CONNECT", "UDP4-CONNECT":
		return KindUDP, ipNet("udp", kw), true
	case "UDP6", "UDP6-CONNECT":
		return KindUDP, "udp6", true
	case "UDP-LISTEN", "UDP4-LISTEN", "UDP-L", "UDP4-L":
		return KindUDPListen, ipNet("udp", kw), true
	case "UDP6-LISTEN", "UDP6-L":
		return KindUDPListen, "udp6", true
	case "FILE", "OPEN":
		return KindFile, "", true
	case "EXEC", "SYSTEM":
		return KindExec, "", true
	default:
		return KindInvalid, "", false
	}
}

// ipNet returns base+"4" for keywords like TCP4, and base (either IP family)
// otherwise.
func ipNet(base, kw string) string {
	if strings.Contains(kw, "4") {
		return base + "4"
	}
	return base
}

// splitColons splits on ':' outside square brackets, so "[::1]:80" works.
func splitColons(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// stripBrackets turns "[::1]" into "::1".
func stripBrackets(s string) string {
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		return s[1 : len(s)-1]
	}
	return s
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
