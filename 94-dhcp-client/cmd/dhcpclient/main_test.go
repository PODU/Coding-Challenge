package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDryRunPrintsDiscoverAndExitsZero(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--mac", "aa:bb:cc:dd:ee:ff", "--xid", "956560166"}, &out, &errb)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, exitOK, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "built DISCOVER xid=0x3903f326") {
		t.Errorf("stdout missing DISCOVER line:\n%s", s)
	}
	if !strings.Contains(s, "mac=aa:bb:cc:dd:ee:ff") {
		t.Errorf("stdout missing mac:\n%s", s)
	}
	if !strings.Contains(s, "no packets sent") {
		t.Errorf("stdout missing offline note:\n%s", s)
	}
}

func TestInvalidMACIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--mac", "not-a-mac"}, &out, &errb)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestInvalidRequestIPIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--mac", "aa:bb:cc:dd:ee:ff", "--request-ip", "999.1.1.1"}, &out, &errb)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}
