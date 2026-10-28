//go:build windows

package platform

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/podu2997/cckvm/internal/protocol"
)

// The Windows backend uses only documented user32 APIs through syscall,
// with no cgo:
//
//   - capture: WH_MOUSE_LL / WH_KEYBOARD_LL hooks on a dedicated thread
//     that pumps messages. While captured the hooks swallow every event and
//     report mouse motion as the distance from a parked cursor position.
//   - injection: SendInput (keys, buttons, wheel, relative motion) and
//     SetCursorPos (absolute motion).
//   - clipboard: CF_UNICODETEXT through a message-only window owned by the
//     hook thread; changes are found by polling GetClipboardSequenceNumber.
//   - screensaver: polling SPI_GETSCREENSAVERRUNNING; started with
//     SC_SCREENSAVE.

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pSetWindowsHookExW          = user32.NewProc("SetWindowsHookExW")
	pCallNextHookEx             = user32.NewProc("CallNextHookEx")
	pUnhookWindowsHookEx        = user32.NewProc("UnhookWindowsHookEx")
	pGetMessageW                = user32.NewProc("GetMessageW")
	pPostThreadMessageW         = user32.NewProc("PostThreadMessageW")
	pPostMessageW               = user32.NewProc("PostMessageW")
	pCreateWindowExW            = user32.NewProc("CreateWindowExW")
	pDestroyWindow              = user32.NewProc("DestroyWindow")
	pGetCursorPos               = user32.NewProc("GetCursorPos")
	pSetCursorPos               = user32.NewProc("SetCursorPos")
	pSendInput                  = user32.NewProc("SendInput")
	pGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	pSetProcessDPIAware         = user32.NewProc("SetProcessDPIAware")
	pSetProcessDpiAwarenessCtx  = user32.NewProc("SetProcessDpiAwarenessContext")
	pMapVirtualKeyW             = user32.NewProc("MapVirtualKeyW")
	pToUnicodeEx                = user32.NewProc("ToUnicodeEx")
	pGetKeyboardLayout          = user32.NewProc("GetKeyboardLayout")
	pVkKeyScanExW               = user32.NewProc("VkKeyScanExW")
	pGetKeyState                = user32.NewProc("GetKeyState")
	pGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	pGetDesktopWindow           = user32.NewProc("GetDesktopWindow")
	pGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	pOpenClipboard              = user32.NewProc("OpenClipboard")
	pCloseClipboard             = user32.NewProc("CloseClipboard")
	pEmptyClipboard             = user32.NewProc("EmptyClipboard")
	pGetClipboardData           = user32.NewProc("GetClipboardData")
	pSetClipboardData           = user32.NewProc("SetClipboardData")
	pGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	pSystemParametersInfoW      = user32.NewProc("SystemParametersInfoW")

	pGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	pGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc        = kernel32.NewProc("GlobalAlloc")
	pGlobalFree         = kernel32.NewProc("GlobalFree")
	pGlobalLock         = kernel32.NewProc("GlobalLock")
	pGlobalUnlock       = kernel32.NewProc("GlobalUnlock")
)

const (
	whKeyboardLL = 13
	whMouseLL    = 14

	wmQuit        = 0x0012
	wmApp         = 0x8000
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmSysCommand  = 0x0112
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmMButtonDown = 0x0207
	wmMButtonUp   = 0x0208
	wmMouseWheel  = 0x020A
	wmXButtonDown = 0x020B
	wmXButtonUp   = 0x020C
	wmMouseHWheel = 0x020E

	scScreenSave = 0xF140

	llkhfExtended = 0x01
	llkhfInjected = 0x10
	llmhfInjected = 0x01

	inputMouse    = 0
	inputKeyboard = 1

	mouseMove       = 0x0001
	mouseLeftDown   = 0x0002
	mouseLeftUp     = 0x0004
	mouseRightDown  = 0x0008
	mouseRightUp    = 0x0010
	mouseMiddleDown = 0x0020
	mouseMiddleUp   = 0x0040
	mouseXDown      = 0x0080
	mouseXUp        = 0x0100
	mouseWheel      = 0x0800
	mouseHWheel     = 0x1000

	keyExtended = 0x0001
	keyUp       = 0x0002
	keyUnicode  = 0x0004

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	cfUnicodeText = 13
	gmemMoveable  = 0x0002
	hwndMessage   = ^uintptr(2) // (HWND)-3

	spiGetScreenSaverRunning = 0x0072

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkCapital = 0x14
	vkNumLock = 0x90
	vkScroll  = 0x91
)

