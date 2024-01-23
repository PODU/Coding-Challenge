package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

// HostReport is the per-host section of a report.
type HostReport struct {
	Host     string   `json:"host"`
	Open     int      `json:"open"`
	Closed   int      `json:"closed"`
	Filtered int      `json:"filtered"`
	Ports    []Result `json:"ports"`
}

// Report is the full machine-readable scan report.
type Report struct {
	Concurrency  int          `json:"concurrency"`
	TimeoutMs    int64        `json:"timeout_ms"`
	PortsPerHost int          `json:"ports_per_host"`
	ElapsedMs    int64        `json:"elapsed_ms"`
	Hosts        []HostReport `json:"hosts"`
}

// BuildReport groups results by host. Unless verbose is set, only open ports
// are listed, though closed and filtered ones are still counted.
func BuildReport(results []Result, concurrency int, timeout, elapsed time.Duration, portsPerHost int, verbose bool) Report {
	byHost := make(map[string]*HostReport)
	var order []string

	for _, r := range results {
		hr, ok := byHost[r.Host]
		if !ok {
			hr = &HostReport{Host: r.Host, Ports: []Result{}}
			byHost[r.Host] = hr
			order = append(order, r.Host)
		}
		switch r.State {
		case Open:
			hr.Open++
		case Closed:
			hr.Closed++
		case Filtered:
			hr.Filtered++
		}
		if verbose || r.State == Open {
			hr.Ports = append(hr.Ports, r)
		}
	}

	sort.Strings(order)
	hosts := make([]HostReport, 0, len(order))
	for _, h := range order {
		hr := byHost[h]
		sort.SliceStable(hr.Ports, func(i, j int) bool { return hr.Ports[i].Port < hr.Ports[j].Port })
		hosts = append(hosts, *hr)
	}

	return Report{
		Concurrency:  concurrency,
		TimeoutMs:    timeout.Milliseconds(),
		PortsPerHost: portsPerHost,
		ElapsedMs:    elapsed.Milliseconds(),
		Hosts:        hosts,
	}
}

// RenderJSON writes the report as indented JSON.
func RenderJSON(w io.Writer, rep Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// RenderText writes the report as a table per host.
func RenderText(w io.Writer, rep Report, verbose bool) error {
	for _, hr := range rep.Hosts {
		if _, err := fmt.Fprintf(w, "Host: %s\n", hr.Host); err != nil {
			return err
		}

		if len(hr.Ports) == 0 {
			if _, err := fmt.Fprintln(w, "  (no open ports — host may be down, firewalled, or all ports closed)"); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "  %-10s %-9s %s\n", "PORT", "STATE", "SERVICE"); err != nil {
				return err
			}
			for _, p := range hr.Ports {
				if _, err := fmt.Fprintf(w, "  %-10s %-9s %s\n", fmt.Sprintf("%d/tcp", p.Port), p.State, p.Service); err != nil {
					return err
				}
			}
		}

		total := hr.Open + hr.Closed + hr.Filtered
		if _, err := fmt.Fprintf(w, "\n  %d ports scanned: %d open, %d closed, %d filtered  (elapsed %s)\n\n",
			total, hr.Open, hr.Closed, hr.Filtered, time.Duration(rep.ElapsedMs)*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}
