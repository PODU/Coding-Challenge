package aggregate

import (
	"testing"
	"time"

	"gitgraph/internal/model"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}

func d(y int, m time.Month, day int) model.Date { return model.Date{Year: y, Month: m, Day: day} }

func TestBucketSameDayCountsTwo(t *testing.T) {
	commits := []model.Commit{
		{SHA: "a", When: mustTime(t, "2026-03-01T09:00:00Z")},
		{SHA: "b", When: mustTime(t, "2026-03-01T18:00:00Z")},
		{SHA: "c", When: mustTime(t, "2026-03-02T01:00:00Z")},
	}
	counts := Bucket(commits, time.UTC, d(2026, 3, 1), d(2026, 3, 31))
	if got := counts[d(2026, 3, 1)]; got != 2 {
		t.Errorf("Mar 1 count = %d, want 2", got)
	}
	if got := counts[d(2026, 3, 2)]; got != 1 {
		t.Errorf("Mar 2 count = %d, want 1", got)
	}
}

func TestBucketTimezoneBoundary(t *testing.T) {
	// 23:30 on Mar 1 in -05:00. In America/New_York this is still Mar 1;
	// converted to UTC it is 04:30 on Mar 2.
	commit := model.Commit{SHA: "x", When: mustTime(t, "2026-03-01T23:30:00-05:00")}

	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tz database unavailable: %v", err)
	}
	nyCounts := Bucket([]model.Commit{commit}, ny, d(2026, 3, 1), d(2026, 3, 3))
	if nyCounts[d(2026, 3, 1)] != 1 || nyCounts[d(2026, 3, 2)] != 0 {
		t.Errorf("New_York bucketing wrong: %v", nyCounts)
	}

	utcCounts := Bucket([]model.Commit{commit}, time.UTC, d(2026, 3, 1), d(2026, 3, 3))
	if utcCounts[d(2026, 3, 2)] != 1 || utcCounts[d(2026, 3, 1)] != 0 {
		t.Errorf("UTC bucketing wrong: %v", utcCounts)
	}
}

func TestFilterAuthorNameAndEmail(t *testing.T) {
	commits := []model.Commit{
		{SHA: "1", AuthorName: "Ada Lovelace", AuthorEmail: "ada@example.com"},
		{SHA: "2", AuthorName: "Bob", AuthorEmail: "bob@corp.io"},
		{SHA: "3", AuthorName: "Carol", AuthorEmail: "CAROL@ADA.io"},
	}
	// substring on name (case-insensitive)
	if got := Filter(commits, "ada"); len(got) != 2 {
		t.Errorf("filter 'ada' matched %d, want 2 (name + email domain)", len(got))
	}
	// substring on email only
	if got := Filter(commits, "corp.io"); len(got) != 1 || got[0].SHA != "2" {
		t.Errorf("filter 'corp.io' = %v, want [2]", got)
	}
	// empty matches all
	if got := Filter(commits, ""); len(got) != 3 {
		t.Errorf("empty filter dropped commits: %d", len(got))
	}
}

func TestBucketWindowInclusiveEndpoints(t *testing.T) {
	commits := []model.Commit{
		{SHA: "before", When: mustTime(t, "2026-02-28T12:00:00Z")},
		{SHA: "start", When: mustTime(t, "2026-03-01T12:00:00Z")},
		{SHA: "end", When: mustTime(t, "2026-03-31T12:00:00Z")},
		{SHA: "after", When: mustTime(t, "2026-04-01T12:00:00Z")},
	}
	counts := Bucket(commits, time.UTC, d(2026, 3, 1), d(2026, 3, 31))
	if counts[d(2026, 3, 1)] != 1 {
		t.Error("start day (inclusive) missing")
	}
	if counts[d(2026, 3, 31)] != 1 {
		t.Error("end day (inclusive) missing")
	}
	if counts[d(2026, 2, 28)] != 0 || counts[d(2026, 4, 1)] != 0 {
		t.Error("out-of-window commit leaked in")
	}
}

func TestDedupBySHA(t *testing.T) {
	commits := []model.Commit{
		{SHA: "dup", When: mustTime(t, "2026-03-01T00:00:00Z")},
		{SHA: "dup", When: mustTime(t, "2026-03-01T00:00:00Z")},
		{SHA: "uniq", When: mustTime(t, "2026-03-02T00:00:00Z")},
	}
	got := Dedup(commits)
	if len(got) != 2 {
		t.Errorf("Dedup len = %d, want 2", len(got))
	}
}

