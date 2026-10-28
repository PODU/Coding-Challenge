package protocol

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Golden bytes for the greeting, as a real Barrier 2.x server sends it.
func TestHelloWireBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, Hello{1, 6}.Marshal()); err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 0, 11, 'B', 'a', 'r', 'r', 'i', 'e', 'r', 0, 1, 0, 6}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("got % x\nwant % x", buf.Bytes(), want)
	}
}

func TestHelloBackWireBytes(t *testing.T) {
	got := HelloBack{1, 6, "laptop"}.Marshal()
	want := append([]byte("Barrier\x00\x01\x00\x06\x00\x00\x00\x06"), "laptop"...)
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x\nwant % x", got, want)
	}
}

func TestSignedFields(t *testing.T) {
	b := MouseMove{X: -5, Y: 1080}.Marshal()
	if want := []byte("DMMV\xff\xfb\x04\x38"); !bytes.Equal(b, want) {
		t.Fatalf("got % x want % x", b, want)
	}
	m, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if m != (MouseMove{-5, 1080}) {
		t.Fatalf("round trip: %#v", m)
	}
}

func TestRoundTripAll(t *testing.T) {
	msgs := []Message{
		Hello{1, 6}, HelloBack{1, 6, "box"},
		Noop{}, Close{}, Enter{-10, 20, 7, ModShift | ModCapsLock}, Leave{},
		Grab{1, 99}, ScreenSaver{true}, ScreenSaver{false}, ResetOptions{}, InfoAck{}, KeepAlive{},
		KeyDown{'a', ModShift, 30}, KeyRepeat{KeyLeftArrow, 0, 3, 75}, KeyUp{KeyReturn, ModControl, 28},
		MouseDown{ButtonRight}, MouseUp{ButtonExtra1}, MouseMove{100, -200}, MouseRelMove{-3, 4},
		MouseWheel{0, -WheelDelta}, ClipboardData{0, 5, MarkChunk, "hello\x00world"},
		Info{0, 0, 1920, 1080, 0, 960, 540}, SetOptions{[]uint32{0x48415254, 5000}},
		FileTransfer{1, "x"}, DragInfo{2, "a,b,"},
		QueryInfo{}, Incompatible{1, 6}, Busy{}, Unknown{}, Bad{},
	}
	for _, m := range msgs {
		got, err := Parse(m.Marshal())
		if err != nil {
			t.Fatalf("%T: %v", m, err)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatalf("%T: got %#v want %#v", m, got, m)
		}
		if !strings.HasPrefix(string(m.Marshal()), m.Code()) {
			t.Fatalf("%T: payload does not start with %q", m, m.Code())
		}
	}
}

func TestEnterLayout(t *testing.T) {
	// CINN%2i%2i%4i%2i: x, y, seq, mask
	got := Enter{X: 1, Y: 2, Seq: 0x01020304, Mask: 0x1001}.Marshal()
	want := []byte("CINN\x00\x01\x00\x02\x01\x02\x03\x04\x10\x01")
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x want % x", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	var unk *UnknownMessageError
	if _, err := Parse([]byte("ZZZZ")); !errors.As(err, &unk) || unk.Code != "ZZZZ" {
		t.Fatalf("unknown code: %v", err)
	}
	if _, err := Parse([]byte("DMMV\x00")); !errors.Is(err, ErrShort) {
		t.Fatalf("short: %v", err)
	}
	if _, err := Parse([]byte("ab")); err == nil {
		t.Fatal("2-byte message accepted")
	}
	// A %s claiming 4 GB must be rejected before any allocation.
	if _, err := Parse([]byte("DCLP\x00\x00\x00\x00\x01\x02\xff\xff\xff\xff")); err == nil {
		t.Fatal("huge string accepted")
	}
}

func TestTrailingBytesIgnored(t *testing.T) {
	m, err := Parse([]byte("DMDN\x01extra"))
	if err != nil || m != (MouseDown{1}) {
		t.Fatalf("got %v %v", m, err)
	}
}

