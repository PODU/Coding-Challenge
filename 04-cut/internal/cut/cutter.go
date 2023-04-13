package cut

import (
	"bufio"
	"io"
)

type Mode int

const (
	ModeFields Mode = iota // -f
	ModeBytes              // -b
	ModeChars              // -c
)

// Config is a validated set of options, as produced by cli.Parse.
type Config struct {
	Mode        Mode
	Sel         Selector
	Delim       byte // field delimiter, default '\t'
	OutDelim    string
	OutDelimSet bool     // --output-delimiter was given
	OnlyDelim   bool     // -s
	Files       []string // "-" means stdin
}

type Cutter struct {
	cfg Config
}

// NewCutter returns a Cutter for cfg. Without --output-delimiter, fields are
// joined with the input delimiter and bytes/characters are just concatenated,
// as in GNU cut.
func NewCutter(cfg Config) *Cutter {
	if !cfg.OutDelimSet && cfg.Mode == ModeFields {
		cfg.OutDelim = string(cfg.Delim)
	}
	return &Cutter{cfg: cfg}
}

// ProcessStream cuts r line by line into w. If the last line has no trailing
// newline, the output won't either.
func (c *Cutter) ProcessStream(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	bw, isBuffered := w.(*bufio.Writer)
	if !isBuffered {
		bw = bufio.NewWriter(w)
	}

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			hasNL := line[len(line)-1] == '\n'
			content := line
			if hasNL {
				content = line[:len(line)-1]
			}
			c.processLine(content, hasNL, bw)
		}
		if err != nil {
			if !isBuffered {
				if ferr := bw.Flush(); ferr != nil {
					return ferr
				}
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// processLine handles one line with its newline already stripped.
func (c *Cutter) processLine(line []byte, hasNL bool, w *bufio.Writer) {
	switch c.cfg.Mode {
	case ModeFields:
		if !c.cutFields(line, w) {
			return
		}
	case ModeBytes:
		c.cutBytes(line, w)
	case ModeChars:
		c.cutChars(line, w)
	}
	if hasNL {
		w.WriteByte('\n')
	}
}
