// Package resolver resolves names iteratively, starting at the root servers
// and following referrals until some server answers. It follows CNAMEs and
// keeps a trace of every server it asked.
package resolver

import (
	"errors"
	"fmt"
	"math/rand"
	"net/netip"
	"strings"

	"dnsresolve/internal/message"
	"dnsresolve/internal/transport"
)

var (
	ErrNXDomain = errors.New("resolver: NXDOMAIN (name does not exist)")
	ErrNoAnswer = errors.New("resolver: no records found")
	// ErrNoProgress means a referral named nameservers we couldn't reach.
	ErrNoProgress = errors.New("resolver: no answer and no usable referral")
	ErrHopLimit   = errors.New("resolver: hop limit exceeded")
	// ErrServFail means none of the servers for a zone gave a usable reply.
	ErrServFail = errors.New("resolver: all nameservers failed")
)

const (
	maxHops   = 32 // referrals followed for one name
	maxDepth  = 16 // nested lookups for nameservers without glue
	dnsPort   = 53
	maxCNAMEs = 16
)

// Trace records one query: which server was asked and what it said.
type Trace struct {
	ServerName string
	ServerAddr netip.Addr
	QName      string
	QType      uint16
	RCode      uint16
	Kind       string   // "answer", "referral", "cname", "error"
	Detail     []string // records or referral targets, for printing
}

type Result struct {
	Name    string
	QType   uint16
	Addrs   []netip.Addr     // for A and AAAA
	Answers []message.Record // records of the requested type
	Names   []string         // for NS and CNAME
	CNAME   string           // the last name in the CNAME chain, if any
	RCode   uint16
	Trace   []Trace
}

type Resolver struct {
	Exchanger transport.Exchanger
}

func New(x transport.Exchanger) *Resolver {
	return &Resolver{Exchanger: x}
}

func (r *Resolver) newID() uint16 {
	return uint16(rand.Intn(1 << 16))
}

func (r *Resolver) Resolve(name string, qtype uint16) (*Result, error) {
	name = normalize(name)
	res := &Result{Name: name, QType: qtype}
	err := r.resolve(name, qtype, 0, 0, res)
	return res, err
}

// resolve does the work of Resolve. depth counts nested lookups of nameserver
// addresses and cnames counts CNAMEs followed so far.
func (r *Resolver) resolve(name string, qtype uint16, depth, cnames int, res *Result) error {
	if depth > maxDepth {
		return fmt.Errorf("resolver: max recursion depth exceeded resolving %q", name)
	}

	servers := rootCandidates()

	for hop := 0; hop < maxHops; hop++ {
		msg, raw, srv, err := r.queryAny(servers, name, qtype, res)
		if err != nil {
			return err
		}

		rcode := msg.Header.RCode()
		if rcode == message.RCodeNXDomain {
			res.RCode = rcode
			r.trace(res, srv, name, qtype, rcode, "answer", []string{"NXDOMAIN"})
			return ErrNXDomain
		}
		if rcode != message.RCodeNoError {
			res.RCode = rcode
			return fmt.Errorf("resolver: server %s returned %s", srv.Name, message.RCodeName(rcode))
		}

		if recs := matchAnswers(msg.Answers, name, qtype); len(recs) > 0 {
			res.RCode = rcode
			res.Answers = append(res.Answers, recs...)
			res.Addrs = append(res.Addrs, recordsToAddrs(recs)...)
			res.Names = append(res.Names, recordNames(recs, raw)...)
			r.trace(res, srv, name, qtype, rcode, "answer", describeRecords(recs, raw))
			return nil
		}

		// An alias: start again from the root with the canonical name.
		if cname, ok := findCNAME(msg.Answers, name, raw); ok {
			if cnames+1 > maxCNAMEs {
				return fmt.Errorf("resolver: CNAME chain too long at %q", name)
			}
			res.CNAME = cname
			r.trace(res, srv, name, qtype, rcode, "cname", []string{name + " -> " + cname})
			// Some servers include the target's records alongside the CNAME.
			if recs := matchAnswers(msg.Answers, cname, qtype); len(recs) > 0 {
				res.Answers = append(res.Answers, recs...)
				res.Addrs = append(res.Addrs, recordsToAddrs(recs)...)
				res.Names = append(res.Names, recordNames(recs, raw)...)
				return nil
			}
			return r.resolve(cname, qtype, depth, cnames+1, res)
		}

		// Otherwise it should be a referral to the next zone down.
		next, detail, err := r.referralServers(msg, raw, depth, cnames, res)
		if err != nil {
			return err
		}
		if len(next) == 0 {
			r.trace(res, srv, name, qtype, rcode, "answer", []string{"no records"})
			res.RCode = rcode
			return ErrNoAnswer
		}
		r.trace(res, srv, name, qtype, rcode, "referral", detail)
		servers = next
	}
	return ErrHopLimit
}

