//go:build windows

package platform

import (
	"os"
	"testing"
	"unsafe"

	"github.com/podu2997/cckvm/internal/protocol"
)

func TestInputStructSize(t *testing.T) {
	want := uintptr(40) // sizeof(INPUT) on 64-bit Windows
	if unsafe.Sizeof(uintptr(0)) == 4 {
		want = 28
	}
	if got := unsafe.Sizeof(input{}); got != want {
		t.Fatalf("sizeof(input) = %d, want %d", got, want)
	}
}

func TestKeyInputLayout(t *testing.T) {
	in := keyInput(0x41, 0x1E, keyUp)
	b := unsafe.Slice((*byte)(unsafe.Pointer(&in.mi)), unsafe.Sizeof(in.mi))
	// KEYBDINPUT: wVk=0x41, wScan=0x1E, dwFlags=KEYEVENTF_KEYUP, time=0
	want := []byte{0x41, 0, 0x1E, 0, keyUp, 0, 0, 0, 0, 0, 0, 0}
	for i, c := range want {
		if b[i] != c {
			t.Fatalf("byte %d = %#x, want %#x (% x)", i, b[i], c, b[:12])
		}
	}
}

func TestKeyToVK(t *testing.T) {
	cases := map[uint16]struct {
		vk  uint16
		ext bool
	}{
		protocol.KeyLeftArrow: {0x25, true},
		protocol.KeyReturn:    {0x0D, false},
		protocol.KeyKPEnter:   {0x0D, true},
		protocol.KeyShiftL:    {0xA0, false},
		protocol.KeyControlR:  {0xA3, true},
		protocol.KeyF1 + 11:   {0x7B, false}, // F12
		protocol.KeyKP0 + 5:   {0x65, false},
		protocol.KeyPrint:     {0x2C, true},
		protocol.KeySuperL:    {0x5B, true},
	}
	for id, want := range cases {
		if got := keyToVK[id]; got != want {
			t.Errorf("%#x: got %+v want %+v", id, got, want)
		}
	}
}

func TestKeyInputsForCharacters(t *testing.T) {
	// A letter with the right shift state uses its virtual key; a
	// character no key produces falls back to KEYEVENTF_UNICODE.
	down, up := keyInputs('a', 0)
	if len(down) != 1 || len(up) != 1 {
		t.Fatalf("got %d/%d inputs", len(down), len(up))
	}
	if vk := uint16(down[0].mi.dx); vk != 'A' {
		t.Skipf("keyboard layout maps 'a' to vk %#x; skipping layout-specific checks", vk)
	}
	down, _ = keyInputs('A', protocol.ModShift)
	if uint16(down[0].mi.dx) != 'A' || uint32(down[0].mi.dy)&keyUnicode != 0 {
		t.Fatalf("shifted A: %+v", down[0])
	}
	// 'A' without shift cannot come from the A key: type the code point.
	down, _ = keyInputs('A', 0)
	if uint32(down[0].mi.dy)&keyUnicode == 0 || uint16(down[0].mi.dx>>16) != 'A' {
		t.Fatalf("unshifted A: %+v", down[0])
	}
	down, _ = keyInputs(0x263A, 0) // ☺
	if uint32(down[0].mi.dy)&keyUnicode == 0 {
		t.Fatalf("smiley: %+v", down[0])
	}
	if d, _ := keyInputs(0xE123, 0); len(d) != 0 {
		t.Fatal("unknown special key produced input")
	}
}

// TestNativeDesktop touches the real desktop (it briefly replaces the
// clipboard, then restores it), so it only runs when asked for.
func TestNativeDesktop(t *testing.T) {
	if os.Getenv("CCKVM_NATIVE_TEST") == "" {
		t.Skip("set CCKVM_NATIVE_TEST=1 to test against the real desktop")
	}
	s, err := Native(NativeOptions{Capture: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Native(NativeOptions{Capture: true}); err == nil {
		t.Fatal("second capture accepted")
	}
	r := s.Shape()
	if r.W <= 0 || r.H <= 0 {
		t.Fatalf("shape %v", r)
	}
	x, y := s.CursorPos()
	s.MouseMove(x, y)

	old, hadOld := s.Clipboard(protocol.ClipboardMain)
	if !hadOld {
		t.Skip("clipboard holds no text; not overwriting it")
	}
	s.SetClipboard(protocol.ClipboardMain, "cckvm test\nline two ☺")
	got, ok := s.Clipboard(protocol.ClipboardMain)
	s.SetClipboard(protocol.ClipboardMain, old)
	if !ok || got != "cckvm test\nline two ☺" {
		t.Fatalf("clipboard round trip: %q %v", got, ok)
	}
}
