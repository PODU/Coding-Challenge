// Command ircclient is a small terminal IRC client: connect (optionally over
// TLS), register, join and part channels, change nick, chat, and quit. It runs
// as a full-screen tcell UI, or with -plain as a simple line-based one.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"ircclient/internal/client"
	"ircclient/internal/conn"
	"ircclient/internal/parser"
	"ircclient/internal/ui"
)

func main() {
	var (
		server  = flag.String("server", "irc.libera.chat", "IRC server host")
		port    = flag.Int("port", 6667, "IRC server port")
		useTLS  = flag.Bool("tls", false, "use TLS (implies -port 6697 if unset)")
		nick    = flag.String("nick", "CCClient", "nickname")
		user    = flag.String("user", "guest", "username for USER command")
		real    = flag.String("real", "Coding Challenges Client", "real name")
		channel = flag.String("channel", "", "channel to auto-join after registration")
		plain   = flag.Bool("plain", false, "headless line mode instead of full TUI")
		debug   = flag.Bool("debug", false, "show raw protocol lines (incl. PING/PONG)")
	)
	flag.Parse()

	if *useTLS && !portFlagSet() {
		*port = 6697
	}
	addr := net.JoinHostPort(*server, strconv.Itoa(*port))

	cfg := client.Config{
		Nick:    *nick,
		User:    *user,
		Real:    *real,
		Channel: *channel,
		Debug:   *debug,
	}

	if err := run(addr, *useTLS, *plain, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "ircclient:", err)
		os.Exit(1)
	}
}

func portFlagSet() bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			set = true
		}
	})
	return set
}

func run(addr string, useTLS, plain bool, cfg client.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var front ui.UI
	if plain {
		front = ui.NewPlain(os.Stdin, os.Stdout)
	} else {
		front = ui.NewTcell()
	}
	if err := front.Start(); err != nil {
		return fmt.Errorf("starting UI: %w", err)
	}
	defer front.Stop()

	front.Display(client.Event{Kind: client.KindInfo, Text: "* Connecting to " + addr + " ..."})

	c, err := conn.Dial(ctx, addr, useTLS)
	if err != nil {
		front.Display(client.Event{Kind: client.KindError, Text: "! connect failed: " + err.Error()})
		return err
	}
	defer c.Close()

	cl := client.New(cfg, c)
	front.SetStatus(cl.Nick(), cl.Channel())
	if err := cl.Register(); err != nil {
		return fmt.Errorf("registering: %w", err)
	}

	// Send a proper QUIT on Ctrl-C or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-sigCh:
			_, _ = cl.HandleInput("/quit")
			return nil

		case <-front.Quit():
			_, _ = cl.HandleInput("/quit")
			return nil

		case line, ok := <-c.Lines():
			if !ok {
				front.Display(client.Event{Kind: client.KindError, Text: "! disconnected from server"})
				return nil
			}
			if cfg.Debug {
				front.Display(client.Event{Kind: client.KindRaw, Text: "<< " + line})
			}
			m, perr := parser.Parse(line)
			if perr != nil {
				continue
			}
			if dispatch(front, cl.Handle(m)) {
				return nil
			}
			front.SetStatus(cl.Nick(), cl.Channel())

		case err := <-c.Errs():
			front.Display(client.Event{Kind: client.KindError, Text: "! connection error: " + err.Error()})
			return nil

		case in, ok := <-front.Lines():
			if !ok {
				return nil
			}
			evs, herr := cl.HandleInput(in)
			if herr != nil {
				front.Display(client.Event{Kind: client.KindError, Text: "! " + herr.Error()})
				continue
			}
			if dispatch(front, evs) {
				return nil
			}
			front.SetStatus(cl.Nick(), cl.Channel())
		}
	}
}

// dispatch displays evs and reports whether one of them was a shutdown.
func dispatch(front ui.UI, evs []client.Event) bool {
	shutdown := false
	for _, e := range evs {
		if e.Kind == client.KindShutdown {
			shutdown = true
			continue
		}
		front.Display(e)
	}
	return shutdown
}
