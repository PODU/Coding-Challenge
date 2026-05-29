package main

// applyConversions applies swab and then ucase/lcase to b in place. conv=sync
// and conv=sparse are handled by the copy loop and the output writer.
func applyConversions(b []byte, c ConvFlags) {
	if c.Swab {
		swab(b)
	}
	if c.Ucase {
		ucase(b)
	}
	if c.Lcase {
		lcase(b)
	}
}

// swab swaps adjacent byte pairs. An odd trailing byte is left alone.
func swab(b []byte) {
	for i := 0; i+1 < len(b); i += 2 {
		b[i], b[i+1] = b[i+1], b[i]
	}
}

func ucase(b []byte) {
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
}

func lcase(b []byte) {
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
