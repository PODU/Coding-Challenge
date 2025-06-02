package smtp

import "testing"

// run issues a single command line through the session's dispatcher and returns
// the reply code emitted plus whether the session signaled quit.
func run(s *Session, conn *fakeConn, line string) (code int, quit bool) {
	before := len(conn.output())
	cmd, err := ParseLine(line)
	if err != nil {
		s.send(reply(CodeSyntaxError, "Error: command not recognized"))
		return lastCode(conn.output()[before:]), false
	}
	quit = s.dispatch(cmd)
	return lastCode(conn.output()[before:]), quit
}

func TestStateMachineTransitions(t *testing.T) {
	t.Run("MAIL before HELO -> 503", func(t *testing.T) {
		s, conn, _ := newTestSession("")
		if code, _ := run(s, conn, "MAIL FROM:<a@b>"); code != CodeBadSequence {
			t.Fatalf("got %d, want %d", code, CodeBadSequence)
		}
	})

	t.Run("happy path codes 250,250,250,354", func(t *testing.T) {
		// DATA body preloaded so handleData can read to the terminator.
		s, conn, _ := newTestSession("Subject: hi\r\n\r\nbody\r\n.\r\n")
		if code, _ := run(s, conn, "HELO test"); code != CodeOK {
			t.Fatalf("HELO got %d", code)
		}
		if s.state != StateGreeted {
			t.Fatalf("after HELO state=%v", s.state)
		}
		if code, _ := run(s, conn, "MAIL FROM:<s@ex.com>"); code != CodeOK {
			t.Fatalf("MAIL got %d", code)
		}
		if s.state != StateMail {
			t.Fatalf("after MAIL state=%v", s.state)
		}
		if code, _ := run(s, conn, "RCPT TO:<r@ex.com>"); code != CodeOK {
			t.Fatalf("RCPT got %d", code)
		}
		if s.state != StateRcpt {
			t.Fatalf("after RCPT state=%v", s.state)
		}
		// DATA returns 354 then reads body and returns 250; lastCode sees 250.
		before := len(conn.output())
		run(s, conn, "DATA")
		out := conn.output()[before:]
		lines := splitReplyLines(out)
		if len(lines) < 2 || lines[0][:3] != "354" {
			t.Fatalf("DATA first reply = %q, want 354", lines[0])
		}
		if lastCode(out) != CodeOK {
			t.Fatalf("DATA final reply = %d, want 250", lastCode(out))
		}
		if s.state != StateGreeted {
			t.Fatalf("after DATA store state=%v, want Greeted", s.state)
		}
	})

	t.Run("RCPT before MAIL -> 503", func(t *testing.T) {
		s, conn, _ := newTestSession("")
		run(s, conn, "HELO test")
		if code, _ := run(s, conn, "RCPT TO:<r@ex.com>"); code != CodeBadSequence {
			t.Fatalf("got %d, want 503", code)
		}
	})

	t.Run("DATA with zero recipients -> 503", func(t *testing.T) {
		s, conn, _ := newTestSession("")
		run(s, conn, "HELO test")
		run(s, conn, "MAIL FROM:<s@ex.com>")
		if code, _ := run(s, conn, "DATA"); code != CodeBadSequence {
			t.Fatalf("got %d, want 503", code)
		}
	})

	t.Run("nested MAIL -> 503", func(t *testing.T) {
		s, conn, _ := newTestSession("")
		run(s, conn, "HELO test")
		run(s, conn, "MAIL FROM:<s@ex.com>")
		if code, _ := run(s, conn, "MAIL FROM:<other@ex.com>"); code != CodeBadSequence {
			t.Fatalf("got %d, want 503", code)
		}
	})

	t.Run("RSET from Rcpt -> Greeted, 250, empty txn", func(t *testing.T) {
		s, conn, _ := newTestSession("")
		run(s, conn, "HELO test")
		run(s, conn, "MAIL FROM:<s@ex.com>")
		run(s, conn, "RCPT TO:<r@ex.com>")
		if code, _ := run(s, conn, "RSET"); code != CodeOK {
			t.Fatalf("RSET got %d", code)
		}
		if s.state != StateGreeted {
			t.Fatalf("after RSET state=%v", s.state)
		}
		if s.tx.Started() || s.tx.From != "" || len(s.tx.To) != 0 {
			t.Fatalf("transaction not cleared: %+v", s.tx)
		}
	})

	t.Run("bad syntax MAIL -> 501", func(t *testing.T) {
		s, conn, _ := newTestSession("")
		run(s, conn, "HELO test")
		if code, _ := run(s, conn, "MAIL FROM:garbage"); code != CodeParamError {
			t.Fatalf("got %d, want 501", code)
		}
	})
}

func TestSimpleCommands(t *testing.T) {
	s, conn, _ := newTestSession("")
	if code, _ := run(s, conn, "NOOP"); code != CodeOK {
		t.Errorf("NOOP got %d, want 250", code)
	}
	if code, _ := run(s, conn, "VRFY someone"); code != CodeCannotVRFY {
		t.Errorf("VRFY got %d, want 252", code)
	}
	if code, _ := run(s, conn, "FROBNICATE"); code != CodeSyntaxError {
		t.Errorf("unknown got %d, want 500", code)
	}
	code, quit := run(s, conn, "QUIT")
	if code != CodeServiceClosing || !quit {
		t.Errorf("QUIT got code=%d quit=%v, want 221,true", code, quit)
	}
}

func TestNoopInEveryState(t *testing.T) {
	s, conn, _ := newTestSession("")
	for _, setup := range [][]string{
		{},
		{"HELO test"},
		{"HELO test", "MAIL FROM:<s@ex.com>"},
		{"HELO test", "MAIL FROM:<s@ex.com>", "RCPT TO:<r@ex.com>"},
	} {
		s, conn, _ = newTestSession("")
		for _, c := range setup {
			run(s, conn, c)
		}
		if code, _ := run(s, conn, "NOOP"); code != CodeOK {
			t.Errorf("NOOP after %v got %d", setup, code)
		}
	}
}

func TestEhloMultiline(t *testing.T) {
	s, conn, _ := newTestSession("")
	before := len(conn.output())
	run(s, conn, "EHLO client.example")
	out := conn.output()[before:]
	lines := splitReplyLines(out)
	if len(lines) < 2 {
		t.Fatalf("EHLO produced %d lines, want multiline", len(lines))
	}
	for i, l := range lines {
		if l[:3] != "250" {
			t.Errorf("line %d code = %s, want 250", i, l[:3])
		}
		sep := l[3]
		if i == len(lines)-1 {
			if sep != ' ' {
				t.Errorf("last line separator = %q, want space", sep)
			}
		} else if sep != '-' {
			t.Errorf("line %d separator = %q, want hyphen", i, sep)
		}
	}
}

func TestReplyCodeConstants(t *testing.T) {
	cases := map[int]int{
		CodeServiceReady:    220,
		CodeServiceClosing:  221,
		CodeOK:              250,
		CodeCannotVRFY:      252,
		CodeStartMailInput:  354,
		CodeServiceNotAvail: 421,
		CodeSyntaxError:     500,
		CodeParamError:      501,
		CodeNotImplemented:  502,
		CodeBadSequence:     503,
		CodeMessageTooBig:   552,
		CodeTransactionFail: 554,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant = %d, want %d", got, want)
		}
	}
}