type point struct{ X, Y int32 }

type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type kbdHook struct {
	vkCode, scanCode, flags, time uint32
	extra                         uintptr
}

type mouseHook struct {
	pt                     point
	mouseData, flags, time uint32
	extra                  uintptr
}

// input is INPUT with the MOUSEINPUT arm of the union. KEYBDINPUT is
// smaller and starts at the same offset, so keyboard input is written into
// the same bytes (see keyInput); the struct size matches sizeof(INPUT) on
// both 386 and amd64.
type input struct {
	typ uint32
	mi  mouseInput
}

type mouseInput struct {
	dx, dy    int32
	mouseData uint32
	flags     uint32
	time      uint32
	extra     uintptr
}

func keyInput(vk, scan uint16, flags uint32) input {
	// KEYBDINPUT: wVk, wScan (overlap dx), dwFlags (dy), time (mouseData),
	// dwExtraInfo (zero either way).
	return input{typ: inputKeyboard, mi: mouseInput{dx: int32(uint32(vk) | uint32(scan)<<16), dy: int32(flags)}}
}

// fromAddr turns an address handed over by Windows (an LPARAM or a
// GlobalLock result, memory Go does not manage) into a typed pointer.
func fromAddr[T any](addr uintptr) *T {
	return *(**T)(unsafe.Pointer(&addr))
}

func sendInput(in ...input) {
	if len(in) == 0 {
		return
	}
	pSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
}

// NativeOptions selects what the Windows backend sets up.
type NativeOptions struct {
	// Capture installs the low-level input hooks (needed by the server).
	Capture bool
}

type winScreen struct {
	opts   NativeOptions
	events chan Event

	thread   uint32 // hook/message thread id
	hwnd     uintptr
	hooks    []uintptr
	calls    chan func()
	stopped  chan struct{}
	stopPoll chan struct{}
	once     sync.Once

	captured atomic.Bool
	parkX    atomic.Int32
	parkY    atomic.Int32

	// hook thread only
	mask     uint16
	keysDown [256]bool

	// clipboard
	clipMu  sync.Mutex
	clipSeq uint32

	// injection, used from the client goroutine
	injMu   sync.Mutex
	pressed map[uint16][]input // KeyButton -> the key-up inputs that undo it
}

// hooked is the screen owning the hooks; callbacks cannot carry a receiver.
var hooked atomic.Pointer[winScreen]

var (
	mouseCallback = syscall.NewCallback(func(code, wParam, lParam uintptr) uintptr {
		if w := hooked.Load(); w != nil && int32(code) >= 0 {
			if w.onMouse(uint32(wParam), fromAddr[mouseHook](lParam)) {
				return 1
			}
		}
		r, _, _ := pCallNextHookEx.Call(0, code, wParam, lParam)
		return r
	})
	keyboardCallback = syscall.NewCallback(func(code, wParam, lParam uintptr) uintptr {
		if w := hooked.Load(); w != nil && int32(code) >= 0 {
			if w.onKey(uint32(wParam), fromAddr[kbdHook](lParam)) {
				return 1
			}
		}
		r, _, _ := pCallNextHookEx.Call(0, code, wParam, lParam)
		return r
	})
)

// Native opens the local desktop.
func Native(opts NativeOptions) (Screen, error) {
	// Work in physical pixels so coordinates match what the hooks report.
	if pSetProcessDpiAwarenessCtx.Find() == nil {
		pSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2 (-4)
	} else {
		pSetProcessDPIAware.Call()
	}
	w := &winScreen{
		opts:     opts,
		events:   make(chan Event, 1024),
		calls:    make(chan func(), 16),
		stopped:  make(chan struct{}),
		stopPoll: make(chan struct{}),
		pressed:  map[uint16][]input{},
	}
	if opts.Capture && !hooked.CompareAndSwap(nil, w) {
		return nil, errors.New("platform: input is already captured by this process")
	}
	ready := make(chan error, 1)
	go w.messageThread(ready)
	if err := <-ready; err != nil {
		hooked.CompareAndSwap(w, nil)
		return nil, err
	}
	w.clipSeq = clipboardSeq()
	go w.poll()
	return w, nil
}

