package whois

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestParseStripsCommentsAndBlanks(t *testing.T) {
	raw := "% a comment\n# another comment\n\n>>> banner: ignore <<<\nKey: value\n  Indented: yes\nnot a field line\n: missingkey\n"
	p := Parse(raw)
	if len(p.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d: %#v", len(p.Fields), p.Fields)
	}
	if p.Get("Key") != "value" {
		t.Errorf("Key = %q, want value", p.Get("Key"))
	}
	if p.Get("Indented") != "yes" {
		t.Errorf("Indented = %q, want yes", p.Get("Indented"))
	}
}

func TestParseValueWithColons(t *testing.T) {
	p := Parse("Domain Status: clientTransferProhibited https://icann.org/epp#x\n")
	if got := p.Get("Domain Status"); got != "clientTransferProhibited https://icann.org/epp#x" {
		t.Errorf("value with embedded colon mis-split: %q", got)
	}
}

func TestParseIANAReferral(t *testing.T) {
	p := Parse(loadFixture(t, "iana_com.txt"))
	if got := p.Get("refer"); got != "whois.verisign-grs.com" {
		t.Errorf("refer = %q, want whois.verisign-grs.com", got)
	}
	if got := p.Referral(); got != "whois.verisign-grs.com" {
		t.Errorf("Referral() = %q, want whois.verisign-grs.com", got)
	}
}

func TestParseVerisignRecord(t *testing.T) {
	p := Parse(loadFixture(t, "verisign_example_com.txt"))

	if got := p.Referral(); got != "whois.exampleregistrar.com" {
		t.Errorf("Referral() = %q, want whois.exampleregistrar.com (Registrar WHOIS Server)", got)
	}

	r := &Record{}
	r.Merge(p)

	if r.Domain != "EXAMPLE.COM" {
		t.Errorf("Domain = %q", r.Domain)
	}
	if r.Registrar != "Example Registrar, LLC" {
		t.Errorf("Registrar = %q", r.Registrar)
	}
	if r.RegistrarWHOIS != "whois.exampleregistrar.com" {
		t.Errorf("RegistrarWHOIS = %q", r.RegistrarWHOIS)
	}
	if r.Created != "1995-08-14T04:00:00Z" {
		t.Errorf("Created = %q", r.Created)
	}
	if r.Expires != "2024-08-13T04:00:00Z" {
		t.Errorf("Expires = %q", r.Expires)
	}
	if r.DNSSEC != "signedDelegation" {
		t.Errorf("DNSSEC = %q", r.DNSSEC)
	}
	if len(r.NameServers) != 2 {
		t.Errorf("NameServers = %v, want 2 entries", r.NameServers)
	}
	if len(r.Status) != 2 {
		t.Errorf("Status = %v, want 2 entries", r.Status)
	}
}

func TestMergeRegistryThenRegistrar(t *testing.T) {
	r := &Record{}
	r.Merge(Parse(loadFixture(t, "verisign_example_com.txt")))
	r.Merge(Parse(loadFixture(t, "registrar_example_com.txt")))

	if r.RegistrantOrg != "Internet Example Org" {
		t.Errorf("RegistrantOrg = %q, want Internet Example Org", r.RegistrantOrg)
	}
	if r.RegistrantCountry != "US" {
		t.Errorf("RegistrantCountry = %q, want US", r.RegistrantCountry)
	}
	// Name servers from both hops are de-duplicated case-insensitively.
	if len(r.NameServers) != 2 {
		t.Errorf("NameServers = %v, want 2 (deduped)", r.NameServers)
	}
}

func TestParseARINIPRecord(t *testing.T) {
	p := Parse(loadFixture(t, "arin_ip.txt"))
	r := &Record{}
	r.Merge(p)

	if r.NetRange != "149.72.0.0 - 149.72.255.255" {
		t.Errorf("NetRange = %q", r.NetRange)
	}
	if r.CIDR != "149.72.0.0/16" {
		t.Errorf("CIDR = %q", r.CIDR)
	}
	if r.NetName != "SENDGRID" {
		t.Errorf("NetName = %q", r.NetName)
	}
	if r.OrgName != "Twilio SendGrid" {
		t.Errorf("OrgName = %q, want Twilio SendGrid", r.OrgName)
	}
	if r.Country != "US" {
		t.Errorf("Country = %q", r.Country)
	}
	if p.Referral() != "" {
		t.Errorf("IP record should have no referral, got %q", p.Referral())
	}
}

func TestNoMatchDetection(t *testing.T) {
	p := Parse(loadFixture(t, "no_match.txt"))
	if !p.NoMatch() {
		t.Error("no_match.txt should be detected as no-match")
	}
	good := Parse(loadFixture(t, "verisign_example_com.txt"))
	if good.NoMatch() {
		t.Error("a real record must not be flagged as no-match")
	}
	for _, body := range []string{
		`NOT FOUND`,
		`% No entries found for the selected source(s).`,
		`Domain not found.`,
	} {
		if !Parse(body).NoMatch() {
			t.Errorf("body %q should be no-match", body)
		}
	}
}

func TestCleanServer(t *testing.T) {
	cases := map[string]string{
		"whois.verisign-grs.com":     "whois.verisign-grs.com",
		"whois://whois.ripe.net":     "whois.ripe.net",
		"whois.example.com:43":       "whois.example.com",
		"whois.example.com:4343":     "whois.example.com:4343",
		"https://whois.example.com/": "whois.example.com",
	}
	for in, want := range cases {
		if got := cleanServer(in); got != want {
			t.Errorf("cleanServer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReferralPriorityAndAbsence(t *testing.T) {
	// Registrar WHOIS Server is honoured when refer/whois are absent.
	p := Parse("Registrar WHOIS Server: whois.r.example\n")
	if got := p.Referral(); got != "whois.r.example" {
		t.Errorf("Referral() = %q", got)
	}
	// No referral fields -> empty.
	if got := Parse("Domain Name: example.com\n").Referral(); got != "" {
		t.Errorf("Referral() = %q, want empty", got)
	}
}
