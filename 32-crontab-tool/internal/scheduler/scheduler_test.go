package scheduler

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"crontab/internal/clock"
	"crontab/internal/cron"
	"crontab/internal/job"
)

func quietLogger() *log.Logger { return log.New(io.Discard, "", 0) }

func mkJob(t *testing.T, expr, cmd string) job.Job {
	t.Helper()
	s, err := cron.Parse(expr)
	if err != nil {
		t.Fatalf("parse %q: %v", expr, err)
	}
	return job.Job{Schedule: s, Expr: expr, Command: cmd}
}

// recordRunner records each invocation on a channel and returns immediately.
type recordRunner struct {
	started chan job.Job
}

func (r *recordRunner) Run(ctx context.Context, j job.Job) Result {
	r.started <- j
	return Result{}
}

// blockingRunner records each invocation then blocks until release is closed.
type blockingRunner struct {
	started chan job.Job
	release chan struct{}
}

func (r *blockingRunner) Run(ctx context.Context, j job.Job) Result {
	r.started <- j
	<-r.release
	return Result{}
}

func startOfHour() time.Time {
	return time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
}

func TestSchedulerFiresEveryMinute(t *testing.T) {
	fc := clock.NewFake(startOfHour())
	rr := &recordRunner{started: make(chan job.Job, 100)}

	jobs := []job.Job{
		mkJob(t, "*/1 * * * *", "every-minute"),
		mkJob(t, "*/2 * * * *", "every-two"),
	}
	// AllowOverlap: the fake clock advances faster than job goroutines finish, so
	// the overlap guard would skip runs here. TestSchedulerOverlapGuard covers it.
	sch := New(jobs, Options{Clock: fc, Runner: rr, Logger: quietLogger(), AllowOverlap: true})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Run(ctx)

	// Advance four whole minutes (00:01 .. 00:04).
	for i := 0; i < 4; i++ {
		fc.BlockUntil(1)
		fc.Advance(time.Minute)
	}

	counts := map[string]int{}
	for i := 0; i < 6; i++ { // 4 from every-minute + 2 from every-two
		j := <-rr.started
		counts[j.Command]++
	}
	if counts["every-minute"] != 4 {
		t.Errorf("every-minute fired %d times, want 4", counts["every-minute"])
	}
	if counts["every-two"] != 2 {
		t.Errorf("every-two fired %d times, want 2", counts["every-two"])
	}
}

func TestSchedulerOverlapGuard(t *testing.T) {
	fc := clock.NewFake(startOfHour())
	br := &blockingRunner{started: make(chan job.Job, 10), release: make(chan struct{})}
	sch := New([]job.Job{mkJob(t, "*/1 * * * *", "slow")}, Options{Clock: fc, Runner: br, Logger: quietLogger()})

	ctx, cancel := context.WithCancel(context.Background())
	go sch.Run(ctx)

	// Minute 1: job starts and blocks.
	fc.BlockUntil(1)
	fc.Advance(time.Minute)
	<-br.started

	// Minute 2: previous run still active -> must be skipped.
	fc.BlockUntil(1)
	fc.Advance(time.Minute)
	fc.BlockUntil(1) // scheduler has processed the wake and gone back to sleep

	select {
	case <-br.started:
		t.Fatal("job was double-started despite overlap guard")
	default:
	}

	close(br.release)
	cancel()
	sch.Wait()
}

func TestSchedulerCancellation(t *testing.T) {
	fc := clock.NewFake(startOfHour())
	rr := &recordRunner{started: make(chan job.Job, 10)}
	sch := New([]job.Job{mkJob(t, "*/1 * * * *", "j")}, Options{Clock: fc, Runner: rr, Logger: quietLogger()})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sch.Run(ctx) }()

	fc.BlockUntil(1) // scheduler is asleep, nothing fired yet
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}

	if len(rr.started) != 0 {
		t.Errorf("got %d runs after cancellation, want 0", len(rr.started))
	}
}

func TestSchedulerMissedTickCatchUpOnce(t *testing.T) {
	fc := clock.NewFake(startOfHour())
	rr := &recordRunner{started: make(chan job.Job, 100)}
	sch := New([]job.Job{mkJob(t, "*/1 * * * *", "j")}, Options{Clock: fc, Runner: rr, Logger: quietLogger()})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sch.Run(ctx)

	// Jump 10 minutes forward in one go (machine was asleep). The job must fire
	// at most once on wake, not ten times.
	fc.BlockUntil(1)
	fc.Advance(10 * time.Minute)

	<-rr.started // exactly one run for the landed minute
	fc.BlockUntil(1)
	select {
	case <-rr.started:
		t.Fatal("missed-tick storm: job fired more than once on wake")
	default:
	}
}

func TestRunOnce(t *testing.T) {
	fc := clock.NewFake(startOfHour())
	rr := &recordRunner{started: make(chan job.Job, 10)}
	sch := New([]job.Job{mkJob(t, "*/5 * * * *", "j")}, Options{Clock: fc, Runner: rr, Logger: quietLogger()})

	ctx := context.Background()
	done := make(chan error, 1)
	go func() { done <- sch.RunOnce(ctx) }()

	fc.BlockUntil(1)
	fc.Advance(5 * time.Minute) // reach 00:05, the next fire

	if err := <-done; err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(rr.started) != 1 {
		t.Errorf("RunOnce fired %d jobs, want 1", len(rr.started))
	}
}
