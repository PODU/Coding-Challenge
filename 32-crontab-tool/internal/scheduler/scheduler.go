// Package scheduler runs jobs on their cron schedules. The clock and the
// command runner are pluggable so tests need neither real time nor processes.
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"crontab/internal/clock"
	"crontab/internal/job"
)

type Options struct {
	Clock        clock.Clock
	Runner       Runner
	Logger       *log.Logger
	AllowOverlap bool // start a job again even if its previous run hasn't finished
}

type Scheduler struct {
	jobs         []job.Job
	clock        clock.Clock
	runner       Runner
	logger       *log.Logger
	allowOverlap bool

	mu        sync.Mutex
	lastFired []time.Time // indexed like jobs
	running   []bool
	wg        sync.WaitGroup
}

func New(jobs []job.Job, opts Options) *Scheduler {
	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &Scheduler{
		jobs:         jobs,
		clock:        opts.Clock,
		runner:       opts.Runner,
		logger:       logger,
		allowOverlap: opts.AllowOverlap,
		lastFired:    make([]time.Time, len(jobs)),
		running:      make([]bool, len(jobs)),
	}
}

// Run fires jobs as they come due until ctx is cancelled, then waits for
// running jobs to finish and returns ctx.Err().
func (s *Scheduler) Run(ctx context.Context) error {
	for {
		now := s.clock.Now()
		wake, ok := s.nextWake(now)
		if !ok {
			s.logger.Printf("scheduler: no job will ever fire; stopping")
			s.wg.Wait()
			return nil
		}
		if err := s.clock.Sleep(ctx, wake.Sub(now)); err != nil {
			s.wg.Wait()
			return err
		}
		s.fireDue(ctx, s.clock.Now())
	}
}

// RunOnce waits for the next due minute, runs the jobs due then, and returns
// once they finish.
func (s *Scheduler) RunOnce(ctx context.Context) error {
	now := s.clock.Now()
	wake, ok := s.nextWake(now)
	if !ok {
		return nil
	}
	if err := s.clock.Sleep(ctx, wake.Sub(now)); err != nil {
		s.wg.Wait()
		return err
	}
	s.fireDue(ctx, s.clock.Now())
	s.wg.Wait()
	return nil
}

// Wait blocks until all running jobs have finished.
func (s *Scheduler) Wait() { s.wg.Wait() }

// nextWake returns the soonest time any job is due.
func (s *Scheduler) nextWake(now time.Time) (time.Time, bool) {
	var best time.Time
	found := false
	for i := range s.jobs {
		nx, err := s.jobs[i].Schedule.NextRun(now)
		if err != nil {
			s.logger.Printf("scheduler: job %q (%s) never fires: %v",
				s.jobs[i].Command, s.jobs[i].Expr, err)
			continue
		}
		if !found || nx.Before(best) {
			best = nx
			found = true
		}
	}
	return best, found
}

// fireDue starts the jobs due in now's minute, skipping any already started
// for that minute.
func (s *Scheduler) fireDue(ctx context.Context, now time.Time) {
	minute := now.Truncate(time.Minute)
	for i := range s.jobs {
		if !s.jobs[i].Schedule.Matches(minute) {
			continue
		}

		s.mu.Lock()
		if s.lastFired[i].Equal(minute) {
			s.mu.Unlock()
			continue
		}
		if !s.allowOverlap && s.running[i] {
			s.mu.Unlock()
			s.logger.Printf("scheduler: skipping %q; previous run still active", s.jobs[i].Command)
			continue
		}
		s.lastFired[i] = minute
		s.running[i] = true
		s.mu.Unlock()

		s.wg.Add(1)
		go func(idx int) {
			defer s.wg.Done()
			res := s.runner.Run(ctx, s.jobs[idx])
			s.mu.Lock()
			s.running[idx] = false
			s.mu.Unlock()
			if res.Err != nil || res.ExitCode != 0 {
				s.logger.Printf("scheduler: job %q exited %d: %v",
					s.jobs[idx].Command, res.ExitCode, res.Err)
			}
		}(i)
	}
}