func (w *winScreen) messageThread(ready chan<- error) {
	runtime.LockOSThread()
	defer close(w.stopped)
	tid, _, _ := pGetCurrentThreadId.Call()
	w.thread = uint32(tid)
	hmod, _, _ := pGetModuleHandleW.Call(0)

	cls, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("cckvm")
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
		0, 0, 0, 0, 0, hwndMessage, 0, hmod, 0)
	if hwnd == 0 {
		ready <- fmt.Errorf("platform: CreateWindowEx: %v", err)
		return
	}
	w.hwnd = hwnd
	defer pDestroyWindow.Call(hwnd)

	if w.opts.Capture {
		w.mask = lockState()
		for _, h := range []struct {
			id uintptr
			cb uintptr
		}{{whMouseLL, mouseCallback}, {whKeyboardLL, keyboardCallback}} {
			hk, _, err := pSetWindowsHookExW.Call(h.id, h.cb, hmod, 0)
			if hk == 0 {
				for _, x := range w.hooks {
					pUnhookWindowsHookEx.Call(x)
				}
				ready <- fmt.Errorf("platform: SetWindowsHookEx: %v", err)
				return
			}
			w.hooks = append(w.hooks, hk)
		}
		defer func() {
			for _, h := range w.hooks {
				pUnhookWindowsHookEx.Call(h)
			}
		}()
	}
	ready <- nil

	var m winMsg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		if m.hwnd == 0 && m.message == wmApp {
			for done := false; !done; {
				select {
				case f := <-w.calls:
					f()
				default:
					done = true
				}
			}
		}
	}
}

// onThread runs f on the message thread and waits for it.
func (w *winScreen) onThread(f func()) {
	done := make(chan struct{})
	select {
	case w.calls <- func() { f(); close(done) }:
	case <-w.stopped:
		return
	}
	pPostThreadMessageW.Call(uintptr(w.thread), wmApp, 0, 0)
	select {
	case <-done:
	case <-w.stopped:
	}
}

func (w *winScreen) emit(e Event) {
	select {
	case w.events <- e:
	default: // the consumer is stuck; dropping beats stalling the hook
	}
}

func (w *winScreen) onMouse(msg uint32, h *mouseHook) bool {
	if h.flags&llmhfInjected != 0 {
		return false
	}
	captured := w.captured.Load()
	x, y := int(h.pt.X), int(h.pt.Y)
	switch msg {
	case wmMouseMove:
		if captured {
			dx, dy := x-int(w.parkX.Load()), y-int(w.parkY.Load())
			if dx != 0 || dy != 0 {
				w.emit(Event{Kind: MouseDelta, DX: dx, DY: dy})
			}
			return true
		}
		w.emit(Event{Kind: MouseMotion, X: x, Y: y})
		return false
	case wmLButtonDown, wmLButtonUp:
		w.emit(Event{Kind: MouseButton, Button: protocol.ButtonLeft, Down: msg == wmLButtonDown})
	case wmRButtonDown, wmRButtonUp:
		w.emit(Event{Kind: MouseButton, Button: protocol.ButtonRight, Down: msg == wmRButtonDown})
	case wmMButtonDown, wmMButtonUp:
		w.emit(Event{Kind: MouseButton, Button: protocol.ButtonMiddle, Down: msg == wmMButtonDown})
	case wmXButtonDown, wmXButtonUp:
		b := protocol.ButtonExtra0
		if h.mouseData>>16 == 2 {
			b = protocol.ButtonExtra1
		}
		w.emit(Event{Kind: MouseButton, Button: b, Down: msg == wmXButtonDown})
	case wmMouseWheel:
		w.emit(Event{Kind: MouseWheel, DY: int(int16(h.mouseData >> 16))})
	case wmMouseHWheel:
		w.emit(Event{Kind: MouseWheel, DX: int(int16(h.mouseData >> 16))})
	default:
		return false
	}
	return captured
}

