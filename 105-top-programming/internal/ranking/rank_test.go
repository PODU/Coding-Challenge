package ranking

import (
	"testing"
	"time"

	"github.com/codingchallenges/topstories/internal/model"
)

func base() time.Time { return time.Date(2023, 11, 15, 0, 0, 0, 0, time.UTC) }

func corpus() []model.Story {
	b := base()
	return []model.Story{
		{ID: 1, Title: "Rust memory safety", Score: 300, Type: "story", Timestamp: b},
		{ID: 2, Title: "Go generics deep dive", Score: 150, Type: "story", Timestamp: b.Add(1 * time.Hour)},
		{ID: 3, Title: "Hiring: Backend Engineer", Score: 5, Type: "job", Timestamp: b.Add(2 * time.Hour)},
		{ID: 4, Title: "Ask HN: best Go editor?", Score: 150, Type: "story", Timestamp: b.Add(3 * time.Hour)},
		{ID: 5, Title: "Poll: tabs or spaces", Score: 90, Type: "poll", Timestamp: b.Add(4 * time.Hour)},
	}
}

func TestApplyMinScore(t *testing.T) {
	got := Apply(corpus(), Filter{MinScore: 100})
	if len(got) != 3 {
		t.Fatalf("min score 100: got %d", len(got))
	}
	for _, s := range got {
		if s.Score < 100 {
			t.Errorf("leaked low score: %+v", s)
		}
	}
}

func TestApplyTimeWindow(t *testing.T) {
	b := base()
	got := Apply(corpus(), Filter{
		Since: b.Add(1 * time.Hour),
		Until: b.Add(3 * time.Hour),
	})
	// IDs 2,3,4 fall in [b+1h, b+3h]
	if len(got) != 3 {
		t.Fatalf("time window: got %d (%v)", len(got), got)
	}
}

func TestApplyTypeFilter(t *testing.T) {
	got := Apply(corpus(), Filter{Types: []string{"story"}})
	if len(got) != 3 {
		t.Fatalf("type story: got %d", len(got))
	}
	got = Apply(corpus(), Filter{Types: []string{"job", "poll"}})
	if len(got) != 2 {
		t.Fatalf("type job/poll: got %d", len(got))
	}
}

func TestApplyKeywordCaseInsensitive(t *testing.T) {
	got := Apply(corpus(), Filter{Keywords: []string{"go"}})
	// titles containing "go": "Go generics", "best Go editor"
	if len(got) != 2 {
		t.Fatalf("keyword go: got %d (%v)", len(got), titles(got))
	}
	got = Apply(corpus(), Filter{Keywords: []string{"RUST"}})
	if len(got) != 1 {
		t.Fatalf("keyword RUST: got %d", len(got))
	}
}

func TestApplyCombined(t *testing.T) {
	got := Apply(corpus(), Filter{MinScore: 100, Types: []string{"story"}, Keywords: []string{"go"}})
	if len(got) != 2 {
		t.Fatalf("combined: got %d (%v)", len(got), titles(got))
	}
}

func TestApplyEmptyInput(t *testing.T) {
	got := Apply(nil, Filter{MinScore: 10})
	if got == nil || len(got) != 0 {
		t.Fatalf("empty input should yield empty non-nil slice, got %v", got)
	}
}

func TestTopNSortAndTies(t *testing.T) {
	got := TopN(corpus(), 3)
	wantIDs := []int64{1, 2, 4} // 300, then tie 150/150 broken by ID asc
	if len(got) != 3 {
		t.Fatalf("TopN 3: got %d", len(got))
	}
	for i, id := range wantIDs {
		if got[i].ID != id {
			t.Fatalf("TopN order: got %v want %v", idsOf(got), wantIDs)
		}
	}
}

func TestTopNExceedsLen(t *testing.T) {
	got := TopN(corpus(), 100)
	if len(got) != 5 {
		t.Fatalf("N>len should return all: got %d", len(got))
	}
}

func TestTopNZeroReturnsAll(t *testing.T) {
	if len(TopN(corpus(), 0)) != 5 {
		t.Fatal("n<=0 should return all")
	}
}

func TestTopNDoesNotMutateInput(t *testing.T) {
	in := corpus()
	first := in[0].ID
	_ = TopN(in, 2)
	if in[0].ID != first {
		t.Fatal("TopN mutated input slice order")
	}
}

func titles(s []model.Story) []string {
	out := make([]string, len(s))
	for i := range s {
		out[i] = s[i].Title
	}
	return out
}

func idsOf(s []model.Story) []int64 {
	out := make([]int64, len(s))
	for i := range s {
		out[i] = s[i].ID
	}
	return out
}
