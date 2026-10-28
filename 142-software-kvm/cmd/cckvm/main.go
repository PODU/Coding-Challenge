// Command cckvm is a software KVM that speaks the Barrier protocol: run the
// server on the machine with the keyboard and mouse, and the client (or a
// genuine Barrier client) on every other machine.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/podu2997/cckvm/internal/client"
	"github.com/podu2997/cckvm/internal/layout"
	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/server"
)

const defaultPort = "24800"

const usage = `usage:
  cckvm server -c LAYOUT [-n NAME] [-a ADDR] [--relative] [--tls] [--backend native|log] [-v]
  cckvm client [-n NAME] [--tls --fingerprint SHA256:..] [--no-reconnect] [--backend native|log] [-v] HOST[:PORT]
  cckvm fingerprint [--cert FILE]

NAME defaults to this machine's host name. Run "cckvm server -h" or
"cckvm client -h" for every option.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch os.Args[1] {
	case "server":
		err = runServer(ctx, os.Args[2:])
	case "client":
		err = runClient(ctx, os.Args[2:])
	case "fingerprint":
		err = runFingerprint(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "cckvm: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "cckvm:", err)
		os.Exit(1)
	}
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return strings.SplitN(h, ".", 2)[0]
}

func newLogger(verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// openScreen returns the local desktop backend.
func openScreen(backend string, capture bool, size string) (platform.Screen, error) {
	switch backend {
	case "native":
		return platform.Native(platform.NativeOptions{Capture: capture})
	case "log":
		w, h, err := parseSize(size)
		if err != nil {
			return nil, err
		}
		return platform.NewLog(os.Stdout, platform.Rect{W: w, H: h}), nil
	}
	return nil, fmt.Errorf("unknown backend %q (want native or log)", backend)
}

func parseSize(s string) (int, int, error) {
	ws, hs, ok := strings.Cut(strings.ToLower(s), "x")
	w, err1 := strconv.Atoi(ws)
	h, err2 := strconv.Atoi(hs)
	if !ok || err1 != nil || err2 != nil || w < 3 || h < 3 || w > 32767 || h > 32767 {
		return 0, 0, fmt.Errorf("bad size %q (want WIDTHxHEIGHT)", s)
	}
	return w, h, nil
}

func runServer(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	cfgPath := fs.String("c", "", "layout configuration file (required)")
	name := fs.String("n", hostname(), "this machine's screen name in the layout")
	addr := fs.String("a", ":"+defaultPort, "address to listen on")
	relative := fs.Bool("relative", false, "send relative mouse moves (DMRM); also set by the relativeMouseMoves option")
	useTLS := fs.Bool("tls", false, "encrypt connections (clients must enable TLS too)")
	certPath := fs.String("cert", "", "TLS certificate+key PEM file (default: generated in the user config dir)")
	backend := fs.String("backend", "native", "native (real input) or log (protocol only)")
	size := fs.String("size", "1920x1080", "screen size for the log backend")
	verbose := fs.Bool("v", false, "debug logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" || fs.NArg() != 0 {
		fs.Usage()
		return errors.New("server needs -c LAYOUT and no other arguments")
	}
	log := newLogger(*verbose)
	lay, err := layout.Load(*cfgPath)
	if err != nil {
		return err
	}
	scr, err := openScreen(*backend, true, *size)
	if err != nil {
		return err
	}
	defer scr.Close()

	srv, err := server.New(server.Config{
		Layout:   lay,
		Name:     *name,
		Screen:   scr,
		Relative: *relative || lay.Bool("relativeMouseMoves", false),
		Logger:   log,
	})
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	if *useTLS {
		cfg, fp, err := serverTLS(*certPath)
		if err != nil {
			ln.Close()
			return err
		}
		ln = tlsListener(ln, cfg)
		log.Info("TLS enabled", "fingerprint", fp)
	}
	log.Info("listening", "addr", ln.Addr().String(), "screen", *name, "screens", lay.Screens())
	return srv.Serve(ctx, ln)
}

func runClient(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	name := fs.String("n", hostname(), "this screen's name in the server's layout")
	useTLS := fs.Bool("tls", false, "connect with TLS")
	fingerprint := fs.String("fingerprint", "", "expected server certificate SHA-256 fingerprint (required with --tls)")
	noReconnect := fs.Bool("no-reconnect", false, "exit when the connection drops instead of retrying")
	backend := fs.String("backend", "native", "native (inject input) or log (print it)")
	size := fs.String("size", "1920x1080", "screen size for the log backend")
	verbose := fs.Bool("v", false, "debug logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("client needs exactly one server address")
	}
	addr := fs.Arg(0)
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, defaultPort)
	}
	log := newLogger(*verbose)
	scr, err := openScreen(*backend, false, *size)
	if err != nil {
		return err
	}
	defer scr.Close()

	cfg := client.Config{
		Name:      *name,
		Addr:      addr,
		Screen:    scr,
		Reconnect: !*noReconnect,
		Logger:    log,
	}
	if *useTLS {
		dial, err := clientTLS(*fingerprint)
		if err != nil {
			return err
		}
		cfg.Dial = dial
	}
	log.Info("connecting", "server", addr, "screen", *name)
	return client.Run(ctx, cfg)
}

func runFingerprint(args []string) error {
	fs := flag.NewFlagSet("fingerprint", flag.ContinueOnError)
	certPath := fs.String("cert", "", "certificate PEM file (default: the server's generated one)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, fp, err := serverTLS(*certPath)
	if err != nil {
		return err
	}
	fmt.Println(fp)
	return nil
}
