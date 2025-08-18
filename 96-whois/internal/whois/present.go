package whois

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PrintRaw writes each hop's raw response under the server's name.
func PrintRaw(w io.Writer, res *Result) {
	for i, h := range res.Hops {
		if i > 0 {
			fmt.Fprintln(w, strings.Repeat("-", 60))
		}
		fmt.Fprintf(w, "[%s]\n", h.Server)
		fmt.Fprintln(w, strings.TrimRight(h.Raw, "\n"))
	}
}

// PrintJSON writes the merged record as indented JSON.
func PrintJSON(w io.Writer, res *Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res.Record)
}

// PrintRecord writes the record as aligned "key : value" lines.
func PrintRecord(w io.Writer, res *Result) {
	r := res.Record

	type row struct{ k, v string }
	var rows []row
	add := func(k, v string) {
		if v != "" {
			rows = append(rows, row{k, v})
		}
	}

	switch res.Type {
	case TypeIPv4, TypeIPv6:
		add("Query", r.Query)
		add("NetRange", r.NetRange)
		add("CIDR", r.CIDR)
		add("NetName", r.NetName)
		add("Organization", r.OrgName)
		add("Country", r.Country)
	default:
		add("Domain", firstNonEmpty(r.Domain, r.Query))
		add("Registrar", r.Registrar)
		add("Registrar WHOIS", r.RegistrarWHOIS)
		add("Created", r.Created)
		add("Updated", r.Updated)
		add("Expires", r.Expires)
		add("Registrant Org", r.RegistrantOrg)
		add("Registrant Country", r.RegistrantCountry)
		add("DNSSEC", r.DNSSEC)
	}

	for _, ns := range r.NameServers {
		rows = append(rows, row{"Name Server", ns})
	}
	for _, s := range r.Status {
		rows = append(rows, row{"Status", s})
	}

	width := 0
	for _, rw := range rows {
		if len(rw.k) > width {
			width = len(rw.k)
		}
	}
	for _, rw := range rows {
		fmt.Fprintf(w, "%-*s : %s\n", width, rw.k, rw.v)
	}
}
