package protocol

import (
	"bytes"
	"fmt"
)

// Protocol version spoken by this implementation (Barrier 2.x).
const (
	MajorVersion = 1
	MinorVersion = 6
)

// Format strings, verbatim from Barrier's protocol_types.cpp.
const (
	FmtHello        = "Barrier%2i%2i"
	FmtHelloBack    = "Barrier%2i%2i%s"
	FmtNoop         = "CNOP"
	FmtClose        = "CBYE"
	FmtEnter        = "CINN%2i%2i%4i%2i"
	FmtLeave        = "COUT"
	FmtGrab         = "CCLP%1i%4i"
	FmtScreenSaver  = "CSEC%1i"
	FmtResetOptions = "CROP"
	FmtInfoAck      = "CIAK"
	FmtKeepAlive    = "CALV"
	FmtKeyDown      = "DKDN%2i%2i%2i"
	FmtKeyRepeat    = "DKRP%2i%2i%2i%2i"
	FmtKeyUp        = "DKUP%2i%2i%2i"
	FmtMouseDown    = "DMDN%1i"
	FmtMouseUp      = "DMUP%1i"
	FmtMouseMove    = "DMMV%2i%2i"
	FmtMouseRelMove = "DMRM%2i%2i"
	FmtMouseWheel   = "DMWM%2i%2i"
	FmtClipboard    = "DCLP%1i%4i%1i%s"
	FmtInfo         = "DINF%2i%2i%2i%2i%2i%2i%2i"
	FmtSetOptions   = "DSOP%4I"
	FmtFileTransfer = "DFTR%1i%s"
	FmtDragInfo     = "DDRG%2i%s"
	FmtQueryInfo    = "QINF"
	FmtIncompatible = "EICV%2i%2i"
	FmtBusy         = "EBSY"
	FmtUnknown      = "EUNK"
	FmtBad          = "EBAD"
)

// Message is one decoded protocol message.
type Message interface {
	// Code is the 4-character message code, or "Barrier" for the greetings.
	Code() string
	Marshal() []byte
}

// Greetings.
type (
	Hello     struct{ Major, Minor uint16 }
	HelloBack struct {
		Major, Minor uint16
		Name         string
	}
)

// Commands (C*), queries (Q*) and errors (E*) without payloads.
type (
	Noop         struct{}
	Close        struct{}
	Leave        struct{}
	ResetOptions struct{}
	InfoAck      struct{}
	KeepAlive    struct{}
	QueryInfo    struct{}
	Busy         struct{}
	Unknown      struct{}
	Bad          struct{}
)

// Enter tells a screen the cursor has arrived at (X, Y). Seq numbers every
// entry so stale clipboard grabs can be told apart. Mask is the modifier
// state at the moment of entry.
type Enter struct {
	X, Y int16
	Seq  uint32
	Mask uint16
}

// Grab announces that a screen took ownership of a clipboard (CCLP).
type Grab struct {
	ID  uint8
	Seq uint32
}

// ScreenSaver reports the server's screensaver starting or stopping.
type ScreenSaver struct{ On bool }

// Keyboard events. ID is a Barrier KeyID, Mask a modifier mask and Button
// the sender's physical key code (used only to pair downs with ups).
type (
	KeyDown   struct{ ID, Mask, Button uint16 }
	KeyUp     struct{ ID, Mask, Button uint16 }
	KeyRepeat struct{ ID, Mask, Count, Button uint16 }
)

// Mouse events.
type (
	MouseDown    struct{ Button uint8 }
	MouseUp      struct{ Button uint8 }
	MouseMove    struct{ X, Y int16 }
	MouseRelMove struct{ DX, DY int16 }
	MouseWheel   struct{ DX, DY int16 }
)

// ClipboardData is one chunk of a DCLP transfer (see ChunkClipboard).
type ClipboardData struct {
	ID   uint8
	Seq  uint32
	Mark uint8
	Data string
}

// Info describes a client screen (DINF): its rectangle, the obsolete warp
// zone size (always 0) and where the cursor is.
type Info struct {
	X, Y, W, H, Warp, MX, MY int16
}

// SetOptions carries (option, value) pairs flattened into one list.
type SetOptions struct{ Options []uint32 }

// FileTransfer and DragInfo are parsed so they can be logged and skipped.
type (
	FileTransfer struct {
		Mark uint8
		Data string
	}
	DragInfo struct {
		Count uint16
		Data  string
	}
)

// Incompatible (EICV) carries the version of the side that refused.
type Incompatible struct{ Major, Minor uint16 }

