package interp

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// setRecord installs a new $0 and marks fields for lazy splitting.
func (p *Interp) setRecord(line string) {
	p.line = line
	p.fieldsValid = false
}

// ensureSplit splits $0 into fields if not already done.
func (p *Interp) ensureSplit() {
	if p.fieldsValid {
		return
	}
	p.fields = p.splitRecord(p.line)
	p.nf = len(p.fields)
	p.fieldsValid = true
}

// splitRecord splits a record into fields using the current FS.
func (p *Interp) splitRecord(record string) []string {
	return p.splitString(record, p.fs, p.rs == "")
}

// splitString splits s using field separator fs. paragraph indicates RS=""
// mode in which newline is always an additional separator.
func (p *Interp) splitString(s, fs string, paragraph bool) []string {
	if s == "" {
		return nil
	}
	if fs == " " {
		return strings.FieldsFunc(s, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n'
		})
	}
	if paragraph {
		// In paragraph mode newline is always a separator in addition to FS.
		if fs == "\n" {
			return strings.Split(s, "\n")
		}
		re := p.compileRegex("(" + regexpEscapeIfLiteral(fs) + ")|\n")
		return re.Split(s, -1)
	}
	if len(fs) == 1 {
		if fs == "\t" {
			return strings.Split(s, "\t")
		}
		return strings.Split(s, fs)
	}
	// multi-character FS is a regular expression
	re := p.compileRegex(fs)
	return re.Split(s, -1)
}

// regexpEscapeIfLiteral quotes fs when it is a single literal character so it
// can be embedded inside a larger regex.
func regexpEscapeIfLiteral(fs string) string {
	if len(fs) == 1 {
		return regexp.QuoteMeta(fs)
	}
	return fs
}

// getField returns $i.
func (p *Interp) getField(i int) value {
	if i == 0 {
		return strnum(p.line)
	}
	if i < 0 {
		p.fatalf("attempt to access field %d", i)
	}
	p.ensureSplit()
	if i > p.nf {
		return strnum("")
	}
	return strnum(p.fields[i-1])
}

// setField assigns to $i, rebuilding $0 as needed.
func (p *Interp) setField(i int, val string) {
	if i < 0 {
		p.fatalf("attempt to assign to field %d", i)
	}
	if i == 0 {
		p.setRecord(val)
		p.ensureSplit()
		return
	}
	p.ensureSplit()
	if i > p.nf {
		for len(p.fields) < i {
			p.fields = append(p.fields, "")
		}
		p.nf = i
	}
	p.fields[i-1] = val
	p.rebuildRecord()
}

// rebuildRecord joins the fields into $0 using OFS.
func (p *Interp) rebuildRecord() {
	p.line = strings.Join(p.fields[:p.nf], p.ofs)
}

// setNF truncates or extends the field list and rebuilds $0.
func (p *Interp) setNF(n int) {
	p.ensureSplit()
	if n < 0 {
		n = 0
	}
	if n < p.nf {
		p.fields = p.fields[:n]
	} else {
		for len(p.fields) < n {
			p.fields = append(p.fields, "")
		}
	}
	p.nf = n
	p.rebuildRecord()
}

// recordReader reads records from an io.Reader honouring RS.
type recordReader struct {
	r *bufio.Reader
}

func newRecordReader(r io.Reader) *recordReader {
	return &recordReader{r: bufio.NewReader(r)}
}

// next reads the next record. ok is false at end of input.
func (rr *recordReader) next(rs string) (rec string, ok bool, err error) {
	if rs == "" {
		return rr.nextParagraph()
	}
	delim := byte('\n')
	if len(rs) >= 1 {
		delim = rs[0]
	}
	s, e := rr.r.ReadString(delim)
	if e != nil {
		if e == io.EOF {
			if s == "" {
				return "", false, nil
			}
			return s, true, nil
		}
		return "", false, e
	}
	return s[:len(s)-1], true, nil
}

func (rr *recordReader) nextParagraph() (string, bool, error) {
	var lines []string
	started := false
	for {
		line, e := rr.r.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if started {
				return strings.Join(lines, "\n"), true, nil
			}
			if e != nil {
				return "", false, ioEOF(e)
			}
			continue
		}
		started = true
		lines = append(lines, trimmed)
		if e != nil {
			return strings.Join(lines, "\n"), true, ioEOF(e)
		}
	}
}

func ioEOF(e error) error {
	if e == io.EOF {
		return nil
	}
	return e
}
