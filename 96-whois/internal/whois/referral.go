package whois

import (
	"context"
	"strings"
)

// DefaultMaxHops limits how many referrals we follow.
const DefaultMaxHops = 8

// Hop is one server we asked and what it said.
type Hop struct {
	Server  string
	Request string
	Raw     string
	Parsed  *Parsed
}

// Lookup asks the Seed server, follows referrals and merges the answers.
type Lookup struct {
	Client  *Client
	Seed    string
	MaxHops int
	Follow  bool
	Verbose bool
	Log     func(format string, args ...any) // used when Verbose is set
}

// Result is what a lookup found.
type Result struct {
	Query   string
	Type    QueryType
	Hops    []Hop
	Record  *Record
	NoMatch bool
}

// NewLookup returns a Lookup seeded at IANA that follows referrals.
func NewLookup(client *Client) *Lookup {
	return &Lookup{
		Client:  client,
		Seed:    ianaServer,
		MaxHops: DefaultMaxHops,
		Follow:  true,
	}
}

func (l *Lookup) logf(format string, args ...any) {
	if l.Verbose && l.Log != nil {
		l.Log(format, args...)
	}
}

// Do looks up query. If the first server can't be reached that is an error;
// if a later one fails we return what we have so far.
func (l *Lookup) Do(ctx context.Context, query string) (*Result, error) {
	qtype := Classify(query)
	res := &Result{Query: query, Type: qtype, Record: &Record{Query: query}}

	server := l.Seed
	if server == "" {
		server = ianaServer
	}
	maxHops := l.MaxHops
	if maxHops <= 0 {
		maxHops = DefaultMaxHops
	}
	visited := map[string]bool{}

	for hop := 0; hop < maxHops && server != ""; hop++ {
		key := strings.ToLower(server)
		if visited[key] {
			break
		}
		visited[key] = true

		request := FormatQuery(server, query, qtype)
		l.logf("hop %d: %s <- %q", hop+1, server, request)

		raw, err := l.Client.Query(ctx, server, request)
		if err != nil {
			if len(res.Hops) == 0 {
				return nil, err
			}
			l.logf("hop %d error: %v", hop+1, err)
			break
		}

		parsed := Parse(raw)
		res.Hops = append(res.Hops, Hop{Server: server, Request: request, Raw: raw, Parsed: parsed})
		res.Record.Merge(parsed)

		if !l.Follow {
			break
		}
		next := parsed.Referral()
		if next == "" || visited[strings.ToLower(next)] {
			break
		}
		server = next
	}

	// Only call it a miss if the last server said so and no hop gave us a
	// usable record.
	if n := len(res.Hops); n > 0 {
		if res.Hops[n-1].Parsed.NoMatch() && res.Record.isEmpty() {
			res.NoMatch = true
		}
	}
	return res, nil
}
