package ui

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"ccfzf/internal/cli"
	"ccfzf/internal/item"
)

// previewDelay keeps the preview command from being launched for every item
// the cursor passes over while a key is held down.
const previewDelay = 60 * time.Millisecond

type previewState struct {
	opts *cli.Options

	mu      sync.Mutex
	lines   []string
	shown   string // the command whose output `lines` holds
	pending string
	since   time.Time
	running string
	cancel  context.CancelFunc
	changed bool
}

func newPreviewState(opts *cli.Options) *previewState {
	return &previewState{opts: opts}
}

// request records the command the current item would need. It is called on
// every frame; the work only starts once the same command has been requested
// for previewDelay.
func (p *previewState) request(cmd string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cmd != p.pending {
		p.pending = cmd
		p.since = time.Now()
	}
}

// poll starts a due command and reports whether the pane needs redrawing.
func (p *previewState) poll() bool {
	p.mu.Lock()
	changed := p.changed
	p.changed = false
	cmd := p.pending
	due := time.Since(p.since) >= previewDelay
	start := due && cmd != p.running && cmd != p.shown
	if start {
		if p.cancel != nil {
			p.cancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		p.cancel = cancel
		p.running = cmd
		go p.run(ctx, cmd)
	}
	p.mu.Unlock()
	return changed
}

func (p *previewState) run(ctx context.Context, cmdline string) {
	var out []byte
	if cmdline != "" {
		name, args := shellCommand(cmdline)
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = os.Environ()
		out, _ = cmd.CombinedOutput()
	}
	if ctx.Err() != nil {
		return
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	p.mu.Lock()
	if p.running == cmdline {
		p.lines = lines
		p.shown = cmdline
		p.running = ""
		p.changed = true
	}
	p.mu.Unlock()
}

func (p *previewState) snapshot() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lines
}

func (p *previewState) stop() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.mu.Unlock()
}

func shellCommand(cmdline string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", cmdline}
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, []string{"-c", cmdline}
}

// quote makes a string safe to paste into a shell command line.
func quote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// previewCommand fills in the placeholders of --preview for the current state.
//
//	{}     the current item
//	{n}    its index in the input, or -1 when there is no current item
//	{q}    the query
//	{+}    every selected item, or the current one when nothing is marked
//	{1}... the nth field of the current item
func previewCommand(tmpl string, cur *item.Item, selected []string, query string, delim item.Delimiter) string {
	if cur == nil && !strings.Contains(tmpl, "{q}") {
		return ""
	}
	raw, index := "", "-1"
	var tokens []string
	if cur != nil {
		raw = cur.Raw
		index = strconv.Itoa(int(cur.Index))
		tokens = item.Tokenize(raw, delim)
	}
	var sb strings.Builder
	for i := 0; i < len(tmpl); i++ {
		if tmpl[i] != '{' {
			sb.WriteByte(tmpl[i])
			continue
		}
		end := strings.IndexByte(tmpl[i:], '}')
		if end < 0 {
			sb.WriteString(tmpl[i:])
			break
		}
		spec := tmpl[i+1 : i+end]
		i += end
		switch spec {
		case "":
			sb.WriteString(quote(raw))
		case "n":
			sb.WriteString(index)
		case "q":
			sb.WriteString(quote(query))
		case "+":
			if len(selected) == 0 && cur != nil {
				selected = []string{raw}
			}
			for j, s := range selected {
				if j > 0 {
					sb.WriteByte(' ')
				}
				sb.WriteString(quote(s))
			}
		default:
			// {1}, {2..3}, {-1}: the same field syntax as --nth.
			ranges, err := item.ParseNth(spec)
			if err != nil {
				sb.WriteString("{" + spec + "}")
				continue
			}
			sb.WriteString(quote(strings.TrimSpace(item.Transform(tokens, ranges))))
		}
	}
	return sb.String()
}

// currentPreviewCommand resolves the preview command for whatever the cursor
// is on right now.
func (t *Terminal) currentPreviewCommand() string {
	var cur *item.Item
	if t.cy >= 0 && t.cy < len(t.matches) {
		cur = t.matches[t.cy].Item
	}
	return previewCommand(t.opts.Preview, cur, t.selection(), string(t.query), t.opts.Delim)
}
