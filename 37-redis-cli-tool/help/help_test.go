package help

import (
	"strings"
	"testing"
)

func TestOverview(t *testing.T) {
	got := Overview()
	for _, want := range []string{"help <command>", "help @<group>", "@string", "@connection"} {
		if !strings.Contains(got, want) {
			t.Errorf("Overview() missing %q; got:\n%s", want, got)
		}
	}
}

func TestCommandHelp(t *testing.T) {
	got, ok := CommandHelp("get")
	if !ok {
		t.Fatal("CommandHelp(get) not found")
	}
	for _, want := range []string{"GET key", "summary: Get the value of a key", "since: 1.0.0", "group: string"} {
		if !strings.Contains(got, want) {
			t.Errorf("CommandHelp(get) missing %q; got:\n%s", want, got)
		}
	}
	if _, ok := CommandHelp("nosuchcmd"); ok {
		t.Error("CommandHelp(nosuchcmd) should not be found")
	}
}

func TestGroupHelp(t *testing.T) {
	got, ok := GroupHelp("@string")
	if !ok {
		t.Fatal("GroupHelp(@string) not found")
	}
	if !strings.Contains(got, "GET key") || !strings.Contains(got, "SET key") {
		t.Errorf("GroupHelp(@string) incomplete; got:\n%s", got)
	}
	// Leading @ optional.
	if _, ok := GroupHelp("list"); !ok {
		t.Error("GroupHelp(list) should be found without @")
	}
	if _, ok := GroupHelp("nope"); ok {
		t.Error("GroupHelp(nope) should not be found")
	}
}

func TestUsage(t *testing.T) {
	got, ok := Usage("SET")
	if !ok || !strings.HasPrefix(got, "SET key value") {
		t.Errorf("Usage(SET) = %q, %v", got, ok)
	}
	if got, ok := Usage("DBSIZE"); !ok || got != "DBSIZE" {
		t.Errorf("Usage(DBSIZE) = %q, %v; want bare name", got, ok)
	}
}
