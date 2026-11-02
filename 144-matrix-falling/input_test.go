package main

import (
	"reflect"
	"testing"
)

func TestParseInput(t *testing.T) {
	tests := []struct {
		in   string
		want []Event
	}{
		{"q", []Event{{Kind: EvKey, Key: 'q'}}},
		{"a5", []Event{{Kind: EvKey, Key: 'a'}, {Kind: EvKey, Key: '5'}}},
		{"\x1b[A", []Event{{Kind: EvSpecial}}},
		{"\x1b[1;5A", []Event{{Kind: EvSpecial}}}, // Ctrl+Up: digits must not leak out
		{"\x1b[15~3", []Event{{Kind: EvSpecial}, {Kind: EvKey, Key: '3'}}},
		{"\x1bOP", []Event{{Kind: EvSpecial}}},
		{"\x1b", []Event{{Kind: EvSpecial}}},
		{"\x1bx", []Event{{Kind: EvSpecial}}},
		{"\x1b[<0;12;5M\x1b[<0;12;5m", []Event{{Kind: EvMouse}, {Kind: EvMouse}}},
		{"\x1b[M !!q", []Event{{Kind: EvMouse}, {Kind: EvKey, Key: 'q'}}},
		{"\r", []Event{{Kind: EvKey, Key: '\r'}}},
	}
	for _, tt := range tests {
		if got := parseInput([]byte(tt.in)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestDecideNormalMode(t *testing.T) {
	key := func(b byte) Event { return Event{Kind: EvKey, Key: b} }
	if !decide(key('q'), false).quit || !decide(key(0x03), false).quit {
		t.Fatal("q and Ctrl+C should quit")
	}
	for _, ev := range []Event{key('a'), key('x'), key(' '), key('\r'), {Kind: EvSpecial}, {Kind: EvMouse}} {
		if a := decide(ev, false); a.quit || a.speed != -1 {
			t.Errorf("%+v should be ignored, got %+v", ev, a)
		}
	}
	for d := byte(0); d <= 9; d++ {
		if a := decide(key('0'+d), false); a.quit || a.speed != int(d) {
			t.Errorf("digit %d: got %+v", d, a)
		}
	}
}

func TestDecideScreensaverMode(t *testing.T) {
	for _, ev := range []Event{
		{Kind: EvKey, Key: ' '}, {Kind: EvKey, Key: 'a'}, {Kind: EvKey, Key: '7'},
		{Kind: EvKey, Key: '\r'}, {Kind: EvSpecial}, {Kind: EvMouse},
	} {
		if !decide(ev, true).quit {
			t.Errorf("%+v should exit in screensaver mode", ev)
		}
	}
}
