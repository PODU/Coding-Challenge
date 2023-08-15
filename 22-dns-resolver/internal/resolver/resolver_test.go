package resolver

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"dnsresolve/internal/message"
)

// reply describes what one nameserver returns for one query.
type reply struct {
	answers    []message.Record
	authority  []message.Record
	additional []message.Record
	rcode      uint16
}

// fakeExchanger answers queries from an in-memory topology. It echoes the query
// ID and question so the resolver's verification passes, exercising the real
// pack/unpack codec. No network is used.
type fakeExchanger struct {
	handler func(server netip.Addr, qname string, qtype uint16) reply
	calls   []netip.Addr
}

func (f *fakeExchanger) Exchange(server netip.AddrPort, query []byte) ([]byte, error) {
	qm, err := message.Unpack(query)
	if err != nil {
		return nil, err
	}
	q := qm.Questions[0]
	f.calls = append(f.calls, server.Addr())
	rep := f.handler(server.Addr(), strings.ToLower(q.Name), q.Type)

	resp := &message.Message{
		Questions:  qm.Questions,
		Answers:    rep.answers,
		Authority:  rep.authority,
		Additional: rep.additional,
	}
	resp.Header.ID = qm.Header.ID
	resp.Header.SetQR(true)
	resp.Header.Flags |= rep.rcode & 0x000F
	out, err := resp.Pack()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func encName(name string) []byte {
	name = strings.TrimSuffix(name, ".")
	var b []byte
	if name != "" {
		for _, l := range strings.Split(name, ".") {
			b = append(b, byte(len(l)))
			b = append(b, l...)
		}
	}
	return append(b, 0)
}

func aRecord(name, ip string) message.Record {
	addr := netip.MustParseAddr(ip).As4()
	return message.Record{Name: name, Type: message.TypeA, Class: message.ClassIN, TTL: 300, RData: addr[:]}
}

func nsRecord(zone, ns string) message.Record {
	return message.Record{Name: zone, Type: message.TypeNS, Class: message.ClassIN, TTL: 600, RData: encName(ns)}
}

func cnameRecord(name, canonical string) message.Record {
	return message.Record{Name: name, Type: message.TypeCNAME, Class: message.ClassIN, TTL: 300, RData: encName(canonical)}
}

const (
	rootAddr   = "198.41.0.4" // a.root-servers.net, tried first
	gtldAddr   = "192.5.6.30"
	googleAddr = "216.239.32.10"
)

// dnsGoogleTopology models root -> com -> google.com -> answer.
func dnsGoogleTopology(server netip.Addr, qname string, qtype uint16) reply {
	switch server.String() {
	case rootAddr:
		return reply{
			authority:  []message.Record{nsRecord("com", "a.gtld-servers.net")},
			additional: []message.Record{aRecord("a.gtld-servers.net", gtldAddr)},
		}
	case gtldAddr:
		return reply{
			authority:  []message.Record{nsRecord("google.com", "ns1.google.com")},
			additional: []message.Record{aRecord("ns1.google.com", googleAddr)},
		}
	case googleAddr:
		if qname == "dns.google.com" {
			return reply{answers: []message.Record{
				aRecord("dns.google.com", "8.8.8.8"),
				aRecord("dns.google.com", "8.8.4.4"),
			}}
		}
		return reply{rcode: message.RCodeNXDomain}
	}
	return reply{rcode: message.RCodeServFail}
}

func TestResolveRecursiveFromRoot(t *testing.T) {
	fx := &fakeExchanger{handler: dnsGoogleTopology}
	res, err := New(fx).Resolve("dns.google.com", message.TypeA)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got := addrStrings(res.Addrs)
	want := []string{"8.8.8.8", "8.8.4.4"}
	if !equalStrings(got, want) {
		t.Errorf("addrs = %v, want %v", got, want)
	}

	// The trace must show the progression root -> gtld -> google.
	kinds := traceKinds(res.Trace)
	if len(kinds) != 3 || kinds[0] != "referral" || kinds[1] != "referral" || kinds[2] != "answer" {
		t.Errorf("trace kinds = %v, want [referral referral answer]", kinds)
	}
	if res.Trace[0].ServerAddr.String() != rootAddr {
		t.Errorf("first server = %s, want root %s", res.Trace[0].ServerAddr, rootAddr)
	}
	if res.Trace[2].ServerAddr.String() != googleAddr {
		t.Errorf("final server = %s, want %s", res.Trace[2].ServerAddr, googleAddr)
	}
}

func TestResolveNXDomain(t *testing.T) {
	fx := &fakeExchanger{handler: dnsGoogleTopology}
	_, err := New(fx).Resolve("nope.google.com", message.TypeA)
	if !errors.Is(err, ErrNXDomain) {
		t.Fatalf("err = %v, want ErrNXDomain", err)
	}
}

// cnameTopology returns a CNAME for www.example.com and resolves the canonical.
func cnameTopology(server netip.Addr, qname string, qtype uint16) reply {
	const exampleAddr = "93.184.216.34"
	const exampleNS = "1.1.1.1"
	switch server.String() {
	case rootAddr:
		return reply{
			authority:  []message.Record{nsRecord("com", "a.gtld-servers.net")},
			additional: []message.Record{aRecord("a.gtld-servers.net", gtldAddr)},
		}
	case gtldAddr:
		return reply{
			authority:  []message.Record{nsRecord("example.com", "ns.example.com")},
			additional: []message.Record{aRecord("ns.example.com", exampleNS)},
		}
	case exampleNS:
		switch qname {
		case "www.example.com":
			return reply{answers: []message.Record{cnameRecord("www.example.com", "target.example.com")}}
		case "target.example.com":
			return reply{answers: []message.Record{aRecord("target.example.com", exampleAddr)}}
		}
	}
	return reply{rcode: message.RCodeNXDomain}
}

func TestResolveCNAMERestart(t *testing.T) {
	fx := &fakeExchanger{handler: cnameTopology}
	res, err := New(fx).Resolve("www.example.com", message.TypeA)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := addrStrings(res.Addrs); !equalStrings(got, []string{"93.184.216.34"}) {
		t.Errorf("addrs = %v, want [93.184.216.34]", got)
	}
	if res.CNAME != "target.example.com" {
		t.Errorf("CNAME = %q, want target.example.com", res.CNAME)
	}
}

// glulessTopology forces resolution of an NS hostname that has no glue.
func gluelessTopology(server netip.Addr, qname string, qtype uint16) reply {
	const betaNS = "10.0.0.2"
	switch server.String() {
	case rootAddr:
		if strings.HasSuffix(qname, "beta") {
			// beta zone: in-zone NS WITH glue.
			return reply{
				authority:  []message.Record{nsRecord("beta", "ns.beta")},
				additional: []message.Record{aRecord("ns.beta", betaNS)},
			}
		}
		// alpha zone: delegated to ns.beta, NO glue provided.
		return reply{authority: []message.Record{nsRecord("alpha", "ns.beta")}}
	case betaNS:
		switch qname {
		case "ns.beta":
			return reply{answers: []message.Record{aRecord("ns.beta", betaNS)}}
		case "www.alpha":
			return reply{answers: []message.Record{aRecord("www.alpha", "5.6.7.8")}}
		}
	}
	return reply{rcode: message.RCodeServFail}
}

func TestResolveNSWithoutGlue(t *testing.T) {
	fx := &fakeExchanger{handler: gluelessTopology}
	res, err := New(fx).Resolve("www.alpha", message.TypeA)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := addrStrings(res.Addrs); !equalStrings(got, []string{"5.6.7.8"}) {
		t.Errorf("addrs = %v, want [5.6.7.8]", got)
	}
}

func TestResolveNSQuery(t *testing.T) {
	// Authoritative NS answer in the ANSWER section.
	handler := func(server netip.Addr, qname string, qtype uint16) reply {
		switch server.String() {
		case rootAddr:
			return reply{
				authority:  []message.Record{nsRecord("com", "a.gtld-servers.net")},
				additional: []message.Record{aRecord("a.gtld-servers.net", gtldAddr)},
			}
		case gtldAddr:
			if qtype == message.TypeNS && qname == "example.com" {
				return reply{answers: []message.Record{
					nsRecord("example.com", "ns1.example.com"),
					nsRecord("example.com", "ns2.example.com"),
				}}
			}
		}
		return reply{rcode: message.RCodeServFail}
	}
	fx := &fakeExchanger{handler: handler}
	res, err := New(fx).Resolve("example.com", message.TypeNS)
	if err != nil {
		t.Fatalf("Resolve NS: %v", err)
	}
	if !equalStrings(res.Names, []string{"ns1.example.com", "ns2.example.com"}) {
		t.Errorf("NS names = %v", res.Names)
	}
}

func addrStrings(addrs []netip.Addr) []string {
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

func traceKinds(trace []Trace) []string {
	out := make([]string, len(trace))
	for i, t := range trace {
		out[i] = t.Kind
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
