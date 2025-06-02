package smtp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

const (
	// The RFC limit for a line is 1000 octets; we are more lenient.
	maxLineBytes    = 64 * 1024
	maxMessageBytes = 25 * 1024 * 1024
)

// Session is one client connection.
type Session struct {
	server *Server
	conn   net.Conn
	r      *bufio.Reader
	w      *bufio.Writer

	state State
	tx    Transaction
	helo  string // domain the client gave in HELO/EHLO

	timeout time.Duration
}

func newSession(s *Server, conn net.Conn) *Session {
	return &Session{
		server:  s,
		conn:    conn,
		r:       bufio.NewReaderSize(conn, 4096),
		w:       bufio.NewWriter(conn),
		state:   StateGreet,
		timeout: s.Timeout,
	}
}

func (s *Session) serve() {
	defer s.conn.Close()

	s.send(reply(CodeServiceReady, s.server.Hostname+" CC SMTP Server ready"))

	for {
		s.touchDeadline()
		line, err := s.readLine()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				s.send(reply(CodeServiceNotAvail, "Timeout, closing connection"))
			}
			return
		}

		cmd, err := ParseLine(line)
		if err != nil {
			s.send(reply(CodeSyntaxError, "Error: command not recognized"))
			continue
		}

		if quit := s.dispatch(cmd); quit {
			return
		}
	}
}

// dispatch handles one command and reports whether the session should end.
func (s *Session) dispatch(cmd Command) (quit bool) {
	switch cmd.Verb {
	case VerbHELO:
		s.handleHelo(cmd.Arg)
	case VerbEHLO:
		s.handleEhlo(cmd.Arg)
	case VerbMAIL:
		s.handleMail(cmd.Arg)
	case VerbRCPT:
		s.handleRcpt(cmd.Arg)
	case VerbDATA:
		s.handleData()
	case VerbRSET:
		s.handleRset()
	case VerbNOOP:
		s.send(reply(CodeOK, "OK"))
	case VerbVRFY:
		s.send(reply(CodeCannotVRFY, "Cannot VRFY user, but will accept message"))
	case VerbQUIT:
		s.send(reply(CodeServiceClosing, s.server.Hostname+" closing connection"))
		return true
	default:
		s.send(reply(CodeSyntaxError, "Error: command not recognized"))
	}
	return false
}

func (s *Session) handleHelo(arg string) {
	domain := strings.TrimSpace(arg)
	if domain == "" {
		s.send(reply(CodeParamError, "HELO requires domain argument"))
		return
	}
	s.helo = domain
	s.tx.reset()
	s.state = StateGreeted
	s.send(reply(CodeOK, s.server.Hostname+" Hello "+s.helo))
}

func (s *Session) handleEhlo(arg string) {
	domain := strings.TrimSpace(arg)
	if domain == "" {
		s.send(reply(CodeParamError, "EHLO requires domain argument"))
		return
	}
	s.helo = domain
	s.tx.reset()
	s.state = StateGreeted
	lines := []string{
		s.server.Hostname + " Hello " + s.helo,
		"PIPELINING",
		"SIZE 26214400",
		"8BITMIME",
		"HELP",
	}
	s.send(multiReply(CodeOK, lines))
}

func (s *Session) handleMail(arg string) {
	if s.state == StateGreet {
		s.send(reply(CodeBadSequence, "Bad sequence: send HELO/EHLO first"))
		return
	}
	if s.tx.Started() {
		s.send(reply(CodeBadSequence, "Bad sequence: nested MAIL command"))
		return
	}
	from, err := ParseMailFrom(arg)
	if err != nil {
		s.send(reply(CodeParamError, "Syntax error in MAIL FROM argument"))
		return
	}
	s.tx.begin(from)
	s.state = StateMail
	s.send(reply(CodeOK, "OK"))
}

func (s *Session) handleRcpt(arg string) {
	if s.state != StateMail && s.state != StateRcpt {
		s.send(reply(CodeBadSequence, "Bad sequence: need MAIL FROM first"))
		return
	}
	to, err := ParseRcptTo(arg)
	if err != nil {
		s.send(reply(CodeParamError, "Syntax error in RCPT TO argument"))
		return
	}
	s.tx.addRecipient(to)
	s.state = StateRcpt
	s.send(reply(CodeOK, "OK"))
}

func (s *Session) handleData() {
	if s.state != StateRcpt {
		s.send(reply(CodeBadSequence, "Bad sequence: need RCPT TO first"))
		return
	}
	s.send(reply(CodeStartMailInput, "Start mail input; end with <CRLF>.<CRLF>"))

	body, err := s.readData()
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			s.send(reply(CodeServiceNotAvail, "Timeout, closing connection"))
		}
		if errors.Is(err, errMessageTooBig) {
			s.send(reply(CodeMessageTooBig, "Message exceeds maximum size"))
			s.resetTx()
		}
		return
	}

	env := Envelope{
		From:     s.tx.From,
		To:       append([]string(nil), s.tx.To...),
		Data:     body,
		Received: time.Now(),
	}
	if _, err := s.server.Store.Save(env); err != nil {
		s.send(reply(CodeTransactionFail, "Transaction failed: could not store message"))
		s.resetTx()
		return
	}
	s.resetTx()
	s.send(reply(CodeOK, "OK: message accepted for delivery"))
}

func (s *Session) handleRset() {
	s.resetTx()
	s.send(reply(CodeOK, "OK"))
}

// resetTx abandons the current transaction but keeps the HELO identity, as
// RSET requires.
func (s *Session) resetTx() {
	s.tx.reset()
	if s.state != StateGreet {
		s.state = StateGreeted
	}
}

var errMessageTooBig = errors.New("message too big")

// readData reads the message body up to the lone "." line and undoes
// dot-stuffing.
func (s *Session) readData() ([]byte, error) {
	s.state = StateData
	var buf bytes.Buffer
	for {
		s.touchDeadline()
		line, err := s.readLine()
		if err != nil {
			return nil, err
		}
		if line == "." {
			break
		}
		line = strings.TrimPrefix(line, ".")
		buf.WriteString(line)
		buf.WriteString("\r\n")
		if buf.Len() > maxMessageBytes {
			// Ideally we would read the rest of the body to stay in sync with
			// the client; for now the transaction is just aborted.
			return nil, errMessageTooBig
		}
	}
	return buf.Bytes(), nil
}

// readLine reads one line and strips its CRLF or bare LF.
func (s *Session) readLine() (string, error) {
	var sb strings.Builder
	for {
		chunk, err := s.r.ReadString('\n')
		sb.WriteString(chunk)
		if sb.Len() > maxLineBytes {
			return "", errors.New("line too long")
		}
		if err != nil {
			if err == io.EOF && sb.Len() > 0 {
				// Return the partial line now; the next call gets the EOF.
				return trimEOL(sb.String()), nil
			}
			return "", err
		}
		if strings.HasSuffix(chunk, "\n") {
			return trimEOL(sb.String()), nil
		}
	}
}

func trimEOL(s string) string {
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")
	return s
}

func (s *Session) touchDeadline() {
	if s.timeout > 0 {
		_ = s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	}
}

func (s *Session) send(text string) {
	if s.timeout > 0 {
		_ = s.conn.SetWriteDeadline(time.Now().Add(s.timeout))
	}
	_, _ = s.w.WriteString(text)
	_ = s.w.Flush()
}
