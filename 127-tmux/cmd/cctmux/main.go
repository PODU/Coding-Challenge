// Command cctmux is a small tmux clone. The real client and server need Linux
// (PTYs and Unix sockets); `cctmux demo` and the tests use mock PTYs and run
// anywhere.
package main

import (
	"fmt"
	"os"

	"cctmux/internal/protocol"
)

const usage = `cctmux — build-your-own tmux (Coding Challenge #127)

Usage:
  cctmux [flags] [command]

Commands:
  new-session, new     Create a session and attach (flag: -s <name>)
  attach-session, a    Attach to an existing session (flag: -t <name>)
  list-sessions, ls    List running sessions
  kill-session         Kill a session (flag: -t <name>)
  demo                 Render a sample layout with mock shells (any OS)
  help                 Show this help
  version              Show version

Global flags:
  -S <path>   server socket path
  -f <path>   config file (default ~/.cctmux.conf); supports: set prefix C-a
  -p <key>    prefix key, e.g. C-a (default C-b)

Prefix keybindings (default C-b):
  d detach   c new-window   n/p next/prev   0-9 select window
  " split horizontal   % split vertical   arrows move   Ctrl+arrows resize
  z zoom   x kill-pane   [ copy-mode (q to exit)   : command prompt
`

// request is what platformRun should do once the command line is parsed.
type request struct {
	server bool
	verb   protocol.Type
	target string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cctmux:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg := defaultConfig()
	confPath := defaultConfigPath()
	var prefixFlag string

	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-S":
			i++
			if i < len(args) {
				cfg.Socket = args[i]
			}
		case "-f":
			i++
			if i < len(args) {
				confPath = args[i]
			}
		case "-p":
			i++
			if i < len(args) {
				prefixFlag = args[i]
			}
		default:
			rest = append(rest, args[i])
		}
	}

	if err := LoadConfigFile(confPath, &cfg); err != nil {
		return err
	}
	if prefixFlag != "" {
		p, err := ParsePrefix(prefixFlag)
		if err != nil {
			return err
		}
		cfg.Prefix = p
	}

	cmd := ""
	if len(rest) > 0 {
		cmd = rest[0]
		rest = rest[1:]
	}

	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version", "--version":
		fmt.Println("cctmux 1.0.0")
		return nil
	case "demo":
		return runDemo()
	case "__server":
		return platformRun(cfg, request{server: true})
	case "list-sessions", "ls":
		return platformRun(cfg, request{verb: protocol.ListSessions})
	case "kill-session":
		return platformRun(cfg, request{verb: protocol.KillSession, target: flagValue(rest, "-t")})
	case "attach-session", "attach", "a":
		return platformRun(cfg, request{verb: protocol.AttachSession, target: flagValue(rest, "-t")})
	case "", "new-session", "new":
		return platformRun(cfg, request{verb: protocol.NewSession, target: flagValue(rest, "-s")})
	default:
		return fmt.Errorf("unknown command %q (try `cctmux help`)", cmd)
	}
}

func flagValue(args []string, name string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func defaultConfigPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home + string(os.PathSeparator) + ".cctmux.conf"
	}
	return ".cctmux.conf"
}
