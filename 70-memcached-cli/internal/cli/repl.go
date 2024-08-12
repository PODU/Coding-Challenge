package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/codingchallenges/ccmc/internal/client"
)

// Reconnector opens a new connection when the REPL's connection drops.
type Reconnector func() (*client.Client, error)

// REPL reads commands from in and runs them until "quit", "exit" or EOF. A bad
// command just prints an error. If reconnect is nil a dropped connection is
// not reopened.
func REPL(c *client.Client, g Globals, in io.Reader, out, errw io.Writer, reconnect Reconnector) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	fmt.Fprintln(out, "ccmc interactive mode. Type 'help' for commands, 'quit' to exit.")
	prompt(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			prompt(out)
			continue
		}
		switch strings.ToLower(line) {
		case "quit", "exit":
			return nil
		case "help", "?":
			fmt.Fprint(out, ReplHelp())
			prompt(out)
			continue
		}

		fields := strings.Fields(line)
		cmd, err := ParseCommand(fields, g, nil)
		if err != nil {
			fmt.Fprintf(errw, "%v\n", err)
			prompt(out)
			continue
		}

		c = execWithReconnect(c, cmd, out, errw, reconnect)
		prompt(out)
	}
	return scanner.Err()
}

// execWithReconnect runs cmd. If the connection turns out to be dead it
// reconnects and tries once more. It returns the client to use from now on.
func execWithReconnect(c *client.Client, cmd *Command, out, errw io.Writer, reconnect Reconnector) *client.Client {
	collector := &errCapture{Writer: errw}
	code := Execute(c, cmd, out, collector)
	if code == ExitProtocol && reconnect != nil && collector.looksLikeConnLoss() {
		fmt.Fprintln(errw, "connection lost; reconnecting...")
		_ = c.Close()
		nc, err := reconnect()
		if err != nil {
			fmt.Fprintf(errw, "reconnect failed: %v\n", err)
			return c
		}
		c = nc
		Execute(c, cmd, out, errw)
	}
	return c
}

func prompt(out io.Writer) { fmt.Fprint(out, "mc> ") }

// errCapture passes writes through but keeps a copy, so we can guess from the
// error text whether the connection was lost.
type errCapture struct {
	io.Writer
	buf strings.Builder
}

func (e *errCapture) Write(p []byte) (int, error) {
	e.buf.Write(p)
	return e.Writer.Write(p)
}

func (e *errCapture) looksLikeConnLoss() bool {
	s := e.buf.String()
	for _, marker := range []string{"EOF", "broken pipe", "reset", "closed", "refused"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}
