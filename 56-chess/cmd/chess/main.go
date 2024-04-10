// Command chess plays chess in the terminal: two humans, human against the
// computer, or computer against itself.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"chess/internal/ui"
)

func main() {
	mode := flag.String("mode", "hva", "play mode: hvh (human-human), hva (human-AI), ava (AI-AI)")
	depth := flag.Int("depth", 4, "AI search depth (0 = random legal moves)")
	ascii := flag.Bool("ascii", false, "use ASCII letters instead of Unicode chess glyphs")
	seed := flag.Int64("seed", time.Now().UnixNano(), "RNG seed for random-move AI")
	flag.Parse()

	var m ui.Mode
	switch *mode {
	case "hvh":
		m = ui.HumanVsHuman
	case "hva":
		m = ui.HumanVsAI
	case "ava":
		m = ui.AIVsAI
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q (want hvh, hva or ava)\n", *mode)
		os.Exit(2)
	}

	cli := ui.NewCLI(os.Stdin, os.Stdout, ui.Config{
		Mode:    m,
		Depth:   *depth,
		Unicode: !*ascii,
		Seed:    *seed,
	})
	cli.Run()
}
