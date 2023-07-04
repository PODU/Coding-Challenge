package client

import (
	"strings"
	"sync"
	"testing"

	"ircclient/internal/parser"
)

// fakeSender records every line sent, with locking so it is safe to share with
// concurrent senders.
type fakeSender struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeSender) Send(line string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, line)
	return nil
}

func (f *fakeSender) lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	copy(out, f.sent)
	return out
}

func (f *fakeSender) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1]
}

func newTestClient(cfg Config) (*Client, *fakeSender) {
	if cfg.Nick == "" {
		cfg.Nick = "CCClient"
	}
	if cfg.User == "" {
		cfg.User = "guest"
	}
	if cfg.Real == "" {
		cfg.Real = "Coding Challenges Client"
	}
	f := &fakeSender{}
	return New(cfg, f), f
}

func mustParse(t *testing.T, line string) parser.Message {
	t.Helper()
	m, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	return m
}

func TestRegister(t *testing.T) {
	c, f := newTestClient(Config{Nick: "CCClient", User: "guest", Real: "Coding Challenges Client"})
	if err := c.Register(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"NICK CCClient",
		"USER guest 0 * :Coding Challenges Client",
	}
	got := f.lines()
	if len(got) != len(want) {
		t.Fatalf("sent %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sent[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAutoPong(t *testing.T) {
	c, f := newTestClient(Config{})
	evs := c.Handle(mustParse(t, "PING :LAG1234567"))
	if got := f.last(); got != "PONG :LAG1234567" {
		t.Errorf("auto-PONG sent %q, want %q", got, "PONG :LAG1234567")
	}
	if len(evs) != 0 {
		t.Errorf("PING should be silent without debug, got events %v", evs)
	}
}

func TestPingBeforeRegistration(t *testing.T) {
	c, f := newTestClient(Config{})
	if c.Registered() {
		t.Fatal("should not be registered yet")
	}
	c.Handle(mustParse(t, "PING :early"))
	if f.last() != "PONG :early" {
		t.Errorf("PONG must work before registration, got %q", f.last())
	}
}

func TestWelcomeSetsRegistered(t *testing.T) {
	c, f := newTestClient(Config{Nick: "CCClient", Channel: "#cc"})
	evs := c.Handle(mustParse(t, ":*.libera.chat 001 CCIRC :Welcome to the Network CCIRC"))
	if !c.Registered() {
		t.Error("expected registered after 001")
	}
	if c.Nick() != "CCIRC" {
		t.Errorf("nick = %q, want server-assigned CCIRC", c.Nick())
	}
	if len(evs) != 1 || evs[0].Kind != KindInfo {
		t.Fatalf("expected one info event, got %v", evs)
	}
	if !strings.Contains(evs[0].Text, "Connected to *.libera.chat as CCIRC") {
		t.Errorf("welcome text = %q", evs[0].Text)
	}
	if f.last() != "JOIN #cc" {
		t.Errorf("expected auto-join, last sent = %q", f.last())
	}
}

func TestSelfJoinSetsActiveChannel(t *testing.T) {
	c, _ := newTestClient(Config{Nick: "CCIRC"})
	evs := c.Handle(mustParse(t, ":CCIRC!~guest@host JOIN :#cc"))
	if c.Channel() != "#cc" {
		t.Errorf("active channel = %q, want #cc", c.Channel())
	}
	if len(evs) != 1 || evs[0].Kind != KindJoin || evs[0].Text != "* CCIRC has joined #cc" {
		t.Errorf("join event = %v", evs)
	}
}

func TestOtherJoinDoesNotChangeChannel(t *testing.T) {
	c, _ := newTestClient(Config{Nick: "CCIRC"})
	c.Handle(mustParse(t, ":CCIRC!~g@h JOIN :#cc"))
	c.Handle(mustParse(t, ":Bob!~b@h JOIN :#other"))
	if c.Channel() != "#cc" {
		t.Errorf("channel changed to %q on other user's join", c.Channel())
	}
}

func TestSelfNickUpdatesState(t *testing.T) {
	c, _ := newTestClient(Config{Nick: "Old"})
	evs := c.Handle(mustParse(t, ":Old!~g@h NICK :New"))
	if c.Nick() != "New" {
		t.Errorf("nick = %q, want New", c.Nick())
	}
	if evs[0].Text != "* Old is now known as New" {
		t.Errorf("nick event text = %q", evs[0].Text)
	}
}

func TestPrivmsgRendering(t *testing.T) {
	c, _ := newTestClient(Config{Nick: "CCIRC"})
	tests := []struct {
		line string
		kind Kind
		text string
	}{
		{":Alice!~a@h PRIVMSG #cc :hello world", KindMessage, "Alice: hello world"},
		{":Alice!~a@h PRIVMSG CCIRC :hi privately", KindPM, "[PM] Alice: hi privately"},
		{":Alice!~a@h PRIVMSG #cc :\x01ACTION waves\x01", KindAction, "* Alice waves"},
	}
	for _, tt := range tests {
		evs := c.Handle(mustParse(t, tt.line))
		if len(evs) != 1 || evs[0].Kind != tt.kind || evs[0].Text != tt.text {
			t.Errorf("Handle(%q) = %v, want kind %d text %q", tt.line, evs, tt.kind, tt.text)
		}
	}
}

func TestQuitRendering(t *testing.T) {
	c, _ := newTestClient(Config{})
	evs := c.Handle(mustParse(t, ":Alice!~a@h QUIT :Quit: bye bye!"))
	if evs[0].Text != "* Alice has left IRC (Quit: bye bye!)" {
		t.Errorf("quit text = %q", evs[0].Text)
	}
}

func TestNumericErrorRendering(t *testing.T) {
	c, _ := newTestClient(Config{})
	evs := c.Handle(mustParse(t, ":srv 433 * CCClient :Nickname is already in use"))
	if evs[0].Kind != KindError || !strings.Contains(evs[0].Text, "433") {
		t.Errorf("433 event = %v", evs)
	}
}

func TestServerErrorTriggersShutdown(t *testing.T) {
	c, _ := newTestClient(Config{})
	evs := c.Handle(mustParse(t, "ERROR :Closing Link: timeout"))
	if len(evs) != 2 || evs[1].Kind != KindShutdown {
		t.Fatalf("ERROR should produce error + shutdown, got %v", evs)
	}
}

func TestHandleInputMapping(t *testing.T) {
	tests := []struct {
		name      string
		preChan   string
		input     string
		wantSends []string
		wantKind  *Kind
		wantErr   bool
	}{
		{name: "plain text", preChan: "#cc", input: "hello there", wantSends: []string{"PRIVMSG #cc :hello there"}},
		{name: "plain no channel", input: "hello", wantSends: nil, wantKind: kindp(KindError)},
		{name: "join", input: "/join #go", wantSends: []string{"JOIN #go"}},
		{name: "part active", preChan: "#cc", input: "/part", wantSends: []string{"PART :#cc"}},
		{name: "part named", preChan: "#cc", input: "/part #other", wantSends: []string{"PART :#other"}},
		{name: "nick", input: "/nick Better", wantSends: []string{"NICK Better"}},
		{name: "msg", input: "/msg Bob hi there", wantSends: []string{"PRIVMSG Bob :hi there"}},
		{name: "me", preChan: "#cc", input: "/me waves", wantSends: []string{"PRIVMSG #cc :\x01ACTION waves\x01"}},
		{name: "raw", input: "/raw CAP LS 302", wantSends: []string{"CAP LS 302"}},
		{name: "unknown command", input: "/bogus", wantSends: nil, wantKind: kindp(KindError)},
		{name: "empty ignored", input: "   ", wantSends: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, f := newTestClient(Config{Nick: "Me"})
			if tt.preChan != "" {
				c.mu.Lock()
				c.channel = tt.preChan
				c.mu.Unlock()
			}
			evs, err := c.HandleInput(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			got := f.lines()
			if len(got) != len(tt.wantSends) {
				t.Fatalf("sends = %v, want %v", got, tt.wantSends)
			}
			for i := range tt.wantSends {
				if got[i] != tt.wantSends[i] {
					t.Errorf("send[%d] = %q, want %q", i, got[i], tt.wantSends[i])
				}
			}
			if tt.wantKind != nil {
				if len(evs) == 0 || evs[0].Kind != *tt.wantKind {
					t.Errorf("events = %v, want first kind %d", evs, *tt.wantKind)
				}
			}
		})
	}
}

func TestQuitInputShutsDown(t *testing.T) {
	c, f := newTestClient(Config{})
	evs, _ := c.HandleInput("/quit see ya")
	if f.last() != "QUIT :see ya" {
		t.Errorf("quit sent %q", f.last())
	}
	if len(evs) != 1 || evs[0].Kind != KindShutdown {
		t.Errorf("expected shutdown event, got %v", evs)
	}
}

func TestQuitInputDefaultReason(t *testing.T) {
	c, f := newTestClient(Config{})
	c.HandleInput("/quit")
	if !strings.HasPrefix(f.last(), "QUIT :") {
		t.Errorf("default quit = %q", f.last())
	}
}

// TestConcurrentSendSerialisation fires many concurrent input lines and checks
// that no send line is garbled (the client/sender must serialise writes).
func TestConcurrentSends(t *testing.T) {
	c, f := newTestClient(Config{Nick: "Me"})
	c.mu.Lock()
	c.channel = "#cc"
	c.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.HandleInput("hello")
		}()
	}
	wg.Wait()
	for _, l := range f.lines() {
		if l != "PRIVMSG #cc :hello" {
			t.Fatalf("garbled concurrent send: %q", l)
		}
	}
}

func kindp(k Kind) *Kind { return &k }
