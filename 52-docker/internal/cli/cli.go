// Package cli parses the ccrun command line into a container.Spec.
package cli

import (
	"fmt"
	"strings"

	"ccrun/internal/container"
)

// Usage is printed with parse errors.
const Usage = "usage: ccrun run [--rootfs DIR] [--memory BYTES] [--cpus N] [--hostname NAME] [--tag TAG] <IMAGE|COMMAND> [ARGS...]"

// Parse parses os.Args[1:], which looks like
//
//	run [flags] <IMAGE|COMMAND> [ARGS...]
//
// Flags stop at the first non-flag or at "--", so the container command can
// have flags of its own. With --rootfs the first operand is the command;
// otherwise it is the image and the optional second operand is the command.
func Parse(args []string) (container.Spec, error) {
	var spec container.Spec

	if len(args) == 0 {
		return spec, fmt.Errorf("no subcommand given\n%s", Usage)
	}
	if args[0] != "run" {
		return spec, fmt.Errorf("unknown subcommand %q (only \"run\" is supported)\n%s", args[0], Usage)
	}

	rest := args[1:]
	var positionals []string

	i := 0
	for i < len(rest) {
		tok := rest[i]

		if tok == "--" {
			positionals = append(positionals, rest[i+1:]...)
			i = len(rest)
			break
		}

		if !strings.HasPrefix(tok, "--") {
			positionals = append(positionals, rest[i:]...)
			break
		}

		name, inlineVal, hasInline := strings.Cut(tok[2:], "=")

		value := func() (string, error) {
			if hasInline {
				return inlineVal, nil
			}
			if i+1 >= len(rest) {
				return "", fmt.Errorf("flag --%s requires a value", name)
			}
			i++
			return rest[i], nil
		}

		var err error
		switch name {
		case "rootfs":
			spec.Rootfs, err = value()
		case "memory":
			spec.Memory, err = value()
		case "cpus":
			spec.CPUs, err = value()
		case "hostname":
			spec.Hostname, err = value()
		case "tag":
			spec.Tag, err = value()
		default:
			return spec, fmt.Errorf("unknown flag --%s\n%s", name, Usage)
		}
		if err != nil {
			return spec, err
		}
		i++
	}

	if len(positionals) == 0 {
		return spec, fmt.Errorf("run requires an image or command\n%s", Usage)
	}

	if spec.Rootfs != "" {
		spec.Command = positionals[0]
		spec.Args = tail(positionals, 1)
	} else {
		spec.Image = positionals[0]
		if len(positionals) > 1 {
			spec.Command = positionals[1]
			spec.Args = tail(positionals, 2)
		}
	}

	return spec, nil
}

// tail returns positionals[n:], or nil rather than an empty slice.
func tail(positionals []string, n int) []string {
	if len(positionals) <= n {
		return nil
	}
	return positionals[n:]
}
