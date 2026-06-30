package report

import (
	"fmt"
	"io"

	"du/internal/walk"
)

// Reporter decides which walker entries to print (depth, threshold, -s) and
// prints them.
type Reporter struct {
	out  io.Writer
	opts walk.Options
}

// New returns a Reporter that writes to out.
func New(out io.Writer, opts walk.Options) *Reporter {
	return &Reporter{out: out, opts: opts}
}

// Report prints the entries that pass the active filters.
func (r *Reporter) Report(entries []walk.Entry) {
	for _, e := range entries {
		if !r.shouldPrint(e) {
			continue
		}
		fmt.Fprintf(r.out, "%s\t%s\n", FormatSize(e.Size, r.opts), e.Path)
	}
}

// Total prints the grand-total line for -c.
func (r *Reporter) Total(size int64) {
	fmt.Fprintf(r.out, "%s\ttotal\n", FormatSize(size, r.opts))
}

func (r *Reporter) shouldPrint(e walk.Entry) bool {
	switch {
	case r.opts.Summarize:
		if e.Depth != 0 {
			return false
		}
	case r.opts.MaxDepth >= 0:
		if e.Depth > r.opts.MaxDepth {
			return false
		}
	}
	return r.passThreshold(e.Size)
}

func (r *Reporter) passThreshold(size int64) bool {
	t := r.opts.Threshold
	switch {
	case t == 0:
		return true
	case t > 0:
		return size >= t
	default:
		return size <= -t
	}
}
