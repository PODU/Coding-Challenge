// Command greetls is a language server for the toy ".greet" language. It
// speaks LSP over stdin/stdout and logs to stderr.
package main

import (
	"log"
	"os"

	"greetls/internal/server"
)

func main() {
	// stdout carries the protocol, so logs must never go there.
	log.SetOutput(os.Stderr)
	log.SetPrefix("greetls: ")
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	os.Exit(server.New(os.Stdin, os.Stdout).Run())
}
