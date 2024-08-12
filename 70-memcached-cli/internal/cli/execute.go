package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/codingchallenges/ccmc/internal/client"
	"github.com/codingchallenges/ccmc/internal/protocol"
)

// Exit codes.
const (
	ExitOK       = 0 // success / stored / value found
	ExitMiss     = 1 // logical miss: not found / NOT_STORED / EXISTS
	ExitUsage    = 2 // usage error
	ExitProtocol = 3 // connection or protocol error
)

// Execute runs cmd and returns the exit code. Results go to out, errors and
// misses to errw.
func Execute(c *client.Client, cmd *Command, out, errw io.Writer) int {
	switch cmd.Name {
	case "set", "add", "replace", "append", "prepend", "cas":
		return execStore(c, cmd, out, errw)
	case "get", "gets":
		return execGet(c, cmd, out, errw)
	case "delete":
		return execDelete(c, cmd, out, errw)
	case "incr", "decr":
		return execIncrDecr(c, cmd, out, errw)
	case "stats":
		return execStats(c, cmd, out, errw)
	default:
		fmt.Fprintf(errw, "unknown command %q\n", cmd.Name)
		return ExitUsage
	}
}

func execStore(c *client.Client, cmd *Command, out, errw io.Writer) int {
	status, err := c.Store(cmd.StorageCommand())
	if err != nil {
		return reportErr(errw, err)
	}
	fmt.Fprintln(out, FormatStore(status))
	switch status {
	case protocol.NotStored, protocol.Exists, protocol.NotFound:
		return ExitMiss
	default:
		return ExitOK
	}
}

func execGet(c *client.Client, cmd *Command, out, errw io.Writer) int {
	var (
		vals []protocol.Value
		err  error
	)
	if cmd.Name == "gets" {
		vals, err = c.Gets(cmd.Keys)
	} else {
		vals, err = c.Get(cmd.Keys)
	}
	if err != nil {
		return reportErr(errw, err)
	}
	res := FormatGet(cmd.Keys, vals, cmd.Quiet)
	if res.Stdout != "" {
		fmt.Fprintln(out, res.Stdout)
	}
	for _, k := range res.Misses {
		fmt.Fprintf(errw, "%s: (not found)\n", k)
	}
	if len(res.Misses) > 0 {
		return ExitMiss
	}
	return ExitOK
}

func execDelete(c *client.Client, cmd *Command, out, errw io.Writer) int {
	deleted, err := c.Delete(cmd.Keys[0], cmd.NoReply)
	if err != nil {
		return reportErr(errw, err)
	}
	if cmd.NoReply {
		fmt.Fprintln(out, "(noreply)")
		return ExitOK
	}
	fmt.Fprintln(out, FormatDelete(deleted))
	if !deleted {
		return ExitMiss
	}
	return ExitOK
}

func execIncrDecr(c *client.Client, cmd *Command, out, errw io.Writer) int {
	value, found, err := c.IncrDecr(cmd.Name, cmd.Keys[0], cmd.Delta, cmd.NoReply)
	if err != nil {
		return reportErr(errw, err)
	}
	if cmd.NoReply {
		fmt.Fprintln(out, "(noreply)")
		return ExitOK
	}
	fmt.Fprintln(out, FormatNumeric(value, found))
	if !found {
		return ExitMiss
	}
	return ExitOK
}

func execStats(c *client.Client, cmd *Command, out, errw io.Writer) int {
	arg := ""
	if len(cmd.Keys) == 1 {
		arg = cmd.Keys[0]
	}
	stats, err := c.Stats(arg)
	if err != nil {
		return reportErr(errw, err)
	}
	fmt.Fprintln(out, FormatStats(stats))
	return ExitOK
}

// reportErr prints err. Server errors and I/O errors both exit with
// ExitProtocol.
func reportErr(errw io.Writer, err error) int {
	var perr *protocol.ProtocolError
	if errors.As(err, &perr) {
		fmt.Fprintf(errw, "server error: %s\n", perr.Error())
	} else {
		fmt.Fprintf(errw, "error: %v\n", err)
	}
	return ExitProtocol
}
