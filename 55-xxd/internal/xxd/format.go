package xxd

const hexDigits = "0123456789abcdef"

func appendByteHex(dst []byte, b byte) []byte {
	return append(dst, hexDigits[b>>4], hexDigits[b&0x0f])
}

// hexFieldWidth is the width of the hex column on a full line: two characters
// per byte plus a space between groups.
func hexFieldWidth(cfg Config) int {
	return cfg.Cols*2 + (cfg.Cols-1)/cfg.effectiveGroup()
}

// appendHexField appends the hex column for one line. Short lines are padded
// to hexFieldWidth so the ASCII column stays aligned.
func appendHexField(dst []byte, data []byte, cfg Config) []byte {
	gs := cfg.effectiveGroup()
	n := len(data)

	for g := 0; g < cfg.Cols; g += gs {
		if g > 0 {
			dst = append(dst, ' ')
		}
		end := g + gs
		if end > cfg.Cols {
			end = cfg.Cols
		}
		if cfg.LittleEndian {
			// Missing bytes end up as leading spaces in the group.
			for k := end - 1; k >= g; k-- {
				if k < n {
					dst = appendByteHex(dst, data[k])
				} else {
					dst = append(dst, ' ', ' ')
				}
			}
		} else {
			for k := g; k < end; k++ {
				if k < n {
					dst = appendByteHex(dst, data[k])
				} else {
					dst = append(dst, ' ', ' ')
				}
			}
		}
	}
	return dst
}

// appendASCII appends the text column: printable ASCII as is, anything else
// as '.'.
func appendASCII(dst []byte, data []byte) []byte {
	for _, b := range data {
		if b >= 0x20 && b <= 0x7e {
			dst = append(dst, b)
		} else {
			dst = append(dst, '.')
		}
	}
	return dst
}

// appendOffset appends off as zero-padded hex, at least width digits.
func appendOffset(dst []byte, off int64, width int) []byte {
	var tmp [16]byte
	i := len(tmp)
	v := uint64(off)
	for {
		i--
		tmp[i] = hexDigits[v&0x0f]
		v >>= 4
		if v == 0 {
			break
		}
	}
	for n := len(tmp) - i; n < width; n++ {
		dst = append(dst, '0')
	}
	return append(dst, tmp[i:]...)
}

// offsetWidth is the number of hex digits needed for maxOffset, minimum 8.
func offsetWidth(maxOffset int64) int {
	w := 0
	v := uint64(maxOffset)
	for v > 0 {
		w++
		v >>= 4
	}
	if w < 8 {
		w = 8
	}
	return w
}