func (w *winScreen) onKey(msg uint32, h *kbdHook) bool {
	if h.flags&llkhfInjected != 0 || h.vkCode > 255 {
		return false
	}
	down := msg == wmKeyDown || msg == wmSysKeyDown
	if !down && msg != wmKeyUp && msg != wmSysKeyUp {
		return false
	}
	vk := h.vkCode
	ext := h.flags&llkhfExtended != 0
	id := w.keyID(vk, h.scanCode, ext)
	repeat := down && w.keysDown[vk]
	w.keysDown[vk] = down
	if bit := protocol.ModifierFor(id); bit != 0 {
		if down {
			w.mask |= bit
		} else if !w.otherSideDown(vk) {
			w.mask &^= bit
		}
	}
	if down && !repeat {
		switch vk {
		case vkCapital:
			w.mask ^= protocol.ModCapsLock
		case vkNumLock:
			w.mask ^= protocol.ModNumLock
		case vkScroll:
			w.mask ^= protocol.ModScrollLock
		}
	}
	button := uint16(h.scanCode & 0xff)
	if ext {
		button |= 0x100
	}
	if id != protocol.KeyNone {
		w.emit(Event{Kind: Key, KeyID: id, Mask: w.mask, KeyButton: button, Down: down, Repeat: repeat})
	}
	return w.captured.Load()
}

// otherSideDown reports whether the twin of a left/right modifier is held.
func (w *winScreen) otherSideDown(vk uint32) bool {
	twin := map[uint32]uint32{0xA0: 0xA1, 0xA1: 0xA0, 0xA2: 0xA3, 0xA3: 0xA2, 0xA4: 0xA5, 0xA5: 0xA4, 0x5B: 0x5C, 0x5C: 0x5B}
	t, ok := twin[vk]
	return ok && w.keysDown[t]
}

// keyID converts a virtual key to a Barrier KeyID. Printable keys become
// the character they type with the current Shift and Caps Lock state (not
// Ctrl, which would turn letters into control codes), so the client can
// reproduce them on any keyboard layout.
func (w *winScreen) keyID(vk, scan uint32, ext bool) uint16 {
	if vk == 0x0D && ext {
		return protocol.KeyKPEnter
	}
	if id, ok := vkToKey[vk]; ok {
		return id
	}
	switch {
	case vk >= 0x60 && vk <= 0x69:
		return protocol.KeyKP0 + uint16(vk-0x60)
	case vk >= 0x70 && vk <= 0x87:
		return protocol.KeyF1 + uint16(vk-0x70)
	}
	var state [256]byte
	if w.mask&protocol.ModShift != 0 {
		state[vkShift] = 0x80
	}
	if w.mask&protocol.ModCapsLock != 0 {
		state[vkCapital] = 0x01
	}
	if w.mask&protocol.ModAltGr != 0 || (w.mask&protocol.ModControl != 0 && w.keysDown[0xA5]) {
		state[vkControl], state[vkMenu] = 0x80, 0x80
	}
	var buf [8]uint16
	// Flag 4: do not change the keyboard state, so dead keys still work in
	// the application that receives this key.
	n, _, _ := pToUnicodeEx.Call(uintptr(vk), uintptr(scan), uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 4, foregroundLayout())
	if int32(n) >= 1 && buf[0] >= 0x20 {
		return buf[0]
	}
	// Dead keys and anything else unprintable: fall back to the unshifted
	// character the key is labelled with.
	c, _, _ := pMapVirtualKeyW.Call(uintptr(vk), 2) // MAPVK_VK_TO_CHAR
	c &= 0x7fffffff
	if c >= 0x20 && c < 0xE000 {
		return uint16(unicodeLower(rune(c)))
	}
	return protocol.KeyNone
}

func unicodeLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 'a' - 'A'
	}
	return r
}

func foregroundLayout() uintptr {
	fg, _, _ := pGetForegroundWindow.Call()
	tid := uintptr(0)
	if fg != 0 {
		tid, _, _ = pGetWindowThreadProcessId.Call(fg, 0)
	}
	hkl, _, _ := pGetKeyboardLayout.Call(tid)
	return hkl
}

func lockState() uint16 {
	var m uint16
	for vk, bit := range map[uintptr]uint16{vkCapital: protocol.ModCapsLock, vkNumLock: protocol.ModNumLock, vkScroll: protocol.ModScrollLock} {
		if s, _, _ := pGetKeyState.Call(vk); s&1 != 0 {
			m |= bit
		}
	}
	return m
}

