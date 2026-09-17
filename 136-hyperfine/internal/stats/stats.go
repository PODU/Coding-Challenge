// Package stats provides the descriptive statistics and outlier detection that
// turn a list of raw timings into something a human can reason about.
package stats

import (
	"math"
	"sort"
)

// OutlierThreshold is the modified z-score above which a sample is treated as
// an outlier. It matches hyperfine's constant (10 * 1.4826), which is the
// classic "ten MADs from the median" rule expressed on an unscaled MAD.
const OutlierThreshold = 14.826

// Summary holds every statistic the reporter needs for one benchmark.
type Summary struct {
	Mean   float64
	StdDev float64
	Median float64
	Min    float64
	Max    float64
	Count  int
}

// Summarize computes the descriptive statistics of xs. An empty slice yields a
// zero Summary rather than a panic, so callers can report failed benchmarks.
func Summarize(xs []float64) Summary {
	if len(xs) == 0 {
		return Summary{}
	}
	return Summary{
		Mean:   Mean(xs),
		StdDev: StdDev(xs),
		Median: Median(xs),
		Min:    Min(xs),
		Max:    Max(xs),
		Count:  len(xs),
	}
}

// Mean is the arithmetic mean of xs.
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// StdDev is the sample standard deviation (Bessel-corrected, n-1). A single
// sample has no spread to speak of, so it reports 0.
func StdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	var sum float64
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(xs)-1))
}

// Median returns the middle value of xs, averaging the two central values for
// even-length input. The input slice is not modified.
func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// MAD is the median absolute deviation from the median: a spread estimator that
// does not itself get dragged around by the outliers we are trying to find.
func MAD(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	med := Median(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - med)
	}
	return Median(dev)
}

// Min returns the smallest value in xs.
func Min(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

// Max returns the largest value in xs.
func Max(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

// ModifiedZScores expresses each sample's distance from the median in units of
// MAD. When the MAD is zero (every sample identical, or a majority identical)
// there is no meaningful scale, so every score is reported as 0.
func ModifiedZScores(xs []float64) []float64 {
	out := make([]float64, len(xs))
	mad := MAD(xs)
	if mad == 0 {
		return out
	}
	med := Median(xs)
	for i, x := range xs {
		out[i] = (x - med) / mad
	}
	return out
}

// HasOutliers reports whether any sample sits further than OutlierThreshold
// MADs from the median.
func HasOutliers(xs []float64) bool {
	for _, z := range ModifiedZScores(xs) {
		if math.Abs(z) > OutlierThreshold {
			return true
		}
	}
	return false
}

// FirstRunIsOutlier reports whether the first sample is both an outlier and the
// slowest of the batch -- the signature of a cold cache rather than of random
// interference partway through the run.
func FirstRunIsOutlier(xs []float64) bool {
	if len(xs) < 2 {
		return false
	}
	z := ModifiedZScores(xs)
	return z[0] > OutlierThreshold && xs[0] == Max(xs)
}

// RatioError propagates the relative uncertainty of two independent means
// through their quotient, giving the "±" on a "1.23 ± 0.04 times faster" claim.
func RatioError(ratio, meanA, sdA, meanB, sdB float64) float64 {
	if meanA == 0 || meanB == 0 {
		return 0
	}
	ra := sdA / meanA
	rb := sdB / meanB
	return ratio * math.Sqrt(ra*ra+rb*rb)
}
