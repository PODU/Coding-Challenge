package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// progressInterval is a variable so tests can shorten it.
var progressInterval = 1 * time.Second

// outWriter groups output into obs-sized records, seeks over all-NUL blocks
// for conv=sparse, and tracks the output position for the final truncate.
type outWriter struct {
	w         io.Writer
	seeker    io.Seeker // nil unless Seek actually works on w
	truncFile *os.File  // nil for stdout, which is never truncated
	obs       int64
	sparse    bool
	stats     *Stats

	buf         []byte
	pos         int64
	lastWasHole bool
}

// write buffers p and flushes every complete obs-sized record.
func (o *outWriter) write(p []byte) error {
	o.buf = append(o.buf, p...)
	for int64(len(o.buf)) >= o.obs {
		if err := o.flushBlock(o.buf[:o.obs], true); err != nil {
			return err
		}
		o.buf = o.buf[o.obs:]
	}
	return nil
}

// writeWhole writes p as one output record, as dd does when bs= is given.
func (o *outWriter) writeWhole(p []byte) error {
	return o.flushBlock(p, int64(len(p)) == o.obs)
}

func (o *outWriter) flushBlock(block []byte, full bool) error {
	n := int64(len(block))
	o.stats.addBytes(n)

	if o.sparse && o.seeker != nil && allZero(block) {
		if _, err := o.seeker.Seek(n, io.SeekCurrent); err != nil {
			return err
		}
		o.pos += n
		o.lastWasHole = true
		o.stats.recordOut(n, o.obs)
		return nil
	}

	if _, err := o.w.Write(block); err != nil {
		return err
	}
	o.pos += n
	o.lastWasHole = false
	o.stats.recordOut(n, o.obs)
	return nil
}

// finish flushes what is left in the buffer and truncates the output file.
func (o *outWriter) finish(notrunc bool) error {
	if len(o.buf) > 0 {
		if err := o.flushBlock(o.buf, int64(len(o.buf)) == o.obs); err != nil {
			return err
		}
		o.buf = nil
	}

	f := o.truncFile
	if f == nil || o.seeker == nil {
		return nil
	}
	if !notrunc {
		return f.Truncate(o.pos)
	}
	// With notrunc we never shrink the file, but a trailing sparse hole only
	// exists on disk once the file is extended to cover it.
	if o.lastWasHole {
		if st, err := f.Stat(); err == nil && st.Size() < o.pos {
			return f.Truncate(o.pos)
		}
	}
	return nil
}

// Run performs the copy described by cfg and returns the stats, the exit code
// and any I/O error.
func Run(cfg *Config, stdin io.Reader, stdout io.Writer, stderr io.Writer) (*Stats, int, error) {
	stats := NewStats()

	var in io.Reader
	var inSeeker io.Seeker
	if cfg.InFile != "" {
		f, err := os.Open(cfg.InFile)
		if err != nil {
			return stats, 2, fmt.Errorf("failed to open input %q: %w", cfg.InFile, err)
		}
		defer f.Close()
		in = f
		inSeeker = f
	} else {
		in = stdin
	}
	// A pipe is an *os.File too, so check that Seek really works.
	if s, ok := in.(io.Seeker); ok {
		if _, err := s.Seek(0, io.SeekCurrent); err == nil {
			inSeeker = s
		}
	}

	if cfg.Skip > 0 {
		off := cfg.Skip * cfg.Ibs
		if inSeeker != nil {
			if _, err := inSeeker.Seek(off, io.SeekStart); err != nil {
				return stats, 2, fmt.Errorf("seeking input: %w", err)
			}
		} else if err := discard(in, off); err != nil {
			return stats, 2, fmt.Errorf("skipping input: %w", err)
		}
	}

	var out io.Writer
	var outFile *os.File
	if cfg.OutFile != "" {
		flags := os.O_WRONLY | os.O_CREATE
		// seek= keeps the bytes before the seek point, so don't truncate on open.
		if cfg.Seek == 0 && !cfg.Conv.Notrunc {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(cfg.OutFile, flags, 0o644)
		if err != nil {
			return stats, 2, fmt.Errorf("failed to open output %q: %w", cfg.OutFile, err)
		}
		defer f.Close()
		out = f
		outFile = f
	} else {
		out = stdout
	}

	ow := &outWriter{w: out, obs: cfg.Obs, sparse: cfg.Conv.Sparse, stats: stats, truncFile: outFile}
	// Pipes and consoles are *os.File but can't seek or truncate.
	if s, ok := out.(io.Seeker); ok {
		if _, err := s.Seek(0, io.SeekCurrent); err == nil {
			ow.seeker = s
		}
	}

	if cfg.Seek > 0 {
		off := cfg.Seek * cfg.Obs
		if ow.seeker != nil {
			if _, err := ow.seeker.Seek(off, io.SeekStart); err != nil {
				return stats, 2, fmt.Errorf("seeking output: %w", err)
			}
		} else if err := writeZeros(out, off); err != nil {
			return stats, 2, fmt.Errorf("padding output: %w", err)
		}
		ow.pos = off
	}

	stopProgress := func() {}
	if cfg.Status == StatusProgress {
		done := make(chan struct{})
		go func() {
			t := time.NewTicker(progressInterval)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					fmt.Fprintf(stderr, "\r%s", trimNewline(stats.transferLine()))
				case <-done:
					return
				}
			}
		}()
		stopProgress = func() {
			close(done)
			fmt.Fprintln(stderr) // finish the line we kept rewriting with \r
		}
	}

	stopSignals := setupSignalHandler(stats, stderr)
	copyErr := copyLoop(cfg, in, ow, stats, stderr)
	stopSignals()
	stopProgress()

	if copyErr != nil {
		// Still write out whatever was copied before the error.
		_ = ow.finish(cfg.Conv.Notrunc)
		return stats, 2, copyErr
	}

	if err := ow.finish(cfg.Conv.Notrunc); err != nil {
		return stats, 2, fmt.Errorf("finishing output: %w", err)
	}

	return stats, 0, nil
}

