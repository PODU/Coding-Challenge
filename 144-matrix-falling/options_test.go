package main

import (
	"errors"
	"strings"
	"testing"
)

func TestParseArgsDefaults(t *testing.T) {
	o, err := parseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o.Delay != 4 || o.Colour != colGreen || o.Screensaver || o.Rainbow || o.Katakana || o.Bold != boldNone {
		t.Fatalf("unexpected defaults: %+v", o)
	}
}

func TestParseArgsFlags(t *testing.T) {
	tests := []struct {
		args []string
		want Options
	}{
		{[]string{"-s"}, Options{Screensaver: true, Delay: 4, Colour: colGreen}},
		{[]string{"-u", "0"}, Options{Delay: 0, Colour: colGreen}},
		{[]string{"-u10"}, Options{Delay: 10, Colour: colGreen}},
		{[]string{"-C", "red"}, Options{Delay: 4, Colour: colRed}},
		{[]string{"-C", "CYAN"}, Options{Delay: 4, Colour: colCyan}},
		{[]string{"-r"}, Options{Delay: 4, Colour: colGreen, Rainbow: true}},
		{[]string{"-c", "-C", "cyan", "-u", "2"}, Options{Delay: 2, Colour: colCyan, Katakana: true}},
		{[]string{"-scu", "7"}, Options{Screensaver: true, Katakana: true, Delay: 7, Colour: colGreen}},
		{[]string{"-rCblue"}, Options{Rainbow: true, Delay: 4, Colour: colBlue}},
		{[]string{"-b"}, Options{Delay: 4, Colour: colGreen, Bold: boldSome}},
		{[]string{"-B", "-b"}, Options{Delay: 4, Colour: colGreen, Bold: boldAll}},
		{[]string{"-f"}, Options{Delay: 4, Colour: colGreen, Flicker: true}},
		{[]string{"-h"}, Options{Delay: 4, Colour: colGreen, Help: true}},
		{[]string{"--help"}, Options{Delay: 4, Colour: colGreen, Help: true}},
	}
	for _, tt := range tests {
		got, err := parseArgs(tt.args)
		if err != nil {
			t.Errorf("%v: %v", tt.args, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%v: got %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestParseArgsErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-u", "11"}, {"-u", "-1"}, {"-u", "fast"}, {"-u", "2.5"}, {"-u"},
		{"-C"}, {"-x"}, {"file"}, {"--verbose"},
	} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
}

func TestUnknownColourListsSupported(t *testing.T) {
	_, err := parseArgs([]string{"-C", "purple"})
	var ce unknownColourError
	if !errors.As(err, &ce) {
		t.Fatalf("got %v, want unknownColourError", err)
	}
	for _, c := range []string{"green", "red", "blue", "white", "yellow", "cyan", "magenta", "black"} {
		if !strings.Contains(err.Error(), c) {
			t.Errorf("message %q does not list %s", err, c)
		}
	}
}

func TestRealMainErrorsExitNonZero(t *testing.T) {
	for _, args := range [][]string{{"-u", "11"}, {"-u", "fast"}, {"-C", "purple"}} {
		if code := realMain(args); code == 0 {
			t.Errorf("%v: exit code 0", args)
		}
	}
}