func (Hello) Code() string        { return "Barrier" }
func (HelloBack) Code() string    { return "Barrier" }
func (Noop) Code() string         { return "CNOP" }
func (Close) Code() string        { return "CBYE" }
func (Enter) Code() string        { return "CINN" }
func (Leave) Code() string        { return "COUT" }
func (Grab) Code() string         { return "CCLP" }
func (ScreenSaver) Code() string  { return "CSEC" }
func (ResetOptions) Code() string { return "CROP" }
func (InfoAck) Code() string      { return "CIAK" }
func (KeepAlive) Code() string    { return "CALV" }
func (KeyDown) Code() string      { return "DKDN" }
func (KeyRepeat) Code() string    { return "DKRP" }
func (KeyUp) Code() string        { return "DKUP" }
func (MouseDown) Code() string    { return "DMDN" }
func (MouseUp) Code() string      { return "DMUP" }
func (MouseMove) Code() string    { return "DMMV" }
func (MouseRelMove) Code() string { return "DMRM" }
func (MouseWheel) Code() string   { return "DMWM" }
func (ClipboardData) Code() string {
	return "DCLP"
}
func (Info) Code() string         { return "DINF" }
func (SetOptions) Code() string   { return "DSOP" }
func (FileTransfer) Code() string { return "DFTR" }
func (DragInfo) Code() string     { return "DDRG" }
func (QueryInfo) Code() string    { return "QINF" }
func (Incompatible) Code() string { return "EICV" }
func (Busy) Code() string         { return "EBSY" }
func (Unknown) Code() string      { return "EUNK" }
func (Bad) Code() string          { return "EBAD" }

func (m Hello) Marshal() []byte     { return MustPack(FmtHello, m.Major, m.Minor) }
func (m HelloBack) Marshal() []byte { return MustPack(FmtHelloBack, m.Major, m.Minor, m.Name) }
func (Noop) Marshal() []byte        { return []byte(FmtNoop) }
func (Close) Marshal() []byte       { return []byte(FmtClose) }
func (m Enter) Marshal() []byte     { return MustPack(FmtEnter, m.X, m.Y, m.Seq, m.Mask) }
func (Leave) Marshal() []byte       { return []byte(FmtLeave) }
func (m Grab) Marshal() []byte      { return MustPack(FmtGrab, m.ID, m.Seq) }
func (m ScreenSaver) Marshal() []byte {
	return MustPack(FmtScreenSaver, m.On)
}
func (ResetOptions) Marshal() []byte   { return []byte(FmtResetOptions) }
func (InfoAck) Marshal() []byte        { return []byte(FmtInfoAck) }
func (KeepAlive) Marshal() []byte      { return []byte(FmtKeepAlive) }
func (m KeyDown) Marshal() []byte      { return MustPack(FmtKeyDown, m.ID, m.Mask, m.Button) }
func (m KeyRepeat) Marshal() []byte    { return MustPack(FmtKeyRepeat, m.ID, m.Mask, m.Count, m.Button) }
func (m KeyUp) Marshal() []byte        { return MustPack(FmtKeyUp, m.ID, m.Mask, m.Button) }
func (m MouseDown) Marshal() []byte    { return MustPack(FmtMouseDown, m.Button) }
func (m MouseUp) Marshal() []byte      { return MustPack(FmtMouseUp, m.Button) }
func (m MouseMove) Marshal() []byte    { return MustPack(FmtMouseMove, m.X, m.Y) }
func (m MouseRelMove) Marshal() []byte { return MustPack(FmtMouseRelMove, m.DX, m.DY) }
func (m MouseWheel) Marshal() []byte   { return MustPack(FmtMouseWheel, m.DX, m.DY) }
func (m ClipboardData) Marshal() []byte {
	return MustPack(FmtClipboard, m.ID, m.Seq, m.Mark, m.Data)
}
func (m Info) Marshal() []byte {
	return MustPack(FmtInfo, m.X, m.Y, m.W, m.H, m.Warp, m.MX, m.MY)
}
func (m SetOptions) Marshal() []byte {
	opts := m.Options
	if opts == nil {
		opts = []uint32{}
	}
	return MustPack(FmtSetOptions, opts)
}
func (m FileTransfer) Marshal() []byte { return MustPack(FmtFileTransfer, m.Mark, m.Data) }
func (m DragInfo) Marshal() []byte     { return MustPack(FmtDragInfo, m.Count, m.Data) }
func (QueryInfo) Marshal() []byte      { return []byte(FmtQueryInfo) }
func (m Incompatible) Marshal() []byte { return MustPack(FmtIncompatible, m.Major, m.Minor) }
func (Busy) Marshal() []byte           { return []byte(FmtBusy) }
func (Unknown) Marshal() []byte        { return []byte(FmtUnknown) }
func (Bad) Marshal() []byte            { return []byte(FmtBad) }

