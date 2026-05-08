package comm

import "bytes"

// compareLines orders lines byte by byte, like `LC_ALL=C sort`. With
// caseInsensitive, ASCII letters are lowercased first, as `comm -i` does in
// the C locale.
func compareLines(a, b []byte, caseInsensitive bool) int {
	if !caseInsensitive {
		return bytes.Compare(a, b)
	}

	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		ca := toLowerASCII(a[i])
		cb := toLowerASCII(b[i])
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

func toLowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
