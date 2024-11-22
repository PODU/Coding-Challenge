// Command ccsg is a small static site generator.
// Usage: ccsg <command> [args].
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"ccsg/internal/builder"
	"ccsg/internal/scaffold"
	"ccsg/internal/server"
)

// cliError is an error with the exit code to use for it.
type cliError struct {
	code int
	msg  string
}

func (e *cliError) Error() string { return e.msg }

func usageErr(format string, a ...any) error {
	return &cliError{code: 1, msg: fmt.Sprintf(format, a...)}
}

func runtimeErr(err error) error {
	return &cliError{code: 2, msg: err.Error()}
}

func main() {
	err := run(os.Args[1:])
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "ccsg: "+err.Error())
	code := 1
	var ce *cliError
	if errors.As(err, &ce) {
		code = ce.code
	}
	os.Exit(code)
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return &cliError{code: 1, msg: "no command given"}
	}

	switch args[0] {
	case "new":
		return cmdNew(args[1:])
	case "build":
		return cmdBuild()
	case "server", "serve":
		return cmdServer(args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		// `ccsg <name>` is short for `ccsg new site <name>`.
		return cmdNewSite(args[0])
	}
}

func cmdNew(args []string) error {
	if len(args) < 2 {
		return usageErr("usage: ccsg new <site|theme|page> <name>")
	}
	kind, name := args[0], args[1]
	switch kind {
	case "site":
		return cmdNewSite(name)
	case "theme":
		root, err := scaffold.FindSiteRoot(".")
		if err != nil {
			return runtimeErr(err)
		}
		if err := scaffold.NewTheme(root, name); err != nil {
			return scaffoldErr(err)
		}
		fmt.Printf("created theme: themes/%s/index.html\n", name)
		return nil
	case "page":
		root, err := scaffold.FindSiteRoot(".")
		if err != nil {
			return runtimeErr(err)
		}
		path, err := scaffold.NewPage(root, name)
		if err != nil {
			return scaffoldErr(err)
		}
		fmt.Printf("created page: %s\n", path)
		return nil
	default:
		return usageErr("unknown 'new' target %q (want site, theme, or page)", kind)
	}
}

func cmdNewSite(name string) error {
	if err := scaffold.NewSite(name); err != nil {
		return scaffoldErr(err)
	}
	fmt.Printf("created site: %s\n", name)
	fmt.Printf("  next: cd %s && ccsg build\n", name)
	return nil
}

func cmdBuild() error {
	root, err := scaffold.FindSiteRoot(".")
	if err != nil {
		return runtimeErr(err)
	}
	res, err := builder.Build(root)
	if err != nil {
		return runtimeErr(err)
	}
	if res.MissingContentTag {
		fmt.Fprintln(os.Stderr, "ccsg: warning: theme template has no {{ Content }} tag")
	}
	for _, p := range res.Pages {
		fmt.Println("built: " + p)
	}
	fmt.Printf("built %d page(s) using theme '%s'\n", len(res.Pages), res.Theme)
	return nil
}

func cmdServer(args []string) error {
	port := 8080
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			if i+1 >= len(args) {
				return usageErr("--port requires a value")
			}
			i++
			p, perr := strconv.Atoi(args[i])
			if perr != nil {
				return usageErr("invalid port %q", args[i])
			}
			port = p
		default:
			return usageErr("unknown server flag %q", args[i])
		}
	}

	root, err := scaffold.FindSiteRoot(".")
	if err != nil {
		return runtimeErr(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv := server.New(root)
	if err := srv.ListenAndServe(ctx, port); err != nil {
		return runtimeErr(err)
	}
	return nil
}

// scaffoldErr treats "already exists" as a usage error (exit 1) and anything
// else as a runtime error (exit 2).
func scaffoldErr(err error) error {
	if errors.Is(err, scaffold.ErrExists) {
		return usageErr("%s", err)
	}
	return runtimeErr(err)
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `ccsg - a small static site generator

Usage:
  ccsg new site <name>     Scaffold a new site (placeholder homepage + default theme)
  ccsg new theme <name>    Create themes/<name>/index.html (run inside a site)
  ccsg new page <name>     Create content/<name>.md (run inside a site)
  ccsg build               Render content/**.md -> public/**.html using the active theme
  ccsg server [--port N]    Build, serve public/ (default :8080), live-reload on changes
  ccsg help                Show this help

  ccsg <name>              Alias for 'ccsg new site <name>'
`)
}
