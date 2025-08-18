package whois

import (
	"net"
	"strings"
)

// Field is one "key: value" line. Keys can repeat, e.g. Name Server.
type Field struct {
	Key   string
	Value string
}

// Parsed is one parsed WHOIS response.
type Parsed struct {
	Fields []Field // in the order they appeared
	Raw    string
}

// Parse splits a WHOIS response into fields. WHOIS has no fixed format, so
// this just takes every "key: value" line and ignores comments and anything
// else.
func Parse(raw string) *Parsed {
	p := &Parsed{Raw: raw}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if isCommentLine(line) {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		p.Fields = append(p.Fields, Field{Key: key, Value: val})
	}
	return p
}

// isCommentLine reports whether line is blank, a % or # comment, or one of
// the ">>> Last update ..." banner lines.
func isCommentLine(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || t[0] == '%' || t[0] == '#' || strings.HasPrefix(t, ">>>")
}

// Get returns the first value for a key (case-insensitive), or "".
func (p *Parsed) Get(key string) string {
	for _, f := range p.Fields {
		if strings.EqualFold(f.Key, key) {
			return f.Value
		}
	}
	return ""
}

// GetAll returns every value for a key (case-insensitive), in order.
func (p *Parsed) GetAll(key string) []string {
	var out []string
	for _, f := range p.Fields {
		if strings.EqualFold(f.Key, key) {
			out = append(out, f.Value)
		}
	}
	return out
}

// First returns the value of the first of keys that is present.
func (p *Parsed) First(keys ...string) string {
	for _, k := range keys {
		if v := p.Get(k); v != "" {
			return v
		}
	}
	return ""
}

// referralKeys are the fields that name the next server to ask, best first.
var referralKeys = []string{"refer", "whois", "Registrar WHOIS Server"}

// Referral returns the next server to query, or "".
func (p *Parsed) Referral() string {
	for _, k := range referralKeys {
		if v := p.Get(k); v != "" {
			return cleanServer(v)
		}
	}
	return ""
}

// cleanServer turns a referral such as "whois://whois.example.com:43/" into
// something we can dial.
func cleanServer(v string) string {
	v = strings.TrimSpace(v)
	for _, scheme := range []string{"whois://", "rwhois://", "https://", "http://"} {
		v = strings.TrimPrefix(v, scheme)
	}
	v = strings.TrimSuffix(v, "/")
	if host, port, err := net.SplitHostPort(v); err == nil && port == "43" {
		return host
	}
	return v
}

// noMatchPatterns are the ways servers say "not found", in lower case. There
// is no standard wording.
var noMatchPatterns = []string{
	"no match for",
	"not found",
	"no entries found",
	"no data found",
	"domain not found",
	"no object found",
	"object does not exist",
	"nothing found",
	"status: free",
}

// NoMatch reports whether the response says the object wasn't found.
func (p *Parsed) NoMatch() bool {
	lower := strings.ToLower(p.Raw)
	for _, pat := range noMatchPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// Record combines the responses from every hop into one set of fields.
type Record struct {
	Query             string   `json:"query,omitempty"`
	Domain            string   `json:"domain,omitempty"`
	Registrar         string   `json:"registrar,omitempty"`
	RegistrarWHOIS    string   `json:"registrar_whois,omitempty"`
	Status            []string `json:"status,omitempty"`
	Created           string   `json:"created,omitempty"`
	Updated           string   `json:"updated,omitempty"`
	Expires           string   `json:"expires,omitempty"`
	NameServers       []string `json:"name_servers,omitempty"`
	DNSSEC            string   `json:"dnssec,omitempty"`
	RegistrantOrg     string   `json:"registrant_org,omitempty"`
	RegistrantCountry string   `json:"registrant_country,omitempty"`

	// IP address lookups.
	NetRange string `json:"net_range,omitempty"`
	CIDR     string `json:"cidr,omitempty"`
	NetName  string `json:"net_name,omitempty"`
	OrgName  string `json:"org_name,omitempty"`
	Country  string `json:"country,omitempty"`
}

// Merge adds p to the record. Later hops are closer to the source, so their
// values win; list fields are combined without duplicates.
func (r *Record) Merge(p *Parsed) {
	set := func(dst *string, keys ...string) {
		if v := p.First(keys...); v != "" {
			*dst = v
		}
	}
	set(&r.Domain, "Domain Name", "domain")
	set(&r.Registrar, "Registrar", "Sponsoring Registrar")
	set(&r.RegistrarWHOIS, "Registrar WHOIS Server")
	set(&r.Created, "Creation Date", "created", "Registered on", "Registration Time")
	set(&r.Updated, "Updated Date", "last-update", "changed", "Last Modified")
	set(&r.Expires, "Registry Expiry Date", "Registrar Registration Expiration Date",
		"Expiration Date", "Expiry Date", "paid-till")
	set(&r.DNSSEC, "DNSSEC")
	set(&r.RegistrantOrg, "Registrant Organization", "org")
	set(&r.RegistrantCountry, "Registrant Country")

	set(&r.NetRange, "NetRange", "inetnum")
	set(&r.CIDR, "CIDR")
	set(&r.NetName, "NetName", "netname")
	set(&r.OrgName, "OrgName", "org-name", "Organization", "owner")
	set(&r.Country, "Country", "country")

	r.Status = appendUnique(r.Status, p.GetAll("Domain Status")...)
	r.Status = appendUnique(r.Status, p.GetAll("status")...)
	r.NameServers = appendUnique(r.NameServers, p.GetAll("Name Server")...)
	r.NameServers = appendUnique(r.NameServers, p.GetAll("nserver")...)
}

// isEmpty reports whether the record has nothing that identifies an object.
func (r *Record) isEmpty() bool {
	return r.Domain == "" && r.Registrar == "" &&
		r.NetRange == "" && r.CIDR == "" && r.OrgName == ""
}

// appendUnique appends the values not already in dst, ignoring case.
func appendUnique(dst []string, vals ...string) []string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		dup := false
		for _, e := range dst {
			if strings.EqualFold(e, v) {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, v)
		}
	}
	return dst
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
