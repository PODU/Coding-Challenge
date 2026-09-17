package bench

import (
	"fmt"

	"cchyperfine/internal/cli"
	"cchyperfine/internal/stats"
)

// Alert kinds, in the order they are reported.
const (
	AlertNonZeroExit = "non-zero-exit"
	AlertTooFast     = "too-fast"
	AlertSlowFirst   = "slow-first-run"
	AlertOutliers    = "outliers"
)

// alerts inspects a finished benchmark and produces the diagnostics that tell a
// reader when not to trust the numbers.
//
// The four checks answer four different questions: did the command actually
// work; is it fast enough that our own measurement error dominates; was the
// first run special (cold cache); and did something outside the benchmark
// interfere partway through.
func (r *Runner) alerts(res *Result) []Alert {
	var out []Alert

	if n := countFailures(res.ExitCodes); n > 0 {
		out = append(out, Alert{
			Kind: AlertNonZeroExit,
			Message: fmt.Sprintf("Ignoring non-zero exit code: %d of %d runs failed.",
				n, len(res.ExitCodes)),
		})
	}

	if res.Wall.Mean > 0 && res.Wall.Mean < cli.MinAccurateTimeSeconds {
		msg := "Command took less than 5 ms to complete. Note that the results might be inaccurate " +
			"because hyperfine can not calibrate the shell startup time much more precise than this limit."
		if r.Opts.Shell.Enabled() {
			msg += " You can try to use the '-N'/'--shell=none' option to disable the shell completely."
		}
		out = append(out, Alert{Kind: AlertTooFast, Message: msg})
	}

	// A slow first run is a more specific diagnosis than "there are outliers",
	// so when it applies we report only that.
	switch {
	case stats.FirstRunIsOutlier(res.Times):
		out = append(out, Alert{
			Kind: AlertSlowFirst,
			Message: "The first benchmarking run for this command was significantly slower than the " +
				"rest. This could be caused by (filesystem) caches that were not filled until after " +
				"the first run. You should consider using the '--warmup' option to fill those caches " +
				"before the actual benchmark. Alternatively, use the '--prepare' option to clear the " +
				"caches before each timing run.",
		})
	case stats.HasOutliers(res.Times):
		out = append(out, Alert{
			Kind: AlertOutliers,
			Message: "Statistical outliers were detected. Consider re-running this benchmark on a " +
				"quiet system without any interferences from other programs. It might help to use " +
				"the '--warmup' or '--prepare' options.",
		})
	}

	return out
}

func countFailures(codes []int) int {
	n := 0
	for _, c := range codes {
		if c != 0 {
			n++
		}
	}
	return n
}
