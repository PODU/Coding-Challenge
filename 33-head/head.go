package main

import (
	"bufio"
	"errors"
	"io"
)

// copyFirstLines writes the first n lines of r. A last line with no trailing
// newline still counts.
func copyFirstLines(w io.Writer, r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	br := bufio.NewReader(r)
	var emitted int64
	for emitted < n {
		chunk, err := br.ReadBytes('\n')
		if len(chunk) > 0 {
			if _, werr := w.Write(chunk); werr != nil {
				return werr
			}
			emitted++
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	return nil
}

// copyFirstBytes writes the first n bytes of r. Hitting EOF early is fine.
func copyFirstBytes(w io.Writer, r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	_, err := io.CopyN(w, r, n)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// copyAllButLastLines writes every line of r except the last k, holding at
// most k lines in memory.
func copyAllButLastLines(w io.Writer, r io.Reader, k int64) error {
	if k <= 0 {
		_, err := io.Copy(w, r)
		return err
	}
	br := bufio.NewReader(r)
	pending := make([][]byte, 0, k+1)
	for {
		chunk, err := br.ReadBytes('\n')
		if len(chunk) > 0 {
			pending = append(pending, chunk)
			if int64(len(pending)) > k {
				oldest := pending[0]
				pending = pending[1:]
				if _, werr := w.Write(oldest); werr != nil {
					return werr
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// copyAllButLastBytes writes everything except the final k bytes of r.
func copyAllButLastBytes(w io.Writer, r io.Reader, k int64) error {
	if k <= 0 {
		_, err := io.Copy(w, r)
		return err
	}
	// Keep the most recent k bytes back; anything older than that is safe to write.
	held := make([]byte, 0, k)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		held = append(held, buf[:n]...)
		if extra := int64(len(held)) - k; extra > 0 {
			if _, werr := w.Write(held[:extra]); werr != nil {
				return werr
			}
			held = append(held[:0], held[extra:]...)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
