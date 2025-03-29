package tracer

// MemReader reads C strings from the traced process.
//
// ReadString reads up to max bytes at addr, stopping at a NUL. ok is false if
// the memory can't be read, in which case the caller prints the pointer.
type MemReader interface {
	ReadString(addr uint64, max int) (string, bool)
}

// NopMem is a MemReader that never succeeds.
type NopMem struct{}

// ReadString always reports failure.
func (NopMem) ReadString(uint64, int) (string, bool) { return "", false }
