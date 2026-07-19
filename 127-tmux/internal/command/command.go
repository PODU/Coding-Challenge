// Package command runs tmux-style commands such as split-window or
// rename-window, as typed at the prefix+: prompt.
package command

import (
	"fmt"
	"strconv"
	"strings"

	"cctmux/internal/layout"
	"cctmux/internal/session"
)

type Result struct {
	Output string
	// KillSession names a session to kill. The caller does it, so that it can
	// also deal with the attached clients.
	KillSession string
	Detach      bool
}

// Exec runs one command line. cur is the client's session; it may be nil, but
// then only commands that don't need a session will work.
func Exec(m *session.Manager, cur *session.Session, line string) (Result, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Result{}, nil
	}
	verb, args := fields[0], fields[1:]
	fl := parseFlags(args)

	switch verb {
	case "new-window", "neww":
		if _, err := cur.NewWindow(fl.str("n", fl.pos(0))); err != nil {
			return Result{}, err
		}
		return Result{}, nil

	case "split-window", "splitw":
		o := layout.Horizontal
		if fl.has("h") { // -h means side by side, as in tmux
			o = layout.Vertical
		}
		if _, err := cur.SplitActive(o); err != nil {
			return Result{}, err
		}
		return Result{}, nil

	case "select-window", "selectw":
		n, err := strconv.Atoi(fl.str("t", fl.pos(0)))
		if err != nil {
			return Result{}, fmt.Errorf("select-window: need a window index")
		}
		if !cur.SelectWindow(n) {
			return Result{}, fmt.Errorf("no such window: %d", n)
		}
		return Result{}, nil

	case "next-window", "next":
		cur.NextWindow()
		return Result{}, nil

	case "previous-window", "prev":
		cur.PrevWindow()
		return Result{}, nil

	case "rename-window", "renamew":
		name := fl.str("n", fl.pos(0))
		if name == "" {
			return Result{}, fmt.Errorf("rename-window: need a name")
		}
		cur.RenameWindow(name)
		return Result{}, nil

	case "rename-session", "rename":
		name := fl.str("s", fl.pos(0))
		if name == "" {
			return Result{}, fmt.Errorf("rename-session: need a name")
		}
		if !m.Rename(cur.Name, name) {
			return Result{}, fmt.Errorf("cannot rename to %q", name)
		}
		return Result{}, nil

	case "kill-window", "killw", "kill-pane", "killp":
		if !cur.CloseActivePane() {
			return Result{KillSession: cur.Name}, nil
		}
		return Result{}, nil

	case "kill-session", "kills":
		target := fl.str("t", "")
		if target == "" && cur != nil {
			target = cur.Name
		}
		if target == "" || m.Get(target) == nil {
			return Result{}, fmt.Errorf("no such session: %q", target)
		}
		return Result{KillSession: target}, nil

	case "list-sessions", "ls":
		return Result{Output: m.FormatList()}, nil

	case "resize-pane", "resizep":
		d, ok := resizeDir(fl)
		if !ok {
			return Result{}, fmt.Errorf("resize-pane: need -U/-D/-L/-R")
		}
		amt := 1
		if v := fl.pos(0); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				amt = n
			}
		}
		cur.ResizePane(d, amt)
		return Result{}, nil

	case "select-pane", "selectp":
		switch {
		case fl.has("U"):
			cur.Navigate(layout.Up)
		case fl.has("D"):
			cur.Navigate(layout.Down)
		case fl.has("L"):
			cur.Navigate(layout.Left)
		case fl.has("R"):
			cur.Navigate(layout.Right)
		default:
			return Result{}, fmt.Errorf("select-pane: need a direction")
		}
		return Result{}, nil

	case "resize-window", "zoom", "resize-pane-zoom":
		cur.ToggleZoom()
		return Result{}, nil

	case "detach", "detach-client":
		return Result{Detach: true}, nil

	default:
		return Result{}, fmt.Errorf("unknown command: %s", verb)
	}
}

func resizeDir(fl flags) (layout.Direction, bool) {
	switch {
	case fl.has("U"):
		return layout.Up, true
	case fl.has("D"):
		return layout.Down, true
	case fl.has("L"):
		return layout.Left, true
	case fl.has("R"):
		return layout.Right, true
	}
	return 0, false
}

type flags struct {
	bools      map[string]bool
	values     map[string]string
	positional []string
}

// parseFlags splits args into flags and positional arguments. -t, -n and -s
// take a value; any other flag is a boolean.
func parseFlags(args []string) flags {
	f := flags{bools: map[string]bool{}, values: map[string]string{}}
	valueFlags := map[string]bool{"t": true, "n": true, "s": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			name := strings.TrimPrefix(a, "-")
			if valueFlags[name] && i+1 < len(args) {
				f.values[name] = args[i+1]
				i++
			} else {
				f.bools[name] = true
			}
			continue
		}
		f.positional = append(f.positional, a)
	}
	return f
}

func (f flags) has(name string) bool { return f.bools[name] }

func (f flags) str(name, def string) string {
	if v, ok := f.values[name]; ok {
		return v
	}
	return def
}

func (f flags) pos(i int) string {
	if i < len(f.positional) {
		return f.positional[i]
	}
	return ""
}