func copyLoop(cfg *Config, in io.Reader, ow *outWriter, stats *Stats, stderr io.Writer) error {
	buf := make([]byte, cfg.Ibs)
	var blocks int64

	for {
		if cfg.Count >= 0 && blocks >= cfg.Count {
			break
		}

		n, err := readBlock(in, buf, cfg.Fullblock)

		if n > 0 {
			stats.recordIn(int64(n), cfg.Ibs)
			blocks++

			data := buf[:n]
			if cfg.Conv.Sync && int64(n) < cfg.Ibs {
				for i := n; int64(i) < cfg.Ibs; i++ {
					buf[i] = 0
				}
				data = buf[:cfg.Ibs]
			}

			applyConversions(data, cfg.Conv)
			if err := writeOut(cfg, ow, data); err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			if cfg.Conv.Noerror {
				fmt.Fprintf(stderr, "dd: error reading %q: %v\n", cfg.InFile, err)
				stats.addError()
				if cfg.Conv.Sync {
					// Write a zero block for the failed read so offsets stay aligned.
					zero := make([]byte, cfg.Ibs)
					if werr := writeOut(cfg, ow, zero); werr != nil {
						return werr
					}
					blocks++
				}
				if n == 0 {
					// Without seeking past the bad spot we'd just fail again.
					break
				}
				continue
			}
			return fmt.Errorf("reading %q: %w", cfg.InFile, err)
		}
	}
	return nil
}

func writeOut(cfg *Config, ow *outWriter, data []byte) error {
	if cfg.BsMode {
		return ow.writeWhole(data)
	}
	return ow.write(data)
}

// readBlock reads one input block. Without fullblock a short read is a partial
// block; with it, reading continues until buf is full or EOF.
func readBlock(in io.Reader, buf []byte, fullblock bool) (int, error) {
	if fullblock {
		n, err := io.ReadFull(in, buf)
		switch {
		case err == io.ErrUnexpectedEOF:
			return n, io.EOF
		case err == io.EOF:
			return 0, io.EOF
		default:
			return n, err
		}
	}
	for {
		n, err := in.Read(buf)
		if n == 0 && err == nil {
			continue
		}
		return n, err
	}
}

// discard skips n bytes of input that can't seek. Hitting EOF early is fine.
func discard(r io.Reader, n int64) error {
	_, err := io.CopyN(io.Discard, r, n)
	if err == io.EOF {
		return nil
	}
	return err
}

// writeZeros implements seek= on output that can't seek.
func writeZeros(w io.Writer, n int64) error {
	const chunk = 64 * 1024
	z := make([]byte, chunk)
	for n > 0 {
		m := int64(chunk)
		if m > n {
			m = n
		}
		if _, err := w.Write(z[:m]); err != nil {
			return err
		}
		n -= m
	}
	return nil
}

func trimNewline(s string) string {
	return strings.TrimRight(s, "\n")
}
