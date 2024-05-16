// Command dropbox is the sync client: it watches a local folder and keeps it in
// two-way sync with a dropd server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"dropbox/internal/atomic"
	"dropbox/internal/client"
	"dropbox/internal/sync"
)

// config is the client configuration persisted at <root>/.dropbox/config.json
// (or a path given with --config).
type config struct {
	Server   string   `json:"server"`
	Token    string   `json:"token"`
	Root     string   `json:"root"`
	Interval string   `json:"interval"`
	Ignore   []string `json:"ignore"`
}

var defaultIgnore = []string{".dropbox/", "*.tmp", ".DS_Store", "Thumbs.db"}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "init":
		cmdInit(args)
	case "login":
		cmdLogin(args)
	case "status":
		cmdSync(args, true)
	case "sync":
		cmdSync(args, false)
	case "watch":
		cmdWatch(args)
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: dropbox <command> [flags]

Commands:
  init    --root <dir> --server <url>     create <root>/.dropbox and config
  login   --server <url> --user <u> --password-stdin
  status  show the plan without acting
  sync    run one diff + transfer pass, then exit
  watch   --interval <dur>                daemon: poll + sync on change

Global flags: --config --server --token --root -v/--verbose --dry-run
Exit codes: 0 ok  1 error  2 conflicts written  3 auth failure`)
}

type globalFlags struct {
	fs      *flag.FlagSet
	config  *string
	server  *string
	token   *string
	root    *string
	verbose *bool
	dryRun  *bool
}

func newGlobal(name string) *globalFlags {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	g := &globalFlags{
		fs:      fs,
		config:  fs.String("config", "", "config file (default <root>/.dropbox/config.json)"),
		server:  fs.String("server", "", "server base URL (env DROPBOX_SERVER)"),
		token:   fs.String("token", "", "bearer token (env DROPBOX_TOKEN)"),
		root:    fs.String("root", "", "local sync root folder"),
		verbose: fs.Bool("verbose", false, "verbose logging"),
		dryRun:  fs.Bool("dry-run", false, "print the plan; perform no transfers"),
	}
	fs.BoolVar(g.verbose, "v", false, "verbose logging")
	return g
}

func (g *globalFlags) configPath() string {
	if *g.config != "" {
		return *g.config
	}
	root := *g.root
	if root == "" {
		root = "."
	}
	return filepath.Join(root, ".dropbox", "config.json")
}

// resolve merges the config file with flags and environment, which win.
func (g *globalFlags) resolve() config {
	cfg, _ := loadConfig(g.configPath())
	if *g.server != "" {
		cfg.Server = *g.server
	} else if v := os.Getenv("DROPBOX_SERVER"); v != "" && cfg.Server == "" {
		cfg.Server = v
	}
	if *g.token != "" {
		cfg.Token = *g.token
	} else if v := os.Getenv("DROPBOX_TOKEN"); v != "" && cfg.Token == "" {
		cfg.Token = v
	}
	if *g.root != "" {
		cfg.Root = *g.root
	}
	if cfg.Root == "" {
		cfg.Root = "."
	}
	if len(cfg.Ignore) == 0 {
		cfg.Ignore = defaultIgnore
	}
	return cfg
}

func loadConfig(path string) (config, error) {
	var cfg config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if len(b) > 0 {
		err = json.Unmarshal(b, &cfg)
	}
	return cfg, err
}

func saveConfig(path string, cfg config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return atomic.WriteBytes(path, 0o600, b)
}

func cmdInit(args []string) {
	g := newGlobal("init")
	_ = g.fs.Parse(args)
	root := *g.root
	if root == "" {
		log.Fatal("init: --root is required")
	}
	abs, _ := filepath.Abs(root)
	if err := os.MkdirAll(filepath.Join(abs, ".dropbox"), 0o755); err != nil {
		log.Fatalf("init: %v", err)
	}
	cfg := config{
		Server:   *g.server,
		Token:    *g.token,
		Root:     abs,
		Interval: "15s",
		Ignore:   defaultIgnore,
	}
	path := filepath.Join(abs, ".dropbox", "config.json")
	if err := saveConfig(path, cfg); err != nil {
		log.Fatalf("init: %v", err)
	}
	fmt.Printf("initialised sync root %s\nconfig: %s\n", abs, path)
	if cfg.Server == "" || cfg.Token == "" {
		fmt.Println("next: dropbox login --server <url> --user <u> --password-stdin")
	}
}

func cmdLogin(args []string) {
	g := newGlobal("login")
	user := g.fs.String("user", "", "user name")
	pwStdin := g.fs.Bool("password-stdin", false, "read password from stdin")
	_ = g.fs.Parse(args)
	cfg := g.resolve()
	if cfg.Server == "" || *user == "" {
		log.Fatal("login: --server and --user are required")
	}
	var password string
	if *pwStdin {
		b, _ := io.ReadAll(os.Stdin)
		password = strings.TrimRight(string(b), "\r\n")
	} else {
		fmt.Print("password: ")
		fmt.Scanln(&password)
	}
	c := client.New(cfg.Server, "", nil)
	tok, err := c.Login(*user, password)
	if err != nil {
		log.Printf("login failed: %v", err)
		os.Exit(3)
	}
	cfg.Token = tok
	if err := saveConfig(g.configPath(), cfg); err != nil {
		log.Fatalf("login: %v", err)
	}
	fmt.Println("logged in; token stored")
}

func newSyncer(g *globalFlags, cfg config) *client.Syncer {
	c := client.New(cfg.Server, cfg.Token, nil)
	return &client.Syncer{
		Root:   cfg.Root,
		Ignore: cfg.Ignore,
		Client: c,
		Now:    time.Now,
		DryRun: *g.dryRun,
		Logf: func(f string, a ...any) {
			if *g.verbose {
				log.Printf(f, a...)
			}
		},
	}
}

func cmdSync(args []string, statusOnly bool) {
	name := "sync"
	if statusOnly {
		name = "status"
	}
	g := newGlobal(name)
	_ = g.fs.Parse(args)
	cfg := g.resolve()
	requireServer(cfg)
	s := newSyncer(g, cfg)
	if statusOnly {
		s.DryRun = true
	}
	res, err := s.SyncOnce()
	if err != nil {
		log.Printf("%s: %v", name, err)
		os.Exit(1)
	}
	printPlan(res, statusOnly)
	if res.Conflicts > 0 {
		os.Exit(2)
	}
}

func cmdWatch(args []string) {
	g := newGlobal("watch")
	interval := g.fs.String("interval", "", "poll interval (default 15s)")
	_ = g.fs.Parse(args)
	cfg := g.resolve()
	requireServer(cfg)
	d := 15 * time.Second
	src := *interval
	if src == "" {
		src = cfg.Interval
	}
	if src != "" {
		if v, err := time.ParseDuration(src); err == nil {
			d = v
		}
	}
	s := newSyncer(g, cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	log.Printf("watching %s every %s", cfg.Root, d)
	if err := s.Watch(ctx, d); err != nil && err != context.Canceled {
		log.Printf("watch: %v", err)
		os.Exit(1)
	}
}

func requireServer(cfg config) {
	if cfg.Server == "" {
		log.Fatal("no server configured (use --server or dropbox init)")
	}
	if cfg.Token == "" {
		log.Fatal("no token configured (run: dropbox login ...)")
	}
}

func printPlan(res client.Result, statusOnly bool) {
	if len(res.Actions) == 0 {
		fmt.Println("up to date")
		return
	}
	verb := "did"
	if statusOnly {
		verb = "would"
	}
	for _, a := range res.Actions {
		switch a.Kind {
		case sync.Conflict:
			fmt.Printf("  conflict %s -> %s\n", a.Path, a.ConflictPath)
		default:
			fmt.Printf("  %-13s %s\n", a.Kind, a.Path)
		}
	}
	fmt.Printf("%s: %d up, %d down, %d del-local, %d del-remote, %d conflicts\n",
		verb, res.Uploaded, res.Downloaded, res.DeletedLocal, res.DeletedRemote, res.Conflicts)
}
