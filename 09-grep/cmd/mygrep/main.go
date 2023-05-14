// Command mygrep is a small grep clone.
package main

import (
	"fmt"
	"os"

	"mygrep/internal/cli"
	"mygrep/internal/grep"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mygrep: %v\n\n%s", err, cli.Usage)
		os.Exit(2)
	}
	if cfg.Help {
		fmt.Print(cli.Usage)
		os.Exit(0)
	}
	os.Exit(grep.Run(cfg, os.Stdin, os.Stdout, os.Stderr))
}
