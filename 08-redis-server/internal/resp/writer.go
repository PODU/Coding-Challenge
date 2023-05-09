package resp

import (
	"bufio"
	"strconv"
)

// Writer serializes RESP values to a buffered byte stream.
type Writer struct {
	w *bufio.Writer
}

// NewWriter returns a Writer that encodes to w.
func NewWriter(w *bufio.Writer) *Writer { return &Writer{w: w} }

// Flush sends any buffered output.
func (wr *Writer) Flush() error { return wr.w.Flush() }

// Write serializes v without flushing.
func (wr *Writer) Write(v Value) error {
	switch v.Kind {
	case Simple:
		return wr.writeLine('+', v.Str)
	case Error:
		return wr.writeLine('-', v.Str)
	case Integer:
		return wr.writeLine(':', strconv.FormatInt(v.Int, 10))
	case Bulk:
		if v.IsNull {
			_, err := wr.w.WriteString("$-1\r\n")
			return err
		}
		if err := wr.writeLine('$', strconv.Itoa(len(v.Str))); err != nil {
			return err
		}
		if _, err := wr.w.WriteString(v.Str); err != nil {
			return err
		}
		_, err := wr.w.WriteString("\r\n")
		return err
	case Array:
		if v.IsNull {
			_, err := wr.w.WriteString("*-1\r\n")
			return err
		}
		if err := wr.writeLine('*', strconv.Itoa(len(v.Arr))); err != nil {
			return err
		}
		for _, e := range v.Arr {
			if err := wr.Write(e); err != nil {
				return err
			}
		}
		return nil
	default:
		return wr.writeLine('-', "ERR internal: unknown RESP kind")
	}
}

func (wr *Writer) writeLine(prefix byte, body string) error {
	if err := wr.w.WriteByte(prefix); err != nil {
		return err
	}
	if _, err := wr.w.WriteString(body); err != nil {
		return err
	}
	_, err := wr.w.WriteString("\r\n")
	return err
}
