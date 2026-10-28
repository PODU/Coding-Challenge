package protocol

// Barrier KeyIDs (key_types.h). Printable keys are sent as their Unicode
// code point; everything else uses X11 keysyms with 0xFF.. remapped to 0xEF..
const (
	KeyNone        uint16 = 0x0000
	KeyBackSpace   uint16 = 0xEF08
	KeyTab         uint16 = 0xEF09
	KeyClear       uint16 = 0xEF0B
	KeyReturn      uint16 = 0xEF0D
	KeyPause       uint16 = 0xEF13
	KeyScrollLock  uint16 = 0xEF14
	KeySysReq      uint16 = 0xEF15
	KeyEscape      uint16 = 0xEF1B
	KeyHome        uint16 = 0xEF50
	KeyLeftArrow   uint16 = 0xEF51
	KeyUpArrow     uint16 = 0xEF52
	KeyRightArrow  uint16 = 0xEF53
	KeyDownArrow   uint16 = 0xEF54
	KeyPageUp      uint16 = 0xEF55
	KeyPageDown    uint16 = 0xEF56
	KeyEnd         uint16 = 0xEF57
	KeySelect      uint16 = 0xEF60
	KeyPrint       uint16 = 0xEF61
	KeyExecute     uint16 = 0xEF62
	KeyInsert      uint16 = 0xEF63
	KeyMenu        uint16 = 0xEF67
	KeyHelp        uint16 = 0xEF6A
	KeyBreak       uint16 = 0xEF6B
	KeyNumLock     uint16 = 0xEF7F
	KeyKPEnter     uint16 = 0xEF8D
	KeyKPMultiply  uint16 = 0xEFAA
	KeyKPAdd       uint16 = 0xEFAB
	KeyKPSeparator uint16 = 0xEFAC
	KeyKPSubtract  uint16 = 0xEFAD
	KeyKPDecimal   uint16 = 0xEFAE
	KeyKPDivide    uint16 = 0xEFAF
	KeyKP0         uint16 = 0xEFB0 // through KeyKP0+9
	KeyF1          uint16 = 0xEFBE // through KeyF1+23 (F24)
	KeyShiftL      uint16 = 0xEFE1
	KeyShiftR      uint16 = 0xEFE2
	KeyControlL    uint16 = 0xEFE3
	KeyControlR    uint16 = 0xEFE4
	KeyCapsLock    uint16 = 0xEFE5
	KeyMetaL       uint16 = 0xEFE7
	KeyMetaR       uint16 = 0xEFE8
	KeyAltL        uint16 = 0xEFE9
	KeyAltR        uint16 = 0xEFEA
	KeySuperL      uint16 = 0xEFEB
	KeySuperR      uint16 = 0xEFEC
	KeyDelete      uint16 = 0xEFFF

	KeyAudioMute uint16 = 0xE0AD
	KeyAudioDown uint16 = 0xE0AE
	KeyAudioUp   uint16 = 0xE0AF
	KeyAudioNext uint16 = 0xE0B0
	KeyAudioPrev uint16 = 0xE0B1
	KeyAudioStop uint16 = 0xE0B2
	KeyAudioPlay uint16 = 0xE0B3
)

// Modifier mask bits (KeyModifierMask).
const (
	ModShift      uint16 = 0x0001
	ModControl    uint16 = 0x0002
	ModAlt        uint16 = 0x0004
	ModMeta       uint16 = 0x0008
	ModSuper      uint16 = 0x0010
	ModAltGr      uint16 = 0x0020
	ModLevel5Lock uint16 = 0x0040
	ModCapsLock   uint16 = 0x1000
	ModNumLock    uint16 = 0x2000
	ModScrollLock uint16 = 0x4000
)

// Mouse buttons (ButtonID).
const (
	ButtonLeft   uint8 = 1
	ButtonMiddle uint8 = 2
	ButtonRight  uint8 = 3
	ButtonExtra0 uint8 = 4 // "back"
	ButtonExtra1 uint8 = 5 // "forward"
)

// WheelDelta is one notch of a mouse wheel, in DMWM units.
const WheelDelta = 120

// ModifierFor returns the mask bit a modifier key controls, or 0 if id is
// not a modifier. Lock keys are toggles and are reported separately.
func ModifierFor(id uint16) uint16 {
	switch id {
	case KeyShiftL, KeyShiftR:
		return ModShift
	case KeyControlL, KeyControlR:
		return ModControl
	case KeyAltL, KeyAltR:
		return ModAlt
	case KeyMetaL, KeyMetaR:
		return ModMeta
	case KeySuperL, KeySuperR:
		return ModSuper
	}
	return 0
}