// queryAny returns the first reply it gets from servers, trying them in order.
func (r *Resolver) queryAny(servers []nameAddr, name string, qtype uint16, res *Result) (*message.Message, []byte, nameAddr, error) {
	var lastErr error
	for _, srv := range servers {
		msg, raw, err := r.queryOne(srv.Addr, name, qtype)
		if err != nil {
			lastErr = err
			r.trace(res, srv, name, qtype, 0, "error", []string{err.Error()})
			continue
		}
		return msg, raw, srv, nil
	}
	if lastErr == nil {
		lastErr = ErrServFail
	}
	return nil, nil, nameAddr{}, fmt.Errorf("%w: %v", ErrServFail, lastErr)
}

// queryOne sends a non-recursive query and checks that the reply matches it.
// The raw bytes are returned too, for decoding compressed names in RData.
func (r *Resolver) queryOne(server netip.Addr, name string, qtype uint16) (*message.Message, []byte, error) {
	id := r.newID()
	q := message.NewQuery(id, name, qtype, message.ClassIN, false)
	query, err := q.Pack()
	if err != nil {
		return nil, nil, err
	}
	raw, err := r.Exchanger.Exchange(netip.AddrPortFrom(server, dnsPort), query)
	if err != nil {
		return nil, nil, err
	}
	msg, err := message.Unpack(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("decode response from %s: %w", server, err)
	}
	if msg.Header.ID != id {
		return nil, nil, fmt.Errorf("response ID mismatch from %s: want %d got %d", server, id, msg.Header.ID)
	}
	if !msg.Header.QR() {
		return nil, nil, fmt.Errorf("response from %s has QR=0", server)
	}
	if len(msg.Questions) != 1 || !equalName(msg.Questions[0].Name, name) || msg.Questions[0].Type != qtype {
		return nil, nil, fmt.Errorf("response from %s echoed wrong question", server)
	}
	return msg, raw, nil
}

// referralServers returns the nameservers a referral points to. Addresses
// come from the glue records if there are any; otherwise the first
// nameserver name that can be resolved is used.
func (r *Resolver) referralServers(msg *message.Message, raw []byte, depth, cnames int, res *Result) ([]nameAddr, []string, error) {
	var nsNames []string
	for _, rec := range msg.Authority {
		if rec.Type == message.TypeNS {
			if ns, ok := rec.NSString(raw); ok {
				nsNames = append(nsNames, normalize(ns))
			}
		}
	}
	if len(nsNames) == 0 {
		return nil, nil, nil
	}

	glue := map[string][]netip.Addr{}
	for _, rec := range msg.Additional {
		if addr, ok := rec.AString(); ok {
			glue[normalize(rec.Name)] = append(glue[normalize(rec.Name)], addr)
		}
	}

	var next []nameAddr
	var detail []string
	for _, ns := range nsNames {
		if addrs := glue[ns]; len(addrs) > 0 {
			for _, a := range addrs {
				next = append(next, nameAddr{Name: ns, Addr: a})
			}
			detail = append(detail, fmt.Sprintf("NS %s (glue %s)", ns, addrs[0]))
		} else {
			detail = append(detail, fmt.Sprintf("NS %s (no glue)", ns))
		}
	}

	if len(next) == 0 {
		for _, ns := range nsNames {
			sub := &Result{Name: ns, QType: message.TypeA}
			if err := r.resolve(ns, message.TypeA, depth+1, cnames, sub); err == nil && len(sub.Addrs) > 0 {
				res.Trace = append(res.Trace, sub.Trace...)
				for _, a := range sub.Addrs {
					next = append(next, nameAddr{Name: ns, Addr: a})
				}
				detail = append(detail, fmt.Sprintf("NS %s (resolved %s)", ns, sub.Addrs[0]))
				break
			}
		}
	}

	if len(next) == 0 {
		return nil, detail, ErrNoProgress
	}
	return next, detail, nil
}

