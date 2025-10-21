// Command bittorrent downloads single-file torrents. It announces to an HTTP
// tracker and fetches pieces from several peers at once, checking each piece's
// SHA-1 hash.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"bittorrent/internal/download"
	"bittorrent/internal/metainfo"
	"bittorrent/internal/tracker"
)

const (
	exitOK         = 0
	exitUsage      = 1
	exitParse      = 2
	exitTracker    = 3
	exitIncomplete = 4
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitUsage
	}
	switch args[0] {
	case "info":
		return cmdInfo(args[1:])
	case "peers":
		return cmdPeers(args[1:])
	case "download":
		return cmdDownload(args[1:])
	case "-h", "--help", "help":
		usage()
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", args[0])
		usage()
		return exitUsage
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `bittorrent — single-file BitTorrent download client

Usage:
  bittorrent info     <path-to-.torrent>
  bittorrent peers     [--net] <path-to-.torrent>
  bittorrent download  [flags]   <path-to-.torrent>

Download flags:
  -o string        Output file path (default: info.name from torrent)
  -p int           Local port advertised to tracker (default 6881)
  --peers int      Max concurrent peer connections (default 30)
  --net            Enable network operations (tracker announce + live download)
  --timeout dur    Per-peer dial/read timeout (default 15s)
  -v               Verbose logging
`)
}

func newPeerID() [20]byte {
	var id [20]byte
	copy(id[:], "-GO0001-")
	_, _ = rand.Read(id[8:])
	return id
}

func loadTorrent(path string) (*metainfo.Torrent, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read torrent: %v\n", err)
		return nil, exitParse
	}
	t, err := metainfo.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return nil, exitParse
	}
	return t, exitOK
}

func cmdInfo(args []string) int {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: bittorrent info <path-to-.torrent>")
		return exitUsage
	}
	t, code := loadTorrent(fs.Arg(0))
	if code != exitOK {
		return code
	}
	fmt.Printf("Announce:     %s\n", t.Announce)
	fmt.Printf("Info Hash:    %x\n", t.InfoHash)
	fmt.Printf("Name:         %s\n", t.Name)
	fmt.Printf("Length:       %d bytes\n", t.Length)
	fmt.Printf("Piece Length: %d bytes\n", t.PieceLength)
	fmt.Printf("Pieces:       %d\n", t.NumPieces())
	if n := t.NumPieces(); n > 0 {
		fmt.Printf("Last Piece:   %d bytes\n", t.PieceSize(n-1))
	}
	return exitOK
}

func cmdPeers(args []string) int {
	fs := flag.NewFlagSet("peers", flag.ContinueOnError)
	net := fs.Bool("net", false, "enable network operations")
	port := fs.Int("p", 6881, "local port")
	timeout := fs.Duration("timeout", 15*time.Second, "tracker timeout")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: bittorrent peers [--net] <path-to-.torrent>")
		return exitUsage
	}
	if !*net {
		fmt.Fprintln(os.Stderr, "error: peers requires --net (network access disabled by default)")
		return exitUsage
	}
	t, code := loadTorrent(fs.Arg(0))
	if code != exitOK {
		return code
	}
	resp, err := tracker.Announce(t, newPeerID(), *port, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitTracker
	}
	fmt.Printf("Interval: %d\n", resp.Interval)
	fmt.Printf("Peers (%d):\n", len(resp.Peers))
	for _, p := range resp.Peers {
		fmt.Printf("  %s\n", p.Addr())
	}
	return exitOK
}

func cmdDownload(args []string) int {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	out := fs.String("o", "", "output file path")
	port := fs.Int("p", 6881, "local port")
	maxPeers := fs.Int("peers", 30, "max concurrent peers")
	net := fs.Bool("net", false, "enable network operations")
	timeout := fs.Duration("timeout", 15*time.Second, "per-peer timeout")
	verbose := fs.Bool("v", false, "verbose logging")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: bittorrent download [flags] <path-to-.torrent>")
		return exitUsage
	}
	if !*net {
		fmt.Fprintln(os.Stderr, "error: download requires --net (network access disabled by default)")
		return exitUsage
	}
	t, code := loadTorrent(fs.Arg(0))
	if code != exitOK {
		return code
	}

	peerID := newPeerID()
	resp, err := tracker.Announce(t, peerID, *port, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: tracker: %v\n", err)
		return exitTracker
	}
	if len(resp.Peers) == 0 {
		fmt.Fprintln(os.Stderr, "error: tracker returned no peers")
		return exitTracker
	}
	fmt.Printf("Tracker returned %d peers; downloading %q (%d bytes, %d pieces)\n",
		len(resp.Peers), t.Name, t.Length, t.NumPieces())

	outPath := *out
	if outPath == "" {
		outPath = filepath.Base(t.Name)
	}
	f, err := os.OpenFile(outPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open output: %v\n", err)
		return exitParse
	}
	defer f.Close()
	if err := f.Truncate(t.Length); err != nil {
		fmt.Fprintf(os.Stderr, "error: size output: %v\n", err)
		return exitParse
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	mgr := &download.Manager{
		Torrent:  t,
		PeerID:   peerID,
		Timeout:  *timeout,
		MaxPeers: *maxPeers,
		Verbose:  *verbose,
	}
	if err := mgr.Run(ctx, resp.Peers, f); err != nil {
		fmt.Fprintf(os.Stderr, "error: download: %v\n", err)
		return exitIncomplete
	}
	fmt.Printf("Done. Wrote %s (%d bytes)\n", outPath, t.Length)
	return exitOK
}
