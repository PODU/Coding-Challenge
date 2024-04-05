package xxd

import (
	"bufio"
	"io"
)

// Dump writes a hex dump of r to w. Input is read one line's worth at a time.
func Dump(r io.Reader, w io.Writer, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	maxOffset, err := skipSeek(r, cfg.Seek)
	if err != nil {
		return err
	}

	src := io.Reader(r)
	if cfg.Length >= 0 {
		src = io.LimitReader(r, cfg.Length)
		if end := cfg.Seek + cfg.Length; end > maxOffset {
			maxOffset = end
		}
	}

	br := bufio.NewReader(src)
	bw := bufio.NewWriter(w)

	if cfg.Plain {
		if err := dumpPlain(br, bw, cfg); err != nil {
			return err
		}
		return bw.Flush()
	}

	width := offsetWidth(maxOffset)
	buf := make([]byte, cfg.Cols)
	line := make([]byte, 0, width+2+hexFieldWidth(cfg)+2+cfg.Cols+1)
	offset := cfg.Seek

	for {
		n, err := io.ReadFull(br, buf)
		if n > 0 {
			line = line[:0]
			line = appendOffset(line, offset, width)
			line = append(line, ':', ' ')
			line = appendHexField(line, buf[:n], cfg)
			line = append(line, ' ', ' ')
			line = appendASCII(line, buf[:n])
			line = append(line, '\n')
			if _, werr := bw.Write(line); werr != nil {
				return werr
			}
			offset += int64(n)
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return bw.Flush()
}

// dumpPlain writes the -p format: just hex, cfg.Cols bytes per line.
func dumpPlain(r io.Reader, w *bufio.Writer, cfg Config) error {
	buf := make([]byte, cfg.Cols)
	line := make([]byte, 0, cfg.Cols*2+1)
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			line = line[:0]
			for _, b := range buf[:n] {
				line = appendByteHex(line, b)
			}
			line = append(line, '\n')
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// skipSeek advances r by seek bytes, seeking if it can and otherwise reading
// and discarding (pipes, stdin). It returns the new offset.
func skipSeek(r io.Reader, seek int64) (int64, error) {
	if seek <= 0 {
		return seek, nil
	}
	if s, ok := r.(io.Seeker); ok {
		if _, err := s.Seek(seek, io.SeekStart); err == nil {
			return seek, nil
		}
	}
	// Hitting EOF early is fine; there is just nothing left to dump.
	if _, err := io.CopyN(io.Discard, r, seek); err != nil && err != io.EOF {
		return seek, err
	}
	return seek, nil
}
