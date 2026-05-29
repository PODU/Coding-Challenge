package main

import (
	"fmt"
	"os"
)

const usage = `Usage: dd [operand=value ...]

OPERANDS
  if=FILE        read from FILE (default: stdin)
  of=FILE        write to FILE  (default: stdout)
  bs=N           input AND output block size (overrides ibs/obs)
  ibs=N          input block size                       (default 512)
  obs=N          output block size                      (default 512)
  count=N        copy only N input blocks
  skip=N         skip N input blocks before copying
  seek=N         skip N output blocks before writing
  conv=L[,L...]  ucase,lcase,swab,notrunc,noerror,sync,sparse
  status=LEVEL   none | noxfer | progress
  iflag=L[,L...] fullblock,direct          (platform-dependent)
  oflag=L[,L...] dsync,direct              (platform-dependent)

SIZES accept suffixes: c=1 w=2 b=512 k=1024 M=1024^2 G=1024^3 ; "NxM" multiplies

EXIT  0 success ; 1 usage/operand error ; 2 I/O error
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprint(stdout, usage)
			return 0
		}
		if a == "--version" {
			fmt.Fprintln(stdout, "dd (Coding Challenge #121) 1.0.0")
			return 0
		}
	}

	cfg, err := ParseOperands(args)
	if err != nil {
		fmt.Fprintf(stderr, "dd: %v\n", err)
		return 1
	}

	stats, code, runErr := Run(cfg, stdin, stdout, stderr)
	if runErr != nil {
		fmt.Fprintf(stderr, "dd: %v\n", runErr)
	}

	if summary := stats.Summary(cfg.Status); summary != "" {
		fmt.Fprint(stderr, summary)
	}

	return code
}
