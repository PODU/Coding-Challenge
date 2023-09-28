// Command ntpclient is a small SNTP client. It asks an NTP server for the time
// and reports how far off the local clock is, plus the round-trip delay.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/codingchallenges/ntpclient/internal/ntp"
)

const (
	exitOK       = 0
	exitUsage    = 1
	exitNetwork  = 2
	exitProtocol = 3
)

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

type jsonResult struct {
	Server      string  `json:"server"`
	Stratum     uint8   `json:"stratum"`
	OffsetMS    float64 `json:"offset_ms"`
	DelayMS     float64 `json:"delay_ms"`
	OffsetNanos int64   `json:"offset_ns"`
	DelayNanos  int64   `json:"delay_ns"`
	Leap        uint8   `json:"leap"`
	Version     uint8   `json:"version"`
	RefID       string  `json:"ref_id"`
	T0          string  `json:"t0_originate"`
	T1          string  `json:"t1_receive"`
	T2          string  `json:"t2_transmit"`
	T3          string  `json:"t3_destination"`
	LocalTime   string  `json:"local_time"`
	NTPTime     string  `json:"ntp_time"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		server  = flag.String("server", ntp.DefaultServer, "NTP server host[:port]")
		timeout = flag.Duration("timeout", ntp.DefaultTimeout, "UDP read timeout")
		samples = flag.Int("samples", 1, "number of queries; report best (lowest delay)")
		asJSON  = flag.Bool("json", false, "emit machine-readable JSON instead of text")
		setClk  = flag.Bool("set", false, "step the system clock by the offset (requires privilege)")
		verbose = flag.Bool("v", false, "verbose: print all four timestamps and packet header")
	)
	flag.Parse()

	if *samples < 1 {
		fmt.Fprintln(os.Stderr, "error: -samples must be >= 1")
		return exitUsage
	}

	res, err := ntp.QueryBest(*server, *samples, ntp.Options{Timeout: *timeout})
	if err != nil {
		return reportError(err)
	}

	if *asJSON {
		printJSON(res)
	} else {
		printText(res, *verbose)
	}

	if *setClk {
		fmt.Fprintln(os.Stderr, "error: -set (clock step) is not supported on this build; "+
			"adjust the system clock with your OS tooling using the reported offset")
		return exitUsage
	}

	return exitOK
}

// reportError prints err and returns the exit code for it.
func reportError(err error) int {
	var kod *ntp.KissOfDeathError
	switch {
	case errors.As(err, &kod):
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitProtocol
	case errors.Is(err, ntp.ErrLeapNotInSync),
		errors.Is(err, ntp.ErrBadMode),
		errors.Is(err, ntp.ErrBadVersion),
		errors.Is(err, ntp.ErrBadStratum),
		errors.Is(err, ntp.ErrOriginateMismatch):
		fmt.Fprintf(os.Stderr, "error: invalid response: %v\n", err)
		return exitProtocol
	default:
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitNetwork
	}
}

func printText(res *ntp.Result, verbose bool) {
	fmt.Printf("server:   %s\n", res.Server)
	fmt.Printf("stratum:  %d\n", res.Stratum)
	fmt.Printf("ref id:   %#08x (%s)\n", res.ReferenceID, printableRefID(res.RefIDString))
	fmt.Printf("offset:   %s\n", formatMS(res.Offset))
	fmt.Printf("delay:    %s\n", formatMS(res.Delay))
	fmt.Printf("local:    %s\n", res.T3.Format(timeFormat))
	fmt.Printf("ntp time: %s\n", res.T2.Format(timeFormat))

	if verbose {
		fmt.Println("---")
		fmt.Printf("leap:      %d\n", res.LeapIndicator)
		fmt.Printf("version:   %d\n", res.Version)
		fmt.Printf("precision: %d (%.3g s)\n", res.Precision, precisionSeconds(res.Precision))
		fmt.Printf("t0 originate:   %s\n", res.T0.Format(timeFormat))
		fmt.Printf("t1 receive:     %s\n", res.T1.Format(timeFormat))
		fmt.Printf("t2 transmit:    %s\n", res.T2.Format(timeFormat))
		fmt.Printf("t3 destination: %s\n", res.T3.Format(timeFormat))
	}
}

func printJSON(res *ntp.Result) {
	out := jsonResult{
		Server:      res.Server,
		Stratum:     res.Stratum,
		OffsetMS:    float64(res.Offset) / float64(time.Millisecond),
		DelayMS:     float64(res.Delay) / float64(time.Millisecond),
		OffsetNanos: res.Offset.Nanoseconds(),
		DelayNanos:  res.Delay.Nanoseconds(),
		Leap:        res.LeapIndicator,
		Version:     res.Version,
		RefID:       printableRefID(res.RefIDString),
		T0:          res.T0.Format(timeFormat),
		T1:          res.T1.Format(timeFormat),
		T2:          res.T2.Format(timeFormat),
		T3:          res.T3.Format(timeFormat),
		LocalTime:   res.T3.Format(timeFormat),
		NTPTime:     res.T2.Format(timeFormat),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func formatMS(d time.Duration) string {
	return fmt.Sprintf("%.3f ms", float64(d)/float64(time.Millisecond))
}

// precisionSeconds converts the precision field (a power of two) to seconds.
func precisionSeconds(p int8) float64 {
	return math.Ldexp(1, int(p))
}

// printableRefID replaces non-printable characters with dots.
func printableRefID(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r < 0x20 || r > 0x7e {
			out[i] = '.'
		}
	}
	return string(out)
}