func TestComputeStatsStreaks(t *testing.T) {
	start, end := d(2026, 3, 1), d(2026, 3, 10)
	// Active: Mar 1,2,3 (longest run 3), gap Mar4-5, Mar6, gap, Mar 8,9,10 (current 3).
	counts := map[model.Date]int{
		d(2026, 3, 1):  1,
		d(2026, 3, 2):  2,
		d(2026, 3, 3):  1,
		d(2026, 3, 6):  5,
		d(2026, 3, 8):  1,
		d(2026, 3, 9):  1,
		d(2026, 3, 10): 1,
	}
	st := ComputeStats(counts, start, end)
	if st.Total != 12 {
		t.Errorf("Total = %d, want 12", st.Total)
	}
	if st.ActiveDays != 7 {
		t.Errorf("ActiveDays = %d, want 7", st.ActiveDays)
	}
	if st.Max != 5 || !st.Busiest.Equal(d(2026, 3, 6)) {
		t.Errorf("Busiest = %s (%d), want 2026-03-06 (5)", st.Busiest, st.Max)
	}
	if st.LongestStreak != 3 {
		t.Errorf("LongestStreak = %d, want 3", st.LongestStreak)
	}
	if st.CurrentStreak != 3 {
		t.Errorf("CurrentStreak = %d, want 3", st.CurrentStreak)
	}
	if st.Days != 10 {
		t.Errorf("Days = %d, want 10", st.Days)
	}
}

func TestComputeStatsSingleDayStreakAndGapReset(t *testing.T) {
	start, end := d(2026, 3, 1), d(2026, 3, 5)
	counts := map[model.Date]int{d(2026, 3, 3): 4} // isolated single day, not at end
	st := ComputeStats(counts, start, end)
	if st.LongestStreak != 1 {
		t.Errorf("LongestStreak = %d, want 1", st.LongestStreak)
	}
	if st.CurrentStreak != 0 {
		t.Errorf("CurrentStreak = %d, want 0 (end day inactive)", st.CurrentStreak)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	st := ComputeStats(map[model.Date]int{}, d(2026, 3, 1), d(2026, 3, 10))
	if st.Total != 0 || st.ActiveDays != 0 || st.Max != 0 ||
		st.CurrentStreak != 0 || st.LongestStreak != 0 {
		t.Errorf("empty stats not zero-valued: %+v", st)
	}
}

func TestDayCountsSorted(t *testing.T) {
	counts := map[model.Date]int{
		d(2026, 3, 3): 1,
		d(2026, 3, 1): 2,
		d(2026, 3, 2): 3,
	}
	dc := DayCounts(counts)
	if len(dc) != 3 {
		t.Fatalf("len = %d, want 3", len(dc))
	}
	if !dc[0].Date.Equal(d(2026, 3, 1)) || !dc[2].Date.Equal(d(2026, 3, 3)) {
		t.Errorf("DayCounts not date-sorted: %+v", dc)
	}
}

func TestBusiestTieBreakEarliest(t *testing.T) {
	start, end := d(2026, 3, 1), d(2026, 3, 31)
	counts := map[model.Date]int{
		d(2026, 3, 10): 3,
		d(2026, 3, 5):  3, // same max, earlier date wins
	}
	st := ComputeStats(counts, start, end)
	if !st.Busiest.Equal(d(2026, 3, 5)) {
		t.Errorf("Busiest = %s, want 2026-03-05 (earliest on tie)", st.Busiest)
	}
}

func TestMatchesAuthorEmptyMatchesAll(t *testing.T) {
	if !MatchesAuthor(model.Commit{AuthorName: "x"}, "") {
		t.Error("empty substring should match")
	}
}

func TestLeaderboardEmailFallbackToName(t *testing.T) {
	start, end := d(2026, 3, 1), d(2026, 3, 31)
	commits := []model.Commit{
		{SHA: "1", AuthorName: "NoEmail", When: mustTime(t, "2026-03-01T10:00:00Z")},
	}
	board := Leaderboard(commits, nil, start, end) // nil loc -> UTC
	if len(board) != 1 || board[0].Name != "NoEmail" || board[0].Total != 1 {
		t.Errorf("leaderboard fallback wrong: %+v", board)
	}
}

func TestLeaderboard(t *testing.T) {
	start, end := d(2026, 3, 1), d(2026, 3, 31)
	commits := []model.Commit{
		{SHA: "1", AuthorName: "Ada", AuthorEmail: "ada@x.io", When: mustTime(t, "2026-03-01T10:00:00Z")},
		{SHA: "2", AuthorName: "Ada", AuthorEmail: "ada@x.io", When: mustTime(t, "2026-03-02T10:00:00Z")},
		{SHA: "3", AuthorName: "Bob", AuthorEmail: "bob@x.io", When: mustTime(t, "2026-03-02T10:00:00Z")},
	}
	board := Leaderboard(commits, time.UTC, start, end)
	if len(board) != 2 {
		t.Fatalf("contributors = %d, want 2", len(board))
	}
	if board[0].Name != "Ada" || board[0].Total != 2 || board[0].Active != 2 {
		t.Errorf("top contributor = %+v", board[0])
	}
}
