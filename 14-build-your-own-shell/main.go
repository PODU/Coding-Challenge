package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/peterh/liner"
)

const prompt = "ccsh> "

func main() {
	cmdFlag := flag.String("c", "", "run a single command string and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: ccsh [-c \"command\"] [script-file]")
		flag.PrintDefaults()
	}
	flag.Parse()

	sh := NewShell()
	if err := sh.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "ccsh: warning: could not load history: %v\n", err)
	}

	if *cmdFlag != "" {
		status, _ := sh.Execute(*cmdFlag)
		os.Exit(status)
	}

	if args := flag.Args(); len(args) > 0 {
		os.Exit(runScript(sh, args[0]))
	}

	// Not a terminal: treat stdin as a script.
	if !liner.TerminalSupported() {
		os.Exit(runReader(sh, os.Stdin))
	}

	runInteractive(sh)
}

func runScript(sh *Shell, path string) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccsh: %s: %v\n", path, err)
		return statusGeneralError
	}
	defer f.Close()
	return runReader(sh, f)
}

// runReader runs each line of r until EOF or exit. Blank lines and # comments
// are skipped.
func runReader(sh *Shell, r io.Reader) int {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		status, quit := sh.Execute(line)
		if quit {
			return status
		}
	}
	return sh.LastStatus
}

// runInteractive runs the prompt loop with line editing and history.
func runInteractive(sh *Shell) {
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)

	for _, h := range sh.History.Entries() {
		line.AppendHistory(h)
	}

	// Forward SIGINT to the running child instead of letting it kill the shell.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		for range sigCh {
			sh.InterruptForeground()
		}
	}()

	for {
		input, err := line.Prompt(prompt)
		if err == liner.ErrPromptAborted {
			// Ctrl-C at the prompt just discards the line.
			continue
		}
		if err == io.EOF {
			// Ctrl-D
			fmt.Fprintln(sh.Stdout)
			break
		}
		if err != nil {
			fmt.Fprintf(sh.Stderr, "ccsh: input error: %v\n", err)
			break
		}

		if strings.TrimSpace(input) == "" {
			continue
		}

		line.AppendHistory(input)
		_ = sh.History.Add(input)

		status, quit := sh.Execute(input)
		if quit {
			_ = sh.History.Save()
			os.Exit(status)
		}
	}

	_ = sh.History.Save()
}
