package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	code := run(args, &out, &errb, now)
	return code, out.String(), errb.String()
}

func TestCLIConvert(t *testing.T) {
	code, out, _ := runCLI("-t", "2025-03-09 09:00", "-f", "new york", "london", "tokyo")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "Europe/London") || !strings.Contains(out, "Asia/Tokyo") {
		t.Errorf("missing zones:\n%s", out)
	}
}

func TestCLIUnknownZone(t *testing.T) {
	code, _, errb := runCLI("-f", "UTC", "Mars/Olympus")
	if code != exitUnknownZone {
		t.Errorf("exit = %d, want %d", code, exitUnknownZone)
	}
	if !strings.Contains(errb, "unknown zone") {
		t.Errorf("stderr = %q", errb)
	}
}

func TestCLIBadTime(t *testing.T) {
	code, _, errb := runCLI("-t", "tomorrowish", "-f", "UTC", "london")
	if code != exitBadTime {
		t.Errorf("exit = %d, want %d", code, exitBadTime)
	}
	if !strings.Contains(errb, "could not parse") {
		t.Errorf("stderr = %q", errb)
	}
}

func TestCLINoTargets(t *testing.T) {
	code, _, _ := runCLI("-f", "UTC")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
}

func TestCLISpringForwardWarning(t *testing.T) {
	_, out, _ := runCLI("-t", "2025-03-09 02:30", "-f", "America/New_York", "UTC")
	if !strings.Contains(out, "spring-forward gap") {
		t.Errorf("expected spring-forward warning:\n%s", out)
	}
}

func TestCLIJSON(t *testing.T) {
	code, out, _ := runCLI("-t", "2025-06-01 09:00", "-f", "UTC", "Asia/Kolkata", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	var doc struct {
		Source      string `json:"source"`
		Conversions []struct {
			Zone   string `json:"zone"`
			Offset string `json:"offset"`
			Local  string `json:"local"`
		} `json:"conversions"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(doc.Conversions) != 2 {
		t.Fatalf("conversions = %d", len(doc.Conversions))
	}
	var kolkata bool
	for _, c := range doc.Conversions {
		if c.Zone == "Asia/Kolkata" {
			kolkata = true
			if c.Offset != "+05:30" || c.Local != "2025-06-01 14:30" {
				t.Errorf("kolkata = %+v", c)
			}
		}
	}
	if !kolkata {
		t.Errorf("missing Asia/Kolkata in JSON")
	}
}

func TestCLIGrid(t *testing.T) {
	code, out, _ := runCLI("--grid", "-f", "UTC", "Asia/Kolkata")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "Planner grid") {
		t.Errorf("missing grid header:\n%s", out)
	}
	lines := strings.Count(out, "\n")
	if lines < 3 {
		t.Errorf("grid too short:\n%s", out)
	}
}

func TestCLIRange(t *testing.T) {
	code, out, _ := runCLI("--range", "09:00..17:00", "-f", "london", "new_york", "tokyo")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "START") || !strings.Contains(out, "END") {
		t.Errorf("missing range columns:\n%s", out)
	}
}

func TestCLIList(t *testing.T) {
	code, out, _ := runCLI("--list")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "City aliases") || !strings.Contains(out, "Abbreviations") {
		t.Errorf("list output incomplete:\n%s", out)
	}
}
