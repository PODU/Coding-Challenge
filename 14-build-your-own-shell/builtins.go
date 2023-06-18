package main

import (
	"fmt"
	"os"
	"strconv"
)

// Builtin is a command implemented inside the shell. It returns an exit status
// and whether the shell should quit afterwards.
type Builtin func(args []string, sh *Shell) (status int, quit bool)

var builtins map[string]Builtin

func init() {
	builtins = map[string]Builtin{
		"cd":      builtinCd,
		"pwd":     builtinPwd,
		"exit":    builtinExit,
		"history": builtinHistory,
	}
}

// IsBuiltin reports whether name is handled in-process.
func IsBuiltin(name string) bool {
	_, ok := builtins[name]
	return ok
}

func builtinCd(args []string, sh *Shell) (int, bool) {
	var target string
	switch {
	case len(args) < 2:
		target = homeDir()
		if target == "" {
			fmt.Fprintln(sh.Stderr, "ccsh: cd: HOME not set")
			return 1, false
		}
	case args[1] == "-":
		if sh.PrevDir == "" {
			fmt.Fprintln(sh.Stderr, "ccsh: cd: OLDPWD not set")
			return 1, false
		}
		target = sh.PrevDir
		fmt.Fprintln(sh.Stdout, target)
	default:
		target = args[1]
	}

	if err := os.Chdir(target); err != nil {
		fmt.Fprintf(sh.Stderr, "ccsh: cd: %s: %v\n", target, err)
		return 1, false
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = target
	}
	sh.PrevDir = sh.Cwd
	sh.Cwd = wd
	return 0, false
}

func builtinPwd(args []string, sh *Shell) (int, bool) {
	wd := sh.Cwd
	if w, err := os.Getwd(); err == nil {
		wd = w
		sh.Cwd = w
	}
	fmt.Fprintln(sh.Stdout, wd)
	return 0, false
}

func builtinExit(args []string, sh *Shell) (int, bool) {
	code := sh.LastStatus
	if len(args) > 1 {
		n, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(sh.Stderr, "ccsh: exit: %s: numeric argument required\n", args[1])
			return 2, true
		}
		code = n
	}
	return code, true
}

func builtinHistory(args []string, sh *Shell) (int, bool) {
	if len(args) > 1 && args[1] == "-c" {
		sh.History.Clear()
		return 0, false
	}
	for i, entry := range sh.History.Entries() {
		fmt.Fprintf(sh.Stdout, "%5d  %s\n", i+1, entry)
	}
	return 0, false
}