// Parse decodes one framed payload. A greeting with a name is a HelloBack,
// one without is a Hello.
func Parse(b []byte) (Message, error) {
	if bytes.HasPrefix(b, []byte("Barrier")) {
		if len(b) > 11 {
			v, err := Unpack(b, FmtHelloBack)
			if err != nil {
				return nil, fmt.Errorf("HelloBack: %w", err)
			}
			return HelloBack{u16(v[0]), u16(v[1]), v[2].(string)}, nil
		}
		v, err := Unpack(b, FmtHello)
		if err != nil {
			return nil, fmt.Errorf("Hello: %w", err)
		}
		return Hello{u16(v[0]), u16(v[1])}, nil
	}
	if len(b) < 4 {
		return nil, fmt.Errorf("protocol: %d-byte message has no code", len(b))
	}
	code := string(b[:4])
	f, ok := parsers[code]
	if !ok {
		return nil, &UnknownMessageError{Code: code}
	}
	v, err := Unpack(b, f.format)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", code, err)
	}
	return f.build(v), nil
}

// UnknownMessageError reports a well-framed message with a code this
// implementation does not know. The stream is still in sync, so callers
// can log it and carry on.
type UnknownMessageError struct{ Code string }

func (e *UnknownMessageError) Error() string {
	return fmt.Sprintf("protocol: unknown message %q", e.Code)
}

type parser struct {
	format string
	build  func([]any) Message
}

var parsers = map[string]parser{
	"CNOP": {FmtNoop, func([]any) Message { return Noop{} }},
	"CBYE": {FmtClose, func([]any) Message { return Close{} }},
	"CINN": {FmtEnter, func(v []any) Message { return Enter{s16(v[0]), s16(v[1]), v[2].(uint32), u16(v[3])} }},
	"COUT": {FmtLeave, func([]any) Message { return Leave{} }},
	"CCLP": {FmtGrab, func(v []any) Message { return Grab{u8(v[0]), v[1].(uint32)} }},
	"CSEC": {FmtScreenSaver, func(v []any) Message { return ScreenSaver{v[0].(uint32) != 0} }},
	"CROP": {FmtResetOptions, func([]any) Message { return ResetOptions{} }},
	"CIAK": {FmtInfoAck, func([]any) Message { return InfoAck{} }},
	"CALV": {FmtKeepAlive, func([]any) Message { return KeepAlive{} }},
	"DKDN": {FmtKeyDown, func(v []any) Message { return KeyDown{u16(v[0]), u16(v[1]), u16(v[2])} }},
	"DKRP": {FmtKeyRepeat, func(v []any) Message { return KeyRepeat{u16(v[0]), u16(v[1]), u16(v[2]), u16(v[3])} }},
	"DKUP": {FmtKeyUp, func(v []any) Message { return KeyUp{u16(v[0]), u16(v[1]), u16(v[2])} }},
	"DMDN": {FmtMouseDown, func(v []any) Message { return MouseDown{u8(v[0])} }},
	"DMUP": {FmtMouseUp, func(v []any) Message { return MouseUp{u8(v[0])} }},
	"DMMV": {FmtMouseMove, func(v []any) Message { return MouseMove{s16(v[0]), s16(v[1])} }},
	"DMRM": {FmtMouseRelMove, func(v []any) Message { return MouseRelMove{s16(v[0]), s16(v[1])} }},
	"DMWM": {FmtMouseWheel, func(v []any) Message { return MouseWheel{s16(v[0]), s16(v[1])} }},
	"DCLP": {FmtClipboard, func(v []any) Message {
		return ClipboardData{u8(v[0]), v[1].(uint32), u8(v[2]), v[3].(string)}
	}},
	"DINF": {FmtInfo, func(v []any) Message {
		return Info{s16(v[0]), s16(v[1]), s16(v[2]), s16(v[3]), s16(v[4]), s16(v[5]), s16(v[6])}
	}},
	"DSOP": {FmtSetOptions, func(v []any) Message { return SetOptions{v[0].([]uint32)} }},
	"DFTR": {FmtFileTransfer, func(v []any) Message { return FileTransfer{u8(v[0]), v[1].(string)} }},
	"DDRG": {FmtDragInfo, func(v []any) Message { return DragInfo{u16(v[0]), v[1].(string)} }},
	"QINF": {FmtQueryInfo, func([]any) Message { return QueryInfo{} }},
	"EICV": {FmtIncompatible, func(v []any) Message { return Incompatible{u16(v[0]), u16(v[1])} }},
	"EBSY": {FmtBusy, func([]any) Message { return Busy{} }},
	"EUNK": {FmtUnknown, func([]any) Message { return Unknown{} }},
	"EBAD": {FmtBad, func([]any) Message { return Bad{} }},
}

func u8(a any) uint8   { return uint8(a.(uint32)) }
func u16(a any) uint16 { return uint16(a.(uint32)) }
func s16(a any) int16  { return int16(uint16(a.(uint32))) }
