package whois

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		in   string
		want QueryType
	}{
		{"example.com", TypeDomain},
		{"sub.example.co.uk", TypeDomain},
		{"8.8.8.8", TypeIPv4},
		{"149.72.235.118", TypeIPv4},
		{"2001:4860:4860::8888", TypeIPv6},
		{"AS15169", TypeASN},
		{"as13335", TypeASN},
		{"AS", TypeDomain},      // no digits -> not an ASN
		{"ASDF", TypeDomain},    // letters after AS -> not an ASN
		{" 8.8.8.8 ", TypeIPv4}, // surrounding whitespace tolerated
	}
	for _, c := range cases {
		if got := Classify(c.in); got != c.want {
			t.Errorf("Classify(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFormatQuery(t *testing.T) {
	cases := []struct {
		name   string
		server string
		query  string
		typ    QueryType
		want   string
	}{
		{"plain domain", "whois.nic.example", "example.com", TypeDomain, "example.com"},
		{"verisign forces domain flag", "whois.verisign-grs.com", "example.com", TypeDomain, "domain example.com"},
		{"arin ip flag", "whois.arin.net", "8.8.8.8", TypeIPv4, "n + 8.8.8.8"},
		{"arin ipv6 flag", "whois.arin.net", "2001:4860:4860::8888", TypeIPv6, "n + 2001:4860:4860::8888"},
		{"arin asn flag", "whois.arin.net", "AS15169", TypeASN, "a + AS15169"},
		{"bare asn elsewhere", "whois.iana.org", "AS15169", TypeASN, "AS15169"},
		{"trims whitespace", "whois.nic.example", "  example.com  ", TypeDomain, "example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FormatQuery(c.server, c.query, c.typ); got != c.want {
				t.Errorf("FormatQuery(%q, %q, %v) = %q, want %q",
					c.server, c.query, c.typ, got, c.want)
			}
		})
	}
}