func (r *Resolver) trace(res *Result, srv nameAddr, qname string, qtype, rcode uint16, kind string, detail []string) {
	res.Trace = append(res.Trace, Trace{
		ServerName: srv.Name,
		ServerAddr: srv.Addr,
		QName:      qname,
		QType:      qtype,
		RCode:      rcode,
		Kind:       kind,
		Detail:     detail,
	})
}

type nameAddr struct {
	Name string
	Addr netip.Addr
}

func rootCandidates() []nameAddr {
	roots := rootServers()
	out := make([]nameAddr, 0, len(roots))
	for _, rs := range roots {
		out = append(out, nameAddr{Name: rs.Name, Addr: rs.Addr})
	}
	return out
}

func matchAnswers(recs []message.Record, name string, qtype uint16) []message.Record {
	var out []message.Record
	for _, rec := range recs {
		if rec.Type == qtype && equalName(rec.Name, name) {
			out = append(out, rec)
		}
	}
	return out
}

// findCNAME returns the target of a CNAME for name, if recs has one.
func findCNAME(recs []message.Record, name string, raw []byte) (string, bool) {
	for _, rec := range recs {
		if rec.Type == message.TypeCNAME && equalName(rec.Name, name) {
			if cn, ok := rec.CNAMEString(raw); ok {
				return normalize(cn), true
			}
		}
	}
	return "", false
}

// recordNames returns the names held by any NS and CNAME records.
func recordNames(recs []message.Record, raw []byte) []string {
	var out []string
	for _, rec := range recs {
		switch rec.Type {
		case message.TypeNS:
			if ns, ok := rec.NSString(raw); ok {
				out = append(out, ns)
			}
		case message.TypeCNAME:
			if cn, ok := rec.CNAMEString(raw); ok {
				out = append(out, cn)
			}
		}
	}
	return out
}

func recordsToAddrs(recs []message.Record) []netip.Addr {
	var out []netip.Addr
	for _, rec := range recs {
		if a, ok := rec.AString(); ok {
			out = append(out, a)
		} else if a, ok := rec.AAAAString(); ok {
			out = append(out, a)
		}
	}
	return out
}

func describeRecords(recs []message.Record, raw []byte) []string {
	var out []string
	for _, rec := range recs {
		switch {
		case rec.Type == message.TypeA, rec.Type == message.TypeAAAA:
			if a, ok := rec.AString(); ok {
				out = append(out, fmt.Sprintf("%s A %s (ttl %d)", rec.Name, a, rec.TTL))
			} else if a, ok := rec.AAAAString(); ok {
				out = append(out, fmt.Sprintf("%s AAAA %s (ttl %d)", rec.Name, a, rec.TTL))
			}
		case rec.Type == message.TypeNS:
			if ns, ok := rec.NSString(raw); ok {
				out = append(out, fmt.Sprintf("%s NS %s (ttl %d)", rec.Name, ns, rec.TTL))
			}
		case rec.Type == message.TypeCNAME:
			if cn, ok := rec.CNAMEString(raw); ok {
				out = append(out, fmt.Sprintf("%s CNAME %s (ttl %d)", rec.Name, cn, rec.TTL))
			}
		default:
			out = append(out, fmt.Sprintf("%s %s (ttl %d)", rec.Name, message.TypeName(rec.Type), rec.TTL))
		}
	}
	return out
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, "."))
}

func equalName(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}
