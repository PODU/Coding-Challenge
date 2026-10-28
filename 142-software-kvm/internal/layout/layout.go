// Package layout parses Barrier-style screen configuration files.
//
//	section: screens
//	    desktop:
//	    laptop:
//	        halfDuplexCapsLock = false
//	end
//	section: aliases
//	    laptop:
//	        laptop.local
//	end
//	section: links
//	    desktop:
//	        right = laptop
//	    laptop:
//	        left = desktop
//	end
//	section: options
//	    relativeMouseMoves = true
//	end
//
// Links are one-way, as in Barrier: list both directions for a round trip.
// Screen names are matched case-insensitively.
package layout

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Direction is a screen edge.
type Direction int

const (
	Left Direction = iota
	Right
	Up
	Down
)

func (d Direction) String() string {
	return [...]string{"left", "right", "up", "down"}[d]
}

// Opposite is the edge a cursor arrives on after leaving through d.
func (d Direction) Opposite() Direction {
	return [...]Direction{Right, Left, Down, Up}[d]
}

func parseDirection(s string) (Direction, bool) {
	switch strings.ToLower(s) {
	case "left":
		return Left, true
	case "right":
		return Right, true
	case "up", "top":
		return Up, true
	case "down", "bottom":
		return Down, true
	}
	return 0, false
}

// Config is a parsed layout.
type Config struct {
	screens       []string                        // canonical names, in file order
	canon         map[string]string               // lower-cased name or alias -> canonical
	links         map[string]map[Direction]string // canonical -> edge -> canonical
	screenOptions map[string]map[string]string
	Options       map[string]string
}

// Screens returns the canonical screen names in file order.
func (c *Config) Screens() []string { return append([]string(nil), c.screens...) }

// Canonical resolves a name or alias to the screen's canonical name.
func (c *Config) Canonical(name string) (string, bool) {
	n, ok := c.canon[strings.ToLower(name)]
	return n, ok
}

// Neighbor returns the screen beyond edge d of screen, if one is linked.
func (c *Config) Neighbor(screen string, d Direction) (string, bool) {
	n, ok := c.links[screen][d]
	return n, ok
}

// ScreenOption returns a per-screen option from the screens section.
func (c *Config) ScreenOption(screen, key string) (string, bool) {
	v, ok := c.screenOptions[screen][strings.ToLower(key)]
	return v, ok
}

// Bool reads a global boolean option ("true"/"false"); def if unset.
func (c *Config) Bool(key string, def bool) bool {
	v, ok := c.Options[strings.ToLower(key)]
	if !ok {
		return def
	}
	return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
}

// Load parses a configuration file.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	c, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Parse reads a configuration and validates every reference in it.
func Parse(r io.Reader) (*Config, error) {
	c := &Config{
		canon:         map[string]string{},
		links:         map[string]map[Direction]string{},
		screenOptions: map[string]map[string]string{},
		Options:       map[string]string{},
	}
	type pendingLink struct {
		line         int
		from, target string
		dir          Direction
	}
	type pendingAlias struct {
		line          int
		screen, alias string
	}
	var links []pendingLink
	var aliases []pendingAlias

	sc := bufio.NewScanner(r)
	section, current, lineNo := "", "", 0
	errf := func(format string, a ...any) error {
		return fmt.Errorf("line %d: %s", lineNo, fmt.Sprintf(format, a...))
	}
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if section == "" {
			name, ok := strings.CutPrefix(line, "section:")
			if !ok {
				return nil, errf("expected \"section: <name>\", got %q", line)
			}
			section = strings.ToLower(strings.TrimSpace(name))
			switch section {
			case "screens", "links", "aliases", "options":
			default:
				return nil, errf("unknown section %q", section)
			}
			current = ""
			continue
		}
		if line == "end" {
			section = ""
			continue
		}
		// "name:" opens a screen block inside screens, links and aliases.
		if name, ok := strings.CutSuffix(line, ":"); ok && section != "options" && !strings.Contains(name, "=") {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, errf("empty screen name")
			}
			if section == "screens" {
				if _, dup := c.canon[strings.ToLower(name)]; dup {
					return nil, errf("screen %q defined twice", name)
				}
				c.screens = append(c.screens, name)
				c.canon[strings.ToLower(name)] = name
			}
			current = name
			continue
		}
		switch section {
		case "options":
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil, errf("expected \"option = value\", got %q", line)
			}
			c.Options[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		case "screens":
			k, v, ok := strings.Cut(line, "=")
			if !ok || current == "" {
				return nil, errf("expected \"option = value\" under a screen, got %q", line)
			}
			opts := c.screenOptions[current]
			if opts == nil {
				opts = map[string]string{}
				c.screenOptions[current] = opts
			}
			opts[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		case "aliases":
			if current == "" {
				return nil, errf("alias %q is not under a screen", line)
			}
			aliases = append(aliases, pendingAlias{lineNo, current, line})
		case "links":
			if current == "" {
				return nil, errf("link %q is not under a screen", line)
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil, errf("expected \"<edge> = <screen>\", got %q", line)
			}
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			if strings.ContainsAny(k, "()") || strings.ContainsAny(v, "()") {
				return nil, errf("partial-edge ranges like %q are not supported", line)
			}
			d, ok := parseDirection(k)
			if !ok {
				return nil, errf("unknown edge %q (want left, right, up or down)", k)
			}
			links = append(links, pendingLink{lineNo, current, v, d})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if section != "" {
		return nil, fmt.Errorf("section %q is missing its \"end\"", section)
	}
	if len(c.screens) == 0 {
		return nil, fmt.Errorf("no screens defined")
	}

	for _, a := range aliases {
		lineNo = a.line
		screen, ok := c.Canonical(a.screen)
		if !ok {
			return nil, errf("alias for unknown screen %q", a.screen)
		}
		if other, taken := c.canon[strings.ToLower(a.alias)]; taken && other != screen {
			return nil, errf("alias %q already names screen %q", a.alias, other)
		}
		c.canon[strings.ToLower(a.alias)] = screen
	}
	for _, l := range links {
		lineNo = l.line
		from, ok := c.Canonical(l.from)
		if !ok {
			return nil, errf("links section names unknown screen %q", l.from)
		}
		to, ok := c.Canonical(l.target)
		if !ok {
			return nil, errf("%s %s = %s: unknown screen %q", from, l.dir, l.target, l.target)
		}
		if to == from {
			return nil, errf("screen %q links to itself", from)
		}
		if c.links[from] == nil {
			c.links[from] = map[Direction]string{}
		}
		c.links[from][l.dir] = to
	}
	return c, nil
}
