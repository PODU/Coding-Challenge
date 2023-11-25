// Command redis-cli is a small clone of Redis's redis-cli. It runs one command
// given on the command line, or reads commands from stdin, interactively if
// stdin is a terminal.
//
// Usage:
//
//	redis-cli [-h <host>] [-p <port>] [COMMAND [ARG ...]]
package main

import (
	"flag"
	"fmt"
	"os"

	"rediscli/client"
	"rediscli/format"
	"rediscli/help"
	"rediscli/repl"
	"rediscli/resp"
)

func main() {
	host := flag.String("h", "127.0.0.1", "Server hostname")
	port := flag.String("p", "6379", "Server port")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-h host] [-p port] [command [arg ...]]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()

	// Help works without a server.
	if len(args) > 0 && isHelp(args[0]) {
		fmt.Println(renderArgvHelp(args[1:]))
		return
	}

	c, err := client.Dial(*host, *port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to Redis at %s:%s: %v\n", *host, *port, err)
		os.Exit(1)
	}
	defer c.Close()

	if len(args) > 0 {
		v, err := c.Send(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(format.Render(v))
		if v.Type == resp.Error {
			os.Exit(1)
		}
		return
	}

	os.Exit(repl.Run(os.Stdin, os.Stdout, c, isTerminal(os.Stdin)))
}

func isHelp(s string) bool { return s == "help" || s == "HELP" }

func renderArgvHelp(rest []string) string {
	if len(rest) == 0 {
		return help.Overview()
	}
	if rest[0][0] == '@' {
		if s, ok := help.GroupHelp(rest[0]); ok {
			return s
		}
		return "Unknown group: " + rest[0]
	}
	if s, ok := help.CommandHelp(rest[0]); ok {
		return s
	}
	return "Unknown command or group: " + rest[0]
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
