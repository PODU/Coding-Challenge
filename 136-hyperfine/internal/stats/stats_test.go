package stats

import (
	"math"
	"testing"
)

func almost(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	almost(t, s.Mean, 5, "mean")
	almost(t, s.Median, 4.5, "median")
	almost(t, s.Min, 2, "min")
	almost(t, s.Max, 9, "max")
	if s.Count != 8 {
		t.Errorf("count = %d, want 8", s.Count)
	}
	// Sample (n-1) standard deviation of the classic example set.
	almost(t, s.StdDev, math.Sqrt(32.0/7.0), "stddev")
}

func TestSummarizeEmpty(t *testing.T) {
	if got := Summarize(nil); got != (Summary{}) {
		t.Errorf("Summarize(nil) = %+v, want zero value", got)
	}
}

func TestStdDevSingleSample(t *testing.T) {
	almost(t, StdDev([]float64{42}), 0, "stddev of one sample")
}

func TestMedianDoesNotMutateInput(t *testing.T) {
	xs := []float64{3, 1, 2}
	Median(xs)
	if xs[0] != 3 || xs[1] != 1 || xs[2] != 2 {
		t.Errorf("Median reordered its input: %v", xs)
	}
}

func TestMAD(t *testing.T) {
	// Deviations from the median (3) are 2,1,0,1,2 -> median 1.
	almost(t, MAD([]float64{1, 2, 3, 4, 5}), 1, "mad")
}

func TestHasOutliers(t *testing.T) {
	clean := []float64{1.00, 1.01, 0.99, 1.02, 0.98}
	if HasOutliers(clean) {
		t.Error("clean sample reported outliers")
	}
	dirty := append(append([]float64{}, clean...), 12.0)
	if !HasOutliers(dirty) {
		t.Error("sample with a 12x spike reported no outliers")
	}
}

func TestHasOutliersWithZeroMAD(t *testing.T) {
	// Identical samples have no scale to measure deviation against; the
	// function must not divide by zero and report everything as an outlier.
	if HasOutliers([]float64{1, 1, 1, 1}) {
		t.Error("identical samples reported outliers")
	}
}

func TestFirstRunIsOutlier(t *testing.T) {
	cold := []float64{9.0, 1.00, 1.01, 0.99, 1.02}
	if !FirstRunIsOutlier(cold) {
		t.Error("slow first run not detected")
	}
	late := []float64{1.00, 1.01, 0.99, 1.02, 9.0}
	if FirstRunIsOutlier(late) {
		t.Error("a spike in the last run was misreported as a slow first run")
	}
	if FirstRunIsOutlier([]float64{1}) {
		t.Error("single sample cannot be a slow first run")
	}
}

func TestRatioError(t *testing.T) {
	// 10% relative error on each side propagates to sqrt(2)*10% of the ratio.
	got := RatioError(2, 1.0, 0.1, 0.5, 0.05)
	almost(t, got, 2*math.Sqrt(0.02), "ratio error")

	if RatioError(2, 0, 0.1, 0.5, 0.05) != 0 {
		t.Error("zero mean should yield zero error rather than NaN")
	}
}
