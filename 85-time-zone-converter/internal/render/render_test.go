package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tzc/internal/convert"
	"tzc/internal/zones"
)

func TestFormatOffset(t *testing.T) {
	cases := map[int]string{
		0:              "+00:00",
		5*3600 + 30*60: "+05:30",
		5*3600 + 45*60: "+05:45",
		-5 * 3600:      "-05:00",
		-8 * 3600:      "-08:00",
		9 * 3600:       "+09:00",
	}
	for sec, want := range cases {
		if got := FormatOffset(sec); got != want {
			t.Errorf("FormatOffset(%d) = %q, want %q", sec, got, want)
		}
	}
}

func TestResolveLayout(t *testing.T) {
	if l, _ := ResolveLayout("", false); l != "2006-01-02 15:04" {
		t.Errorf("default layout = %q", l)
	}
	if l, _ := ResolveLayout("", true); !strings.Contains(l, "PM") {
		t.Errorf("12h layout should contain PM, got %q", l)
	}
	if l, _ := ResolveLayout("iso", false); l != time.RFC3339 {
		t.Errorf("iso layout = %q", l)
	}
}

// T21: JSON output validates against the documented schema and is stable.
func TestJSONOutput(t *testing.T) {
	src, _ := zones.Resolve("UTC")
	tgt, _ := zones.Resolve("Asia/Kolkata")
	instant := time.Date(2025, 6, 1, 9, 0, 0, 0, time.UTC)
	cs := convert.Convert(instant, []zones.Zone{src.Zone(), tgt.Zone()}, src.Loc)

	out := Output{Source: src.Name, SourceTime: instant.Format(time.RFC3339)}
	for _, c := range cs {
		out.Conversions = append(out.Conversions, ToConversionJSON(c, "2006-01-02 15:04"))
	}

	var buf bytes.Buffer
	if err := JSON(&buf, out); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	// Round-trip back into the schema.
	var got Output
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Source != "UTC" {
		t.Errorf("source = %q", got.Source)
	}
	if len(got.Conversions) != 2 {
		t.Fatalf("conversions = %d, want 2", len(got.Conversions))
	}
	kolkata := got.Conversions[1]
	if kolkata.Zone != "Asia/Kolkata" {
		t.Errorf("zone = %q", kolkata.Zone)
	}
	if kolkata.Offset != "+05:30" {
		t.Errorf("offset = %q, want +05:30", kolkata.Offset)
	}
	if kolkata.Local != "2025-06-01 14:30" {
		t.Errorf("local = %q, want 2025-06-01 14:30", kolkata.Local)
	}
	if kolkata.DeltaSeconds != 5*3600+30*60 {
		t.Errorf("delta = %d", kolkata.DeltaSeconds)
	}

	// Stability: encoding twice yields identical bytes.
	var buf2 bytes.Buffer
	_ = JSON(&buf2, out)
	if buf.String() != buf2.String() {
		t.Errorf("JSON output not stable")
	}
}

func TestConversionsText(t *testing.T) {
	src, _ := zones.Resolve("UTC")
	tgt, _ := zones.Resolve("Asia/Tokyo")
	instant := time.Date(2025, 1, 15, 23, 0, 0, 0, time.UTC)
	cs := convert.Convert(instant, []zones.Zone{src.Zone(), tgt.Zone()}, src.Loc)
	var buf bytes.Buffer
	Conversions(&buf, "Source: UTC", cs, nil, Options{Layout: "2006-01-02 15:04"})
	s := buf.String()
	if !strings.Contains(s, "Asia/Tokyo") {
		t.Errorf("output missing Asia/Tokyo:\n%s", s)
	}
	// Tokyo is +1 day relative to 23:00 UTC.
	if !strings.Contains(s, "(+1d)") {
		t.Errorf("output should mark cross-day rollover:\n%s", s)
	}
}