var vkToKey = map[uint32]uint16{
	0x08: protocol.KeyBackSpace, 0x09: protocol.KeyTab, 0x0C: protocol.KeyClear, 0x0D: protocol.KeyReturn,
	0x13: protocol.KeyPause, 0x14: protocol.KeyCapsLock, 0x1B: protocol.KeyEscape,
	0x21: protocol.KeyPageUp, 0x22: protocol.KeyPageDown, 0x23: protocol.KeyEnd, 0x24: protocol.KeyHome,
	0x25: protocol.KeyLeftArrow, 0x26: protocol.KeyUpArrow, 0x27: protocol.KeyRightArrow, 0x28: protocol.KeyDownArrow,
	0x29: protocol.KeySelect, 0x2A: protocol.KeyPrint, 0x2B: protocol.KeyExecute, 0x2C: protocol.KeyPrint,
	0x2D: protocol.KeyInsert, 0x2E: protocol.KeyDelete, 0x2F: protocol.KeyHelp,
	0x5B: protocol.KeySuperL, 0x5C: protocol.KeySuperR, 0x5D: protocol.KeyMenu,
	0x6A: protocol.KeyKPMultiply, 0x6B: protocol.KeyKPAdd, 0x6C: protocol.KeyKPSeparator,
	0x6D: protocol.KeyKPSubtract, 0x6E: protocol.KeyKPDecimal, 0x6F: protocol.KeyKPDivide,
	0x90: protocol.KeyNumLock, 0x91: protocol.KeyScrollLock,
	0xA0: protocol.KeyShiftL, 0xA1: protocol.KeyShiftR, 0x10: protocol.KeyShiftL,
	0xA2: protocol.KeyControlL, 0xA3: protocol.KeyControlR, 0x11: protocol.KeyControlL,
	0xA4: protocol.KeyAltL, 0xA5: protocol.KeyAltR, 0x12: protocol.KeyAltL,
	0xAD: protocol.KeyAudioMute, 0xAE: protocol.KeyAudioDown, 0xAF: protocol.KeyAudioUp,
	0xB0: protocol.KeyAudioNext, 0xB1: protocol.KeyAudioPrev, 0xB2: protocol.KeyAudioStop, 0xB3: protocol.KeyAudioPlay,
}

// keyToVK is the reverse of vkToKey for injection, with the extended-key
// flag SendInput needs for the grey navigation keys and right modifiers.
var keyToVK = func() map[uint16]struct {
	vk  uint16
	ext bool
} {
	m := map[uint16]struct {
		vk  uint16
		ext bool
	}{}
	for vk, id := range vkToKey {
		if vk == 0x10 || vk == 0x11 || vk == 0x12 || vk == 0x2A {
			continue // prefer the sided variants and VK_SNAPSHOT
		}
		ext := false
		switch vk {
		case 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2C, 0x2D, 0x2E,
			0x5B, 0x5C, 0x5D, 0x6F, 0x90, 0xA3, 0xA5:
			ext = true
		}
		m[id] = struct {
			vk  uint16
			ext bool
		}{uint16(vk), ext}
	}
	m[protocol.KeyKPEnter] = struct {
		vk  uint16
		ext bool
	}{0x0D, true}
	for i := uint16(0); i < 10; i++ {
		m[protocol.KeyKP0+i] = struct {
			vk  uint16
			ext bool
		}{0x60 + i, false}
	}
	for i := uint16(0); i < 24; i++ {
		m[protocol.KeyF1+i] = struct {
			vk  uint16
			ext bool
		}{0x70 + i, false}
	}
	return m
}()

// keyInputs builds the key-down inputs for id and the matching key-ups.
func keyInputs(id, mask uint16) (down, up []input) {
	vkEvent := func(vk uint16, ext bool) {
		scan, _, _ := pMapVirtualKeyW.Call(uintptr(vk), 0) // MAPVK_VK_TO_VSC
		f := uint32(0)
		if ext {
			f = keyExtended
		}
		down = append(down, keyInput(vk, uint16(scan), f))
		up = append(up, keyInput(vk, uint16(scan), f|keyUp))
	}
	if k, ok := keyToVK[id]; ok {
		vkEvent(k.vk, k.ext)
		return
	}
	if id == protocol.KeyNone || id >= 0xE000 {
		return
	}
	// A character: use the key that types it if that needs exactly the
	// Shift state the server already gave us; otherwise type the code point.
	r, _, _ := pVkKeyScanExW.Call(uintptr(id), foregroundLayout())
	if int16(r) != -1 {
		vk, shift := uint16(r&0xff), r&0x100 != 0
		caps := mask&protocol.ModCapsLock != 0 && vk >= 'A' && vk <= 'Z'
		// Letters come out shifted when exactly one of Shift and Caps Lock
		// is on; other keys ignore Caps Lock.
		shifted := (mask&protocol.ModShift != 0) != caps
		if shift == shifted && r&0x600 == 0 {
			vkEvent(vk, false)
			return
		}
	}
	down = append(down, keyInput(0, id, keyUnicode))
	up = append(up, keyInput(0, id, keyUnicode|keyUp))
	return
}

