// Package help answers "help", "help <command>" and "help @<group>" from the
// command list in commands.json.
package help

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed commands.json
var commandsJSON []byte

type Command struct {
	Name    string `json:"name"`
	Group   string `json:"group"`
	Args    string `json:"args"`
	Summary string `json:"summary"`
	Since   string `json:"since"`
}

var (
	byName  = map[string]Command{}
	byGroup = map[string][]Command{}
	groups  []string
)

func init() {
	var cmds []Command
	if err := json.Unmarshal(commandsJSON, &cmds); err != nil {
		panic("help: invalid embedded commands.json: " + err.Error())
	}
	for _, c := range cmds {
		byName[strings.ToUpper(c.Name)] = c
		byGroup[c.Group] = append(byGroup[c.Group], c)
	}
	for g := range byGroup {
		groups = append(groups, g)
		sort.Slice(byGroup[g], func(i, j int) bool {
			return byGroup[g][i].Name < byGroup[g][j].Name
		})
	}
	sort.Strings(groups)
}

// Overview is the text for a bare "help".
func Overview() string {
	var b strings.Builder
	b.WriteString("redis-cli\n")
	b.WriteString("To get help about Redis commands type:\n")
	b.WriteString("      \"help <command>\" for help on <command>\n")
	b.WriteString("      \"help @<group>\" to get a list of commands in <group>\n")
	b.WriteString("      \"quit\" to exit\n\n")
	b.WriteString("Command groups:\n")
	for _, g := range groups {
		fmt.Fprintf(&b, "      @%s\n", g)
	}
	return strings.TrimRight(b.String(), "\n")
}

// CommandHelp describes one command. It returns false for unknown commands.
func CommandHelp(name string) (string, bool) {
	c, ok := byName[strings.ToUpper(name)]
	if !ok {
		return "", false
	}
	var b strings.Builder
	if c.Args != "" {
		fmt.Fprintf(&b, "  %s %s\n", c.Name, c.Args)
	} else {
		fmt.Fprintf(&b, "  %s\n", c.Name)
	}
	fmt.Fprintf(&b, "  summary: %s\n", c.Summary)
	fmt.Fprintf(&b, "  since: %s\n", c.Since)
	fmt.Fprintf(&b, "  group: %s", c.Group)
	return b.String(), true
}

// GroupHelp lists the commands in a group. The leading @ is optional.
func GroupHelp(group string) (string, bool) {
	group = strings.TrimPrefix(group, "@")
	cmds, ok := byGroup[group]
	if !ok {
		return "", false
	}
	var b strings.Builder
	for i, c := range cmds {
		if i > 0 {
			b.WriteByte('\n')
		}
		if c.Args != "" {
			fmt.Fprintf(&b, "  %s %s\n", c.Name, c.Args)
		} else {
			fmt.Fprintf(&b, "  %s\n", c.Name)
		}
		fmt.Fprintf(&b, "  summary: %s", c.Summary)
	}
	return b.String(), true
}

// Usage returns a command's name and arguments, e.g. "GET key".
func Usage(name string) (string, bool) {
	c, ok := byName[strings.ToUpper(name)]
	if !ok {
		return "", false
	}
	if c.Args == "" {
		return c.Name, true
	}
	return c.Name + " " + c.Args, true
}
