package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Colour codes are the ANSI base colours (30 + n for foreground).
const (
	colBlack uint8 = iota
	colRed
	colGreen
	colYellow
	colBlue
	colMagenta
	colCyan
	colWhite
)

// colourNames is in the order the challenge lists them; it is also the order
// printed when an unknown colour is given.
var colourNames = []string{"green", "red", "blue", "white", "yellow", "cyan", "magenta", "black"}

var colourCodes = map[string]uint8{
	"black": colBlack, "red": colRed, "green": colGreen, "yellow": colYellow,
	"blue": colBlue, "magenta": colMagenta, "cyan": colCyan, "white": colWhite,
}

// rainbowColours excludes black (invisible on a dark background) and white
// (indistinguishable from the heads).
var rainbowColours = []uint8{colRed, colGreen, colYellow, colBlue, colMagenta, colCyan}

const (
	boldNone = iota
	boldSome
	boldAll
)

const (
	minDelay     = 0
	maxDelay     = 10
	defaultDelay = 4
)

type Options struct {
	Screensaver bool  // -s: any key or mouse event exits
	Delay       int   // -u: 0 (fastest) .. 10
	Colour      uint8 // -C
	Rainbow     bool  // -r
	Katakana    bool  // -c
	Bold        int   // -b some, -B all
	Flicker     bool  // -f: trail characters keep changing
	Help        bool
}

const usage = `Usage: ccmatrix [-s] [-u DELAY] [-C COLOUR] [-r] [-c] [-b|-B] [-f]

Falling green text in the terminal. Press q to quit, 0-9 to change speed.

  -s          screensaver mode: any key or mouse click exits
  -u DELAY    delay between updates, 0 (fastest) to 10 (default 4)
  -C COLOUR   trail colour: green (default), red, blue, white, yellow,
              cyan, magenta, black
  -r          rainbow trails
  -c          half-width katakana instead of letters and digits
  -b          some characters bold
  -B          all characters bold
  -f          trail characters flicker as they fall
  -h          show this help
`

// unknownColourError is reported separately so the message can list the
// supported colours.
type unknownColourError struct{ name string }

func (e unknownColourError) Error() string {
	return fmt.Sprintf("unsupported colour %q\nsupported colours: %s", e.name, strings.Join(colourNames, ", "))
}

// parseArgs accepts cmatrix-style short options. Flags may be grouped (-sr),
// and a value may be attached (-u2) or separate (-u 2).
func parseArgs(args []string) (Options, error) {
	o := Options{Delay: defaultDelay, Colour: colGreen}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--help" {
			o.Help = true
			continue
		}
		if a == "--" {
			if i+1 < len(args) {
				return o, fmt.Errorf("unexpected argument %q", args[i+1])
			}
			break
		}
		if len(a) < 2 || a[0] != '-' || a[1] == '-' {
			return o, fmt.Errorf("unexpected argument %q", a)
		}
		for j := 1; j < len(a); j++ {
			f := a[j]
			switch f {
			case 's':
				o.Screensaver = true
			case 'r':
				o.Rainbow = true
			case 'c':
				o.Katakana = true
			case 'b':
				if o.Bold < boldSome {
					o.Bold = boldSome
				}
			case 'B':
				o.Bold = boldAll
			case 'f':
				o.Flicker = true
			case 'h':
				o.Help = true
			case 'u', 'C':
				val := a[j+1:]
				if val == "" {
					if i+1 >= len(args) {
						return o, fmt.Errorf("option -%c requires a value", f)
					}
					i++
					val = args[i]
				}
				if err := o.setValue(f, val); err != nil {
					return o, err
				}
				j = len(a) // the rest of the argument was the value
			default:
				return o, fmt.Errorf("unknown option -%c", f)
			}
		}
	}
	return o, nil
}

func (o *Options) setValue(flag byte, val string) error {
	switch flag {
	case 'u':
		n, err := strconv.Atoi(val)
		if err != nil || n < minDelay || n > maxDelay {
			return fmt.Errorf("invalid delay %q: must be a whole number from %d to %d", val, minDelay, maxDelay)
		}
		o.Delay = n
	case 'C':
		c, ok := colourCodes[strings.ToLower(val)]
		if !ok {
			return unknownColourError{val}
		}
		o.Colour = c
	}
	return nil
}