func (w *winScreen) Shape() Rect {
	m := func(i uintptr) int { v, _, _ := pGetSystemMetrics.Call(i); return int(int32(v)) }
	return Rect{X: m(smXVirtualScreen), Y: m(smYVirtualScreen), W: m(smCXVirtualScreen), H: m(smCYVirtualScreen)}
}

func (w *winScreen) CursorPos() (int, int) {
	var p point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return int(p.X), int(p.Y)
}

func (w *winScreen) Events() <-chan Event { return w.events }

func (w *winScreen) Capture(on bool) {
	if on {
		x, y := w.Shape().Center()
		w.parkX.Store(int32(x))
		w.parkY.Store(int32(y))
		pSetCursorPos.Call(uintptr(x), uintptr(y))
	}
	w.captured.Store(on)
}

func (w *winScreen) Enter(x, y int, mask uint16) {
	pSetCursorPos.Call(uintptr(x), uintptr(y))
	// Match the server's lock keys so typing does not come out inverted.
	have := lockState()
	for vk, bit := range map[uint16]uint16{vkCapital: protocol.ModCapsLock, vkNumLock: protocol.ModNumLock} {
		if have&bit != mask&bit {
			sendInput(keyInput(vk, 0, 0), keyInput(vk, 0, keyUp))
		}
	}
}

// Leave releases every key still held, so nothing sticks down while the
// user types on another screen.
func (w *winScreen) Leave() {
	w.injMu.Lock()
	defer w.injMu.Unlock()
	for b, ups := range w.pressed {
		sendInput(ups...)
		delete(w.pressed, b)
	}
}

func (w *winScreen) MouseMove(x, y int) { pSetCursorPos.Call(uintptr(x), uintptr(y)) }

func (w *winScreen) MouseRelMove(dx, dy int) {
	sendInput(input{typ: inputMouse, mi: mouseInput{dx: int32(dx), dy: int32(dy), flags: mouseMove}})
}

func (w *winScreen) MouseButton(b uint8, down bool) {
	var f, data uint32
	switch b {
	case protocol.ButtonLeft:
		f = mouseLeftDown
	case protocol.ButtonRight:
		f = mouseRightDown
	case protocol.ButtonMiddle:
		f = mouseMiddleDown
	case protocol.ButtonExtra0, protocol.ButtonExtra1:
		f, data = mouseXDown, uint32(b-protocol.ButtonExtra0+1)
	default:
		return
	}
	if !down {
		f <<= 1 // every *UP flag is its *DOWN flag shifted left once
	}
	sendInput(input{typ: inputMouse, mi: mouseInput{mouseData: data, flags: f}})
}

func (w *winScreen) MouseWheel(dx, dy int) {
	if dy != 0 {
		sendInput(input{typ: inputMouse, mi: mouseInput{mouseData: uint32(int32(dy)), flags: mouseWheel}})
	}
	if dx != 0 {
		sendInput(input{typ: inputMouse, mi: mouseInput{mouseData: uint32(int32(dx)), flags: mouseHWheel}})
	}
}

func (w *winScreen) KeyDown(id, mask, button uint16) {
	down, up := keyInputs(id, mask)
	if len(down) == 0 {
		return
	}
	w.injMu.Lock()
	if old, ok := w.pressed[button]; ok {
		sendInput(old...)
	}
	w.pressed[button] = up
	w.injMu.Unlock()
	sendInput(down...)
}

func (w *winScreen) KeyRepeat(id, mask, count, button uint16) {
	down, _ := keyInputs(id, mask)
	for range min(count, 64) {
		sendInput(down...)
	}
}

