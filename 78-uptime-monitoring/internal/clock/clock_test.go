package clock

import (
	"testing"
	"time"
)

func TestFakeClockNowAdvance(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(start)
	if !f.Now().Equal(start) {
		t.Fatalf("Now = %v, want %v", f.Now(), start)
	}
	f.Advance(90 * time.Second)
	if got := f.Now(); !got.Equal(start.Add(90 * time.Second)) {
		t.Fatalf("after advance Now = %v", got)
	}
}

func TestFakeClockAfter(t *testing.T) {
	f := NewFake(time.Time{})
	ch := f.After(50 * time.Millisecond)
	select {
	case <-ch:
		t.Fatal("After fired before time advanced")
	default:
	}
	f.Advance(50 * time.Millisecond)
	select {
	case <-ch:
	default:
		t.Fatal("After did not fire after advancing")
	}
}

func TestFakeClockTickerBlocksUntilConsumed(t *testing.T) {
	f := NewFake(time.Time{})
	tk := f.NewTicker(30 * time.Second)
	defer tk.Stop()

	got := make(chan int, 8)
	go func() {
		for i := 0; i < 3; i++ {
			<-tk.C()
			got <- i
		}
	}()

	f.Advance(90 * time.Second) // should fire exactly 3 ticks, paced by consumer
	for i := 0; i < 3; i++ {
		select {
		case <-got:
		case <-time.After(2 * time.Second):
			t.Fatalf("only received %d ticks", i)
		}
	}
	select {
	case <-tk.C():
		t.Fatal("unexpected extra tick")
	default:
	}
}

func TestFakeTickerStop(t *testing.T) {
	f := NewFake(time.Time{})
	tk := f.NewTicker(10 * time.Second)
	tk.Stop()
	// After stop, advancing should not block on a send to a stopped ticker.
	done := make(chan struct{})
	go func() { f.Advance(100 * time.Second); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Advance blocked on a stopped ticker")
	}
}
