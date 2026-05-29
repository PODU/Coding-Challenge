package main

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"
)

// Stats holds dd's record and byte counters. They are atomic because the
// progress ticker and the SIGUSR1 handler read them mid-copy.
type Stats struct {
	inFull     atomic.Int64
	inPartial  atomic.Int64
	outFull    atomic.Int64
	outPartial atomic.Int64
	bytes      atomic.Int64
	errors     atomic.Int64
	start      time.Time
}

func NewStats() *Stats {
	return &Stats{start: time.Now()}
}

func (s *Stats) recordIn(n, ibs int64) {
	if n == ibs {
		s.inFull.Add(1)
	} else {
		s.inPartial.Add(1)
	}
}

func (s *Stats) recordOut(n, obs int64) {
	if n == obs {
		s.outFull.Add(1)
	} else {
		s.outPartial.Add(1)
	}
}

func (s *Stats) addBytes(n int64) { s.bytes.Add(n) }
func (s *Stats) addError()        { s.errors.Add(1) }

func (s *Stats) recordLines() string {
	return fmt.Sprintf("%d+%d records in\n%d+%d records out\n",
		s.inFull.Load(), s.inPartial.Load(),
		s.outFull.Load(), s.outPartial.Load())
}

func (s *Stats) transferLine() string {
	b := s.bytes.Load()
	secs := time.Since(s.start).Seconds()
	rate := 0.0
	if secs > 0 {
		rate = float64(b) / secs / 1e6
	}
	return fmt.Sprintf("%d bytes copied, %s s, %s MB/s\n", b, fmtFloat(secs), fmtFloat(rate))
}

// Summary returns the end-of-run report for the given status level.
func (s *Stats) Summary(level StatusLevel) string {
	switch level {
	case StatusNone:
		return ""
	case StatusNoxfer:
		return s.recordLines()
	default:
		return s.recordLines() + s.transferLine()
	}
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', 4, 64)
}