func (w *winScreen) KeyUp(id, mask, button uint16) {
	w.injMu.Lock()
	up, ok := w.pressed[button]
	delete(w.pressed, button)
	w.injMu.Unlock()
	if !ok {
		_, up = keyInputs(id, mask)
	}
	sendInput(up...)
}

func (w *winScreen) ScreenSaver(on bool) {
	if on {
		target, _, _ := pGetForegroundWindow.Call()
		if target == 0 {
			target, _, _ = pGetDesktopWindow.Call()
		}
		pPostMessageW.Call(target, wmSysCommand, scScreenSave, 0)
		return
	}
	// Any real input ends the screensaver; a one-pixel wiggle is invisible.
	sendInput(
		input{typ: inputMouse, mi: mouseInput{dx: 1, flags: mouseMove}},
		input{typ: inputMouse, mi: mouseInput{dx: -1, flags: mouseMove}},
	)
}

func clipboardSeq() uint32 {
	s, _, _ := pGetClipboardSequenceNumber.Call()
	return uint32(s)
}

// openClipboard retries briefly: another program may hold it.
func (w *winScreen) openClipboard() bool {
	for range 20 {
		if r, _, _ := pOpenClipboard.Call(w.hwnd); r != 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (w *winScreen) Clipboard(id uint8) (text string, ok bool) {
	if id != protocol.ClipboardMain {
		return "", false
	}
	w.onThread(func() {
		if !w.openClipboard() {
			return
		}
		defer pCloseClipboard.Call()
		h, _, _ := pGetClipboardData.Call(cfUnicodeText)
		if h == 0 {
			return
		}
		p, _, _ := pGlobalLock.Call(h)
		if p == 0 {
			return
		}
		defer pGlobalUnlock.Call(h)
		var u []uint16
		for i := uintptr(0); i < protocol.MaxMessageSize; i++ {
			c := *fromAddr[uint16](p + 2*i)
			if c == 0 {
				break
			}
			u = append(u, c)
		}
		text, ok = strings.ReplaceAll(string(utf16.Decode(u)), "\r\n", "\n"), true
	})
	return text, ok
}

func (w *winScreen) SetClipboard(id uint8, text string) {
	if id != protocol.ClipboardMain {
		return
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	u := utf16.Encode([]rune(text + "\x00"))
	w.clipMu.Lock()
	defer w.clipMu.Unlock()
	w.onThread(func() {
		if !w.openClipboard() {
			return
		}
		defer pCloseClipboard.Call()
		pEmptyClipboard.Call()
		h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(len(u)*2))
		if h == 0 {
			return
		}
		p, _, _ := pGlobalLock.Call(h)
		if p == 0 {
			pGlobalFree.Call(h)
			return
		}
		copy(unsafe.Slice(fromAddr[uint16](p), len(u)), u)
		pGlobalUnlock.Call(h)
		if r, _, _ := pSetClipboardData.Call(cfUnicodeText, h); r == 0 {
			pGlobalFree.Call(h)
		}
	})
	w.clipSeq = clipboardSeq() // our own change is not news
}

// poll watches the clipboard and the screensaver.
func (w *winScreen) poll() {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	saver := false
	for n := 0; ; n++ {
		select {
		case <-w.stopPoll:
			return
		case <-t.C:
		}
		w.clipMu.Lock()
		if s := clipboardSeq(); s != w.clipSeq {
			w.clipSeq = s
			w.emit(Event{Kind: ClipboardChanged, ClipboardID: protocol.ClipboardMain})
		}
		w.clipMu.Unlock()
		if w.opts.Capture && n%4 == 0 {
			var running int32
			pSystemParametersInfoW.Call(spiGetScreenSaverRunning, 0, uintptr(unsafe.Pointer(&running)), 0)
			if on := running != 0; on != saver {
				saver = on
				w.emit(Event{Kind: ScreenSaver, On: on})
			}
		}
	}
}

func (w *winScreen) Close() error {
	w.once.Do(func() {
		close(w.stopPoll)
		w.captured.Store(false)
		w.Leave()
		pPostThreadMessageW.Call(uintptr(w.thread), wmQuit, 0, 0)
		<-w.stopped
		hooked.CompareAndSwap(w, nil)
	})
	return nil
}