func TestPackErrors(t *testing.T) {
	if _, err := Pack("X%2i"); err == nil {
		t.Fatal("missing arg accepted")
	}
	if _, err := Pack("X", 1); err == nil {
		t.Fatal("extra arg accepted")
	}
	if _, err := Pack("%2i", "x"); err == nil {
		t.Fatal("string for int accepted")
	}
	b, err := Pack("100%%")
	if err != nil || string(b) != "100%" {
		t.Fatalf("%%%%: %q %v", b, err)
	}
}

func TestFrameLimits(t *testing.T) {
	if _, err := ReadFrame(bytes.NewReader([]byte{0xff, 0, 0, 0})); err == nil {
		t.Fatal("oversized frame accepted")
	}
	if _, err := ReadFrame(bytes.NewReader([]byte{0, 0, 0, 5, 'a'})); err == nil {
		t.Fatal("truncated frame accepted")
	}
}

func TestClipboardMarshal(t *testing.T) {
	data := MarshalClipboard(map[uint32]string{FormatHTML: "<b>x</b>", FormatText: "x"})
	want := "\x00\x00\x00\x02" + "\x00\x00\x00\x00\x00\x00\x00\x01x" + "\x00\x00\x00\x01\x00\x00\x00\x08<b>x</b>"
	if data != want {
		t.Fatalf("got %q", data)
	}
	got, err := UnmarshalClipboard(data)
	if err != nil || got[FormatText] != "x" || got[FormatHTML] != "<b>x</b>" {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := UnmarshalClipboard("\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x09ab"); err == nil {
		t.Fatal("truncated clipboard accepted")
	}
}

func TestClipboardChunking(t *testing.T) {
	big := strings.Repeat("0123456789", ChunkSize/4) // 2.5 chunks
	data := TextClipboard(big)
	msgs := ChunkClipboard(0, 42, data)
	if msgs[0].Mark != MarkStart || msgs[len(msgs)-1].Mark != MarkEnd || len(msgs) != 5 {
		t.Fatalf("bad chunk sequence: %d msgs", len(msgs))
	}
	var a Assembler
	var out string
	for i, m := range msgs {
		// Each chunk must survive the wire format too.
		p, err := Parse(m.Marshal())
		if err != nil {
			t.Fatal(err)
		}
		d, done, err := a.Add(p.(ClipboardData))
		if err != nil {
			t.Fatal(err)
		}
		if done != (i == len(msgs)-1) {
			t.Fatalf("done=%v at %d", done, i)
		}
		out = d
	}
	f, _ := UnmarshalClipboard(out)
	if f[FormatText] != big {
		t.Fatal("reassembled text differs")
	}
}

func TestAssemblerRejectsBadSequences(t *testing.T) {
	var a Assembler
	if _, _, err := a.Add(ClipboardData{Mark: MarkChunk, Data: "x"}); err == nil {
		t.Fatal("chunk without start")
	}
	a.Add(ClipboardData{Mark: MarkStart, Data: "1"})
	if _, _, err := a.Add(ClipboardData{Mark: MarkChunk, Data: "xy"}); err == nil {
		t.Fatal("overflow accepted")
	}
	a.Add(ClipboardData{Mark: MarkStart, Data: "3"})
	a.Add(ClipboardData{Mark: MarkChunk, Data: "x"})
	if _, _, err := a.Add(ClipboardData{Mark: MarkEnd}); err == nil {
		t.Fatal("short transfer accepted")
	}
	if _, _, err := a.Add(ClipboardData{ID: 9, Mark: MarkStart, Data: "0"}); err == nil {
		t.Fatal("bad id accepted")
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte("DCLP\x00\x00\x00\x00\x01\x02\x00\x00\x00\x01x"))
	f.Add([]byte("Barrier\x00\x01\x00\x06\x00\x00\x00\x01a"))
	f.Add([]byte("DSOP\x00\x00\x00\x02\x00\x00\x00\x01\x00\x00\x00\x02"))
	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := Parse(b)
		if err != nil {
			return
		}
		if _, err := Parse(m.Marshal()); err != nil {
			t.Fatalf("re-parse of %#v failed: %v", m, err)
		}
	})
}
