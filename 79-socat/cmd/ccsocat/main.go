// Command ccsocat is a small socat: it opens two addresses and copies bytes
// between them.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"ccsocat/internal/addr"
	"ccsocat/internal/endpoint"
	"ccsocat/internal/relay"
)

const usage = `ccsocat [options] <address1> <address2>

Options:
  -u            unidirectional: transfer address1 -> address2 only
  -U            reverse unidirectional: address2 -> address1 only
  -d            increase verbosity (repeatable) — diagnostics to stderr
  -h, --help    show this help

Addresses (Kind[:param[:param]][,opt[,opt]]):
  -  | STDIO                          stdin/stdout
  TCP[4|6]:host:port                  TCP connect (client)
  TCP[4|6]-LISTEN:port[,reuseaddr][,fork]
  UDP[4|6]:host:port                  UDP "connected" client
  UDP[4|6]-LISTEN:port
  FILE:path[,create][,append]         ( OPEN: is an alias )
  EXEC:'command args'                 spawn process, relay stdio
`

// counter is a repeatable boolean flag (-d -d -> 2).
type counter int

func (c *counter) String() string { return fmt.Sprintf("%d", *c) }
func (c *counter) Set(string) error {
	*c++
	return nil
}
func (c *counter) IsBoolFlag() bool { return true }

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("ccsocat", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	var uni, revUni bool
	var verbosity counter
	fs.BoolVar(&uni, "u", false, "unidirectional left->right")
	fs.BoolVar(&revUni, "U", false, "unidirectional right->left")
	fs.Var(&verbosity, "d", "increase verbosity")

	if err := fs.Parse(args); err != nil {
		// flag has already printed the error and usage.
		return 2
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if uni && revUni {
		fmt.Fprintln(os.Stderr, "ccsocat: -u and -U are mutually exclusive")
		return 2
	}

	specA, err := addr.Parse(rest[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccsocat: %v\n", err)
		return 2
	}
	specB, err := addr.Parse(rest[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccsocat: %v\n", err)
		return 2
	}

	mode := relay.Bidirectional
	if uni {
		mode = relay.UniLeftToRight
	} else if revUni {
		mode = relay.UniRightToLeft
	}

	log := func(level int, format string, a ...any) {
		if int(verbosity) >= level {
			fmt.Fprintf(os.Stderr, "ccsocat: "+format+"\n", a...)
		}
	}

	if err := serve(specA, specB, mode, log); err != nil {
		fmt.Fprintf(os.Stderr, "ccsocat: %v\n", err)
		return 1
	}
	return 0
}

type logf func(level int, format string, a ...any)

// serve connects the two addresses, accepting first if one of them listens,
// and relays until done or interrupted.
func serve(specA, specB *addr.AddressSpec, mode relay.Mode, log logf) error {
	aListen, bListen := specA.IsListener(), specB.IsListener()
	if aListen && bListen {
		return fmt.Errorf("both addresses are listeners; at most one may listen")
	}

	if !aListen && !bListen {
		return serveDirect(specA, specB, mode, log)
	}

	listenSpec, peerSpec := specA, specB
	listenIsLeft := true
	if bListen {
		listenSpec, peerSpec = specB, specA
		listenIsLeft = false
	}

	ln, err := endpoint.Listen(listenSpec)
	if err != nil {
		return err
	}
	defer ln.Close()
	log(1, "listening on %s", ln.Addr())

	// Closing the listener on a signal unblocks Accept so we can exit.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log(1, "signal received, shutting down")
		ln.Close()
	}()

	fork := listenSpec.HasOption("fork")
	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			break // listener closed
		}
		log(1, "accepted connection")

		handle := func(c endpoint.Endpoint) error {
			peer, err := endpoint.Open(peerSpec, peerRole(listenIsLeft))
			if err != nil {
				c.Close()
				return err
			}
			left, right := orient(c, peer, listenIsLeft)
			return relay.Relay(left, right, mode)
		}

		if fork {
			wg.Add(1)
			go func(c endpoint.Endpoint) {
				defer wg.Done()
				if err := handle(c); err != nil {
					log(0, "handler error: %v", err)
				}
			}(conn)
			continue
		}

		// Without fork we serve one connection and stop.
		err = handle(conn)
		ln.Close()
		wg.Wait()
		return err
	}
	wg.Wait()
	return nil
}

func serveDirect(specA, specB *addr.AddressSpec, mode relay.Mode, log logf) error {
	left, err := endpoint.Open(specA, endpoint.RoleLeft)
	if err != nil {
		return err
	}
	right, err := endpoint.Open(specB, endpoint.RoleRight)
	if err != nil {
		left.Close()
		return err
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- relay.Relay(left, right, mode) }()

	select {
	case err := <-done:
		return err
	case <-stop:
		log(1, "signal received, closing endpoints")
		left.Close()
		right.Close()
		return <-done
	}
}

// orient puts the accepted conn and the peer back in command-line order, which
// -u and -U depend on.
func orient(conn, peer endpoint.Endpoint, listenIsLeft bool) (endpoint.Endpoint, endpoint.Endpoint) {
	if listenIsLeft {
		return conn, peer
	}
	return peer, conn
}

// peerRole is the side the non-listening address sits on. FILE uses it to
// pick read or write mode.
func peerRole(listenIsLeft bool) endpoint.Role {
	if listenIsLeft {
		return endpoint.RoleRight
	}
	return endpoint.RoleLeft
}
